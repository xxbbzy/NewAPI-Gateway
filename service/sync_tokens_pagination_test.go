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

func TestSyncTokensFetchesDetailWhenListKeyMasked(t *testing.T) {
	originDB := model.DB
	model.DB = prepareSyncTokenPaginationTestDB(t)
	defer func() { model.DB = originDB }()

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
		case r.URL.Path == "/api/token/1" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"message": "",
				"data": map[string]interface{}{
					"id": 1, "key": "T32rABCDEFGHIJKLXQMn", "name": "t1", "status": 1, "group": "g1",
				},
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
	if tokens[0].KeyStatus != model.ProviderTokenKeyStatusReady {
		t.Fatalf("expected ready key_status, got %q", tokens[0].KeyStatus)
	}
}

func TestSyncTokensMarksUnresolvedWhenPlaintextUnavailable(t *testing.T) {
	originDB := model.DB
	model.DB = prepareSyncTokenPaginationTestDB(t)
	defer func() { model.DB = originDB }()

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
						{"id": 7, "key": "abc********xyz", "name": "new-unresolved", "status": 1, "group": "default"},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(payload)
		case r.URL.Path == "/api/token/7" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"message": "",
				"data": map[string]interface{}{
					"id": 7, "key": "", "name": "new-unresolved", "status": 1, "group": "default",
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewUpstreamClient(server.URL, "token", 1)
	provider := &model.Provider{Id: 1, Name: "p-unresolved", Priority: 0, Weight: 10}
	if err := syncTokens(client, provider); err != nil {
		t.Fatalf("syncTokens failed: %v", err)
	}

	var token model.ProviderToken
	if err := model.DB.Where("provider_id = ? AND upstream_token_id = ?", 1, 7).First(&token).Error; err != nil {
		t.Fatalf("query unresolved token failed: %v", err)
	}
	if token.SkKey != "" {
		t.Fatalf("expected empty sk_key for unresolved token, got %q", token.SkKey)
	}
	if token.KeyStatus != model.ProviderTokenKeyStatusUnresolved {
		t.Fatalf("expected unresolved key_status, got %q", token.KeyStatus)
	}
	if token.KeyUnresolvedReason != model.ProviderTokenKeyUnresolvedReasonPlaintextNotRecovered {
		t.Fatalf("unexpected unresolved reason: %q", token.KeyUnresolvedReason)
	}
}

func TestSyncTokensPreservesExistingReadyKeyWhenUpstreamStillUnresolved(t *testing.T) {
	originDB := model.DB
	model.DB = prepareSyncTokenPaginationTestDB(t)
	defer func() { model.DB = originDB }()

	if err := model.DB.Create(&model.ProviderToken{
		ProviderId:          1,
		UpstreamTokenId:     8,
		SkKey:               "sk-existing-plaintext",
		KeyStatus:           model.ProviderTokenKeyStatusReady,
		KeyUnresolvedReason: "",
		Name:                "legacy-name",
		GroupName:           "default",
		Status:              1,
		Priority:            0,
		Weight:              10,
	}).Error; err != nil {
		t.Fatalf("seed ready token failed: %v", err)
	}

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
						{"id": 8, "key": "abc********xyz", "name": "updated-name", "status": 1, "group": "default"},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(payload)
		case r.URL.Path == "/api/token/8" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"message": "",
				"data": map[string]interface{}{
					"id": 8, "key": "", "name": "updated-name", "status": 1, "group": "default",
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewUpstreamClient(server.URL, "token", 1)
	provider := &model.Provider{Id: 1, Name: "p-preserve-ready", Priority: 0, Weight: 10}
	if err := syncTokens(client, provider); err != nil {
		t.Fatalf("syncTokens failed: %v", err)
	}

	var token model.ProviderToken
	if err := model.DB.Where("provider_id = ? AND upstream_token_id = ?", 1, 8).First(&token).Error; err != nil {
		t.Fatalf("query preserved token failed: %v", err)
	}
	if token.SkKey != "sk-existing-plaintext" {
		t.Fatalf("expected existing plaintext key to be preserved, got %q", token.SkKey)
	}
	if token.KeyStatus != model.ProviderTokenKeyStatusReady {
		t.Fatalf("expected ready key_status after preserve, got %q", token.KeyStatus)
	}
	if token.KeyUnresolvedReason != "" {
		t.Fatalf("expected empty unresolved reason for preserved ready key, got %q", token.KeyUnresolvedReason)
	}
	if token.Name != "updated-name" {
		t.Fatalf("expected metadata update to keep latest upstream name, got %q", token.Name)
	}
}

func TestSyncTokensRepairsContaminatedRowWhenDetailReturnsPlaintext(t *testing.T) {
	originDB := model.DB
	model.DB = prepareSyncTokenPaginationTestDB(t)
	defer func() { model.DB = originDB }()

	if err := model.DB.Create(&model.ProviderToken{
		ProviderId:          1,
		UpstreamTokenId:     9,
		SkKey:               "abc********xyz",
		KeyStatus:           model.ProviderTokenKeyStatusUnresolved,
		KeyUnresolvedReason: model.ProviderTokenKeyUnresolvedReasonLegacyContaminated,
		Name:                "dirty-token",
		GroupName:           "default",
		Status:              1,
		Priority:            0,
		Weight:              10,
	}).Error; err != nil {
		t.Fatalf("seed contaminated token failed: %v", err)
	}

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
						{"id": 9, "key": "abc********xyz", "name": "dirty-token", "status": 1, "group": "default"},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(payload)
		case r.URL.Path == "/api/token/9" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"message": "",
				"data": map[string]interface{}{
					"id": 9, "key": "RESTOREDPLAINTEXT", "name": "dirty-token", "status": 1, "group": "default",
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewUpstreamClient(server.URL, "token", 1)
	provider := &model.Provider{Id: 1, Name: "p-repair-dirty", Priority: 0, Weight: 10}
	if err := syncTokens(client, provider); err != nil {
		t.Fatalf("syncTokens failed: %v", err)
	}

	var token model.ProviderToken
	if err := model.DB.Where("provider_id = ? AND upstream_token_id = ?", 1, 9).First(&token).Error; err != nil {
		t.Fatalf("query repaired token failed: %v", err)
	}
	if token.SkKey != "sk-RESTOREDPLAINTEXT" {
		t.Fatalf("expected repaired plaintext key, got %q", token.SkKey)
	}
	if token.KeyStatus != model.ProviderTokenKeyStatusReady {
		t.Fatalf("expected ready key_status after repair, got %q", token.KeyStatus)
	}
	if token.KeyUnresolvedReason != "" {
		t.Fatalf("expected unresolved reason cleared after repair, got %q", token.KeyUnresolvedReason)
	}
}
