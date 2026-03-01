package service

import (
	"NewAPI-Gateway/common"
	"NewAPI-Gateway/model"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type BackupConfig struct {
	Enabled                bool
	TriggerMode            string
	ScheduleCron           string
	MinInterval            time.Duration
	Debounce               time.Duration
	WebDAVURL              string
	WebDAVUsername         string
	WebDAVPassword         string
	WebDAVBasePath         string
	EncryptEnabled         bool
	EncryptPassphrase      string
	RetentionDays          int
	RetentionMaxFiles      int
	SpoolDir               string
	CommandTimeout         time.Duration
	MaxRetries             int
	RetryBase              time.Duration
	MySQLDumpCommand       string
	PostgresDumpCommand    string
	MySQLRestoreCommand    string
	PostgresRestoreCommand string
}

type BackupStatusView struct {
	Config             BackupConfig     `json:"config"`
	PendingRetryCount  int64            `json:"pending_retry_count"`
	LastRun            *model.BackupRun `json:"last_run"`
	DirtyPending       bool             `json:"dirty_pending"`
	LastDirtyReason    string           `json:"last_dirty_reason"`
	LastScheduleEvalAt int64            `json:"last_schedule_eval_at"`
}

func (cfg BackupConfig) RedactedForAPI() BackupConfig {
	redacted := cfg
	if strings.TrimSpace(redacted.WebDAVPassword) != "" {
		redacted.WebDAVPassword = "***"
	}
	if strings.TrimSpace(redacted.EncryptPassphrase) != "" {
		redacted.EncryptPassphrase = "***"
	}
	return redacted
}

func parseBool(raw string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func parseInt(raw string, fallback int, min int, max int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	if parsed < min || parsed > max {
		return fallback
	}
	return parsed
}

func GetBackupConfig() BackupConfig {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()

	cfg := BackupConfig{
		Enabled:                parseBool(common.OptionMap[model.BackupEnabledOptionKey], false),
		TriggerMode:            strings.TrimSpace(common.OptionMap[model.BackupTriggerModeOptionKey]),
		ScheduleCron:           strings.TrimSpace(common.OptionMap[model.BackupScheduleCronOptionKey]),
		WebDAVURL:              strings.TrimSpace(common.OptionMap[model.BackupWebDAVURLOptionKey]),
		WebDAVUsername:         strings.TrimSpace(common.OptionMap[model.BackupWebDAVUsernameOptionKey]),
		WebDAVPassword:         strings.TrimSpace(common.OptionMap[model.BackupWebDAVPasswordOptionKey]),
		WebDAVBasePath:         strings.TrimSpace(common.OptionMap[model.BackupWebDAVBasePathOptionKey]),
		EncryptEnabled:         parseBool(common.OptionMap[model.BackupEncryptEnabledOptionKey], true),
		EncryptPassphrase:      strings.TrimSpace(common.OptionMap[model.BackupEncryptPassphraseOptionKey]),
		RetentionDays:          parseInt(common.OptionMap[model.BackupRetentionDaysOptionKey], 14, 0, 3650),
		RetentionMaxFiles:      parseInt(common.OptionMap[model.BackupRetentionMaxFilesOptionKey], 100, 0, 100000),
		SpoolDir:               strings.TrimSpace(common.OptionMap[model.BackupSpoolDirOptionKey]),
		MaxRetries:             parseInt(common.OptionMap[model.BackupMaxRetriesOptionKey], 8, 0, 1000),
		MySQLDumpCommand:       strings.TrimSpace(common.OptionMap[model.BackupMySQLDumpCommandOptionKey]),
		PostgresDumpCommand:    strings.TrimSpace(common.OptionMap[model.BackupPostgresDumpCommandOptionKey]),
		MySQLRestoreCommand:    strings.TrimSpace(common.OptionMap[model.BackupMySQLRestoreCommandOptionKey]),
		PostgresRestoreCommand: strings.TrimSpace(common.OptionMap[model.BackupPostgresRestoreCommandOptionKey]),
	}

	cfg.TriggerMode = model.BackupTriggerModeHybrid
	if normalized := normalizeBackupTriggerMode(common.OptionMap[model.BackupTriggerModeOptionKey]); normalized != "" {
		cfg.TriggerMode = normalized
	}
	cfg.ScheduleCron = strings.TrimSpace(common.OptionMap[model.BackupScheduleCronOptionKey])
	if cfg.ScheduleCron == "" {
		cfg.ScheduleCron = "0 */6 * * *"
	}
	cfg.MinInterval = time.Duration(parseInt(common.OptionMap[model.BackupMinIntervalSecondsOptionKey], 600, 0, 365*24*60*60)) * time.Second
	cfg.Debounce = time.Duration(parseInt(common.OptionMap[model.BackupDebounceSecondsOptionKey], 30, 0, 86400)) * time.Second
	cfg.CommandTimeout = time.Duration(parseInt(common.OptionMap[model.BackupCommandTimeoutSecondsOptionKey], 600, 1, 365*24*60*60)) * time.Second
	cfg.RetryBase = time.Duration(parseInt(common.OptionMap[model.BackupRetryBaseSecondsOptionKey], 30, 1, 86400)) * time.Second
	if cfg.WebDAVBasePath == "" {
		cfg.WebDAVBasePath = "/newapi-gateway-backups"
	}
	if cfg.SpoolDir == "" {
		cfg.SpoolDir = filepath.Join(common.UploadPath, "backup-spool")
	}
	if cfg.MySQLDumpCommand == "" {
		cfg.MySQLDumpCommand = "mysqldump"
	}
	if cfg.PostgresDumpCommand == "" {
		cfg.PostgresDumpCommand = "pg_dump"
	}
	if cfg.MySQLRestoreCommand == "" {
		cfg.MySQLRestoreCommand = "mysql"
	}
	if cfg.PostgresRestoreCommand == "" {
		cfg.PostgresRestoreCommand = "psql"
	}
	return cfg
}

func getActiveSQLDriverAndDSN() (string, string, error) {
	dsn := strings.TrimSpace(os.Getenv("SQL_DSN"))
	driver := strings.ToLower(strings.TrimSpace(os.Getenv("SQL_DRIVER")))
	switch driver {
	case "mysql":
		if dsn == "" {
			return "", "", fmt.Errorf("SQL_DSN is required when SQL_DRIVER=mysql")
		}
		return "mysql", dsn, nil
	case "postgres", "postgresql":
		if dsn == "" {
			return "", "", fmt.Errorf("SQL_DSN is required when SQL_DRIVER=postgres")
		}
		return "postgres", dsn, nil
	case "sqlite", "sqlite3":
		if dsn == "" {
			return "sqlite", common.SQLitePath, nil
		}
		return "sqlite", dsn, nil
	case "":
		if dsn == "" {
			return "sqlite", common.SQLitePath, nil
		}
		dsnLower := strings.ToLower(strings.TrimSpace(dsn))
		if strings.HasPrefix(dsnLower, "postgres://") || strings.HasPrefix(dsnLower, "postgresql://") || (strings.Contains(dsnLower, "dbname=") && strings.Contains(dsnLower, "user=")) {
			return "postgres", dsn, nil
		}
		return "mysql", dsn, nil
	default:
		return "", "", fmt.Errorf("unsupported SQL_DRIVER: %s", driver)
	}
}

func normalizeBackupTriggerMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case model.BackupTriggerModeHybrid:
		return model.BackupTriggerModeHybrid
	case model.BackupTriggerModeEvent:
		return model.BackupTriggerModeEvent
	case model.BackupTriggerModeSchedule:
		return model.BackupTriggerModeSchedule
	default:
		return ""
	}
}

func shouldAllowEventTrigger(mode string) bool {
	normalized := normalizeBackupTriggerMode(mode)
	return normalized == model.BackupTriggerModeHybrid || normalized == model.BackupTriggerModeEvent
}

func shouldAllowScheduleTrigger(mode string) bool {
	normalized := normalizeBackupTriggerMode(mode)
	return normalized == model.BackupTriggerModeHybrid || normalized == model.BackupTriggerModeSchedule
}

func ensureBackupSpoolDir(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("backup spool directory is empty")
	}
	return os.MkdirAll(path, 0o755)
}

func shouldRunCronExpression(cronExpr string, now time.Time) (bool, error) {
	fields := strings.Fields(strings.TrimSpace(cronExpr))
	if len(fields) != 5 {
		return false, fmt.Errorf("invalid cron expression: %s", cronExpr)
	}
	values := []int{now.Minute(), now.Hour(), now.Day(), int(now.Month()), int(now.Weekday())}
	for idx, token := range fields {
		matched, err := matchCronToken(token, values[idx])
		if err != nil {
			return false, err
		}
		if !matched {
			return false, nil
		}
	}
	return true, nil
}

func matchCronToken(token string, value int) (bool, error) {
	trimmed := strings.TrimSpace(token)
	if trimmed == "*" {
		return true, nil
	}
	if strings.HasPrefix(trimmed, "*/") {
		step, err := strconv.Atoi(strings.TrimPrefix(trimmed, "*/"))
		if err != nil || step <= 0 {
			return false, fmt.Errorf("invalid cron step: %s", token)
		}
		return value%step == 0, nil
	}
	if strings.Contains(trimmed, ",") {
		parts := strings.Split(trimmed, ",")
		for _, part := range parts {
			matched, err := matchCronToken(part, value)
			if err != nil {
				return false, err
			}
			if matched {
				return true, nil
			}
		}
		return false, nil
	}
	parsed, err := strconv.Atoi(trimmed)
	if err != nil {
		return false, fmt.Errorf("invalid cron token: %s", token)
	}
	return parsed == value, nil
}
