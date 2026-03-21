package service

import (
	"NewAPI-Gateway/common"
	"NewAPI-Gateway/model"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

var newUpstreamClientForProvider = NewUpstreamClientForProvider
var syncPricingStep = syncPricing
var syncTokensStep = syncTokens
var syncBalanceStep = syncBalance
var rebuildProviderRoutesForProvider = RebuildProviderRoutes

// SyncProvider synchronizes pricing, tokens, and balance from an upstream provider
func SyncProvider(provider *model.Provider) error {
	client, err := newUpstreamClientForProvider(provider)
	if err != nil {
		common.SysLog(fmt.Sprintf("build upstream client failed for provider %s: %v", provider.Name, err))
		if reason, ok := classifyProviderReachabilityError(err); ok {
			markProviderHealthFailure(provider, reason)
		}
		return err
	}

	var healthObservationRecorded bool
	var healthReachable bool
	var healthFailureReason string
	recordReachabilityFailure := func(err error) {
		reason, ok := classifyProviderReachabilityError(err)
		if !ok {
			return
		}
		healthObservationRecorded = true
		healthReachable = false
		healthFailureReason = reason
	}
	recordReachabilitySuccess := func() {
		healthObservationRecorded = true
		healthReachable = true
		healthFailureReason = ""
	}

	// 1. Sync pricing
	if err := syncPricingStep(client, provider); err != nil {
		common.SysLog(fmt.Sprintf("sync pricing failed for provider %s: %v", provider.Name, err))
		recordReachabilityFailure(err)
	} else {
		recordReachabilitySuccess()
	}

	// 2. Sync tokens
	if err := syncTokensStep(client, provider); err != nil {
		common.SysLog(fmt.Sprintf("sync tokens failed for provider %s: %v", provider.Name, err))
		recordReachabilityFailure(err)
	} else {
		recordReachabilitySuccess()
	}

	// 3. Sync balance
	if err := syncBalanceStep(client, provider); err != nil {
		common.SysLog(fmt.Sprintf("sync balance failed for provider %s: %v", provider.Name, err))
		recordReachabilityFailure(err)
	} else {
		recordReachabilitySuccess()
	}

	if healthObservationRecorded {
		if healthReachable {
			markProviderHealthSuccess(provider)
		} else {
			markProviderHealthFailure(provider, healthFailureReason)
		}
	}

	// 4. Rebuild routes for this provider
	if err := rebuildProviderRoutesForProvider(provider.Id); err != nil {
		common.SysLog(fmt.Sprintf("rebuild routes failed for provider %s: %v", provider.Name, err))
	}

	return nil
}

func syncPricing(client *UpstreamClient, provider *model.Provider) error {
	pricingPayload, err := client.GetPricing()
	if err != nil {
		return err
	}
	pricingList := pricingPayload.Data

	for _, p := range pricingList {
		enableGroupsJSON, _ := json.Marshal(p.EnableGroups)
		supportedEndpointTypesJSON, _ := json.Marshal(p.SupportedEndpointTypes)
		mp := &model.ModelPricing{
			ModelName:              p.ModelName,
			ProviderId:             provider.Id,
			QuotaType:              p.QuotaType,
			ModelRatio:             p.ModelRatio,
			CompletionRatio:        p.CompletionRatio,
			ModelPrice:             p.ModelPrice,
			EnableGroups:           string(enableGroupsJSON),
			SupportedEndpointTypes: string(supportedEndpointTypesJSON),
			LastSynced:             time.Now().Unix(),
		}
		if err := model.UpsertModelPricing(mp); err != nil {
			common.SysLog(fmt.Sprintf("upsert pricing failed for model %s: %v", p.ModelName, err))
		}
	}

	groupRatioJSON, _ := json.Marshal(pricingPayload.GroupRatio)
	provider.UpdatePricingGroupRatio(string(groupRatioJSON))
	usableGroupJSON, _ := json.Marshal(pricingPayload.UsableGroup)
	provider.UpdatePricingUsableGroup(string(usableGroupJSON))
	supportedEndpointJSON, _ := json.Marshal(pricingPayload.SupportedEndpoint)
	provider.UpdatePricingSupportedEndpoint(string(supportedEndpointJSON))

	common.SysLog(fmt.Sprintf("synced %d pricing records for provider %s", len(pricingList), provider.Name))
	return nil
}

func syncTokens(client *UpstreamClient, provider *model.Provider) error {
	// Fetch all tokens (paginate)
	var allTokens []UpstreamToken
	seenTokenIDs := make(map[int]struct{})
	page := 0
	pageSize := 100
	for {
		tokenPage, err := client.GetTokens(page, pageSize)
		if err != nil {
			return err
		}
		tokens := tokenPage.Items
		for _, t := range tokens {
			if _, exists := seenTokenIDs[t.Id]; exists {
				continue
			}
			seenTokenIDs[t.Id] = struct{}{}
			allTokens = append(allTokens, t)
		}
		if len(tokens) == 0 {
			break
		}
		if tokenPage.Total > 0 {
			if len(allTokens) >= tokenPage.Total {
				break
			}
		} else if len(tokens) < tokenPage.PageSize {
			break
		}
		page++
	}

	// Upsert each token
	var upstreamIds []int
	for _, t := range allTokens {
		upstreamIds = append(upstreamIds, t.Id)

		// Resolve the real sk_key.
		// Upstream variants:
		//   a) Full key returned (ideal)
		//   b) Key masked with ** (some forks)
		//   c) Key emptied by Clean() (some forks set key="")
		//   d) Key already prefixed with sk- (some forks)
		rawKey := strings.TrimPrefix(t.Key, "sk-") // strip if upstream includes prefix
		needsFetch := rawKey == "" || model.IsMaskedKey(rawKey)

		if needsFetch {
			resolved := false
			// Try GET /api/token/:id/key
			fullKey, err := client.GetTokenKey(t.Id)
			if err == nil && fullKey != "" && !model.IsMaskedKey(fullKey) {
				rawKey = strings.TrimPrefix(fullKey, "sk-")
				resolved = true
			}
			if !resolved {
				// Try GET /api/token/:id (single token detail may return full key)
				detailKey, err := client.GetTokenDetail(t.Id)
				if err == nil && detailKey != "" && !model.IsMaskedKey(detailKey) {
					rawKey = strings.TrimPrefix(detailKey, "sk-")
					resolved = true
				}
			}
			if !resolved {
				// Fallback: preserve existing local key
				existing, _ := model.GetProviderTokenByUpstream(provider.Id, t.Id)
				if existing != nil && len(existing.SkKey) > 3 && !model.IsMaskedKey(existing.SkKey) {
					rawKey = strings.TrimPrefix(existing.SkKey, "sk-")
					resolved = true
				}
			}
			if !resolved {
				common.SysLog(fmt.Sprintf("[sync-token] WARNING: could not resolve full key for upstream token %d of provider %s (upstream_key=%q), skipping sk_key update", t.Id, provider.Name, t.Key))
				// Skip this token's sk_key update — only update metadata
				existing, _ := model.GetProviderTokenByUpstream(provider.Id, t.Id)
				if existing != nil {
					existing.Name = t.Name
					existing.GroupName = t.Group
					existing.Status = t.Status
					existing.Priority = provider.Priority
					existing.Weight = provider.Weight
					existing.RemainQuota = t.RemainQuota
					existing.UnlimitedQuota = t.UnlimitedQuota
					existing.UsedQuota = t.UsedQuota
					existing.ModelLimits = t.ModelLimits
					existing.LastSynced = time.Now().Unix()
					_ = existing.Update()
				}
				continue
			}
		}

		skKey := "sk-" + rawKey
		pt := &model.ProviderToken{
			ProviderId:      provider.Id,
			UpstreamTokenId: t.Id,
			SkKey:           skKey,
			Name:            t.Name,
			GroupName:       t.Group,
			Status:          t.Status,
			Priority:        provider.Priority,
			Weight:          provider.Weight,
			RemainQuota:     t.RemainQuota,
			UnlimitedQuota:  t.UnlimitedQuota,
			UsedQuota:       t.UsedQuota,
			ModelLimits:     t.ModelLimits,
			LastSynced:      time.Now().Unix(),
		}
		if err := model.UpsertProviderToken(pt); err != nil {
			common.SysLog(fmt.Sprintf("upsert token failed for upstream token %d: %v", t.Id, err))
		}
	}

	// Delete tokens that no longer exist upstream
	if err := model.DeleteProviderTokensNotInIds(provider.Id, upstreamIds); err != nil {
		common.SysLog(fmt.Sprintf("cleanup old tokens failed for provider %s: %v", provider.Name, err))
	}

	common.SysLog(fmt.Sprintf("synced %d tokens for provider %s", len(allTokens), provider.Name))
	return nil
}

func syncBalance(client *UpstreamClient, provider *model.Provider) error {
	userSelf, err := client.GetUserSelf()
	if err != nil {
		return err
	}
	balanceUSD := float64(userSelf.Balance) / 500000.0
	provider.UpdateBalance(fmt.Sprintf("$%.2f", balanceUSD))
	return nil
}

// RebuildProviderRoutes rebuilds model routes for a specific provider
// Logic: for each provider_token, find its group_name, then find all models
// whose pricing.enable_groups contains that group_name → create route entries
func RebuildProviderRoutes(providerId int) error {
	tokens, err := model.GetEnabledProviderTokensByProviderId(providerId)
	if err != nil {
		return err
	}

	pricing, err := model.GetModelPricingByProvider(providerId)
	if err != nil {
		return err
	}

	// Build group → models mapping from pricing
	groupModels := make(map[string][]string)
	for _, p := range pricing {
		var groups []string
		if err := json.Unmarshal([]byte(p.EnableGroups), &groups); err != nil {
			continue
		}
		for _, g := range groups {
			groupModels[g] = append(groupModels[g], p.ModelName)
		}
	}

	// Generate routes
	var routes []model.ModelRoute
	routeSet := make(map[string]bool) // deduplicate
	for _, token := range tokens {
		models, ok := groupModels[token.GroupName]
		if !ok {
			continue
		}
		// If token has model_limits, filter
		var allowedModels map[string]bool
		if token.ModelLimits != "" {
			allowedModels = make(map[string]bool)
			for _, m := range strings.Split(token.ModelLimits, ",") {
				allowedModels[strings.TrimSpace(m)] = true
			}
		}

		for _, modelName := range models {
			if allowedModels != nil && !allowedModels[modelName] {
				continue
			}
			key := fmt.Sprintf("%s|%d", modelName, token.Id)
			if routeSet[key] {
				continue
			}
			routeSet[key] = true
			routes = append(routes, model.ModelRoute{
				ModelName:       modelName,
				ProviderTokenId: token.Id,
				ProviderId:      providerId,
				Enabled:         true,
				Priority:        token.Priority,
				Weight:          token.Weight,
			})
		}
	}

	if err := model.RebuildRoutesForProvider(providerId, routes); err != nil {
		return err
	}

	common.SysLog(fmt.Sprintf("rebuilt %d routes for provider %d", len(routes), providerId))
	return nil
}

// RebuildAllRoutes rebuilds routes for all enabled providers
func RebuildAllRoutes() error {
	providers, err := model.GetEnabledProviders()
	if err != nil {
		return err
	}
	for _, p := range providers {
		if err := RebuildProviderRoutes(p.Id); err != nil {
			common.SysLog(fmt.Sprintf("rebuild routes failed for provider %s: %v", p.Name, err))
		}
	}
	return nil
}
