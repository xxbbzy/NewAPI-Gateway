package service

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUpstreamCheckinResponseSemantics(t *testing.T) {
	tests := []struct {
		name        string
		response    string
		wantErr     bool
		wantQuota   int64
		wantMessage string
		errContains string
	}{
		{
			name:        "normal success response",
			response:    `{"success":true,"message":"签到成功","data":{"quota_awarded":12}}`,
			wantErr:     false,
			wantQuota:   12,
			wantMessage: "签到成功",
		},
		{
			name:        "already signed should be idempotent success",
			response:    `{"success":false,"message":" 今日 已签到 ","data":{"quota_awarded":0}}`,
			wantErr:     false,
			wantQuota:   0,
			wantMessage: "今日 已签到",
		},
		{
			name:        "non-whitelisted failure remains error",
			response:    `{"success":false,"message":"token invalid","data":{"quota_awarded":0}}`,
			wantErr:     true,
			errContains: "checkin failed: token invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Fatalf("unexpected method: %s", r.Method)
				}
				if r.URL.Path != "/api/user/checkin" {
					t.Fatalf("unexpected path: %s", r.URL.Path)
				}
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(tt.response))
			}))
			defer server.Close()

			client := NewUpstreamClient(server.URL, "token", 1)
			client.HTTPClient = server.Client()

			resp, err := client.Checkin()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.errContains != "" && err.Error() != tt.errContains {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			if resp.QuotaAwarded != tt.wantQuota {
				t.Fatalf("unexpected quota: got=%d want=%d", resp.QuotaAwarded, tt.wantQuota)
			}
			if resp.Message != tt.wantMessage {
				t.Fatalf("unexpected message: got=%q want=%q", resp.Message, tt.wantMessage)
			}
		})
	}
}
