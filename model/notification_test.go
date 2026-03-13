package model

import (
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func prepareNotificationModelTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:notification_model_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db failed: %v", err)
	}
	if err := db.AutoMigrate(&NotificationAlertState{}); err != nil {
		t.Fatalf("migrate notification alert state failed: %v", err)
	}
	return db
}

func TestValidateNotificationOption(t *testing.T) {
	if message, ok := ValidateNotificationOption(NotificationBarkServerOptionKey, "https://api.day.app"); !ok || message != "" {
		t.Fatalf("expected bark server validation success, got ok=%v message=%q", ok, message)
	}
	if message, ok := ValidateNotificationOption(NotificationWebhookURLOptionKey, "ftp://example.com"); ok || message == "" {
		t.Fatalf("expected webhook URL validation failure, got ok=%v message=%q", ok, message)
	}
	if message, ok := ValidateNotificationOption(NotificationSMTPRecipientsOptionKey, "invalid-address"); ok || message == "" {
		t.Fatalf("expected smtp recipients validation failure, got ok=%v message=%q", ok, message)
	}
	if message, ok := ValidateNotificationOption(NotificationVerbosityModeOptionKey, "verbose"); ok || message == "" {
		t.Fatalf("expected verbosity validation failure, got ok=%v message=%q", ok, message)
	}
	if message, ok := ValidateNotificationOption(NotificationRequestFailureThresholdOptionKey, "0"); ok || message == "" {
		t.Fatalf("expected threshold validation failure, got ok=%v message=%q", ok, message)
	}
}

func TestNotificationSensitiveOptionKeys(t *testing.T) {
	cases := []struct {
		key      string
		expected bool
	}{
		{NotificationBarkDeviceKeyOptionKey, true},
		{NotificationWebhookURLOptionKey, true},
		{NotificationWebhookTokenOptionKey, true},
		{NotificationBarkServerOptionKey, false},
		{NotificationSMTPRecipientsOptionKey, false},
	}
	for _, tc := range cases {
		if got := IsSensitiveOptionKey(tc.key); got != tc.expected {
			t.Fatalf("IsSensitiveOptionKey(%q)=%v, expected %v", tc.key, got, tc.expected)
		}
	}
}

func TestNotificationAlertStatePersistence(t *testing.T) {
	originDB := DB
	DB = prepareNotificationModelTestDB(t)
	defer func() { DB = originDB }()

	state := &NotificationAlertState{
		DedupeKey:      "provider-health:1",
		EventFamily:    "provider_health",
		Status:         NotificationAlertStatusOpen,
		LastFiredAt:    111,
		LastObservedAt: 112,
		WindowCount:    3,
		LastSummary:    "dial tcp timeout",
	}
	if err := SaveNotificationAlertState(state); err != nil {
		t.Fatalf("save alert state failed: %v", err)
	}

	reloaded, err := GetNotificationAlertState("provider-health:1")
	if err != nil {
		t.Fatalf("load alert state failed: %v", err)
	}
	if reloaded == nil {
		t.Fatalf("expected alert state to exist")
	}
	if reloaded.Status != NotificationAlertStatusOpen {
		t.Fatalf("expected open status, got %s", reloaded.Status)
	}
	if reloaded.WindowCount != 3 {
		t.Fatalf("expected window count 3, got %d", reloaded.WindowCount)
	}

	reloaded.Status = NotificationAlertStatusClosed
	reloaded.LastResolvedAt = 113
	if err := SaveNotificationAlertState(reloaded); err != nil {
		t.Fatalf("update alert state failed: %v", err)
	}

	updated, err := GetNotificationAlertState("provider-health:1")
	if err != nil {
		t.Fatalf("reload alert state failed: %v", err)
	}
	if updated.Status != NotificationAlertStatusClosed {
		t.Fatalf("expected closed status, got %s", updated.Status)
	}
	if updated.LastResolvedAt != 113 {
		t.Fatalf("expected resolved timestamp 113, got %d", updated.LastResolvedAt)
	}
}
