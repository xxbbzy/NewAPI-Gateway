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

func prepareProviderUpdateCheckinTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:controller_provider_update_checkin_%s_%d?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"), time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db failed: %v", err)
	}
	if err := db.AutoMigrate(&model.Provider{}, &model.ProviderToken{}, &model.ModelRoute{}, &model.ModelPricing{}); err != nil {
		t.Fatalf("migrate test db failed: %v", err)
	}
	return db
}

func makeUpdateProviderContext(body []byte) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/provider/", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	return ctx, recorder
}

func TestUpdateProviderPersistsCheckinDisabledFalse(t *testing.T) {
	originDB := model.DB
	model.DB = prepareProviderUpdateCheckinTestDB(t)
	defer func() { model.DB = originDB }()

	gin.SetMode(gin.TestMode)

	provider := &model.Provider{
		Name:           "provider-disable-test",
		BaseURL:        "https://example.com",
		AccessToken:    "token",
		Status:         1,
		CheckinEnabled: true,
	}
	if err := provider.Insert(); err != nil {
		t.Fatalf("insert provider failed: %v", err)
	}

	body := []byte(fmt.Sprintf(`{"id":%d,"checkin_enabled":false}`, provider.Id))
	ctx, recorder := makeUpdateProviderContext(body)
	UpdateProvider(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", recorder.Code)
	}

	var resp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success response, got message: %s", resp.Message)
	}

	reloaded, err := model.GetProviderById(provider.Id)
	if err != nil {
		t.Fatalf("reload provider failed: %v", err)
	}
	if reloaded.CheckinEnabled {
		t.Fatalf("expected checkin_enabled=false after update")
	}
}

func TestUpdateProviderWithoutCheckinPayloadKeepsCheckinState(t *testing.T) {
	originDB := model.DB
	model.DB = prepareProviderUpdateCheckinTestDB(t)
	defer func() { model.DB = originDB }()

	gin.SetMode(gin.TestMode)

	provider := &model.Provider{
		Name:           "provider-keep-checkin-test",
		BaseURL:        "https://example.com",
		AccessToken:    "token",
		Status:         1,
		CheckinEnabled: true,
		Remark:         "before",
	}
	if err := provider.Insert(); err != nil {
		t.Fatalf("insert provider failed: %v", err)
	}

	body := []byte(fmt.Sprintf(`{"id":%d,"remark":"after"}`, provider.Id))
	ctx, recorder := makeUpdateProviderContext(body)
	UpdateProvider(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", recorder.Code)
	}

	var resp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success response, got message: %s", resp.Message)
	}

	reloaded, err := model.GetProviderById(provider.Id)
	if err != nil {
		t.Fatalf("reload provider failed: %v", err)
	}
	if !reloaded.CheckinEnabled {
		t.Fatalf("expected checkin_enabled to stay true when payload omits checkin_enabled")
	}
	if reloaded.Remark != "after" {
		t.Fatalf("expected remark to update, got: %s", reloaded.Remark)
	}
}
