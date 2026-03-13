package service

import (
	"NewAPI-Gateway/common"
	"NewAPI-Gateway/model"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"sort"
	"strings"
	"time"
)

const (
	NotificationFamilyCheckinSummary      = "checkin_summary"
	NotificationFamilyCheckinFailure      = "checkin_failure"
	NotificationFamilyProviderAutoDisable = "provider_auto_disable"
	NotificationFamilyProviderHealth      = "provider_health"
	NotificationFamilyRequestFailure      = "request_failure"

	NotificationTypeCheckinRunCompleted   = "checkin.run.completed"
	NotificationTypeCheckinProviderFailed = "checkin.provider.failed"
	NotificationTypeCheckinAutoDisabled   = "checkin.provider.auto_disabled"
	NotificationTypeProviderUnreachable   = "provider.health.unreachable"
	NotificationTypeProviderRecovered     = "provider.health.recovered"
	NotificationTypeRequestFailureAlert   = "relay.request_failure.threshold_reached"
)

type NotificationEvent struct {
	Type            string            `json:"type"`
	Family          string            `json:"family"`
	Severity        string            `json:"severity"`
	Summary         string            `json:"summary"`
	Detail          string            `json:"detail,omitempty"`
	DedupeKey       string            `json:"dedupe_key,omitempty"`
	OccurredAt      int64             `json:"occurred_at"`
	ProviderID      int               `json:"provider_id,omitempty"`
	ProviderName    string            `json:"provider_name,omitempty"`
	TriggerSource   string            `json:"trigger_source,omitempty"`
	FailureCategory string            `json:"failure_category,omitempty"`
	Reason          string            `json:"reason,omitempty"`
	Counts          map[string]int    `json:"counts,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

type NotificationDispatcher interface {
	Dispatch(ctx context.Context, settings model.NotificationSettings, event NotificationEvent) error
}

type notificationChannel interface {
	Name() string
	Deliver(ctx context.Context, settings model.NotificationSettings, event NotificationEvent) error
}

type fanoutNotificationDispatcher struct {
	channels []notificationChannel
}

type barkNotificationChannel struct {
	client *http.Client
}

type webhookNotificationChannel struct {
	client *http.Client
}

type smtpNotificationChannel struct{}

var notificationAsyncExecutor = func(fn func()) {
	go fn()
}

var notificationDispatchRunner = dispatchNotificationEvent

var newNotificationHTTPClient = func() *http.Client {
	return &http.Client{Timeout: 10 * time.Second}
}

var sendNotificationEmail = common.SendEmail

func NotifyAsync(event NotificationEvent) {
	if strings.TrimSpace(event.Type) == "" || strings.TrimSpace(event.Family) == "" {
		return
	}
	if event.OccurredAt == 0 {
		event.OccurredAt = time.Now().Unix()
	}
	event.Summary = strings.TrimSpace(event.Summary)
	event.Detail = strings.TrimSpace(event.Detail)
	event.Reason = strings.TrimSpace(event.Reason)
	event.DedupeKey = strings.TrimSpace(event.DedupeKey)
	event.ProviderName = strings.TrimSpace(event.ProviderName)
	event.TriggerSource = strings.TrimSpace(event.TriggerSource)
	event.FailureCategory = strings.TrimSpace(event.FailureCategory)
	notificationAsyncExecutor(func() {
		if err := notificationDispatchRunner(event); err != nil {
			common.SysLog(fmt.Sprintf("notification dispatch failed: type=%s err=%v", event.Type, err))
		}
	})
}

func NewNotificationDispatcher() NotificationDispatcher {
	client := newNotificationHTTPClient()
	return &fanoutNotificationDispatcher{
		channels: []notificationChannel{
			&barkNotificationChannel{client: client},
			&webhookNotificationChannel{client: client},
			&smtpNotificationChannel{},
		},
	}
}

func (d *fanoutNotificationDispatcher) Dispatch(ctx context.Context, settings model.NotificationSettings, event NotificationEvent) error {
	for _, channel := range d.channels {
		if err := channel.Deliver(ctx, settings, event); err != nil {
			common.SysLog(fmt.Sprintf("notification channel %s failed: %v", channel.Name(), err))
		}
	}
	return nil
}

func (c *barkNotificationChannel) Name() string {
	return "bark"
}

func (c *barkNotificationChannel) Deliver(ctx context.Context, settings model.NotificationSettings, event NotificationEvent) error {
	if !settings.Bark.Enabled {
		return nil
	}
	server := strings.TrimRight(strings.TrimSpace(settings.Bark.Server), "/")
	deviceKey := strings.TrimSpace(settings.Bark.DeviceKey)
	if server == "" || deviceKey == "" {
		return nil
	}
	bodyText := renderNotificationText(settings.Policy.VerbosityMode, event)
	payload := map[string]string{
		"device_key": deviceKey,
		"title":      buildNotificationTitle(event),
		"body":       bodyText,
	}
	if strings.TrimSpace(settings.Bark.Group) != "" {
		payload["group"] = strings.TrimSpace(settings.Bark.Group)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, server+"/push", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("bark responded with status %d", resp.StatusCode)
	}
	return nil
}

func (c *webhookNotificationChannel) Name() string {
	return "webhook"
}

func (c *webhookNotificationChannel) Deliver(ctx context.Context, settings model.NotificationSettings, event NotificationEvent) error {
	if !settings.Webhook.Enabled {
		return nil
	}
	webhookURL := strings.TrimSpace(settings.Webhook.URL)
	if webhookURL == "" {
		return nil
	}
	payload := buildWebhookPayload(settings.Policy.VerbosityMode, event)
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token := strings.TrimSpace(settings.Webhook.Token); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("webhook responded with status %d", resp.StatusCode)
	}
	return nil
}

func (c *smtpNotificationChannel) Name() string {
	return "smtp"
}

func (c *smtpNotificationChannel) Deliver(_ context.Context, settings model.NotificationSettings, event NotificationEvent) error {
	if !settings.SMTP.Enabled || len(settings.SMTP.Recipients) == 0 {
		return nil
	}
	subjectPrefix := strings.TrimSpace(settings.SMTP.SubjectPrefix)
	subject := buildNotificationTitle(event)
	if subjectPrefix != "" {
		subject = subjectPrefix + " " + subject
	}
	content := renderNotificationHTML(settings.Policy.VerbosityMode, event)
	return sendNotificationEmail(subject, strings.Join(settings.SMTP.Recipients, ";"), content)
}

func dispatchNotificationEvent(event NotificationEvent) error {
	settings := model.ParseNotificationSettings()
	if !isNotificationEventEnabled(settings.Policy, event.Family) {
		return nil
	}
	if !hasAnyNotificationChannelEnabled(settings) {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return NewNotificationDispatcher().Dispatch(ctx, settings, event)
}

func hasAnyNotificationChannelEnabled(settings model.NotificationSettings) bool {
	return settings.Bark.Enabled || settings.Webhook.Enabled || settings.SMTP.Enabled
}

func isNotificationEventEnabled(policy model.NotificationPolicyConfig, family string) bool {
	switch strings.TrimSpace(family) {
	case NotificationFamilyCheckinSummary:
		return policy.CheckinSummaryEnabled
	case NotificationFamilyCheckinFailure:
		return policy.CheckinFailureEnabled
	case NotificationFamilyProviderAutoDisable:
		return policy.ProviderAutoDisableEnabled
	case NotificationFamilyProviderHealth:
		return policy.ProviderHealthEnabled
	case NotificationFamilyRequestFailure:
		return policy.RequestFailureEnabled
	default:
		return false
	}
}

func buildNotificationTitle(event NotificationEvent) string {
	summary := strings.TrimSpace(event.Summary)
	if summary == "" {
		summary = strings.TrimSpace(event.Type)
	}
	return summary
}

func renderNotificationText(mode string, event NotificationEvent) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if strings.EqualFold(mode, model.NotificationVerbosityDetailed) {
		mode = model.NotificationVerbosityDetailed
	} else {
		mode = model.NotificationVerbosityConcise
	}
	lines := []string{strings.TrimSpace(event.Summary)}
	if mode == model.NotificationVerbosityDetailed {
		if detail := strings.TrimSpace(event.Detail); detail != "" {
			lines = append(lines, "", detail)
		}
		if event.ProviderName != "" {
			lines = append(lines, "", "Provider: "+event.ProviderName)
		}
		if event.TriggerSource != "" {
			lines = append(lines, "Trigger: "+event.TriggerSource)
		}
		if event.FailureCategory != "" {
			lines = append(lines, "Failure category: "+event.FailureCategory)
		}
		if event.Reason != "" {
			lines = append(lines, "Reason: "+event.Reason)
		}
		for _, line := range notificationCountLines(event.Counts) {
			lines = append(lines, line)
		}
		for _, line := range notificationMetadataLines(event.Metadata) {
			lines = append(lines, line)
		}
	}
	lines = append(lines, "", "Occurred at: "+time.Unix(event.OccurredAt, 0).Format(time.RFC3339))
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func renderNotificationHTML(mode string, event NotificationEvent) string {
	text := renderNotificationText(mode, event)
	return "<pre style=\"font-family:ui-monospace,SFMono-Regular,Menlo,monospace;white-space:pre-wrap;\">" + html.EscapeString(text) + "</pre>"
}

func buildWebhookPayload(mode string, event NotificationEvent) map[string]interface{} {
	payload := map[string]interface{}{
		"type":          event.Type,
		"family":        event.Family,
		"severity":      event.Severity,
		"summary":       event.Summary,
		"detail":        event.Detail,
		"dedupe_key":    event.DedupeKey,
		"occurred_at":   event.OccurredAt,
		"mode":          mode,
		"text":          renderNotificationText(mode, event),
		"provider_id":   event.ProviderID,
		"provider_name": event.ProviderName,
	}
	if event.TriggerSource != "" {
		payload["trigger_source"] = event.TriggerSource
	}
	if event.FailureCategory != "" {
		payload["failure_category"] = event.FailureCategory
	}
	if event.Reason != "" {
		payload["reason"] = event.Reason
	}
	if len(event.Counts) > 0 {
		payload["counts"] = event.Counts
	}
	if len(event.Metadata) > 0 {
		payload["metadata"] = event.Metadata
	}
	return payload
}

func notificationCountLines(counts map[string]int) []string {
	if len(counts) == 0 {
		return nil
	}
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		lines = append(lines, fmt.Sprintf("%s: %d", key, counts[key]))
	}
	return lines
}

func notificationMetadataLines(metadata map[string]string) []string {
	if len(metadata) == 0 {
		return nil
	}
	keys := make([]string, 0, len(metadata))
	for key := range metadata {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		value := strings.TrimSpace(metadata[key])
		if value == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s: %s", key, value))
	}
	return lines
}

func buildProviderHealthAlertKey(providerID int) string {
	return fmt.Sprintf("provider-health:%d", providerID)
}

func buildRequestFailureAlertKey(providerID int) string {
	return fmt.Sprintf("request-failure:%d", providerID)
}

func markProviderHealthAlertOpen(provider *model.Provider, reason string) {
	if provider == nil || provider.Id == 0 {
		return
	}
	now := time.Now().Unix()
	state, err := model.GetNotificationAlertState(buildProviderHealthAlertKey(provider.Id))
	if err != nil {
		common.SysLog("load provider notification alert state failed: " + err.Error())
		return
	}
	if state == nil {
		state = &model.NotificationAlertState{
			DedupeKey:   buildProviderHealthAlertKey(provider.Id),
			EventFamily: NotificationFamilyProviderHealth,
		}
	}
	state.Status = model.NotificationAlertStatusOpen
	state.LastObservedAt = now
	state.LastFiredAt = now
	state.LastSummary = strings.TrimSpace(reason)
	if err := model.SaveNotificationAlertState(state); err != nil {
		common.SysLog("save provider notification alert state failed: " + err.Error())
	}
}

func markProviderHealthAlertClosed(provider *model.Provider, reason string) {
	if provider == nil || provider.Id == 0 {
		return
	}
	now := time.Now().Unix()
	state, err := model.GetNotificationAlertState(buildProviderHealthAlertKey(provider.Id))
	if err != nil {
		common.SysLog("load provider notification alert state failed: " + err.Error())
		return
	}
	if state == nil {
		state = &model.NotificationAlertState{
			DedupeKey:   buildProviderHealthAlertKey(provider.Id),
			EventFamily: NotificationFamilyProviderHealth,
		}
	}
	state.Status = model.NotificationAlertStatusClosed
	state.LastObservedAt = now
	state.LastResolvedAt = now
	state.LastSummary = strings.TrimSpace(reason)
	if err := model.SaveNotificationAlertState(state); err != nil {
		common.SysLog("save provider notification alert state failed: " + err.Error())
	}
}

func EvaluateRequestFailureAlert(log *model.UsageLog) {
	if log == nil || log.ProviderId == 0 {
		return
	}
	if log.Status == 1 && strings.TrimSpace(log.ErrorMessage) == "" {
		return
	}
	settings := model.ParseNotificationSettings()
	if !settings.Policy.RequestFailureEnabled {
		return
	}
	windowMinutes := settings.Policy.RequestFailureWindowMinutes
	threshold := settings.Policy.RequestFailureThreshold
	if windowMinutes <= 0 || threshold <= 0 {
		return
	}
	now := log.CreatedAt
	if now == 0 {
		now = time.Now().Unix()
	}
	windowStart := now - int64(windowMinutes*60) + 1
	count, err := model.CountFailedUsageLogsSince(log.ProviderId, windowStart)
	if err != nil {
		common.SysLog("count request failure logs failed: " + err.Error())
		return
	}
	samples, err := model.GetRecentFailedUsageLogsSince(log.ProviderId, windowStart, 5)
	if err != nil {
		common.SysLog("query request failure samples failed: " + err.Error())
		return
	}
	state, err := model.GetNotificationAlertState(buildRequestFailureAlertKey(log.ProviderId))
	if err != nil {
		common.SysLog("load request failure alert state failed: " + err.Error())
		return
	}
	if state == nil {
		state = &model.NotificationAlertState{
			DedupeKey:   buildRequestFailureAlertKey(log.ProviderId),
			EventFamily: NotificationFamilyRequestFailure,
		}
	}
	if state.LastObservedAt > 0 && now-state.LastObservedAt > int64(windowMinutes*60) {
		state.Status = model.NotificationAlertStatusClosed
		state.WindowStartedAt = 0
		state.WindowCount = 0
	}
	state.LastObservedAt = now
	state.WindowStartedAt = windowStart
	state.WindowCount = int(count)
	state.LastSummary = buildRequestFailureSummary(samples)

	if count >= int64(threshold) && state.Status != model.NotificationAlertStatusOpen {
		event := buildRequestFailureNotificationEvent(log, samples, int(count), windowMinutes)
		state.Status = model.NotificationAlertStatusOpen
		state.LastFiredAt = now
		if err := model.SaveNotificationAlertState(state); err != nil {
			common.SysLog("save request failure alert state failed: " + err.Error())
		}
		NotifyAsync(event)
		return
	}
	if err := model.SaveNotificationAlertState(state); err != nil {
		common.SysLog("save request failure alert state failed: " + err.Error())
	}
}

func buildRequestFailureNotificationEvent(log *model.UsageLog, samples []*model.UsageLog, count int, windowMinutes int) NotificationEvent {
	reasonSummary, category := summarizeUsageFailureSamples(samples)
	detail := fmt.Sprintf("最近 %d 分钟内，提供商 %s 出现 %d 次失败请求。", windowMinutes, strings.TrimSpace(log.ProviderName), count)
	if reasonSummary != "" {
		detail += " 最近错误摘要：" + reasonSummary
	}
	return NotificationEvent{
		Type:            NotificationTypeRequestFailureAlert,
		Family:          NotificationFamilyRequestFailure,
		Severity:        "warning",
		Summary:         fmt.Sprintf("提供商 %s 请求失败达到阈值", strings.TrimSpace(log.ProviderName)),
		Detail:          detail,
		DedupeKey:       buildRequestFailureAlertKey(log.ProviderId),
		OccurredAt:      log.CreatedAt,
		ProviderID:      log.ProviderId,
		ProviderName:    strings.TrimSpace(log.ProviderName),
		FailureCategory: category,
		Reason:          reasonSummary,
		Counts: map[string]int{
			"failure_count":  count,
			"window_minutes": windowMinutes,
		},
	}
}

func summarizeUsageFailureSamples(samples []*model.UsageLog) (string, string) {
	reasons := make([]string, 0, len(samples))
	categoryCount := map[string]int{}
	for _, sample := range samples {
		if sample == nil {
			continue
		}
		if category := strings.TrimSpace(sample.FailureCategory); category != "" {
			categoryCount[category]++
		}
		reason := strings.TrimSpace(sample.ErrorMessage)
		if reason == "" {
			reason = strings.TrimSpace(sample.InvalidReason)
		}
		if reason != "" {
			reasons = append(reasons, reason)
		}
	}
	sort.SliceStable(reasons, func(i, j int) bool {
		return len(reasons[i]) < len(reasons[j])
	})
	category := ""
	categoryMax := 0
	for key, value := range categoryCount {
		if value > categoryMax || (value == categoryMax && key < category) {
			category = key
			categoryMax = value
		}
	}
	switch len(reasons) {
	case 0:
		return "", category
	case 1:
		return reasons[0], category
	default:
		return reasons[0] + " 等", category
	}
}

func buildRequestFailureSummary(samples []*model.UsageLog) string {
	summary, _ := summarizeUsageFailureSamples(samples)
	return summary
}

func buildCheckinSummaryNotificationEvent(run *model.CheckinRun) NotificationEvent {
	if run == nil {
		return NotificationEvent{}
	}
	return NotificationEvent{
		Type:          NotificationTypeCheckinRunCompleted,
		Family:        NotificationFamilyCheckinSummary,
		Severity:      "info",
		Summary:       fmt.Sprintf("签到任务完成：成功 %d，失败 %d，未签到 %d", run.SuccessCount, run.FailureCount, run.UncheckinCount),
		Detail:        strings.TrimSpace(run.Message),
		DedupeKey:     fmt.Sprintf("checkin-run:%d", run.Id),
		OccurredAt:    run.EndedAt,
		TriggerSource: strings.TrimSpace(run.TriggerType),
		Counts: map[string]int{
			"run_id":          run.Id,
			"total_count":     run.TotalCount,
			"success_count":   run.SuccessCount,
			"failure_count":   run.FailureCount,
			"uncheckin_count": run.UncheckinCount,
		},
		Metadata: map[string]string{
			"timezone":       strings.TrimSpace(run.Timezone),
			"scheduled_date": strings.TrimSpace(run.ScheduledDate),
			"status":         strings.TrimSpace(run.Status),
		},
	}
}

func buildCheckinFailureNotificationEvent(run *model.CheckinRun, item *model.CheckinRunItem) NotificationEvent {
	if run == nil || item == nil {
		return NotificationEvent{}
	}
	return NotificationEvent{
		Type:          NotificationTypeCheckinProviderFailed,
		Family:        NotificationFamilyCheckinFailure,
		Severity:      "warning",
		Summary:       fmt.Sprintf("提供商 %s 签到失败", strings.TrimSpace(item.ProviderName)),
		Detail:        strings.TrimSpace(item.Message),
		DedupeKey:     fmt.Sprintf("checkin-item:%d", item.Id),
		OccurredAt:    item.CheckedAt,
		ProviderID:    item.ProviderId,
		ProviderName:  strings.TrimSpace(item.ProviderName),
		TriggerSource: strings.TrimSpace(run.TriggerType),
		Reason:        strings.TrimSpace(item.Message),
		Counts: map[string]int{
			"run_id":        run.Id,
			"quota_awarded": int(item.QuotaAwarded),
			"run_failures":  run.FailureCount,
			"run_successes": run.SuccessCount,
			"run_total":     run.TotalCount,
		},
		Metadata: map[string]string{
			"run_status": strings.TrimSpace(run.Status),
		},
	}
}

func buildProviderAutoDisableNotificationEvent(run *model.CheckinRun, item *model.CheckinRunItem) NotificationEvent {
	if run == nil || item == nil {
		return NotificationEvent{}
	}
	return NotificationEvent{
		Type:          NotificationTypeCheckinAutoDisabled,
		Family:        NotificationFamilyProviderAutoDisable,
		Severity:      "critical",
		Summary:       fmt.Sprintf("提供商 %s 已自动关闭签到", strings.TrimSpace(item.ProviderName)),
		Detail:        strings.TrimSpace(item.Message),
		DedupeKey:     fmt.Sprintf("checkin-auto-disable:%d", item.Id),
		OccurredAt:    item.CheckedAt,
		ProviderID:    item.ProviderId,
		ProviderName:  strings.TrimSpace(item.ProviderName),
		TriggerSource: strings.TrimSpace(run.TriggerType),
		Reason:        strings.TrimSpace(item.Message),
		Counts: map[string]int{
			"run_id": run.Id,
		},
	}
}

func buildProviderUnreachableNotificationEvent(provider *model.Provider, reason string) NotificationEvent {
	if provider == nil {
		return NotificationEvent{}
	}
	occurredAt := provider.HealthFailureAt
	if occurredAt == 0 {
		occurredAt = time.Now().Unix()
	}
	return NotificationEvent{
		Type:         NotificationTypeProviderUnreachable,
		Family:       NotificationFamilyProviderHealth,
		Severity:     "critical",
		Summary:      fmt.Sprintf("提供商 %s 当前不可达", strings.TrimSpace(provider.Name)),
		Detail:       strings.TrimSpace(reason),
		DedupeKey:    buildProviderHealthAlertKey(provider.Id),
		OccurredAt:   occurredAt,
		ProviderID:   provider.Id,
		ProviderName: strings.TrimSpace(provider.Name),
		Reason:       strings.TrimSpace(reason),
		Metadata: map[string]string{
			"health_status": strings.TrimSpace(provider.HealthStatus),
		},
	}
}

func buildProviderRecoveryNotificationEvent(provider *model.Provider, previousReason string) NotificationEvent {
	if provider == nil {
		return NotificationEvent{}
	}
	occurredAt := provider.HealthSuccessAt
	if occurredAt == 0 {
		occurredAt = time.Now().Unix()
	}
	return NotificationEvent{
		Type:         NotificationTypeProviderRecovered,
		Family:       NotificationFamilyProviderHealth,
		Severity:     "info",
		Summary:      fmt.Sprintf("提供商 %s 已恢复可用", strings.TrimSpace(provider.Name)),
		Detail:       "提供商已从之前的不可达状态恢复。",
		DedupeKey:    buildProviderHealthAlertKey(provider.Id),
		OccurredAt:   occurredAt,
		ProviderID:   provider.Id,
		ProviderName: strings.TrimSpace(provider.Name),
		Reason:       strings.TrimSpace(previousReason),
		Metadata: map[string]string{
			"previous_reason": strings.TrimSpace(previousReason),
			"health_status":   strings.TrimSpace(provider.HealthStatus),
		},
	}
}
