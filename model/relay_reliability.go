package model

import (
	"NewAPI-Gateway/common"
	"fmt"
	"strings"
)

const (
	RelayResponseValidityGuardModeOptionKey                   = "RelayResponseValidityGuardMode"
	RelayResponseValidityGuardEnabledOptionKey                = "RelayResponseValidityGuardEnabled"
	RoutingInvalidResponseSuppressionEnabledOptionKey         = "RoutingInvalidResponseSuppressionEnabled"
	RoutingInvalidResponseSuppressionThresholdOptionKey       = "RoutingInvalidResponseSuppressionThreshold"
	RoutingInvalidResponseSuppressionWindowMinutesOptionKey   = "RoutingInvalidResponseSuppressionWindowMinutes"
	RoutingInvalidResponseSuppressionCooldownMinutesOptionKey = "RoutingInvalidResponseSuppressionCooldownMinutes"

	RelayResponseValidityModeOff     = "off"
	RelayResponseValidityModeObserve = "observe"
	RelayResponseValidityModeEnforce = "enforce"

	defaultRelayResponseValidityGuardMode            = RelayResponseValidityModeEnforce
	defaultRelayResponseValidityGuardEnabled         = true
	defaultInvalidResponseSuppressionEnabled         = false
	defaultInvalidResponseSuppressionThreshold       = 3
	defaultInvalidResponseSuppressionWindowMinutes   = 10
	defaultInvalidResponseSuppressionCooldownMinutes = 15
)

type relayReliabilityConfig struct {
	ResponseValidityMode string
}

func loadRelayReliabilityConfig() relayReliabilityConfig {
	config := relayReliabilityConfig{
		ResponseValidityMode: defaultRelayResponseValidityGuardMode,
	}

	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()

	modeRaw := strings.TrimSpace(common.OptionMap[RelayResponseValidityGuardModeOptionKey])
	mode := normalizeRelayResponseValidityMode(modeRaw)
	if mode == "" {
		legacyEnabled := parseOptionBool(
			common.OptionMap[RelayResponseValidityGuardEnabledOptionKey],
			defaultRelayResponseValidityGuardEnabled,
		)
		if legacyEnabled {
			mode = RelayResponseValidityModeEnforce
		} else {
			mode = RelayResponseValidityModeOff
		}
	}
	config.ResponseValidityMode = mode
	return config
}

func normalizeRelayResponseValidityMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case RelayResponseValidityModeOff:
		return RelayResponseValidityModeOff
	case RelayResponseValidityModeObserve:
		return RelayResponseValidityModeObserve
	case RelayResponseValidityModeEnforce:
		return RelayResponseValidityModeEnforce
	default:
		return ""
	}
}

func GetRelayResponseValidityMode() string {
	return loadRelayReliabilityConfig().ResponseValidityMode
}

func ShouldValidateRelayResponse() bool {
	return GetRelayResponseValidityMode() != RelayResponseValidityModeOff
}

func ShouldEnforceRelayResponse() bool {
	return GetRelayResponseValidityMode() == RelayResponseValidityModeEnforce
}

func IsRelayResponseValidityGuardEnabled() bool {
	return ShouldEnforceRelayResponse()
}

func ensureUsageLogObservabilityIndexes() error {
	if DB == nil {
		return fmt.Errorf("database is not initialized")
	}
	migrator := DB.Migrator()
	indexes := []string{
		"idx_usage_logs_relay_attempt",
		"idx_usage_logs_route_window",
		"idx_usage_logs_invalid_window",
	}
	for _, indexName := range indexes {
		if migrator.HasIndex(&UsageLog{}, indexName) {
			continue
		}
		if err := migrator.CreateIndex(&UsageLog{}, indexName); err != nil {
			return fmt.Errorf("create index %s failed: %w", indexName, err)
		}
	}
	return nil
}

func RunRelayReliabilityStartupPreflight() error {
	if DB == nil {
		return fmt.Errorf("database is not initialized")
	}
	config := loadRelayReliabilityConfig()
	common.OptionMapRWMutex.RLock()
	suppressionEnabled := parseOptionBool(
		common.OptionMap[RoutingInvalidResponseSuppressionEnabledOptionKey],
		defaultInvalidResponseSuppressionEnabled,
	)
	common.OptionMapRWMutex.RUnlock()

	requiredColumns := []string{
		"relay_request_id",
		"attempt_index",
		"usage_source",
		"usage_parser",
		"failure_category",
		"invalid_reason",
		"transport_status_code",
		"request_model_original",
		"request_model_canonical",
		"request_model_resolved",
	}
	if config.ResponseValidityMode != RelayResponseValidityModeOff || suppressionEnabled {
		for _, column := range requiredColumns {
			if DB.Migrator().HasColumn(&UsageLog{}, column) {
				continue
			}
			return fmt.Errorf("relay reliability preflight failed: usage_logs.%s missing while reliability flags are enabled", column)
		}
	}

	warnIndexes := []string{
		"idx_usage_logs_relay_attempt",
		"idx_usage_logs_route_window",
		"idx_usage_logs_invalid_window",
	}
	for _, indexName := range warnIndexes {
		if DB.Migrator().HasIndex(&UsageLog{}, indexName) {
			continue
		}
		common.SysLog("relay reliability preflight warning: usage_logs index missing: " + indexName)
	}

	return nil
}

func ValidateRelayReliabilityOption(key string, value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	switch key {
	case RelayResponseValidityGuardModeOptionKey:
		mode := normalizeRelayResponseValidityMode(trimmed)
		if mode == "" {
			return "响应有效性模式必须是 off、observe 或 enforce", false
		}
		return "", true
	case RelayResponseValidityGuardEnabledOptionKey, RoutingInvalidResponseSuppressionEnabledOptionKey:
		normalized := strings.ToLower(trimmed)
		if normalized != "true" && normalized != "false" {
			return "该开关必须是 true 或 false", false
		}
		return "", true
	case RoutingInvalidResponseSuppressionThresholdOptionKey:
		parsed := parseOptionIntInRange(trimmed, -1, 1, 1000)
		if parsed < 1 {
			return "无效响应阈值必须是 1 到 1000 的整数", false
		}
		return "", true
	case RoutingInvalidResponseSuppressionWindowMinutesOptionKey, RoutingInvalidResponseSuppressionCooldownMinutesOptionKey:
		parsed := parseOptionIntInRange(trimmed, -1, 1, 24*60)
		if parsed < 1 {
			return "无效响应窗口/冷却时间必须是 1 到 1440 的整数分钟", false
		}
		return "", true
	default:
		return "", false
	}
}
