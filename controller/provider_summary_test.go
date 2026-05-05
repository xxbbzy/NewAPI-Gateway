package controller

import (
	"NewAPI-Gateway/model"
	"bytes"
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

func prepareProviderSummaryControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:controller_provider_summary_%s_%d?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"), time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db failed: %v", err)
	}
	if err := db.AutoMigrate(&model.Provider{}, &model.CheckinRunItem{}); err != nil {
		t.Fatalf("migrate test db failed: %v", err)
	}
	return db
}

func TestGetProviderSummaryReturnsAggregatedData(t *testing.T) {
	originDB := model.DB
	model.DB = prepareProviderSummaryControllerTestDB(t)
	defer func() { model.DB = originDB }()

	provider := &model.Provider{
		Name:           "summary-provider",
		BaseURL:        "https://summary.example.com",
		AccessToken:    "token",
		Balance:        "$7.25",
		BalanceUpdated: time.Now().Unix(),
		HealthStatus:   model.ProviderHealthStatusHealthy,
		Status:         1,
	}
	if err := provider.Insert(); err != nil {
		t.Fatalf("insert provider failed: %v", err)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/provider/summary", nil)

	GetProviderSummary(ctx)

	var resp struct {
		Success bool                  `json:"success"`
		Data    model.ProviderSummary `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success response")
	}
	if resp.Data.TotalProviders != 1 || resp.Data.BalanceTotalUSD != 7.25 {
		t.Fatalf("unexpected summary response: %+v", resp.Data)
	}
}

func TestGetProviderDetailRedactsProxyFields(t *testing.T) {
	originDB := model.DB
	model.DB = prepareProviderSummaryControllerTestDB(t)
	defer func() { model.DB = originDB }()

	provider := &model.Provider{
		Name:         "proxy-provider",
		BaseURL:      "https://proxy.example.com",
		AccessToken:  "secret-token",
		ProxyEnabled: true,
		ProxyURL:     "http://user:pass@proxy.internal:9000",
		HealthStatus: model.ProviderHealthStatusHealthy,
		BalanceUpdated: time.Now().Unix(),
		Status:       1,
	}
	if err := provider.Insert(); err != nil {
		t.Fatalf("insert provider failed: %v", err)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", provider.Id)}}
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/provider/"+fmt.Sprintf("%d", provider.Id), nil)

	GetProviderDetail(ctx)

	var resp struct {
		Success bool           `json:"success"`
		Data    model.Provider `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success response")
	}
	if resp.Data.ProxyURL != "" {
		t.Fatalf("expected raw proxy URL to be redacted")
	}
	if resp.Data.ProxyURLRedacted != "http://proxy.internal:9000" {
		t.Fatalf("unexpected redacted proxy URL: %s", resp.Data.ProxyURLRedacted)
	}
	if resp.Data.AccessToken != "" {
		t.Fatalf("expected access token to be redacted")
	}
}

func TestExportProvidersIncludesRawProxyURLForReImport(t *testing.T) {
	originDB := model.DB
	model.DB = prepareProviderSummaryControllerTestDB(t)
	defer func() { model.DB = originDB }()

	provider := &model.Provider{
		Name:         "export-provider",
		BaseURL:      "https://export.example.com",
		AccessToken:  "secret-token",
		ProxyEnabled: true,
		ProxyURL:     "http://user:pass@proxy.internal:9000",
		HealthStatus: model.ProviderHealthStatusHealthy,
		BalanceUpdated: time.Now().Unix(),
		Status:       1,
	}
	if err := provider.Insert(); err != nil {
		t.Fatalf("insert provider failed: %v", err)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/provider/export", nil)

	ExportProviders(ctx)

	var resp struct {
		Success bool `json:"success"`
		Data    []struct {
			ProxyEnabled     bool   `json:"proxy_enabled"`
			ProxyURL         string `json:"proxy_url"`
			ProxyURLRedacted string `json:"proxy_url_redacted"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if !resp.Success || len(resp.Data) != 1 {
		t.Fatalf("unexpected export response: %s", recorder.Body.String())
	}
	if !resp.Data[0].ProxyEnabled {
		t.Fatalf("expected proxy_enabled=true in export")
	}
	if resp.Data[0].ProxyURL != provider.ProxyURL {
		t.Fatalf("expected raw proxy URL for re-import, got %q", resp.Data[0].ProxyURL)
	}
	if resp.Data[0].ProxyURLRedacted != "http://proxy.internal:9000" {
		t.Fatalf("unexpected redacted proxy URL: %s", resp.Data[0].ProxyURLRedacted)
	}
}

func TestImportProvidersAcceptsExportedProxyURL(t *testing.T) {
	originDB := model.DB
	model.DB = prepareProviderSummaryControllerTestDB(t)
	defer func() { model.DB = originDB }()

	body := []byte(`[{
		"name":"import-provider",
		"base_url":"https://import.example.com",
		"access_token":"secret-token",
		"proxy_enabled":true,
		"proxy_url":"http://user:pass@proxy.internal:9000"
	}]`)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/provider/import", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")

	ImportProviders(ctx)

	var resp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected import success, got message: %s", resp.Message)
	}

	providers, err := model.GetAllProviders(0, 10)
	if err != nil {
		t.Fatalf("query providers failed: %v", err)
	}
	if len(providers) != 1 {
		t.Fatalf("expected one imported provider, got %d", len(providers))
	}
	if !providers[0].ProxyEnabled || providers[0].ProxyURL != "http://user:pass@proxy.internal:9000" {
		t.Fatalf("unexpected imported proxy config: %+v", providers[0])
	}
}
