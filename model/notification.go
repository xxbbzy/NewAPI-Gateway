package model

import (
	"NewAPI-Gateway/common"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	NotificationBarkEnabledOptionKey                 = "NotificationBarkEnabled"
	NotificationBarkServerOptionKey                  = "NotificationBarkServer"
	NotificationBarkDeviceKeyOptionKey               = "NotificationBarkDeviceKey"
	NotificationBarkGroupOptionKey                   = "NotificationBarkGroup"
	NotificationWebhookEnabledOptionKey              = "NotificationWebhookEnabled"
	NotificationWebhookURLOptionKey                  = "NotificationWebhookURL"
	NotificationWebhookTokenOptionKey                = "NotificationWebhookToken"
	NotificationSMTPEnabledOptionKey                 = "NotificationSMTPEnabled"
	NotificationSMTPRecipientsOptionKey              = "NotificationSMTPRecipients"
	NotificationSMTPSubjectPrefixOptionKey           = "NotificationSMTPSubjectPrefix"
	NotificationCheckinSummaryEnabledOptionKey       = "NotificationCheckinSummaryEnabled"
	NotificationCheckinFailureEnabledOptionKey       = "NotificationCheckinFailureEnabled"
	NotificationProviderAutoDisableEnabledOptionKey  = "NotificationProviderAutoDisableEnabled"
	NotificationProviderHealthEnabledOptionKey       = "NotificationProviderHealthEnabled"
	NotificationRequestFailureEnabledOptionKey       = "NotificationRequestFailureEnabled"
	NotificationVerbosityModeOptionKey               = "NotificationVerbosityMode"
	NotificationRequestFailureThresholdOptionKey     = "NotificationRequestFailureThreshold"
	NotificationRequestFailureWindowMinutesOptionKey = "NotificationRequestFailureWindowMinutes"
)

const (
	NotificationVerbosityConcise  = "concise"
	NotificationVerbosityDetailed = "detailed"

	NotificationAlertStatusOpen   = "open"
	NotificationAlertStatusClosed = "closed"
)

const (
	defaultNotificationBarkEnabled                 = false
	defaultNotificationWebhookEnabled              = false
	defaultNotificationSMTPEnabled                 = false
	defaultNotificationCheckinSummaryEnabled       = false
	defaultNotificationCheckinFailureEnabled       = true
	defaultNotificationProviderAutoDisableEnabled  = true
	defaultNotificationProviderHealthEnabled       = true
	defaultNotificationRequestFailureEnabled       = false
	defaultNotificationVerbosityMode               = NotificationVerbosityConcise
	defaultNotificationRequestFailureThreshold     = 5
	defaultNotificationRequestFailureWindowMinutes = 10
	defaultNotificationSMTPSubjectPrefix           = "[NewAPI Gateway]"
)

type NotificationBarkConfig struct {
	Enabled   bool
	Server    string
	DeviceKey string
	Group     string
}

type NotificationWebhookConfig struct {
	Enabled bool
	URL     string
	Token   string
}

type NotificationSMTPConfig struct {
	Enabled       bool
	Recipients    []string
	SubjectPrefix string
}

type NotificationPolicyConfig struct {
	CheckinSummaryEnabled       bool
	CheckinFailureEnabled       bool
	ProviderAutoDisableEnabled  bool
	ProviderHealthEnabled       bool
	RequestFailureEnabled       bool
	VerbosityMode               string
	RequestFailureThreshold     int
	RequestFailureWindowMinutes int
}

type NotificationSettings struct {
	Bark    NotificationBarkConfig
	Webhook NotificationWebhookConfig
	SMTP    NotificationSMTPConfig
	Policy  NotificationPolicyConfig
}

type NotificationAlertState struct {
	Id              int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	DedupeKey       string `json:"dedupe_key" gorm:"type:varchar(255);uniqueIndex;not null"`
	EventFamily     string `json:"event_family" gorm:"type:varchar(64);index;not null"`
	Status          string `json:"status" gorm:"type:varchar(16);index;not null"`
	LastFiredAt     int64  `json:"last_fired_at"`
	LastObservedAt  int64  `json:"last_observed_at"`
	LastResolvedAt  int64  `json:"last_resolved_at"`
	WindowStartedAt int64  `json:"window_started_at"`
	WindowCount     int    `json:"window_count"`
	LastSummary     string `json:"last_summary" gorm:"type:text"`
	CreatedAt       int64  `json:"created_at" gorm:"index"`
	UpdatedAt       int64  `json:"updated_at" gorm:"index"`
}

func (s *NotificationAlertState) BeforeCreate(_ *gorm.DB) error {
	now := time.Now().Unix()
	if s.CreatedAt == 0 {
		s.CreatedAt = now
	}
	s.UpdatedAt = now
	if strings.TrimSpace(s.Status) == "" {
		s.Status = NotificationAlertStatusClosed
	}
	s.DedupeKey = strings.TrimSpace(s.DedupeKey)
	s.EventFamily = strings.TrimSpace(s.EventFamily)
	s.LastSummary = strings.TrimSpace(s.LastSummary)
	return nil
}

func (s *NotificationAlertState) BeforeUpdate(_ *gorm.DB) error {
	s.UpdatedAt = time.Now().Unix()
	s.DedupeKey = strings.TrimSpace(s.DedupeKey)
	s.EventFamily = strings.TrimSpace(s.EventFamily)
	s.LastSummary = strings.TrimSpace(s.LastSummary)
	if strings.TrimSpace(s.Status) == "" {
		s.Status = NotificationAlertStatusClosed
	}
	return nil
}

func ApplyNotificationOptionDefaults(optionMap map[string]string) {
	if optionMap == nil {
		return
	}
	defaults := map[string]string{
		NotificationBarkEnabledOptionKey:                 strconv.FormatBool(defaultNotificationBarkEnabled),
		NotificationBarkServerOptionKey:                  "",
		NotificationBarkDeviceKeyOptionKey:               "",
		NotificationBarkGroupOptionKey:                   "",
		NotificationWebhookEnabledOptionKey:              strconv.FormatBool(defaultNotificationWebhookEnabled),
		NotificationWebhookURLOptionKey:                  "",
		NotificationWebhookTokenOptionKey:                "",
		NotificationSMTPEnabledOptionKey:                 strconv.FormatBool(defaultNotificationSMTPEnabled),
		NotificationSMTPRecipientsOptionKey:              "",
		NotificationSMTPSubjectPrefixOptionKey:           defaultNotificationSMTPSubjectPrefix,
		NotificationCheckinSummaryEnabledOptionKey:       strconv.FormatBool(defaultNotificationCheckinSummaryEnabled),
		NotificationCheckinFailureEnabledOptionKey:       strconv.FormatBool(defaultNotificationCheckinFailureEnabled),
		NotificationProviderAutoDisableEnabledOptionKey:  strconv.FormatBool(defaultNotificationProviderAutoDisableEnabled),
		NotificationProviderHealthEnabledOptionKey:       strconv.FormatBool(defaultNotificationProviderHealthEnabled),
		NotificationRequestFailureEnabledOptionKey:       strconv.FormatBool(defaultNotificationRequestFailureEnabled),
		NotificationVerbosityModeOptionKey:               defaultNotificationVerbosityMode,
		NotificationRequestFailureThresholdOptionKey:     strconv.Itoa(defaultNotificationRequestFailureThreshold),
		NotificationRequestFailureWindowMinutesOptionKey: strconv.Itoa(defaultNotificationRequestFailureWindowMinutes),
	}
	for key, value := range defaults {
		optionMap[key] = value
	}
}

func ParseNotificationSettings() NotificationSettings {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()

	recipients := splitNotificationRecipients(common.OptionMap[NotificationSMTPRecipientsOptionKey])
	return NotificationSettings{
		Bark: NotificationBarkConfig{
			Enabled:   parseNotificationBool(common.OptionMap[NotificationBarkEnabledOptionKey], defaultNotificationBarkEnabled),
			Server:    strings.TrimSpace(common.OptionMap[NotificationBarkServerOptionKey]),
			DeviceKey: strings.TrimSpace(common.OptionMap[NotificationBarkDeviceKeyOptionKey]),
			Group:     strings.TrimSpace(common.OptionMap[NotificationBarkGroupOptionKey]),
		},
		Webhook: NotificationWebhookConfig{
			Enabled: parseNotificationBool(common.OptionMap[NotificationWebhookEnabledOptionKey], defaultNotificationWebhookEnabled),
			URL:     strings.TrimSpace(common.OptionMap[NotificationWebhookURLOptionKey]),
			Token:   strings.TrimSpace(common.OptionMap[NotificationWebhookTokenOptionKey]),
		},
		SMTP: NotificationSMTPConfig{
			Enabled:       parseNotificationBool(common.OptionMap[NotificationSMTPEnabledOptionKey], defaultNotificationSMTPEnabled),
			Recipients:    recipients,
			SubjectPrefix: strings.TrimSpace(common.OptionMap[NotificationSMTPSubjectPrefixOptionKey]),
		},
		Policy: NotificationPolicyConfig{
			CheckinSummaryEnabled:       parseNotificationBool(common.OptionMap[NotificationCheckinSummaryEnabledOptionKey], defaultNotificationCheckinSummaryEnabled),
			CheckinFailureEnabled:       parseNotificationBool(common.OptionMap[NotificationCheckinFailureEnabledOptionKey], defaultNotificationCheckinFailureEnabled),
			ProviderAutoDisableEnabled:  parseNotificationBool(common.OptionMap[NotificationProviderAutoDisableEnabledOptionKey], defaultNotificationProviderAutoDisableEnabled),
			ProviderHealthEnabled:       parseNotificationBool(common.OptionMap[NotificationProviderHealthEnabledOptionKey], defaultNotificationProviderHealthEnabled),
			RequestFailureEnabled:       parseNotificationBool(common.OptionMap[NotificationRequestFailureEnabledOptionKey], defaultNotificationRequestFailureEnabled),
			VerbosityMode:               normalizeNotificationVerbosity(common.OptionMap[NotificationVerbosityModeOptionKey]),
			RequestFailureThreshold:     parseNotificationInt(common.OptionMap[NotificationRequestFailureThresholdOptionKey], defaultNotificationRequestFailureThreshold),
			RequestFailureWindowMinutes: parseNotificationInt(common.OptionMap[NotificationRequestFailureWindowMinutesOptionKey], defaultNotificationRequestFailureWindowMinutes),
		},
	}
}

func parseNotificationBool(raw string, fallback bool) bool {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	switch normalized {
	case "true":
		return true
	case "false":
		return false
	default:
		return fallback
	}
}

func parseNotificationInt(raw string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return parsed
}

func normalizeNotificationVerbosity(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case NotificationVerbosityDetailed:
		return NotificationVerbosityDetailed
	default:
		return NotificationVerbosityConcise
	}
}

func splitNotificationRecipients(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ';' || r == ',' || r == '\n'
	})
	recipients := make([]string, 0, len(fields))
	for _, field := range fields {
		trimmed := strings.TrimSpace(field)
		if trimmed == "" {
			continue
		}
		recipients = append(recipients, trimmed)
	}
	return recipients
}

func ValidateNotificationOption(key string, value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	switch key {
	case NotificationBarkEnabledOptionKey,
		NotificationWebhookEnabledOptionKey,
		NotificationSMTPEnabledOptionKey,
		NotificationCheckinSummaryEnabledOptionKey,
		NotificationCheckinFailureEnabledOptionKey,
		NotificationProviderAutoDisableEnabledOptionKey,
		NotificationProviderHealthEnabledOptionKey,
		NotificationRequestFailureEnabledOptionKey:
		normalized := strings.ToLower(trimmed)
		if normalized != "true" && normalized != "false" {
			return "通知开关必须是 true 或 false", false
		}
		return "", true
	case NotificationBarkServerOptionKey:
		if trimmed == "" {
			return "", true
		}
		if !isHTTPURL(trimmed) {
			return "Bark 服务器地址必须以 http:// 或 https:// 开头", false
		}
		return "", true
	case NotificationBarkDeviceKeyOptionKey, NotificationBarkGroupOptionKey:
		return "", true
	case NotificationWebhookURLOptionKey:
		if trimmed == "" {
			return "", true
		}
		if !isHTTPURL(trimmed) {
			return "Webhook 地址必须以 http:// 或 https:// 开头", false
		}
		return "", true
	case NotificationWebhookTokenOptionKey:
		return "", true
	case NotificationSMTPRecipientsOptionKey:
		if trimmed == "" {
			return "", true
		}
		recipients := splitNotificationRecipients(trimmed)
		if len(recipients) == 0 {
			return "邮件接收人不能为空", false
		}
		for _, recipient := range recipients {
			if _, err := mail.ParseAddress(recipient); err != nil {
				return "邮件接收人格式无效，请使用逗号或分号分隔邮箱地址", false
			}
		}
		return "", true
	case NotificationSMTPSubjectPrefixOptionKey:
		if len(trimmed) > 120 {
			return "邮件标题前缀不能超过 120 个字符", false
		}
		return "", true
	case NotificationVerbosityModeOptionKey:
		if normalizeNotificationVerbosity(trimmed) != trimmed && !strings.EqualFold(trimmed, NotificationVerbosityDetailed) {
			return "通知详细度必须是 concise 或 detailed", false
		}
		return "", true
	case NotificationRequestFailureThresholdOptionKey:
		parsed := parseOptionIntInRange(trimmed, -1, 1, 1000)
		if parsed < 1 {
			return "请求失败告警阈值必须是 1 到 1000 的整数", false
		}
		return "", true
	case NotificationRequestFailureWindowMinutesOptionKey:
		parsed := parseOptionIntInRange(trimmed, -1, 1, 24*60)
		if parsed < 1 {
			return "请求失败统计窗口必须是 1 到 1440 分钟的整数", false
		}
		return "", true
	default:
		return "", false
	}
}

func isNotificationSensitiveOptionKey(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case strings.ToLower(NotificationBarkDeviceKeyOptionKey),
		strings.ToLower(NotificationWebhookURLOptionKey),
		strings.ToLower(NotificationWebhookTokenOptionKey):
		return true
	default:
		return false
	}
}

func isHTTPURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed == nil {
		return false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	return strings.TrimSpace(parsed.Host) != ""
}

func GetNotificationAlertState(dedupeKey string) (*NotificationAlertState, error) {
	trimmed := strings.TrimSpace(dedupeKey)
	if trimmed == "" {
		return nil, nil
	}
	var state NotificationAlertState
	err := DB.Where("dedupe_key = ?", trimmed).First(&state).Error
	if err == nil {
		return &state, nil
	}
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return nil, err
}

func SaveNotificationAlertState(state *NotificationAlertState) error {
	if state == nil {
		return nil
	}
	state.DedupeKey = strings.TrimSpace(state.DedupeKey)
	state.EventFamily = strings.TrimSpace(state.EventFamily)
	state.LastSummary = strings.TrimSpace(state.LastSummary)
	if state.DedupeKey == "" {
		return nil
	}
	var existing NotificationAlertState
	err := DB.Where("dedupe_key = ?", state.DedupeKey).First(&existing).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}
	if err == gorm.ErrRecordNotFound {
		return DB.Create(state).Error
	}
	state.Id = existing.Id
	state.CreatedAt = existing.CreatedAt
	return DB.Save(state).Error
}
