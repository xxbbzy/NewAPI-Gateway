package service

import (
	"NewAPI-Gateway/common"
	"NewAPI-Gateway/model"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// SyncProvider synchronizes pricing, tokens, and balance from an upstream provider
func SyncProvider(provider *model.Provider) error {
	client := NewUpstreamClient(provider.BaseURL, provider.AccessToken, provider.UserID)

	// 1. Sync pricing
	if err := syncPricing(client, provider); err != nil {
		common.SysLog(fmt.Sprintf("sync pricing failed for provider %s: %v", provider.Name, err))
	}

	// 2. Sync tokens
	if err := syncTokens(client, provider); err != nil {
		common.SysLog(fmt.Sprintf("sync tokens failed for provider %s: %v", provider.Name, err))
	}

	// 3. Sync balance
	if err := syncBalance(client, provider); err != nil {
		common.SysLog(fmt.Sprintf("sync balance failed for provider %s: %v", provider.Name, err))
	}

	// 4. Rebuild routes for this provider
	if err := RebuildProviderRoutes(provider.Id); err != nil {
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
	page := 1
	pageSize := 100
	for {
		tokens, err := client.GetTokens(page, pageSize)
		if err != nil {
			return err
		}
		allTokens = append(allTokens, tokens...)
		if len(tokens) < pageSize {
			break
		}
		page++
	}

	// Upsert each token
	var upstreamIds []int
	for _, t := range allTokens {
		upstreamIds = append(upstreamIds, t.Id)
		pt := &model.ProviderToken{
			ProviderId:      provider.Id,
			UpstreamTokenId: t.Id,
			SkKey:           "sk-" + t.Key,
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
