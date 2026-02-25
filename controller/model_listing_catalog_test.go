package controller

import (
	"NewAPI-Gateway/model"
	"encoding/json"
	"net/http"
	"reflect"
	"sort"
	"testing"

	"github.com/gin-gonic/gin"
)

func sortedStrings(values []string) []string {
	copied := append([]string{}, values...)
	sort.Strings(copied)
	return copied
}

func TestListModelsAndRouteModelsUseCanonicalCatalog(t *testing.T) {
	originDB := model.DB
	model.DB = prepareControllerModelCatalogTestDB(t)
	defer func() { model.DB = originDB }()
	gin.SetMode(gin.TestMode)

	seedControllerModelCatalogData(t, 1, 1)

	allIDs := mustGetModelIDs(t, http.MethodGet, "/v1/models", newCatalogAggToken(""))
	expectedAll := []string{"aaa", "gpt-4o"}
	if !reflect.DeepEqual(sortedStrings(allIDs), sortedStrings(expectedAll)) {
		t.Fatalf("unexpected canonical model list, got=%v expected=%v", allIDs, expectedAll)
	}

	limitedIDs := mustGetModelIDs(t, http.MethodGet, "/v1/models", newCatalogAggToken("aaa"))
	expectedLimited := []string{"aaa"}
	if !reflect.DeepEqual(sortedStrings(limitedIDs), sortedStrings(expectedLimited)) {
		t.Fatalf("unexpected limited model list, got=%v expected=%v", limitedIDs, expectedLimited)
	}

	ctx, recorder := newTestContext(http.MethodGet, "/api/route/models", nil)
	GetAllModels(ctx)
	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status for /api/route/models: %d", recorder.Code)
	}
	var routeResp struct {
		Success bool     `json:"success"`
		Data    []string `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &routeResp); err != nil {
		t.Fatalf("unmarshal /api/route/models response failed: %v", err)
	}
	if !routeResp.Success {
		t.Fatalf("expected success for /api/route/models")
	}
	if !reflect.DeepEqual(sortedStrings(routeResp.Data), sortedStrings(expectedAll)) {
		t.Fatalf("unexpected /api/route/models data, got=%v expected=%v", routeResp.Data, expectedAll)
	}
}

func TestGetModelAcceptsAliasAndTarget(t *testing.T) {
	originDB := model.DB
	model.DB = prepareControllerModelCatalogTestDB(t)
	defer func() { model.DB = originDB }()
	gin.SetMode(gin.TestMode)

	seedControllerModelCatalogData(t, 1, 1)
	token := newCatalogAggToken("")

	ctxAlias, recorderAlias := newTestContext(http.MethodGet, "/v1/models/aaa", nil)
	ctxAlias.Params = gin.Params{{Key: "model", Value: "aaa"}}
	setAggTokenContext(ctxAlias, token)
	GetModel(ctxAlias)
	if recorderAlias.Code != http.StatusOK {
		t.Fatalf("expected alias lookup to return 200, got %d", recorderAlias.Code)
	}
	var aliasResp struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(recorderAlias.Body.Bytes(), &aliasResp); err != nil {
		t.Fatalf("unmarshal alias response failed: %v", err)
	}
	if aliasResp.ID != "aaa" {
		t.Fatalf("expected canonical id aaa for alias request, got %s", aliasResp.ID)
	}

	ctxTarget, recorderTarget := newTestContext(http.MethodGet, "/v1/models/bbbxxxcccddd", nil)
	ctxTarget.Params = gin.Params{{Key: "model", Value: "bbbxxxcccddd"}}
	setAggTokenContext(ctxTarget, token)
	GetModel(ctxTarget)
	if recorderTarget.Code != http.StatusOK {
		t.Fatalf("expected target lookup to return 200, got %d", recorderTarget.Code)
	}
	var targetResp struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(recorderTarget.Body.Bytes(), &targetResp); err != nil {
		t.Fatalf("unmarshal target response failed: %v", err)
	}
	if targetResp.ID != "aaa" {
		t.Fatalf("expected canonical id aaa for target request, got %s", targetResp.ID)
	}

	ctxDenied, recorderDenied := newTestContext(http.MethodGet, "/v1/models/aaa", nil)
	ctxDenied.Params = gin.Params{{Key: "model", Value: "aaa"}}
	setAggTokenContext(ctxDenied, newCatalogAggToken("gpt-4o"))
	GetModel(ctxDenied)
	if recorderDenied.Code != http.StatusNotFound {
		t.Fatalf("expected denied model to return 404, got %d", recorderDenied.Code)
	}
}
