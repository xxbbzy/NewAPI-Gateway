package service

import (
	"NewAPI-Gateway/model"
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type SnapshotResult struct {
	PayloadPath string
	Format      string
}

type Snapshotter interface {
	CreateSnapshot(ctx context.Context, workDir string) (*SnapshotResult, error)
	Driver() string
	Preflight() error
}

type commandRunner func(ctx context.Context, name string, args ...string) *exec.Cmd

var runCommand commandRunner = exec.CommandContext

func getSnapshotter(driver string, dsn string, cfg BackupConfig) (Snapshotter, error) {
	switch driver {
	case "sqlite":
		return &sqliteSnapshotter{sqlitePath: dsn}, nil
	case "mysql":
		return &mysqlSnapshotter{dsn: dsn, cmdName: cfg.MySQLDumpCommand, timeout: cfg.CommandTimeout}, nil
	case "postgres":
		return &postgresSnapshotter{dsn: dsn, cmdName: cfg.PostgresDumpCommand, timeout: cfg.CommandTimeout}, nil
	default:
		return nil, fmt.Errorf("unsupported SQL driver: %s", driver)
	}
}

type sqliteSnapshotter struct {
	sqlitePath string
}

func (s *sqliteSnapshotter) Driver() string {
	return "sqlite"
}

func (s *sqliteSnapshotter) Preflight() error {
	if strings.TrimSpace(s.sqlitePath) == "" {
		return fmt.Errorf("sqlite path is empty")
	}
	return nil
}

func (s *sqliteSnapshotter) CreateSnapshot(ctx context.Context, workDir string) (*SnapshotResult, error) {
	if err := s.Preflight(); err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	target := filepath.Join(workDir, "sqlite-snapshot.db")
	_ = os.Remove(target)
	escaped := strings.ReplaceAll(target, "'", "''")
	// VACUUM INTO creates a transactionally consistent snapshot for SQLite.
	if err := model.DB.Exec(fmt.Sprintf("VACUUM INTO '%s'", escaped)).Error; err != nil {
		return nil, fmt.Errorf("sqlite snapshot failed: %w", err)
	}
	return &SnapshotResult{PayloadPath: target, Format: "sqlite-db"}, nil
}

type mysqlSnapshotter struct {
	dsn     string
	cmdName string
	timeout time.Duration
}

func (s *mysqlSnapshotter) Driver() string {
	return "mysql"
}

func (s *mysqlSnapshotter) Preflight() error {
	if strings.TrimSpace(s.cmdName) == "" {
		return fmt.Errorf("mysql dump command is empty")
	}
	if _, err := exec.LookPath(s.cmdName); err != nil {
		return fmt.Errorf("mysql dump command not found: %w", err)
	}
	if _, _, _, _, err := parseMySQLDSN(s.dsn); err != nil {
		return err
	}
	return nil
}

func (s *mysqlSnapshotter) CreateSnapshot(ctx context.Context, workDir string) (*SnapshotResult, error) {
	if err := s.Preflight(); err != nil {
		return nil, err
	}
	user, pass, host, dbName, err := parseMySQLDSN(s.dsn)
	if err != nil {
		return nil, err
	}
	payloadPath := filepath.Join(workDir, "mysql-dump.sql")
	file, err := os.Create(payloadPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	cmdCtx := ctx
	if s.timeout > 0 {
		var cancel context.CancelFunc
		cmdCtx, cancel = context.WithTimeout(ctx, s.timeout)
		defer cancel()
	}
	hostname, port, splitErr := net.SplitHostPort(host)
	if splitErr != nil {
		hostname = host
		port = "3306"
	}
	args := []string{
		"--single-transaction",
		"--skip-lock-tables",
		"--quick",
		"--no-tablespaces",
		"-h", hostname,
		"-P", port,
		"-u", user,
		"--password=" + pass,
		dbName,
	}
	cmd := runCommand(cmdCtx, s.cmdName, args...)
	cmd.Stdout = file
	cmd.Stderr = file
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("mysql dump failed: %w", err)
	}
	return &SnapshotResult{PayloadPath: payloadPath, Format: "mysql-sql"}, nil
}

type postgresSnapshotter struct {
	dsn     string
	cmdName string
	timeout time.Duration
}

func (s *postgresSnapshotter) Driver() string {
	return "postgres"
}

func (s *postgresSnapshotter) Preflight() error {
	if strings.TrimSpace(s.cmdName) == "" {
		return fmt.Errorf("postgres dump command is empty")
	}
	if _, err := exec.LookPath(s.cmdName); err != nil {
		return fmt.Errorf("postgres dump command not found: %w", err)
	}
	if strings.TrimSpace(s.dsn) == "" {
		return fmt.Errorf("postgres dsn is empty")
	}
	return nil
}

func (s *postgresSnapshotter) CreateSnapshot(ctx context.Context, workDir string) (*SnapshotResult, error) {
	if err := s.Preflight(); err != nil {
		return nil, err
	}
	payloadPath := filepath.Join(workDir, "postgres-dump.sql")
	cmdCtx := ctx
	if s.timeout > 0 {
		var cancel context.CancelFunc
		cmdCtx, cancel = context.WithTimeout(ctx, s.timeout)
		defer cancel()
	}
	cmd := runCommand(cmdCtx, s.cmdName, "--dbname", s.dsn, "-f", payloadPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("postgres dump failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return &SnapshotResult{PayloadPath: payloadPath, Format: "postgres-sql"}, nil
}

func parseMySQLDSN(dsn string) (user string, pass string, host string, dbName string, err error) {
	trimmed := strings.TrimSpace(dsn)
	if trimmed == "" {
		return "", "", "", "", fmt.Errorf("mysql dsn is empty")
	}
	if strings.HasPrefix(trimmed, "mysql://") {
		parsed, parseErr := url.Parse(trimmed)
		if parseErr != nil {
			return "", "", "", "", fmt.Errorf("invalid mysql dsn: %w", parseErr)
		}
		host = parsed.Host
		if !strings.Contains(host, ":") {
			host = net.JoinHostPort(host, "3306")
		}
		if parsed.User != nil {
			user = parsed.User.Username()
			pass, _ = parsed.User.Password()
		}
		dbName = strings.TrimPrefix(parsed.Path, "/")
		if dbName == "" {
			return "", "", "", "", fmt.Errorf("mysql dsn missing db name")
		}
		return user, pass, host, dbName, nil
	}

	atIdx := strings.LastIndex(trimmed, "@")
	slashIdx := strings.LastIndex(trimmed, "/")
	if atIdx <= 0 || slashIdx <= atIdx {
		return "", "", "", "", fmt.Errorf("unsupported mysql dsn format")
	}
	userInfo := trimmed[:atIdx]
	hostPart := trimmed[atIdx+1 : slashIdx]
	dbPart := trimmed[slashIdx+1:]
	qIdx := strings.Index(dbPart, "?")
	if qIdx >= 0 {
		dbPart = dbPart[:qIdx]
	}
	if dbPart == "" {
		return "", "", "", "", fmt.Errorf("mysql dsn missing db name")
	}
	colonIdx := strings.Index(userInfo, ":")
	if colonIdx < 0 {
		user = userInfo
	} else {
		user = userInfo[:colonIdx]
		pass = userInfo[colonIdx+1:]
	}
	if strings.HasPrefix(hostPart, "tcp(") && strings.HasSuffix(hostPart, ")") {
		hostPart = strings.TrimSuffix(strings.TrimPrefix(hostPart, "tcp("), ")")
	}
	if !strings.Contains(hostPart, ":") {
		hostPart = hostPart + ":3306"
	}
	if _, _, splitErr := net.SplitHostPort(hostPart); splitErr != nil {
		return "", "", "", "", fmt.Errorf("invalid mysql host: %s", hostPart)
	}
	if user == "" {
		return "", "", "", "", fmt.Errorf("mysql dsn missing user")
	}
	if _, pErr := strconv.Atoi(strings.Split(hostPart, ":")[1]); pErr != nil {
		return "", "", "", "", fmt.Errorf("invalid mysql port")
	}
	return user, pass, hostPart, dbPart, nil
}

type SnapshotPreflight struct {
	Driver  string `json:"driver"`
	Ready   bool   `json:"ready"`
	Message string `json:"message"`
	Command string `json:"command"`
}

func BackupSnapshotPreflight(cfg BackupConfig) (*SnapshotPreflight, error) {
	driver, dsn, err := getActiveSQLDriverAndDSN()
	if err != nil {
		return nil, err
	}
	snapshotter, err := getSnapshotter(driver, dsn, cfg)
	if err != nil {
		return nil, err
	}
	status := &SnapshotPreflight{Driver: driver, Ready: false}
	switch driver {
	case "mysql":
		status.Command = cfg.MySQLDumpCommand
	case "postgres":
		status.Command = cfg.PostgresDumpCommand
	default:
		status.Command = "internal"
	}
	if err := snapshotter.Preflight(); err != nil {
		status.Message = err.Error()
		return status, nil
	}
	status.Ready = true
	status.Message = "ok"
	return status, nil
}
