package controller

import (
	"NewAPI-Gateway/model"
	"NewAPI-Gateway/service"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func prepareProviderTokenValidationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:controller_provider_token_validation_%s_%d?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"), time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db failed: %v", err)
	}
	if err := db.AutoMigrate(&model.Provider{}, &model.ModelPricing{}, &model.ProviderToken{}, &model.ModelRoute{}); err != nil {
		t.Fatalf("migrate test db failed: %v", err)
	}
	return db
}

func newProviderWithPricing(t *testing.T, baseURL string) *model.Provider {
	t.Helper()
	provider := &model.Provider{
		Name:              "provider-test",
		BaseURL:           baseURL,
		AccessToken:       "test-token",
		UserID:            88,
		PricingGroupRatio: `{"pro":0.8,"default":1,"vip":1.5}`,
		PricingUsableGroup: `{
			"88":"vip",
			"default":"default"
		}`,
	}
	if err := provider.Insert(); err != nil {
		t.Fatalf("insert provider failed: %v", err)
	}
	pricing := []*model.ModelPricing{
		{
			ModelName:    "gpt-4o",
			ProviderId:   provider.Id,
			EnableGroups: `["default","vip"]`,
		},
		{
			ModelName:    "gpt-4o-mini",
			ProviderId:   provider.Id,
			EnableGroups: `["pro"]`,
		},
	}
	for _, item := range pricing {
		if err := model.UpsertModelPricing(item); err != nil {
			t.Fatalf("upsert model pricing failed: %v", err)
		}
	}
	return provider
}

func makeProviderRequestContext(method string, path string, body []byte, providerID int) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(providerID)}}
	ctx.Request = httptest.NewRequest(method, path, bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	return ctx, recorder
}

func TestGetProviderPricingReturnsTokenGroupOptionsAndDefaultGroup(t *testing.T) {
	originDB := model.DB
	model.DB = prepareProviderTokenValidationTestDB(t)
	defer func() { model.DB = originDB }()

	gin.SetMode(gin.TestMode)
	provider := newProviderWithPricing(t, "https://example.invalid")
	ctx, recorder := makeProviderRequestContext(http.MethodGet, "/api/provider/"+strconv.Itoa(provider.Id)+"/pricing", nil, provider.Id)

	GetProviderPricing(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", recorder.Code)
	}
	var resp struct {
		Success           bool   `json:"success"`
		DefaultGroup      string `json:"default_group"`
		TokenGroupOptions []struct {
			GroupName string  `json:"group_name"`
			Ratio     float64 `json:"ratio"`
		} `json:"token_group_options"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success response")
	}
	if resp.DefaultGroup != "vip" {
		t.Fatalf("expected default group vip, got %s", resp.DefaultGroup)
	}
	if len(resp.TokenGroupOptions) != 3 {
		t.Fatalf("expected 3 group options, got %d", len(resp.TokenGroupOptions))
	}
	if resp.TokenGroupOptions[0].GroupName != "pro" || resp.TokenGroupOptions[0].Ratio != 0.8 {
		t.Fatalf("expected first option pro x0.8, got %s x%v", resp.TokenGroupOptions[0].GroupName, resp.TokenGroupOptions[0].Ratio)
	}
}

func TestGetProviderPricingFallsBackToLowestRatioGroupWhenUsableGroupUnknown(t *testing.T) {
	originDB := model.DB
	model.DB = prepareProviderTokenValidationTestDB(t)
	defer func() { model.DB = originDB }()

	gin.SetMode(gin.TestMode)
	provider := newProviderWithPricing(t, "https://example.invalid")
	provider.PricingUsableGroup = `{"unknown":"not-exists"}`
	if err := model.DB.Model(provider).Update("pricing_usable_group", provider.PricingUsableGroup).Error; err != nil {
		t.Fatalf("update pricing_usable_group failed: %v", err)
	}

	ctx, recorder := makeProviderRequestContext(http.MethodGet, "/api/provider/"+strconv.Itoa(provider.Id)+"/pricing", nil, provider.Id)
	GetProviderPricing(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", recorder.Code)
	}
	var resp struct {
		Success      bool   `json:"success"`
		DefaultGroup string `json:"default_group"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success response")
	}
	if resp.DefaultGroup != "pro" {
		t.Fatalf("expected fallback default group pro, got %s", resp.DefaultGroup)
	}
}

func TestCreateProviderTokenRejectsEmptyAndOutOfScopeGroup(t *testing.T) {
	originDB := model.DB
	model.DB = prepareProviderTokenValidationTestDB(t)
	defer func() { model.DB = originDB }()

	gin.SetMode(gin.TestMode)
	provider := newProviderWithPricing(t, "https://example.invalid")

	emptyBody := []byte(`{"name":"demo","group_name":"","unlimited_quota":true}`)
	ctxEmpty, recorderEmpty := makeProviderRequestContext(http.MethodPost, "/api/provider/"+strconv.Itoa(provider.Id)+"/tokens", emptyBody, provider.Id)
	CreateProviderToken(ctxEmpty)

	var emptyResp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(recorderEmpty.Body.Bytes(), &emptyResp); err != nil {
		t.Fatalf("unmarshal empty-group response failed: %v", err)
	}
	if emptyResp.Success {
		t.Fatalf("expected empty group to be rejected")
	}
	if !strings.Contains(emptyResp.Message, "分组不能为空") {
		t.Fatalf("unexpected empty-group message: %s", emptyResp.Message)
	}

	invalidBody := []byte(`{"name":"demo","group_name":"not-allowed","unlimited_quota":true}`)
	ctxInvalid, recorderInvalid := makeProviderRequestContext(http.MethodPost, "/api/provider/"+strconv.Itoa(provider.Id)+"/tokens", invalidBody, provider.Id)
	CreateProviderToken(ctxInvalid)

	var invalidResp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(recorderInvalid.Body.Bytes(), &invalidResp); err != nil {
		t.Fatalf("unmarshal invalid-group response failed: %v", err)
	}
	if invalidResp.Success {
		t.Fatalf("expected out-of-scope group to be rejected")
	}
	if !strings.Contains(invalidResp.Message, "分组不属于该渠道可用分组") {
		t.Fatalf("unexpected out-of-scope message: %s", invalidResp.Message)
	}
}

func TestCreateProviderTokenAcceptsValidGroup(t *testing.T) {
	originDB := model.DB
	model.DB = prepareProviderTokenValidationTestDB(t)
	defer func() { model.DB = originDB }()

	gin.SetMode(gin.TestMode)
	created := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/token/":
			created = true
			_, _ = w.Write([]byte(`{"success":true,"message":"","data":{}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/token/":
			page := r.URL.Query().Get("p")
			if page == "" {
				page = "0"
			}
			if !created || page != "0" {
				_, _ = w.Write([]byte(`{"success":true,"message":"","data":{"page":1,"page_size":100,"total":0,"items":[]}}`))
				return
			}
			_, _ = w.Write([]byte(`{"success":true,"message":"","data":{"page":0,"page_size":100,"total":1,"items":[{"id":101,"key":"abcdefghijklmnop","name":"demo","status":1,"group":"default","remain_quota":0,"unlimited_quota":true,"used_quota":0,"model_limits_enabled":false,"model_limits":""}]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	provider := newProviderWithPricing(t, upstream.URL)

	validBody := []byte(`{"name":"demo","group_name":"default","unlimited_quota":true}`)
	ctxValid, recorderValid := makeProviderRequestContext(http.MethodPost, "/api/provider/"+strconv.Itoa(provider.Id)+"/tokens", validBody, provider.Id)
	CreateProviderToken(ctxValid)

	var validResp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Data    struct {
			UpstreamCreated        bool   `json:"upstream_created"`
			CreatedTokenIdentified bool   `json:"created_token_identified"`
			ProviderTokenId        int    `json:"provider_token_id"`
			UpstreamTokenId        int    `json:"upstream_token_id"`
			KeyStatus              string `json:"key_status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorderValid.Body.Bytes(), &validResp); err != nil {
		t.Fatalf("unmarshal valid-group response failed: %v", err)
	}
	if !validResp.Success {
		t.Fatalf("expected valid group to pass, got message: %s", validResp.Message)
	}
	if validResp.Data.UpstreamCreated != true || validResp.Data.CreatedTokenIdentified != true {
		t.Fatalf("expected create outcome to confirm upstream create and identification, got %+v", validResp.Data)
	}
	if validResp.Data.UpstreamTokenId != 101 || validResp.Data.ProviderTokenId == 0 {
		t.Fatalf("unexpected create outcome ids: %+v", validResp.Data)
	}
	if validResp.Data.KeyStatus != model.ProviderTokenKeyStatusReady {
		t.Fatalf("expected ready key status, got %+v", validResp.Data)
	}
	if !strings.Contains(validResp.Message, "密钥已同步") {
		t.Fatalf("unexpected success message: %s", validResp.Message)
	}

	stored, err := model.GetProviderTokenById(validResp.Data.ProviderTokenId)
	if err != nil {
		t.Fatalf("load created provider token failed: %v", err)
	}
	if stored.UpstreamTokenId != 101 || stored.SkKey != "sk-abcdefghijklmnop" {
		t.Fatalf("unexpected stored token state: %+v", stored)
	}
}

func TestCreateProviderTokenRecoversPlaintextViaKeyEndpoint(t *testing.T) {
	originDB := model.DB
	model.DB = prepareProviderTokenValidationTestDB(t)
	defer func() { model.DB = originDB }()

	gin.SetMode(gin.TestMode)
	created := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/token/":
			created = true
			_, _ = w.Write([]byte(`{"success":true,"message":"","data":{}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/token/":
			page := r.URL.Query().Get("p")
			if page == "" {
				page = "0"
			}
			if !created || page != "0" {
				_, _ = w.Write([]byte(`{"success":true,"message":"","data":{"page":1,"page_size":100,"total":0,"items":[]}}`))
				return
			}
			_, _ = w.Write([]byte(`{"success":true,"message":"","data":{"page":0,"page_size":100,"total":1,"items":[{"id":102,"key":"abc********xyz","name":"demo","status":1,"group":"default","remain_quota":0,"unlimited_quota":true,"used_quota":0,"model_limits_enabled":false,"model_limits":""}]}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/token/102/key":
			_, _ = w.Write([]byte(`{"success":true,"message":"","data":{"key":"restored-plaintext"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	provider := newProviderWithPricing(t, upstream.URL)

	validBody := []byte(`{"name":"demo","group_name":"default","unlimited_quota":true}`)
	ctxValid, recorderValid := makeProviderRequestContext(http.MethodPost, "/api/provider/"+strconv.Itoa(provider.Id)+"/tokens", validBody, provider.Id)
	CreateProviderToken(ctxValid)

	var validResp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Data    struct {
			UpstreamCreated        bool   `json:"upstream_created"`
			CreatedTokenIdentified bool   `json:"created_token_identified"`
			ProviderTokenId        int    `json:"provider_token_id"`
			UpstreamTokenId        int    `json:"upstream_token_id"`
			KeyStatus              string `json:"key_status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorderValid.Body.Bytes(), &validResp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if !validResp.Success {
		t.Fatalf("expected success, got message: %s", validResp.Message)
	}
	if validResp.Data.KeyStatus != model.ProviderTokenKeyStatusReady {
		t.Fatalf("expected ready key status, got %+v", validResp.Data)
	}
	if !strings.Contains(validResp.Message, "密钥已同步") {
		t.Fatalf("unexpected success message: %s", validResp.Message)
	}

	stored, err := model.GetProviderTokenById(validResp.Data.ProviderTokenId)
	if err != nil {
		t.Fatalf("load created provider token failed: %v", err)
	}
	if stored.UpstreamTokenId != 102 || stored.SkKey != "sk-restored-plaintext" {
		t.Fatalf("unexpected stored token state: %+v", stored)
	}
}

func TestBuildProviderTokenCreateMessageReasonSpecific(t *testing.T) {
	tests := []struct {
		name    string
		outcome *service.ProviderTokenCreateOutcome
		want    string
	}{
		{
			name: "key endpoint unavailable",
			outcome: &service.ProviderTokenCreateOutcome{
				KeyStatus:           model.ProviderTokenKeyStatusUnresolved,
				KeyUnresolvedReason: model.ProviderTokenKeyUnresolvedReasonKeyEndpointUnavailable,
			},
			want: "未开放明文恢复接口",
		},
		{
			name: "key endpoint unauthorized",
			outcome: &service.ProviderTokenCreateOutcome{
				KeyStatus:           model.ProviderTokenKeyStatusUnresolved,
				KeyUnresolvedReason: model.ProviderTokenKeyUnresolvedReasonKeyEndpointUnauthorized,
			},
			want: "鉴权失败",
		},
		{
			name: "key endpoint request failed",
			outcome: &service.ProviderTokenCreateOutcome{
				KeyStatus:           model.ProviderTokenKeyStatusUnresolved,
				KeyUnresolvedReason: model.ProviderTokenKeyUnresolvedReasonKeyEndpointRequestFailed,
			},
			want: "请求失败",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := buildProviderTokenCreateMessage(tt.outcome)
			if !strings.Contains(msg, tt.want) {
				t.Fatalf("unexpected message: %s", msg)
			}
		})
	}
}
