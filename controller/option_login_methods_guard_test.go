package controller

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

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type updateOptionResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func prepareOptionGuardTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:option_guard_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db failed: %v", err)
	}
	if err := db.AutoMigrate(&model.Option{}); err != nil {
		t.Fatalf("migrate option table failed: %v", err)
	}
	return db
}

func performUpdateOptionRequest(t *testing.T, payload map[string]string) updateOptionResponse {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload failed: %v", err)
	}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/option/", bytes.NewReader(body))
	UpdateOption(ctx)

	var resp updateOptionResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response failed: %v, body=%s", err, recorder.Body.String())
	}
	return resp
}

func TestUpdateOptionRejectsDisablingPasswordWhenGitHubLoginDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originDB := model.DB
	model.DB = prepareOptionGuardTestDB(t)
	defer func() { model.DB = originDB }()

	originPasswordLoginEnabled := common.PasswordLoginEnabled
	originGitHubOAuthEnabled := common.GitHubOAuthEnabled
	originGitHubClientID := common.GitHubClientId
	originOptionMap := common.OptionMap
	defer func() {
		common.PasswordLoginEnabled = originPasswordLoginEnabled
		common.GitHubOAuthEnabled = originGitHubOAuthEnabled
		common.GitHubClientId = originGitHubClientID
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originOptionMap
		common.OptionMapRWMutex.Unlock()
	}()

	common.PasswordLoginEnabled = true
	common.GitHubOAuthEnabled = false
	common.GitHubClientId = ""
	common.OptionMapRWMutex.Lock()
	common.OptionMap = map[string]string{
		"PasswordLoginEnabled": "true",
		"GitHubOAuthEnabled":   "false",
	}
	common.OptionMapRWMutex.Unlock()

	resp := performUpdateOptionRequest(t, map[string]string{
		"key":   "PasswordLoginEnabled",
		"value": "false",
	})
	if resp.Success {
		t.Fatalf("expected update to be rejected")
	}
	if !strings.Contains(resp.Message, "至少保留一种登录方式") {
		t.Fatalf("unexpected message: %s", resp.Message)
	}
}

func TestUpdateOptionRejectsDisablingGitHubWhenPasswordLoginDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originDB := model.DB
	model.DB = prepareOptionGuardTestDB(t)
	defer func() { model.DB = originDB }()

	originPasswordLoginEnabled := common.PasswordLoginEnabled
	originGitHubOAuthEnabled := common.GitHubOAuthEnabled
	originGitHubClientID := common.GitHubClientId
	originOptionMap := common.OptionMap
	defer func() {
		common.PasswordLoginEnabled = originPasswordLoginEnabled
		common.GitHubOAuthEnabled = originGitHubOAuthEnabled
		common.GitHubClientId = originGitHubClientID
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originOptionMap
		common.OptionMapRWMutex.Unlock()
	}()

	common.PasswordLoginEnabled = false
	common.GitHubOAuthEnabled = true
	common.GitHubClientId = "test-client-id"
	common.OptionMapRWMutex.Lock()
	common.OptionMap = map[string]string{
		"PasswordLoginEnabled": "false",
		"GitHubOAuthEnabled":   "true",
	}
	common.OptionMapRWMutex.Unlock()

	resp := performUpdateOptionRequest(t, map[string]string{
		"key":   "GitHubOAuthEnabled",
		"value": "false",
	})
	if resp.Success {
		t.Fatalf("expected update to be rejected")
	}
	if !strings.Contains(resp.Message, "至少保留一种登录方式") {
		t.Fatalf("unexpected message: %s", resp.Message)
	}
}

func TestUpdateOptionAllowsRecoveryByEnablingPasswordFromHistoricalInvalidState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originDB := model.DB
	model.DB = prepareOptionGuardTestDB(t)
	defer func() { model.DB = originDB }()

	originPasswordLoginEnabled := common.PasswordLoginEnabled
	originGitHubOAuthEnabled := common.GitHubOAuthEnabled
	originGitHubClientID := common.GitHubClientId
	originOptionMap := common.OptionMap
	defer func() {
		common.PasswordLoginEnabled = originPasswordLoginEnabled
		common.GitHubOAuthEnabled = originGitHubOAuthEnabled
		common.GitHubClientId = originGitHubClientID
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originOptionMap
		common.OptionMapRWMutex.Unlock()
	}()

	common.PasswordLoginEnabled = false
	common.GitHubOAuthEnabled = false
	common.GitHubClientId = ""
	common.OptionMapRWMutex.Lock()
	common.OptionMap = map[string]string{
		"PasswordLoginEnabled": "false",
		"GitHubOAuthEnabled":   "false",
	}
	common.OptionMapRWMutex.Unlock()

	resp := performUpdateOptionRequest(t, map[string]string{
		"key":   "PasswordLoginEnabled",
		"value": "true",
	})
	if !resp.Success {
		t.Fatalf("expected update success, got message: %s", resp.Message)
	}
	if !common.PasswordLoginEnabled {
		t.Fatalf("expected password login to be enabled after recovery")
	}
}

func TestUpdateOptionKeepsGitHubEnableValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originDB := model.DB
	model.DB = prepareOptionGuardTestDB(t)
	defer func() { model.DB = originDB }()

	originPasswordLoginEnabled := common.PasswordLoginEnabled
	originGitHubOAuthEnabled := common.GitHubOAuthEnabled
	originGitHubClientID := common.GitHubClientId
	originOptionMap := common.OptionMap
	defer func() {
		common.PasswordLoginEnabled = originPasswordLoginEnabled
		common.GitHubOAuthEnabled = originGitHubOAuthEnabled
		common.GitHubClientId = originGitHubClientID
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originOptionMap
		common.OptionMapRWMutex.Unlock()
	}()

	common.PasswordLoginEnabled = true
	common.GitHubOAuthEnabled = false
	common.GitHubClientId = ""
	common.OptionMapRWMutex.Lock()
	common.OptionMap = map[string]string{
		"PasswordLoginEnabled": "true",
		"GitHubOAuthEnabled":   "false",
		"GitHubClientId":       "",
	}
	common.OptionMapRWMutex.Unlock()

	resp := performUpdateOptionRequest(t, map[string]string{
		"key":   "GitHubOAuthEnabled",
		"value": "true",
	})
	if resp.Success {
		t.Fatalf("expected update to be rejected")
	}
	if !strings.Contains(resp.Message, "无法启用 GitHub OAuth") {
		t.Fatalf("unexpected message: %s", resp.Message)
	}
}
