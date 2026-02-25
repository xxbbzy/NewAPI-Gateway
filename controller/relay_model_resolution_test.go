package controller

import (
	"NewAPI-Gateway/model"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestExtractRequestedModelPrefersBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := newTestContext(http.MethodPost, "/v1beta/models/gemini-2.5-pro:generateContent", []byte(`{"model":"aaa"}`))
	modelName := extractRequestedModel(ctx)
	if modelName != "aaa" {
		t.Fatalf("expected body model aaa, got %s", modelName)
	}
}

func TestExtractRequestedModelFromGeminiPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := newTestContext(http.MethodPost, "/v1beta/models/gemini-2.5-pro:generateContent", []byte(`{"contents":[]}`))
	modelName := extractRequestedModel(ctx)
	if modelName != "gemini-2.5-pro" {
		t.Fatalf("expected gemini model from path, got %s", modelName)
	}
}

func TestRelayResolvesCatalogModelBeforeWhitelist(t *testing.T) {
	originDB := model.DB
	model.DB = prepareControllerModelCatalogTestDB(t)
	defer func() { model.DB = originDB }()
	gin.SetMode(gin.TestMode)

	// Provider/token intentionally disabled to avoid actual upstream calls.
	seedControllerModelCatalogData(t, 0, 0)

	ctx, recorder := newTestContext(http.MethodPost, "/v1/chat/completions", []byte(`{"model":"bbbxxxcccddd","messages":[]}`))
	setAggTokenContext(ctx, newCatalogAggToken("aaa"))
	Relay(ctx)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 after whitelist pass and no enabled route attempt, got %d", recorder.Code)
	}
	if ctx.GetString("request_model_original") != "bbbxxxcccddd" {
		t.Fatalf("unexpected request_model_original: %s", ctx.GetString("request_model_original"))
	}
	if ctx.GetString("request_model_canonical") != "aaa" {
		t.Fatalf("expected request_model_canonical aaa, got %s", ctx.GetString("request_model_canonical"))
	}
}

func TestRelayExtractsGeminiPathAndResolvesCatalogModel(t *testing.T) {
	originDB := model.DB
	model.DB = prepareControllerModelCatalogTestDB(t)
	defer func() { model.DB = originDB }()
	gin.SetMode(gin.TestMode)

	// Provider/token intentionally disabled to avoid actual upstream calls.
	seedControllerModelCatalogData(t, 0, 0)

	ctx, recorder := newTestContext(http.MethodPost, "/v1beta/models/bbbxxxcccddd:generateContent", []byte(`{"contents":[]}`))
	setAggTokenContext(ctx, newCatalogAggToken("aaa"))
	Relay(ctx)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 after whitelist pass and no enabled route attempt, got %d", recorder.Code)
	}
	if ctx.GetString("request_model_original") != "bbbxxxcccddd" {
		t.Fatalf("expected original model from gemini path, got %s", ctx.GetString("request_model_original"))
	}
	if canonical := ctx.GetString("request_model_canonical"); canonical != "aaa" {
		t.Fatalf("expected canonical aaa, got %s", canonical)
	}
	if !strings.Contains(recorder.Body.String(), "service_unavailable") {
		t.Fatalf("expected service_unavailable error payload, got %s", recorder.Body.String())
	}
}
