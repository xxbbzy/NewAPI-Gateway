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

func prepareControllerModelCatalogTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:controller_model_catalog_%s_%d?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"), time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db failed: %v", err)
	}
	if err := db.AutoMigrate(&model.Provider{}, &model.ProviderToken{}, &model.ModelRoute{}, &model.ModelPricing{}); err != nil {
		t.Fatalf("migrate test db failed: %v", err)
	}
	return db
}

func seedControllerModelCatalogData(t *testing.T, providerStatus int, tokenStatus int) {
	t.Helper()
	provider := &model.Provider{
		Name:              "controller-model-catalog-provider",
		BaseURL:           "https://catalog.example.com",
		AccessToken:       "catalog-token",
		Status:            providerStatus,
		ModelAliasMapping: `{"aaa":"bbbxxxcccddd"}`,
	}
	if err := provider.Insert(); err != nil {
		t.Fatalf("insert provider failed: %v", err)
	}
	token := &model.ProviderToken{
		ProviderId: provider.Id,
		Name:       "controller-model-catalog-token",
		GroupName:  "default",
		Status:     tokenStatus,
		Priority:   0,
		Weight:     10,
	}
	if err := token.Insert(); err != nil {
		t.Fatalf("insert provider token failed: %v", err)
	}

	routes := []model.ModelRoute{
		{ModelName: "bbbxxxcccddd", ProviderId: provider.Id, ProviderTokenId: token.Id, Enabled: true, Priority: 0, Weight: 10},
		{ModelName: "gpt-4o-20250101", ProviderId: provider.Id, ProviderTokenId: token.Id, Enabled: true, Priority: 0, Weight: 10},
	}
	if err := model.RebuildRoutesForProvider(provider.Id, routes); err != nil {
		t.Fatalf("rebuild routes failed: %v", err)
	}
}

func newTestContext(method string, path string, body []byte) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, path, bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	return ctx, recorder
}

func setAggTokenContext(ctx *gin.Context, token *model.AggregatedToken) {
	if token == nil {
		token = &model.AggregatedToken{}
	}
	ctx.Set("agg_token", token)
}

func newCatalogAggToken(modelLimits string) *model.AggregatedToken {
	if strings.TrimSpace(modelLimits) == "" {
		return &model.AggregatedToken{}
	}
	return &model.AggregatedToken{
		ModelLimitsEnabled: true,
		ModelLimits:        modelLimits,
	}
}

func mustGetModelIDs(t *testing.T, method string, path string, token *model.AggregatedToken) []string {
	t.Helper()
	ctx, recorder := newTestContext(method, path, nil)
	setAggTokenContext(ctx, token)
	ListModels(ctx)
	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	var resp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal list models response failed: %v", err)
	}
	ids := make([]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		ids = append(ids, item.ID)
	}
	return ids
}
