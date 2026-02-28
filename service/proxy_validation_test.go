package service

import (
	"NewAPI-Gateway/model"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newProxyValidationContext(t *testing.T, path string) *gin.Context {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"model":"gpt-test"}`))
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req
	ctx.Set("request_model", "gpt-test")
	ctx.Set("request_model_original", "gpt-test")
	ctx.Set("request_model_canonical", "gpt-test")
	ctx.Set("request_model_resolved", "gpt-test")
	return ctx
}

type failingReadCloser struct{}

func (f *failingReadCloser) Read(_ []byte) (int, error) {
	return 0, errors.New("forced read failure")
}

func (f *failingReadCloser) Close() error {
	return nil
}

func prepareProxyValidationDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:proxy_validation_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db failed: %v", err)
	}
	if err := db.AutoMigrate(&model.UsageLog{}, &model.ModelPricing{}); err != nil {
		t.Fatalf("migrate db failed: %v", err)
	}
	return db
}

func TestValidateNonStreamResponseClassification(t *testing.T) {
	gin.SetMode(gin.TestMode)

	validBody := []byte(`{"id":"x","choices":[{"message":{"role":"assistant","content":"hello"}}]}`)
	if result := validateNonStreamResponse("/v1/chat/completions", validBody, true); !result.Valid {
		t.Fatalf("expected valid response, got %+v", result)
	}

	emptyBody := []byte(`{"id":"x","choices":[{"message":{"role":"assistant","content":""}}]}`)
	if result := validateNonStreamResponse("/v1/chat/completions", emptyBody, true); result.Valid || result.InvalidReason != "no_actionable_output" {
		t.Fatalf("expected no_actionable_output classification, got %+v", result)
	}

	errorEnvelope := []byte(`{"error":{"message":"rate limit"}}`)
	if result := validateNonStreamResponse("/v1/chat/completions", errorEnvelope, true); result.Valid || result.InvalidReason != "error_envelope_2xx" {
		t.Fatalf("expected error_envelope_2xx classification, got %+v", result)
	}

	invalidJSON := []byte(`{"choices":[`)
	if result := validateNonStreamResponse("/v1/chat/completions", invalidJSON, true); result.Valid || result.InvalidReason != "payload_parse_failed" {
		t.Fatalf("expected payload_parse_failed classification, got %+v", result)
	}
}

func TestClassifySSEDataLine(t *testing.T) {
	gin.SetMode(gin.TestMode)

	meaningful := classifySSEDataLine(
		"/v1/chat/completions",
		`data: {"choices":[{"delta":{"content":"hello"}}]}`,
		true,
	)
	if !meaningful.MeaningfulDelta {
		t.Fatalf("expected meaningful delta classification, got %+v", meaningful)
	}

	errorEnvelope := classifySSEDataLine(
		"/v1/chat/completions",
		`data: {"error":{"message":"bad"}}`,
		true,
	)
	if !errorEnvelope.ErrorEnvelope || errorEnvelope.InvalidReason != "stream_error_envelope" {
		t.Fatalf("expected stream_error_envelope classification, got %+v", errorEnvelope)
	}

	parseFailed := classifySSEDataLine(
		"/v1/chat/completions",
		`data: {"choices":`,
		true,
	)
	if !parseFailed.ParseFailed || parseFailed.InvalidReason != "stream_payload_parse_failed" {
		t.Fatalf("expected stream_payload_parse_failed classification, got %+v", parseFailed)
	}
}

func TestProxyNonStreamResponseReadFailureIsRetryable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originDB := model.DB
	model.DB = prepareProxyValidationDB(t)
	defer func() { model.DB = originDB }()

	ctx := newProxyValidationContext(t, "/v1/chat/completions")
	resp := &http.Response{
		StatusCode: 200,
		Header:     make(http.Header),
		Body:       &failingReadCloser{},
	}
	aggToken := &model.AggregatedToken{Id: 1, UserId: 1}
	provider := &model.Provider{Id: 1, Name: "p1"}
	token := &model.ProviderToken{Id: 1}

	attemptErr := proxyNonStreamResponse(ctx, resp, aggToken, provider, token, "req-1", []byte(`{"model":"gpt-test"}`), time.Now(), true, true)
	if attemptErr == nil {
		t.Fatalf("expected retryable error on read failure")
	}
	if !attemptErr.Retryable {
		t.Fatalf("expected retryable error, got %+v", attemptErr)
	}
	if attemptErr.FailureCategory != model.UsageFailureCategoryReadError {
		t.Fatalf("expected read_error category, got %+v", attemptErr)
	}
	if attemptErr.InvalidReason != "body_read_failed" {
		t.Fatalf("expected body_read_failed reason, got %+v", attemptErr)
	}
}
