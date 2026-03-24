package service

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetTokenPlaintextKeySuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/token/1/key" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"","data":{"key":"PLAINTEXT"}}`))
	}))
	defer server.Close()

	client := NewUpstreamClient(server.URL, "token", 1)
	client.HTTPClient = server.Client()
	key, err := client.GetTokenPlaintextKey(1)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if key != "PLAINTEXT" {
		t.Fatalf("unexpected key: %q", key)
	}
}

func TestGetTokenPlaintextKeyNoKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":true,"message":"no plaintext","data":{"key":""}}`))
	}))
	defer server.Close()

	client := NewUpstreamClient(server.URL, "token", 1)
	client.HTTPClient = server.Client()
	_, err := client.GetTokenPlaintextKey(1)
	var keyErr *UpstreamTokenPlaintextKeyError
	if !errors.As(err, &keyErr) {
		t.Fatalf("expected UpstreamTokenPlaintextKeyError, got %T", err)
	}
	if keyErr.Kind != UpstreamTokenPlaintextKeyErrorNoKey {
		t.Fatalf("unexpected key error kind: %s", keyErr.Kind)
	}
}

func TestGetTokenPlaintextKeyHTTPClassifiedErrors(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		wantKind UpstreamTokenPlaintextKeyErrorKind
	}{
		{name: "unauthorized", status: http.StatusUnauthorized, wantKind: UpstreamTokenPlaintextKeyErrorUnauthorized},
		{name: "forbidden", status: http.StatusForbidden, wantKind: UpstreamTokenPlaintextKeyErrorUnauthorized},
		{name: "not found", status: http.StatusNotFound, wantKind: UpstreamTokenPlaintextKeyErrorUnavailable},
		{name: "method not allowed", status: http.StatusMethodNotAllowed, wantKind: UpstreamTokenPlaintextKeyErrorUnavailable},
		{name: "server error", status: http.StatusInternalServerError, wantKind: UpstreamTokenPlaintextKeyErrorRequestFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(`{"success":false,"message":"x"}`))
			}))
			defer server.Close()

			client := NewUpstreamClient(server.URL, "token", 1)
			client.HTTPClient = server.Client()
			_, err := client.GetTokenPlaintextKey(1)
			var keyErr *UpstreamTokenPlaintextKeyError
			if !errors.As(err, &keyErr) {
				t.Fatalf("expected UpstreamTokenPlaintextKeyError, got %T", err)
			}
			if keyErr.Kind != tt.wantKind {
				t.Fatalf("unexpected key error kind: got=%s want=%s", keyErr.Kind, tt.wantKind)
			}
		})
	}
}

func TestGetTokenPlaintextKeyNetworkErrorClassified(t *testing.T) {
	client := NewUpstreamClient("http://127.0.0.1:1", "token", 1)
	_, err := client.GetTokenPlaintextKey(1)
	var keyErr *UpstreamTokenPlaintextKeyError
	if !errors.As(err, &keyErr) {
		t.Fatalf("expected UpstreamTokenPlaintextKeyError, got %T", err)
	}
	if keyErr.Kind != UpstreamTokenPlaintextKeyErrorRequestFailed {
		t.Fatalf("unexpected key error kind: %s", keyErr.Kind)
	}
	if strings.TrimSpace(keyErr.Error()) == "" {
		t.Fatalf("expected non-empty error message")
	}
}
