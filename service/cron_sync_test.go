package service

import (
	"NewAPI-Gateway/common"
	"NewAPI-Gateway/model"
	"testing"
)

func TestSyncAllProvidersSkipsUnreachableProviders(t *testing.T) {
	originDB := model.DB
	model.DB = prepareServiceCheckinTestDB(t)
	defer func() { model.DB = originDB }()

	providers := []*model.Provider{
		{
			Name:         "healthy",
			BaseURL:      "https://healthy.example.com",
			AccessToken:  "token-a",
			Status:       1,
			HealthStatus: model.ProviderHealthStatusHealthy,
		},
		{
			Name:         "unreachable",
			BaseURL:      "https://down.example.com",
			AccessToken:  "token-b",
			Status:       1,
			HealthStatus: model.ProviderHealthStatusUnreachable,
		},
		{
			Name:         "disabled",
			BaseURL:      "https://disabled.example.com",
			AccessToken:  "token-c",
			Status:       common.UserStatusDisabled,
			HealthStatus: model.ProviderHealthStatusHealthy,
		},
	}
	for _, provider := range providers {
		if err := provider.Insert(); err != nil {
			t.Fatalf("insert provider failed: %v", err)
		}
	}

	originRunner := syncProviderRunner
	defer func() { syncProviderRunner = originRunner }()

	called := make([]int, 0, 1)
	syncProviderRunner = func(provider *model.Provider) error {
		called = append(called, provider.Id)
		return nil
	}

	syncAllProviders()

	if len(called) != 1 {
		t.Fatalf("expected only one provider to be synced, got %d", len(called))
	}
	if called[0] != providers[0].Id {
		t.Fatalf("unexpected provider synced: got id=%d want=%d", called[0], providers[0].Id)
	}
}
