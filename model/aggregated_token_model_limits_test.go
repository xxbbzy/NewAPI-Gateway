package model

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func prepareModelCatalogPermissionTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:model_catalog_permission_%s_%d?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"), time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db failed: %v", err)
	}
	if err := db.AutoMigrate(&Provider{}, &ProviderToken{}, &ModelRoute{}, &ModelPricing{}); err != nil {
		t.Fatalf("migrate test db failed: %v", err)
	}
	return db
}

func seedAliasRouteForPermissionTests(t *testing.T) {
	t.Helper()
	provider := &Provider{
		Name:              "permission-provider",
		BaseURL:           "https://permission.example.com",
		AccessToken:       "permission-token",
		Status:            1,
		ModelAliasMapping: `{"aaa":"bbbxxxcccddd"}`,
	}
	if err := provider.Insert(); err != nil {
		t.Fatalf("insert provider failed: %v", err)
	}
	token := &ProviderToken{
		ProviderId: provider.Id,
		Name:       "permission-token",
		GroupName:  "default",
		Status:     1,
		Priority:   0,
		Weight:     10,
	}
	if err := token.Insert(); err != nil {
		t.Fatalf("insert provider token failed: %v", err)
	}

	routes := []ModelRoute{
		{ModelName: "bbbxxxcccddd", ProviderId: provider.Id, ProviderTokenId: token.Id, Enabled: true, Priority: 0, Weight: 10},
	}
	if err := RebuildRoutesForProvider(provider.Id, routes); err != nil {
		t.Fatalf("rebuild routes failed: %v", err)
	}
}

func TestAggregatedTokenIsModelAllowedMatchesCatalogSemantics(t *testing.T) {
	originDB := DB
	DB = prepareModelCatalogPermissionTestDB(t)
	defer func() { DB = originDB }()
	invalidateModelRouteCaches()
	defer invalidateModelRouteCaches()

	seedAliasRouteForPermissionTests(t)

	token := &AggregatedToken{ModelLimitsEnabled: true, ModelLimits: "aaa"}
	if !token.IsModelAllowed("bbbxxxcccddd") {
		t.Fatalf("expected canonical limit aaa to allow target request bbbxxxcccddd")
	}

	token.ModelLimits = "bbbxxxcccddd"
	if !token.IsModelAllowed("aaa") {
		t.Fatalf("expected target limit bbbxxxcccddd to allow canonical request aaa")
	}

	token.ModelLimits = "not-exists"
	if token.IsModelAllowed("aaa") {
		t.Fatalf("expected non-matching limit to reject request")
	}
}
