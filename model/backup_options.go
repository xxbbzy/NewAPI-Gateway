package model

import (
	"strings"
)

const (
	BackupEnabledOptionKey                = "BackupEnabled"
	BackupTriggerModeOptionKey            = "BackupTriggerMode"
	BackupScheduleCronOptionKey           = "BackupScheduleCron"
	BackupMinIntervalSecondsOptionKey     = "BackupMinIntervalSeconds"
	BackupDebounceSecondsOptionKey        = "BackupDebounceSeconds"
	BackupWebDAVURLOptionKey              = "BackupWebDAVURL"
	BackupWebDAVUsernameOptionKey         = "BackupWebDAVUsername"
	BackupWebDAVPasswordOptionKey         = "BackupWebDAVPassword"
	BackupWebDAVBasePathOptionKey         = "BackupWebDAVBasePath"
	BackupEncryptEnabledOptionKey         = "BackupEncryptEnabled"
	BackupEncryptPassphraseOptionKey      = "BackupEncryptPassphrase"
	BackupRetentionDaysOptionKey          = "BackupRetentionDays"
	BackupRetentionMaxFilesOptionKey      = "BackupRetentionMaxFiles"
	BackupSpoolDirOptionKey               = "BackupSpoolDir"
	BackupCommandTimeoutSecondsOptionKey  = "BackupCommandTimeoutSeconds"
	BackupMaxRetriesOptionKey             = "BackupMaxRetries"
	BackupRetryBaseSecondsOptionKey       = "BackupRetryBaseSeconds"
	BackupMySQLDumpCommandOptionKey       = "BackupMySQLDumpCommand"
	BackupPostgresDumpCommandOptionKey    = "BackupPostgresDumpCommand"
	BackupMySQLRestoreCommandOptionKey    = "BackupMySQLRestoreCommand"
	BackupPostgresRestoreCommandOptionKey = "BackupPostgresRestoreCommand"
)

const (
	BackupTriggerModeHybrid   = "hybrid"
	BackupTriggerModeEvent    = "event"
	BackupTriggerModeSchedule = "schedule"
)

const (
	defaultBackupEnabled                = false
	defaultBackupTriggerMode            = BackupTriggerModeHybrid
	defaultBackupScheduleCron           = "0 */6 * * *"
	defaultBackupMinIntervalSeconds     = 600
	defaultBackupDebounceSeconds        = 30
	defaultBackupWebDAVBasePath         = "/newapi-gateway-backups"
	defaultBackupEncryptEnabled         = true
	defaultBackupRetentionDays          = 14
	defaultBackupRetentionMaxFiles      = 100
	defaultBackupSpoolDir               = "upload/backup-spool"
	defaultBackupCommandTimeoutSeconds  = 600
	defaultBackupMaxRetries             = 8
	defaultBackupRetryBaseSeconds       = 30
	defaultBackupMySQLDumpCommand       = "mysqldump"
	defaultBackupPostgresDumpCommand    = "pg_dump"
	defaultBackupMySQLRestoreCommand    = "mysql"
	defaultBackupPostgresRestoreCommand = "psql"
)

func normalizeBackupTriggerMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case BackupTriggerModeHybrid:
		return BackupTriggerModeHybrid
	case BackupTriggerModeEvent:
		return BackupTriggerModeEvent
	case BackupTriggerModeSchedule:
		return BackupTriggerModeSchedule
	default:
		return ""
	}
}

func ValidateBackupOption(key string, value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	switch key {
	case BackupEnabledOptionKey, BackupEncryptEnabledOptionKey:
		normalized := strings.ToLower(trimmed)
		if normalized != "true" && normalized != "false" {
			return "备份开关必须是 true 或 false", false
		}
		return "", true
	case BackupTriggerModeOptionKey:
		if normalizeBackupTriggerMode(trimmed) == "" {
			return "备份触发模式必须是 hybrid、event 或 schedule", false
		}
		return "", true
	case BackupScheduleCronOptionKey:
		if strings.TrimSpace(trimmed) == "" {
			return "备份计划不能为空，格式示例：0 */6 * * *", false
		}
		if len(strings.Fields(trimmed)) != 5 {
			return "备份计划必须是标准 5 段 cron 表达式", false
		}
		return "", true
	case BackupMinIntervalSecondsOptionKey, BackupDebounceSecondsOptionKey,
		BackupCommandTimeoutSecondsOptionKey, BackupRetentionDaysOptionKey,
		BackupRetentionMaxFilesOptionKey, BackupMaxRetriesOptionKey,
		BackupRetryBaseSecondsOptionKey:
		parsed := parseOptionIntInRange(trimmed, -1, 0, 365*24*60*60)
		if parsed < 0 {
			return "备份数值配置必须是非负整数", false
		}
		return "", true
	case BackupWebDAVURLOptionKey:
		if trimmed == "" {
			return "WebDAV 地址不能为空", false
		}
		if !strings.HasPrefix(strings.ToLower(trimmed), "http://") && !strings.HasPrefix(strings.ToLower(trimmed), "https://") {
			return "WebDAV 地址必须以 http:// 或 https:// 开头", false
		}
		return "", true
	case BackupWebDAVBasePathOptionKey, BackupSpoolDirOptionKey,
		BackupMySQLDumpCommandOptionKey, BackupPostgresDumpCommandOptionKey,
		BackupMySQLRestoreCommandOptionKey, BackupPostgresRestoreCommandOptionKey:
		if trimmed == "" {
			return "该备份配置不能为空", false
		}
		return "", true
	case BackupEncryptPassphraseOptionKey:
		if len(trimmed) > 0 && len(trimmed) < 8 {
			return "备份加密口令至少需要 8 个字符", false
		}
		return "", true
	case BackupWebDAVUsernameOptionKey, BackupWebDAVPasswordOptionKey:
		return "", true
	default:
		return "", false
	}
}

func IsSensitiveOptionKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	if normalized == "" {
		return false
	}
	switch normalized {
	case "passwordloginenabled", "passwordregisterenabled":
		return false
	}
	return strings.Contains(normalized, "token") ||
		strings.Contains(normalized, "secret") ||
		strings.Contains(normalized, "password") ||
		strings.Contains(normalized, "passphrase")
}
