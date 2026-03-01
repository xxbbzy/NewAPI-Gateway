package service

import (
	"NewAPI-Gateway/common"
	"NewAPI-Gateway/model"
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type BackupRestoreRequest struct {
	RunID      int    `json:"run_id"`
	LocalPath  string `json:"local_path"`
	RemotePath string `json:"remote_path"`
	DryRun     bool   `json:"dry_run"`
	Confirm    bool   `json:"confirm"`
}

type BackupRestoreResult struct {
	Ready    bool             `json:"ready"`
	DryRun   bool             `json:"dry_run"`
	Driver   string           `json:"driver"`
	Message  string           `json:"message"`
	Artifact string           `json:"artifact"`
	Manifest *BackupManifest  `json:"manifest,omitempty"`
	Health   map[string]int64 `json:"health,omitempty"`
	Warnings []string         `json:"warnings,omitempty"`
}

var (
	backupRestoreExecutor    = executeRestoreByDriver
	backupRestoreHealthCheck = runBackupRestoreHealthCheck
)

func ValidateBackupRestoreRequest(req BackupRestoreRequest) (*BackupRestoreResult, error) {
	cfg := GetBackupConfig()
	artifactPath, warnings, err := resolveRestoreArtifactPath(req, cfg)
	if err != nil {
		return nil, err
	}
	payloadPath, manifest, err := unpackArtifactAndManifest(artifactPath, cfg)
	if err != nil {
		return nil, err
	}
	defer os.Remove(payloadPath)
	if err := verifyPayloadChecksum(payloadPath, manifest.PayloadSHA256); err != nil {
		return nil, err
	}
	activeDriver, _, err := getActiveSQLDriverAndDSN()
	if err != nil {
		return nil, err
	}
	if manifest.Driver != activeDriver {
		return &BackupRestoreResult{
			Ready:    false,
			DryRun:   req.DryRun,
			Driver:   activeDriver,
			Message:  fmt.Sprintf("backup driver mismatch: artifact=%s current=%s", manifest.Driver, activeDriver),
			Artifact: artifactPath,
			Manifest: manifest,
			Warnings: warnings,
		}, nil
	}
	return &BackupRestoreResult{
		Ready:    true,
		DryRun:   req.DryRun,
		Driver:   activeDriver,
		Message:  "restore preflight passed",
		Artifact: artifactPath,
		Manifest: manifest,
		Warnings: warnings,
	}, nil
}

func ExecuteBackupRestore(req BackupRestoreRequest) (*BackupRestoreResult, error) {
	result, err := ValidateBackupRestoreRequest(req)
	if err != nil {
		return nil, err
	}
	if !result.Ready {
		return result, nil
	}
	if req.DryRun {
		result.Message = "dry-run passed"
		return result, nil
	}
	if !req.Confirm {
		result.Ready = false
		result.Message = "restore confirmation required"
		return result, nil
	}
	cfg := GetBackupConfig()
	payloadPath, manifest, err := unpackArtifactAndManifest(result.Artifact, cfg)
	if err != nil {
		return nil, err
	}
	defer os.Remove(payloadPath)
	if err := verifyPayloadChecksum(payloadPath, manifest.PayloadSHA256); err != nil {
		return nil, err
	}
	if err := backupRestoreExecutor(manifest.Driver, payloadPath, cfg); err != nil {
		return nil, err
	}
	health, err := backupRestoreHealthCheck()
	if err != nil {
		return nil, err
	}
	result.Message = "restore completed"
	result.Health = health
	return result, nil
}

func resolveRestoreArtifactPath(req BackupRestoreRequest, cfg BackupConfig) (string, []string, error) {
	warnings := make([]string, 0, 2)
	if trimmed := strings.TrimSpace(req.LocalPath); trimmed != "" {
		if _, err := os.Stat(trimmed); err != nil {
			return "", nil, err
		}
		return trimmed, warnings, nil
	}
	if req.RunID > 0 {
		run, err := model.GetBackupRunById(req.RunID)
		if err != nil {
			return "", nil, err
		}
		if run == nil {
			return "", nil, fmt.Errorf("backup run not found")
		}
		if strings.TrimSpace(run.ArtifactPath) != "" {
			if _, err := os.Stat(run.ArtifactPath); err == nil {
				return run.ArtifactPath, warnings, nil
			}
		}
		if strings.TrimSpace(run.RemotePath) != "" {
			if err := ensureBackupSpoolDir(cfg.SpoolDir); err != nil {
				return "", nil, err
			}
			local := filepath.Join(cfg.SpoolDir, fmt.Sprintf("restore-%d%s", time.Now().UnixNano(), filepath.Ext(run.RemotePath)))
			client, err := newWebDAVClient(cfg)
			if err != nil {
				return "", nil, err
			}
			if err := client.downloadFile(run.RemotePath, local); err != nil {
				return "", nil, err
			}
			warnings = append(warnings, "artifact downloaded from remote path")
			return local, warnings, nil
		}
	}
	if remote := strings.TrimSpace(req.RemotePath); remote != "" {
		if err := ensureBackupSpoolDir(cfg.SpoolDir); err != nil {
			return "", nil, err
		}
		local := filepath.Join(cfg.SpoolDir, fmt.Sprintf("restore-%d%s", time.Now().UnixNano(), filepath.Ext(remote)))
		client, err := newWebDAVClient(cfg)
		if err != nil {
			return "", nil, err
		}
		if err := client.downloadFile(remote, local); err != nil {
			return "", nil, err
		}
		warnings = append(warnings, "artifact downloaded from remote path")
		return local, warnings, nil
	}
	return "", nil, fmt.Errorf("missing restore artifact reference")
}

func executeRestoreByDriver(driver string, payloadPath string, cfg BackupConfig) error {
	switch driver {
	case "sqlite":
		// Replace SQLite file atomically; online handles may still require process restart.
		target := common.SQLitePath
		backup := target + ".pre-restore.bak"
		if err := copyFile(target, backup); err != nil {
			return fmt.Errorf("sqlite pre-restore backup failed: %w", err)
		}
		if err := copyFile(payloadPath, target); err != nil {
			return fmt.Errorf("sqlite restore failed: %w", err)
		}
		return nil
	case "mysql":
		_, dsn, err := getActiveSQLDriverAndDSN()
		if err != nil {
			return err
		}
		user, pass, host, dbName, err := parseMySQLDSN(dsn)
		if err != nil {
			return err
		}
		hostname, port, splitErr := net.SplitHostPort(host)
		if splitErr != nil {
			hostname = host
			port = "3306"
		}
		content, err := os.ReadFile(payloadPath)
		if err != nil {
			return err
		}
		ctx := context.Background()
		if cfg.CommandTimeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, cfg.CommandTimeout)
			defer cancel()
		}
		args := []string{"-h", hostname, "-P", port, "-u", user, "--password=" + pass, dbName}
		cmd := runCommand(ctx, cfg.MySQLRestoreCommand, args...)
		cmd.Stdin = bytes.NewReader(content)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("mysql restore failed: %w: %s", err, strings.TrimSpace(string(output)))
		}
		return nil
	case "postgres":
		_, dsn, err := getActiveSQLDriverAndDSN()
		if err != nil {
			return err
		}
		ctx := context.Background()
		if cfg.CommandTimeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, cfg.CommandTimeout)
			defer cancel()
		}
		cmd := runCommand(ctx, cfg.PostgresRestoreCommand, "--dbname", dsn, "-f", payloadPath)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("postgres restore failed: %w: %s", err, strings.TrimSpace(string(output)))
		}
		return nil
	default:
		return fmt.Errorf("unsupported restore driver: %s", driver)
	}
}

func copyFile(src string, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, input, 0o600)
}

func runBackupRestoreHealthCheck() (map[string]int64, error) {
	health := make(map[string]int64)
	var users int64
	if err := model.DB.Model(&model.User{}).Count(&users).Error; err != nil {
		return nil, err
	}
	var providers int64
	if err := model.DB.Model(&model.Provider{}).Count(&providers).Error; err != nil {
		return nil, err
	}
	var options int64
	if err := model.DB.Model(&model.Option{}).Count(&options).Error; err != nil {
		return nil, err
	}
	health["users"] = users
	health["providers"] = providers
	health["options"] = options
	return health, nil
}
