package service

import (
	"NewAPI-Gateway/model"
	"net/http"
	"net/http/httptest"
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
	if reloaded.HealthStatus != model.ProviderHealthStatusUnknown {
		t.Fatalf("expected checkin failure to leave provider health unchanged, got %s", reloaded.HealthStatus)
	}
}

func TestRunProviderCheckinSuccessDoesNotRestoreProviderHealth(t *testing.T) {
	originDB := model.DB
	model.DB = prepareServiceCheckinTestDB(t)
	defer func() { model.DB = originDB }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true,"message":"签到成功","data":{"quota_awarded":1}}`))
	}))
	defer server.Close()

	provider := &model.Provider{
		Name:                "checkin-success-no-health-reset",
		BaseURL:             server.URL,
		AccessToken:         "token",
		UserID:              1,
		Status:              1,
		CheckinEnabled:      true,
		HealthStatus:        model.ProviderHealthStatusUnreachable,
		HealthFailureAt:     123,
		HealthFailureReason: "sync failed earlier",
		HealthCooldownUntil: 456,
	}
	if err := provider.Insert(); err != nil {
		t.Fatalf("insert provider failed: %v", err)
	}

	run, item, err := RunProviderCheckin(provider)
	if err != nil {
		t.Fatalf("expected checkin success, got %v", err)
	}
	if run == nil || item == nil || item.Status != "success" {
		t.Fatalf("expected successful checkin run and item, got run=%v item=%+v", run, item)
	}

	reloaded, queryErr := model.GetProviderById(provider.Id)
	if queryErr != nil {
		t.Fatalf("query provider failed: %v", queryErr)
	}
	if reloaded.HealthStatus != model.ProviderHealthStatusUnreachable {
		t.Fatalf("expected checkin success to preserve existing provider health, got %s", reloaded.HealthStatus)
	}
	if reloaded.HealthFailureReason != "sync failed earlier" {
		t.Fatalf("expected existing health failure reason to remain unchanged, got %s", reloaded.HealthFailureReason)
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

func TestProxyToUpstreamUpdatesProviderHealthFromLiveTraffic(t *testing.T) {
	originDB := model.DB
	model.DB = prepareNotificationServiceTestDB(t)
	defer func() { model.DB = originDB }()

	originRunner := notificationDispatchRunner
	originAsync := notificationAsyncExecutor
	defer func() {
		notificationDispatchRunner = originRunner
		notificationAsyncExecutor = originAsync
	}()
	notificationAsyncExecutor = func(fn func()) { fn() }
	notificationDispatchRunner = func(event NotificationEvent) error { return nil }

	relayHTTPClientCache.mu.Lock()
	relayHTTPClientCache.clients = make(map[string]*http.Client)
	relayHTTPClientCache.mu.Unlock()

	statusCode := http.StatusServiceUnavailable
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		if statusCode >= http.StatusBadRequest {
			_, _ = w.Write([]byte(`{"error":{"message":"temporary outage"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"chatcmpl-1","object":"chat.completion","model":"gpt-test","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
	defer server.Close()

	provider := &model.Provider{
		Name:        "proxy-health-provider",
		BaseURL:     server.URL,
		AccessToken: "token",
		UserID:      1,
		Status:      1,
	}
	if err := provider.Insert(); err != nil {
		t.Fatalf("insert provider failed: %v", err)
	}

	ctx := newProxyValidationContext(t, "/v1/chat/completions")
	ctx.Set("agg_token", &model.AggregatedToken{Id: 1, UserId: 1})
	token := &model.ProviderToken{Id: 1, SkKey: "sk-live", Status: 1}

	attemptErr := ProxyToUpstream(ctx, token, provider)
	if attemptErr == nil {
		t.Fatalf("expected proxy request to fail while upstream is unavailable")
	}

	reloaded, err := model.GetProviderById(provider.Id)
	if err != nil {
		t.Fatalf("reload provider after proxy failure failed: %v", err)
	}
	if reloaded.HealthStatus != model.ProviderHealthStatusUnreachable {
		t.Fatalf("expected live proxy failure to mark provider unreachable, got %s", reloaded.HealthStatus)
	}

	statusCode = http.StatusOK
	ctx = newProxyValidationContext(t, "/v1/chat/completions")
	ctx.Set("agg_token", &model.AggregatedToken{Id: 1, UserId: 1})

	attemptErr = ProxyToUpstream(ctx, token, provider)
	if attemptErr != nil {
		t.Fatalf("expected proxy request to recover provider health, got %v", attemptErr)
	}

	recovered, err := model.GetProviderById(provider.Id)
	if err != nil {
		t.Fatalf("reload provider after proxy recovery failed: %v", err)
	}
	if recovered.HealthStatus != model.ProviderHealthStatusHealthy {
		t.Fatalf("expected live proxy success to restore healthy status, got %s", recovered.HealthStatus)
	}
	if recovered.HealthFailureReason != "" {
		t.Fatalf("expected recovery to clear health failure reason, got %q", recovered.HealthFailureReason)
	}
}
