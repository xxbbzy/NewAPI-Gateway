package service

import (
	"NewAPI-Gateway/model"
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func newProxyLoggingTestContext() *gin.Context {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	ctx.Request = req
	return ctx
}

func TestGetContextModelNamePriority(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name          string
		requestModel  string
		originalModel string
		canonical     string
		resolved      string
		expected      string
	}{
		{
			name:          "resolved takes highest priority",
			requestModel:  "request-model",
			originalModel: "original-model",
			canonical:     "canonical-model",
			resolved:      "resolved-model",
			expected:      "resolved-model",
		},
		{
			name:          "request model is next fallback",
			requestModel:  "request-model",
			originalModel: "original-model",
			canonical:     "canonical-model",
			expected:      "request-model",
		},
		{
			name:          "canonical is next fallback",
			originalModel: "original-model",
			canonical:     "canonical-model",
			expected:      "canonical-model",
		},
		{
			name:          "original is final fallback",
			originalModel: "original-model",
			expected:      "original-model",
		},
		{
			name:     "empty when no model context is present",
			expected: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := newProxyLoggingTestContext()
			ctx.Set("request_model", tc.requestModel)
			ctx.Set("request_model_original", tc.originalModel)
			ctx.Set("request_model_canonical", tc.canonical)
			ctx.Set("request_model_resolved", tc.resolved)

			if got := getContextModelName(ctx); got != tc.expected {
				t.Fatalf("unexpected resolved context model: got=%q expected=%q", got, tc.expected)
			}
		})
	}
}

func TestLogProxyErrorTraceIncludesModelIdentities(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ctx := newProxyLoggingTestContext()
	ctx.Set("request_model_original", "alias-model")
	ctx.Set("request_model_canonical", "canonical-model")
	ctx.Set("request_model_resolved", "target-model")

	provider := &model.Provider{Id: 11, Name: "provider-a"}
	token := &model.ProviderToken{Id: 22}

	var buf bytes.Buffer
	originErrorWriter := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = &buf
	defer func() {
		gin.DefaultErrorWriter = originErrorWriter
	}()

	logProxyErrorTrace(ctx, "req-123", provider, token, "upstream failed\nline2")

	output := buf.String()
	if !strings.Contains(output, "request_id=req-123") {
		t.Fatalf("expected request id in error log, got: %s", output)
	}
	if !strings.Contains(output, "model=target-model") {
		t.Fatalf("expected resolved model to be used in error log, got: %s", output)
	}
	if !strings.Contains(output, "model_original=alias-model") {
		t.Fatalf("expected original model in error log, got: %s", output)
	}
	if !strings.Contains(output, "model_canonical=canonical-model") {
		t.Fatalf("expected canonical model in error log, got: %s", output)
	}
	if !strings.Contains(output, "model_resolved=target-model") {
		t.Fatalf("expected resolved model field in error log, got: %s", output)
	}
	if !strings.Contains(output, "detail=upstream failed line2") {
		t.Fatalf("expected newline-normalized detail in error log, got: %s", output)
	}
}

func TestRequestBodyForErrorLogRedactsByDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DEBUG_PROXY_PAYLOAD", "0")

	ctx := newProxyLoggingTestContext()
	ctx.Request.Header.Set("Content-Type", "application/json")
	body := []byte(`{"model":"gpt","messages":[{"role":"user","content":"secret prompt"}],"api_key":"sk-1234567890abcdef"}`)

	result := requestBodyForErrorLog(ctx, body)
	if !strings.Contains(result, "(json redacted)") {
		t.Fatalf("expected redacted marker, got %q", result)
	}
	if strings.Contains(result, "secret prompt") || strings.Contains(result, "sk-1234567890abcdef") {
		t.Fatalf("expected sensitive content to be redacted, got %q", result)
	}
}

func TestRequestBodyForErrorLogDebugModeMasksCredentialLikeFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DEBUG_PROXY_PAYLOAD", "1")

	ctx := newProxyLoggingTestContext()
	ctx.Request.Header.Set("Content-Type", "application/json")
	body := []byte(`{"authorization":"Bearer sk-abcdefghijklmnop","api_key":"sk-1234567890abcdef","messages":[{"content":"visible"}]}`)

	result := requestBodyForErrorLog(ctx, body)
	if !strings.Contains(result, "[debug_payload]") {
		t.Fatalf("expected debug payload marker, got %q", result)
	}
	if strings.Contains(result, "sk-abcdefghijklmnop") || strings.Contains(result, "sk-1234567890abcdef") {
		t.Fatalf("expected credentials to be masked in debug payload, got %q", result)
	}
	if !strings.Contains(result, "visible") {
		t.Fatalf("expected non-sensitive content to remain visible in debug mode, got %q", result)
	}
}
