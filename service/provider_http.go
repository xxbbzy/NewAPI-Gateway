package service

import (
	"NewAPI-Gateway/common"
	"NewAPI-Gateway/model"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type UpstreamRequestError struct {
	StatusCode int
	Message    string
	Transport  bool
}

func (e *UpstreamRequestError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

var relayHTTPClientCache = struct {
	mu      sync.RWMutex
	clients map[string]*http.Client
}{
	clients: make(map[string]*http.Client),
}

func relayClientCacheKey(provider *model.Provider) string {
	if provider == nil || !provider.ProxyEnabled || strings.TrimSpace(provider.ProxyURL) == "" {
		return "direct"
	}
	return strings.TrimSpace(provider.ProxyURL)
}

func buildProviderTransport(provider *model.Provider) (*http.Transport, error) {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	}
	if provider == nil {
		return transport, nil
	}
	if err := validateProviderProxyForRequest(provider); err != nil {
		return nil, err
	}
	if !provider.ProxyEnabled {
		return transport, nil
	}
	rawProxyURL := strings.TrimSpace(provider.ProxyURL)
	parsedProxyURL, err := url.Parse(rawProxyURL)
	if err != nil || parsedProxyURL == nil || parsedProxyURL.Host == "" || parsedProxyURL.Scheme == "" {
		return nil, fmt.Errorf("invalid provider proxy URL")
	}
	transport.Proxy = http.ProxyURL(parsedProxyURL)
	return transport, nil
}

func NewProviderHTTPClient(provider *model.Provider, timeout time.Duration) (*http.Client, error) {
	transport, err := buildProviderTransport(provider)
	if err != nil {
		return nil, err
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}, nil
}

func getRelayHTTPClientForProvider(provider *model.Provider) (*http.Client, error) {
	key := relayClientCacheKey(provider)
	relayHTTPClientCache.mu.RLock()
	if client, ok := relayHTTPClientCache.clients[key]; ok {
		relayHTTPClientCache.mu.RUnlock()
		return client, nil
	}
	relayHTTPClientCache.mu.RUnlock()

	relayHTTPClientCache.mu.Lock()
	defer relayHTTPClientCache.mu.Unlock()
	if client, ok := relayHTTPClientCache.clients[key]; ok {
		return client, nil
	}
	client, err := NewProviderHTTPClient(provider, 5*time.Minute)
	if err != nil {
		return nil, err
	}
	relayHTTPClientCache.clients[key] = client
	return client, nil
}

func sanitizeProviderErrorMessage(provider *model.Provider, message string) string {
	return model.SanitizeProviderSensitiveText(provider, message)
}

func isProviderReachabilityStatus(statusCode int) bool {
	switch statusCode {
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func classifyProviderReachabilityError(err error) (string, bool) {
	if err == nil {
		return "", false
	}
	var upstreamErr *UpstreamRequestError
	if errors.As(err, &upstreamErr) {
		if upstreamErr.Transport || isProviderReachabilityStatus(upstreamErr.StatusCode) {
			return strings.TrimSpace(upstreamErr.Error()), true
		}
		return "", false
	}
	return "", false
}

func classifyProviderReachabilityProxyError(err *ProxyAttemptError) (string, bool) {
	if err == nil {
		return "", false
	}
	if err.FailureCategory == model.UsageFailureCategoryTransport || isProviderReachabilityStatus(err.StatusCode) {
		return strings.TrimSpace(err.Message), true
	}
	return "", false
}

func markProviderHealthFailure(provider *model.Provider, reason string) {
	if provider == nil {
		return
	}
	sanitized := sanitizeProviderErrorMessage(provider, reason)
	if err := provider.MarkHealthFailure(sanitized); err != nil {
		common.SysLog("mark provider health failure failed: " + err.Error())
	}
}

func markProviderHealthSuccess(provider *model.Provider) {
	if provider == nil {
		return
	}
	if err := provider.MarkHealthSuccess(); err != nil {
		common.SysLog("mark provider health success failed: " + err.Error())
	}
}

func validateProviderProxyForRequest(provider *model.Provider) error {
	if provider == nil {
		return nil
	}
	return model.ValidateProviderProxyConfig(provider.ProxyEnabled, provider.ProxyURL)
}

func sanitizeProxyTransportError(provider *model.Provider, err error) error {
	if err == nil {
		return nil
	}
	return &UpstreamRequestError{
		Message:   sanitizeProviderErrorMessage(provider, err.Error()),
		Transport: true,
	}
}

func sanitizeProxyStatusError(provider *model.Provider, statusCode int, body []byte) error {
	message := fmt.Sprintf("upstream returned status %d: %s", statusCode, sanitizeProviderErrorMessage(provider, string(body)))
	return &UpstreamRequestError{
		StatusCode: statusCode,
		Message:    message,
	}
}

func providerProxyHint(provider *model.Provider) string {
	if provider == nil || !provider.ProxyEnabled {
		return ""
	}
	redacted := model.RedactProxyURL(provider.ProxyURL)
	if redacted == "" {
		return ""
	}
	return " via " + redacted
}

func formatProviderProxyValidationError(provider *model.Provider) error {
	if provider == nil {
		return nil
	}
	if err := validateProviderProxyForRequest(provider); err != nil {
		return &UpstreamRequestError{
			Message:   sanitizeProviderErrorMessage(provider, err.Error()),
			Transport: true,
		}
	}
	return nil
}

func providerRequestLabel(provider *model.Provider) string {
	if provider == nil {
		return "provider"
	}
	if strings.TrimSpace(provider.Name) != "" {
		return provider.Name
	}
	return "provider#" + strconv.Itoa(provider.Id)
}
