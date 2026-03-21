package service

import (
	"NewAPI-Gateway/model"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// UpstreamClient wraps HTTP calls to an upstream NewAPI instance
type UpstreamClient struct {
	BaseURL     string
	AccessToken string
	UserID      int
	Provider    *model.Provider
	HTTPClient  *http.Client
}

func NewUpstreamClient(baseURL string, accessToken string, userID int) *UpstreamClient {
	return &UpstreamClient{
		BaseURL:     baseURL,
		AccessToken: accessToken,
		UserID:      userID,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func NewUpstreamClientForProvider(provider *model.Provider) (*UpstreamClient, error) {
	if provider == nil {
		return NewUpstreamClient("", "", 0), nil
	}
	client := NewUpstreamClient(provider.BaseURL, provider.AccessToken, provider.UserID)
	client.Provider = provider
	httpClient, err := NewProviderHTTPClient(provider, 30*time.Second)
	if err != nil {
		return nil, sanitizeProxyTransportError(provider, err)
	}
	client.HTTPClient = httpClient
	return client, nil
}

// UpstreamResponse is the standard NewAPI response wrapper
type UpstreamResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// UpstreamPricing mirrors the upstream Pricing structure
type UpstreamPricing struct {
	ModelName              string   `json:"model_name"`
	QuotaType              int      `json:"quota_type"`
	ModelRatio             float64  `json:"model_ratio"`
	ModelPrice             float64  `json:"model_price"`
	CompletionRatio        float64  `json:"completion_ratio"`
	EnableGroups           []string `json:"enable_groups"`
	SupportedEndpointTypes []string `json:"supported_endpoint_types"`
}

type UpstreamEndpointInfo struct {
	Path   string `json:"path"`
	Method string `json:"method"`
}

type UpstreamPricingPayload struct {
	Data              []UpstreamPricing               `json:"data"`
	GroupRatio        map[string]float64              `json:"group_ratio"`
	UsableGroup       map[string]string               `json:"usable_group"`
	SupportedEndpoint map[string]UpstreamEndpointInfo `json:"supported_endpoint"`
}

// UpstreamToken mirrors the upstream Token structure
type UpstreamToken struct {
	Id                 int    `json:"id"`
	Key                string `json:"key"`
	Name               string `json:"name"`
	Status             int    `json:"status"`
	Group              string `json:"group"`
	RemainQuota        int64  `json:"remain_quota"`
	UnlimitedQuota     bool   `json:"unlimited_quota"`
	UsedQuota          int64  `json:"used_quota"`
	ModelLimitsEnabled bool   `json:"model_limits_enabled"`
	ModelLimits        string `json:"model_limits"`
}

type UpstreamTokenPage struct {
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
	Total    int             `json:"total"`
	Items    []UpstreamToken `json:"items"`
}

// UpstreamUserSelf mirrors partial user/self response
type UpstreamUserSelf struct {
	Id      int   `json:"id"`
	Balance int64 `json:"quota"`
	Status  int   `json:"status"`
}

// CheckinResponse for the checkin endpoint
type CheckinResponse struct {
	QuotaAwarded int64  `json:"quota_awarded"`
	Message      string `json:"-"`
}

var idempotentCheckinMessagePatterns = []string{
	"今日已签到",
	"alreadycheckedintoday",
}

func normalizeCheckinMessage(message string) string {
	compact := strings.Join(strings.Fields(strings.TrimSpace(message)), "")
	return strings.ToLower(compact)
}

func isIdempotentCheckinMessage(message string) bool {
	normalized := normalizeCheckinMessage(message)
	if normalized == "" {
		return false
	}
	for _, pattern := range idempotentCheckinMessagePatterns {
		if strings.Contains(normalized, pattern) {
			return true
		}
	}
	return false
}

func (c *UpstreamClient) doRequest(method string, path string) ([]byte, error) {
	url := c.BaseURL + path
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.AccessToken)
	req.Header.Set("New-Api-User", strconv.Itoa(c.UserID))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, &UpstreamRequestError{
			Message:   sanitizeProviderErrorMessage(c.Provider, err.Error()),
			Transport: true,
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &UpstreamRequestError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("upstream returned status %d: %s", resp.StatusCode, sanitizeProviderErrorMessage(c.Provider, string(body))),
		}
	}

	return body, nil
}

// doRequestWithBody performs an HTTP request with a JSON body
func (c *UpstreamClient) doRequestWithBody(method string, path string, payload interface{}) ([]byte, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	url := c.BaseURL + path
	req, err := http.NewRequest(method, url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.AccessToken)
	req.Header.Set("New-Api-User", strconv.Itoa(c.UserID))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, &UpstreamRequestError{
			Message:   sanitizeProviderErrorMessage(c.Provider, err.Error()),
			Transport: true,
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &UpstreamRequestError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("upstream returned status %d: %s", resp.StatusCode, sanitizeProviderErrorMessage(c.Provider, string(body))),
		}
	}
	return body, nil
}

// GetPricing fetches /api/pricing from the upstream.
// It is compatible with both:
// 1) {data:[...]}
// 2) {success:true,data:[...],group_ratio:{...},usable_group:{...}}
func (c *UpstreamClient) GetPricing() (*UpstreamPricingPayload, error) {
	body, err := c.doRequest("GET", "/api/pricing")
	if err != nil {
		return nil, err
	}
	var resp UpstreamPricingPayload
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if resp.GroupRatio == nil {
		resp.GroupRatio = make(map[string]float64)
	}
	if resp.UsableGroup == nil {
		resp.UsableGroup = make(map[string]string)
	}
	if resp.SupportedEndpoint == nil {
		resp.SupportedEndpoint = make(map[string]UpstreamEndpointInfo)
	}
	return &resp, nil
}

// GetTokens fetches /api/token/ from the upstream
func (c *UpstreamClient) GetTokens(page int, pageSize int) (*UpstreamTokenPage, error) {
	path := fmt.Sprintf("/api/token/?p=%d&page_size=%d", page, pageSize)
	body, err := c.doRequest("GET", path)
	if err != nil {
		return nil, err
	}
	var resp UpstreamResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("upstream token list failed: %s", resp.Message)
	}
	// Upstream returns a paginated object: {page, page_size, total, items: [...tokens]}
	var pageInfo UpstreamTokenPage
	if err := json.Unmarshal(resp.Data, &pageInfo); err != nil {
		return nil, fmt.Errorf("failed to parse paginated token response: %w", err)
	}
	if pageInfo.PageSize <= 0 {
		pageInfo.PageSize = pageSize
	}
	if pageInfo.Page < 0 {
		pageInfo.Page = page
	}
	if pageInfo.Total < 0 {
		pageInfo.Total = 0
	}
	if pageInfo.Items == nil {
		pageInfo.Items = make([]UpstreamToken, 0)
	}
	return &pageInfo, nil
}

// GetTokenKey fetches the full unmasked key for a token via GET /api/token/:id/key
func (c *UpstreamClient) GetTokenKey(tokenId int) (string, error) {
	path := fmt.Sprintf("/api/token/%d/key", tokenId)
	body, err := c.doRequest("GET", path)
	if err != nil {
		return "", err
	}
	var resp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Data    string `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", err
	}
	if !resp.Success {
		return "", fmt.Errorf("upstream get token key failed: %s", resp.Message)
	}
	return resp.Data, nil
}

// CreateUpstreamToken calls upstream POST /api/token/ to create a new token
func (c *UpstreamClient) CreateUpstreamToken(name string, group string, unlimitedQuota bool, remainQuota int64, modelLimits string) error {
	payload := map[string]interface{}{
		"name":                 name,
		"group":                group,
		"unlimited_quota":      unlimitedQuota,
		"remain_quota":         remainQuota,
		"expired_time":         -1,
		"model_limits_enabled": modelLimits != "",
		"model_limits":         modelLimits,
	}
	body, err := c.doRequestWithBody("POST", "/api/token/", payload)
	if err != nil {
		return err
	}
	var resp UpstreamResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("upstream create token failed: %s", resp.Message)
	}
	return nil
}

// DeleteUpstreamToken calls upstream DELETE /api/token/:id to remove a token.
// Some upstream deployments accept trailing slash variants, so we try both.
func (c *UpstreamClient) DeleteUpstreamToken(tokenId int) error {
	paths := []string{
		fmt.Sprintf("/api/token/%d", tokenId),
		fmt.Sprintf("/api/token/%d/", tokenId),
	}
	var lastErr error
	for _, path := range paths {
		body, err := c.doRequest("DELETE", path)
		if err != nil {
			lastErr = err
			continue
		}
		var resp UpstreamResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return err
		}
		if !resp.Success {
			return fmt.Errorf("upstream delete token failed: %s", resp.Message)
		}
		return nil
	}
	return lastErr
}

// GetUserSelf fetches /api/user/self from the upstream
func (c *UpstreamClient) GetUserSelf() (*UpstreamUserSelf, error) {
	body, err := c.doRequest("GET", "/api/user/self")
	if err != nil {
		return nil, err
	}
	var resp struct {
		Success bool             `json:"success"`
		Data    UpstreamUserSelf `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// Checkin performs /api/user/checkin on the upstream
func (c *UpstreamClient) Checkin() (*CheckinResponse, error) {
	url := c.BaseURL + "/api/user/checkin"
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.AccessToken)
	req.Header.Set("New-Api-User", strconv.Itoa(c.UserID))

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, &UpstreamRequestError{
			Message:   sanitizeProviderErrorMessage(c.Provider, err.Error()),
			Transport: true,
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Success bool            `json:"success"`
		Message string          `json:"message"`
		Data    CheckinResponse `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if !result.Success {
		message := strings.TrimSpace(result.Message)
		if isIdempotentCheckinMessage(message) {
			result.Data.Message = message
			return &result.Data, nil
		}
		return nil, fmt.Errorf("checkin failed: %s", result.Message)
	}
	if strings.TrimSpace(result.Data.Message) == "" {
		result.Data.Message = strings.TrimSpace(result.Message)
	}
	return &result.Data, nil
}
