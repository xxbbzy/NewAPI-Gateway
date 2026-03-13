package model

import (
	"NewAPI-Gateway/common"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	routingStaticSnapshotTTL   = 60 * time.Second
	routingRuntimeMetricTTL    = 15 * time.Second
	routingRuntimeMetricMaxKey = 256
)

type routingStaticSnapshot struct {
	enabledRoutes              []ModelRoute
	routesByExactModel         map[string][]ModelRoute
	routesByNormalizedModel    map[string][]ModelRoute
	routesByVersionAgnostic    map[string][]ModelRoute
	routesByProviderExactModel map[int]map[string][]ModelRoute
	routesByProviderNormModel  map[int]map[string][]ModelRoute
	providerAliasLookups       map[int]providerModelAliasLookup
	providerAliasReverseLookup map[int]providerModelAliasReverseLookup
	providersByID              map[int]*Provider
	tokensByID                 map[int]*ProviderToken
	pricingLookup              map[string]ModelPricing
	modelCatalog               modelCatalogSnapshot
}

type routingUsageCacheEntry struct {
	expiresAt time.Time
	data      map[string]float64
}

type routingHealthCacheEntry struct {
	expiresAt time.Time
	data      map[string]routeHealthStats
}

var routingStaticCacheState = struct {
	mu        sync.RWMutex
	snapshot  *routingStaticSnapshot
	expiresAt time.Time
}{}

var routingUsageCacheState = struct {
	mu      sync.RWMutex
	entries map[string]routingUsageCacheEntry
}{
	entries: make(map[string]routingUsageCacheEntry),
}

var routingHealthCacheState = struct {
	mu      sync.RWMutex
	entries map[string]routingHealthCacheEntry
}{
	entries: make(map[string]routingHealthCacheEntry),
}

func invalidateModelRouteCaches() {
	invalidateRoutingStaticSnapshot()
	invalidateRoutingRuntimeMetricCaches()
}

func invalidateRoutingStaticSnapshot() {
	routingStaticCacheState.mu.Lock()
	routingStaticCacheState.snapshot = nil
	routingStaticCacheState.expiresAt = time.Time{}
	routingStaticCacheState.mu.Unlock()
}

func invalidateRoutingRuntimeMetricCaches() {
	routingUsageCacheState.mu.Lock()
	routingUsageCacheState.entries = make(map[string]routingUsageCacheEntry)
	routingUsageCacheState.mu.Unlock()

	routingHealthCacheState.mu.Lock()
	routingHealthCacheState.entries = make(map[string]routingHealthCacheEntry)
	routingHealthCacheState.mu.Unlock()
}

func getRoutingStaticSnapshot() (*routingStaticSnapshot, error) {
	now := time.Now()

	routingStaticCacheState.mu.RLock()
	snapshot := routingStaticCacheState.snapshot
	expiresAt := routingStaticCacheState.expiresAt
	routingStaticCacheState.mu.RUnlock()

	if snapshot != nil && now.Before(expiresAt) {
		return snapshot, nil
	}

	routingStaticCacheState.mu.Lock()
	defer routingStaticCacheState.mu.Unlock()

	now = time.Now()
	if routingStaticCacheState.snapshot != nil && now.Before(routingStaticCacheState.expiresAt) {
		return routingStaticCacheState.snapshot, nil
	}

	freshSnapshot, err := buildRoutingStaticSnapshot()
	if err != nil {
		if routingStaticCacheState.snapshot != nil {
			return routingStaticCacheState.snapshot, nil
		}
		return nil, err
	}

	routingStaticCacheState.snapshot = freshSnapshot
	routingStaticCacheState.expiresAt = now.Add(routingStaticSnapshotTTL)
	return freshSnapshot, nil
}

func buildRoutingStaticSnapshot() (*routingStaticSnapshot, error) {
	snapshot := &routingStaticSnapshot{
		enabledRoutes:              make([]ModelRoute, 0),
		routesByExactModel:         make(map[string][]ModelRoute),
		routesByNormalizedModel:    make(map[string][]ModelRoute),
		routesByVersionAgnostic:    make(map[string][]ModelRoute),
		routesByProviderExactModel: make(map[int]map[string][]ModelRoute),
		routesByProviderNormModel:  make(map[int]map[string][]ModelRoute),
		providerAliasLookups:       make(map[int]providerModelAliasLookup),
		providerAliasReverseLookup: make(map[int]providerModelAliasReverseLookup),
		providersByID:              make(map[int]*Provider),
		tokensByID:                 make(map[int]*ProviderToken),
		pricingLookup:              make(map[string]ModelPricing),
		modelCatalog:               newModelCatalogSnapshot(),
	}

	var routes []ModelRoute
	if err := DB.Where("enabled = ?", true).
		Order("priority DESC, id ASC").Find(&routes).Error; err != nil {
		return nil, err
	}
	if len(routes) == 0 {
		return snapshot, nil
	}
	snapshot.enabledRoutes = routes

	providerSet := make(map[int]bool)
	providerIds := make([]int, 0)
	tokenSet := make(map[int]bool)
	tokenIds := make([]int, 0)
	modelSet := make(map[string]bool)
	modelNames := make([]string, 0)

	for _, route := range routes {
		if !providerSet[route.ProviderId] {
			providerSet[route.ProviderId] = true
			providerIds = append(providerIds, route.ProviderId)
		}
		if !tokenSet[route.ProviderTokenId] {
			tokenSet[route.ProviderTokenId] = true
			tokenIds = append(tokenIds, route.ProviderTokenId)
		}
		modelName := strings.TrimSpace(route.ModelName)
		if modelName != "" && !modelSet[modelName] {
			modelSet[modelName] = true
			modelNames = append(modelNames, modelName)
		}
	}

	if len(providerIds) > 0 {
		var providers []Provider
		if err := applyProviderReadProjection(DB).Where("id IN ?", providerIds).Find(&providers).Error; err != nil {
			return nil, err
		}
		for i := range providers {
			provider := &providers[i]
			snapshot.providersByID[provider.Id] = provider
			mapping := ParseProviderAliasMapping(provider.ModelAliasMapping)
			snapshot.providerAliasLookups[provider.Id] = buildProviderModelAliasLookup(mapping)
			snapshot.providerAliasReverseLookup[provider.Id] = buildProviderModelAliasReverseLookup(mapping)
		}
	}

	if len(tokenIds) > 0 {
		var tokens []ProviderToken
		if err := DB.Where("id IN ?", tokenIds).Find(&tokens).Error; err != nil {
			return nil, err
		}
		for i := range tokens {
			token := &tokens[i]
			snapshot.tokensByID[token.Id] = token
		}
	}

	if len(providerIds) > 0 && len(modelNames) > 0 {
		var pricings []ModelPricing
		if err := DB.Where("provider_id IN ? AND model_name IN ?", providerIds, modelNames).Find(&pricings).Error; err != nil {
			return nil, err
		}
		for _, pricing := range pricings {
			snapshot.pricingLookup[routePricingKey(pricing.ProviderId, pricing.ModelName)] = pricing
		}
	}

	for _, route := range routes {
		routeName := strings.TrimSpace(route.ModelName)
		if routeName == "" {
			continue
		}
		exactKey := strings.ToLower(routeName)
		snapshot.routesByExactModel[exactKey] = append(snapshot.routesByExactModel[exactKey], route)

		normalizedKey := common.NormalizeModelName(routeName)
		if normalizedKey != "" {
			snapshot.routesByNormalizedModel[normalizedKey] = append(snapshot.routesByNormalizedModel[normalizedKey], route)
			versionKey := common.ToVersionAgnosticKey(normalizedKey)
			if versionKey != "" {
				snapshot.routesByVersionAgnostic[versionKey] = append(snapshot.routesByVersionAgnostic[versionKey], route)
			}
		}

		if _, ok := snapshot.routesByProviderExactModel[route.ProviderId]; !ok {
			snapshot.routesByProviderExactModel[route.ProviderId] = make(map[string][]ModelRoute)
		}
		snapshot.routesByProviderExactModel[route.ProviderId][exactKey] = append(snapshot.routesByProviderExactModel[route.ProviderId][exactKey], route)

		if normalizedKey != "" {
			if _, ok := snapshot.routesByProviderNormModel[route.ProviderId]; !ok {
				snapshot.routesByProviderNormModel[route.ProviderId] = make(map[string][]ModelRoute)
			}
			snapshot.routesByProviderNormModel[route.ProviderId][normalizedKey] = append(snapshot.routesByProviderNormModel[route.ProviderId][normalizedKey], route)
		}
	}

	snapshot.modelCatalog = buildModelCatalogSnapshot(routes, snapshot.providerAliasReverseLookup)

	return snapshot, nil
}

func getCandidateRoutesByModelCached(requestedModel string) ([]ModelRoute, error) {
	snapshot, err := getRoutingStaticSnapshot()
	if err != nil {
		return nil, err
	}
	if snapshot == nil || len(snapshot.enabledRoutes) == 0 {
		return nil, nil
	}

	requestedModel = strings.TrimSpace(requestedModel)
	if requestedModel == "" {
		return nil, nil
	}

	candidateMap := make(map[int]ModelRoute)
	appendCandidates := func(routes []ModelRoute) {
		for _, route := range routes {
			candidateMap[route.Id] = route
		}
	}

	requestedExact := strings.ToLower(requestedModel)
	if requestedExact != "" {
		appendCandidates(snapshot.routesByExactModel[requestedExact])
	}

	requestedNormalized := common.NormalizeModelName(requestedModel)
	if requestedNormalized != "" {
		appendCandidates(snapshot.routesByNormalizedModel[requestedNormalized])
		requestedVersionKey := common.ToVersionAgnosticKey(requestedNormalized)
		if requestedVersionKey != "" {
			appendCandidates(snapshot.routesByVersionAgnostic[requestedVersionKey])
		}
	}

	if entry, ok := snapshot.modelCatalog.resolve(requestedModel); ok {
		for _, target := range entry.RouteTargets {
			target = strings.TrimSpace(target)
			if target == "" {
				continue
			}
			appendCandidates(snapshot.routesByExactModel[strings.ToLower(target)])

			normalizedTarget := common.NormalizeModelName(target)
			if normalizedTarget == "" {
				continue
			}
			appendCandidates(snapshot.routesByNormalizedModel[normalizedTarget])
			versionKey := common.ToVersionAgnosticKey(normalizedTarget)
			if versionKey != "" {
				appendCandidates(snapshot.routesByVersionAgnostic[versionKey])
			}
		}
	}

	for providerID, lookup := range snapshot.providerAliasLookups {
		mappedModel, ok := lookup.Resolve(requestedModel)
		if !ok {
			continue
		}
		mappedModel = strings.TrimSpace(mappedModel)
		if mappedModel == "" {
			continue
		}

		mappedExact := strings.ToLower(mappedModel)
		if mappedExact != "" {
			if routesByModel, ok := snapshot.routesByProviderExactModel[providerID]; ok {
				appendCandidates(routesByModel[mappedExact])
			}
		}

		mappedNormalized := common.NormalizeModelName(mappedModel)
		if mappedNormalized != "" {
			if routesByModel, ok := snapshot.routesByProviderNormModel[providerID]; ok {
				appendCandidates(routesByModel[mappedNormalized])
			}
		}
	}

	if len(candidateMap) == 0 {
		return nil, nil
	}

	candidates := make([]ModelRoute, 0, len(candidateMap))
	for _, route := range candidateMap {
		candidates = append(candidates, route)
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Priority == candidates[j].Priority {
			return candidates[i].Id < candidates[j].Id
		}
		return candidates[i].Priority > candidates[j].Priority
	})
	return candidates, nil
}

func loadPricingLookupByProviderModels(providerIds []int, modelNames []string) (map[string]ModelPricing, error) {
	lookup := make(map[string]ModelPricing)
	if len(providerIds) == 0 || len(modelNames) == 0 {
		return lookup, nil
	}

	snapshot, err := getRoutingStaticSnapshot()
	if err == nil && snapshot != nil {
		providerSet := make(map[int]bool, len(providerIds))
		for _, providerId := range providerIds {
			providerSet[providerId] = true
		}
		modelSet := make(map[string]bool, len(modelNames))
		for _, modelName := range modelNames {
			trimmed := strings.TrimSpace(modelName)
			if trimmed != "" {
				modelSet[trimmed] = true
			}
		}
		for key, pricing := range snapshot.pricingLookup {
			if !providerSet[pricing.ProviderId] {
				continue
			}
			if !modelSet[strings.TrimSpace(pricing.ModelName)] {
				continue
			}
			lookup[key] = pricing
		}
		return lookup, nil
	}

	var pricings []ModelPricing
	if err := DB.Where("provider_id IN ? AND model_name IN ?", providerIds, modelNames).Find(&pricings).Error; err != nil {
		return nil, err
	}
	for _, pricing := range pricings {
		lookup[routePricingKey(pricing.ProviderId, pricing.ModelName)] = pricing
	}
	return lookup, nil
}

func getCachedRecentUsageCostByTokenModel(tokenIds []int, modelNames []string, usageWindowHours int) (map[string]float64, bool) {
	cacheKey := buildRoutingMetricCacheKey(tokenIds, modelNames, usageWindowHours)
	if cacheKey == "" {
		return nil, false
	}

	now := time.Now()
	routingUsageCacheState.mu.RLock()
	entry, ok := routingUsageCacheState.entries[cacheKey]
	routingUsageCacheState.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if now.After(entry.expiresAt) {
		routingUsageCacheState.mu.Lock()
		delete(routingUsageCacheState.entries, cacheKey)
		routingUsageCacheState.mu.Unlock()
		return nil, false
	}
	return cloneUsageLookup(entry.data), true
}

func setCachedRecentUsageCostByTokenModel(tokenIds []int, modelNames []string, usageWindowHours int, usageLookup map[string]float64) {
	cacheKey := buildRoutingMetricCacheKey(tokenIds, modelNames, usageWindowHours)
	if cacheKey == "" {
		return
	}

	routingUsageCacheState.mu.Lock()
	defer routingUsageCacheState.mu.Unlock()

	now := time.Now()
	cleanupExpiredUsageEntriesLocked(now)
	evictUsageEntriesIfNeededLocked()
	routingUsageCacheState.entries[cacheKey] = routingUsageCacheEntry{
		expiresAt: now.Add(routingRuntimeMetricTTL),
		data:      cloneUsageLookup(usageLookup),
	}
}

func getCachedRouteHealthStatsByTokenModel(tokenIds []int, modelNames []string, windowHours int) (map[string]routeHealthStats, bool) {
	cacheKey := buildRoutingMetricCacheKey(tokenIds, modelNames, windowHours)
	if cacheKey == "" {
		return nil, false
	}

	now := time.Now()
	routingHealthCacheState.mu.RLock()
	entry, ok := routingHealthCacheState.entries[cacheKey]
	routingHealthCacheState.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if now.After(entry.expiresAt) {
		routingHealthCacheState.mu.Lock()
		delete(routingHealthCacheState.entries, cacheKey)
		routingHealthCacheState.mu.Unlock()
		return nil, false
	}
	return cloneHealthLookup(entry.data), true
}

func setCachedRouteHealthStatsByTokenModel(tokenIds []int, modelNames []string, windowHours int, statsLookup map[string]routeHealthStats) {
	cacheKey := buildRoutingMetricCacheKey(tokenIds, modelNames, windowHours)
	if cacheKey == "" {
		return
	}

	routingHealthCacheState.mu.Lock()
	defer routingHealthCacheState.mu.Unlock()

	now := time.Now()
	cleanupExpiredHealthEntriesLocked(now)
	evictHealthEntriesIfNeededLocked()
	routingHealthCacheState.entries[cacheKey] = routingHealthCacheEntry{
		expiresAt: now.Add(routingRuntimeMetricTTL),
		data:      cloneHealthLookup(statsLookup),
	}
}

func cleanupExpiredUsageEntriesLocked(now time.Time) {
	for key, entry := range routingUsageCacheState.entries {
		if now.After(entry.expiresAt) {
			delete(routingUsageCacheState.entries, key)
		}
	}
}

func cleanupExpiredHealthEntriesLocked(now time.Time) {
	for key, entry := range routingHealthCacheState.entries {
		if now.After(entry.expiresAt) {
			delete(routingHealthCacheState.entries, key)
		}
	}
}

func evictUsageEntriesIfNeededLocked() {
	for len(routingUsageCacheState.entries) >= routingRuntimeMetricMaxKey {
		for key := range routingUsageCacheState.entries {
			delete(routingUsageCacheState.entries, key)
			break
		}
	}
}

func evictHealthEntriesIfNeededLocked() {
	for len(routingHealthCacheState.entries) >= routingRuntimeMetricMaxKey {
		for key := range routingHealthCacheState.entries {
			delete(routingHealthCacheState.entries, key)
			break
		}
	}
}

func buildRoutingMetricCacheKey(tokenIds []int, modelNames []string, windowHours int) string {
	tokenList := uniqueSortedInts(tokenIds)
	modelList := uniqueSortedStrings(modelNames)
	if len(tokenList) == 0 || len(modelList) == 0 {
		return ""
	}
	tokenParts := make([]string, 0, len(tokenList))
	for _, tokenId := range tokenList {
		tokenParts = append(tokenParts, strconv.Itoa(tokenId))
	}

	return strconv.Itoa(windowHours) + "|" + strings.Join(tokenParts, ",") + "|" + strings.Join(modelList, ",")
}

func uniqueSortedInts(values []int) []int {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[int]bool, len(values))
	result := make([]int, 0, len(values))
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	sort.Ints(result)
	return result
}

func uniqueSortedStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		result = append(result, trimmed)
	}
	sort.Strings(result)
	return result
}

func cloneUsageLookup(source map[string]float64) map[string]float64 {
	copied := make(map[string]float64, len(source))
	for key, value := range source {
		copied[key] = value
	}
	return copied
}

func cloneHealthLookup(source map[string]routeHealthStats) map[string]routeHealthStats {
	copied := make(map[string]routeHealthStats, len(source))
	for key, value := range source {
		copied[key] = value
	}
	return copied
}
