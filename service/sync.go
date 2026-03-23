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

type ProviderTokenReconcileResult struct {
	UpstreamToken              UpstreamToken
	ProviderToken              *model.ProviderToken
	ExistingLocalKeyPreserved  bool
	RecoveredFrom              string
}

type ProviderTokenSyncSummary struct {
	ResultsByUpstreamID map[int]*ProviderTokenReconcileResult
	Total              int
	ReadyCount         int
	UnresolvedCount    int
}

type ProviderTokenCreateOutcome struct {
	UpstreamCreated        bool   `json:"upstream_created"`
	CreatedTokenIdentified bool   `json:"created_token_identified"`
	ProviderTokenId        int    `json:"provider_token_id,omitempty"`
	UpstreamTokenId        int    `json:"upstream_token_id,omitempty"`
	KeyStatus              string `json:"key_status,omitempty"`
	KeyUnresolvedReason    string `json:"key_unresolved_reason,omitempty"`
}

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

func fetchAllUpstreamTokens(client *UpstreamClient) ([]UpstreamToken, error) {
	var allTokens []UpstreamToken
	seenTokenIDs := make(map[int]struct{})
	page := 0
	pageSize := 100
	for {
		tokenPage, err := client.GetTokens(page, pageSize)
		if err != nil {
			return nil, err
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
	return allTokens, nil
}

func buildProviderTokenSnapshot(provider *model.Provider, upstreamToken UpstreamToken) *model.ProviderToken {
	return &model.ProviderToken{
		ProviderId:      provider.Id,
		UpstreamTokenId: upstreamToken.Id,
		Name:            upstreamToken.Name,
		GroupName:       upstreamToken.Group,
		Status:          upstreamToken.Status,
		Priority:        provider.Priority,
		Weight:          provider.Weight,
		RemainQuota:     upstreamToken.RemainQuota,
		UnlimitedQuota:  upstreamToken.UnlimitedQuota,
		UsedQuota:       upstreamToken.UsedQuota,
		ModelLimits:     upstreamToken.ModelLimits,
		LastSynced:      time.Now().Unix(),
	}
}

func unresolvedReasonForExisting(existing *model.ProviderToken) string {
	if existing == nil {
		return model.ProviderTokenKeyUnresolvedReasonPlaintextNotRecovered
	}
	if strings.TrimSpace(existing.KeyUnresolvedReason) != "" {
		return existing.KeyUnresolvedReason
	}
	if model.ClassifyProviderTokenKeyMaterial(existing.SkKey) == model.ProviderTokenKeyMaterialMasked {
		return model.ProviderTokenKeyUnresolvedReasonLegacyContaminated
	}
	return model.ProviderTokenKeyUnresolvedReasonPlaintextNotRecovered
}

func reconcileProviderToken(client *UpstreamClient, provider *model.Provider, upstreamToken UpstreamToken) (*ProviderTokenReconcileResult, error) {
	existing, err := model.GetProviderTokenByUpstream(provider.Id, upstreamToken.Id)
	if err != nil {
		return nil, err
	}

	localToken := buildProviderTokenSnapshot(provider, upstreamToken)
	result := &ProviderTokenReconcileResult{
		UpstreamToken: upstreamToken,
		ProviderToken: localToken,
	}

	if model.IsUsableProviderTokenKey(upstreamToken.Key) {
		localToken.MarkKeyReady(upstreamToken.Key)
		result.RecoveredFrom = "list"
		return result, nil
	}

	detailToken, detailErr := client.GetTokenDetail(upstreamToken.Id)
	if detailErr == nil && detailToken != nil && model.IsUsableProviderTokenKey(detailToken.Key) {
		localToken.MarkKeyReady(detailToken.Key)
		result.RecoveredFrom = "detail"
		return result, nil
	}

	if localToken.PreserveReadyKeyFrom(existing) {
		result.ExistingLocalKeyPreserved = true
		result.RecoveredFrom = "local_preserved"
		return result, nil
	}

	localToken.MarkKeyUnresolved(unresolvedReasonForExisting(existing))
	result.RecoveredFrom = "unresolved"
	return result, nil
}

func reconcileProviderTokens(client *UpstreamClient, provider *model.Provider, allTokens []UpstreamToken) (*ProviderTokenSyncSummary, error) {
	summary := &ProviderTokenSyncSummary{
		ResultsByUpstreamID: make(map[int]*ProviderTokenReconcileResult, len(allTokens)),
	}
	upstreamIds := make([]int, 0, len(allTokens))

	for _, upstreamToken := range allTokens {
		upstreamIds = append(upstreamIds, upstreamToken.Id)
		reconcileResult, err := reconcileProviderToken(client, provider, upstreamToken)
		if err != nil {
			return nil, err
		}
		if err := model.UpsertProviderToken(reconcileResult.ProviderToken); err != nil {
			return nil, err
		}
		summary.ResultsByUpstreamID[upstreamToken.Id] = reconcileResult
		summary.Total++
		if reconcileResult.ProviderToken.IsKeyReady() {
			summary.ReadyCount++
		} else {
			summary.UnresolvedCount++
		}
	}

	if err := model.DeleteProviderTokensNotInIds(provider.Id, upstreamIds); err != nil {
		common.SysLog(fmt.Sprintf("cleanup old tokens failed for provider %s: %v", provider.Name, err))
	}

	return summary, nil
}

func SyncProviderTokensOnly(provider *model.Provider) (*ProviderTokenSyncSummary, error) {
	client, err := newUpstreamClientForProvider(provider)
	if err != nil {
		return nil, err
	}
	allTokens, err := fetchAllUpstreamTokens(client)
	if err != nil {
		return nil, err
	}
	summary, err := reconcileProviderTokens(client, provider, allTokens)
	if err != nil {
		return nil, err
	}
	if err := rebuildProviderRoutesForProvider(provider.Id); err != nil {
		common.SysLog(fmt.Sprintf("rebuild routes failed for provider %s after token-only sync: %v", provider.Name, err))
	}
	return summary, nil
}

func identifyCreatedUpstreamTokenID(beforeTokens []UpstreamToken, summary *ProviderTokenSyncSummary, reqName string, reqGroup string, reqUnlimitedQuota bool, reqRemainQuota int64, reqModelLimits string, createdResult *UpstreamTokenCreateResult) int {
	if summary == nil {
		return 0
	}
	beforeSet := make(map[int]struct{}, len(beforeTokens))
	for _, token := range beforeTokens {
		beforeSet[token.Id] = struct{}{}
	}
	candidates := make([]int, 0)
	for upstreamID, result := range summary.ResultsByUpstreamID {
		if _, existed := beforeSet[upstreamID]; existed {
			continue
		}
		candidates = append(candidates, upstreamID)
		_ = result
	}
	if len(candidates) == 1 {
		return candidates[0]
	}
	if len(candidates) > 1 {
		trimmedName := strings.TrimSpace(reqName)
		trimmedGroup := strings.TrimSpace(reqGroup)
		trimmedModelLimits := strings.TrimSpace(reqModelLimits)
		filtered := make([]int, 0, len(candidates))
		for _, candidateID := range candidates {
			result := summary.ResultsByUpstreamID[candidateID]
			if result == nil {
				continue
			}
			upstream := result.UpstreamToken
			if trimmedName != "" && strings.TrimSpace(upstream.Name) != trimmedName {
				continue
			}
			if trimmedGroup != "" && strings.TrimSpace(upstream.Group) != trimmedGroup {
				continue
			}
			if upstream.UnlimitedQuota != reqUnlimitedQuota {
				continue
			}
			if !reqUnlimitedQuota && upstream.RemainQuota != reqRemainQuota {
				continue
			}
			if strings.TrimSpace(upstream.ModelLimits) != trimmedModelLimits {
				continue
			}
			filtered = append(filtered, candidateID)
		}
		if len(filtered) == 1 {
			return filtered[0]
		}
	}
	if createdResult != nil && createdResult.Token != nil && createdResult.Token.Id > 0 {
		if _, ok := summary.ResultsByUpstreamID[createdResult.Token.Id]; ok {
			return createdResult.Token.Id
		}
	}
	return 0
}

func CreateProviderTokenWithReconciliation(provider *model.Provider, name string, group string, unlimitedQuota bool, remainQuota int64, modelLimits string) (*ProviderTokenCreateOutcome, error) {
	client, err := newUpstreamClientForProvider(provider)
	if err != nil {
		return nil, err
	}
	beforeTokens, err := fetchAllUpstreamTokens(client)
	if err != nil {
		return nil, err
	}
	createdResult, err := client.CreateUpstreamToken(name, group, unlimitedQuota, remainQuota, modelLimits)
	if err != nil {
		return nil, err
	}
	summary, err := SyncProviderTokensOnly(provider)
	if err != nil {
		return nil, err
	}
	outcome := &ProviderTokenCreateOutcome{UpstreamCreated: true}
	identifiedUpstreamTokenID := identifyCreatedUpstreamTokenID(beforeTokens, summary, name, group, unlimitedQuota, remainQuota, modelLimits, createdResult)
	if identifiedUpstreamTokenID == 0 {
		outcome.KeyStatus = model.ProviderTokenKeyStatusUnresolved
		outcome.KeyUnresolvedReason = model.ProviderTokenKeyUnresolvedReasonCreatedNotIdentified
		return outcome, nil
	}
	outcome.CreatedTokenIdentified = true
	outcome.UpstreamTokenId = identifiedUpstreamTokenID
	if result := summary.ResultsByUpstreamID[identifiedUpstreamTokenID]; result != nil && result.ProviderToken != nil {
		outcome.ProviderTokenId = result.ProviderToken.Id
		outcome.KeyStatus = result.ProviderToken.KeyStatus
		outcome.KeyUnresolvedReason = result.ProviderToken.KeyUnresolvedReason
		if result.ProviderToken.IsKeyReady() {
			outcome.KeyStatus = model.ProviderTokenKeyStatusReady
			outcome.KeyUnresolvedReason = ""
		}
	}
	if strings.TrimSpace(outcome.KeyStatus) == "" {
		outcome.KeyStatus = model.ProviderTokenKeyStatusUnresolved
	}
	if outcome.KeyStatus == model.ProviderTokenKeyStatusUnresolved && strings.TrimSpace(outcome.KeyUnresolvedReason) == "" {
		outcome.KeyUnresolvedReason = model.ProviderTokenKeyUnresolvedReasonPlaintextNotRecovered
	}
	return outcome, nil
}

func syncTokens(client *UpstreamClient, provider *model.Provider) error {
	allTokens, err := fetchAllUpstreamTokens(client)
	if err != nil {
		return err
	}
	summary, err := reconcileProviderTokens(client, provider, allTokens)
	if err != nil {
		return err
	}
	if summary.UnresolvedCount > 0 {
		common.SysLog(fmt.Sprintf("synced %d tokens for provider %s (%d ready, %d unresolved)", summary.Total, provider.Name, summary.ReadyCount, summary.UnresolvedCount))
		return nil
	}
	common.SysLog(fmt.Sprintf("synced %d tokens for provider %s", summary.Total, provider.Name))
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
