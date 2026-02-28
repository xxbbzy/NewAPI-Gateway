package model

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func prepareUsageLogAggregationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:usage_log_aggregation_%s_%d?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"), time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db failed: %v", err)
	}
	if err := db.AutoMigrate(&UsageLog{}, &Provider{}, &ModelRoute{}); err != nil {
		t.Fatalf("migrate test db failed: %v", err)
	}
	return db
}

func TestQueryUsageLogsRequestAggregationPrefersSuccessfulAttempt(t *testing.T) {
	originDB := DB
	DB = prepareUsageLogAggregationTestDB(t)
	defer func() { DB = originDB }()

	rows := []*UsageLog{
		{RelayRequestId: "r-1", AttemptIndex: 1, RequestId: "a-1", ProviderName: "p1", ModelName: "gpt", Status: 0, ErrorMessage: "upstream failed", PromptTokens: 0, CompletionTokens: 0, CreatedAt: 100},
		{RelayRequestId: "r-1", AttemptIndex: 2, RequestId: "a-2", ProviderName: "p2", ModelName: "gpt", Status: 1, ErrorMessage: "", PromptTokens: 12, CompletionTokens: 3, CreatedAt: 101},
		{RelayRequestId: "r-2", AttemptIndex: 1, RequestId: "b-1", ProviderName: "p3", ModelName: "glm", Status: 0, ErrorMessage: "fail-1", PromptTokens: 1, CompletionTokens: 0, CreatedAt: 102},
		{RelayRequestId: "r-2", AttemptIndex: 2, RequestId: "b-2", ProviderName: "p4", ModelName: "glm", Status: 0, ErrorMessage: "fail-2", PromptTokens: 2, CompletionTokens: 0, CreatedAt: 103},
	}
	for _, row := range rows {
		if err := DB.Create(row).Error; err != nil {
			t.Fatalf("seed usage log failed: %v", err)
		}
	}

	requestLogs, total, err := QueryUsageLogs(UsageLogQuery{Aggregation: UsageAggregationRequest, Offset: 0, Limit: 20})
	if err != nil {
		t.Fatalf("query request aggregation failed: %v", err)
	}
	if total != 2 || len(requestLogs) != 2 {
		t.Fatalf("expected 2 request-level rows, got total=%d len=%d", total, len(requestLogs))
	}

	var group1 *UsageLog
	var group2 *UsageLog
	for _, log := range requestLogs {
		switch log.RelayRequestId {
		case "r-1":
			group1 = log
		case "r-2":
			group2 = log
		}
	}
	if group1 == nil || group2 == nil {
		t.Fatalf("missing grouped rows: group1=%v group2=%v", group1 != nil, group2 != nil)
	}
	if group1.RequestId != "a-2" || group1.Status != 1 || group1.PromptTokens != 12 || group1.CompletionTokens != 3 {
		t.Fatalf("expected successful attempt selected for r-1, got %+v", *group1)
	}
	if group2.RequestId != "b-2" || group2.Status != 0 {
		t.Fatalf("expected latest failed attempt selected for r-2, got %+v", *group2)
	}

	attemptLogs, attemptTotal, err := QueryUsageLogs(UsageLogQuery{Aggregation: UsageAggregationAttempt, Offset: 0, Limit: 20})
	if err != nil {
		t.Fatalf("query attempt aggregation failed: %v", err)
	}
	if attemptTotal != 4 || len(attemptLogs) != 4 {
		t.Fatalf("expected 4 attempt rows, got total=%d len=%d", attemptTotal, len(attemptLogs))
	}
}

func TestQueryUsageLogSummaryRequestAggregationUsesCollapsedRows(t *testing.T) {
	originDB := DB
	DB = prepareUsageLogAggregationTestDB(t)
	defer func() { DB = originDB }()

	rows := []*UsageLog{
		{RelayRequestId: "r-1", AttemptIndex: 1, RequestId: "a-1", ProviderName: "p1", ModelName: "gpt", Status: 0, ErrorMessage: "upstream failed", PromptTokens: 0, CompletionTokens: 0, CostUSD: 0, ResponseTimeMs: 100, CreatedAt: 100},
		{RelayRequestId: "r-1", AttemptIndex: 2, RequestId: "a-2", ProviderName: "p2", ModelName: "gpt", Status: 1, ErrorMessage: "", PromptTokens: 20, CompletionTokens: 5, CostUSD: 0.1, ResponseTimeMs: 120, CreatedAt: 101},
		{RelayRequestId: "r-2", AttemptIndex: 1, RequestId: "b-1", ProviderName: "p3", ModelName: "glm", Status: 0, ErrorMessage: "fail", PromptTokens: 2, CompletionTokens: 0, CostUSD: 0.01, ResponseTimeMs: 200, CreatedAt: 102},
	}
	for _, row := range rows {
		if err := DB.Create(row).Error; err != nil {
			t.Fatalf("seed usage log failed: %v", err)
		}
	}

	summary, err := QueryUsageLogSummary(UsageLogQuery{Aggregation: UsageAggregationRequest})
	if err != nil {
		t.Fatalf("request summary failed: %v", err)
	}
	if summary.Total != 2 || summary.SuccessCount != 1 || summary.ErrorCount != 1 {
		t.Fatalf("unexpected request summary counts: %+v", summary)
	}
	if summary.InputTokens != 22 || summary.OutputTokens != 5 {
		t.Fatalf("unexpected token summary: %+v", summary)
	}

	attemptSummary, err := QueryUsageLogSummary(UsageLogQuery{Aggregation: UsageAggregationAttempt})
	if err != nil {
		t.Fatalf("attempt summary failed: %v", err)
	}
	if attemptSummary.Total != 3 {
		t.Fatalf("expected attempt summary total=3, got %+v", attemptSummary)
	}
	if attemptSummary.InvalidResponseCount != 0 || attemptSummary.InvalidResponseRate != 0 {
		t.Fatalf("expected no invalid-response stats in this fixture, got %+v", attemptSummary)
	}
}

func TestRequestAggregationRepresentativeDeterministicWithFailureMetadata(t *testing.T) {
	originDB := DB
	DB = prepareUsageLogAggregationTestDB(t)
	defer func() { DB = originDB }()

	rows := []*UsageLog{
		{
			RelayRequestId: "relay-1", AttemptIndex: 1, RequestId: "req-1a", ProviderName: "p1", ModelName: "gpt",
			Status: 0, ErrorMessage: "invalid response", FailureCategory: UsageFailureCategoryInvalidResponse, InvalidReason: "no_actionable_output",
			RequestModelOriginal: "alias-gpt", RequestModelCanonical: "gpt", RequestModelResolved: "gpt-2026",
			CreatedAt: 100,
		},
		{
			RelayRequestId: "relay-1", AttemptIndex: 2, RequestId: "req-1b", ProviderName: "p2", ModelName: "gpt",
			Status: 1, ErrorMessage: "", FailureCategory: "", InvalidReason: "",
			RequestModelOriginal: "alias-gpt", RequestModelCanonical: "gpt", RequestModelResolved: "gpt-2026",
			CreatedAt: 101,
		},
	}
	for _, row := range rows {
		if err := DB.Create(row).Error; err != nil {
			t.Fatalf("seed usage log failed: %v", err)
		}
	}

	requestLogs, total, err := QueryUsageLogs(UsageLogQuery{Aggregation: UsageAggregationRequest, Offset: 0, Limit: 10})
	if err != nil {
		t.Fatalf("query request aggregation failed: %v", err)
	}
	if total != 1 || len(requestLogs) != 1 {
		t.Fatalf("expected one aggregated request row, got total=%d len=%d", total, len(requestLogs))
	}
	selected := requestLogs[0]
	if selected.RequestId != "req-1b" || selected.Status != 1 {
		t.Fatalf("expected successful attempt selected as representative, got %+v", *selected)
	}
	if selected.RequestModelOriginal != "alias-gpt" || selected.RequestModelCanonical != "gpt" || selected.RequestModelResolved != "gpt-2026" {
		t.Fatalf("expected model identity metadata preserved, got %+v", *selected)
	}
}

func TestSummaryAndDashboardExposeInvalidResponseMetrics(t *testing.T) {
	originDB := DB
	DB = prepareUsageLogAggregationTestDB(t)
	defer func() { DB = originDB }()

	rows := []*UsageLog{
		{RelayRequestId: "r-1", AttemptIndex: 1, RequestId: "a-1", ProviderName: "p1", ProviderId: 1, ModelName: "gpt", Status: 0, FailureCategory: UsageFailureCategoryInvalidResponse, InvalidReason: "no_actionable_output", CreatedAt: 100},
		{RelayRequestId: "r-1", AttemptIndex: 2, RequestId: "a-2", ProviderName: "p1", ProviderId: 1, ModelName: "gpt", Status: 1, CreatedAt: 101},
		{RelayRequestId: "r-2", AttemptIndex: 1, RequestId: "b-1", ProviderName: "p2", ProviderId: 2, ModelName: "glm", Status: 0, ErrorMessage: "transport fail", FailureCategory: UsageFailureCategoryTransport, CreatedAt: 102},
	}
	for _, row := range rows {
		if err := DB.Create(row).Error; err != nil {
			t.Fatalf("seed usage log failed: %v", err)
		}
	}

	attemptSummary, err := QueryUsageLogSummary(UsageLogQuery{Aggregation: UsageAggregationAttempt})
	if err != nil {
		t.Fatalf("attempt summary failed: %v", err)
	}
	if attemptSummary.Total != 3 || attemptSummary.InvalidResponseCount != 1 {
		t.Fatalf("unexpected invalid-response summary counts: %+v", attemptSummary)
	}
	if attemptSummary.InvalidResponseRate <= 0.3 || attemptSummary.InvalidResponseRate >= 0.34 {
		t.Fatalf("unexpected invalid-response rate: %+v", attemptSummary)
	}

	statsAttempt, err := GetDashboardStatsWithAggregation(UsageAggregationAttempt)
	if err != nil {
		t.Fatalf("attempt dashboard stats failed: %v", err)
	}
	if statsAttempt.InvalidResponses != 1 {
		t.Fatalf("expected dashboard invalid responses=1, got %+v", statsAttempt)
	}
	if statsAttempt.InvalidRate <= 0.3 || statsAttempt.InvalidRate >= 0.34 {
		t.Fatalf("unexpected dashboard invalid rate: %+v", statsAttempt)
	}
}

func TestGetDashboardStatsWithAggregationSupportsLegacyRows(t *testing.T) {
	originDB := DB
	DB = prepareUsageLogAggregationTestDB(t)
	defer func() { DB = originDB }()

	legacyRows := []*UsageLog{
		{RequestId: "legacy-a", RelayRequestId: "", AttemptIndex: 0, ProviderName: "p1", ProviderId: 1, ModelName: "gpt-4o", Status: 1, PromptTokens: 10, CompletionTokens: 4, ResponseTimeMs: 50, CreatedAt: time.Now().Unix()},
		{RequestId: "legacy-b", RelayRequestId: "", AttemptIndex: 0, ProviderName: "p2", ProviderId: 2, ModelName: "gpt-4o-mini", Status: 0, ErrorMessage: "fail", PromptTokens: 1, CompletionTokens: 0, ResponseTimeMs: 70, CreatedAt: time.Now().Unix()},
	}
	for _, row := range legacyRows {
		if err := DB.Create(row).Error; err != nil {
			t.Fatalf("seed legacy usage log failed: %v", err)
		}
	}

	statsRequest, err := GetDashboardStatsWithAggregation(UsageAggregationRequest)
	if err != nil {
		t.Fatalf("request dashboard stats failed: %v", err)
	}
	if statsRequest.TotalRequests != 2 {
		t.Fatalf("expected request dashboard total=2, got %d", statsRequest.TotalRequests)
	}

	statsAttempt, err := GetDashboardStatsWithAggregation(UsageAggregationAttempt)
	if err != nil {
		t.Fatalf("attempt dashboard stats failed: %v", err)
	}
	if statsAttempt.TotalRequests != 2 {
		t.Fatalf("expected attempt dashboard total=2, got %d", statsAttempt.TotalRequests)
	}
}
