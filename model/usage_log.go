package model

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	UsageAggregationRequest = "request"
	UsageAggregationAttempt = "attempt"

	UsageSourceExact     = "exact"
	UsageSourceEstimated = "estimated"
	UsageSourceMissing   = "missing"

	UsageFailureCategoryTransport       = "transport_error"
	UsageFailureCategoryUpstream        = "upstream_error"
	UsageFailureCategoryInvalidResponse = "invalid_response"
	UsageFailureCategoryReadError       = "read_error"
	UsageFailureCategoryParseError      = "parse_error"
)

type UsageLog struct {
	Id                    int64   `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId                int     `json:"user_id" gorm:"index"`
	AggregatedTokenId     int     `json:"aggregated_token_id"`
	ProviderId            int     `json:"provider_id" gorm:"index"`
	ProviderName          string  `json:"provider_name" gorm:"type:varchar(128)"`
	ProviderTokenId       int     `json:"provider_token_id" gorm:"index;index:idx_usage_logs_route_window,priority:1;index:idx_usage_logs_invalid_window,priority:1"`
	ModelName             string  `json:"model_name" gorm:"type:varchar(255);index;index:idx_usage_logs_route_window,priority:2;index:idx_usage_logs_invalid_window,priority:2"`
	PromptTokens          int     `json:"prompt_tokens"`
	CompletionTokens      int     `json:"completion_tokens"`
	CacheTokens           int     `json:"cache_tokens"`
	CacheCreationTokens   int     `json:"cache_creation_tokens"`
	CacheCreation5mTokens int     `json:"cache_creation_5m_tokens"`
	CacheCreation1hTokens int     `json:"cache_creation_1h_tokens"`
	ResponseTimeMs        int     `json:"response_time_ms"`
	FirstTokenMs          int     `json:"first_token_ms"`
	IsStream              bool    `json:"is_stream"`
	CostUSD               float64 `json:"cost_usd"`
	Status                int     `json:"status"`
	ErrorMessage          string  `json:"error_message" gorm:"type:text"`
	FailureCategory       string  `json:"failure_category" gorm:"type:varchar(32);index"`
	InvalidReason         string  `json:"invalid_reason" gorm:"type:varchar(64);index"`
	TransportStatusCode   int     `json:"transport_status_code"`
	ClientIp              string  `json:"client_ip" gorm:"type:varchar(64)"`
	RequestId             string  `json:"request_id" gorm:"type:varchar(64);index"`
	RelayRequestId        string  `json:"relay_request_id" gorm:"type:varchar(64);index;index:idx_usage_logs_relay_attempt,priority:1"`
	AttemptIndex          int     `json:"attempt_index" gorm:"index;index:idx_usage_logs_relay_attempt,priority:2"`
	RequestModelOriginal  string  `json:"request_model_original" gorm:"type:varchar(255)"`
	RequestModelCanonical string  `json:"request_model_canonical" gorm:"type:varchar(255)"`
	RequestModelResolved  string  `json:"request_model_resolved" gorm:"type:varchar(255)"`
	UsageSource           string  `json:"usage_source" gorm:"type:varchar(16);index"`
	UsageParser           string  `json:"usage_parser" gorm:"type:varchar(64)"`
	CreatedAt             int64   `json:"created_at" gorm:"index;index:idx_usage_logs_route_window,priority:3;index:idx_usage_logs_invalid_window,priority:3"`
}

func (l *UsageLog) Insert() error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}
	if strings.TrimSpace(l.UsageSource) == "" {
		l.UsageSource = UsageSourceMissing
	}
	if strings.TrimSpace(l.UsageParser) == "" {
		l.UsageParser = "none"
	}
	l.CreatedAt = time.Now().Unix()
	return DB.Model(&UsageLog{}).Create(map[string]interface{}{
		"user_id":                 l.UserId,
		"aggregated_token_id":     l.AggregatedTokenId,
		"provider_id":             l.ProviderId,
		"provider_name":           l.ProviderName,
		"provider_token_id":       l.ProviderTokenId,
		"model_name":              l.ModelName,
		"prompt_tokens":           l.PromptTokens,
		"completion_tokens":       l.CompletionTokens,
		"cache_tokens":            l.CacheTokens,
		"cache_creation_tokens":   l.CacheCreationTokens,
		"cache_creation5m_tokens": l.CacheCreation5mTokens,
		"cache_creation1h_tokens": l.CacheCreation1hTokens,
		"response_time_ms":        l.ResponseTimeMs,
		"first_token_ms":          l.FirstTokenMs,
		"is_stream":               l.IsStream,
		"cost_usd":                l.CostUSD,
		"status":                  l.Status,
		"error_message":           l.ErrorMessage,
		"failure_category":        l.FailureCategory,
		"invalid_reason":          l.InvalidReason,
		"transport_status_code":   l.TransportStatusCode,
		"client_ip":               l.ClientIp,
		"request_id":              l.RequestId,
		"relay_request_id":        l.RelayRequestId,
		"attempt_index":           l.AttemptIndex,
		"request_model_original":  l.RequestModelOriginal,
		"request_model_canonical": l.RequestModelCanonical,
		"request_model_resolved":  l.RequestModelResolved,
		"usage_source":            l.UsageSource,
		"usage_parser":            l.UsageParser,
		"created_at":              l.CreatedAt,
	}).Error
}

type UsageLogQuery struct {
	UserID       *int
	Offset       int
	Limit        int
	Keyword      string
	ProviderName string
	Status       string
	ViewTab      string
	Aggregation  string
}

type UsageLogSummary struct {
	Total                int64   `json:"total"`
	SuccessCount         int64   `json:"success_count"`
	ErrorCount           int64   `json:"error_count"`
	InvalidResponseCount int64   `json:"invalid_response_count"`
	InvalidResponseRate  float64 `json:"invalid_response_rate"`
	InputTokens          int64   `json:"input_tokens"`
	OutputTokens         int64   `json:"output_tokens"`
	CacheTokens          int64   `json:"cache_tokens"`
	TotalCost            float64 `json:"total_cost"`
	AvgLatency           int64   `json:"avg_latency"`
}

func normalizeUsageAggregation(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case UsageAggregationAttempt:
		return UsageAggregationAttempt
	default:
		return UsageAggregationRequest
	}
}

func isUsageLogSuccess(log *UsageLog) bool {
	if log == nil {
		return false
	}
	return log.Status == 1 && strings.TrimSpace(log.ErrorMessage) == ""
}

func isUsageLogInvalidResponse(log *UsageLog) bool {
	if log == nil {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(log.FailureCategory), UsageFailureCategoryInvalidResponse) {
		return true
	}
	return strings.TrimSpace(log.InvalidReason) != ""
}

func usageLogInvalidResponseSQLCondition() string {
	return "(failure_category = '" + UsageFailureCategoryInvalidResponse + "' OR (invalid_reason IS NOT NULL AND TRIM(invalid_reason) <> ''))"
}

func applyUsageLogCommonFilters(db *gorm.DB, query UsageLogQuery) *gorm.DB {
	if query.UserID != nil {
		db = db.Where("user_id = ?", *query.UserID)
	}
	if providerName := strings.TrimSpace(query.ProviderName); providerName != "" {
		db = db.Where("provider_name = ?", providerName)
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where(
			"(model_name LIKE ? OR provider_name LIKE ? OR request_id LIKE ? OR relay_request_id LIKE ? OR usage_source LIKE ? OR error_message LIKE ? OR client_ip LIKE ? OR failure_category LIKE ? OR invalid_reason LIKE ? OR request_model_original LIKE ? OR request_model_canonical LIKE ? OR request_model_resolved LIKE ?)",
			like, like, like, like, like, like, like, like, like, like, like, like,
		)
	}
	return db
}

func applyUsageLogStatusFilters(db *gorm.DB, query UsageLogQuery) *gorm.DB {
	isErrorCondition := "(status <> 1 OR (error_message IS NOT NULL AND TRIM(error_message) <> ''))"
	isSuccessCondition := "(status = 1 AND (error_message IS NULL OR TRIM(error_message) = ''))"
	if query.ViewTab == "error" {
		db = db.Where(isErrorCondition)
	}
	switch query.Status {
	case "success":
		db = db.Where(isSuccessCondition)
	case "error":
		db = db.Where(isErrorCondition)
	}
	return db
}

func applyUsageLogFilters(db *gorm.DB, query UsageLogQuery) *gorm.DB {
	return applyUsageLogStatusFilters(applyUsageLogCommonFilters(db, query), query)
}

func matchesUsageLogStatus(log *UsageLog, query UsageLogQuery) bool {
	if log == nil {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(query.ViewTab), "error") && isUsageLogSuccess(log) {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(query.Status)) {
	case "success":
		return isUsageLogSuccess(log)
	case "error":
		return !isUsageLogSuccess(log)
	default:
		return true
	}
}

func usageLogGroupKey(log *UsageLog) string {
	if log == nil {
		return ""
	}
	if relay := strings.TrimSpace(log.RelayRequestId); relay != "" {
		return "relay:" + relay
	}
	if request := strings.TrimSpace(log.RequestId); request != "" {
		return "request:" + request
	}
	return fmt.Sprintf("id:%d", log.Id)
}

func derivedRelayRequestID(log *UsageLog) string {
	if log == nil {
		return ""
	}
	if relay := strings.TrimSpace(log.RelayRequestId); relay != "" {
		return relay
	}
	if request := strings.TrimSpace(log.RequestId); request != "" {
		return "legacy-" + request
	}
	return fmt.Sprintf("legacy-id-%d", log.Id)
}

func compareUsageLogAttemptOrder(a *UsageLog, b *UsageLog) int {
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return -1
	}
	if b == nil {
		return 1
	}
	if a.AttemptIndex != b.AttemptIndex {
		if a.AttemptIndex > b.AttemptIndex {
			return 1
		}
		return -1
	}
	if a.CreatedAt != b.CreatedAt {
		if a.CreatedAt > b.CreatedAt {
			return 1
		}
		return -1
	}
	if a.Id > b.Id {
		return 1
	}
	if a.Id < b.Id {
		return -1
	}
	return 0
}

func selectRequestRepresentative(attempts []*UsageLog) *UsageLog {
	var bestSuccess *UsageLog
	var bestAny *UsageLog
	for _, attempt := range attempts {
		if compareUsageLogAttemptOrder(attempt, bestAny) > 0 {
			bestAny = attempt
		}
		if isUsageLogSuccess(attempt) && compareUsageLogAttemptOrder(attempt, bestSuccess) > 0 {
			bestSuccess = attempt
		}
	}
	if bestSuccess != nil {
		return bestSuccess
	}
	return bestAny
}

func collapseUsageLogsByRequest(attempts []*UsageLog) []*UsageLog {
	if len(attempts) == 0 {
		return []*UsageLog{}
	}
	groups := make(map[string][]*UsageLog, len(attempts))
	order := make([]string, 0, len(attempts))
	for _, attempt := range attempts {
		key := usageLogGroupKey(attempt)
		if key == "" {
			continue
		}
		if _, exists := groups[key]; !exists {
			order = append(order, key)
		}
		groups[key] = append(groups[key], attempt)
	}

	collapsed := make([]*UsageLog, 0, len(groups))
	for _, key := range order {
		selected := selectRequestRepresentative(groups[key])
		if selected == nil {
			continue
		}
		copyLog := *selected
		if strings.TrimSpace(copyLog.RelayRequestId) == "" {
			copyLog.RelayRequestId = derivedRelayRequestID(selected)
		}
		collapsed = append(collapsed, &copyLog)
	}
	sort.Slice(collapsed, func(i, j int) bool {
		if collapsed[i].Id == collapsed[j].Id {
			return collapsed[i].CreatedAt > collapsed[j].CreatedAt
		}
		return collapsed[i].Id > collapsed[j].Id
	})
	return collapsed
}

func filterUsageLogsByStatus(logs []*UsageLog, query UsageLogQuery) []*UsageLog {
	if len(logs) == 0 {
		return []*UsageLog{}
	}
	filtered := make([]*UsageLog, 0, len(logs))
	for _, log := range logs {
		if matchesUsageLogStatus(log, query) {
			filtered = append(filtered, log)
		}
	}
	return filtered
}

func paginateUsageLogs(logs []*UsageLog, offset int, limit int) []*UsageLog {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 15
	}
	if offset >= len(logs) {
		return []*UsageLog{}
	}
	end := offset + limit
	if end > len(logs) {
		end = len(logs)
	}
	return logs[offset:end]
}

func queryRequestAggregatedLogs(query UsageLogQuery) ([]*UsageLog, error) {
	baseQuery := applyUsageLogCommonFilters(DB.Model(&UsageLog{}), query)
	var attempts []*UsageLog
	if err := baseQuery.Order("id desc").Find(&attempts).Error; err != nil {
		return nil, err
	}
	collapsed := collapseUsageLogsByRequest(attempts)
	return filterUsageLogsByStatus(collapsed, query), nil
}

func QueryUsageLogs(query UsageLogQuery) ([]*UsageLog, int64, error) {
	if query.Limit <= 0 {
		query.Limit = 15
	}
	if query.Offset < 0 {
		query.Offset = 0
	}
	query.Aggregation = normalizeUsageAggregation(query.Aggregation)

	if query.Aggregation == UsageAggregationAttempt {
		baseQuery := applyUsageLogFilters(DB.Model(&UsageLog{}), query)

		var total int64
		if err := baseQuery.Count(&total).Error; err != nil {
			return nil, 0, err
		}

		var logs []*UsageLog
		err := baseQuery.Order("id desc").Limit(query.Limit).Offset(query.Offset).Find(&logs).Error
		return logs, total, err
	}

	logs, err := queryRequestAggregatedLogs(query)
	if err != nil {
		return nil, 0, err
	}
	total := int64(len(logs))
	return paginateUsageLogs(logs, query.Offset, query.Limit), total, nil
}

func QueryUsageLogProviders(query UsageLogQuery) ([]string, error) {
	// Provider options should not collapse to the currently selected provider.
	providerQuery := query
	providerQuery.ProviderName = ""
	providerQuery.Aggregation = normalizeUsageAggregation(providerQuery.Aggregation)

	if providerQuery.Aggregation == UsageAggregationAttempt {
		var providers []string
		err := applyUsageLogFilters(DB.Model(&UsageLog{}), providerQuery).
			Where("provider_name IS NOT NULL AND TRIM(provider_name) <> ''").
			Distinct("provider_name").
			Order("provider_name asc").
			Pluck("provider_name", &providers).Error
		return providers, err
	}

	logs, err := queryRequestAggregatedLogs(providerQuery)
	if err != nil {
		return nil, err
	}
	providerSet := make(map[string]struct{})
	for _, log := range logs {
		provider := strings.TrimSpace(log.ProviderName)
		if provider == "" {
			continue
		}
		providerSet[provider] = struct{}{}
	}
	providers := make([]string, 0, len(providerSet))
	for provider := range providerSet {
		providers = append(providers, provider)
	}
	sort.Strings(providers)
	return providers, nil
}

func summarizeUsageLogs(logs []*UsageLog) UsageLogSummary {
	if len(logs) == 0 {
		return UsageLogSummary{}
	}
	var successCount int64
	var invalidResponseCount int64
	var inputTokens int64
	var outputTokens int64
	var cacheTokens int64
	var totalCost float64
	var latencySum int64

	for _, log := range logs {
		if isUsageLogSuccess(log) {
			successCount++
		}
		if isUsageLogInvalidResponse(log) {
			invalidResponseCount++
		}
		inputTokens += int64(log.PromptTokens)
		outputTokens += int64(log.CompletionTokens)
		cacheTokens += int64(log.CacheTokens)
		totalCost += log.CostUSD
		latencySum += int64(log.ResponseTimeMs)
	}

	total := int64(len(logs))
	avgLatency := int64(0)
	invalidRate := 0.0
	if total > 0 {
		avgLatency = int64(math.Round(float64(latencySum) / float64(total)))
		invalidRate = float64(invalidResponseCount) / float64(total)
	}

	return UsageLogSummary{
		Total:                total,
		SuccessCount:         successCount,
		ErrorCount:           total - successCount,
		InvalidResponseCount: invalidResponseCount,
		InvalidResponseRate:  invalidRate,
		InputTokens:          inputTokens,
		OutputTokens:         outputTokens,
		CacheTokens:          cacheTokens,
		TotalCost:            totalCost,
		AvgLatency:           avgLatency,
	}
}

func QueryUsageLogSummary(query UsageLogQuery) (UsageLogSummary, error) {
	query.Aggregation = normalizeUsageAggregation(query.Aggregation)
	if query.Aggregation == UsageAggregationAttempt {
		isErrorCondition := "(status <> 1 OR (error_message IS NOT NULL AND TRIM(error_message) <> ''))"
		isSuccessCondition := "(status = 1 AND (error_message IS NULL OR TRIM(error_message) = ''))"

		type usageLogSummaryRaw struct {
			Total                int64
			SuccessCount         int64
			ErrorCount           int64
			InvalidResponseCount int64
			InputTokens          int64
			OutputTokens         int64
			CacheTokens          int64
			TotalCost            float64
			AvgLatency           float64
		}

		var raw usageLogSummaryRaw
		invalidCondition := usageLogInvalidResponseSQLCondition()
		err := applyUsageLogFilters(DB.Model(&UsageLog{}), query).
			Select(
				"COUNT(*) AS total",
				"SUM(CASE WHEN "+isSuccessCondition+" THEN 1 ELSE 0 END) AS success_count",
				"SUM(CASE WHEN "+isErrorCondition+" THEN 1 ELSE 0 END) AS error_count",
				"SUM(CASE WHEN "+invalidCondition+" THEN 1 ELSE 0 END) AS invalid_response_count",
				"COALESCE(SUM(prompt_tokens), 0) AS input_tokens",
				"COALESCE(SUM(completion_tokens), 0) AS output_tokens",
				"COALESCE(SUM(cache_tokens), 0) AS cache_tokens",
				"COALESCE(SUM(cost_usd), 0) AS total_cost",
				"COALESCE(AVG(response_time_ms), 0) AS avg_latency",
			).
			Scan(&raw).Error
		if err != nil {
			return UsageLogSummary{}, err
		}

		invalidRate := 0.0
		if raw.Total > 0 {
			invalidRate = float64(raw.InvalidResponseCount) / float64(raw.Total)
		}

		return UsageLogSummary{
			Total:                raw.Total,
			SuccessCount:         raw.SuccessCount,
			ErrorCount:           raw.ErrorCount,
			InvalidResponseCount: raw.InvalidResponseCount,
			InvalidResponseRate:  invalidRate,
			InputTokens:          raw.InputTokens,
			OutputTokens:         raw.OutputTokens,
			CacheTokens:          raw.CacheTokens,
			TotalCost:            raw.TotalCost,
			AvgLatency:           int64(math.Round(raw.AvgLatency)),
		}, nil
	}

	logs, err := queryRequestAggregatedLogs(query)
	if err != nil {
		return UsageLogSummary{}, err
	}
	return summarizeUsageLogs(logs), nil
}

// GetUserLogs returns logs for a specific user
func GetUserLogs(userId int, startIdx int, num int) ([]*UsageLog, error) {
	logQuery := UsageLogQuery{
		UserID:      &userId,
		Offset:      startIdx,
		Limit:       num,
		Aggregation: UsageAggregationRequest,
	}
	logs, _, err := QueryUsageLogs(logQuery)
	return logs, err
}

// GetAllLogs returns all logs (admin)
func GetAllLogs(startIdx int, num int) ([]*UsageLog, error) {
	logQuery := UsageLogQuery{
		Offset:      startIdx,
		Limit:       num,
		Aggregation: UsageAggregationRequest,
	}
	logs, _, err := QueryUsageLogs(logQuery)
	return logs, err
}

// CountUserLogs counts total logs for a user
func CountUserLogs(userId int) int64 {
	var count int64
	DB.Model(&UsageLog{}).Where("user_id = ?", userId).Count(&count)
	return count
}

// CountAllLogs counts total logs
func CountAllLogs() int64 {
	var count int64
	DB.Model(&UsageLog{}).Count(&count)
	return count
}

// DashboardStats holds aggregated statistics
type DashboardStats struct {
	TotalRequests    int64               `json:"total_requests"`
	SuccessRequests  int64               `json:"success_requests"`
	FailedRequests   int64               `json:"failed_requests"`
	InvalidResponses int64               `json:"invalid_responses"`
	InvalidRate      float64             `json:"invalid_rate"`
	TotalProviders   int64               `json:"total_providers"`
	TotalModels      int64               `json:"total_models"`
	TotalRoutes      int64               `json:"total_routes"`
	ByProvider       []ProviderStat      `json:"by_provider"`
	ByModel          []ModelStat         `json:"by_model"`
	RecentRequests   []DailyRequestCount `json:"recent_requests"`
	RecentMetrics    []DailyTrendStat    `json:"recent_metrics"`
	RecentModelStats []DailyModelStat    `json:"recent_model_stats"`
}

type ProviderStat struct {
	ProviderId   int    `json:"provider_id"`
	ProviderName string `json:"provider_name"`
	RequestCount int64  `json:"request_count"`
}

type ModelStat struct {
	ModelName    string `json:"model_name"`
	RequestCount int64  `json:"request_count"`
}

type DailyRequestCount struct {
	Date         string `json:"date"`
	RequestCount int64  `json:"request_count"`
}

type DailyTrendStat struct {
	Date         string  `json:"date"`
	RequestCount int64   `json:"request_count"`
	CostUSD      float64 `json:"cost_usd"`
	TokenCount   int64   `json:"token_count"`
}

type DailyModelStat struct {
	Date       string `json:"date"`
	ModelName  string `json:"model_name"`
	TokenCount int64  `json:"token_count"`
}

// GetDashboardStats returns aggregated stats for the admin dashboard.
func GetDashboardStats() (*DashboardStats, error) {
	return GetDashboardStatsWithAggregation(UsageAggregationRequest)
}

func GetDashboardStatsWithAggregation(aggregation string) (*DashboardStats, error) {
	if normalizeUsageAggregation(aggregation) == UsageAggregationAttempt {
		return getDashboardStatsAttempt()
	}
	return getDashboardStatsRequest()
}

func getDashboardStatsAttempt() (*DashboardStats, error) {
	stats := &DashboardStats{}

	// Total counts
	DB.Model(&UsageLog{}).Count(&stats.TotalRequests)
	DB.Model(&UsageLog{}).Where("status = 1 AND (error_message = '' OR error_message IS NULL)").Count(&stats.SuccessRequests)
	stats.FailedRequests = stats.TotalRequests - stats.SuccessRequests
	DB.Model(&UsageLog{}).Where(usageLogInvalidResponseSQLCondition()).Count(&stats.InvalidResponses)
	if stats.TotalRequests > 0 {
		stats.InvalidRate = float64(stats.InvalidResponses) / float64(stats.TotalRequests)
	}
	stats.TotalProviders = CountProviders()
	stats.TotalRoutes = CountModelRoutes()

	models, _ := GetDistinctModels()
	stats.TotalModels = int64(len(models))

	// By provider
	DB.Model(&UsageLog{}).Select("provider_id, provider_name, count(*) as request_count").
		Group("provider_id, provider_name").Order("request_count desc").
		Limit(10).Scan(&stats.ByProvider)

	// By model
	DB.Model(&UsageLog{}).Select("model_name, count(*) as request_count").
		Group("model_name").Order("request_count desc").
		Limit(10).Scan(&stats.ByModel)

	// Recent trends (last 7 days, including today)
	type dailyTrendRow struct {
		Date         string  `json:"date"`
		RequestCount int64   `json:"request_count"`
		CostUSD      float64 `json:"cost_usd"`
		TokenCount   int64   `json:"token_count"`
	}
	now := time.Now()
	startDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -6)
	startUnix := startDay.Unix()

	var dateExpr string
	switch DB.Dialector.Name() {
	case "mysql":
		dateExpr = "DATE_FORMAT(FROM_UNIXTIME(created_at), '%Y-%m-%d')"
	case "postgres":
		dateExpr = "TO_CHAR(TO_TIMESTAMP(created_at), 'YYYY-MM-DD')"
	default:
		dateExpr = "strftime('%Y-%m-%d', datetime(created_at, 'unixepoch', 'localtime'))"
	}

	var recentRows []dailyTrendRow
	if err := DB.Model(&UsageLog{}).
		Select(dateExpr+" AS date, COUNT(*) AS request_count, COALESCE(SUM(cost_usd), 0) AS cost_usd, COALESCE(SUM(prompt_tokens + completion_tokens), 0) AS token_count").
		Where("created_at >= ?", startUnix).
		Group(dateExpr).
		Order(dateExpr + " ASC").
		Scan(&recentRows).Error; err != nil {
		return nil, err
	}

	trendMap := make(map[string]DailyTrendStat, len(recentRows))
	for _, row := range recentRows {
		trendMap[row.Date] = DailyTrendStat{
			Date:         row.Date,
			RequestCount: row.RequestCount,
			CostUSD:      row.CostUSD,
			TokenCount:   row.TokenCount,
		}
	}

	stats.RecentRequests = make([]DailyRequestCount, 0, 7)
	stats.RecentMetrics = make([]DailyTrendStat, 0, 7)
	for i := 0; i < 7; i++ {
		day := startDay.AddDate(0, 0, i)
		dayStr := day.Format("2006-01-02")
		trend := trendMap[dayStr]
		trend.Date = dayStr

		stats.RecentMetrics = append(stats.RecentMetrics, trend)
		stats.RecentRequests = append(stats.RecentRequests, DailyRequestCount{
			Date:         dayStr,
			RequestCount: trend.RequestCount,
		})
	}

	type modelTokenTotalRow struct {
		ModelName  string `json:"model_name"`
		TokenCount int64  `json:"token_count"`
	}
	type modelDailyRow struct {
		Date       string `json:"date"`
		ModelName  string `json:"model_name"`
		TokenCount int64  `json:"token_count"`
	}

	var topModelRows []modelTokenTotalRow
	if err := DB.Model(&UsageLog{}).
		Select("model_name, COALESCE(SUM(prompt_tokens + completion_tokens), 0) AS token_count").
		Where("created_at >= ? AND model_name IS NOT NULL AND TRIM(model_name) <> ''", startUnix).
		Group("model_name").
		Order("token_count DESC").
		Limit(8).
		Scan(&topModelRows).Error; err != nil {
		return nil, err
	}

	if len(topModelRows) > 0 {
		modelNames := make([]string, 0, len(topModelRows))
		for _, row := range topModelRows {
			modelNames = append(modelNames, row.ModelName)
		}

		var modelRows []modelDailyRow
		if err := DB.Model(&UsageLog{}).
			Select(dateExpr+" AS date, model_name, COALESCE(SUM(prompt_tokens + completion_tokens), 0) AS token_count").
			Where("created_at >= ? AND model_name IN ?", startUnix, modelNames).
			Group(dateExpr + ", model_name").
			Order(dateExpr + " ASC").
			Scan(&modelRows).Error; err != nil {
			return nil, err
		}
		stats.RecentModelStats = make([]DailyModelStat, 0, len(modelRows))
		for _, row := range modelRows {
			stats.RecentModelStats = append(stats.RecentModelStats, DailyModelStat{
				Date:       row.Date,
				ModelName:  row.ModelName,
				TokenCount: row.TokenCount,
			})
		}
	} else {
		stats.RecentModelStats = []DailyModelStat{}
	}

	return stats, nil
}

func getDashboardStatsRequest() (*DashboardStats, error) {
	stats := &DashboardStats{}
	var attempts []*UsageLog
	if err := DB.Model(&UsageLog{}).Order("id desc").Find(&attempts).Error; err != nil {
		return nil, err
	}
	logs := collapseUsageLogsByRequest(attempts)

	stats.TotalRequests = int64(len(logs))
	for _, log := range logs {
		if isUsageLogSuccess(log) {
			stats.SuccessRequests++
		}
		if isUsageLogInvalidResponse(log) {
			stats.InvalidResponses++
		}
	}
	stats.FailedRequests = stats.TotalRequests - stats.SuccessRequests
	if stats.TotalRequests > 0 {
		stats.InvalidRate = float64(stats.InvalidResponses) / float64(stats.TotalRequests)
	}
	stats.TotalProviders = CountProviders()
	stats.TotalRoutes = CountModelRoutes()
	models, _ := GetDistinctModels()
	stats.TotalModels = int64(len(models))

	type providerKey struct {
		id   int
		name string
	}
	providerCount := make(map[providerKey]int64)
	modelCount := make(map[string]int64)
	for _, log := range logs {
		providerCount[providerKey{id: log.ProviderId, name: log.ProviderName}]++
		modelName := strings.TrimSpace(log.ModelName)
		if modelName != "" {
			modelCount[modelName]++
		}
	}

	providerStats := make([]ProviderStat, 0, len(providerCount))
	for key, count := range providerCount {
		providerStats = append(providerStats, ProviderStat{ProviderId: key.id, ProviderName: key.name, RequestCount: count})
	}
	sort.Slice(providerStats, func(i, j int) bool {
		if providerStats[i].RequestCount == providerStats[j].RequestCount {
			return providerStats[i].ProviderName < providerStats[j].ProviderName
		}
		return providerStats[i].RequestCount > providerStats[j].RequestCount
	})
	if len(providerStats) > 10 {
		providerStats = providerStats[:10]
	}
	stats.ByProvider = providerStats

	modelStats := make([]ModelStat, 0, len(modelCount))
	for modelName, count := range modelCount {
		modelStats = append(modelStats, ModelStat{ModelName: modelName, RequestCount: count})
	}
	sort.Slice(modelStats, func(i, j int) bool {
		if modelStats[i].RequestCount == modelStats[j].RequestCount {
			return modelStats[i].ModelName < modelStats[j].ModelName
		}
		return modelStats[i].RequestCount > modelStats[j].RequestCount
	})
	if len(modelStats) > 10 {
		modelStats = modelStats[:10]
	}
	stats.ByModel = modelStats

	now := time.Now()
	startDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -6)
	startUnix := startDay.Unix()

	type dailyAccum struct {
		requestCount int64
		costUSD      float64
		tokenCount   int64
	}
	daily := make(map[string]*dailyAccum)
	modelTokenTotals := make(map[string]int64)
	dailyModel := make(map[string]map[string]int64)

	for _, log := range logs {
		if log.CreatedAt < startUnix {
			continue
		}
		day := time.Unix(log.CreatedAt, 0).In(now.Location()).Format("2006-01-02")
		accum := daily[day]
		if accum == nil {
			accum = &dailyAccum{}
			daily[day] = accum
		}
		accum.requestCount++
		accum.costUSD += log.CostUSD
		tokens := int64(log.PromptTokens + log.CompletionTokens)
		accum.tokenCount += tokens

		modelName := strings.TrimSpace(log.ModelName)
		if modelName == "" {
			continue
		}
		modelTokenTotals[modelName] += tokens
		if _, ok := dailyModel[day]; !ok {
			dailyModel[day] = make(map[string]int64)
		}
		dailyModel[day][modelName] += tokens
	}

	stats.RecentRequests = make([]DailyRequestCount, 0, 7)
	stats.RecentMetrics = make([]DailyTrendStat, 0, 7)
	for i := 0; i < 7; i++ {
		day := startDay.AddDate(0, 0, i).Format("2006-01-02")
		accum := daily[day]
		trend := DailyTrendStat{Date: day}
		if accum != nil {
			trend.RequestCount = accum.requestCount
			trend.CostUSD = accum.costUSD
			trend.TokenCount = accum.tokenCount
		}
		stats.RecentMetrics = append(stats.RecentMetrics, trend)
		stats.RecentRequests = append(stats.RecentRequests, DailyRequestCount{Date: day, RequestCount: trend.RequestCount})
	}

	type namedTotal struct {
		name  string
		total int64
	}
	totals := make([]namedTotal, 0, len(modelTokenTotals))
	for modelName, total := range modelTokenTotals {
		totals = append(totals, namedTotal{name: modelName, total: total})
	}
	sort.Slice(totals, func(i, j int) bool {
		if totals[i].total == totals[j].total {
			return totals[i].name < totals[j].name
		}
		return totals[i].total > totals[j].total
	})
	if len(totals) > 8 {
		totals = totals[:8]
	}

	stats.RecentModelStats = make([]DailyModelStat, 0, len(totals)*7)
	for i := 0; i < 7; i++ {
		day := startDay.AddDate(0, 0, i).Format("2006-01-02")
		for _, total := range totals {
			tokenCount := int64(0)
			if perDay, ok := dailyModel[day]; ok {
				tokenCount = perDay[total.name]
			}
			stats.RecentModelStats = append(stats.RecentModelStats, DailyModelStat{
				Date:       day,
				ModelName:  total.name,
				TokenCount: tokenCount,
			})
		}
	}

	return stats, nil
}
