package model

import (
	"context"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	gormlogger "gorm.io/gorm/logger"
	"gorm.io/gorm"
)

type sqlCaptureLogger struct {
	lastSQL string
}

func (l *sqlCaptureLogger) LogMode(gormlogger.LogLevel) gormlogger.Interface {
	return l
}

func (l *sqlCaptureLogger) Info(context.Context, string, ...interface{}) {}

func (l *sqlCaptureLogger) Warn(context.Context, string, ...interface{}) {}

func (l *sqlCaptureLogger) Error(context.Context, string, ...interface{}) {}

func (l *sqlCaptureLogger) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	l.lastSQL, _ = fc()
}

func prepareAggregatedTokenPostgresQueryTestDB(t *testing.T, capture *sqlCaptureLogger) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN: "host=localhost user=postgres password=postgres dbname=newapi_gateway_test port=5432 sslmode=disable",
	}), &gorm.Config{
		DryRun:               true,
		DisableAutomaticPing: true,
		Logger:               capture,
	})
	if err != nil {
		t.Fatalf("open postgres dry-run db failed: %v", err)
	}
	return db
}

func TestGetAggTokenByKeyBuildsPostgresSafeSQL(t *testing.T) {
	originDB := DB
	capture := &sqlCaptureLogger{}
	DB = prepareAggregatedTokenPostgresQueryTestDB(t, capture)
	defer func() { DB = originDB }()

	if _, err := GetAggTokenByKey("agg-postgres-key"); err != nil {
		t.Fatalf("unexpected dry-run error: %v", err)
	}

	if strings.Contains(capture.lastSQL, "`key`") {
		t.Fatalf("expected postgres query to avoid MySQL backticks, got SQL: %s", capture.lastSQL)
	}
	if !strings.Contains(capture.lastSQL, `"key"`) {
		t.Fatalf("expected postgres query to quote key with double quotes, got SQL: %s", capture.lastSQL)
	}
}
