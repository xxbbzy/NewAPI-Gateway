package model

import (
	"NewAPI-Gateway/common"
	"encoding/json"
	"errors"
	"math"
	"math/rand"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	defaultRoutingUsageWindowHours = 24
	defaultRoutingBaseWeightFactor = 0.2
	defaultRoutingValueScoreFactor = 0.8
	defaultRoutingHealthEnabled    = false
	defaultRoutingHealthWindowHour = 6
	defaultRoutingFailurePenaltyA  = 4.0
	defaultRoutingHealthRewardBeta = 0.08
	defaultRoutingHealthMinMult    = 0.05
	defaultRoutingHealthMaxMult    = 1.12
	defaultRoutingHealthMinSamples = 5

	routingUsageWindowHoursOptionKey    = "RoutingUsageWindowHours"
	routingBaseWeightFactorOptionKey    = "RoutingBaseWeightFactor"
	routingValueScoreFactorOptionKey    = "RoutingValueScoreFactor"
	routingHealthEnabledOptionKey       = "RoutingHealthAdjustmentEnabled"
	routingHealthWindowHoursOptionKey   = "RoutingHealthWindowHours"
	routingFailurePenaltyAlphaOptionKey = "RoutingFailurePenaltyAlpha"
	routingHealthRewardBetaOptionKey    = "RoutingHealthRewardBeta"
	routingHealthMinMultiplierOptionKey = "RoutingHealthMinMultiplier"
	routingHealthMaxMultiplierOptionKey = "RoutingHealthMaxMultiplier"
	routingHealthMinSamplesOptionKey    = "RoutingHealthMinSamples"
)

var balanceNumberPattern = regexp.MustCompile(`[-+]?\d*\.?\d+`)

type ModelRoute struct {
	Id              int    `json:"id"`
	ModelName       string `json:"model_name" gorm:"type:varchar(255);index;not null"`
	ProviderTokenId int    `json:"provider_token_id" gorm:"index;not null"`
	ProviderId      int    `json:"provider_id" gorm:"index;not null"`
	Enabled         bool   `json:"enabled" gorm:"default:true"`
	Priority        int    `json:"priority" gorm:"default:0;index"`
	Weight          int    `json:"weight" gorm:"default:10"`
}

type ModelRoutePatch struct {
	Id       int   `json:"id"`
	Priority *int  `json:"priority,omitempty"`
	Weight   *int  `json:"weight,omitempty"`
	Enabled  *bool `json:"enabled,omitempty"`
}

func (p *ModelRoutePatch) ToUpdates() map[string]interface{} {
	updates := make(map[string]interface{})
	if p.Priority != nil {
		updates["priority"] = *p.Priority
	}
	if p.Weight != nil {
		updates["weight"] = *p.Weight
	}
	if p.Enabled != nil {
		updates["enabled"] = *p.Enabled
	}
	return updates
}

type ModelRouteOverviewItem struct {
	Id                      int      `json:"id"`
	DisplayModelName        string   `json:"display_model_name"`
	ModelName               string   `json:"model_name"`
	ProviderId              int      `json:"provider_id"`
	ProviderName            string   `json:"provider_name"`
	ProviderBalance         string   `json:"provider_balance"`
	ProviderStatus          int      `json:"provider_status"`
	ProviderTokenId         int      `json:"provider_token_id"`
	TokenName               string   `json:"token_name"`
	TokenGroupName          string   `json:"token_group_name"`
	TokenStatus             int      `json:"token_status"`
	Enabled                 bool     `json:"enabled"`
	Priority                int      `json:"priority"`
	Weight                  int      `json:"weight"`
	BillingType             string   `json:"billing_type"`
	GroupRatio              float64  `json:"group_ratio"`
	PromptPricePer1M        *float64 `json:"prompt_price_per_1m"`
	CompletionPricePer1M    *float64 `json:"completion_price_per_1m"`
	PerCallPrice            *float64 `json:"per_call_price"`
	RecentUsageCostUSD      float64  `json:"recent_usage_cost_usd"`
	ValueScore              *float64 `json:"value_score"`
	UsageWindowHours        int      `json:"usage_window_hours"`
	BaseWeightFactor        float64  `json:"base_weight_factor"`
	ValueScoreFactor        float64  `json:"value_score_factor"`
	HealthAdjustmentEnabled bool     `json:"health_adjustment_enabled"`
	HealthWindowHours       int      `json:"health_window_hours"`
	FailurePenaltyAlpha     float64  `json:"failure_penalty_alpha"`
	HealthRewardBeta        float64  `json:"health_reward_beta"`
	HealthMinMultiplier     float64  `json:"health_min_multiplier"`
	HealthMaxMultiplier     float64  `json:"health_max_multiplier"`
	HealthMinSamples        int      `json:"health_min_samples"`
	HealthMultiplier        float64  `json:"health_multiplier"`
	HealthSampleCount       int64    `json:"health_sample_count"`
	HealthSuccessRate       *float64 `json:"health_success_rate"`
	HealthFailRate          *float64 `json:"health_fail_rate"`
	HealthAvgLatencyMs      *float64 `json:"health_avg_latency_ms"`
	EffectiveSharePercent   *float64 `json:"effective_share_percent"`
}

type modelRouteOverviewRow struct {
	Id                int     `gorm:"column:id"`
	ModelName         string  `gorm:"column:model_name"`
	ProviderId        int     `gorm:"column:provider_id"`
	ProviderName      string  `gorm:"column:provider_name"`
	ProviderBalance   string  `gorm:"column:provider_balance"`
	ProviderStatus    int     `gorm:"column:provider_status"`
	ProviderTokenId   int     `gorm:"column:provider_token_id"`
	TokenName         string  `gorm:"column:token_name"`
	TokenGroupName    string  `gorm:"column:token_group_name"`
	TokenStatus       int     `gorm:"column:token_status"`
	Enabled           bool    `gorm:"column:enabled"`
	Priority          int     `gorm:"column:priority"`
	Weight            int     `gorm:"column:weight"`
	PricingGroupRatio string  `gorm:"column:pricing_group_ratio"`
	QuotaType         int     `gorm:"column:quota_type"`
	ModelRatio        float64 `gorm:"column:model_ratio"`
	CompletionRatio   float64 `gorm:"column:completion_ratio"`
	ModelPrice        float64 `gorm:"column:model_price"`
}

type RouteAttempt struct {
	Route              ModelRoute
	Token              *ProviderToken
	Provider           *Provider
	Contribution       float64
	ValueScore         float64
	ProviderBalance    float64
	RecentUsageCostUSD float64
}

type routeRuntimeMetrics struct {
	UnitCostUSD        float64
	ProviderBalanceUSD float64
	RecentUsageCostUSD float64
	ValueScore         float64
	HealthMultiplier   float64
	HealthSampleCount  int64
	HealthSuccessRate  float64
	HealthFailRate     float64
	HealthAvgLatencyMs float64
}

type routingTuningConfig struct {
	UsageWindowHours    int
	BaseWeightFactor    float64
	ValueScoreFactor    float64
	HealthEnabled       bool
	HealthWindowHours   int
	FailurePenaltyAlpha float64
	HealthRewardBeta    float64
	HealthMinMultiplier float64
	HealthMaxMultiplier float64
	HealthMinSamples    int
}

type routeHealthStats struct {
	SuccessCount int64
	ErrorCount   int64
	SampleCount  int64
	AvgLatencyMs float64
	SuccessRate  float64
	FailRate     float64
	HealthScore  float64
	Multiplier   float64
}

// SelectProviderToken selects a provider token for a specific priority-retry index.
// It is kept for compatibility with existing callers and now uses the dynamic route plan.
func SelectProviderToken(modelName string, retry int) (*ProviderToken, *Provider, string, error) {
	requestedModel := strings.TrimSpace(modelName)
	if requestedModel == "" {
		return nil, nil, "", errors.New("无效的模型名称")
	}

	plan, err := BuildRouteAttemptsByPriority(requestedModel)
	if err != nil {
		return nil, nil, "", err
	}
	if len(plan) == 0 {
		return nil, nil, "", errors.New("无可用的模型路由: " + requestedModel)
	}

	idx := retry
	if idx < 0 {
		idx = 0
	}
	if idx >= len(plan) {
		idx = len(plan) - 1
	}
	if len(plan[idx]) == 0 {
		return nil, nil, "", errors.New("路由选择失败")
	}
	chosen := plan[idx][0]
	return chosen.Token, chosen.Provider, chosen.Route.ModelName, nil
}

// BuildRouteAttemptsByPriority returns retry plan grouped by priority (high -> low).
// Inside one priority level, routes are ordered by weighted-random without replacement.
func BuildRouteAttemptsByPriority(modelName string) ([][]RouteAttempt, error) {
	requestedModel := strings.TrimSpace(modelName)
	if requestedModel == "" {
		return nil, errors.New("无效的模型名称")
	}

	candidateRoutes, err := getCandidateRoutesByModel(requestedModel)
	if err != nil {
		return nil, err
	}
	if len(candidateRoutes) == 0 {
		return nil, errors.New("无可用的模型路由: " + requestedModel)
	}
	config := loadRoutingTuningConfig()

	providerIds := make([]int, 0)
	providerSeen := make(map[int]bool)
	tokenIds := make([]int, 0)
	tokenSeen := make(map[int]bool)
	modelNames := make([]string, 0)
	modelSeen := make(map[string]bool)
	for _, route := range candidateRoutes {
		if !providerSeen[route.ProviderId] {
			providerSeen[route.ProviderId] = true
			providerIds = append(providerIds, route.ProviderId)
		}
		if !tokenSeen[route.ProviderTokenId] {
			tokenSeen[route.ProviderTokenId] = true
			tokenIds = append(tokenIds, route.ProviderTokenId)
		}
		if !modelSeen[route.ModelName] {
			modelSeen[route.ModelName] = true
			modelNames = append(modelNames, route.ModelName)
		}
	}

	providerLookup, err := loadProvidersByIDs(providerIds)
	if err != nil {
		return nil, err
	}
	tokenLookup, err := loadProviderTokensByIDs(tokenIds)
	if err != nil {
		return nil, err
	}

	metricLookup, err := buildRouteRuntimeMetrics(candidateRoutes, providerLookup, tokenLookup, modelNames, config)
	if err != nil {
		return nil, err
	}

	prioritySet := make(map[int]bool)
	priorities := make([]int, 0)
	routesByPriority := make(map[int][]ModelRoute)
	for _, route := range candidateRoutes {
		if !prioritySet[route.Priority] {
			prioritySet[route.Priority] = true
			priorities = append(priorities, route.Priority)
		}
		routesByPriority[route.Priority] = append(routesByPriority[route.Priority], route)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(priorities)))

	plan := make([][]RouteAttempt, 0, len(priorities))
	for _, priority := range priorities {
		routes := routesByPriority[priority]
		attempts := make([]RouteAttempt, 0, len(routes))
		maxScore := 0.0
		for _, route := range routes {
			provider := providerLookup[route.ProviderId]
			token := tokenLookup[route.ProviderTokenId]
			if provider == nil || token == nil {
				continue
			}
			if provider.Status != common.UserStatusEnabled || token.Status != common.UserStatusEnabled {
				continue
			}
			metric := metricLookup[route.Id]
			if metric.ValueScore > maxScore {
				maxScore = metric.ValueScore
			}
			attempts = append(attempts, RouteAttempt{
				Route:              route,
				Token:              token,
				Provider:           provider,
				ValueScore:         metric.ValueScore,
				ProviderBalance:    metric.ProviderBalanceUSD,
				RecentUsageCostUSD: metric.RecentUsageCostUSD,
			})
		}
		if len(attempts) == 0 {
			continue
		}
		for i := range attempts {
			baseContribution := computeRouteContribution(
				attempts[i].Route.Weight,
				attempts[i].ValueScore,
				maxScore,
				config.BaseWeightFactor,
				config.ValueScoreFactor,
			)
			healthMultiplier := 1.0
			if metric, ok := metricLookup[attempts[i].Route.Id]; ok {
				healthMultiplier = metric.HealthMultiplier
			}
			if healthMultiplier <= 0 {
				healthMultiplier = 0.0001
			}
			attempts[i].Contribution = baseContribution * healthMultiplier
		}
		plan = append(plan, weightedShuffleAttempts(attempts))
	}

	if len(plan) == 0 {
		return nil, errors.New("无可用的模型路由: " + requestedModel)
	}
	return plan, nil
}

func getCandidateRoutesByModel(requestedModel string) ([]ModelRoute, error) {
	return getCandidateRoutesByModelCached(requestedModel)
}

func loadProviderAliasLookups(routes []ModelRoute) (map[int]providerModelAliasLookup, error) {
	providerIds := make([]int, 0)
	providerSet := make(map[int]bool)
	for _, route := range routes {
		if providerSet[route.ProviderId] {
			continue
		}
		providerSet[route.ProviderId] = true
		providerIds = append(providerIds, route.ProviderId)
	}

	lookups := make(map[int]providerModelAliasLookup)
	if len(providerIds) == 0 {
		return lookups, nil
	}

	var providers []Provider
	if err := DB.Select("id", "model_alias_mapping").Where("id IN ?", providerIds).Find(&providers).Error; err != nil {
		return nil, err
	}
	for _, p := range providers {
		mapping := ParseProviderAliasMapping(p.ModelAliasMapping)
		lookups[p.Id] = buildProviderModelAliasLookup(mapping)
	}
	return lookups, nil
}

func routeMatchesRequestedModel(routeModelName string, requestedModel string, requestedNormalized string,
	requestedVersionKey string, lookup providerModelAliasLookup) bool {
	routeName := strings.TrimSpace(routeModelName)
	if routeName == "" {
		return false
	}

	if strings.EqualFold(routeName, requestedModel) {
		return true
	}

	routeNormalized := common.NormalizeModelName(routeName)
	if requestedNormalized != "" && routeNormalized != "" && routeNormalized == requestedNormalized {
		return true
	}
	if requestedVersionKey != "" && routeNormalized != "" && common.ToVersionAgnosticKey(routeNormalized) == requestedVersionKey {
		return true
	}

	if mappedModel, ok := lookup.Resolve(requestedModel); ok {
		if strings.EqualFold(routeName, mappedModel) {
			return true
		}
		mappedNormalized := common.NormalizeModelName(mappedModel)
		if mappedNormalized != "" && routeNormalized != "" && mappedNormalized == routeNormalized {
			return true
		}
	}
	return false
}

func loadProvidersByIDs(providerIds []int) (map[int]*Provider, error) {
	lookup := make(map[int]*Provider)
	if len(providerIds) == 0 {
		return lookup, nil
	}

	missingIds := make([]int, 0, len(providerIds))
	snapshot, snapshotErr := getRoutingStaticSnapshot()
	if snapshotErr == nil && snapshot != nil {
		for _, id := range providerIds {
			if provider, ok := snapshot.providersByID[id]; ok {
				lookup[id] = provider
				continue
			}
			missingIds = append(missingIds, id)
		}
	} else {
		missingIds = append(missingIds, providerIds...)
	}

	if len(missingIds) > 0 {
		var providers []Provider
		if err := DB.Where("id IN ?", missingIds).Find(&providers).Error; err != nil {
			return nil, err
		}
		for i := range providers {
			provider := &providers[i]
			lookup[provider.Id] = provider
		}
	}
	return lookup, nil
}

func loadProviderTokensByIDs(tokenIds []int) (map[int]*ProviderToken, error) {
	lookup := make(map[int]*ProviderToken)
	if len(tokenIds) == 0 {
		return lookup, nil
	}

	missingIds := make([]int, 0, len(tokenIds))
	snapshot, snapshotErr := getRoutingStaticSnapshot()
	if snapshotErr == nil && snapshot != nil {
		for _, id := range tokenIds {
			if token, ok := snapshot.tokensByID[id]; ok {
				lookup[id] = token
				continue
			}
			missingIds = append(missingIds, id)
		}
	} else {
		missingIds = append(missingIds, tokenIds...)
	}

	if len(missingIds) > 0 {
		var tokens []ProviderToken
		if err := DB.Where("id IN ?", missingIds).Find(&tokens).Error; err != nil {
			return nil, err
		}
		for i := range tokens {
			token := &tokens[i]
			lookup[token.Id] = token
		}
	}
	return lookup, nil
}

func buildRouteRuntimeMetrics(routes []ModelRoute, providers map[int]*Provider, tokens map[int]*ProviderToken, modelNames []string, config routingTuningConfig) (map[int]routeRuntimeMetrics, error) {
	metrics := make(map[int]routeRuntimeMetrics)
	if len(routes) == 0 {
		return metrics, nil
	}

	providerIds := make([]int, 0, len(providers))
	for id := range providers {
		providerIds = append(providerIds, id)
	}

	groupRatioLookup := make(map[int]map[string]float64)
	for providerId, provider := range providers {
		groupRatioLookup[providerId] = parseGroupRatioMap(provider.PricingGroupRatio)
	}

	tokenIds := make([]int, 0, len(tokens))
	tokenGroupLookup := make(map[int]string)
	for tokenId, token := range tokens {
		tokenIds = append(tokenIds, tokenId)
		tokenGroupLookup[tokenId] = token.GroupName
	}

	pricingLookup, err := loadPricingLookupByProviderModels(providerIds, modelNames)
	if err != nil {
		return nil, err
	}

	usageLookup, err := loadRecentUsageCostByTokenModel(tokenIds, modelNames, config.UsageWindowHours)
	if err != nil {
		return nil, err
	}

	healthLookup, err := loadRouteHealthStatsByTokenModel(tokenIds, modelNames, config.HealthWindowHours)
	if err != nil {
		return nil, err
	}

	for _, route := range routes {
		provider := providers[route.ProviderId]
		if provider == nil {
			continue
		}
		balanceUSD := parseBalanceUSD(provider.Balance)
		tokenGroup := tokenGroupLookup[route.ProviderTokenId]
		unitCostUSD := calcRouteUnitCostUSD(route.ProviderId, route.ModelName, tokenGroup, groupRatioLookup, pricingLookup)
		recentUsageUSD := usageLookup[routeUsageKey(route.ProviderTokenId, route.ModelName)]
		valueScore := computeRouteValueScore(unitCostUSD, balanceUSD, recentUsageUSD)
		healthStats := finalizeRouteHealthStat(
			healthLookup[routeUsageKey(route.ProviderTokenId, route.ModelName)],
			config,
		)
		metrics[route.Id] = routeRuntimeMetrics{
			UnitCostUSD:        unitCostUSD,
			ProviderBalanceUSD: balanceUSD,
			RecentUsageCostUSD: recentUsageUSD,
			ValueScore:         valueScore,
			HealthMultiplier:   healthStats.Multiplier,
			HealthSampleCount:  healthStats.SampleCount,
			HealthSuccessRate:  healthStats.SuccessRate,
			HealthFailRate:     healthStats.FailRate,
			HealthAvgLatencyMs: healthStats.AvgLatencyMs,
		}
	}

	return metrics, nil
}

func weightedShuffleAttempts(attempts []RouteAttempt) []RouteAttempt {
	pool := make([]RouteAttempt, len(attempts))
	copy(pool, attempts)
	ordered := make([]RouteAttempt, 0, len(attempts))

	for len(pool) > 0 {
		totalWeight := 0.0
		for _, item := range pool {
			if item.Contribution > 0 {
				totalWeight += item.Contribution
			}
		}

		selectedIdx := 0
		if totalWeight > 0 {
			pick := rand.Float64() * totalWeight
			acc := 0.0
			for i, item := range pool {
				weight := item.Contribution
				if weight < 0 {
					weight = 0
				}
				acc += weight
				if pick <= acc {
					selectedIdx = i
					break
				}
			}
		} else {
			selectedIdx = rand.Intn(len(pool))
		}

		ordered = append(ordered, pool[selectedIdx])
		pool = append(pool[:selectedIdx], pool[selectedIdx+1:]...)
	}
	return ordered
}

func routePricingKey(providerId int, modelName string) string {
	return strconv.Itoa(providerId) + "#" + strings.TrimSpace(modelName)
}

func routeUsageKey(providerTokenId int, modelName string) string {
	return strconv.Itoa(providerTokenId) + "#" + strings.TrimSpace(modelName)
}

func routeModelTokenKey(modelName string, providerTokenId int) string {
	return strings.TrimSpace(modelName) + "|" + strconv.Itoa(providerTokenId)
}

func calcRouteUnitCostUSD(providerId int, modelName string, tokenGroup string, groupRatioLookup map[int]map[string]float64, pricingLookup map[string]ModelPricing) float64 {
	pricing, ok := pricingLookup[routePricingKey(providerId, modelName)]
	if !ok {
		return 0
	}
	groupRatioMap := groupRatioLookup[providerId]
	groupRatio := getGroupRatio(tokenGroup, groupRatioMap)

	if pricing.ModelPrice > 0 || pricing.QuotaType == 1 {
		cost := pricing.ModelPrice * groupRatio
		if cost < 0 {
			return 0
		}
		return cost
	}

	if pricing.ModelRatio <= 0 {
		return 0
	}
	promptPrice := pricing.ModelRatio * 2 * groupRatio
	completionRatio := pricing.CompletionRatio
	if completionRatio <= 0 {
		completionRatio = 1
	}
	completionPrice := promptPrice * completionRatio
	total := promptPrice + completionPrice
	if total < 0 {
		return 0
	}
	return total
}

func computeRouteValueScore(unitCostUSD float64, balanceUSD float64, recentUsageUSD float64) float64 {
	costScore := 0.5
	if unitCostUSD > 0 {
		costScore = 1 / (1 + unitCostUSD)
	}

	if balanceUSD < 0 {
		balanceUSD = 0
	}
	if recentUsageUSD < 0 {
		recentUsageUSD = 0
	}
	budgetScore := (balanceUSD + 1) / (balanceUSD + recentUsageUSD + 1)
	if budgetScore < 0 {
		budgetScore = 0
	}
	return costScore * budgetScore
}

func loadRoutingTuningConfig() routingTuningConfig {
	config := routingTuningConfig{
		UsageWindowHours:    defaultRoutingUsageWindowHours,
		BaseWeightFactor:    defaultRoutingBaseWeightFactor,
		ValueScoreFactor:    defaultRoutingValueScoreFactor,
		HealthEnabled:       defaultRoutingHealthEnabled,
		HealthWindowHours:   defaultRoutingHealthWindowHour,
		FailurePenaltyAlpha: defaultRoutingFailurePenaltyA,
		HealthRewardBeta:    defaultRoutingHealthRewardBeta,
		HealthMinMultiplier: defaultRoutingHealthMinMult,
		HealthMaxMultiplier: defaultRoutingHealthMaxMult,
		HealthMinSamples:    defaultRoutingHealthMinSamples,
	}

	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()

	config.UsageWindowHours = parseOptionIntInRange(
		common.OptionMap[routingUsageWindowHoursOptionKey],
		defaultRoutingUsageWindowHours,
		1,
		24*30,
	)
	config.BaseWeightFactor = parseOptionFloatInRange(
		common.OptionMap[routingBaseWeightFactorOptionKey],
		defaultRoutingBaseWeightFactor,
		0,
		10,
	)
	config.ValueScoreFactor = parseOptionFloatInRange(
		common.OptionMap[routingValueScoreFactorOptionKey],
		defaultRoutingValueScoreFactor,
		0,
		10,
	)
	config.HealthEnabled = parseOptionBool(
		common.OptionMap[routingHealthEnabledOptionKey],
		defaultRoutingHealthEnabled,
	)
	config.HealthWindowHours = parseOptionIntInRange(
		common.OptionMap[routingHealthWindowHoursOptionKey],
		defaultRoutingHealthWindowHour,
		1,
		24*30,
	)
	config.FailurePenaltyAlpha = parseOptionFloatInRange(
		common.OptionMap[routingFailurePenaltyAlphaOptionKey],
		defaultRoutingFailurePenaltyA,
		0,
		20,
	)
	config.HealthRewardBeta = parseOptionFloatInRange(
		common.OptionMap[routingHealthRewardBetaOptionKey],
		defaultRoutingHealthRewardBeta,
		0,
		2,
	)
	config.HealthMinMultiplier = parseOptionFloatInRange(
		common.OptionMap[routingHealthMinMultiplierOptionKey],
		defaultRoutingHealthMinMult,
		0,
		10,
	)
	config.HealthMaxMultiplier = parseOptionFloatInRange(
		common.OptionMap[routingHealthMaxMultiplierOptionKey],
		defaultRoutingHealthMaxMult,
		0,
		10,
	)
	config.HealthMinSamples = parseOptionIntInRange(
		common.OptionMap[routingHealthMinSamplesOptionKey],
		defaultRoutingHealthMinSamples,
		1,
		1000,
	)
	if config.BaseWeightFactor == 0 && config.ValueScoreFactor == 0 {
		config.BaseWeightFactor = defaultRoutingBaseWeightFactor
		config.ValueScoreFactor = defaultRoutingValueScoreFactor
	}
	if config.HealthMaxMultiplier < config.HealthMinMultiplier {
		config.HealthMaxMultiplier = config.HealthMinMultiplier
	}
	return config
}

func parseOptionBool(raw string, fallback bool) bool {
	text := strings.TrimSpace(strings.ToLower(raw))
	if text == "" {
		return fallback
	}
	switch text {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	default:
		return fallback
	}
}

func parseOptionIntInRange(raw string, fallback int, min int, max int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	if value < min || value > max {
		return fallback
	}
	return value
}

func parseOptionFloatInRange(raw string, fallback float64, min float64, max float64) float64 {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return fallback
	}
	if value < min || value > max {
		return fallback
	}
	return value
}

func computeRouteContribution(weight int, valueScore float64, maxValueScore float64, baseWeightFactor float64, valueScoreFactor float64) float64 {
	base := float64(weight + 10)
	if base <= 0 {
		return 0
	}
	if baseWeightFactor < 0 {
		baseWeightFactor = 0
	}
	if valueScoreFactor < 0 {
		valueScoreFactor = 0
	}
	if baseWeightFactor == 0 && valueScoreFactor == 0 {
		baseWeightFactor = defaultRoutingBaseWeightFactor
		valueScoreFactor = defaultRoutingValueScoreFactor
	}
	if maxValueScore <= 0 {
		return base
	}
	normalized := valueScore / maxValueScore
	if normalized < 0 {
		normalized = 0
	}
	if normalized > 1 {
		normalized = 1
	}
	// Keep a baseline from manual weight while allowing value score to bias traffic.
	multiplier := baseWeightFactor + normalized*valueScoreFactor
	return base * multiplier
}

func parseBalanceUSD(raw string) float64 {
	text := strings.TrimSpace(raw)
	if text == "" {
		return 0
	}
	text = strings.ReplaceAll(text, ",", "")
	text = strings.TrimPrefix(text, "$")
	text = strings.TrimPrefix(strings.ToLower(text), "usd")
	text = strings.TrimSpace(text)
	if value, err := strconv.ParseFloat(text, 64); err == nil && !math.IsNaN(value) && !math.IsInf(value, 0) {
		if value < 0 {
			return 0
		}
		return value
	}
	matched := balanceNumberPattern.FindString(text)
	if matched == "" {
		return 0
	}
	value, err := strconv.ParseFloat(matched, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return 0
	}
	return value
}

func loadRecentUsageCostByTokenModel(tokenIds []int, modelNames []string, usageWindowHours int) (map[string]float64, error) {
	usageLookup := make(map[string]float64)
	if len(tokenIds) == 0 || len(modelNames) == 0 {
		return usageLookup, nil
	}
	if usageWindowHours <= 0 {
		usageWindowHours = defaultRoutingUsageWindowHours
	}
	if cached, ok := getCachedRecentUsageCostByTokenModel(tokenIds, modelNames, usageWindowHours); ok {
		return cached, nil
	}

	type usageRow struct {
		ProviderTokenId int     `gorm:"column:provider_token_id"`
		ModelName       string  `gorm:"column:model_name"`
		TotalCost       float64 `gorm:"column:total_cost"`
	}
	var rows []usageRow
	since := time.Now().Add(-time.Duration(usageWindowHours) * time.Hour).Unix()
	if err := DB.Table("usage_logs").
		Select("provider_token_id, model_name, COALESCE(SUM(cost_usd), 0) AS total_cost").
		Where("created_at >= ? AND provider_token_id IN ? AND model_name IN ? AND status = 1", since, tokenIds, modelNames).
		Group("provider_token_id, model_name").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		usageLookup[routeUsageKey(row.ProviderTokenId, row.ModelName)] = row.TotalCost
	}
	setCachedRecentUsageCostByTokenModel(tokenIds, modelNames, usageWindowHours, usageLookup)
	return usageLookup, nil
}

func loadRouteHealthStatsByTokenModel(tokenIds []int, modelNames []string, windowHours int) (map[string]routeHealthStats, error) {
	statsLookup := make(map[string]routeHealthStats)
	if len(tokenIds) == 0 || len(modelNames) == 0 {
		return statsLookup, nil
	}
	if windowHours <= 0 {
		windowHours = defaultRoutingHealthWindowHour
	}
	if cached, ok := getCachedRouteHealthStatsByTokenModel(tokenIds, modelNames, windowHours); ok {
		return cached, nil
	}

	type healthRow struct {
		ProviderTokenId int     `gorm:"column:provider_token_id"`
		ModelName       string  `gorm:"column:model_name"`
		SuccessCount    int64   `gorm:"column:success_count"`
		ErrorCount      int64   `gorm:"column:error_count"`
		SampleCount     int64   `gorm:"column:sample_count"`
		AvgLatencyMs    float64 `gorm:"column:avg_latency_ms"`
	}

	const successCondition = "(status = 1 AND (error_message IS NULL OR TRIM(error_message) = ''))"
	const errorCondition = "(status <> 1 OR (error_message IS NOT NULL AND TRIM(error_message) <> ''))"

	var rows []healthRow
	since := time.Now().Add(-time.Duration(windowHours) * time.Hour).Unix()
	if err := DB.Table("usage_logs").
		Select(
			"provider_token_id",
			"model_name",
			"SUM(CASE WHEN "+successCondition+" THEN 1 ELSE 0 END) AS success_count",
			"SUM(CASE WHEN "+errorCondition+" THEN 1 ELSE 0 END) AS error_count",
			"COUNT(*) AS sample_count",
			"COALESCE(AVG(response_time_ms), 0) AS avg_latency_ms",
		).
		Where("created_at >= ? AND provider_token_id IN ? AND model_name IN ?", since, tokenIds, modelNames).
		Group("provider_token_id, model_name").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	for _, row := range rows {
		successRate := 0.0
		failRate := 0.0
		if row.SampleCount > 0 {
			successRate = float64(row.SuccessCount) / float64(row.SampleCount)
			failRate = float64(row.ErrorCount) / float64(row.SampleCount)
		}
		statsLookup[routeUsageKey(row.ProviderTokenId, row.ModelName)] = routeHealthStats{
			SuccessCount: row.SuccessCount,
			ErrorCount:   row.ErrorCount,
			SampleCount:  row.SampleCount,
			AvgLatencyMs: row.AvgLatencyMs,
			SuccessRate:  successRate,
			FailRate:     failRate,
		}
	}
	setCachedRouteHealthStatsByTokenModel(tokenIds, modelNames, windowHours, statsLookup)
	return statsLookup, nil
}

func finalizeRouteHealthStat(stat routeHealthStats, config routingTuningConfig) routeHealthStats {
	stat.Multiplier = 1
	if !config.HealthEnabled {
		return stat
	}
	if stat.SampleCount < int64(config.HealthMinSamples) {
		return stat
	}

	failRate := stat.FailRate
	if failRate < 0 {
		failRate = 0
	}
	if failRate > 1 {
		failRate = 1
	}
	successRate := stat.SuccessRate
	if successRate < 0 {
		successRate = 0
	}
	if successRate > 1 {
		successRate = 1
	}

	latencyScore := 1.0
	if stat.AvgLatencyMs > 0 {
		const lowMs = 1500.0
		const highMs = 10000.0
		if stat.AvgLatencyMs <= lowMs {
			latencyScore = 1
		} else if stat.AvgLatencyMs >= highMs {
			latencyScore = 0
		} else {
			latencyScore = 1 - (stat.AvgLatencyMs-lowMs)/(highMs-lowMs)
		}
	}
	if latencyScore < 0 {
		latencyScore = 0
	}
	if latencyScore > 1 {
		latencyScore = 1
	}

	confidence := math.Min(1, float64(stat.SampleCount)/50.0)
	healthScore := successRate*0.75 + latencyScore*0.25
	penalty := math.Exp(-config.FailurePenaltyAlpha * failRate)
	reward := 1 + config.HealthRewardBeta*healthScore*confidence
	multiplier := penalty * reward

	if multiplier < config.HealthMinMultiplier {
		multiplier = config.HealthMinMultiplier
	}
	if multiplier > config.HealthMaxMultiplier {
		multiplier = config.HealthMaxMultiplier
	}

	stat.HealthScore = healthScore
	stat.SuccessRate = successRate
	stat.FailRate = failRate
	stat.Multiplier = multiplier
	return stat
}

// GetAllModelRoutes returns all routes with optional model name filter
func GetAllModelRoutes(modelName string, startIdx int, num int) ([]*ModelRoute, error) {
	var routes []*ModelRoute
	query := DB.Order("model_name ASC, priority DESC")
	if modelName != "" {
		query = query.Where("model_name LIKE ?", "%"+modelName+"%")
	}
	err := query.Limit(num).Offset(startIdx).Find(&routes).Error
	return routes, err
}

// GetDistinctModels returns all unique model names available in routes
func GetDistinctModels() ([]string, error) {
	var models []string
	err := DB.Model(&ModelRoute{}).Where("enabled = ?", true).
		Distinct("model_name").Pluck("model_name", &models).Error
	return models, err
}

func (r *ModelRoute) Update() error {
	if err := DB.Model(r).Updates(r).Error; err != nil {
		return err
	}
	invalidateModelRouteCaches()
	return nil
}

func UpdateModelRouteFields(id int, updates map[string]interface{}) error {
	if id <= 0 {
		return errors.New("无效的路由 ID")
	}
	if len(updates) == 0 {
		return errors.New("没有可更新的字段")
	}
	if err := DB.Model(&ModelRoute{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return err
	}
	invalidateModelRouteCaches()
	return nil
}

func BatchUpdateModelRoutes(patches []ModelRoutePatch) error {
	if len(patches) == 0 {
		return errors.New("更新列表为空")
	}
	tx := DB.Begin()
	for _, patch := range patches {
		if patch.Id <= 0 {
			tx.Rollback()
			return errors.New("存在无效的路由 ID")
		}
		updates := patch.ToUpdates()
		if len(updates) == 0 {
			continue
		}
		if err := tx.Model(&ModelRoute{}).Where("id = ?", patch.Id).Updates(updates).Error; err != nil {
			tx.Rollback()
			return err
		}
	}
	if err := tx.Commit().Error; err != nil {
		return err
	}
	invalidateModelRouteCaches()
	return nil
}

func GetModelRouteOverview(modelName string, providerId int, enabledOnly bool) ([]*ModelRouteOverviewItem, error) {
	config := loadRoutingTuningConfig()

	query := DB.Table("model_routes AS mr").
		Select(strings.Join([]string{
			"mr.id",
			"mr.model_name",
			"mr.provider_id",
			"COALESCE(p.name, '') AS provider_name",
			"COALESCE(p.balance, '') AS provider_balance",
			"COALESCE(p.status, 0) AS provider_status",
			"mr.provider_token_id",
			"COALESCE(pt.name, '') AS token_name",
			"COALESCE(pt.group_name, '') AS token_group_name",
			"COALESCE(pt.status, 0) AS token_status",
			"mr.enabled",
			"mr.priority",
			"mr.weight",
			"COALESCE(p.pricing_group_ratio, '') AS pricing_group_ratio",
			"COALESCE(mp.quota_type, 0) AS quota_type",
			"COALESCE(mp.model_ratio, 0) AS model_ratio",
			"COALESCE(mp.completion_ratio, 0) AS completion_ratio",
			"COALESCE(mp.model_price, 0) AS model_price",
		}, ", ")).
		Joins("LEFT JOIN providers AS p ON p.id = mr.provider_id").
		Joins("LEFT JOIN provider_tokens AS pt ON pt.id = mr.provider_token_id").
		Joins("LEFT JOIN model_pricings AS mp ON mp.provider_id = mr.provider_id AND mp.model_name = mr.model_name")

	if modelName != "" {
		query = query.Where("mr.model_name LIKE ?", "%"+modelName+"%")
	}
	if providerId > 0 {
		query = query.Where("mr.provider_id = ?", providerId)
	}
	if enabledOnly {
		query = query.Where("mr.enabled = ?", true)
	}

	var rows []modelRouteOverviewRow
	err := query.Order("mr.model_name ASC, mr.priority DESC, mr.provider_id ASC, mr.provider_token_id ASC, mr.id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	ratioCache := make(map[int]map[string]float64)
	aliasDisplayLookupCache := make(map[int]providerModelAliasReverseLookup)

	providerIds := make([]int, 0)
	providerSeen := make(map[int]bool)
	for _, row := range rows {
		if providerSeen[row.ProviderId] {
			continue
		}
		providerSeen[row.ProviderId] = true
		providerIds = append(providerIds, row.ProviderId)
	}
	if len(providerIds) > 0 {
		var providers []Provider
		if err := DB.Select("id", "model_alias_mapping").Where("id IN ?", providerIds).Find(&providers).Error; err != nil {
			return nil, err
		}
		for _, p := range providers {
			aliasDisplayLookupCache[p.Id] = buildProviderModelAliasReverseLookup(ParseProviderAliasMapping(p.ModelAliasMapping))
		}
	}

	items := make([]*ModelRouteOverviewItem, 0, len(rows))
	for _, row := range rows {
		groupRatioMap, ok := ratioCache[row.ProviderId]
		if !ok {
			groupRatioMap = parseGroupRatioMap(row.PricingGroupRatio)
			ratioCache[row.ProviderId] = groupRatioMap
		}
		groupRatio := getGroupRatio(row.TokenGroupName, groupRatioMap)
		item := &ModelRouteOverviewItem{
			Id:                      row.Id,
			DisplayModelName:        "",
			ModelName:               row.ModelName,
			ProviderId:              row.ProviderId,
			ProviderName:            row.ProviderName,
			ProviderBalance:         row.ProviderBalance,
			ProviderStatus:          row.ProviderStatus,
			ProviderTokenId:         row.ProviderTokenId,
			TokenName:               row.TokenName,
			TokenGroupName:          row.TokenGroupName,
			TokenStatus:             row.TokenStatus,
			Enabled:                 row.Enabled,
			Priority:                row.Priority,
			Weight:                  row.Weight,
			GroupRatio:              groupRatio,
			UsageWindowHours:        config.UsageWindowHours,
			BaseWeightFactor:        config.BaseWeightFactor,
			ValueScoreFactor:        config.ValueScoreFactor,
			HealthAdjustmentEnabled: config.HealthEnabled,
			HealthWindowHours:       config.HealthWindowHours,
			FailurePenaltyAlpha:     config.FailurePenaltyAlpha,
			HealthRewardBeta:        config.HealthRewardBeta,
			HealthMinMultiplier:     config.HealthMinMultiplier,
			HealthMaxMultiplier:     config.HealthMaxMultiplier,
			HealthMinSamples:        config.HealthMinSamples,
			HealthMultiplier:        1,
		}
		if reverseLookup, ok := aliasDisplayLookupCache[row.ProviderId]; ok {
			if sourceModelName, matched := reverseLookup.ResolveByTarget(row.ModelName); matched {
				item.DisplayModelName = sourceModelName
			}
		}

		isPerCallBilling := row.ModelPrice > 0 || row.QuotaType == 1
		if isPerCallBilling {
			item.BillingType = "per_call"
			perCallPrice := row.ModelPrice * groupRatio
			item.PerCallPrice = &perCallPrice
		} else {
			item.BillingType = "per_token"
			promptPrice := row.ModelRatio * 2 * groupRatio
			completionRatio := row.CompletionRatio
			if completionRatio <= 0 {
				completionRatio = 1
			}
			completionPrice := promptPrice * completionRatio
			item.PromptPricePer1M = &promptPrice
			item.CompletionPricePer1M = &completionPrice
		}

		items = append(items, item)
	}

	tokenIds := make([]int, 0)
	tokenSeen := make(map[int]bool)
	modelNames := make([]string, 0)
	modelSeen := make(map[string]bool)
	for _, item := range items {
		if !tokenSeen[item.ProviderTokenId] {
			tokenSeen[item.ProviderTokenId] = true
			tokenIds = append(tokenIds, item.ProviderTokenId)
		}
		if !modelSeen[item.ModelName] {
			modelSeen[item.ModelName] = true
			modelNames = append(modelNames, item.ModelName)
		}
	}
	usageLookup, err := loadRecentUsageCostByTokenModel(tokenIds, modelNames, config.UsageWindowHours)
	if err != nil {
		return nil, err
	}
	healthLookup, err := loadRouteHealthStatsByTokenModel(tokenIds, modelNames, config.HealthWindowHours)
	if err != nil {
		return nil, err
	}

	groupMaxScore := make(map[string]float64)
	routeContribution := make(map[int]float64)
	for _, item := range items {
		unitCostUSD := 0.0
		if item.BillingType == "per_call" && item.PerCallPrice != nil {
			unitCostUSD = *item.PerCallPrice
		} else if item.PromptPricePer1M != nil && item.CompletionPricePer1M != nil {
			unitCostUSD = *item.PromptPricePer1M + *item.CompletionPricePer1M
		}

		balanceUSD := parseBalanceUSD(item.ProviderBalance)
		recentUsage := usageLookup[routeUsageKey(item.ProviderTokenId, item.ModelName)]
		item.RecentUsageCostUSD = recentUsage

		score := computeRouteValueScore(unitCostUSD, balanceUSD, recentUsage)
		if score > 0 {
			scoreCopy := score
			item.ValueScore = &scoreCopy
		}
		key := item.ModelName + "#" + strconv.Itoa(item.Priority)
		if score > groupMaxScore[key] {
			groupMaxScore[key] = score
		}

		healthStat := finalizeRouteHealthStat(
			healthLookup[routeUsageKey(item.ProviderTokenId, item.ModelName)],
			config,
		)
		item.HealthMultiplier = healthStat.Multiplier
		item.HealthSampleCount = healthStat.SampleCount
		if healthStat.SampleCount > 0 {
			successRate := healthStat.SuccessRate
			failRate := healthStat.FailRate
			avgLatencyMs := healthStat.AvgLatencyMs
			item.HealthSuccessRate = &successRate
			item.HealthFailRate = &failRate
			item.HealthAvgLatencyMs = &avgLatencyMs
		}
	}

	shareSum := make(map[string]float64)
	for _, item := range items {
		if !item.Enabled {
			continue
		}
		key := item.ModelName + "#" + strconv.Itoa(item.Priority)
		score := 0.0
		if item.ValueScore != nil {
			score = *item.ValueScore
		}
		contribution := computeRouteContribution(
			item.Weight,
			score,
			groupMaxScore[key],
			config.BaseWeightFactor,
			config.ValueScoreFactor,
		)
		if item.HealthMultiplier > 0 {
			contribution *= item.HealthMultiplier
		}
		routeContribution[item.Id] = contribution
		shareSum[key] += contribution
	}

	for _, item := range items {
		if !item.Enabled {
			item.EffectiveSharePercent = nil
			continue
		}
		contribution := routeContribution[item.Id]
		key := item.ModelName + "#" + strconv.Itoa(item.Priority)
		total := shareSum[key]
		if total <= 0 || contribution <= 0 {
			item.EffectiveSharePercent = nil
			continue
		}
		percent := contribution / total * 100
		item.EffectiveSharePercent = &percent
	}

	return items, nil
}

func parseGroupRatioMap(raw string) map[string]float64 {
	result := make(map[string]float64)
	if strings.TrimSpace(raw) == "" {
		return result
	}
	_ = json.Unmarshal([]byte(raw), &result)
	return result
}

func getGroupRatio(groupName string, groupRatioMap map[string]float64) float64 {
	if strings.TrimSpace(groupName) == "" {
		return 1
	}
	ratio, ok := groupRatioMap[groupName]
	if !ok || ratio <= 0 {
		return 1
	}
	return ratio
}

// RebuildRoutesForProvider rebuilds all model routes for a specific provider
func RebuildRoutesForProvider(providerId int, routes []ModelRoute) error {
	tx := DB.Begin()

	var existingRoutes []ModelRoute
	if err := tx.Where("provider_id = ?", providerId).Find(&existingRoutes).Error; err != nil {
		tx.Rollback()
		return err
	}
	existingMap := make(map[string]ModelRoute, len(existingRoutes))
	for _, route := range existingRoutes {
		existingMap[routeModelTokenKey(route.ModelName, route.ProviderTokenId)] = route
	}

	for i := range routes {
		key := routeModelTokenKey(routes[i].ModelName, routes[i].ProviderTokenId)
		if previous, ok := existingMap[key]; ok {
			routes[i].Enabled = previous.Enabled
			routes[i].Priority = previous.Priority
			routes[i].Weight = previous.Weight
		}
	}

	// Delete old routes
	if err := tx.Where("provider_id = ?", providerId).Delete(&ModelRoute{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Insert new routes in batches
	batchSize := 50
	for i := 0; i < len(routes); i += batchSize {
		end := i + batchSize
		if end > len(routes) {
			end = len(routes)
		}
		if err := tx.Create(routes[i:end]).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}
	invalidateModelRouteCaches()
	return nil
}

func CountModelRoutes() int64 {
	var count int64
	DB.Model(&ModelRoute{}).Count(&count)
	return count
}
