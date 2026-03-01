package service

import (
	"NewAPI-Gateway/model"
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	backupState struct {
		mu                 sync.Mutex
		dirty              bool
		dirtyReason        string
		lastDirtyAt        time.Time
		lastRunAt          time.Time
		running            bool
		lastScheduleMinute int64
		lastScheduleEvalAt int64
	}
	backupTriggerFunc       = TriggerBackupNow
	processBackupRetryQueue = ProcessBackupRetryQueue
)

func MarkBackupDirty(reason string) {
	cfg := GetBackupConfig()
	if !cfg.Enabled || !shouldAllowEventTrigger(cfg.TriggerMode) {
		return
	}
	backupState.mu.Lock()
	defer backupState.mu.Unlock()
	backupState.dirty = true
	backupState.dirtyReason = strings.TrimSpace(reason)
	backupState.lastDirtyAt = time.Now()
}

func EvaluateBackupScheduleIfNeeded(now time.Time) {
	cfg := GetBackupConfig()
	backupState.mu.Lock()
	backupState.lastScheduleEvalAt = now.Unix()
	running := backupState.running
	backupState.mu.Unlock()
	if !cfg.Enabled || running {
		return
	}

	if shouldAllowEventTrigger(cfg.TriggerMode) {
		if shouldRunBackupByDirty(now, cfg) {
			go runBackupAsync("event")
			return
		}
	}
	if shouldAllowScheduleTrigger(cfg.TriggerMode) {
		matched, err := shouldRunCronExpression(cfg.ScheduleCron, now)
		if err != nil {
			logBackupError("invalid backup cron: " + err.Error())
			return
		}
		if matched {
			minuteKey := now.Unix() / 60
			backupState.mu.Lock()
			alreadyRanThisMinute := backupState.lastScheduleMinute == minuteKey
			backupState.mu.Unlock()
			if !alreadyRanThisMinute && canRunByInterval(now, cfg) {
				go runBackupAsync("schedule")
			}
		}
	}
	go processBackupRetryQueue(now)
}

func shouldRunBackupByDirty(now time.Time, cfg BackupConfig) bool {
	backupState.mu.Lock()
	defer backupState.mu.Unlock()
	if !backupState.dirty {
		return false
	}
	if cfg.Debounce > 0 && now.Sub(backupState.lastDirtyAt) < cfg.Debounce {
		return false
	}
	if cfg.MinInterval > 0 && !backupState.lastRunAt.IsZero() && now.Sub(backupState.lastRunAt) < cfg.MinInterval {
		return false
	}
	return true
}

func canRunByInterval(now time.Time, cfg BackupConfig) bool {
	backupState.mu.Lock()
	defer backupState.mu.Unlock()
	if cfg.MinInterval > 0 && !backupState.lastRunAt.IsZero() && now.Sub(backupState.lastRunAt) < cfg.MinInterval {
		return false
	}
	return true
}

func runBackupAsync(trigger string) {
	if _, err := backupTriggerFunc(trigger); err != nil {
		logBackupError("run backup failed: " + err.Error())
	}
}

func TriggerBackupNow(trigger string) (*model.BackupRun, error) {
	cfg := GetBackupConfig()
	if !cfg.Enabled {
		return nil, errors.New("backup is disabled")
	}
	backupState.mu.Lock()
	if backupState.running {
		backupState.mu.Unlock()
		return nil, errors.New("backup is already running")
	}
	backupState.running = true
	backupState.mu.Unlock()
	defer func() {
		backupState.mu.Lock()
		backupState.running = false
		backupState.mu.Unlock()
	}()

	now := time.Now()
	driver, dsn, err := getActiveSQLDriverAndDSN()
	if err != nil {
		return nil, err
	}
	if err := ensureBackupSpoolDir(cfg.SpoolDir); err != nil {
		return nil, err
	}
	workDir := filepath.Join(cfg.SpoolDir, fmt.Sprintf("run-%d", now.UnixNano()))
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return nil, err
	}
	cleanupWorkDir := true
	run := &model.BackupRun{
		TriggerType: trigger,
		DBDriver:    driver,
		Status:      "running",
		StartedAt:   now.Unix(),
		CreatedAt:   now.Unix(),
	}
	if err := run.Insert(); err != nil {
		return nil, err
	}

	finishRun := func(status string, artifactPath string, remotePath string, artifactSize int64, err error) (*model.BackupRun, error) {
		run.Status = status
		run.ArtifactPath = artifactPath
		run.RemotePath = remotePath
		run.ArtifactSize = artifactSize
		run.EndedAt = time.Now().Unix()
		run.DurationMs = maxInt64(0, (run.EndedAt-run.StartedAt)*1000)
		if err != nil {
			run.ErrorMessage = redactBackupMessage(err.Error())
		}
		_ = run.Update()
		backupState.mu.Lock()
		backupState.lastRunAt = time.Now()
		if status == "success" {
			backupState.dirty = false
			backupState.dirtyReason = ""
		}
		if trigger == "schedule" {
			backupState.lastScheduleMinute = backupState.lastRunAt.Unix() / 60
		}
		backupState.mu.Unlock()
		if cleanupWorkDir {
			_ = os.RemoveAll(workDir)
		}
		if err != nil {
			return run, err
		}
		return run, nil
	}

	ctx := context.Background()
	snapshotter, err := getSnapshotter(driver, dsn, cfg)
	if err != nil {
		return finishRun("failed", "", "", 0, err)
	}
	if err := snapshotter.Preflight(); err != nil {
		return finishRun("failed", "", "", 0, err)
	}
	snapshot, err := snapshotter.CreateSnapshot(ctx, workDir)
	if err != nil {
		return finishRun("failed", "", "", 0, err)
	}
	artifact, err := buildBackupArtifact(workDir, snapshot, driver, trigger, cfg)
	if err != nil {
		return finishRun("failed", "", "", 0, err)
	}

	client, err := newWebDAVClient(cfg)
	if err != nil {
		cleanupWorkDir = false
		_ = enqueueBackupRetry(run.Id, artifact.LocalPath, "", cfg, err)
		return finishRun("failed", artifact.LocalPath, "", artifact.PayloadSize, err)
	}
	if err := client.ensureCollection(cfg.WebDAVBasePath); err != nil {
		cleanupWorkDir = false
		_ = enqueueBackupRetry(run.Id, artifact.LocalPath, "", cfg, err)
		return finishRun("failed", artifact.LocalPath, "", artifact.PayloadSize, err)
	}
	remotePath := path.Join(cfg.WebDAVBasePath, artifact.RelativeName)
	if err := client.uploadFile(artifact.LocalPath, remotePath); err != nil {
		cleanupWorkDir = false
		_ = enqueueBackupRetry(run.Id, artifact.LocalPath, remotePath, cfg, err)
		return finishRun("failed", artifact.LocalPath, remotePath, artifact.PayloadSize, err)
	}
	_ = applyWebDAVRetention(client, cfg.WebDAVBasePath, cfg.RetentionDays, cfg.RetentionMaxFiles)
	_ = os.Remove(artifact.LocalPath)
	return finishRun("success", artifact.LocalPath, remotePath, artifact.PayloadSize, nil)
}

func enqueueBackupRetry(runID int, artifactPath string, remotePath string, cfg BackupConfig, err error) error {
	now := time.Now().Unix()
	row := &model.BackupUploadRetry{
		RunId:        runID,
		ArtifactPath: artifactPath,
		RemotePath:   remotePath,
		RetryCount:   0,
		MaxRetries:   cfg.MaxRetries,
		NextRetryAt:  now + int64(cfg.RetryBase.Seconds()),
		LastError:    redactBackupMessage(err.Error()),
		Status:       "pending",
	}
	return row.Insert()
}

func retryBackoff(base time.Duration, retryCount int) time.Duration {
	if base <= 0 {
		base = 30 * time.Second
	}
	if retryCount <= 0 {
		return base
	}
	result := base
	for i := 0; i < retryCount; i++ {
		result *= 2
		if result > 24*time.Hour {
			return 24 * time.Hour
		}
	}
	return result
}

func ProcessBackupRetryQueue(now time.Time) {
	cfg := GetBackupConfig()
	if !cfg.Enabled {
		return
	}
	rows, err := model.GetDueBackupUploadRetries(now.Unix(), 10)
	if err != nil {
		logBackupError("query retry queue failed: " + err.Error())
		return
	}
	if len(rows) == 0 {
		return
	}
	client, err := newWebDAVClient(cfg)
	if err != nil {
		logBackupError("retry init webdav client failed: " + err.Error())
		return
	}
	if err := client.ensureCollection(cfg.WebDAVBasePath); err != nil {
		logBackupError("retry ensure collection failed: " + err.Error())
		return
	}
	for _, row := range rows {
		remotePath := row.RemotePath
		if strings.TrimSpace(remotePath) == "" {
			remotePath = path.Join(cfg.WebDAVBasePath, filepath.Base(row.ArtifactPath))
		}
		uploadErr := client.uploadFile(row.ArtifactPath, remotePath)
		if uploadErr == nil {
			row.Status = "done"
			row.LastError = ""
			_ = row.Update()
			if run, runErr := model.GetBackupRunById(row.RunId); runErr == nil && run != nil {
				run.Status = "success"
				run.RemotePath = remotePath
				run.ErrorMessage = ""
				_ = run.Update()
			}
			_ = os.Remove(row.ArtifactPath)
			_ = applyWebDAVRetention(client, cfg.WebDAVBasePath, cfg.RetentionDays, cfg.RetentionMaxFiles)
			continue
		}
		row.RetryCount++
		row.LastError = redactBackupMessage(uploadErr.Error())
		if row.RetryCount > row.MaxRetries {
			row.Status = "failed"
			_ = row.Update()
			continue
		}
		row.NextRetryAt = now.Unix() + int64(retryBackoff(cfg.RetryBase, row.RetryCount).Seconds())
		_ = row.Update()
	}
}

func GetBackupStatusView() (*BackupStatusView, error) {
	latest, err := model.GetLatestBackupRun()
	if err != nil {
		return nil, err
	}
	pending, _ := model.CountPendingBackupRetries()
	cfg := GetBackupConfig()
	view := &BackupStatusView{
		Config:            cfg.RedactedForAPI(),
		PendingRetryCount: pending,
		LastRun:           latest,
	}
	backupState.mu.Lock()
	view.DirtyPending = backupState.dirty
	view.LastDirtyReason = backupState.dirtyReason
	view.LastScheduleEvalAt = backupState.lastScheduleEvalAt
	backupState.mu.Unlock()
	return view, nil
}

func maxInt64(a int64, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
