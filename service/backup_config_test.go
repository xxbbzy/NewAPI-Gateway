package service

import "testing"

func TestBackupConfigRedactedForAPI(t *testing.T) {
	origin := BackupConfig{
		WebDAVURL:         "https://dav.example.com",
		WebDAVUsername:    "admin",
		WebDAVPassword:    "dav-password",
		EncryptEnabled:    true,
		EncryptPassphrase: "encrypt-passphrase",
	}

	redacted := origin.RedactedForAPI()
	if redacted.WebDAVPassword != "***" {
		t.Fatalf("expected webdav password to be redacted")
	}
	if redacted.EncryptPassphrase != "***" {
		t.Fatalf("expected encrypt passphrase to be redacted")
	}
	if redacted.WebDAVURL != origin.WebDAVURL || redacted.WebDAVUsername != origin.WebDAVUsername {
		t.Fatalf("non-sensitive fields should remain unchanged")
	}
	if origin.WebDAVPassword != "dav-password" || origin.EncryptPassphrase != "encrypt-passphrase" {
		t.Fatalf("redaction should not mutate original config")
	}
}
