package service

import (
	"NewAPI-Gateway/common"
	"NewAPI-Gateway/model"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

var ErrCheckinRunInProgress = errors.New("checkin run is already in progress")

const upstreamCheckinDisabledKeyword = "签到功能未启用"
const upstreamCheckinAutoDisableHint = "签到功能上游未启用，已自动关闭签到"
const manualUncheckedNoopMessage = "签到完成：今日无未签到渠道，无需执行"

type CheckinScheduleConfig struct {
	Enabled  bool
	Time     string
	Timezone string
}

var checkinRunGuard = make(chan struct{}, 1)
var scheduledCheckinState struct {
	mu         sync.Mutex
	lastRunDay string
}

func tryAcquireCheckinRun() bool {
	select {
	case checkinRunGuard <- struct{}{}:
		return true
	default:
		return false
	}
}

func releaseCheckinRun() {
	select {
	case <-checkinRunGuard:
	default:
	}
}

func parseScheduleTime(value string) (int, int, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return 0, 0, errors.New("empty schedule time")
	}
	parts := strings.Split(raw, ":")
	if len(parts) != 2 {
		return 0, 0, errors.New("invalid schedule time format")
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return 0, 0, errors.New("invalid schedule hour")
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return 0, 0, errors.New("invalid schedule minute")
	}
	return hour, minute, nil
}

func resolveLocation(timezone string) *time.Location {
	tz := strings.TrimSpace(timezone)
	if tz == "" {
		tz = common.CheckinScheduleTimezone
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		common.SysLog("invalid checkin timezone: " + tz + ", fallback to UTC")
		return time.UTC
	}
	return loc
}

func GetCheckinScheduleConfig() CheckinScheduleConfig {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()

	enabledRaw := strings.TrimSpace(common.OptionMap["CheckinScheduleEnabled"])
	timeRaw := strings.TrimSpace(common.OptionMap["CheckinScheduleTime"])
	timezoneRaw := strings.TrimSpace(common.OptionMap["CheckinScheduleTimezone"])

	if enabledRaw == "" {
		enabledRaw = strconv.FormatBool(common.CheckinScheduleEnabled)
	}
	if timeRaw == "" {
		timeRaw = common.CheckinScheduleTime
	}
	if timezoneRaw == "" {
		timezoneRaw = common.CheckinScheduleTimezone
	}

	return CheckinScheduleConfig{
		Enabled:  strings.EqualFold(enabledRaw, "true"),
		Time:     timeRaw,
		Timezone: timezoneRaw,
	}
}

func checkinDayStart(now time.Time, timezone string) (int64, string, *time.Location) {
	loc := resolveLocation(timezone)
	localNow := now.In(loc)
	dayStart := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, loc)
	return dayStart.Unix(), localNow.Format("2006-01-02"), loc
}

func ShouldRunScheduledCheckin(now time.Time, hour int, minute int, loc *time.Location, lastRunDay string) (bool, string) {
	if loc == nil {
		loc = time.UTC
	}
	localNow := now.In(loc)
	dayKey := localNow.Format("2006-01-02")
	if lastRunDay == dayKey {
		return false, dayKey
	}
	if localNow.Hour() < hour {
		return false, dayKey
	}
	if localNow.Hour() == hour && localNow.Minute() < minute {
		return false, dayKey
	}
	return true, dayKey
}

func buildCheckinRunMessage(successCount int, failureCount int, uncheckinCount int) string {
	return fmt.Sprintf("签到完成：成功 %d，失败 %d，未签到 %d", successCount, failureCount, uncheckinCount)
}

func buildUncheckedCheckinRunMessage(successCount int, failureCount int, uncheckinCount int) string {
	return fmt.Sprintf("未签到渠道签到完成：成功 %d，失败 %d，未签到 %d", successCount, failureCount, uncheckinCount)
}

func normalizeCheckinFailureMessage(message string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(message))), "")
}

func isUpstreamCheckinDisabledFailure(message string) bool {
	normalized := normalizeCheckinFailureMessage(message)
	if normalized == "" {
		return false
	}
	keyword := normalizeCheckinFailureMessage(upstreamCheckinDisabledKeyword)
	return strings.Contains(normalized, keyword)
}

func appendUpstreamCheckinAutoDisableHint(message string) string {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return upstreamCheckinAutoDisableHint
	}
	if strings.Contains(trimmed, upstreamCheckinAutoDisableHint) {
		return trimmed
	}
	return fmt.Sprintf("%s（%s）", trimmed, upstreamCheckinAutoDisableHint)
}

func checkinProviderWithResult(provider *model.Provider) (*model.CheckinRunItem, error) {
	checkedAt := time.Now().Unix()
	item := &model.CheckinRunItem{
		ProviderId:   provider.Id,
		ProviderName: provider.Name,
		Status:       "failed",
		CheckedAt:    checkedAt,
	}

	client, clientErr := NewUpstreamClientForProvider(provider)
	if clientErr != nil {
		item.Message = strings.TrimSpace(sanitizeProviderErrorMessage(provider, clientErr.Error()))
		if reason, ok := classifyProviderReachabilityError(clientErr); ok {
			markProviderHealthFailure(provider, reason)
		}
		return item, clientErr
	}
	result, err := client.Checkin()
	if err != nil {
		item.Message = strings.TrimSpace(sanitizeProviderErrorMessage(provider, err.Error()))
		if isUpstreamCheckinDisabledFailure(item.Message) {
			if disableErr := provider.UpdateCheckinEnabled(false); disableErr != nil {
				common.SysLog(fmt.Sprintf("failed to auto-disable checkin for provider %s: %v", provider.Name, disableErr))
			} else {
				item.AutoDisabled = true
				item.Message = appendUpstreamCheckinAutoDisableHint(item.Message)
				common.SysLog(fmt.Sprintf("provider %s checkin auto-disabled due to upstream unavailable", provider.Name))
			}
			return item, err
		}
		if reason, ok := classifyProviderReachabilityError(err); ok {
			markProviderHealthFailure(provider, reason)
		}
		return item, err
	}

	provider.UpdateCheckinTime()
	markProviderHealthSuccess(provider)
	item.Status = "success"
	item.Message = strings.TrimSpace(result.Message)
	if item.Message == "" {
		item.Message = "签到成功"
	}
	item.QuotaAwarded = result.QuotaAwarded
	common.SysLog(fmt.Sprintf("provider %s checkin success, quota_awarded: %d, message: %s", provider.Name, result.QuotaAwarded, item.Message))
	return item, nil
}

func getUncheckinCount(now time.Time, timezone string) int {
	dayStart, _, _ := checkinDayStart(now, timezone)
	providers, err := model.GetUncheckinProviders(dayStart)
	if err != nil {
		common.SysLog("failed to count uncheckin providers: " + err.Error())
		return 0
	}
	return len(providers)
}

func executeCheckinRun(
	triggerType string,
	startedAt time.Time,
	timezone string,
	dayStart int64,
	dayKey string,
	providers []*model.Provider,
	emptyMessage string,
	summaryBuilder func(successCount int, failureCount int, uncheckinCount int) string,
) (*model.CheckinRun, error) {
	run := &model.CheckinRun{
		TriggerType:    triggerType,
		Status:         "running",
		Timezone:       timezone,
		ScheduledDate:  dayKey,
		StartedAt:      startedAt.Unix(),
		TotalCount:     len(providers),
		SuccessCount:   0,
		FailureCount:   0,
		UncheckinCount: 0,
		Message:        "签到任务执行中",
	}
	if err := run.Insert(); err != nil {
		return nil, err
	}

	items := make([]*model.CheckinRunItem, 0, len(providers))
	for _, provider := range providers {
		item, checkinErr := checkinProviderWithResult(provider)
		item.RunId = run.Id
		items = append(items, item)
		if checkinErr != nil {
			run.FailureCount++
			common.SysLog(fmt.Sprintf("checkin failed for provider %s: %v", provider.Name, checkinErr))
			continue
		}
		run.SuccessCount++
	}

	if err := model.InsertCheckinRunItems(items); err != nil {
		common.SysLog("insert checkin run items failed: " + err.Error())
	}

	uncheckinProviders, err := model.GetUncheckinProviders(dayStart)
	if err != nil {
		common.SysLog("query uncheckin providers failed: " + err.Error())
	}
	run.UncheckinCount = len(uncheckinProviders)
	run.EndedAt = time.Now().Unix()
	run.Status = "success"
	if run.FailureCount > 0 {
		run.Status = "partial"
	}
	if len(providers) == 0 && strings.TrimSpace(emptyMessage) != "" {
		run.Message = emptyMessage
	} else {
		if summaryBuilder == nil {
			summaryBuilder = buildCheckinRunMessage
		}
		run.Message = summaryBuilder(run.SuccessCount, run.FailureCount, run.UncheckinCount)
	}
	if err := run.Update(); err != nil {
		common.SysLog("update checkin run summary failed: " + err.Error())
	}
	return run, nil
}

func triggerCheckinRun(
	triggerType string,
	providerLoader func(dayStart int64) ([]*model.Provider, error),
	emptyMessage string,
	summaryBuilder func(successCount int, failureCount int, uncheckinCount int) string,
) (*model.CheckinRun, error) {
	if !tryAcquireCheckinRun() {
		return nil, ErrCheckinRunInProgress
	}
	defer releaseCheckinRun()

	config := GetCheckinScheduleConfig()
	startedAt := time.Now()
	dayStart, dayKey, _ := checkinDayStart(startedAt, config.Timezone)
	providers, err := providerLoader(dayStart)
	if err != nil {
		return nil, err
	}
	return executeCheckinRun(triggerType, startedAt, config.Timezone, dayStart, dayKey, providers, emptyMessage, summaryBuilder)
}

func TriggerFullCheckinRun(triggerType string) (*model.CheckinRun, error) {
	return triggerCheckinRun(triggerType, func(_ int64) ([]*model.Provider, error) {
		return model.GetCheckinEnabledProviders()
	}, "", nil)
}

func TriggerUncheckedCheckinRun(triggerType string) (*model.CheckinRun, error) {
	return triggerCheckinRun(triggerType, model.GetUncheckinProviders, manualUncheckedNoopMessage, buildUncheckedCheckinRunMessage)
}

func RunScheduledCheckinIfNeeded(now time.Time) {
	config := GetCheckinScheduleConfig()
	if !config.Enabled {
		return
	}

	hour, minute, err := parseScheduleTime(config.Time)
	if err != nil {
		common.SysLog("invalid checkin schedule time: " + config.Time)
		return
	}
	loc := resolveLocation(config.Timezone)

	scheduledCheckinState.mu.Lock()
	lastRunDay := scheduledCheckinState.lastRunDay
	scheduledCheckinState.mu.Unlock()

	shouldRun, dayKey := ShouldRunScheduledCheckin(now, hour, minute, loc, lastRunDay)
	if !shouldRun {
		return
	}
	alreadyRan, queryErr := model.ExistsCheckinRunByTriggerAndScheduledDate("cron", dayKey)
	if queryErr != nil {
		common.SysLog("scheduled checkin dedupe query failed: " + queryErr.Error())
		return
	}
	if alreadyRan {
		scheduledCheckinState.mu.Lock()
		scheduledCheckinState.lastRunDay = dayKey
		scheduledCheckinState.mu.Unlock()
		common.SysLog("skip scheduled checkin: cron run already exists for day " + dayKey)
		return
	}

	run, err := TriggerFullCheckinRun("cron")
	if err != nil {
		if errors.Is(err, ErrCheckinRunInProgress) {
			common.SysLog("skip scheduled checkin: another run is in progress")
			return
		}
		common.SysLog("scheduled checkin failed to start: " + err.Error())
		return
	}

	scheduledCheckinState.mu.Lock()
	scheduledCheckinState.lastRunDay = dayKey
	scheduledCheckinState.mu.Unlock()
	common.SysLog(fmt.Sprintf("scheduled checkin run finished: run_id=%d, success=%d, failure=%d", run.Id, run.SuccessCount, run.FailureCount))
}

func RunProviderCheckin(provider *model.Provider) (*model.CheckinRun, *model.CheckinRunItem, error) {
	config := GetCheckinScheduleConfig()
	startedAt := time.Now()
	_, dayKey, _ := checkinDayStart(startedAt, config.Timezone)

	run := &model.CheckinRun{
		TriggerType:    "manual-provider",
		Status:         "running",
		Timezone:       config.Timezone,
		ScheduledDate:  dayKey,
		StartedAt:      startedAt.Unix(),
		TotalCount:     1,
		SuccessCount:   0,
		FailureCount:   0,
		UncheckinCount: 0,
		Message:        "单供应商签到执行中",
	}
	if err := run.Insert(); err != nil {
		return nil, nil, err
	}

	item, err := checkinProviderWithResult(provider)
	item.RunId = run.Id
	if err != nil {
		run.FailureCount = 1
		run.Status = "failed"
		run.Message = "签到失败：" + item.Message
	} else {
		run.SuccessCount = 1
		run.Status = "success"
		run.Message = strings.TrimSpace(item.Message)
		if run.Message == "" {
			run.Message = "签到成功"
		}
	}
	if insertErr := model.InsertCheckinRunItems([]*model.CheckinRunItem{item}); insertErr != nil {
		common.SysLog("insert provider checkin item failed: " + insertErr.Error())
	}

	run.UncheckinCount = getUncheckinCount(time.Now(), config.Timezone)
	run.EndedAt = time.Now().Unix()
	if updateErr := run.Update(); updateErr != nil {
		common.SysLog("update provider checkin run failed: " + updateErr.Error())
	}

	if err != nil {
		return run, item, err
	}
	return run, item, nil
}

// CheckinProvider performs and records a checkin for one provider.
func CheckinProvider(provider *model.Provider) error {
	_, _, err := RunProviderCheckin(provider)
	return err
}

// CheckinAllProviders performs and records a full checkin run.
func CheckinAllProviders() {
	if _, err := TriggerFullCheckinRun("cron"); err != nil {
		if errors.Is(err, ErrCheckinRunInProgress) {
			common.SysLog("skip checkin: another run is in progress")
			return
		}
		common.SysLog("checkin run failed: " + err.Error())
	}
}

func GetUncheckinProviders(now time.Time) ([]*model.Provider, int64, string, error) {
	config := GetCheckinScheduleConfig()
	dayStart, _, _ := checkinDayStart(now, config.Timezone)
	providers, err := model.GetUncheckinProviders(dayStart)
	return providers, dayStart, config.Timezone, err
}
