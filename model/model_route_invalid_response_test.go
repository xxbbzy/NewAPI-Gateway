package model

import (
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func prepareRouteInvalidResponseTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:route_invalid_response_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db failed: %v", err)
	}
	if err := db.AutoMigrate(&UsageLog{}); err != nil {
		t.Fatalf("migrate db failed: %v", err)
	}
	return db
}

func TestRouteHealthStatsIncludeInvalidResponses(t *testing.T) {
	originDB := DB
	DB = prepareRouteInvalidResponseTestDB(t)
	defer func() { DB = originDB }()
	invalidateRoutingRuntimeMetricCaches()
	defer invalidateRoutingRuntimeMetricCaches()

	now := time.Now().Unix()
	rows := []UsageLog{
		{ProviderTokenId: 1, ModelName: "gpt-health", Status: 1, CreatedAt: now, ResponseTimeMs: 100},
		{ProviderTokenId: 1, ModelName: "gpt-health", Status: 0, FailureCategory: UsageFailureCategoryTransport, ErrorMessage: "transport", CreatedAt: now, ResponseTimeMs: 200},
		{ProviderTokenId: 1, ModelName: "gpt-health", Status: 0, FailureCategory: UsageFailureCategoryInvalidResponse, InvalidReason: "no_actionable_output", CreatedAt: now, ResponseTimeMs: 300},
		{ProviderTokenId: 1, ModelName: "gpt-health", Status: 0, FailureCategory: UsageFailureCategoryInvalidResponse, InvalidReason: "stream_no_meaningful_delta", CreatedAt: now, ResponseTimeMs: 400},
	}
	for i := range rows {
		if err := DB.Create(&rows[i]).Error; err != nil {
			t.Fatalf("seed usage log failed: %v", err)
		}
	}

	lookup, err := loadRouteHealthStatsByTokenModel([]int{1}, []string{"gpt-health"}, 1)
	if err != nil {
		t.Fatalf("load health stats failed: %v", err)
	}
	stat := lookup[routeUsageKey(1, "gpt-health")]
	if stat.SampleCount != 4 || stat.ErrorCount != 3 || stat.InvalidResponseCount != 2 {
		t.Fatalf("unexpected stat counts: %+v", stat)
	}
	if stat.FailRate <= 0.74 || stat.FailRate >= 0.76 {
		t.Fatalf("unexpected fail rate: %+v", stat)
	}
	if stat.InvalidResponseRate <= 0.49 || stat.InvalidResponseRate >= 0.51 {
		t.Fatalf("unexpected invalid response rate: %+v", stat)
	}

	config := routingTuningConfig{
		HealthEnabled:       true,
		HealthMinSamples:    1,
		FailurePenaltyAlpha: 4,
		HealthRewardBeta:    0.08,
		HealthMinMultiplier: 0.05,
		HealthMaxMultiplier: 1.12,
	}
	finalized := finalizeRouteHealthStat(stat, config)
	if finalized.Multiplier >= 1 {
		t.Fatalf("expected degraded multiplier under mixed failures, got %+v", finalized)
	}
}

func TestInvalidResponseSuppressionCooldownAndRecovery(t *testing.T) {
	originDB := DB
	DB = prepareRouteInvalidResponseTestDB(t)
	defer func() { DB = originDB }()

	now := time.Now().Unix()
	activeRows := []UsageLog{
		{ProviderTokenId: 2, ModelName: "gpt-suppress", Status: 0, FailureCategory: UsageFailureCategoryInvalidResponse, InvalidReason: "no_actionable_output", CreatedAt: now},
		{ProviderTokenId: 2, ModelName: "gpt-suppress", Status: 0, FailureCategory: UsageFailureCategoryInvalidResponse, InvalidReason: "no_actionable_output", CreatedAt: now},
	}
	for i := range activeRows {
		if err := DB.Create(&activeRows[i]).Error; err != nil {
			t.Fatalf("seed active suppression log failed: %v", err)
		}
	}

	configActive := routingTuningConfig{
		InvalidResponseSuppressionEnabled: true,
		InvalidResponseThreshold:          2,
		InvalidResponseWindowMinutes:      10,
		InvalidResponseCooldownMinutes:    5,
	}
	active, err := loadRouteInvalidResponseSuppressionByTokenModel([]int{2}, []string{"gpt-suppress"}, configActive)
	if err != nil {
		t.Fatalf("load active suppression failed: %v", err)
	}
	if active[routeUsageKey(2, "gpt-suppress")] <= now {
		t.Fatalf("expected active suppression with future cooldown, got %+v", active)
	}

	recoveredRows := []UsageLog{
		{ProviderTokenId: 3, ModelName: "gpt-recover", Status: 0, FailureCategory: UsageFailureCategoryInvalidResponse, InvalidReason: "no_actionable_output", CreatedAt: now - 300},
		{ProviderTokenId: 3, ModelName: "gpt-recover", Status: 0, FailureCategory: UsageFailureCategoryInvalidResponse, InvalidReason: "no_actionable_output", CreatedAt: now - 300},
		{ProviderTokenId: 3, ModelName: "gpt-recover", Status: 1, ErrorMessage: "", CreatedAt: now - 10},
		{ProviderTokenId: 3, ModelName: "gpt-recover", Status: 1, ErrorMessage: "", CreatedAt: now - 9},
		{ProviderTokenId: 3, ModelName: "gpt-recover", Status: 1, ErrorMessage: "", CreatedAt: now - 8},
	}
	for i := range recoveredRows {
		if err := DB.Create(&recoveredRows[i]).Error; err != nil {
			t.Fatalf("seed recovery log failed: %v", err)
		}
	}

	configRecovered := routingTuningConfig{
		InvalidResponseSuppressionEnabled: true,
		InvalidResponseThreshold:          2,
		InvalidResponseWindowMinutes:      10,
		InvalidResponseCooldownMinutes:    1,
	}
	recovered, err := loadRouteInvalidResponseSuppressionByTokenModel([]int{3}, []string{"gpt-recover"}, configRecovered)
	if err != nil {
		t.Fatalf("load recovery suppression failed: %v", err)
	}
	if _, ok := recovered[routeUsageKey(3, "gpt-recover")]; ok {
		t.Fatalf("expected route to recover after cooldown, got %+v", recovered)
	}
}
