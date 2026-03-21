package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"NewAPI-Gateway/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func prepareSyncTokenPaginationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:service_sync_token_%s_%d?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"), time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db failed: %v", err)
	}
	if err := db.AutoMigrate(&model.ProviderToken{}); err != nil {
		t.Fatalf("migrate test db failed: %v", err)
	}
	return db
}

func TestSyncTokensIncludesFirstPageAndDeduplicates(t *testing.T) {
	originDB := model.DB
	model.DB = prepareSyncTokenPaginationTestDB(t)
	defer func() { model.DB = originDB }()

	// Existing stale token should be removed by cleanup after sync.
	if err := model.DB.Create(&model.ProviderToken{
		ProviderId:      1,
		UpstreamTokenId: 999,
		SkKey:           "sk-stale",
		Name:            "stale",
		GroupName:       "default",
		Status:          1,
	}).Error; err != nil {
		t.Fatalf("seed stale token failed: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/token/") {
			http.NotFound(w, r)
			return
		}
		page := r.URL.Query().Get("p")
		payload := map[string]interface{}{
			"success": true,
			"message": "",
			"data": map[string]interface{}{
				"page_size": 100,
				"total":     3,
			},
		}
		switch page {
		case "0":
			payload["data"].(map[string]interface{})["page"] = 0
			payload["data"].(map[string]interface{})["items"] = []map[string]interface{}{
				{"id": 1, "key": "k1", "name": "t1", "status": 1, "group": "g1"},
				{"id": 2, "key": "k2", "name": "t2", "status": 1, "group": "g1"},
			}
		case "1":
			payload["data"].(map[string]interface{})["page"] = 1
			payload["data"].(map[string]interface{})["items"] = []map[string]interface{}{
				{"id": 2, "key": "k2", "name": "t2", "status": 1, "group": "g1"},
				{"id": 3, "key": "k3", "name": "t3", "status": 1, "group": "g1"},
			}
		default:
			payload["data"].(map[string]interface{})["page"] = 2
			payload["data"].(map[string]interface{})["items"] = []map[string]interface{}{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	client := NewUpstreamClient(server.URL, "token", 1)
	provider := &model.Provider{Id: 1, Name: "p1", Priority: 0, Weight: 10}
	if err := syncTokens(client, provider); err != nil {
		t.Fatalf("syncTokens failed: %v", err)
	}

	var tokens []model.ProviderToken
	if err := model.DB.Where("provider_id = ?", 1).Order("upstream_token_id asc").Find(&tokens).Error; err != nil {
		t.Fatalf("query synced tokens failed: %v", err)
	}
	if len(tokens) != 3 {
		t.Fatalf("expected 3 unique tokens after sync, got %d", len(tokens))
	}
	if tokens[0].UpstreamTokenId != 1 {
		t.Fatalf("expected first-page token id=1 to be included, got first id=%d", tokens[0].UpstreamTokenId)
	}
	for _, token := range tokens {
		if token.UpstreamTokenId == 999 {
			t.Fatalf("stale token id=999 should have been removed")
		}
	}
}

func TestSyncTokensDoesNotPersistMaskedUpstreamKey(t *testing.T) {
	originDB := model.DB
	model.DB = prepareSyncTokenPaginationTestDB(t)
	defer func() { model.DB = originDB }()

	if err := model.DB.Create(&model.ProviderToken{
		ProviderId:      1,
		UpstreamTokenId: 10,
		SkKey:           "sk-abcd**********wxyz",
		Name:            "existing",
		GroupName:       "default",
		Status:          1,
	}).Error; err != nil {
		t.Fatalf("seed existing token failed: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/token/":
			payload := map[string]interface{}{
				"success": true,
				"message": "",
				"data": map[string]interface{}{
					"page":      0,
					"page_size": 100,
					"total":     2,
					"items": []map[string]interface{}{
						{"id": 10, "key": "abcd**********wxyz", "name": "existing", "status": 1, "group": "default"},
						{"id": 11, "key": "lmno**********pqrs", "name": "new", "status": 1, "group": "default"},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(payload)
		case "/api/token/10/key":
			_, _ = w.Write([]byte(`{"success":true,"message":"","data":{"key":"real-existing-key"}}`))
		case "/api/token/11/key":
			_, _ = w.Write([]byte(`{"success":true,"message":"","data":{"key":"real-new-key"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewUpstreamClient(server.URL, "token", 1)
	provider := &model.Provider{Id: 1, Name: "p1", Priority: 0, Weight: 10}
	if err := syncTokens(client, provider); err != nil {
		t.Fatalf("syncTokens failed: %v", err)
	}

	var existing model.ProviderToken
	if err := model.DB.Where("provider_id = ? AND upstream_token_id = ?", 1, 10).First(&existing).Error; err != nil {
		t.Fatalf("query existing token failed: %v", err)
	}
	if existing.SkKey != "sk-real-existing-key" {
		t.Fatalf("expected existing masked key to be healed, got %q", existing.SkKey)
	}

	var created model.ProviderToken
	if err := model.DB.Where("provider_id = ? AND upstream_token_id = ?", 1, 11).First(&created).Error; err != nil {
		t.Fatalf("query created token failed: %v", err)
	}
	if created.SkKey != "sk-real-new-key" {
		t.Fatalf("expected new token key to be fetched from key endpoint, got %q", created.SkKey)
	}
}
