package model

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func preparePaginationQueryTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:model_pagination_query_%s_%d?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"), time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db failed: %v", err)
	}
	if err := db.AutoMigrate(&Provider{}, &User{}, &File{}, &AggregatedToken{}); err != nil {
		t.Fatalf("migrate test db failed: %v", err)
	}
	return db
}

func TestQueryProvidersSupportsEmptyAndOutOfRange(t *testing.T) {
	originDB := DB
	DB = preparePaginationQueryTestDB(t)
	defer func() { DB = originDB }()

	items, total, err := QueryProviders(0, 10)
	if err != nil {
		t.Fatalf("unexpected error for empty query: %v", err)
	}
	if len(items) != 0 || total != 0 {
		t.Fatalf("expected empty result and total=0, got len=%d total=%d", len(items), total)
	}

	for i := 0; i < 2; i++ {
		if err := DB.Create(&Provider{Name: fmt.Sprintf("p-%d", i), BaseURL: "https://example.com", AccessToken: "token"}).Error; err != nil {
			t.Fatalf("seed provider failed: %v", err)
		}
	}

	items, total, err = QueryProviders(100, 10)
	if err != nil {
		t.Fatalf("unexpected error for out-of-range query: %v", err)
	}
	if len(items) != 0 || total != 2 {
		t.Fatalf("expected out-of-range len=0 total=2, got len=%d total=%d", len(items), total)
	}
}

func TestQueryUsersSupportsEmptyAndOutOfRange(t *testing.T) {
	originDB := DB
	DB = preparePaginationQueryTestDB(t)
	defer func() { DB = originDB }()

	items, total, err := QueryUsers(0, 10)
	if err != nil {
		t.Fatalf("unexpected error for empty query: %v", err)
	}
	if len(items) != 0 || total != 0 {
		t.Fatalf("expected empty result and total=0, got len=%d total=%d", len(items), total)
	}

	for i := 0; i < 2; i++ {
		if err := DB.Create(&User{
			Username:    fmt.Sprintf("u%d", i),
			Password:    "password-123",
			DisplayName: fmt.Sprintf("User %d", i),
			Role:        1,
			Status:      1,
			Email:       fmt.Sprintf("u%d@test.com", i),
		}).Error; err != nil {
			t.Fatalf("seed user failed: %v", err)
		}
	}

	items, total, err = QueryUsers(100, 10)
	if err != nil {
		t.Fatalf("unexpected error for out-of-range query: %v", err)
	}
	if len(items) != 0 || total != 2 {
		t.Fatalf("expected out-of-range len=0 total=2, got len=%d total=%d", len(items), total)
	}
}

func TestQueryFilesSupportsEmptyAndOutOfRange(t *testing.T) {
	originDB := DB
	DB = preparePaginationQueryTestDB(t)
	defer func() { DB = originDB }()

	items, total, err := QueryFiles(0, 10)
	if err != nil {
		t.Fatalf("unexpected error for empty query: %v", err)
	}
	if len(items) != 0 || total != 0 {
		t.Fatalf("expected empty result and total=0, got len=%d total=%d", len(items), total)
	}

	for i := 0; i < 2; i++ {
		if err := DB.Create(&File{
			Filename:   fmt.Sprintf("f%d.txt", i),
			Uploader:   "tester",
			UploaderId: 1,
			Link:       fmt.Sprintf("f-%d-link", i),
			UploadTime: "2026-02-25 10:00:00",
		}).Error; err != nil {
			t.Fatalf("seed file failed: %v", err)
		}
	}

	items, total, err = QueryFiles(100, 10)
	if err != nil {
		t.Fatalf("unexpected error for out-of-range query: %v", err)
	}
	if len(items) != 0 || total != 2 {
		t.Fatalf("expected out-of-range len=0 total=2, got len=%d total=%d", len(items), total)
	}
}

func TestQueryUserAggTokensSupportsEmptyAndOutOfRange(t *testing.T) {
	originDB := DB
	DB = preparePaginationQueryTestDB(t)
	defer func() { DB = originDB }()

	items, total, err := QueryUserAggTokens(1, 0, 10)
	if err != nil {
		t.Fatalf("unexpected error for empty query: %v", err)
	}
	if len(items) != 0 || total != 0 {
		t.Fatalf("expected empty result and total=0, got len=%d total=%d", len(items), total)
	}

	for i := 0; i < 2; i++ {
		if err := DB.Create(&AggregatedToken{
			UserId:      1,
			Key:         fmt.Sprintf("agg-token-%d", i),
			Name:        fmt.Sprintf("token-%d", i),
			Status:      1,
			ExpiredTime: -1,
		}).Error; err != nil {
			t.Fatalf("seed token failed: %v", err)
		}
	}

	items, total, err = QueryUserAggTokens(1, 100, 10)
	if err != nil {
		t.Fatalf("unexpected error for out-of-range query: %v", err)
	}
	if len(items) != 0 || total != 2 {
		t.Fatalf("expected out-of-range len=0 total=2, got len=%d total=%d", len(items), total)
	}
}
