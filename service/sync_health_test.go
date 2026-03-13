package service

import (
	"NewAPI-Gateway/model"
	"testing"
)

func TestSyncProviderUsesLatestReachabilitySignalPerRun(t *testing.T) {
	originDB := model.DB
	model.DB = prepareServiceCheckinTestDB(t)
	defer func() { model.DB = originDB }()

	originClientFactory := newUpstreamClientForProvider
	originSyncPricing := syncPricingStep
	originSyncTokens := syncTokensStep
	originSyncBalance := syncBalanceStep
	originRebuildRoutes := rebuildProviderRoutesForProvider
	defer func() {
		newUpstreamClientForProvider = originClientFactory
		syncPricingStep = originSyncPricing
		syncTokensStep = originSyncTokens
		syncBalanceStep = originSyncBalance
		rebuildProviderRoutesForProvider = originRebuildRoutes
	}()

	provider := &model.Provider{
		Name:        "sync-health-provider",
		BaseURL:     "https://example.com",
		AccessToken: "token",
		UserID:      1,
		Status:      1,
	}
	if err := provider.Insert(); err != nil {
		t.Fatalf("insert provider failed: %v", err)
	}

	newUpstreamClientForProvider = func(provider *model.Provider) (*UpstreamClient, error) {
		return &UpstreamClient{Provider: provider}, nil
	}
	syncPricingStep = func(client *UpstreamClient, provider *model.Provider) error {
		return &UpstreamRequestError{Message: "dial tcp timeout", Transport: true}
	}
	syncTokensStep = func(client *UpstreamClient, provider *model.Provider) error {
		return nil
	}
	syncBalanceStep = func(client *UpstreamClient, provider *model.Provider) error {
		return nil
	}
	rebuildProviderRoutesForProvider = func(providerID int) error {
		return nil
	}

	if err := SyncProvider(provider); err != nil {
		t.Fatalf("sync provider returned unexpected error: %v", err)
	}

	reloaded, err := model.GetProviderById(provider.Id)
	if err != nil {
		t.Fatalf("reload provider failed: %v", err)
	}
	if reloaded.HealthStatus != model.ProviderHealthStatusHealthy {
		t.Fatalf("expected later successful sync steps to restore healthy status, got %s", reloaded.HealthStatus)
	}
	if reloaded.HealthFailureReason != "" {
		t.Fatalf("expected failure reason to be cleared after later successful sync steps, got %q", reloaded.HealthFailureReason)
	}

	syncPricingStep = func(client *UpstreamClient, provider *model.Provider) error {
		return &UpstreamRequestError{Message: "dial tcp timeout", Transport: true}
	}
	syncTokensStep = func(client *UpstreamClient, provider *model.Provider) error {
		return &UpstreamRequestError{Message: "dial tcp timeout", Transport: true}
	}
	syncBalanceStep = func(client *UpstreamClient, provider *model.Provider) error {
		return &UpstreamRequestError{Message: "dial tcp timeout", Transport: true}
	}

	if err := SyncProvider(provider); err != nil {
		t.Fatalf("sync provider repeated failure run returned unexpected error: %v", err)
	}

	failed, err := model.GetProviderById(provider.Id)
	if err != nil {
		t.Fatalf("reload failed provider failed: %v", err)
	}
	if failed.HealthStatus != model.ProviderHealthStatusUnreachable {
		t.Fatalf("expected latest failed reachability signal to mark provider unreachable, got %s", failed.HealthStatus)
	}
	if failed.HealthFailureReason != "dial tcp timeout" {
		t.Fatalf("expected failure reason from latest failed reachability signal, got %q", failed.HealthFailureReason)
	}
}
