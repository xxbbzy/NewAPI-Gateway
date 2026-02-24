package model

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func prepareCheckinTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db failed: %v", err)
	}
	if err := db.AutoMigrate(&Provider{}, &CheckinRun{}, &CheckinRunItem{}); err != nil {
		t.Fatalf("migrate test db failed: %v", err)
	}
	return db
}

func TestGetUncheckinProviders(t *testing.T) {
	originDB := DB
	DB = prepareCheckinTestDB(t)
	defer func() { DB = originDB }()

	dayStart := time.Now().Add(-2 * time.Hour).Unix()
	providers := []*Provider{
		{Name: "A", BaseURL: "https://a.example.com", AccessToken: "a", Status: 1, CheckinEnabled: true, LastCheckinAt: dayStart - 100},
		{Name: "B", BaseURL: "https://b.example.com", AccessToken: "b", Status: 1, CheckinEnabled: true, LastCheckinAt: dayStart + 100},
		{Name: "C", BaseURL: "https://c.example.com", AccessToken: "c", Status: 1, CheckinEnabled: false, LastCheckinAt: dayStart - 100},
	}
	for _, provider := range providers {
		if err := provider.Insert(); err != nil {
			t.Fatalf("insert provider failed: %v", err)
		}
	}

	result, err := GetUncheckinProviders(dayStart)
	if err != nil {
		t.Fatalf("query uncheckin providers failed: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 uncheckin provider, got %d", len(result))
	}
	if result[0].Name != "A" {
		t.Fatalf("unexpected uncheckin provider: %s", result[0].Name)
	}
}

func TestCheckinRunPersistence(t *testing.T) {
	originDB := DB
	DB = prepareCheckinTestDB(t)
	defer func() { DB = originDB }()

	run := &CheckinRun{
		TriggerType:   "manual",
		Status:        "success",
		Timezone:      "Asia/Shanghai",
		ScheduledDate: "2026-02-24",
		StartedAt:     time.Now().Unix(),
	}
	if err := run.Insert(); err != nil {
		t.Fatalf("insert checkin run failed: %v", err)
	}

	items := []*CheckinRunItem{
		{RunId: run.Id, ProviderId: 1, ProviderName: "A", Status: "success", Message: "ok"},
		{RunId: run.Id, ProviderId: 2, ProviderName: "B", Status: "failed", Message: "network error"},
	}
	if err := InsertCheckinRunItems(items); err != nil {
		t.Fatalf("insert checkin run items failed: %v", err)
	}

	runs, err := GetRecentCheckinRuns(10)
	if err != nil {
		t.Fatalf("query recent runs failed: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}

	runItems, err := GetRecentCheckinRunItems(10)
	if err != nil {
		t.Fatalf("query recent run items failed: %v", err)
	}
	if len(runItems) != 2 {
		t.Fatalf("expected 2 run items, got %d", len(runItems))
	}
}

func TestExistsCheckinRunByTriggerAndScheduledDate(t *testing.T) {
	originDB := DB
	DB = prepareCheckinTestDB(t)
	defer func() { DB = originDB }()

	run := &CheckinRun{
		TriggerType:   "cron",
		Status:        "success",
		ScheduledDate: "2026-02-24",
	}
	if err := run.Insert(); err != nil {
		t.Fatalf("insert checkin run failed: %v", err)
	}

	exists, err := ExistsCheckinRunByTriggerAndScheduledDate("cron", "2026-02-24")
	if err != nil {
		t.Fatalf("query checkin run existence failed: %v", err)
	}
	if !exists {
		t.Fatalf("expected checkin run to exist")
	}

	exists, err = ExistsCheckinRunByTriggerAndScheduledDate("cron", "2026-02-25")
	if err != nil {
		t.Fatalf("query non-existing checkin run failed: %v", err)
	}
	if exists {
		t.Fatalf("expected no checkin run for non-existing scheduled date")
	}
}
