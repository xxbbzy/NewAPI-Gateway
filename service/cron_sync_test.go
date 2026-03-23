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
	if err := model.DB.Create(&model.ProviderToken{
		ProviderId:          providers[1].Id,
		UpstreamTokenId:     501,
		SkKey:               "",
		KeyStatus:           model.ProviderTokenKeyStatusUnresolved,
		KeyUnresolvedReason: model.ProviderTokenKeyUnresolvedReasonLegacyContaminated,
		Name:                "dirty-token",
		GroupName:           "default",
		Status:              common.UserStatusEnabled,
	}).Error; err != nil {
		t.Fatalf("seed unresolved provider token failed: %v", err)
	}
	if err := model.DB.Create(&model.ProviderToken{
		ProviderId:          providers[2].Id,
		UpstreamTokenId:     502,
		SkKey:               "",
		KeyStatus:           model.ProviderTokenKeyStatusUnresolved,
		KeyUnresolvedReason: model.ProviderTokenKeyUnresolvedReasonLegacyContaminated,
		Name:                "disabled-dirty-token",
		GroupName:           "default",
		Status:              common.UserStatusEnabled,
	}).Error; err != nil {
		t.Fatalf("seed unresolved token for disabled provider failed: %v", err)
	}

	originRunner := syncProviderRunner
	defer func() { syncProviderRunner = originRunner }()

	called := make([]int, 0, 2)
	syncProviderRunner = func(provider *model.Provider) error {
		called = append(called, provider.Id)
		return nil
	}

	syncAllProviders()

	if len(called) != 2 {
		t.Fatalf("expected healthy sync plus targeted repair, got %d calls", len(called))
	}
	if called[0] != providers[0].Id {
		t.Fatalf("expected first synced provider to be healthy id=%d, got %d", providers[0].Id, called[0])
	}
	if called[1] != providers[1].Id {
		t.Fatalf("expected targeted repair for unreachable provider id=%d, got %d", providers[1].Id, called[1])
	}
	for _, id := range called {
		if id == providers[2].Id {
			t.Fatalf("expected disabled provider id=%d to be skipped by repair sync", providers[2].Id)
		}
	}
}
