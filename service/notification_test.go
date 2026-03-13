package service

import (
	"NewAPI-Gateway/common"
	"NewAPI-Gateway/model"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func prepareNotificationServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:notification_service_%s_%d?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"), time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db failed: %v", err)
	}
	if err := db.AutoMigrate(&model.Provider{}, &model.CheckinRun{}, &model.CheckinRunItem{}, &model.UsageLog{}, &model.NotificationAlertState{}); err != nil {
		t.Fatalf("migrate notification service db failed: %v", err)
	}
	return db
}

func withNotificationOptionMap(t *testing.T, overrides map[string]string) func() {
	t.Helper()
	originOptionMap := common.OptionMap
	common.OptionMapRWMutex.Lock()
	common.OptionMap = map[string]string{}
	model.ApplyNotificationOptionDefaults(common.OptionMap)
	for key, value := range overrides {
		common.OptionMap[key] = value
	}
	common.OptionMapRWMutex.Unlock()
	return func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originOptionMap
		common.OptionMapRWMutex.Unlock()
	}
}

func TestDispatchNotificationEventFansOutToConfiguredChannels(t *testing.T) {
	restoreOptionMap := withNotificationOptionMap(t, map[string]string{
		model.NotificationBarkEnabledOptionKey:           "true",
		model.NotificationBarkDeviceKeyOptionKey:         "device-key",
		model.NotificationWebhookEnabledOptionKey:        "true",
		model.NotificationSMTPEnabledOptionKey:           "true",
		model.NotificationSMTPRecipientsOptionKey:        "ops@example.com;oncall@example.com",
		model.NotificationSMTPSubjectPrefixOptionKey:     "[Ops]",
		model.NotificationCheckinFailureEnabledOptionKey: "true",
		model.NotificationVerbosityModeOptionKey:         model.NotificationVerbosityDetailed,
	})
	defer restoreOptionMap()

	var barkPayload map[string]string
	barkServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&barkPayload); err != nil {
			t.Fatalf("decode bark payload failed: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer barkServer.Close()

	var webhookPayload map[string]interface{}
	var webhookAuth string
	webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		webhookAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&webhookPayload); err != nil {
			t.Fatalf("decode webhook payload failed: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer webhookServer.Close()

	common.OptionMapRWMutex.Lock()
	common.OptionMap[model.NotificationBarkServerOptionKey] = barkServer.URL
	common.OptionMap[model.NotificationWebhookURLOptionKey] = webhookServer.URL
	common.OptionMap[model.NotificationWebhookTokenOptionKey] = "secret-token"
	common.OptionMapRWMutex.Unlock()

	originEmailSender := sendNotificationEmail
	defer func() { sendNotificationEmail = originEmailSender }()

	emailCalls := 0
	emailSubject := ""
	emailReceiver := ""
	emailContent := ""
	sendNotificationEmail = func(subject string, receiver string, content string) error {
		emailCalls++
		emailSubject = subject
		emailReceiver = receiver
		emailContent = content
		return nil
	}

	event := NotificationEvent{
		Type:         NotificationTypeCheckinProviderFailed,
		Family:       NotificationFamilyCheckinFailure,
		Severity:     "warning",
		Summary:      "提供商 provider-a 签到失败",
		Detail:       "checkin failed: token invalid",
		OccurredAt:   1730000000,
		ProviderID:   1,
		ProviderName: "provider-a",
		Reason:       "checkin failed: token invalid",
	}
	if err := dispatchNotificationEvent(event); err != nil {
		t.Fatalf("dispatch notification event failed: %v", err)
	}

	if barkPayload["device_key"] != "device-key" {
		t.Fatalf("unexpected bark device key: %+v", barkPayload)
	}
	if !strings.Contains(barkPayload["body"], "Provider: provider-a") {
		t.Fatalf("expected detailed bark body, got %q", barkPayload["body"])
	}
	if webhookAuth != "Bearer secret-token" {
		t.Fatalf("unexpected webhook auth header: %s", webhookAuth)
	}
	if webhookPayload["provider_name"] != "provider-a" {
		t.Fatalf("expected webhook payload provider_name, got %+v", webhookPayload)
	}
	if emailCalls != 1 {
		t.Fatalf("expected 1 email call, got %d", emailCalls)
	}
	if emailSubject != "[Ops] 提供商 provider-a 签到失败" {
		t.Fatalf("unexpected email subject: %s", emailSubject)
	}
	if emailReceiver != "ops@example.com;oncall@example.com" {
		t.Fatalf("unexpected email receiver: %s", emailReceiver)
	}
	if !strings.Contains(emailContent, "checkin failed: token invalid") {
		t.Fatalf("expected email content to contain error detail, got %s", emailContent)
	}
}

func TestTriggerFullCheckinRunEmitsSummaryFailureAndAutoDisableNotifications(t *testing.T) {
	originDB := model.DB
	model.DB = prepareNotificationServiceTestDB(t)
	defer func() { model.DB = originDB }()

	originRunner := notificationDispatchRunner
	originAsync := notificationAsyncExecutor
	defer func() {
		notificationDispatchRunner = originRunner
		notificationAsyncExecutor = originAsync
	}()

	var events []NotificationEvent
	notificationAsyncExecutor = func(fn func()) { fn() }
	notificationDispatchRunner = func(event NotificationEvent) error {
		events = append(events, event)
		return nil
	}

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

	run, err := TriggerFullCheckinRun("manual")
	if err != nil {
		t.Fatalf("trigger full checkin run failed: %v", err)
	}
	if run == nil {
		t.Fatalf("expected run to be returned")
	}
	if len(events) != 3 {
		t.Fatalf("expected summary, failure, and auto-disable notifications, got %d events: %+v", len(events), events)
	}
	if events[0].Type != NotificationTypeCheckinRunCompleted {
		t.Fatalf("expected first event to be checkin summary, got %+v", events[0])
	}
	if events[1].Type != NotificationTypeCheckinProviderFailed {
		t.Fatalf("expected second event to be checkin failure, got %+v", events[1])
	}
	if events[2].Type != NotificationTypeCheckinAutoDisabled {
		t.Fatalf("expected third event to be auto-disable, got %+v", events[2])
	}
}

func TestSyncProviderEmitsTransitionNotificationsWithoutDuplicates(t *testing.T) {
	originDB := model.DB
	model.DB = prepareNotificationServiceTestDB(t)
	defer func() { model.DB = originDB }()

	originRunner := notificationDispatchRunner
	originAsync := notificationAsyncExecutor
	originClientFactory := newUpstreamClientForProvider
	originSyncPricing := syncPricingStep
	originSyncTokens := syncTokensStep
	originSyncBalance := syncBalanceStep
	originRebuildRoutes := rebuildProviderRoutesForProvider
	defer func() {
		notificationDispatchRunner = originRunner
		notificationAsyncExecutor = originAsync
		newUpstreamClientForProvider = originClientFactory
		syncPricingStep = originSyncPricing
		syncTokensStep = originSyncTokens
		syncBalanceStep = originSyncBalance
		rebuildProviderRoutesForProvider = originRebuildRoutes
	}()

	var events []NotificationEvent
	notificationAsyncExecutor = func(fn func()) { fn() }
	notificationDispatchRunner = func(event NotificationEvent) error {
		events = append(events, event)
		return nil
	}

	provider := &model.Provider{
		Name:        "sync-health-provider",
		BaseURL:     "https://example.com",
		AccessToken: "token",
		UserID:      1,
		Status:      1,
	}
	if err := provider.Insert(); err != nil {
		t.Fatalf("insert provider failed: %v", err)
	}

	newUpstreamClientForProvider = func(provider *model.Provider) (*UpstreamClient, error) {
		return &UpstreamClient{Provider: provider}, nil
	}
	syncPricingStep = func(client *UpstreamClient, provider *model.Provider) error {
		return &UpstreamRequestError{Message: "dial tcp timeout", Transport: true}
	}
	syncTokensStep = func(client *UpstreamClient, provider *model.Provider) error {
		return &UpstreamRequestError{Message: "dial tcp timeout", Transport: true}
	}
	syncBalanceStep = func(client *UpstreamClient, provider *model.Provider) error {
		return nil
	}
	rebuildProviderRoutesForProvider = func(providerID int) error { return nil }

	if err := SyncProvider(provider); err != nil {
		t.Fatalf("unexpected sync failure: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected no notification when a later sync step proves reachability, got %+v", events)
	}

	syncBalanceStep = func(client *UpstreamClient, provider *model.Provider) error {
		return &UpstreamRequestError{Message: "dial tcp timeout", Transport: true}
	}
	if err := SyncProvider(provider); err != nil {
		t.Fatalf("unexpected repeated sync failure: %v", err)
	}
	if len(events) != 1 || events[0].Type != NotificationTypeProviderUnreachable {
		t.Fatalf("expected a single unreachable notification after fully failed sync, got %+v", events)
	}

	if err := SyncProvider(provider); err != nil {
		t.Fatalf("unexpected duplicate-failure sync: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected no duplicate unreachable notification, got %+v", events)
	}

	syncPricingStep = func(client *UpstreamClient, provider *model.Provider) error { return nil }
	syncTokensStep = func(client *UpstreamClient, provider *model.Provider) error { return nil }
	syncBalanceStep = func(client *UpstreamClient, provider *model.Provider) error { return nil }
	if err := SyncProvider(provider); err != nil {
		t.Fatalf("unexpected recovery sync failure: %v", err)
	}
	if len(events) != 2 || events[1].Type != NotificationTypeProviderRecovered {
		t.Fatalf("expected recovery notification after success, got %+v", events)
	}
}

func TestEvaluateRequestFailureAlertThresholdAndDedupe(t *testing.T) {
	originDB := model.DB
	model.DB = prepareNotificationServiceTestDB(t)
	defer func() { model.DB = originDB }()

	restoreOptionMap := withNotificationOptionMap(t, map[string]string{
		model.NotificationRequestFailureEnabledOptionKey:       "true",
		model.NotificationRequestFailureThresholdOptionKey:     "2",
		model.NotificationRequestFailureWindowMinutesOptionKey: "10",
	})
	defer restoreOptionMap()

	originRunner := notificationDispatchRunner
	originAsync := notificationAsyncExecutor
	defer func() {
		notificationDispatchRunner = originRunner
		notificationAsyncExecutor = originAsync
	}()

	var events []NotificationEvent
	notificationAsyncExecutor = func(fn func()) { fn() }
	notificationDispatchRunner = func(event NotificationEvent) error {
		events = append(events, event)
		return nil
	}

	log1 := &model.UsageLog{
		ProviderId:      7,
		ProviderName:    "provider-7",
		Status:          0,
		ErrorMessage:    "dial tcp timeout",
		FailureCategory: model.UsageFailureCategoryTransport,
	}
	if err := log1.Insert(); err != nil {
		t.Fatalf("insert first usage log failed: %v", err)
	}
	EvaluateRequestFailureAlert(log1)
	if len(events) != 0 {
		t.Fatalf("expected no notification below threshold, got %+v", events)
	}

	log2 := &model.UsageLog{
		ProviderId:      7,
		ProviderName:    "provider-7",
		Status:          0,
		ErrorMessage:    "dial tcp timeout",
		FailureCategory: model.UsageFailureCategoryTransport,
	}
	if err := log2.Insert(); err != nil {
		t.Fatalf("insert second usage log failed: %v", err)
	}
	EvaluateRequestFailureAlert(log2)
	if len(events) != 1 || events[0].Type != NotificationTypeRequestFailureAlert {
		t.Fatalf("expected one threshold alert event, got %+v", events)
	}

	log3 := &model.UsageLog{
		ProviderId:      7,
		ProviderName:    "provider-7",
		Status:          0,
		ErrorMessage:    "dial tcp timeout",
		FailureCategory: model.UsageFailureCategoryTransport,
	}
	if err := log3.Insert(); err != nil {
		t.Fatalf("insert third usage log failed: %v", err)
	}
	EvaluateRequestFailureAlert(log3)
	if len(events) != 1 {
		t.Fatalf("expected deduped request failure alert, got %+v", events)
	}
}
