package service

import (
	"NewAPI-Gateway/common"
	"NewAPI-Gateway/model"
	"os"
	"testing"
	"time"
)

func TestRetryBackoffCapsAt24Hours(t *testing.T) {
	got := retryBackoff(30*time.Second, 20)
	if got != 24*time.Hour {
		t.Fatalf("expected capped backoff to 24h, got %s", got)
	}
}

func TestShouldRunBackupByDirty(t *testing.T) {
	backupState.mu.Lock()
	backupState.dirty = true
	backupState.lastDirtyAt = time.Now().Add(-2 * time.Minute)
	backupState.lastRunAt = time.Now().Add(-20 * time.Minute)
	backupState.mu.Unlock()

	cfg := BackupConfig{Debounce: 30 * time.Second, MinInterval: 10 * time.Minute}
	if !shouldRunBackupByDirty(time.Now(), cfg) {
		t.Fatalf("expected dirty backup to run")
	}

	backupState.mu.Lock()
	backupState.lastDirtyAt = time.Now()
	backupState.mu.Unlock()
	if shouldRunBackupByDirty(time.Now(), cfg) {
		t.Fatalf("expected debounce to block run")
	}

	backupState.mu.Lock()
	backupState.lastDirtyAt = time.Now().Add(-2 * time.Minute)
	backupState.lastRunAt = time.Now()
	backupState.mu.Unlock()
	if shouldRunBackupByDirty(time.Now(), cfg) {
		t.Fatalf("expected min interval to block run")
	}
}

func TestEvaluateBackupScheduleIfNeededTriggersScheduleWithoutDirty(t *testing.T) {
	originOptionMap := common.OptionMap
	originTrigger := backupTriggerFunc
	originRetryQueue := processBackupRetryQueue
	origDriver := os.Getenv("SQL_DRIVER")
	origDSN := os.Getenv("SQL_DSN")

	common.OptionMapRWMutex.Lock()
	common.OptionMap = map[string]string{
		model.BackupEnabledOptionKey:            "true",
		model.BackupTriggerModeOptionKey:        model.BackupTriggerModeSchedule,
		model.BackupScheduleCronOptionKey:       "* * * * *",
		model.BackupMinIntervalSecondsOptionKey: "0",
		model.BackupDebounceSecondsOptionKey:    "0",
	}
	common.OptionMapRWMutex.Unlock()
	_ = os.Setenv("SQL_DRIVER", "sqlite")
	_ = os.Setenv("SQL_DSN", os.TempDir()+"/schedule-trigger-test.db")

	triggered := make(chan string, 1)
	backupTriggerFunc = func(trigger string) (*model.BackupRun, error) {
		triggered <- trigger
		return &model.BackupRun{}, nil
	}
	processBackupRetryQueue = func(_ time.Time) {}

	backupState.mu.Lock()
	backupState.dirty = false
	backupState.lastDirtyAt = time.Time{}
	backupState.lastRunAt = time.Time{}
	backupState.running = false
	backupState.lastScheduleMinute = 0
	backupState.lastScheduleEvalAt = 0
	backupState.mu.Unlock()

	defer func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originOptionMap
		common.OptionMapRWMutex.Unlock()
		backupTriggerFunc = originTrigger
		processBackupRetryQueue = originRetryQueue
		_ = os.Setenv("SQL_DRIVER", origDriver)
		_ = os.Setenv("SQL_DSN", origDSN)
	}()

	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	EvaluateBackupScheduleIfNeeded(now)
	select {
	case got := <-triggered:
		if got != "schedule" {
			t.Fatalf("expected schedule trigger, got %s", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("expected schedule backup trigger")
	}
}
