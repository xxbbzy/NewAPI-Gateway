package service

import (
	"NewAPI-Gateway/common"
	"NewAPI-Gateway/model"
	"testing"
	"time"
)

func TestParseScheduleTime(t *testing.T) {
	hour, minute, err := parseScheduleTime("09:30")
	if err != nil {
		t.Fatalf("expected parse success, got error: %v", err)
	}
	if hour != 9 || minute != 30 {
		t.Fatalf("unexpected parse result: hour=%d minute=%d", hour, minute)
	}

	if _, _, err := parseScheduleTime("24:00"); err == nil {
		t.Fatalf("expected error for invalid hour")
	}
	if _, _, err := parseScheduleTime("08:60"); err == nil {
		t.Fatalf("expected error for invalid minute")
	}
	if _, _, err := parseScheduleTime("0830"); err == nil {
		t.Fatalf("expected error for invalid format")
	}
}

func TestShouldRunScheduledCheckin(t *testing.T) {
	loc := time.FixedZone("UTC+8", 8*3600)
	now := time.Date(2026, 2, 24, 10, 5, 0, 0, loc)

	shouldRun, dayKey := ShouldRunScheduledCheckin(now, 9, 0, loc, "")
	if !shouldRun {
		t.Fatalf("expected scheduled run to trigger")
	}
	if dayKey != "2026-02-24" {
		t.Fatalf("unexpected day key: %s", dayKey)
	}

	shouldRun, _ = ShouldRunScheduledCheckin(now, 11, 0, loc, "")
	if shouldRun {
		t.Fatalf("should not run before configured time")
	}

	shouldRun, _ = ShouldRunScheduledCheckin(now, 9, 0, loc, "2026-02-24")
	if shouldRun {
		t.Fatalf("should not run again for the same day")
	}
}

func TestScheduleTimeUpdateAffectsSubsequentDecision(t *testing.T) {
	originOptionMap := common.OptionMap
	originDefaultEnabled := common.CheckinScheduleEnabled
	originDefaultTime := common.CheckinScheduleTime
	originDefaultTimezone := common.CheckinScheduleTimezone
	defer func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originOptionMap
		common.OptionMapRWMutex.Unlock()
		common.CheckinScheduleEnabled = originDefaultEnabled
		common.CheckinScheduleTime = originDefaultTime
		common.CheckinScheduleTimezone = originDefaultTimezone
	}()

	common.OptionMapRWMutex.Lock()
	common.OptionMap = map[string]string{
		"CheckinScheduleEnabled":  "true",
		"CheckinScheduleTime":     "09:00",
		"CheckinScheduleTimezone": "UTC",
	}
	common.OptionMapRWMutex.Unlock()

	now := time.Date(2026, 2, 24, 10, 0, 0, 0, time.UTC)
	config := GetCheckinScheduleConfig()
	hour, minute, err := parseScheduleTime(config.Time)
	if err != nil {
		t.Fatalf("parse schedule time failed: %v", err)
	}
	shouldRun, _ := ShouldRunScheduledCheckin(now, hour, minute, resolveLocation(config.Timezone), "")
	if !shouldRun {
		t.Fatalf("expected run for original schedule time")
	}

	common.OptionMapRWMutex.Lock()
	common.OptionMap["CheckinScheduleTime"] = "11:00"
	common.OptionMapRWMutex.Unlock()

	updatedConfig := GetCheckinScheduleConfig()
	updatedHour, updatedMinute, err := parseScheduleTime(updatedConfig.Time)
	if err != nil {
		t.Fatalf("parse updated schedule time failed: %v", err)
	}
	shouldRun, _ = ShouldRunScheduledCheckin(now, updatedHour, updatedMinute, resolveLocation(updatedConfig.Timezone), "")
	if shouldRun {
		t.Fatalf("expected scheduler to use updated time and not run before 11:00")
	}
}

func TestScheduleTimezoneUpdateAffectsSubsequentDecision(t *testing.T) {
	originOptionMap := common.OptionMap
	originDefaultEnabled := common.CheckinScheduleEnabled
	originDefaultTime := common.CheckinScheduleTime
	originDefaultTimezone := common.CheckinScheduleTimezone
	defer func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originOptionMap
		common.OptionMapRWMutex.Unlock()
		common.CheckinScheduleEnabled = originDefaultEnabled
		common.CheckinScheduleTime = originDefaultTime
		common.CheckinScheduleTimezone = originDefaultTimezone
	}()

	common.OptionMapRWMutex.Lock()
	common.OptionMap = map[string]string{
		"CheckinScheduleEnabled":  "true",
		"CheckinScheduleTime":     "09:00",
		"CheckinScheduleTimezone": "Asia/Shanghai",
	}
	common.OptionMapRWMutex.Unlock()

	now := time.Date(2026, 2, 24, 14, 30, 0, 0, time.UTC)
	config := GetCheckinScheduleConfig()
	hour, minute, err := parseScheduleTime(config.Time)
	if err != nil {
		t.Fatalf("parse schedule time failed: %v", err)
	}
	shouldRun, _ := ShouldRunScheduledCheckin(now, hour, minute, resolveLocation(config.Timezone), "")
	if !shouldRun {
		t.Fatalf("expected run for Asia/Shanghai timezone")
	}

	common.OptionMapRWMutex.Lock()
	common.OptionMap["CheckinScheduleTimezone"] = "America/Los_Angeles"
	common.OptionMapRWMutex.Unlock()

	updatedConfig := GetCheckinScheduleConfig()
	shouldRun, _ = ShouldRunScheduledCheckin(now, hour, minute, resolveLocation(updatedConfig.Timezone), "")
	if shouldRun {
		t.Fatalf("expected scheduler to use updated timezone and not run before local 09:00")
	}
}

func TestRunScheduledCheckinIfNeededSkipsWhenCronRunAlreadyExistsForDay(t *testing.T) {
	originDB := model.DB
	model.DB = prepareServiceCheckinTestDB(t)
	defer func() { model.DB = originDB }()

	originOptionMap := common.OptionMap
	originDefaultEnabled := common.CheckinScheduleEnabled
	originDefaultTime := common.CheckinScheduleTime
	originDefaultTimezone := common.CheckinScheduleTimezone
	defer func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originOptionMap
		common.OptionMapRWMutex.Unlock()
		common.CheckinScheduleEnabled = originDefaultEnabled
		common.CheckinScheduleTime = originDefaultTime
		common.CheckinScheduleTimezone = originDefaultTimezone
	}()

	scheduledCheckinState.mu.Lock()
	scheduledCheckinState.lastRunDay = ""
	scheduledCheckinState.mu.Unlock()

	common.OptionMapRWMutex.Lock()
	common.OptionMap = map[string]string{
		"CheckinScheduleEnabled":  "true",
		"CheckinScheduleTime":     "09:00",
		"CheckinScheduleTimezone": "UTC",
	}
	common.OptionMapRWMutex.Unlock()

	existingRun := &model.CheckinRun{
		TriggerType:   "cron",
		Status:        "success",
		ScheduledDate: "2026-02-24",
	}
	if err := existingRun.Insert(); err != nil {
		t.Fatalf("insert existing cron run failed: %v", err)
	}

	RunScheduledCheckinIfNeeded(time.Date(2026, 2, 24, 10, 0, 0, 0, time.UTC))

	runs, err := model.GetRecentCheckinRuns(10)
	if err != nil {
		t.Fatalf("query checkin runs failed: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected no duplicate cron run, got %d runs", len(runs))
	}
	if runs[0].Id != existingRun.Id {
		t.Fatalf("unexpected run inserted, newest run id=%d existing=%d", runs[0].Id, existingRun.Id)
	}
}

func TestRunScheduledCheckinIfNeededAllowsRunOnNextDay(t *testing.T) {
	originDB := model.DB
	model.DB = prepareServiceCheckinTestDB(t)
	defer func() { model.DB = originDB }()

	originOptionMap := common.OptionMap
	originDefaultEnabled := common.CheckinScheduleEnabled
	originDefaultTime := common.CheckinScheduleTime
	originDefaultTimezone := common.CheckinScheduleTimezone
	defer func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originOptionMap
		common.OptionMapRWMutex.Unlock()
		common.CheckinScheduleEnabled = originDefaultEnabled
		common.CheckinScheduleTime = originDefaultTime
		common.CheckinScheduleTimezone = originDefaultTimezone
	}()

	scheduledCheckinState.mu.Lock()
	scheduledCheckinState.lastRunDay = ""
	scheduledCheckinState.mu.Unlock()

	nowUTC := time.Now().UTC()
	previousDay := nowUTC.Add(-24 * time.Hour).Format("2006-01-02")
	currentDay := nowUTC.Format("2006-01-02")

	common.OptionMapRWMutex.Lock()
	common.OptionMap = map[string]string{
		"CheckinScheduleEnabled":  "true",
		"CheckinScheduleTime":     "00:00",
		"CheckinScheduleTimezone": "UTC",
	}
	common.OptionMapRWMutex.Unlock()

	existingRun := &model.CheckinRun{
		TriggerType:   "cron",
		Status:        "success",
		ScheduledDate: previousDay,
	}
	if err := existingRun.Insert(); err != nil {
		t.Fatalf("insert existing cron run failed: %v", err)
	}

	RunScheduledCheckinIfNeeded(nowUTC)

	exists, err := model.ExistsCheckinRunByTriggerAndScheduledDate("cron", currentDay)
	if err != nil {
		t.Fatalf("query next-day cron run failed: %v", err)
	}
	if !exists {
		t.Fatalf("expected scheduler to allow cron run on next day")
	}

	runs, err := model.GetRecentCheckinRuns(10)
	if err != nil {
		t.Fatalf("query checkin runs failed: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("expected 2 runs after next-day trigger, got %d", len(runs))
	}
}
