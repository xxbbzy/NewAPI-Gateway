package controller

import (
	"NewAPI-Gateway/common"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func newPaginationContext(path string) *gin.Context {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, path, nil)
	return ctx
}

func TestParsePaginationParamsDefaultsAndBounds(t *testing.T) {
	ctxDefault := newPaginationContext("/api/provider/")
	paramsDefault := parsePaginationParams(ctxDefault)
	if paramsDefault.P != 0 {
		t.Fatalf("expected default page to be 0, got %d", paramsDefault.P)
	}
	if paramsDefault.PageSize <= 0 {
		t.Fatalf("expected default page_size > 0, got %d", paramsDefault.PageSize)
	}

	ctxInvalid := newPaginationContext("/api/provider/?p=-3&page_size=-1")
	paramsInvalid := parsePaginationParams(ctxInvalid)
	if paramsInvalid.P != 0 {
		t.Fatalf("expected invalid negative page to be normalized to 0, got %d", paramsInvalid.P)
	}
	if paramsInvalid.PageSize != paramsDefault.PageSize {
		t.Fatalf("expected invalid page_size to fallback to default %d, got %d", paramsDefault.PageSize, paramsInvalid.PageSize)
	}

	ctxOversized := newPaginationContext("/api/provider/?p=1&page_size=9999")
	paramsOversized := parsePaginationParams(ctxOversized)
	if paramsOversized.PageSize != common.MaxItemsPerPage {
		t.Fatalf("expected page_size to be clamped to %d, got %d", common.MaxItemsPerPage, paramsOversized.PageSize)
	}
	if paramsOversized.Offset != paramsOversized.P*paramsOversized.PageSize {
		t.Fatalf("offset mismatch: got %d", paramsOversized.Offset)
	}
}

func TestBuildPaginatedDataHasMoreAndTotalPages(t *testing.T) {
	data := buildPaginatedData([]int{1, 2}, PaginationParams{P: 0, PageSize: 10, Offset: 0}, 11)

	totalPages, ok := data["total_pages"].(int)
	if !ok {
		t.Fatalf("expected total_pages to be int")
	}
	if totalPages != 2 {
		t.Fatalf("expected total_pages=2, got %d", totalPages)
	}

	hasMore, ok := data["has_more"].(bool)
	if !ok {
		t.Fatalf("expected has_more to be bool")
	}
	if !hasMore {
		t.Fatalf("expected has_more=true for page 0/2")
	}
}
