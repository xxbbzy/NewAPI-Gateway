package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"NewAPI-Gateway/common"
	"NewAPI-Gateway/model"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type paginationEnvelope struct {
	Success bool `json:"success"`
	Data    struct {
		Items      []json.RawMessage `json:"items"`
		P          int               `json:"p"`
		PageSize   int               `json:"page_size"`
		Total      int64             `json:"total"`
		TotalPages int               `json:"total_pages"`
		HasMore    bool              `json:"has_more"`
	} `json:"data"`
}

func prepareControllerPaginationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:controller_pagination_%s_%d?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"), time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db failed: %v", err)
	}
	if err := db.AutoMigrate(&model.Provider{}, &model.User{}, &model.File{}, &model.AggregatedToken{}, &model.UsageLog{}); err != nil {
		t.Fatalf("migrate test db failed: %v", err)
	}
	return db
}

func newListContext(path string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, path, nil)
	return ctx, recorder
}

func decodePaginationEnvelope(t *testing.T, recorder *httptest.ResponseRecorder) paginationEnvelope {
	t.Helper()
	var resp paginationEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response failed: %v, body=%s", err, recorder.Body.String())
	}
	return resp
}

func TestGetProvidersReturnsPaginationEnvelope(t *testing.T) {
	originDB := model.DB
	model.DB = prepareControllerPaginationTestDB(t)
	defer func() { model.DB = originDB }()

	ctxEmpty, recorderEmpty := newListContext("/api/provider/?p=0&page_size=10")
	GetProviders(ctxEmpty)
	emptyResp := decodePaginationEnvelope(t, recorderEmpty)
	if !emptyResp.Success || emptyResp.Data.Total != 0 || len(emptyResp.Data.Items) != 0 {
		t.Fatalf("expected empty paginated response, got %+v", emptyResp.Data)
	}

	for i := 0; i < 11; i++ {
		if err := model.DB.Create(&model.Provider{Name: fmt.Sprintf("provider-%d", i), BaseURL: "https://example.com", AccessToken: "token"}).Error; err != nil {
			t.Fatalf("seed provider failed: %v", err)
		}
	}

	ctxOut, recorderOut := newListContext("/api/provider/?p=2&page_size=10")
	GetProviders(ctxOut)
	outResp := decodePaginationEnvelope(t, recorderOut)
	if outResp.Data.Total != 11 || outResp.Data.TotalPages != 2 || outResp.Data.HasMore {
		t.Fatalf("unexpected pagination metadata: %+v", outResp.Data)
	}
	if len(outResp.Data.Items) != 0 {
		t.Fatalf("expected out-of-range items length 0, got %d", len(outResp.Data.Items))
	}

	ctxClamped, recorderClamped := newListContext("/api/provider/?p=0&page_size=9999")
	GetProviders(ctxClamped)
	clampedResp := decodePaginationEnvelope(t, recorderClamped)
	if clampedResp.Data.PageSize != common.MaxItemsPerPage {
		t.Fatalf("expected clamped page_size=%d, got %d", common.MaxItemsPerPage, clampedResp.Data.PageSize)
	}
}

func TestGetAllUsersReturnsPaginationEnvelope(t *testing.T) {
	originDB := model.DB
	model.DB = prepareControllerPaginationTestDB(t)
	defer func() { model.DB = originDB }()

	if err := model.DB.Create(&model.User{Username: "u1", Password: "password-123", DisplayName: "u1", Role: 1, Status: 1, Email: "u1@test.com"}).Error; err != nil {
		t.Fatalf("seed user failed: %v", err)
	}

	ctx, recorder := newListContext("/api/user/?p=0&page_size=10")
	GetAllUsers(ctx)
	resp := decodePaginationEnvelope(t, recorder)
	if !resp.Success || resp.Data.Total != 1 || len(resp.Data.Items) != 1 {
		t.Fatalf("unexpected user pagination response: %+v", resp.Data)
	}
}

func TestGetAllFilesReturnsPaginationEnvelope(t *testing.T) {
	originDB := model.DB
	model.DB = prepareControllerPaginationTestDB(t)
	defer func() { model.DB = originDB }()

	if err := model.DB.Create(&model.File{Filename: "a.txt", Uploader: "tester", UploaderId: 1, Link: "a-link", UploadTime: "2026-02-25 10:00:00"}).Error; err != nil {
		t.Fatalf("seed file failed: %v", err)
	}

	ctx, recorder := newListContext("/api/file/?p=0&page_size=10")
	GetAllFiles(ctx)
	resp := decodePaginationEnvelope(t, recorder)
	if !resp.Success || resp.Data.Total != 1 || len(resp.Data.Items) != 1 {
		t.Fatalf("unexpected file pagination response: %+v", resp.Data)
	}
}

func TestGetAggTokensReturnsPaginationEnvelope(t *testing.T) {
	originDB := model.DB
	model.DB = prepareControllerPaginationTestDB(t)
	defer func() { model.DB = originDB }()

	if err := model.DB.Create(&model.AggregatedToken{UserId: 7, Key: "agg-key-1", Name: "token-1", Status: 1, ExpiredTime: -1}).Error; err != nil {
		t.Fatalf("seed aggregated token failed: %v", err)
	}

	ctx, recorder := newListContext("/api/agg-token/?p=0&page_size=10")
	ctx.Set("id", 7)
	GetAggTokens(ctx)
	resp := decodePaginationEnvelope(t, recorder)
	if !resp.Success || resp.Data.Total != 1 || len(resp.Data.Items) != 1 {
		t.Fatalf("unexpected agg-token pagination response: %+v", resp.Data)
	}
}

func TestGetAllLogsReturnsPaginationEnvelope(t *testing.T) {
	originDB := model.DB
	model.DB = prepareControllerPaginationTestDB(t)
	defer func() { model.DB = originDB }()

	ctxEmpty, recorderEmpty := newListContext("/api/log/?p=0&page_size=10")
	GetAllLogs(ctxEmpty)
	emptyResp := decodePaginationEnvelope(t, recorderEmpty)
	if !emptyResp.Success || emptyResp.Data.Total != 0 || len(emptyResp.Data.Items) != 0 {
		t.Fatalf("expected empty paginated response for logs, got %+v", emptyResp.Data)
	}

	for i := 0; i < 11; i++ {
		if err := model.DB.Create(&model.UsageLog{
			UserId:       (i % 2) + 1,
			ProviderName: fmt.Sprintf("provider-%d", i%2),
			ModelName:    "gpt-4o-mini",
			Status:       1,
			CreatedAt:    time.Now().Unix() + int64(i),
		}).Error; err != nil {
			t.Fatalf("seed usage log failed: %v", err)
		}
	}

	ctxOut, recorderOut := newListContext("/api/log/?p=2&page_size=10")
	GetAllLogs(ctxOut)
	outResp := decodePaginationEnvelope(t, recorderOut)
	if outResp.Data.Total != 11 || outResp.Data.TotalPages != 2 || outResp.Data.HasMore {
		t.Fatalf("unexpected log pagination metadata: %+v", outResp.Data)
	}
	if len(outResp.Data.Items) != 0 {
		t.Fatalf("expected out-of-range log items length 0, got %d", len(outResp.Data.Items))
	}

	ctxClamped, recorderClamped := newListContext("/api/log/?p=0&page_size=9999")
	GetAllLogs(ctxClamped)
	clampedResp := decodePaginationEnvelope(t, recorderClamped)
	if clampedResp.Data.PageSize != common.MaxItemsPerPage {
		t.Fatalf("expected clamped log page_size=%d, got %d", common.MaxItemsPerPage, clampedResp.Data.PageSize)
	}
}

func TestGetSelfLogsReturnsPaginationEnvelope(t *testing.T) {
	originDB := model.DB
	model.DB = prepareControllerPaginationTestDB(t)
	defer func() { model.DB = originDB }()

	if err := model.DB.Create(&model.UsageLog{
		UserId:       7,
		ProviderName: "provider-self",
		ModelName:    "gpt-4o-mini",
		Status:       1,
		CreatedAt:    time.Now().Unix(),
	}).Error; err != nil {
		t.Fatalf("seed self usage log failed: %v", err)
	}
	if err := model.DB.Create(&model.UsageLog{
		UserId:       8,
		ProviderName: "provider-other",
		ModelName:    "gpt-4o-mini",
		Status:       1,
		CreatedAt:    time.Now().Unix() + 1,
	}).Error; err != nil {
		t.Fatalf("seed other usage log failed: %v", err)
	}

	ctxOut, recorderOut := newListContext("/api/log/self?p=1&page_size=10")
	ctxOut.Set("id", 7)
	GetSelfLogs(ctxOut)
	outResp := decodePaginationEnvelope(t, recorderOut)
	if outResp.Data.Total != 1 || outResp.Data.TotalPages != 1 || outResp.Data.HasMore {
		t.Fatalf("unexpected self-log pagination metadata: %+v", outResp.Data)
	}
	if len(outResp.Data.Items) != 0 {
		t.Fatalf("expected out-of-range self-log items length 0, got %d", len(outResp.Data.Items))
	}

	ctxFirst, recorderFirst := newListContext("/api/log/self?p=0&page_size=10")
	ctxFirst.Set("id", 7)
	GetSelfLogs(ctxFirst)
	firstResp := decodePaginationEnvelope(t, recorderFirst)
	if !firstResp.Success || firstResp.Data.Total != 1 || len(firstResp.Data.Items) != 1 {
		t.Fatalf("unexpected self-log first-page response: %+v", firstResp.Data)
	}
}
