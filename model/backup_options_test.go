package model

import "testing"

func TestValidateBackupOption(t *testing.T) {
	if message, ok := ValidateBackupOption(BackupEnabledOptionKey, "true"); !ok || message != "" {
		t.Fatalf("expected valid backup enabled option, got ok=%v message=%s", ok, message)
	}
	if message, ok := ValidateBackupOption(BackupTriggerModeOptionKey, "invalid"); ok || message == "" {
		t.Fatalf("expected invalid trigger mode message")
	}
	if message, ok := ValidateBackupOption(BackupScheduleCronOptionKey, "* * *"); ok || message == "" {
		t.Fatalf("expected invalid cron validation error")
	}
	if message, ok := ValidateBackupOption(BackupMinIntervalSecondsOptionKey, "-1"); ok || message == "" {
		t.Fatalf("expected invalid interval validation error")
	}
	if message, ok := ValidateBackupOption(BackupWebDAVURLOptionKey, "dav://example.com"); ok || message == "" {
		t.Fatalf("expected invalid webdav url validation error")
	}
}

func TestIsSensitiveOptionKey(t *testing.T) {
	cases := []struct {
		key      string
		expected bool
	}{
		{key: "BackupWebDAVPassword", expected: true},
		{key: "BackupEncryptPassphrase", expected: true},
		{key: "GitHubClientSecret", expected: true},
		{key: "ApiToken", expected: true},
		{key: "PasswordLoginEnabled", expected: false},
		{key: "PasswordRegisterEnabled", expected: false},
		{key: "BackupWebDAVURL", expected: false},
		{key: "RoutingBaseWeightFactor", expected: false},
	}
	for _, tc := range cases {
		if got := IsSensitiveOptionKey(tc.key); got != tc.expected {
			t.Fatalf("IsSensitiveOptionKey(%q)=%v, expected %v", tc.key, got, tc.expected)
		}
	}
}
