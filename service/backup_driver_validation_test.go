package service

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetSnapshotterByDriver(t *testing.T) {
	cfg := BackupConfig{MySQLDumpCommand: "mysqldump", PostgresDumpCommand: "pg_dump", CommandTimeout: 10}
	for _, driver := range []string{"sqlite", "mysql", "postgres"} {
		s, err := getSnapshotter(driver, "file:test.db", cfg)
		if err != nil {
			t.Fatalf("get snapshotter %s failed: %v", driver, err)
		}
		if s.Driver() != driver {
			t.Fatalf("expected snapshotter driver %s, got %s", driver, s.Driver())
		}
	}
}

func TestBackupSnapshotPreflightSQLite(t *testing.T) {
	origDriver := os.Getenv("SQL_DRIVER")
	origDSN := os.Getenv("SQL_DSN")
	defer func() {
		_ = os.Setenv("SQL_DRIVER", origDriver)
		_ = os.Setenv("SQL_DSN", origDSN)
	}()
	_ = os.Setenv("SQL_DRIVER", "sqlite")
	_ = os.Setenv("SQL_DSN", filepath.Join(t.TempDir(), "test.db"))

	status, err := BackupSnapshotPreflight(GetBackupConfig())
	if err != nil {
		t.Fatalf("preflight failed: %v", err)
	}
	if status.Driver != "sqlite" {
		t.Fatalf("expected sqlite driver, got %s", status.Driver)
	}
	if !status.Ready {
		t.Fatalf("expected sqlite preflight ready, got message=%s", status.Message)
	}
}

func TestValidateBackupRestoreDriverMismatch(t *testing.T) {
	origDriver := os.Getenv("SQL_DRIVER")
	origDSN := os.Getenv("SQL_DSN")
	defer func() {
		_ = os.Setenv("SQL_DRIVER", origDriver)
		_ = os.Setenv("SQL_DSN", origDSN)
	}()
	_ = os.Setenv("SQL_DRIVER", "sqlite")
	_ = os.Setenv("SQL_DSN", filepath.Join(t.TempDir(), "target.db"))

	workDir := t.TempDir()
	payload := filepath.Join(workDir, "payload.sql")
	if err := os.WriteFile(payload, []byte("-- dump"), 0o600); err != nil {
		t.Fatalf("write payload failed: %v", err)
	}
	cfg := GetBackupConfig()
	cfg.EncryptEnabled = false
	artifact, err := buildBackupArtifact(workDir, &SnapshotResult{PayloadPath: payload, Format: "mysql-sql"}, "mysql", "manual", cfg)
	if err != nil {
		t.Fatalf("build artifact failed: %v", err)
	}

	result, err := ValidateBackupRestoreRequest(BackupRestoreRequest{LocalPath: artifact.LocalPath, DryRun: true})
	if err != nil {
		t.Fatalf("validate restore request failed: %v", err)
	}
	if result.Ready {
		t.Fatalf("expected driver mismatch to be not ready")
	}
	if result.Manifest == nil || result.Manifest.Driver != "mysql" {
		t.Fatalf("expected mysql manifest in validation result")
	}
}
