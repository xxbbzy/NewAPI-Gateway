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

func TestSyncTokensFetchesFullKeyWhenMasked(t *testing.T) {
	originDB := model.DB
	model.DB = prepareSyncTokenPaginationTestDB(t)
	defer func() { model.DB = originDB }()

	// Mock upstream: /api/token/ returns masked key, /api/token/:id/key returns full key
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/token/" && r.Method == http.MethodGet:
			payload := map[string]interface{}{
				"success": true,
				"message": "",
				"data": map[string]interface{}{
					"page": 0, "page_size": 100, "total": 1,
					"items": []map[string]interface{}{
						{"id": 1, "key": "T32r**********XQMn", "name": "t1", "status": 1, "group": "g1"},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(payload)
		case r.URL.Path == "/api/token/1/key" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"data":    "T32rABCDEFGHIJKLXQMn",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewUpstreamClient(server.URL, "token", 1)
	provider := &model.Provider{Id: 1, Name: "p-masked", Priority: 0, Weight: 10}
	if err := syncTokens(client, provider); err != nil {
		t.Fatalf("syncTokens failed: %v", err)
	}

	var tokens []model.ProviderToken
	if err := model.DB.Where("provider_id = ?", 1).Find(&tokens).Error; err != nil {
		t.Fatalf("query synced tokens failed: %v", err)
	}
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(tokens))
	}
	if tokens[0].SkKey != "sk-T32rABCDEFGHIJKLXQMn" {
		t.Fatalf("expected full sk_key 'sk-T32rABCDEFGHIJKLXQMn', got %q", tokens[0].SkKey)
	}
}

func TestSyncTokensPreservesExistingKeyWhenFetchFails(t *testing.T) {
	originDB := model.DB
	model.DB = prepareSyncTokenPaginationTestDB(t)
	defer func() { model.DB = originDB }()

	// Seed an existing token with a real key
	if err := model.DB.Create(&model.ProviderToken{
		ProviderId:      1,
		UpstreamTokenId: 42,
		SkKey:           "sk-RealFullKeyValue123",
		Name:            "existing",
		GroupName:       "default",
		Status:          1,
	}).Error; err != nil {
		t.Fatalf("seed existing token failed: %v", err)
	}

	// Mock: /api/token/ returns masked key, /api/token/42/key returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/token/" && r.Method == http.MethodGet:
			payload := map[string]interface{}{
				"success": true,
				"message": "",
				"data": map[string]interface{}{
					"page": 0, "page_size": 100, "total": 1,
					"items": []map[string]interface{}{
						{"id": 42, "key": "Real**********e123", "name": "existing-updated", "status": 1, "group": "default"},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(payload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewUpstreamClient(server.URL, "token", 1)
	provider := &model.Provider{Id: 1, Name: "p-preserve", Priority: 0, Weight: 10}
	if err := syncTokens(client, provider); err != nil {
		t.Fatalf("syncTokens failed: %v", err)
	}

	var token model.ProviderToken
	if err := model.DB.Where("provider_id = ? AND upstream_token_id = ?", 1, 42).First(&token).Error; err != nil {
		t.Fatalf("query token failed: %v", err)
	}
	// Key should be preserved (not overwritten with masked value)
	if token.SkKey != "sk-RealFullKeyValue123" {
		t.Fatalf("expected preserved sk_key 'sk-RealFullKeyValue123', got %q", token.SkKey)
	}
	// But name should be updated
	if token.Name != "existing-updated" {
		t.Fatalf("expected updated name 'existing-updated', got %q", token.Name)
	}
}
