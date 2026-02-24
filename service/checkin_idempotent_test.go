package service

import (
	"NewAPI-Gateway/model"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func prepareServiceCheckinTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:service_checkin_test_%s_%d?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"), time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db failed: %v", err)
	}
	if err := db.AutoMigrate(&model.Provider{}, &model.CheckinRun{}, &model.CheckinRunItem{}); err != nil {
		t.Fatalf("migrate test db failed: %v", err)
	}
	return db
}

func TestRunProviderCheckinTreatsAlreadySignedAsSuccess(t *testing.T) {
	originDB := model.DB
	model.DB = prepareServiceCheckinTestDB(t)
	defer func() { model.DB = originDB }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":false,"message":"今日已签到","data":{"quota_awarded":0}}`))
	}))
	defer server.Close()

	provider := &model.Provider{
		Name:           "already-signed-provider",
		BaseURL:        server.URL,
		AccessToken:    "token",
		UserID:         1,
		Status:         1,
		CheckinEnabled: true,
	}
	if err := provider.Insert(); err != nil {
		t.Fatalf("insert provider failed: %v", err)
	}

	run, item, err := RunProviderCheckin(provider)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if item.Status != "success" {
		t.Fatalf("expected success item status, got: %s", item.Status)
	}
	if item.Message != "今日已签到" {
		t.Fatalf("unexpected item message: %s", item.Message)
	}
	if run.SuccessCount != 1 || run.FailureCount != 0 {
		t.Fatalf("unexpected summary counters: success=%d failure=%d", run.SuccessCount, run.FailureCount)
	}
	if run.Status != "success" {
		t.Fatalf("expected run status success, got: %s", run.Status)
	}
	if run.Message != "今日已签到" {
		t.Fatalf("unexpected run message: %s", run.Message)
	}

	items, queryErr := model.GetRecentCheckinRunItems(10)
	if queryErr != nil {
		t.Fatalf("query checkin items failed: %v", queryErr)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 checkin item, got %d", len(items))
	}
	if items[0].Status != "success" || items[0].Message != "今日已签到" {
		t.Fatalf("unexpected persisted item: status=%s message=%s", items[0].Status, items[0].Message)
	}
}

func TestTriggerFullCheckinRunKeepsRealFailureAndCountsAlreadySignedAsSuccess(t *testing.T) {
	originDB := model.DB
	model.DB = prepareServiceCheckinTestDB(t)
	defer func() { model.DB = originDB }()

	alreadySignedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":false,"message":"今日已签到","data":{"quota_awarded":0}}`))
	}))
	defer alreadySignedServer.Close()

	failureServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":false,"message":"token invalid","data":{"quota_awarded":0}}`))
	}))
	defer failureServer.Close()

	providers := []*model.Provider{
		{
			Name:           "already-signed-provider",
			BaseURL:        alreadySignedServer.URL,
			AccessToken:    "token-a",
			UserID:         1,
			Status:         1,
			CheckinEnabled: true,
		},
		{
			Name:           "failed-provider",
			BaseURL:        failureServer.URL,
			AccessToken:    "token-b",
			UserID:         2,
			Status:         1,
			CheckinEnabled: true,
		},
	}

	for _, provider := range providers {
		if err := provider.Insert(); err != nil {
			t.Fatalf("insert provider failed: %v", err)
		}
	}

	run, err := TriggerFullCheckinRun("manual")
	if err != nil {
		t.Fatalf("trigger full checkin run failed: %v", err)
	}

	if run.SuccessCount != 1 {
		t.Fatalf("expected success count 1, got %d", run.SuccessCount)
	}
	if run.FailureCount != 1 {
		t.Fatalf("expected failure count 1, got %d", run.FailureCount)
	}
	if run.Status != "partial" {
		t.Fatalf("expected partial run status, got %s", run.Status)
	}

	items, queryErr := model.GetRecentCheckinRunItems(10)
	if queryErr != nil {
		t.Fatalf("query checkin items failed: %v", queryErr)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 checkin items, got %d", len(items))
	}

	statusByProvider := map[string]string{}
	messageByProvider := map[string]string{}
	for _, item := range items {
		statusByProvider[item.ProviderName] = item.Status
		messageByProvider[item.ProviderName] = item.Message
	}

	if statusByProvider["already-signed-provider"] != "success" {
		t.Fatalf("already-signed provider should be success, got %s", statusByProvider["already-signed-provider"])
	}
	if messageByProvider["already-signed-provider"] != "今日已签到" {
		t.Fatalf("unexpected already-signed message: %s", messageByProvider["already-signed-provider"])
	}

	if statusByProvider["failed-provider"] != "failed" {
		t.Fatalf("failed provider should remain failed, got %s", statusByProvider["failed-provider"])
	}
	if !strings.Contains(messageByProvider["failed-provider"], "checkin failed: token invalid") {
		t.Fatalf("unexpected failure message: %s", messageByProvider["failed-provider"])
	}
}
