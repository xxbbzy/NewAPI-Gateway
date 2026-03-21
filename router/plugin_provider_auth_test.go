package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"NewAPI-Gateway/common"
	"NewAPI-Gateway/model"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type managementEnvelope struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func forceInMemoryRateLimiter() func() {
	original := common.RedisEnabled
	common.RedisEnabled = false
	return func() {
		common.RedisEnabled = original
	}
}

func preparePluginProviderRouterTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:router_plugin_provider_%s_%d?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"), time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db failed: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.Provider{}, &model.ProviderToken{}, &model.ModelPricing{}, &model.ModelRoute{}); err != nil {
		t.Fatalf("migrate test db failed: %v", err)
	}
	return db
}

func buildPluginProviderRouterTestServer(admin *model.User) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	store := cookie.NewStore([]byte("plugin-provider-test"))
	r.Use(sessions.Sessions("session", store))
	r.GET("/test/session/admin", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("id", admin.Id)
		session.Set("username", admin.Username)
		session.Set("role", admin.Role)
		session.Set("status", admin.Status)
		_ = session.Save()
		c.JSON(http.StatusOK, gin.H{"success": true})
	})
	SetApiRouter(r)
	return r
}

func decodeManagementEnvelope(t *testing.T, recorder *httptest.ResponseRecorder) managementEnvelope {
	t.Helper()
	var resp managementEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response failed: %v, body=%s", err, recorder.Body.String())
	}
	return resp
}

func newRequest(method string, path string, token string, sessionCookie *http.Cookie) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if sessionCookie != nil {
		req.AddCookie(sessionCookie)
	}
	return req
}

func newJSONRequest(method string, path string, body string, token string, sessionCookie *http.Cookie) *http.Request {
	req := httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if sessionCookie != nil {
		req.AddCookie(sessionCookie)
	}
	return req
}

func loginAsAdminAndGetSessionCookie(t *testing.T, router *gin.Engine) *http.Cookie {
	t.Helper()
	loginRecorder := httptest.NewRecorder()
	router.ServeHTTP(loginRecorder, newRequest(http.MethodGet, "/test/session/admin", "", nil))
	if loginRecorder.Code != http.StatusOK {
		t.Fatalf("expected helper login success, got %d", loginRecorder.Code)
	}
	for _, cookie := range loginRecorder.Result().Cookies() {
		if cookie.Name == "session" {
			return cookie
		}
	}
	t.Fatalf("expected session cookie from helper login")
	return nil
}

func seedUsersAndProviderForPluginTests(t *testing.T, providerBaseURL string) (*model.User, *model.User, *model.Provider, string) {
	t.Helper()
	admin := &model.User{
		Username:    "admin_user",
		Password:    "ignored",
		DisplayName: "Admin",
		Role:        common.RoleAdminUser,
		Status:      common.UserStatusEnabled,
		Token:       "admin-token",
	}
	if err := model.DB.Create(admin).Error; err != nil {
		t.Fatalf("seed admin failed: %v", err)
	}
	user := &model.User{
		Username:    "normal_user",
		Password:    "ignored",
		DisplayName: "User",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Token:       "user-token",
	}
	if err := model.DB.Create(user).Error; err != nil {
		t.Fatalf("seed normal user failed: %v", err)
	}

	provider := &model.Provider{
		Name:        "provider-a",
		BaseURL:     providerBaseURL,
		AccessToken: "upstream-access-secret",
		UserID:      1,
		Status:      1,
		Priority:    0,
		Weight:      10,
	}
	if err := provider.Insert(); err != nil {
		t.Fatalf("seed provider failed: %v", err)
	}

	rawSk := "sk-abcdefghijklmnop"
	if err := model.DB.Create(&model.ProviderToken{
		ProviderId:      provider.Id,
		UpstreamTokenId: 100,
		SkKey:           rawSk,
		Name:            "token-a",
		GroupName:       "default",
		Status:          1,
		Priority:        0,
		Weight:          10,
	}).Error; err != nil {
		t.Fatalf("seed provider token failed: %v", err)
	}
	return admin, user, provider, rawSk
}

func TestPluginProviderAdminTokenSuccessAndRedaction(t *testing.T) {
	restoreRateLimiter := forceInMemoryRateLimiter()
	defer restoreRateLimiter()

	originDB := model.DB
	model.DB = preparePluginProviderRouterTestDB(t)
	defer func() { model.DB = originDB }()

	admin, _, provider, rawSk := seedUsersAndProviderForPluginTests(t, "https://example.invalid")
	router := buildPluginProviderRouterTestServer(admin)

	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, newRequest(http.MethodGet, "/api/plugin/provider/?p=0&page_size=10", admin.Token, nil))
	listResp := decodeManagementEnvelope(t, listRecorder)
	if !listResp.Success {
		t.Fatalf("expected plugin provider list success, got message=%s", listResp.Message)
	}

	var page struct {
		Items     []map[string]interface{} `json:"items"`
		P         int                      `json:"p"`
		PageSize  int                      `json:"page_size"`
		Total     int64                    `json:"total"`
		TotalPage int                      `json:"total_pages"`
		HasMore   bool                     `json:"has_more"`
	}
	if err := json.Unmarshal(listResp.Data, &page); err != nil {
		t.Fatalf("decode plugin list page failed: %v", err)
	}
	if page.P != 0 || page.PageSize != 10 || page.Total != 1 || page.TotalPage != 1 || page.HasMore {
		t.Fatalf("unexpected pagination metadata: %+v", page)
	}
	if len(page.Items) != 1 {
		t.Fatalf("expected one provider item, got %d", len(page.Items))
	}
	if value, ok := page.Items[0]["access_token"].(string); !ok || value != "" {
		t.Fatalf("expected provider access_token redacted as empty string, got %#v", page.Items[0]["access_token"])
	}

	tokenRecorder := httptest.NewRecorder()
	path := fmt.Sprintf("/api/plugin/provider/%d/tokens", provider.Id)
	router.ServeHTTP(tokenRecorder, newRequest(http.MethodGet, path, admin.Token, nil))
	tokenResp := decodeManagementEnvelope(t, tokenRecorder)
	if !tokenResp.Success {
		t.Fatalf("expected plugin provider tokens success, got message=%s", tokenResp.Message)
	}

	var tokens []map[string]interface{}
	if err := json.Unmarshal(tokenResp.Data, &tokens); err != nil {
		t.Fatalf("decode token list failed: %v", err)
	}
	if len(tokens) != 1 {
		t.Fatalf("expected one provider token, got %d", len(tokens))
	}
	skValue, _ := tokens[0]["sk_key"].(string)
	if skValue == "" || skValue == rawSk || !strings.Contains(skValue, "****") {
		t.Fatalf("expected masked sk_key, got %q", skValue)
	}
}

func TestPluginProviderRejectsNonAdminToken(t *testing.T) {
	restoreRateLimiter := forceInMemoryRateLimiter()
	defer restoreRateLimiter()

	originDB := model.DB
	model.DB = preparePluginProviderRouterTestDB(t)
	defer func() { model.DB = originDB }()

	admin, user, _, _ := seedUsersAndProviderForPluginTests(t, "https://example.invalid")
	router := buildPluginProviderRouterTestServer(admin)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, newRequest(http.MethodGet, "/api/plugin/provider/?p=0&page_size=10", user.Token, nil))
	resp := decodeManagementEnvelope(t, recorder)
	if resp.Success {
		t.Fatalf("expected non-admin token rejection")
	}
	if !strings.Contains(resp.Message, "权限不足") {
		t.Fatalf("expected permission message, got %s", resp.Message)
	}
}

func TestPluginProviderRejectsSessionOnlyAuth(t *testing.T) {
	restoreRateLimiter := forceInMemoryRateLimiter()
	defer restoreRateLimiter()

	originDB := model.DB
	model.DB = preparePluginProviderRouterTestDB(t)
	defer func() { model.DB = originDB }()

	admin, _, _, _ := seedUsersAndProviderForPluginTests(t, "https://example.invalid")
	router := buildPluginProviderRouterTestServer(admin)
	sessionCookie := loginAsAdminAndGetSessionCookie(t, router)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, newRequest(http.MethodGet, "/api/plugin/provider/?p=0&page_size=10", "", sessionCookie))
	resp := decodeManagementEnvelope(t, recorder)
	if resp.Success {
		t.Fatalf("expected session-only auth rejection")
	}
	if !strings.Contains(resp.Message, "仅支持使用 token") {
		t.Fatalf("expected token-only message, got %s", resp.Message)
	}
}

func TestGeneratePersonalTokenViaSession(t *testing.T) {
	restoreRateLimiter := forceInMemoryRateLimiter()
	defer restoreRateLimiter()

	originDB := model.DB
	model.DB = preparePluginProviderRouterTestDB(t)
	defer func() { model.DB = originDB }()

	admin, _, _, _ := seedUsersAndProviderForPluginTests(t, "https://example.invalid")
	router := buildPluginProviderRouterTestServer(admin)
	sessionCookie := loginAsAdminAndGetSessionCookie(t, router)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, newRequest(http.MethodGet, "/api/user/token", "", sessionCookie))
	resp := decodeManagementEnvelope(t, recorder)
	if !resp.Success {
		t.Fatalf("expected generate token success, got message=%s", resp.Message)
	}

	var generatedToken string
	if err := json.Unmarshal(resp.Data, &generatedToken); err != nil {
		t.Fatalf("decode generated token failed: %v", err)
	}
	if len(generatedToken) != 32 || strings.Contains(generatedToken, "-") {
		t.Fatalf("expected generated token with 32 hex chars, got %q", generatedToken)
	}

	refreshed, err := model.GetUserById(admin.Id, true)
	if err != nil {
		t.Fatalf("reload admin failed: %v", err)
	}
	if refreshed.Token != generatedToken {
		t.Fatalf("expected persisted token %q, got %q", generatedToken, refreshed.Token)
	}
}

func TestLegacyProviderRouteStillRejectsTokenAuth(t *testing.T) {
	restoreRateLimiter := forceInMemoryRateLimiter()
	defer restoreRateLimiter()

	originDB := model.DB
	model.DB = preparePluginProviderRouterTestDB(t)
	defer func() { model.DB = originDB }()

	admin, _, _, _ := seedUsersAndProviderForPluginTests(t, "https://example.invalid")
	router := buildPluginProviderRouterTestServer(admin)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, newRequest(http.MethodGet, "/api/provider/?p=0&page_size=10", admin.Token, nil))
	resp := decodeManagementEnvelope(t, recorder)
	if resp.Success {
		t.Fatalf("expected legacy provider route to reject token auth")
	}
	if !strings.Contains(resp.Message, "不支持使用 token") {
		t.Fatalf("expected NoTokenAuth rejection message, got %s", resp.Message)
	}
}

func TestPluginProviderSyncTriggerReturnsAsyncSuccessMessage(t *testing.T) {
	restoreRateLimiter := forceInMemoryRateLimiter()
	defer restoreRateLimiter()

	originDB := model.DB
	model.DB = preparePluginProviderRouterTestDB(t)
	defer func() { model.DB = originDB }()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/pricing":
			_, _ = w.Write([]byte(`{"data":[],"group_ratio":{},"usable_group":{},"supported_endpoint":{}}`))
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/token/"):
			_, _ = w.Write([]byte(`{"success":true,"message":"","data":{"page":0,"page_size":100,"total":0,"items":[]}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/user/self":
			_, _ = w.Write([]byte(`{"success":true,"data":{"id":1,"quota":0,"status":1}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	admin, _, provider, _ := seedUsersAndProviderForPluginTests(t, upstream.URL)
	router := buildPluginProviderRouterTestServer(admin)

	recorder := httptest.NewRecorder()
	path := fmt.Sprintf("/api/plugin/provider/%d/sync", provider.Id)
	router.ServeHTTP(recorder, newRequest(http.MethodPost, path, admin.Token, nil))
	resp := decodeManagementEnvelope(t, recorder)
	if !resp.Success {
		t.Fatalf("expected plugin sync trigger success, got message=%s", resp.Message)
	}
	if resp.Message != "同步任务已启动" {
		t.Fatalf("expected async trigger message, got %s", resp.Message)
	}

	// Allow background sync goroutine to finish before test DB cleanup.
	time.Sleep(200 * time.Millisecond)
}

func TestPluginProviderImportRouteUpsertWithAdminToken(t *testing.T) {
	restoreRateLimiter := forceInMemoryRateLimiter()
	defer restoreRateLimiter()

	originDB := model.DB
	model.DB = preparePluginProviderRouterTestDB(t)
	defer func() { model.DB = originDB }()

	admin, _, _, _ := seedUsersAndProviderForPluginTests(t, "https://example.invalid")
	router := buildPluginProviderRouterTestServer(admin)

	firstImportBody := `[{
		"name":"import-provider",
		"base_url":"https://import.example",
		"access_token":"import-token-a",
		"user_id":22,
		"status":1,
		"priority":2,
		"weight":7
	}]`
	firstRecorder := httptest.NewRecorder()
	router.ServeHTTP(firstRecorder, newJSONRequest(http.MethodPost, "/api/plugin/provider/import", firstImportBody, admin.Token, nil))
	firstResp := decodeManagementEnvelope(t, firstRecorder)
	if !firstResp.Success {
		t.Fatalf("expected first plugin import success, got message=%s", firstResp.Message)
	}

	importedProvider, err := model.FindProviderByBaseURLAndUserID("https://import.example", 22)
	if err != nil || importedProvider == nil {
		t.Fatalf("expected imported provider, err=%v", err)
	}
	if importedProvider.Name != "import-provider" || importedProvider.AccessToken != "import-token-a" {
		t.Fatalf("unexpected imported provider content: %+v", importedProvider)
	}

	secondImportBody := `[{
		"name":"import-provider-updated",
		"base_url":"https://import.example",
		"access_token":"import-token-b",
		"user_id":22,
		"status":1,
		"priority":4,
		"weight":9
	}]`
	secondRecorder := httptest.NewRecorder()
	router.ServeHTTP(secondRecorder, newJSONRequest(http.MethodPost, "/api/plugin/provider/import", secondImportBody, admin.Token, nil))
	secondResp := decodeManagementEnvelope(t, secondRecorder)
	if !secondResp.Success {
		t.Fatalf("expected second plugin import success, got message=%s", secondResp.Message)
	}

	updatedProvider, err := model.FindProviderByBaseURLAndUserID("https://import.example", 22)
	if err != nil || updatedProvider == nil {
		t.Fatalf("expected updated provider, err=%v", err)
	}
	if updatedProvider.Name != "import-provider-updated" || updatedProvider.AccessToken != "import-token-b" || updatedProvider.Priority != 4 || updatedProvider.Weight != 9 {
		t.Fatalf("unexpected updated provider content: %+v", updatedProvider)
	}

	var matched int64
	if err := model.DB.Model(&model.Provider{}).Where("base_url = ? AND user_id = ?", "https://import.example", 22).Count(&matched).Error; err != nil {
		t.Fatalf("count imported providers failed: %v", err)
	}
	if matched != 1 {
		t.Fatalf("expected one upserted provider row, got %d", matched)
	}
}

func TestPluginProviderTokenCreateRejectsInvalidGroupWithFailureEnvelope(t *testing.T) {
	restoreRateLimiter := forceInMemoryRateLimiter()
	defer restoreRateLimiter()

	originDB := model.DB
	model.DB = preparePluginProviderRouterTestDB(t)
	defer func() { model.DB = originDB }()

	admin, _, provider, _ := seedUsersAndProviderForPluginTests(t, "https://example.invalid")
	if err := model.DB.Create(&model.ModelPricing{
		ModelName:    "gpt-4o-mini",
		ProviderId:   provider.Id,
		EnableGroups: `["default"]`,
	}).Error; err != nil {
		t.Fatalf("seed model pricing failed: %v", err)
	}

	router := buildPluginProviderRouterTestServer(admin)
	requestBody := `{"name":"bad-group-token","group_name":"not-allowed","unlimited_quota":true}`
	recorder := httptest.NewRecorder()
	path := fmt.Sprintf("/api/plugin/provider/%d/tokens", provider.Id)
	router.ServeHTTP(recorder, newJSONRequest(http.MethodPost, path, requestBody, admin.Token, nil))
	resp := decodeManagementEnvelope(t, recorder)
	if resp.Success {
		t.Fatalf("expected invalid group to return failure envelope")
	}
	if !strings.Contains(resp.Message, "分组不属于该渠道可用分组") {
		t.Fatalf("unexpected validation message: %s", resp.Message)
	}
}

func TestPluginProviderTokenLifecycleCreateUpdateDelete(t *testing.T) {
	restoreRateLimiter := forceInMemoryRateLimiter()
	defer restoreRateLimiter()

	originDB := model.DB
	model.DB = preparePluginProviderRouterTestDB(t)
	defer func() { model.DB = originDB }()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/token/":
			_, _ = w.Write([]byte(`{"success":true,"message":"","data":{}}`))
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/token/"):
			_, _ = w.Write([]byte(`{"success":true,"message":"","data":{}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/pricing":
			_, _ = w.Write([]byte(`{"data":[{"model_name":"gpt-4o-mini","enable_groups":["default"]}],"group_ratio":{},"usable_group":{},"supported_endpoint":{}}`))
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/token/"):
			query := r.URL.Query()
			page := query.Get("p")
			if page == "" {
				page = "0"
			}
			if page == "0" {
				_, _ = w.Write([]byte(`{"success":true,"message":"","data":{"page":0,"page_size":100,"total":1,"items":[{"id":100,"key":"abcdefghijklmnop","name":"token-a","status":1,"group":"default","remain_quota":1,"unlimited_quota":true,"used_quota":0,"model_limits_enabled":false,"model_limits":""}]}}`))
				return
			}
			_, _ = w.Write([]byte(`{"success":true,"message":"","data":{"page":1,"page_size":100,"total":1,"items":[]}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/user/self":
			_, _ = w.Write([]byte(`{"success":true,"data":{"id":1,"quota":0,"status":1}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	admin, _, provider, _ := seedUsersAndProviderForPluginTests(t, upstream.URL)
	if err := model.DB.Create(&model.ModelPricing{
		ModelName:    "gpt-4o-mini",
		ProviderId:   provider.Id,
		EnableGroups: `["default"]`,
	}).Error; err != nil {
		t.Fatalf("seed model pricing failed: %v", err)
	}
	router := buildPluginProviderRouterTestServer(admin)

	// Create (plugin namespace) should call upstream and return async sync message.
	createBody := `{"name":"created-by-plugin","group_name":"default","unlimited_quota":true,"remain_quota":0}`
	createRecorder := httptest.NewRecorder()
	createPath := fmt.Sprintf("/api/plugin/provider/%d/tokens", provider.Id)
	router.ServeHTTP(createRecorder, newJSONRequest(http.MethodPost, createPath, createBody, admin.Token, nil))
	createResp := decodeManagementEnvelope(t, createRecorder)
	if !createResp.Success {
		t.Fatalf("expected plugin token create success, got message=%s", createResp.Message)
	}
	if createResp.Message != "Token 已在上游创建并同步完成" {
		t.Fatalf("unexpected create message: %s", createResp.Message)
	}

	// Sync is now synchronous in CreateProviderToken, no need to wait.
	var existing model.ProviderToken
	if err := model.DB.Where("provider_id = ? AND upstream_token_id = ?", provider.Id, 100).First(&existing).Error; err != nil {
		t.Fatalf("expected existing synced token for lifecycle update/delete, err=%v", err)
	}

	// Update (plugin namespace) should update local fields but NOT sk_key.
	updateBody := fmt.Sprintf(`{"provider_id":%d,"name":"plugin-updated-token","group_name":"default","status":1,"priority":6,"weight":12,"sk_key":"sk-EVIL_OVERWRITE"}`, provider.Id)
	updateRecorder := httptest.NewRecorder()
	updatePath := fmt.Sprintf("/api/plugin/provider/token/%d", existing.Id)
	router.ServeHTTP(updateRecorder, newJSONRequest(http.MethodPut, updatePath, updateBody, admin.Token, nil))
	updateResp := decodeManagementEnvelope(t, updateRecorder)
	if !updateResp.Success {
		t.Fatalf("expected plugin token update success, got message=%s", updateResp.Message)
	}

	updatedToken, err := model.GetProviderTokenById(existing.Id)
	if err != nil {
		t.Fatalf("query updated token failed: %v", err)
	}
	if updatedToken.Name != "plugin-updated-token" || updatedToken.Priority != 6 || updatedToken.Weight != 12 {
		t.Fatalf("unexpected updated token content: %+v", updatedToken)
	}
	// sk_key must NOT have been overwritten by the frontend value
	if updatedToken.SkKey == "sk-EVIL_OVERWRITE" {
		t.Fatalf("sk_key was overwritten by frontend request — UpdateMetadataOnly should prevent this")
	}
	if updatedToken.SkKey != "sk-abcdefghijklmnop" {
		t.Fatalf("expected preserved sk_key 'sk-abcdefghijklmnop', got %q", updatedToken.SkKey)
	}

	// Delete (plugin namespace) should delete upstream then local.
	deleteRecorder := httptest.NewRecorder()
	deletePath := fmt.Sprintf("/api/plugin/provider/token/%d", existing.Id)
	router.ServeHTTP(deleteRecorder, newRequest(http.MethodDelete, deletePath, admin.Token, nil))
	deleteResp := decodeManagementEnvelope(t, deleteRecorder)
	if !deleteResp.Success {
		t.Fatalf("expected plugin token delete success, got message=%s", deleteResp.Message)
	}

	if _, err := model.GetProviderTokenById(existing.Id); err == nil {
		t.Fatalf("expected token to be deleted locally")
	}
}
