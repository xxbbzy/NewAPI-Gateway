package controller

import (
	"NewAPI-Gateway/model"
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
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/token/":
			_, _ = w.Write([]byte(`{"success":true,"message":"","data":{}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/pricing":
			_, _ = w.Write([]byte(`{"success":true,"data":[],"group_ratio":{},"usable_group":{},"supported_endpoint":{}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/token/":
			_, _ = w.Write([]byte(`{"success":true,"data":{"items":[],"total":0}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/user/self":
			_, _ = w.Write([]byte(`{"success":true,"data":{"id":1,"quota":0,"status":1}}`))
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
	}
	if err := json.Unmarshal(recorderValid.Body.Bytes(), &validResp); err != nil {
		t.Fatalf("unmarshal valid-group response failed: %v", err)
	}
	if !validResp.Success {
		t.Fatalf("expected valid group to pass, got message: %s", validResp.Message)
	}
}
