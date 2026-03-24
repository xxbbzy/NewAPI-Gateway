package model

import (
	"errors"
	"strings"
	"time"
)

type ProviderTokenKeyMaterial string

const (
	ProviderTokenKeyStatusReady      = "ready"
	ProviderTokenKeyStatusUnresolved = "unresolved"

	ProviderTokenKeyUnresolvedReasonPlaintextNotRecovered    = "plaintext_not_recovered"
	ProviderTokenKeyUnresolvedReasonLegacyContaminated       = "legacy_contaminated"
	ProviderTokenKeyUnresolvedReasonCreatedNotIdentified     = "created_token_not_identified"
	ProviderTokenKeyUnresolvedReasonKeyEndpointUnavailable   = "key_endpoint_unavailable"
	ProviderTokenKeyUnresolvedReasonKeyEndpointUnauthorized  = "key_endpoint_unauthorized"
	ProviderTokenKeyUnresolvedReasonKeyEndpointRequestFailed = "key_endpoint_request_failed"

	ProviderTokenKeyMaterialEmpty  ProviderTokenKeyMaterial = "empty"
	ProviderTokenKeyMaterialMasked ProviderTokenKeyMaterial = "masked"
	ProviderTokenKeyMaterialUsable ProviderTokenKeyMaterial = "usable"
)

type ProviderToken struct {
	Id                  int    `json:"id"`
	ProviderId          int    `json:"provider_id" gorm:"index;not null"`
	UpstreamTokenId     int    `json:"upstream_token_id"`
	SkKey               string `json:"sk_key" gorm:"type:varchar(256)"`
	KeyStatus           string `json:"key_status" gorm:"type:varchar(32);default:'ready';index"`
	KeyUnresolvedReason string `json:"key_unresolved_reason" gorm:"type:varchar(255)"`
	Name                string `json:"name"`
	GroupName           string `json:"group_name" gorm:"type:varchar(64);index"`
	Status              int    `json:"status" gorm:"default:1"`
	Priority            int    `json:"priority" gorm:"default:0"`
	Weight              int    `json:"weight" gorm:"default:10"`
	RemainQuota         int64  `json:"remain_quota"`
	UnlimitedQuota      bool   `json:"unlimited_quota"`
	UsedQuota           int64  `json:"used_quota"`
	ModelLimits         string `json:"model_limits" gorm:"type:varchar(2048)"`
	LastSynced          int64  `json:"last_synced"`
	CreatedAt           int64  `json:"created_at"`
}

func NormalizeProviderTokenKeyStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case ProviderTokenKeyStatusReady:
		return ProviderTokenKeyStatusReady
	case ProviderTokenKeyStatusUnresolved:
		return ProviderTokenKeyStatusUnresolved
	default:
		return ""
	}
}

func NormalizeProviderTokenKeyUnresolvedReason(reason string) string {
	return strings.TrimSpace(reason)
}

// IsMaskedKey returns true if the key contains consecutive asterisks (upstream masked).
func IsMaskedKey(key string) bool {
	return strings.Contains(strings.TrimSpace(key), "**")
}

func ClassifyProviderTokenKeyMaterial(key string) ProviderTokenKeyMaterial {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return ProviderTokenKeyMaterialEmpty
	}
	if IsMaskedKey(trimmed) {
		return ProviderTokenKeyMaterialMasked
	}
	raw := strings.TrimPrefix(trimmed, "sk-")
	if raw == "" {
		return ProviderTokenKeyMaterialEmpty
	}
	if IsMaskedKey(raw) {
		return ProviderTokenKeyMaterialMasked
	}
	return ProviderTokenKeyMaterialUsable
}

func IsUsableProviderTokenKey(key string) bool {
	return ClassifyProviderTokenKeyMaterial(key) == ProviderTokenKeyMaterialUsable
}

func NormalizeProviderTokenKeyForStorage(key string) string {
	trimmed := strings.TrimSpace(key)
	if !IsUsableProviderTokenKey(trimmed) {
		return ""
	}
	return "sk-" + strings.TrimPrefix(trimmed, "sk-")
}

func defaultProviderTokenUnresolvedReason(reason string, key string) string {
	normalized := NormalizeProviderTokenKeyUnresolvedReason(reason)
	if normalized != "" {
		return normalized
	}
	if ClassifyProviderTokenKeyMaterial(key) == ProviderTokenKeyMaterialMasked {
		return ProviderTokenKeyUnresolvedReasonLegacyContaminated
	}
	return ProviderTokenKeyUnresolvedReasonPlaintextNotRecovered
}

func ResolveProviderTokenPersistenceState(skKey string, keyStatus string, unresolvedReason string) (string, string, string) {
	normalizedKey := NormalizeProviderTokenKeyForStorage(skKey)
	if normalizedKey != "" {
		return normalizedKey, ProviderTokenKeyStatusReady, ""
	}
	_ = NormalizeProviderTokenKeyStatus(keyStatus)
	return "", ProviderTokenKeyStatusUnresolved, defaultProviderTokenUnresolvedReason(unresolvedReason, skKey)
}

func (pt *ProviderToken) resolvedPersistenceState() (string, string, string) {
	if pt == nil {
		return "", ProviderTokenKeyStatusUnresolved, ProviderTokenKeyUnresolvedReasonPlaintextNotRecovered
	}
	return ResolveProviderTokenPersistenceState(pt.SkKey, pt.KeyStatus, pt.KeyUnresolvedReason)
}

func (pt *ProviderToken) IsKeyReady() bool {
	if pt == nil {
		return false
	}
	_, status, _ := pt.resolvedPersistenceState()
	return status == ProviderTokenKeyStatusReady
}

func (pt *ProviderToken) CanParticipateInRouting() bool {
	if pt == nil {
		return false
	}
	return pt.Status == 1 && pt.IsKeyReady()
}

func (pt *ProviderToken) MarkKeyReady(skKey string) {
	pt.SkKey = NormalizeProviderTokenKeyForStorage(skKey)
	pt.KeyStatus = ProviderTokenKeyStatusReady
	pt.KeyUnresolvedReason = ""
}

func (pt *ProviderToken) MarkKeyUnresolved(reason string) {
	pt.SkKey = ""
	pt.KeyStatus = ProviderTokenKeyStatusUnresolved
	pt.KeyUnresolvedReason = defaultProviderTokenUnresolvedReason(reason, "")
}

func (pt *ProviderToken) PreserveReadyKeyFrom(existing *ProviderToken) bool {
	if existing == nil || !existing.IsKeyReady() {
		return false
	}
	pt.MarkKeyReady(existing.SkKey)
	return true
}

func (pt *ProviderToken) metadataUpdates() map[string]interface{} {
	return map[string]interface{}{
		"provider_id":       pt.ProviderId,
		"upstream_token_id": pt.UpstreamTokenId,
		"name":              pt.Name,
		"group_name":        pt.GroupName,
		"status":            pt.Status,
		"priority":          pt.Priority,
		"weight":            pt.Weight,
		"remain_quota":      pt.RemainQuota,
		"unlimited_quota":   pt.UnlimitedQuota,
		"used_quota":        pt.UsedQuota,
		"model_limits":      pt.ModelLimits,
		"last_synced":       pt.LastSynced,
	}
}

func (pt *ProviderToken) fullPersistenceUpdates() map[string]interface{} {
	updates := pt.metadataUpdates()
	normalizedKey, normalizedStatus, normalizedReason := pt.resolvedPersistenceState()
	updates["sk_key"] = normalizedKey
	updates["key_status"] = normalizedStatus
	updates["key_unresolved_reason"] = normalizedReason
	return updates
}

func GetProviderTokensByProviderId(providerId int) ([]*ProviderToken, error) {
	var tokens []*ProviderToken
	err := DB.Where("provider_id = ?", providerId).Order("id desc").Find(&tokens).Error
	return tokens, err
}

func GetEnabledProviderTokensByProviderId(providerId int) ([]*ProviderToken, error) {
	var tokens []*ProviderToken
	err := DB.Where("provider_id = ? AND status = 1", providerId).Find(&tokens).Error
	if err != nil {
		return nil, err
	}
	runnable := make([]*ProviderToken, 0, len(tokens))
	for _, token := range tokens {
		if token.CanParticipateInRouting() {
			runnable = append(runnable, token)
		}
	}
	return runnable, nil
}

func GetProviderTokenById(id int) (*ProviderToken, error) {
	if id == 0 {
		return nil, errors.New("id 为空")
	}
	var token ProviderToken
	err := DB.First(&token, "id = ?", id).Error
	return &token, err
}

func (pt *ProviderToken) Insert() error {
	pt.CreatedAt = time.Now().Unix()
	normalizedKey, normalizedStatus, normalizedReason := pt.resolvedPersistenceState()
	pt.SkKey = normalizedKey
	pt.KeyStatus = normalizedStatus
	pt.KeyUnresolvedReason = normalizedReason
	if err := DB.Create(pt).Error; err != nil {
		return err
	}
	invalidateModelRouteCaches()
	return nil
}

func (pt *ProviderToken) Update() error {
	if err := DB.Model(pt).Updates(pt.fullPersistenceUpdates()).Error; err != nil {
		return err
	}
	invalidateModelRouteCaches()
	return nil
}

// UpdateMetadataOnly updates editable metadata fields only, preventing frontend overwrites.
func (pt *ProviderToken) UpdateMetadataOnly() error {
	updates := map[string]interface{}{
		"name":       pt.Name,
		"group_name": pt.GroupName,
		"status":     pt.Status,
		"priority":   pt.Priority,
		"weight":     pt.Weight,
	}
	if err := DB.Model(pt).Updates(updates).Error; err != nil {
		return err
	}
	invalidateModelRouteCaches()
	return nil
}

func (pt *ProviderToken) Delete() error {
	// Clean up related model_routes
	if err := DB.Where("provider_token_id = ?", pt.Id).Delete(&ModelRoute{}).Error; err != nil {
		return err
	}
	if err := DB.Delete(pt).Error; err != nil {
		return err
	}
	invalidateModelRouteCaches()
	return nil
}

// UpsertByUpstreamId creates or updates a provider token based on upstream token id + provider id.
func UpsertProviderToken(pt *ProviderToken) error {
	var existing ProviderToken
	result := DB.Where("provider_id = ? AND upstream_token_id = ?", pt.ProviderId, pt.UpstreamTokenId).First(&existing)
	if result.RowsAffected > 0 {
		pt.Id = existing.Id
		pt.CreatedAt = existing.CreatedAt
		if err := DB.Model(&existing).Updates(pt.fullPersistenceUpdates()).Error; err != nil {
			return err
		}
		invalidateModelRouteCaches()
		return nil
	}
	normalizedKey, normalizedStatus, normalizedReason := pt.resolvedPersistenceState()
	pt.SkKey = normalizedKey
	pt.KeyStatus = normalizedStatus
	pt.KeyUnresolvedReason = normalizedReason
	pt.CreatedAt = time.Now().Unix()
	if err := DB.Create(pt).Error; err != nil {
		return err
	}
	invalidateModelRouteCaches()
	return nil
}

// DeleteProviderTokensNotInIds deletes tokens for a provider that are NOT in the given upstream token ID list.
func DeleteProviderTokensNotInIds(providerId int, upstreamIds []int) error {
	if len(upstreamIds) == 0 {
		if err := DB.Where("provider_id = ?", providerId).Delete(&ProviderToken{}).Error; err != nil {
			return err
		}
		invalidateModelRouteCaches()
		return nil
	}
	if err := DB.Where("provider_id = ? AND upstream_token_id NOT IN (?)", providerId, upstreamIds).Delete(&ProviderToken{}).Error; err != nil {
		return err
	}
	invalidateModelRouteCaches()
	return nil
}

// CleanForResponse keeps plaintext sk_key only for ready rows and exposes explicit unresolved state otherwise.
func (pt *ProviderToken) CleanForResponse() {
	normalizedKey, normalizedStatus, normalizedReason := pt.resolvedPersistenceState()
	pt.KeyStatus = normalizedStatus
	pt.KeyUnresolvedReason = normalizedReason
	if normalizedStatus == ProviderTokenKeyStatusReady {
		pt.SkKey = normalizedKey
		pt.KeyUnresolvedReason = ""
		return
	}
	pt.SkKey = ""
}

// GetProviderTokenByUpstream retrieves an existing token by provider_id + upstream_token_id.
func GetProviderTokenByUpstream(providerId int, upstreamTokenId int) (*ProviderToken, error) {
	var token ProviderToken
	result := DB.Where("provider_id = ? AND upstream_token_id = ?", providerId, upstreamTokenId).First(&token)
	if result.RowsAffected == 0 {
		return nil, nil
	}
	return &token, result.Error
}

func GetProvidersWithUnresolvedProviderTokens() ([]*Provider, error) {
	var providerIds []int
	err := DB.Model(&ProviderToken{}).
		Distinct("provider_id").
		Where("key_status = ?", ProviderTokenKeyStatusUnresolved).
		Order("provider_id asc").
		Pluck("provider_id", &providerIds).Error
	if err != nil {
		return nil, err
	}
	if len(providerIds) == 0 {
		return []*Provider{}, nil
	}
	var providers []*Provider
	err = applyProviderReadProjection(DB).Where("id IN ?", providerIds).Order("id asc").Find(&providers).Error
	return providers, err
}

func BackfillProviderTokenKeyState() error {
	var tokens []ProviderToken
	if err := DB.Order("id asc").Find(&tokens).Error; err != nil {
		return err
	}
	updated := false
	for i := range tokens {
		token := &tokens[i]
		normalizedKey, normalizedStatus, normalizedReason := token.resolvedPersistenceState()
		if token.SkKey == normalizedKey && token.KeyStatus == normalizedStatus && token.KeyUnresolvedReason == normalizedReason {
			continue
		}
		updates := map[string]interface{}{
			"sk_key":                normalizedKey,
			"key_status":            normalizedStatus,
			"key_unresolved_reason": normalizedReason,
		}
		if err := DB.Model(&ProviderToken{}).Where("id = ?", token.Id).Updates(updates).Error; err != nil {
			return err
		}
		updated = true
	}
	if updated {
		invalidateModelRouteCaches()
	}
	return nil
}
