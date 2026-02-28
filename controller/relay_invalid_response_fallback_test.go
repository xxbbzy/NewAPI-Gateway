package controller

import (
	"NewAPI-Gateway/common"
	"NewAPI-Gateway/model"
	"encoding/json"
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

func setRelayValidationModeForTest(t *testing.T, mode string) {
	t.Helper()
	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	originMode, hasOriginMode := common.OptionMap[model.RelayResponseValidityGuardModeOptionKey]
	originLegacy, hasOriginLegacy := common.OptionMap[model.RelayResponseValidityGuardEnabledOptionKey]
	common.OptionMap[model.RelayResponseValidityGuardModeOptionKey] = mode
	common.OptionMap[model.RelayResponseValidityGuardEnabledOptionKey] = "true"
	common.OptionMapRWMutex.Unlock()

	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		if hasOriginMode {
			common.OptionMap[model.RelayResponseValidityGuardModeOptionKey] = originMode
		} else {
			delete(common.OptionMap, model.RelayResponseValidityGuardModeOptionKey)
		}
		if hasOriginLegacy {
			common.OptionMap[model.RelayResponseValidityGuardEnabledOptionKey] = originLegacy
		} else {
			delete(common.OptionMap, model.RelayResponseValidityGuardEnabledOptionKey)
		}
		common.OptionMapRWMutex.Unlock()
	})
}

func waitForUsageLogs(t *testing.T, expected int64) []model.UsageLog {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		var count int64
		if err := model.DB.Model(&model.UsageLog{}).Count(&count).Error; err != nil {
			t.Fatalf("count usage logs failed: %v", err)
		}
		if count >= expected || time.Now().After(deadline) {
			break
		}
		time.Sleep(30 * time.Millisecond)
	}

	var logs []model.UsageLog
	if err := model.DB.Order("attempt_index asc").Find(&logs).Error; err != nil {
		t.Fatalf("query usage logs failed: %v", err)
	}
	return logs
}

func prepareRelayFallbackTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:relay_fallback_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db failed: %v", err)
	}
	if err := db.AutoMigrate(&model.Provider{}, &model.ProviderToken{}, &model.ModelRoute{}, &model.ModelPricing{}, &model.UsageLog{}); err != nil {
		t.Fatalf("migrate db failed: %v", err)
	}
	return db
}

func TestRelayFallbackOnInvalid2xxResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originDB := model.DB
	model.DB = prepareRelayFallbackTestDB(t)
	defer func() { model.DB = originDB }()
	setRelayValidationModeForTest(t, model.RelayResponseValidityModeEnforce)

	invalidServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"bad","choices":[{"message":{"role":"assistant","content":""}}]}`))
	}))
	defer invalidServer.Close()

	validServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"ok","choices":[{"message":{"role":"assistant","content":"hello from fallback"}}]}`))
	}))
	defer validServer.Close()

	providerInvalid := &model.Provider{Name: "invalid-provider", BaseURL: invalidServer.URL, Status: common.UserStatusEnabled}
	if err := providerInvalid.Insert(); err != nil {
		t.Fatalf("insert invalid provider failed: %v", err)
	}
	providerValid := &model.Provider{Name: "valid-provider", BaseURL: validServer.URL, Status: common.UserStatusEnabled}
	if err := providerValid.Insert(); err != nil {
		t.Fatalf("insert valid provider failed: %v", err)
	}

	tokenInvalid := &model.ProviderToken{ProviderId: providerInvalid.Id, Name: "invalid-token", GroupName: "default", Status: common.UserStatusEnabled, Priority: 1, Weight: 10, SkKey: "sk-invalid"}
	if err := tokenInvalid.Insert(); err != nil {
		t.Fatalf("insert invalid token failed: %v", err)
	}
	tokenValid := &model.ProviderToken{ProviderId: providerValid.Id, Name: "valid-token", GroupName: "default", Status: common.UserStatusEnabled, Priority: 0, Weight: 10, SkKey: "sk-valid"}
	if err := tokenValid.Insert(); err != nil {
		t.Fatalf("insert valid token failed: %v", err)
	}

	if err := model.RebuildRoutesForProvider(providerInvalid.Id, []model.ModelRoute{{ModelName: "gpt-fallback", ProviderId: providerInvalid.Id, ProviderTokenId: tokenInvalid.Id, Enabled: true, Priority: 1, Weight: 10}}); err != nil {
		t.Fatalf("rebuild routes for invalid provider failed: %v", err)
	}
	if err := model.RebuildRoutesForProvider(providerValid.Id, []model.ModelRoute{{ModelName: "gpt-fallback", ProviderId: providerValid.Id, ProviderTokenId: tokenValid.Id, Enabled: true, Priority: 0, Weight: 10}}); err != nil {
		t.Fatalf("rebuild routes for valid provider failed: %v", err)
	}

	ctx, recorder := newTestContext(http.MethodPost, "/v1/chat/completions", []byte(`{"model":"gpt-fallback","messages":[{"role":"user","content":"hi"}]}`))
	setAggTokenContext(ctx, &model.AggregatedToken{})
	Relay(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from fallback route, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "hello from fallback") {
		t.Fatalf("expected fallback response body, got %s", recorder.Body.String())
	}

	logs := waitForUsageLogs(t, 2)
	if len(logs) < 2 {
		t.Fatalf("expected at least 2 usage logs, got %d", len(logs))
	}
	if logs[0].FailureCategory != model.UsageFailureCategoryInvalidResponse || logs[0].Status != 0 {
		payload, _ := json.Marshal(logs[0])
		t.Fatalf("expected first attempt classified as invalid_response failure, got %s", string(payload))
	}
	if logs[1].Status != 1 {
		payload, _ := json.Marshal(logs[1])
		t.Fatalf("expected second attempt success, got %s", string(payload))
	}
	if strings.TrimSpace(logs[0].RelayRequestId) == "" || logs[0].RelayRequestId != logs[1].RelayRequestId {
		t.Fatalf("expected attempts to share relay request id, got first=%q second=%q", logs[0].RelayRequestId, logs[1].RelayRequestId)
	}
}

func TestRelayFallbackOnSSEWithoutMeaningfulDelta(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originDB := model.DB
	model.DB = prepareRelayFallbackTestDB(t)
	defer func() { model.DB = originDB }()
	setRelayValidationModeForTest(t, model.RelayResponseValidityModeEnforce)

	invalidStreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"role\":\"assistant\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer invalidStreamServer.Close()

	validServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"ok","choices":[{"message":{"role":"assistant","content":"fallback after empty stream"}}]}`))
	}))
	defer validServer.Close()

	providerInvalid := &model.Provider{Name: "invalid-stream-provider", BaseURL: invalidStreamServer.URL, Status: common.UserStatusEnabled}
	if err := providerInvalid.Insert(); err != nil {
		t.Fatalf("insert invalid stream provider failed: %v", err)
	}
	providerValid := &model.Provider{Name: "valid-provider", BaseURL: validServer.URL, Status: common.UserStatusEnabled}
	if err := providerValid.Insert(); err != nil {
		t.Fatalf("insert valid provider failed: %v", err)
	}

	tokenInvalid := &model.ProviderToken{ProviderId: providerInvalid.Id, Name: "invalid-stream-token", GroupName: "default", Status: common.UserStatusEnabled, Priority: 1, Weight: 10, SkKey: "sk-invalid-stream"}
	if err := tokenInvalid.Insert(); err != nil {
		t.Fatalf("insert invalid stream token failed: %v", err)
	}
	tokenValid := &model.ProviderToken{ProviderId: providerValid.Id, Name: "valid-token", GroupName: "default", Status: common.UserStatusEnabled, Priority: 0, Weight: 10, SkKey: "sk-valid"}
	if err := tokenValid.Insert(); err != nil {
		t.Fatalf("insert valid token failed: %v", err)
	}

	if err := model.RebuildRoutesForProvider(providerInvalid.Id, []model.ModelRoute{{ModelName: "gpt-fallback-stream", ProviderId: providerInvalid.Id, ProviderTokenId: tokenInvalid.Id, Enabled: true, Priority: 1, Weight: 10}}); err != nil {
		t.Fatalf("rebuild routes for invalid stream provider failed: %v", err)
	}
	if err := model.RebuildRoutesForProvider(providerValid.Id, []model.ModelRoute{{ModelName: "gpt-fallback-stream", ProviderId: providerValid.Id, ProviderTokenId: tokenValid.Id, Enabled: true, Priority: 0, Weight: 10}}); err != nil {
		t.Fatalf("rebuild routes for valid provider failed: %v", err)
	}

	ctx, recorder := newTestContext(http.MethodPost, "/v1/chat/completions", []byte(`{"model":"gpt-fallback-stream","messages":[{"role":"user","content":"hi"}]}`))
	setAggTokenContext(ctx, &model.AggregatedToken{})
	Relay(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from fallback route, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "fallback after empty stream") {
		t.Fatalf("expected second route response body, got %s", recorder.Body.String())
	}

	logs := waitForUsageLogs(t, 2)
	if len(logs) < 2 {
		t.Fatalf("expected at least 2 usage logs, got %d", len(logs))
	}
	if logs[0].Status != 0 || logs[0].FailureCategory != model.UsageFailureCategoryInvalidResponse || logs[0].InvalidReason != "stream_no_meaningful_delta" {
		payload, _ := json.Marshal(logs[0])
		t.Fatalf("expected first SSE attempt invalid stream classification, got %s", string(payload))
	}
	if !logs[0].IsStream {
		payload, _ := json.Marshal(logs[0])
		t.Fatalf("expected first attempt to be marked as stream, got %s", string(payload))
	}
	if logs[1].Status != 1 {
		payload, _ := json.Marshal(logs[1])
		t.Fatalf("expected second attempt success, got %s", string(payload))
	}
	if strings.TrimSpace(logs[0].RelayRequestId) == "" || logs[0].RelayRequestId != logs[1].RelayRequestId {
		t.Fatalf("expected attempts to share relay request id, got first=%q second=%q", logs[0].RelayRequestId, logs[1].RelayRequestId)
	}
}
