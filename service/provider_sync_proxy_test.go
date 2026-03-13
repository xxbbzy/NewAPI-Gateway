package service

import (
	"NewAPI-Gateway/model"
	"net/http"
	"strings"
	"testing"
)

func TestSyncProviderMarksHealthFailureOnInvalidProxyConfig(t *testing.T) {
	originDB := model.DB
	model.DB = prepareServiceCheckinTestDB(t)
	defer func() { model.DB = originDB }()

	provider := &model.Provider{
		Name:         "sync-invalid-proxy",
		BaseURL:      "https://example.com",
		AccessToken:  "token",
		UserID:       1,
		Status:       1,
		ProxyEnabled: true,
		ProxyURL:     "proxy.example.com:7890",
	}
	if err := provider.Insert(); err != nil {
		t.Fatalf("insert provider failed: %v", err)
	}

	err := SyncProvider(provider)
	if err == nil {
		t.Fatalf("expected sync to fail for invalid proxy configuration")
	}
	if !strings.Contains(err.Error(), "代理地址格式无效") {
		t.Fatalf("unexpected sync error: %v", err)
	}

	reloaded, queryErr := model.GetProviderById(provider.Id)
	if queryErr != nil {
		t.Fatalf("query provider failed: %v", queryErr)
	}
	if reloaded.HealthStatus != model.ProviderHealthStatusUnreachable {
		t.Fatalf("expected provider to be marked unreachable, got %s", reloaded.HealthStatus)
	}
	if !strings.Contains(reloaded.HealthFailureReason, "代理地址格式无效") {
		t.Fatalf("unexpected health failure reason: %s", reloaded.HealthFailureReason)
	}
}

func TestRunProviderCheckinMarksHealthFailureOnInvalidProxyConfig(t *testing.T) {
	originDB := model.DB
	model.DB = prepareServiceCheckinTestDB(t)
	defer func() { model.DB = originDB }()

	provider := &model.Provider{
		Name:           "checkin-invalid-proxy",
		BaseURL:        "https://example.com",
		AccessToken:    "token",
		UserID:         1,
		Status:         1,
		CheckinEnabled: true,
		ProxyEnabled:   true,
		ProxyURL:       "proxy.example.com:7890",
	}
	if err := provider.Insert(); err != nil {
		t.Fatalf("insert provider failed: %v", err)
	}

	run, item, err := RunProviderCheckin(provider)
	if err == nil {
		t.Fatalf("expected checkin to fail for invalid proxy configuration")
	}
	if run == nil || item == nil {
		t.Fatalf("expected run and item to be recorded")
	}
	if item.Status != "failed" {
		t.Fatalf("expected failed item status, got %s", item.Status)
	}
	if !strings.Contains(item.Message, "代理地址格式无效") {
		t.Fatalf("unexpected checkin item message: %s", item.Message)
	}

	reloaded, queryErr := model.GetProviderById(provider.Id)
	if queryErr != nil {
		t.Fatalf("query provider failed: %v", queryErr)
	}
	if reloaded.HealthStatus != model.ProviderHealthStatusUnreachable {
		t.Fatalf("expected provider to be marked unreachable, got %s", reloaded.HealthStatus)
	}
}

func TestGetRelayHTTPClientForProviderFailsOnInvalidProxyConfig(t *testing.T) {
	relayHTTPClientCache.mu.Lock()
	relayHTTPClientCache.clients = make(map[string]*http.Client)
	relayHTTPClientCache.mu.Unlock()

	provider := &model.Provider{
		ProxyEnabled: true,
		ProxyURL:     "proxy.example.com:7890",
	}

	if _, err := getRelayHTTPClientForProvider(provider); err == nil {
		t.Fatalf("expected relay client creation to fail for invalid proxy configuration")
	}
}
