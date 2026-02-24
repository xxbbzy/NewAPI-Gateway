package controller

import (
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

func prepareProviderControllerCheckinTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:controller_checkin_test_%s_%d?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"), time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db failed: %v", err)
	}
	if err := db.AutoMigrate(&model.CheckinRun{}, &model.CheckinRunItem{}); err != nil {
		t.Fatalf("migrate test db failed: %v", err)
	}
	return db
}

func seedCheckinMessagesForControllerTest(t *testing.T) {
	t.Helper()
	items := []*model.CheckinRunItem{
		{RunId: 1, ProviderId: 1, ProviderName: "oldest", Status: "failed", Message: "m1"},
		{RunId: 1, ProviderId: 2, ProviderName: "middle", Status: "success", Message: "m2"},
		{RunId: 1, ProviderId: 3, ProviderName: "newest", Status: "success", Message: "m3"},
	}
	for _, item := range items {
		if err := model.InsertCheckinRunItems([]*model.CheckinRunItem{item}); err != nil {
			t.Fatalf("insert checkin message failed: %v", err)
		}
	}
}

func TestGetCheckinRunMessagesReturnsReverseChronologicalWithLimit(t *testing.T) {
	originDB := model.DB
	model.DB = prepareProviderControllerCheckinTestDB(t)
	defer func() { model.DB = originDB }()

	seedCheckinMessagesForControllerTest(t)
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/provider/checkin/messages?limit=2", nil)

	GetCheckinRunMessages(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", recorder.Code)
	}

	var resp struct {
		Success bool                   `json:"success"`
		Data    []model.CheckinRunItem `json:"data"`
		Message string                 `json:"message"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success response, got message: %s", resp.Message)
	}
	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(resp.Data))
	}
	if resp.Data[0].ProviderName != "newest" || resp.Data[1].ProviderName != "middle" {
		t.Fatalf("unexpected message order: first=%s second=%s", resp.Data[0].ProviderName, resp.Data[1].ProviderName)
	}
}

func TestGetCheckinRunMessagesUsesDefaultLimitForNonPositiveInput(t *testing.T) {
	originDB := model.DB
	model.DB = prepareProviderControllerCheckinTestDB(t)
	defer func() { model.DB = originDB }()

	seedCheckinMessagesForControllerTest(t)
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/provider/checkin/messages?limit=0", nil)

	GetCheckinRunMessages(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", recorder.Code)
	}

	var resp struct {
		Success bool                   `json:"success"`
		Data    []model.CheckinRunItem `json:"data"`
		Message string                 `json:"message"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success response, got message: %s", resp.Message)
	}
	if len(resp.Data) != 3 {
		t.Fatalf("expected default limit to return all 3 messages, got %d", len(resp.Data))
	}
	if resp.Data[0].ProviderName != "newest" || resp.Data[1].ProviderName != "middle" || resp.Data[2].ProviderName != "oldest" {
		t.Fatalf("unexpected default-limit order: %s, %s, %s", resp.Data[0].ProviderName, resp.Data[1].ProviderName, resp.Data[2].ProviderName)
	}
}

func TestGetCheckinRunMessagesUsesDefaultLimitWhenMissing(t *testing.T) {
	originDB := model.DB
	model.DB = prepareProviderControllerCheckinTestDB(t)
	defer func() { model.DB = originDB }()

	seedCheckinMessagesForControllerTest(t)
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/provider/checkin/messages", nil)

	GetCheckinRunMessages(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", recorder.Code)
	}

	var resp struct {
		Success bool                   `json:"success"`
		Data    []model.CheckinRunItem `json:"data"`
		Message string                 `json:"message"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success response, got message: %s", resp.Message)
	}
	if len(resp.Data) != 3 {
		t.Fatalf("expected default limit to return all 3 messages, got %d", len(resp.Data))
	}
	if resp.Data[0].ProviderName != "newest" || resp.Data[1].ProviderName != "middle" || resp.Data[2].ProviderName != "oldest" {
		t.Fatalf("unexpected default-limit order: %s, %s, %s", resp.Data[0].ProviderName, resp.Data[1].ProviderName, resp.Data[2].ProviderName)
	}
}

func TestGetCheckinRunMessagesUsesDefaultLimitWhenInvalid(t *testing.T) {
	originDB := model.DB
	model.DB = prepareProviderControllerCheckinTestDB(t)
	defer func() { model.DB = originDB }()

	seedCheckinMessagesForControllerTest(t)
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/provider/checkin/messages?limit=abc", nil)

	GetCheckinRunMessages(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", recorder.Code)
	}

	var resp struct {
		Success bool                   `json:"success"`
		Data    []model.CheckinRunItem `json:"data"`
		Message string                 `json:"message"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success response, got message: %s", resp.Message)
	}
	if len(resp.Data) != 3 {
		t.Fatalf("expected default limit to return all 3 messages, got %d", len(resp.Data))
	}
	if resp.Data[0].ProviderName != "newest" || resp.Data[1].ProviderName != "middle" || resp.Data[2].ProviderName != "oldest" {
		t.Fatalf("unexpected default-limit order: %s, %s, %s", resp.Data[0].ProviderName, resp.Data[1].ProviderName, resp.Data[2].ProviderName)
	}
}
