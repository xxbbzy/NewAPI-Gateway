package service

import (
	"NewAPI-Gateway/model"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRunProviderCheckinAutoDisablesOnUpstreamDisabledMessage(t *testing.T) {
	originDB := model.DB
	model.DB = prepareServiceCheckinTestDB(t)
	defer func() { model.DB = originDB }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":false,"message":"签到功能未启用","data":{"quota_awarded":0}}`))
	}))
	defer server.Close()

	provider := &model.Provider{
		Name:           "auto-disable-provider",
		BaseURL:        server.URL,
		AccessToken:    "token",
		UserID:         1,
		Status:         1,
		CheckinEnabled: true,
	}
	if err := provider.Insert(); err != nil {
		t.Fatalf("insert provider failed: %v", err)
	}

	run, item, err := RunProviderCheckin(provider)
	if err == nil {
		t.Fatalf("expected checkin error for upstream-disabled provider")
	}
	if run == nil || item == nil {
		t.Fatalf("expected run and item to be recorded on failure")
	}
	if item.Status != "failed" {
		t.Fatalf("expected failed item status, got %s", item.Status)
	}
	if !item.AutoDisabled {
		t.Fatalf("expected item auto_disabled=true for upstream-disabled failure")
	}
	if !strings.Contains(item.Message, upstreamCheckinAutoDisableHint) {
		t.Fatalf("expected auto-disable hint in message, got %s", item.Message)
	}

	reloaded, queryErr := model.GetProviderById(provider.Id)
	if queryErr != nil {
		t.Fatalf("query provider failed: %v", queryErr)
	}
	if reloaded.CheckinEnabled {
		t.Fatalf("expected provider checkin to be auto-disabled")
	}
}

func TestRunProviderCheckinKeepsCheckinEnabledForGenericFailure(t *testing.T) {
	originDB := model.DB
	model.DB = prepareServiceCheckinTestDB(t)
	defer func() { model.DB = originDB }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":false,"message":"token invalid","data":{"quota_awarded":0}}`))
	}))
	defer server.Close()

	provider := &model.Provider{
		Name:           "generic-failure-provider",
		BaseURL:        server.URL,
		AccessToken:    "token",
		UserID:         1,
		Status:         1,
		CheckinEnabled: true,
	}
	if err := provider.Insert(); err != nil {
		t.Fatalf("insert provider failed: %v", err)
	}

	_, item, err := RunProviderCheckin(provider)
	if err == nil {
		t.Fatalf("expected checkin error for generic failure")
	}
	if item == nil {
		t.Fatalf("expected failed item to be persisted")
	}
	if item.AutoDisabled {
		t.Fatalf("unexpected auto_disabled=true for generic failure")
	}
	if strings.Contains(item.Message, upstreamCheckinAutoDisableHint) {
		t.Fatalf("unexpected auto-disable hint for generic failure: %s", item.Message)
	}

	reloaded, queryErr := model.GetProviderById(provider.Id)
	if queryErr != nil {
		t.Fatalf("query provider failed: %v", queryErr)
	}
	if !reloaded.CheckinEnabled {
		t.Fatalf("expected provider checkin to remain enabled for generic failure")
	}
}

func TestGetUncheckinProvidersExcludesAutoDisabledProvider(t *testing.T) {
	originDB := model.DB
	model.DB = prepareServiceCheckinTestDB(t)
	defer func() { model.DB = originDB }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":false,"message":"checkin failed: 签到功能未启用","data":{"quota_awarded":0}}`))
	}))
	defer server.Close()

	autoDisabledProvider := &model.Provider{
		Name:           "auto-disabled-provider",
		BaseURL:        server.URL,
		AccessToken:    "token-a",
		UserID:         1,
		Status:         1,
		CheckinEnabled: true,
	}
	normalUncheckedProvider := &model.Provider{
		Name:           "normal-unchecked-provider",
		BaseURL:        "https://example.com",
		AccessToken:    "token-b",
		UserID:         2,
		Status:         1,
		CheckinEnabled: true,
		LastCheckinAt:  0,
	}
	if err := autoDisabledProvider.Insert(); err != nil {
		t.Fatalf("insert auto-disabled provider failed: %v", err)
	}
	if err := normalUncheckedProvider.Insert(); err != nil {
		t.Fatalf("insert normal provider failed: %v", err)
	}

	if _, _, err := RunProviderCheckin(autoDisabledProvider); err == nil {
		t.Fatalf("expected upstream-disabled checkin error")
	}

	unchecked, _, _, err := GetUncheckinProviders(time.Now())
	if err != nil {
		t.Fatalf("query unchecked providers failed: %v", err)
	}

	if len(unchecked) != 1 {
		t.Fatalf("expected exactly one unchecked provider, got %d", len(unchecked))
	}
	if unchecked[0].Id != normalUncheckedProvider.Id {
		t.Fatalf("unexpected unchecked provider: %s", unchecked[0].Name)
	}
}
