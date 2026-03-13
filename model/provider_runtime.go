package model

import (
	"NewAPI-Gateway/common"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	ProviderHealthStatusUnknown     = "unknown"
	ProviderHealthStatusHealthy     = "healthy"
	ProviderHealthStatusUnreachable = "unreachable"

	providerHealthCooldownSeconds = 5 * 60
	providerBalanceFreshHours     = 24
)

type ProviderSummary struct {
	TotalProviders           int64   `json:"total_providers"`
	BalanceTotalUSD          float64 `json:"balance_total_usd"`
	BalanceAccountCount      int64   `json:"balance_account_count"`
	BalanceMissingCount      int64   `json:"balance_missing_count"`
	BalanceFreshCount        int64   `json:"balance_fresh_count"`
	BalanceStaleCount        int64   `json:"balance_stale_count"`
	BalanceNeverUpdatedCount int64   `json:"balance_never_updated_count"`
	UnreachableProviders     int64   `json:"unreachable_providers"`
	ProxyEnabledProviders    int64   `json:"proxy_enabled_providers"`
	FreshnessWindowHours     int     `json:"freshness_window_hours"`
}

func normalizeProviderHealthStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case ProviderHealthStatusHealthy:
		return ProviderHealthStatusHealthy
	case ProviderHealthStatusUnreachable:
		return ProviderHealthStatusUnreachable
	default:
		return ProviderHealthStatusUnknown
	}
}

func filterAutomatedUsableProviders(providers []*Provider) []*Provider {
	now := time.Now()
	filtered := make([]*Provider, 0, len(providers))
	for _, provider := range providers {
		if provider == nil {
			continue
		}
		if !provider.CanParticipateInAutomatedUseAt(now) {
			continue
		}
		filtered = append(filtered, provider)
	}
	return filtered
}

func (p *Provider) IsHealthBlockedAt(now time.Time) bool {
	if p == nil {
		return false
	}
	if normalizeProviderHealthStatus(p.HealthStatus) != ProviderHealthStatusUnreachable {
		return false
	}
	if p.HealthCooldownUntil <= 0 {
		return true
	}
	return now.Unix() < p.HealthCooldownUntil
}

func (p *Provider) CanParticipateInAutomatedUseAt(now time.Time) bool {
	if p == nil {
		return false
	}
	if p.Status != common.UserStatusEnabled {
		return false
	}
	return !p.IsHealthBlockedAt(now)
}

func (p *Provider) BalanceFreshnessAt(now time.Time) string {
	if p == nil || p.BalanceUpdated <= 0 {
		return "unknown"
	}
	freshWindow := time.Duration(providerBalanceFreshHours) * time.Hour
	updatedAt := time.Unix(p.BalanceUpdated, 0)
	if now.Sub(updatedAt) <= freshWindow {
		return "fresh"
	}
	return "stale"
}

func (p *Provider) MarkHealthFailure(reason string) error {
	if p == nil || p.Id == 0 {
		return errors.New("invalid provider")
	}
	now := time.Now().Unix()
	normalizedReason := strings.TrimSpace(reason)
	if normalizedReason == "" {
		normalizedReason = "upstream unavailable"
	}
	updates := map[string]interface{}{
		"health_status":         ProviderHealthStatusUnreachable,
		"health_check_at":       now,
		"health_failure_at":     now,
		"health_failure_reason": normalizedReason,
		"health_cooldown_until": now + providerHealthCooldownSeconds,
	}
	if err := DB.Model(&Provider{}).Where("id = ?", p.Id).Updates(updates).Error; err != nil {
		return err
	}
	p.HealthStatus = ProviderHealthStatusUnreachable
	p.HealthCheckAt = now
	p.HealthFailureAt = now
	p.HealthFailureReason = normalizedReason
	p.HealthCooldownUntil = now + providerHealthCooldownSeconds
	invalidateModelRouteCaches()
	return nil
}

func (p *Provider) MarkHealthSuccess() error {
	if p == nil || p.Id == 0 {
		return errors.New("invalid provider")
	}
	now := time.Now().Unix()
	updates := map[string]interface{}{
		"health_status":         ProviderHealthStatusHealthy,
		"health_check_at":       now,
		"health_success_at":     now,
		"health_failure_at":     0,
		"health_failure_reason": "",
		"health_cooldown_until": 0,
	}
	if err := DB.Model(&Provider{}).Where("id = ?", p.Id).Updates(updates).Error; err != nil {
		return err
	}
	p.HealthStatus = ProviderHealthStatusHealthy
	p.HealthCheckAt = now
	p.HealthSuccessAt = now
	p.HealthFailureAt = 0
	p.HealthFailureReason = ""
	p.HealthCooldownUntil = 0
	invalidateModelRouteCaches()
	return nil
}

func RedactProxyURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "(configured)"
	}
	if parsed.Scheme != "" && parsed.Host != "" {
		return parsed.Scheme + "://" + parsed.Host
	}
	if parsed.Host != "" {
		return parsed.Host
	}
	if parsed.Scheme != "" && parsed.Opaque != "" {
		return parsed.Scheme + "://" + parsed.Opaque
	}
	if parsed.Path != "" {
		return parsed.Path
	}
	return "(configured)"
}

func SanitizeProviderSensitiveText(provider *Provider, value string) string {
	text := strings.TrimSpace(value)
	if text == "" || provider == nil {
		return text
	}
	rawProxyURL := strings.TrimSpace(provider.ProxyURL)
	if rawProxyURL == "" {
		return text
	}
	redactedProxyURL := RedactProxyURL(rawProxyURL)
	if redactedProxyURL == "" {
		redactedProxyURL = "(configured)"
	}
	return strings.ReplaceAll(text, rawProxyURL, redactedProxyURL)
}

func ValidateProviderProxyConfig(enabled bool, proxyURL string) error {
	if !enabled {
		return nil
	}
	trimmed := strings.TrimSpace(proxyURL)
	if trimmed == "" {
		return errors.New("启用代理时必须提供代理地址")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed == nil || parsed.Host == "" {
		return errors.New("代理地址格式无效")
	}
	if parsed.Scheme == "" {
		return errors.New("代理地址必须包含协议，例如 http:// 或 socks5://")
	}
	return nil
}

func parseBalanceSummaryValue(balance string) (float64, bool) {
	trimmed := strings.TrimSpace(balance)
	if trimmed == "" {
		return 0, false
	}
	matched := balanceNumberPattern.FindString(trimmed)
	if matched == "" {
		return 0, false
	}
	value, err := strconv.ParseFloat(matched, 64)
	if err != nil {
		return 0, false
	}
	if value < 0 {
		value = 0
	}
	return value, true
}

func GetProviderSummary() (*ProviderSummary, error) {
	summary := &ProviderSummary{
		FreshnessWindowHours: providerBalanceFreshHours,
	}
	var providers []Provider
	if err := applyProviderReadProjection(DB).Find(&providers).Error; err != nil {
		return nil, err
	}
	now := time.Now()
	for i := range providers {
		provider := &providers[i]
		summary.TotalProviders++
		if provider.ProxyEnabled {
			summary.ProxyEnabledProviders++
		}
		if provider.IsHealthBlockedAt(now) {
			summary.UnreachableProviders++
		}
		if value, ok := parseBalanceSummaryValue(provider.Balance); ok {
			summary.BalanceTotalUSD += value
			summary.BalanceAccountCount++
		} else {
			summary.BalanceMissingCount++
		}
		switch provider.BalanceFreshnessAt(now) {
		case "fresh":
			summary.BalanceFreshCount++
		case "stale":
			summary.BalanceStaleCount++
		default:
			summary.BalanceNeverUpdatedCount++
		}
	}
	return summary, nil
}
