package controller

import (
	"NewAPI-Gateway/common"
	"NewAPI-Gateway/model"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGetOptionsOmitsSensitiveValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originMap := common.OptionMap
	common.OptionMapRWMutex.Lock()
	common.OptionMap = map[string]string{
		model.BackupWebDAVURLOptionKey:         "https://dav.example.com",
		model.BackupWebDAVPasswordOptionKey:    "secret-password",
		model.BackupEncryptPassphraseOptionKey: "secret-passphrase",
		"PasswordLoginEnabled":                 "true",
		"PasswordRegisterEnabled":              "false",
		"SomeApiToken":                         "token-value",
		"GitHubClientSecret":                   "secret-value",
		"PlainVisibleOption":                   "visible",
	}
	common.OptionMapRWMutex.Unlock()
	defer func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originMap
		common.OptionMapRWMutex.Unlock()
	}()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/option", nil)
	GetOptions(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	var response struct {
		Success bool            `json:"success"`
		Data    []*model.Option `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if !response.Success {
		t.Fatalf("expected success=true")
	}
	keys := make(map[string]bool, len(response.Data))
	for _, item := range response.Data {
		keys[item.Key] = true
	}

	if keys[model.BackupWebDAVPasswordOptionKey] {
		t.Fatalf("backup webdav password should be omitted from options response")
	}
	if keys[model.BackupEncryptPassphraseOptionKey] {
		t.Fatalf("backup encrypt passphrase should be omitted from options response")
	}
	if keys["SomeApiToken"] {
		t.Fatalf("token-like key should be omitted from options response")
	}
	if keys["GitHubClientSecret"] {
		t.Fatalf("secret-like key should be omitted from options response")
	}
	if !keys[model.BackupWebDAVURLOptionKey] || !keys["PlainVisibleOption"] || !keys["PasswordLoginEnabled"] || !keys["PasswordRegisterEnabled"] {
		t.Fatalf("non-sensitive options should be returned")
	}
}
