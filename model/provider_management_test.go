package model

import (
	"NewAPI-Gateway/common"
	"fmt"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func prepareProviderManagementTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:model_provider_management_%s_%d?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"), time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db failed: %v", err)
	}
	if err := db.AutoMigrate(&Provider{}, &ProviderToken{}, &ModelRoute{}, &ModelPricing{}, &CheckinRun{}, &CheckinRunItem{}); err != nil {
		t.Fatalf("migrate test db failed: %v", err)
	}
	return db
}

func TestQueryProvidersSupportsKeywordFiltering(t *testing.T) {
	originDB := DB
	DB = prepareProviderManagementTestDB(t)
	defer func() { DB = originDB }()

	items := []*Provider{
		{Name: "Alpha Route", BaseURL: "https://alpha.example.com", AccessToken: "a", Remark: "primary"},
		{Name: "Beta Route", BaseURL: "https://beta.example.com", AccessToken: "b", Remark: "backup"},
	}
	for _, item := range items {
		if err := item.Insert(); err != nil {
			t.Fatalf("insert provider failed: %v", err)
		}
	}

	filtered, total, err := QueryProviders("alpha", ProviderRouteFilterAll, 0, 10)
	if err != nil {
		t.Fatalf("query providers failed: %v", err)
	}
	if total != 1 || len(filtered) != 1 {
		t.Fatalf("expected one filtered provider, total=%d len=%d", total, len(filtered))
	}
	if filtered[0].Name != "Alpha Route" {
		t.Fatalf("unexpected provider returned: %s", filtered[0].Name)
	}
}

func TestQueryProvidersSupportsUserIDFiltering(t *testing.T) {
	originDB := DB
	DB = prepareProviderManagementTestDB(t)
	defer func() { DB = originDB }()

	items := []*Provider{
		{Name: "Alpha Route", BaseURL: "https://alpha.example.com", AccessToken: "a", UserID: 7},
		{Name: "Beta Route", BaseURL: "https://beta.example.com", AccessToken: "b", UserID: 42},
	}
	for _, item := range items {
		if err := item.Insert(); err != nil {
			t.Fatalf("insert provider failed: %v", err)
		}
	}

	filtered, total, err := QueryProviders("42", ProviderRouteFilterAll, 0, 10)
	if err != nil {
		t.Fatalf("query providers failed: %v", err)
	}
	if total != 1 || len(filtered) != 1 {
		t.Fatalf("expected one filtered provider, total=%d len=%d", total, len(filtered))
	}
	if filtered[0].UserID != 42 {
		t.Fatalf("unexpected provider returned: %+v", filtered[0])
	}
}

func TestGetProviderSummaryAggregatesBalanceAndHealth(t *testing.T) {
	originDB := DB
	DB = prepareProviderManagementTestDB(t)
	defer func() { DB = originDB }()

	now := time.Now().Unix()
	providers := []*Provider{
		{
			Name:           "healthy-provider",
			BaseURL:        "https://healthy.example.com",
			AccessToken:    "a",
			Balance:        "$12.50",
			BalanceUpdated: now,
			HealthStatus:   ProviderHealthStatusHealthy,
			ProxyEnabled:   true,
			CheckinEnabled: true,
			Status:         common.UserStatusEnabled,
		},
		{
			Name:                "unreachable-provider",
			BaseURL:             "https://down.example.com",
			AccessToken:         "b",
			Balance:             "$3.25",
			BalanceUpdated:      now - int64(3*24*time.Hour/time.Second),
			HealthStatus:        ProviderHealthStatusUnreachable,
			HealthFailureReason: "dial tcp timeout",
			Status:              common.UserStatusEnabled,
		},
		{
			Name:        "never-updated-provider",
			BaseURL:     "https://unknown.example.com",
			AccessToken: "c",
			Status:      common.UserStatusEnabled,
		},
	}
	for _, provider := range providers {
		if err := provider.Insert(); err != nil {
			t.Fatalf("insert provider failed: %v", err)
		}
	}

	summary, err := GetProviderSummary()
	if err != nil {
		t.Fatalf("get provider summary failed: %v", err)
	}
	if summary.TotalProviders != 3 {
		t.Fatalf("expected total providers=3, got %d", summary.TotalProviders)
	}
	if summary.BalanceAccountCount != 2 {
		t.Fatalf("expected balance account count=2, got %d", summary.BalanceAccountCount)
	}
	if summary.BalanceTotalUSD != 15.75 {
		t.Fatalf("expected balance total 15.75, got %.2f", summary.BalanceTotalUSD)
	}
	if summary.UnreachableProviders != 1 {
		t.Fatalf("expected unreachable providers=1, got %d", summary.UnreachableProviders)
	}
	if summary.ProxyEnabledProviders != 1 {
		t.Fatalf("expected proxy-enabled providers=1, got %d", summary.ProxyEnabledProviders)
	}
	if summary.BalanceFreshCount != 1 || summary.BalanceStaleCount != 1 || summary.BalanceNeverUpdatedCount != 1 {
		t.Fatalf("unexpected balance freshness counts: %+v", summary)
	}
}

func TestGetCheckinEnabledProvidersExcludesUnreachableProvider(t *testing.T) {
	originDB := DB
	DB = prepareProviderManagementTestDB(t)
	defer func() { DB = originDB }()

	providers := []*Provider{
		{
			Name:           "reachable",
			BaseURL:        "https://ok.example.com",
			AccessToken:    "a",
			Status:         common.UserStatusEnabled,
			CheckinEnabled: true,
			HealthStatus:   ProviderHealthStatusHealthy,
		},
		{
			Name:           "unreachable",
			BaseURL:        "https://down.example.com",
			AccessToken:    "b",
			Status:         common.UserStatusEnabled,
			CheckinEnabled: true,
			HealthStatus:   ProviderHealthStatusUnreachable,
		},
	}
	for _, provider := range providers {
		if err := provider.Insert(); err != nil {
			t.Fatalf("insert provider failed: %v", err)
		}
	}

	eligible, err := GetCheckinEnabledProviders()
	if err != nil {
		t.Fatalf("get checkin enabled providers failed: %v", err)
	}
	if len(eligible) != 1 || eligible[0].Name != "reachable" {
		t.Fatalf("unexpected eligible providers: %+v", eligible)
	}
}

func TestProviderCooldownExpiryAllowsAutomatedUseAgainWithoutClearingResponseStatus(t *testing.T) {
	originDB := DB
	DB = prepareProviderManagementTestDB(t)
	defer func() { DB = originDB }()

	expiredCooldown := time.Now().Add(-time.Minute).Unix()
	provider := &Provider{
		Name:                "recovering-provider",
		BaseURL:             "https://recovering.example.com",
		AccessToken:         "token",
		Status:              common.UserStatusEnabled,
		CheckinEnabled:      true,
		HealthStatus:        ProviderHealthStatusUnreachable,
		HealthCooldownUntil: expiredCooldown,
	}
	if err := provider.Insert(); err != nil {
		t.Fatalf("insert provider failed: %v", err)
	}

	eligible, err := GetCheckinEnabledProviders()
	if err != nil {
		t.Fatalf("get checkin enabled providers failed: %v", err)
	}
	if len(eligible) != 1 || eligible[0].Id != provider.Id {
		t.Fatalf("expected provider to rejoin automated use after cooldown, got %+v", eligible)
	}

	reloaded, err := GetProviderById(provider.Id)
	if err != nil {
		t.Fatalf("reload provider failed: %v", err)
	}
	reloaded.CleanForResponse()
	if reloaded.HealthBlocked {
		t.Fatalf("expected provider not to be marked blocked after cooldown")
	}
	if reloaded.HealthStatus != ProviderHealthStatusUnreachable {
		t.Fatalf("expected response health status to retain unreachable after cooldown, got %s", reloaded.HealthStatus)
	}
}

func TestBuildRouteAttemptsByPriorityExcludesUnreachableProviders(t *testing.T) {
	originDB := DB
	DB = prepareProviderManagementTestDB(t)
	defer func() { DB = originDB }()
	invalidateModelRouteCaches()
	defer invalidateModelRouteCaches()

	provider := &Provider{
		Name:         "down-provider",
		BaseURL:      "https://down.example.com",
		AccessToken:  "token",
		Status:       common.UserStatusEnabled,
		HealthStatus: ProviderHealthStatusUnreachable,
	}
	if err := provider.Insert(); err != nil {
		t.Fatalf("insert provider failed: %v", err)
	}
	token := &ProviderToken{
		ProviderId: provider.Id,
		Name:       "token-1",
		GroupName:  "default",
		Status:     common.UserStatusEnabled,
		Priority:   0,
		Weight:     10,
	}
	if err := token.Insert(); err != nil {
		t.Fatalf("insert token failed: %v", err)
	}
	if err := RebuildRoutesForProvider(provider.Id, []ModelRoute{
		{ModelName: "gpt-4o-mini", ProviderId: provider.Id, ProviderTokenId: token.Id, Enabled: true, Priority: 0, Weight: 10},
	}); err != nil {
		t.Fatalf("rebuild routes failed: %v", err)
	}

	if _, err := BuildRouteAttemptsByPriority("gpt-4o-mini"); err == nil {
		t.Fatalf("expected unreachable provider to be excluded from route attempts")
	}
}

func TestProviderRouteEligibilityByLocalDayWindow(t *testing.T) {
	originOptions := common.OptionMap
	common.OptionMap = map[string]string{"CheckinScheduleTimezone": "Asia/Shanghai"}
	defer func() { common.OptionMap = originOptions }()

	provider := &Provider{
		Status:         common.UserStatusEnabled,
		HealthStatus:   ProviderHealthStatusHealthy,
		BalanceUpdated: time.Date(2026, 5, 4, 0, 0, 0, 0, time.FixedZone("UTC+8", 8*3600)).Unix(),
	}
	now := time.Date(2026, 5, 5, 16, 5, 48, 0, time.FixedZone("UTC+8", 8*3600))
	if !provider.IsRouteEligibleAt(now) {
		t.Fatalf("expected provider updated on previous local day to remain route-eligible")
	}

	provider.BalanceUpdated = time.Date(2026, 5, 3, 23, 59, 59, 0, time.FixedZone("UTC+8", 8*3600)).Unix()
	if provider.IsRouteEligibleAt(now) {
		t.Fatalf("expected provider updated before two-day local window to be route-blocked")
	}
	if !containsProviderRouteBlockReason(provider.RouteBlockReasonsAt(now), ProviderRouteBlockReasonBalanceStale) {
		t.Fatalf("expected balance_stale route block reason")
	}
}

func TestBuildRouteAttemptsByPriorityExcludesBalanceStaleProviders(t *testing.T) {
	originDB := DB
	originOptions := common.OptionMap
	DB = prepareProviderManagementTestDB(t)
	common.OptionMap = map[string]string{"CheckinScheduleTimezone": "Asia/Shanghai"}
	defer func() {
		DB = originDB
		common.OptionMap = originOptions
	}()
	invalidateModelRouteCaches()
	defer invalidateModelRouteCaches()

	provider := &Provider{
		Name:           "stale-provider",
		BaseURL:        "https://stale.example.com",
		AccessToken:    "token",
		Status:         common.UserStatusEnabled,
		HealthStatus:   ProviderHealthStatusHealthy,
		BalanceUpdated: time.Now().AddDate(0, 0, -3).Unix(),
	}
	if err := provider.Insert(); err != nil {
		t.Fatalf("insert provider failed: %v", err)
	}
	token := &ProviderToken{
		ProviderId: provider.Id,
		Name:       "token-1",
		GroupName:  "default",
		Status:     common.UserStatusEnabled,
		Priority:   0,
		Weight:     10,
	}
	if err := token.Insert(); err != nil {
		t.Fatalf("insert token failed: %v", err)
	}
	if err := RebuildRoutesForProvider(provider.Id, []ModelRoute{
		{ModelName: "gpt-4o-mini", ProviderId: provider.Id, ProviderTokenId: token.Id, Enabled: true, Priority: 0, Weight: 10},
	}); err != nil {
		t.Fatalf("rebuild routes failed: %v", err)
	}

	if _, err := BuildRouteAttemptsByPriority("gpt-4o-mini"); err == nil {
		t.Fatalf("expected balance-stale provider to be excluded from route attempts")
	}
}

func TestRouteEligibilityDoesNotAffectAutomatedProviderLists(t *testing.T) {
	originDB := DB
	DB = prepareProviderManagementTestDB(t)
	defer func() { DB = originDB }()

	provider := &Provider{
		Name:           "stale-but-reachable",
		BaseURL:        "https://ok.example.com",
		AccessToken:    "a",
		Status:         common.UserStatusEnabled,
		CheckinEnabled: true,
		HealthStatus:   ProviderHealthStatusHealthy,
		BalanceUpdated: time.Now().AddDate(0, 0, -5).Unix(),
	}
	if err := provider.Insert(); err != nil {
		t.Fatalf("insert provider failed: %v", err)
	}

	syncProviders, err := GetAutomatedSyncProviders()
	if err != nil {
		t.Fatalf("get automated sync providers failed: %v", err)
	}
	if len(syncProviders) != 1 || syncProviders[0].Id != provider.Id {
		t.Fatalf("expected stale provider to remain in automated sync list, got %+v", syncProviders)
	}

	checkinProviders, err := GetCheckinEnabledProviders()
	if err != nil {
		t.Fatalf("get checkin enabled providers failed: %v", err)
	}
	if len(checkinProviders) != 1 || checkinProviders[0].Id != provider.Id {
		t.Fatalf("expected stale provider to remain in checkin-enabled list, got %+v", checkinProviders)
	}
}

func TestQueryProvidersSupportsRouteFilters(t *testing.T) {
	originDB := DB
	originOptions := common.OptionMap
	DB = prepareProviderManagementTestDB(t)
	common.OptionMap = map[string]string{"CheckinScheduleTimezone": "Asia/Shanghai"}
	defer func() {
		DB = originDB
		common.OptionMap = originOptions
	}()

	now := time.Date(2026, 5, 5, 16, 5, 48, 0, time.FixedZone("UTC+8", 8*3600))
	freshUpdated := time.Date(2026, 5, 4, 10, 0, 0, 0, now.Location()).Unix()
	staleUpdated := time.Date(2026, 5, 3, 10, 0, 0, 0, now.Location()).Unix()

	items := []*Provider{
		{Name: "eligible-alpha", BaseURL: "https://eligible.example.com", AccessToken: "a", Status: common.UserStatusEnabled, HealthStatus: ProviderHealthStatusHealthy, BalanceUpdated: freshUpdated},
		{Name: "down-beta", BaseURL: "https://down.example.com", AccessToken: "b", Status: common.UserStatusEnabled, HealthStatus: ProviderHealthStatusUnreachable, BalanceUpdated: freshUpdated},
		{Name: "stale-gamma", BaseURL: "https://stale.example.com", AccessToken: "c", Status: common.UserStatusEnabled, HealthStatus: ProviderHealthStatusHealthy, BalanceUpdated: staleUpdated},
	}
	for _, item := range items {
		if err := item.Insert(); err != nil {
			t.Fatalf("insert provider failed: %v", err)
		}
	}

	providers, total, err := QueryProviders("", ProviderRouteFilterAbnormal, 0, 10)
	if err != nil {
		t.Fatalf("query abnormal providers failed: %v", err)
	}
	if total != 2 || len(providers) != 2 {
		t.Fatalf("expected 2 abnormal providers, total=%d len=%d", total, len(providers))
	}

	providers, total, err = QueryProviders("", ProviderRouteFilterSiteUnavailable, 0, 10)
	if err != nil {
		t.Fatalf("query site-unavailable providers failed: %v", err)
	}
	if total != 1 || len(providers) != 1 || providers[0].Name != "down-beta" {
		t.Fatalf("unexpected site-unavailable providers: total=%d items=%+v", total, providers)
	}

	providers, total, err = QueryProviders("", ProviderRouteFilterBalanceStale, 0, 10)
	if err != nil {
		t.Fatalf("query balance-stale providers failed: %v", err)
	}
	if total != 1 || len(providers) != 1 || providers[0].Name != "stale-gamma" {
		t.Fatalf("unexpected balance-stale providers: total=%d items=%+v", total, providers)
	}

	providers, total, err = QueryProviders("stale", ProviderRouteFilterAbnormal, 0, 10)
	if err != nil {
		t.Fatalf("query abnormal providers by keyword failed: %v", err)
	}
	if total != 1 || len(providers) != 1 || providers[0].Name != "stale-gamma" {
		t.Fatalf("unexpected keyword+filter result: total=%d items=%+v", total, providers)
	}
}

func TestQueryProvidersEligibleFilterExcludesDisabledProviders(t *testing.T) {
	originDB := DB
	originOptions := common.OptionMap
	DB = prepareProviderManagementTestDB(t)
	common.OptionMap = map[string]string{"CheckinScheduleTimezone": "Asia/Shanghai"}
	defer func() {
		DB = originDB
		common.OptionMap = originOptions
	}()

	enabled := &Provider{
		Name:           "enabled-route",
		BaseURL:        "https://enabled.example.com",
		AccessToken:    "a",
		Status:         common.UserStatusEnabled,
		HealthStatus:   ProviderHealthStatusHealthy,
		BalanceUpdated: time.Now().Add(-12 * time.Hour).Unix(),
	}
	disabled := &Provider{
		Name:           "disabled-route",
		BaseURL:        "https://disabled.example.com",
		AccessToken:    "b",
		Status:         common.UserStatusDisabled,
		HealthStatus:   ProviderHealthStatusHealthy,
		BalanceUpdated: time.Now().Add(-12 * time.Hour).Unix(),
	}
	for _, item := range []*Provider{enabled, disabled} {
		if err := item.Insert(); err != nil {
			t.Fatalf("insert provider failed: %v", err)
		}
	}

	providers, total, err := QueryProviders("", ProviderRouteFilterEligible, 0, 10)
	if err != nil {
		t.Fatalf("query eligible providers failed: %v", err)
	}
	if total != 1 || len(providers) != 1 || providers[0].Name != "enabled-route" {
		t.Fatalf("unexpected eligible providers: total=%d items=%+v", total, providers)
	}
}

func TestProviderResponseCleaningRedactsProxyURL(t *testing.T) {
	provider := &Provider{
		AccessToken:  "secret-token",
		ProxyEnabled: true,
		ProxyURL:     "http://user:pass@proxy.example.com:7890",
	}

	provider.CleanForResponse()

	if provider.AccessToken != "" {
		t.Fatalf("expected access token to be cleared")
	}
	if provider.ProxyURL != "" {
		t.Fatalf("expected proxy URL to be cleared")
	}
	if provider.ProxyURLRedacted != "http://proxy.example.com:7890" {
		t.Fatalf("unexpected redacted proxy URL: %s", provider.ProxyURLRedacted)
	}
}

func TestMarkHealthSuccessClearsFailureMetadata(t *testing.T) {
	originDB := DB
	DB = prepareProviderManagementTestDB(t)
	defer func() { DB = originDB }()

	provider := &Provider{
		Name:                "recover-success",
		BaseURL:             "https://recover-success.example.com",
		AccessToken:         "token",
		Status:              common.UserStatusEnabled,
		HealthStatus:        ProviderHealthStatusUnreachable,
		HealthFailureAt:     time.Now().Add(-time.Hour).Unix(),
		HealthFailureReason: "dial tcp timeout",
		HealthCooldownUntil: time.Now().Add(time.Minute).Unix(),
	}
	if err := provider.Insert(); err != nil {
		t.Fatalf("insert provider failed: %v", err)
	}

	if err := provider.MarkHealthSuccess(); err != nil {
		t.Fatalf("mark health success failed: %v", err)
	}

	reloaded, err := GetProviderById(provider.Id)
	if err != nil {
		t.Fatalf("reload provider failed: %v", err)
	}
	if reloaded.HealthStatus != ProviderHealthStatusHealthy {
		t.Fatalf("expected healthy status, got %s", reloaded.HealthStatus)
	}
	if reloaded.HealthFailureAt != 0 {
		t.Fatalf("expected failure timestamp cleared, got %d", reloaded.HealthFailureAt)
	}
	if reloaded.HealthFailureReason != "" {
		t.Fatalf("expected failure reason cleared, got %q", reloaded.HealthFailureReason)
	}
}
