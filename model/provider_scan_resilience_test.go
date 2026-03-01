package model

import "testing"

func TestGetUncheckinProvidersToleratesNonNumericCreatedAt(t *testing.T) {
	originDB := DB
	DB = prepareCheckinTestDB(t)
	defer func() { DB = originDB }()

	dayStart := int64(1700000000)
	provider := &Provider{
		Name:           "legacy-created-at-provider",
		BaseURL:        "https://legacy.example.com",
		AccessToken:    "token",
		Status:         1,
		CheckinEnabled: true,
		LastCheckinAt:  dayStart - 100,
	}
	if err := provider.Insert(); err != nil {
		t.Fatalf("insert provider failed: %v", err)
	}
	legacyTextValue := `{"default":"默认分组","vip":"vip分组"}`
	if err := DB.Exec("UPDATE providers SET created_at = ? WHERE id = ?", legacyTextValue, provider.Id).Error; err != nil {
		t.Fatalf("inject legacy created_at text failed: %v", err)
	}

	providers, err := GetUncheckinProviders(dayStart)
	if err != nil {
		t.Fatalf("query uncheckin providers should tolerate legacy created_at text, got err=%v", err)
	}
	if len(providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(providers))
	}
	if providers[0].Id != provider.Id {
		t.Fatalf("unexpected provider returned, got id=%d expected id=%d", providers[0].Id, provider.Id)
	}
	// CAST on invalid INTEGER text falls back to 0 in SQLite.
	if providers[0].CreatedAt != 0 {
		t.Fatalf("expected tolerant cast created_at=0 for legacy text, got %d", providers[0].CreatedAt)
	}
}
