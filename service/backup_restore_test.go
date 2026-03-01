package service

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExecuteBackupRestoreSuccessPathWithHealthCheck(t *testing.T) {
	origDriver := os.Getenv("SQL_DRIVER")
	origDSN := os.Getenv("SQL_DSN")
	defer func() {
		_ = os.Setenv("SQL_DRIVER", origDriver)
		_ = os.Setenv("SQL_DSN", origDSN)
	}()
	_ = os.Setenv("SQL_DRIVER", "sqlite")
	_ = os.Setenv("SQL_DSN", filepath.Join(t.TempDir(), "target.db"))

	workDir := t.TempDir()
	payload := filepath.Join(workDir, "payload.db")
	if err := os.WriteFile(payload, []byte("sqlite-snapshot-payload"), 0o600); err != nil {
		t.Fatalf("write payload failed: %v", err)
	}
	cfg := GetBackupConfig()
	cfg.EncryptEnabled = false
	artifact, err := buildBackupArtifact(workDir, &SnapshotResult{PayloadPath: payload, Format: "sqlite-db"}, "sqlite", "manual", cfg)
	if err != nil {
		t.Fatalf("build artifact failed: %v", err)
	}

	originExecutor := backupRestoreExecutor
	originHealthChecker := backupRestoreHealthCheck
	defer func() {
		backupRestoreExecutor = originExecutor
		backupRestoreHealthCheck = originHealthChecker
	}()

	backupRestoreExecutor = func(driver string, payloadPath string, _ BackupConfig) error {
		if driver != "sqlite" {
			t.Fatalf("expected sqlite driver, got %s", driver)
		}
		if _, statErr := os.Stat(payloadPath); statErr != nil {
			t.Fatalf("expected payload file to exist: %v", statErr)
		}
		return nil
	}
	backupRestoreHealthCheck = func() (map[string]int64, error) {
		return map[string]int64{"users": 3, "providers": 2, "options": 5}, nil
	}

	result, err := ExecuteBackupRestore(BackupRestoreRequest{
		LocalPath: artifact.LocalPath,
		DryRun:    false,
		Confirm:   true,
	})
	if err != nil {
		t.Fatalf("execute restore failed: %v", err)
	}
	if !result.Ready {
		t.Fatalf("expected restore result ready=true")
	}
	if result.Message != "restore completed" {
		t.Fatalf("unexpected restore message: %s", result.Message)
	}
	if result.Health == nil || result.Health["users"] != 3 || result.Health["providers"] != 2 || result.Health["options"] != 5 {
		t.Fatalf("expected health check values to be populated, got %+v", result.Health)
	}
}
