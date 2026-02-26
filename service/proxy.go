package service

import (
	"NewAPI-Gateway/common"
	"NewAPI-Gateway/model"
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var proxyHTTPClient = &http.Client{
	Timeout: 5 * time.Minute,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	},
}

type ProxyAttemptError struct {
	StatusCode int
	Message    string
	Retryable  bool
}

func (e *ProxyAttemptError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

// ProxyToUpstream forwards the request once. It writes to client only on success.
func ProxyToUpstream(c *gin.Context, token *model.ProviderToken, provider *model.Provider) *ProxyAttemptError {
	startTime := time.Now()
	requestId := uuid.New().String()[:8]

	// Get user info from context
	aggToken := c.MustGet("agg_token").(*model.AggregatedToken)

	// 1. Read original request body
	bodyBytes, err := getRequestBodyBytes(c)
	if err != nil {
		return &ProxyAttemptError{
			StatusCode: http.StatusBadRequest,
			Message:    "failed to read request body",
			Retryable:  false,
		}
	}

	resolvedModel := strings.TrimSpace(c.GetString("request_model_resolved"))
	if resolvedModel == "" {
		resolvedModel = getContextModelName(c)
	}
	bodyBytes = rewriteRequestModel(bodyBytes, resolvedModel)

	// 2. Construct upstream URL
	upstreamURL := strings.TrimRight(provider.BaseURL, "/") + c.Request.URL.Path
	if c.Request.URL.RawQuery != "" {
		upstreamURL += "?" + c.Request.URL.RawQuery
	}

	// 3. Create upstream request
	req, err := http.NewRequest(c.Request.Method, upstreamURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return &ProxyAttemptError{
			StatusCode: http.StatusInternalServerError,
			Message:    "failed to create upstream request",
			Retryable:  false,
		}
	}

	// 4. Carefully set headers — transparency is KEY
	// Only forward safe headers, remove all proxy-revealing headers
	safeHeaders := []string{
		"Content-Type", "Accept", "Accept-Encoding", "Accept-Language", "User-Agent", "anthropic-beta",
	}
	for _, h := range safeHeaders {
		if v := c.GetHeader(h); v != "" {
			req.Header.Set(h, v)
		}
	}

	// 5. Set authentication — replace ag-token with upstream sk-token
	req.Header.Set("Authorization", "Bearer "+token.SkKey)
	logProxyAuthDebug(c, req, requestId, provider, token)

	// 6. Anthropic compatibility
	if isAnthropicPath(c.Request.URL.Path) {
		req.Header.Set("x-api-key", token.SkKey)
		if v := c.GetHeader("anthropic-version"); v != "" {
			req.Header.Set("anthropic-version", v)
		}
	}

	// 7. Gemini compatibility
	if isGeminiPath(c.Request.URL.Path) {
		req.Header.Set("x-goog-api-key", token.SkKey)
	}

	// 8. REMOVE all proxy-revealing headers
	req.Header.Del("X-Forwarded-For")
	req.Header.Del("X-Forwarded-Host")
	req.Header.Del("X-Forwarded-Proto")
	req.Header.Del("X-Real-IP")
	req.Header.Del("Via")
	req.Header.Del("Forwarded")

	// 9. Send request
	resp, err := proxyHTTPClient.Do(req)
	if err != nil {
		errorMsg := buildErrorMessage(err.Error(), c, bodyBytes)
		logProxyErrorTrace(c, requestId, provider, token, errorMsg)
		logUsage(
			aggToken,
			provider,
			token,
			c,
			requestId,
			usageMetrics{ModelName: getContextModelName(c)},
			false,
			0,
			0,
			errorMsg,
		)
		return &ProxyAttemptError{
			StatusCode: http.StatusBadGateway,
			Message:    "upstream request failed: " + err.Error(),
			Retryable:  true,
		}
	}
	defer resp.Body.Close()

	// 10. Detect if streaming
	contentType := resp.Header.Get("Content-Type")
	isStream := strings.Contains(contentType, "text/event-stream")

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		usage := extractUsageAndModelFromJSON(respBody)
		if usage.ModelName == "" {
			usage.ModelName = getContextModelName(c)
		}
		errorMsg := buildErrorMessage(fmt.Sprintf("upstream status %d: %s", resp.StatusCode, string(respBody)), c, bodyBytes)
		logProxyErrorTrace(c, requestId, provider, token, errorMsg)
		elapsed := time.Since(startTime).Milliseconds()
		logUsage(
			aggToken, provider, token, c, requestId,
			usage, isStream, 0, int(elapsed), errorMsg,
		)
		return &ProxyAttemptError{
			StatusCode: resp.StatusCode,
			Message:    errorMsg,
			Retryable:  true,
		}
	}

	// 11. Copy response headers
	for key, values := range resp.Header {
		lowerKey := strings.ToLower(key)
		// Skip hop-by-hop headers
		if lowerKey == "transfer-encoding" || lowerKey == "connection" {
			continue
		}
		for _, v := range values {
			c.Writer.Header().Add(key, v)
		}
	}

	if isStream {
		// Stream SSE response
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Status(resp.StatusCode)
		flusher, ok := c.Writer.(http.Flusher)
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		streamUsage := usageMetrics{}
		firstTokenMs := 0
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Fprintf(c.Writer, "%s\n", line)
			currentUsage, hasData := extractUsageAndModelFromSSELine(line)
			if hasData && firstTokenMs == 0 {
				firstTokenMs = int(time.Since(startTime).Milliseconds())
			}
			streamUsage = mergeUsageMetrics(streamUsage, currentUsage)
			if ok {
				flusher.Flush()
			}
		}
		errorMsg := ""
		if scanErr := scanner.Err(); scanErr != nil {
			if errorMsg != "" {
				errorMsg += "; scanner error: " + scanErr.Error()
			} else {
				errorMsg = "stream scanner error: " + scanErr.Error()
			}
		}
		if errorMsg != "" {
			errorMsg = buildErrorMessage(errorMsg, c, bodyBytes)
			logProxyErrorTrace(c, requestId, provider, token, errorMsg)
		}
		if streamUsage.ModelName == "" {
			streamUsage.ModelName = getContextModelName(c)
		}
		elapsed := time.Since(startTime).Milliseconds()
		logUsage(
			aggToken, provider, token, c, requestId,
			streamUsage, true, firstTokenMs, int(elapsed), errorMsg,
		)
	} else {
		// Non-streaming response
		c.Status(resp.StatusCode)
		respBody, _ := io.ReadAll(resp.Body)
		c.Writer.Write(respBody)

		elapsed := time.Since(startTime).Milliseconds()
		usage := extractUsageAndModelFromJSON(respBody)
		if usage.ModelName == "" {
			usage.ModelName = getContextModelName(c)
		}
		errorMsg := ""
		if resp.StatusCode >= 400 {
			errorMsg = buildErrorMessage(string(respBody), c, bodyBytes)
			logProxyErrorTrace(c, requestId, provider, token, errorMsg)
		}
		logUsage(
			aggToken, provider, token, c, requestId,
			usage, false, 0, int(elapsed), errorMsg,
		)
	}
	return nil
}

type usageMetrics struct {
	ModelName             string
	PromptTokens          int
	CompletionTokens      int
	CacheTokens           int
	CacheCreationTokens   int
	CacheCreation5mTokens int
	CacheCreation1hTokens int
	CostUSD               float64
	UsageSource           string
	UsageParser           string
}

func mergeUsageMetrics(base usageMetrics, current usageMetrics) usageMetrics {
	if current.PromptTokens > base.PromptTokens {
		base.PromptTokens = current.PromptTokens
	}
	if current.CompletionTokens > base.CompletionTokens {
		base.CompletionTokens = current.CompletionTokens
	}
	if current.CacheTokens > base.CacheTokens {
		base.CacheTokens = current.CacheTokens
	}
	if current.CacheCreationTokens > base.CacheCreationTokens {
		base.CacheCreationTokens = current.CacheCreationTokens
	}
	if current.CacheCreation5mTokens > base.CacheCreation5mTokens {
		base.CacheCreation5mTokens = current.CacheCreation5mTokens
	}
	if current.CacheCreation1hTokens > base.CacheCreation1hTokens {
		base.CacheCreation1hTokens = current.CacheCreation1hTokens
	}
	if current.CostUSD > base.CostUSD {
		base.CostUSD = current.CostUSD
	}
	if current.ModelName != "" {
		base.ModelName = current.ModelName
	}
	if current.UsageSource == model.UsageSourceExact {
		base.UsageSource = model.UsageSourceExact
	}
	if current.UsageParser != "" && current.UsageParser != "none" {
		base.UsageParser = current.UsageParser
	}
	return base
}

func rewriteRequestModel(body []byte, targetModel string) []byte {
	targetModel = strings.TrimSpace(targetModel)
	if targetModel == "" || len(body) == 0 {
		return body
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return body
	}

	currentModel, ok := payload["model"].(string)
	if !ok || strings.TrimSpace(currentModel) == "" || currentModel == targetModel {
		return body
	}

	payload["model"] = targetModel
	updatedBody, err := json.Marshal(payload)
	if err != nil {
		return body
	}
	return updatedBody
}

func getRequestBodyBytes(c *gin.Context) ([]byte, error) {
	if cached, ok := c.Get("proxy_request_body"); ok {
		if bodyBytes, ok := cached.([]byte); ok {
			return bodyBytes, nil
		}
	}
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil, err
	}
	c.Set("proxy_request_body", bodyBytes)
	c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	return bodyBytes, nil
}

func logUsage(aggToken *model.AggregatedToken, provider *model.Provider, token *model.ProviderToken,
	c *gin.Context, requestId string, usage usageMetrics, isStream bool, firstTokenMs int,
	responseTimeMs int, errorMsg string) {

	status := 1
	if errorMsg != "" {
		status = 0
	}

	// Try to extract model from request path or body
	if usage.ModelName == "" {
		usage.ModelName = getContextModelName(c)
	}
	if strings.TrimSpace(usage.UsageSource) == "" {
		usage.UsageSource = model.UsageSourceMissing
	}
	if strings.TrimSpace(usage.UsageParser) == "" {
		usage.UsageParser = "none"
	}
	if usage.CostUSD <= 0 {
		estimated := estimateUsageCostUSD(
			provider.Id,
			usage.ModelName,
			usage.PromptTokens,
			usage.CompletionTokens,
		)
		usage.CostUSD = estimated
		if estimated > 0 && usage.UsageSource == model.UsageSourceMissing {
			usage.UsageSource = model.UsageSourceEstimated
			usage.UsageParser = "cost-estimator"
		}
	}
	relayRequestId := getRelayRequestID(c, requestId)
	attemptIndex := getRelayAttemptIndex(c)

	log := &model.UsageLog{
		UserId:                aggToken.UserId,
		AggregatedTokenId:     aggToken.Id,
		ProviderId:            provider.Id,
		ProviderName:          provider.Name,
		ProviderTokenId:       token.Id,
		ModelName:             usage.ModelName,
		PromptTokens:          usage.PromptTokens,
		CompletionTokens:      usage.CompletionTokens,
		CacheTokens:           usage.CacheTokens,
		CacheCreationTokens:   usage.CacheCreationTokens,
		CacheCreation5mTokens: usage.CacheCreation5mTokens,
		CacheCreation1hTokens: usage.CacheCreation1hTokens,
		ResponseTimeMs:        responseTimeMs,
		FirstTokenMs:          firstTokenMs,
		IsStream:              isStream,
		CostUSD:               usage.CostUSD,
		Status:                status,
		ErrorMessage:          errorMsg,
		ClientIp:              c.ClientIP(),
		RequestId:             requestId,
		RelayRequestId:        relayRequestId,
		AttemptIndex:          attemptIndex,
		UsageSource:           usage.UsageSource,
		UsageParser:           usage.UsageParser,
	}
	go func() {
		if err := log.Insert(); err != nil {
			common.SysLog(fmt.Sprintf("failed to insert usage log: %v", err))
		}
	}()
}

func getRelayRequestID(c *gin.Context, requestId string) string {
	relayRequestID := strings.TrimSpace(c.GetString("relay_request_id"))
	if relayRequestID == "" {
		return "legacy-" + requestId
	}
	return relayRequestID
}

func getRelayAttemptIndex(c *gin.Context) int {
	if raw, ok := c.Get("relay_attempt_index"); ok {
		switch v := raw.(type) {
		case int:
			if v > 0 {
				return v
			}
		case int64:
			if v > 0 {
				return int(v)
			}
		case float64:
			if v > 0 {
				return int(v)
			}
		}
	}
	return 1
}

func isAnthropicPath(path string) bool {
	return strings.Contains(path, "/v1/messages")
}

func isGeminiPath(path string) bool {
	return strings.Contains(path, "/v1beta/")
}

func logProxyAuthDebug(c *gin.Context, req *http.Request, requestId string, provider *model.Provider, token *model.ProviderToken) {
	// Enable only when explicitly requested to avoid noisy/sensitive logs.
	if strings.ToLower(os.Getenv("DEBUG_PROXY_AUTH")) != "1" {
		return
	}

	incomingAuth := c.GetHeader("Authorization")
	if incomingAuth == "" {
		incomingAuth = c.GetHeader("x-api-key")
	}
	if incomingAuth == "" {
		incomingAuth = c.GetHeader("x-goog-api-key")
	}
	if incomingAuth == "" {
		incomingAuth = c.Query("key")
	}

	outgoingAuth := req.Header.Get("Authorization")
	common.SysLog(fmt.Sprintf(
		"[proxy-auth] request_id=%s provider=%s provider_id=%d provider_token_id=%d incoming=%s outgoing=%s",
		requestId,
		provider.Name,
		provider.Id,
		token.Id,
		tokenSummary(incomingAuth),
		tokenSummary(outgoingAuth),
	))
}

func tokenSummary(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return "(empty)"
	}

	value := strings.TrimSpace(raw)
	prefix := ""
	if strings.HasPrefix(strings.ToLower(value), "bearer ") {
		prefix = "Bearer "
		value = strings.TrimSpace(value[7:])
	}

	if value == "" {
		return prefix + "(empty)"
	}

	masked := value
	if len(value) > 8 {
		masked = value[:4] + "..." + value[len(value)-4:]
	}
	hash := sha256.Sum256([]byte(value))
	fp := hex.EncodeToString(hash[:6])
	return prefix + masked + "(sha256:" + fp + ")"
}

func buildErrorMessage(base string, c *gin.Context, bodyBytes []byte) string {
	msg := strings.TrimSpace(base)
	requestBodyLog := requestBodyForErrorLog(c, bodyBytes)
	if requestBodyLog != "" {
		if msg == "" {
			msg = "request body: " + requestBodyLog
		} else {
			msg += "\nrequest body: " + requestBodyLog
		}
	}
	const maxErrorMessageLen = 20000
	if len(msg) > maxErrorMessageLen {
		msg = msg[:maxErrorMessageLen] + "...(truncated)"
	}
	return msg
}

func requestBodyForErrorLog(c *gin.Context, bodyBytes []byte) string {
	if len(bodyBytes) == 0 {
		return "(empty)"
	}
	contentType := strings.ToLower(strings.TrimSpace(c.GetHeader("Content-Type")))
	if strings.Contains(contentType, "application/json") {
		return strings.TrimSpace(string(bodyBytes))
	}
	return fmt.Sprintf("(non-json omitted) content_type=%s body_size=%d", contentType, len(bodyBytes))
}

func logProxyErrorTrace(c *gin.Context, requestId string, provider *model.Provider, token *model.ProviderToken, errorMsg string) {
	compactError := strings.ReplaceAll(strings.ReplaceAll(errorMsg, "\n", " "), "\r", " ")
	if len(compactError) > 1200 {
		compactError = compactError[:1200] + "...(truncated)"
	}
	common.SysError(fmt.Sprintf(
		"[proxy-error] request_id=%s method=%s path=%s provider=%s provider_id=%d provider_token_id=%d model=%s model_original=%s model_canonical=%s model_resolved=%s client_ip=%s detail=%s",
		requestId,
		c.Request.Method,
		c.Request.URL.Path,
		provider.Name,
		provider.Id,
		token.Id,
		getContextModelName(c),
		c.GetString("request_model_original"),
		c.GetString("request_model_canonical"),
		c.GetString("request_model_resolved"),
		c.ClientIP(),
		compactError,
	))
}

func getContextModelName(c *gin.Context) string {
	modelName := strings.TrimSpace(c.GetString("request_model_resolved"))
	if modelName != "" {
		return modelName
	}
	modelName = strings.TrimSpace(c.GetString("request_model"))
	if modelName != "" {
		return modelName
	}
	modelName = strings.TrimSpace(c.GetString("request_model_canonical"))
	if modelName != "" {
		return modelName
	}
	modelName = strings.TrimSpace(c.GetString("request_model_original"))
	return modelName
}

func extractUsageAndModelFromSSELine(line string) (usageMetrics, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || !strings.HasPrefix(trimmed, "data:") {
		return usageMetrics{}, false
	}
	payload := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
	if payload == "" || payload == "[DONE]" {
		return usageMetrics{}, false
	}
	return extractUsageAndModelFromJSON([]byte(payload)), true
}

type usageExtractor struct {
	parser  string
	extract func(payload map[string]interface{}) (map[string]interface{}, string, bool)
}

var usageExtractors = []usageExtractor{
	{parser: "usage-map", extract: extractTopLevelUsageMap},
	{parser: "message-usage-map", extract: extractMessageUsageMap},
	{parser: "data-usage-map", extract: extractDataUsageMap},
	{parser: "response-usage-map", extract: extractResponseUsageMap},
	{parser: "result-usage-map", extract: extractResultUsageMap},
	{parser: "choices-usage-map", extract: extractChoicesUsageMap},
	{parser: "token-usage-map", extract: extractTopLevelTokenUsageMap},
	{parser: "top-level-usage-fields", extract: extractTopLevelUsageFields},
	{parser: "recursive-usage-map", extract: extractRecursiveUsageMap},
}

func extractUsageAndModelFromJSON(body []byte) usageMetrics {
	if len(body) == 0 {
		return usageMetrics{UsageSource: model.UsageSourceMissing, UsageParser: "none"}
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return usageMetrics{UsageSource: model.UsageSourceMissing, UsageParser: "none"}
	}

	out := usageMetrics{}
	if modelName := getStringValue(payload["model"]); modelName != "" {
		out.ModelName = modelName
	}
	out.UsageSource = model.UsageSourceMissing
	out.UsageParser = "none"

	for _, extractor := range usageExtractors {
		usage, modelName, ok := extractor.extract(payload)
		if !ok {
			continue
		}
		parsed, hasUsage := buildUsageMetricsFromMap(usage)
		if !hasUsage {
			continue
		}
		if parsed.ModelName == "" && modelName != "" {
			parsed.ModelName = modelName
		}
		if parsed.ModelName == "" {
			parsed.ModelName = out.ModelName
		}
		parsed.UsageSource = model.UsageSourceExact
		parsed.UsageParser = extractor.parser
		return parsed
	}
	return out
}

func buildUsageMetricsFromMap(usage map[string]interface{}) (usageMetrics, bool) {
	if usage == nil || !hasAnyUsageSignal(usage) {
		return usageMetrics{}, false
	}
	out := usageMetrics{}

	if value, ok := getIntByPaths(usage, []string{"prompt_tokens"}, []string{"input_tokens"}, []string{"prompt_token_count"}, []string{"input_token_count"}, []string{"inputTokenCount"}); ok {
		out.PromptTokens = value
	}
	if value, ok := getIntByPaths(usage, []string{"completion_tokens"}, []string{"output_tokens"}, []string{"completion_token_count"}, []string{"output_token_count"}, []string{"outputTokenCount"}); ok {
		out.CompletionTokens = value
	}
	if out.PromptTokens == 0 && out.CompletionTokens == 0 {
		if value, ok := getIntByPaths(usage, []string{"total_tokens"}, []string{"total_token_count"}, []string{"totalTokenCount"}); ok {
			out.PromptTokens = value
		}
	}

	if value, ok := getIntByPaths(
		usage,
		[]string{"cached_tokens"},
		[]string{"prompt_tokens_details", "cached_tokens"},
		[]string{"input_tokens_details", "cached_tokens"},
		[]string{"prompt_cache_hit_tokens"},
		[]string{"cache_read_input_tokens"},
		[]string{"token_usage", "cached_tokens"},
	); ok {
		out.CacheTokens = value
	}

	if value, ok := getIntByPaths(
		usage,
		[]string{"cache_creation_tokens"},
		[]string{"prompt_tokens_details", "cached_creation_tokens"},
		[]string{"cache_creation_input_tokens"},
		[]string{"token_usage", "cache_creation_tokens"},
	); ok {
		out.CacheCreationTokens = value
	}
	if value, ok := getIntByPaths(
		usage,
		[]string{"cache_creation_5m_tokens"},
		[]string{"claude_cache_creation_5_m_tokens"},
		[]string{"cache_creation", "ephemeral_5m_input_tokens"},
	); ok {
		out.CacheCreation5mTokens = value
	}
	if value, ok := getIntByPaths(
		usage,
		[]string{"cache_creation_1h_tokens"},
		[]string{"claude_cache_creation_1_h_tokens"},
		[]string{"cache_creation", "ephemeral_1h_input_tokens"},
	); ok {
		out.CacheCreation1hTokens = value
	}
	cacheCreationSum := out.CacheCreation5mTokens + out.CacheCreation1hTokens
	out.CacheCreationTokens = maxInt(out.CacheCreationTokens, cacheCreationSum)

	if value, ok := getFloatByPaths(usage, []string{"cost"}, []string{"total_cost"}, []string{"token_usage", "cost"}); ok {
		out.CostUSD = value
	}
	return out, true
}

func hasAnyUsageSignal(usage map[string]interface{}) bool {
	paths := [][]string{
		{"prompt_tokens"}, {"input_tokens"}, {"prompt_token_count"}, {"input_token_count"}, {"inputTokenCount"},
		{"completion_tokens"}, {"output_tokens"}, {"completion_token_count"}, {"output_token_count"}, {"outputTokenCount"},
		{"total_tokens"}, {"total_token_count"}, {"totalTokenCount"},
		{"cached_tokens"}, {"prompt_tokens_details", "cached_tokens"}, {"input_tokens_details", "cached_tokens"},
		{"prompt_cache_hit_tokens"}, {"cache_read_input_tokens"},
		{"cache_creation_tokens"}, {"prompt_tokens_details", "cached_creation_tokens"}, {"cache_creation_input_tokens"},
		{"cache_creation_5m_tokens"}, {"claude_cache_creation_5_m_tokens"}, {"cache_creation", "ephemeral_5m_input_tokens"},
		{"cache_creation_1h_tokens"}, {"claude_cache_creation_1_h_tokens"}, {"cache_creation", "ephemeral_1h_input_tokens"},
		{"cost"}, {"total_cost"},
	}
	for _, path := range paths {
		if _, ok := lookupValueByPath(usage, path); ok {
			return true
		}
	}
	return false
}

func getIntByPaths(parent map[string]interface{}, paths ...[]string) (int, bool) {
	for _, path := range paths {
		if value, ok := lookupValueByPath(parent, path); ok {
			return getIntValue(value), true
		}
	}
	return 0, false
}

func getFloatByPaths(parent map[string]interface{}, paths ...[]string) (float64, bool) {
	for _, path := range paths {
		if value, ok := lookupValueByPath(parent, path); ok {
			return getFloatValue(value), true
		}
	}
	return 0, false
}

func lookupValueByPath(parent map[string]interface{}, path []string) (interface{}, bool) {
	if parent == nil || len(path) == 0 {
		return nil, false
	}
	var current interface{} = parent
	for _, key := range path {
		nextMap, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		nextValue, ok := nextMap[key]
		if !ok {
			return nil, false
		}
		current = nextValue
	}
	return current, true
}

func extractTopLevelUsageMap(payload map[string]interface{}) (map[string]interface{}, string, bool) {
	usageRaw, ok := payload["usage"]
	if !ok {
		return nil, "", false
	}
	usage, ok := usageRaw.(map[string]interface{})
	if !ok {
		return nil, "", false
	}
	return usage, getStringValue(payload["model"]), true
}

func extractMessageUsageMap(payload map[string]interface{}) (map[string]interface{}, string, bool) {
	messageRaw, ok := payload["message"]
	if !ok {
		return nil, "", false
	}
	message, ok := messageRaw.(map[string]interface{})
	if !ok {
		return nil, "", false
	}
	usageRaw, ok := message["usage"]
	if !ok {
		return nil, getStringValue(message["model"]), false
	}
	usage, ok := usageRaw.(map[string]interface{})
	if !ok {
		return nil, getStringValue(message["model"]), false
	}
	return usage, getStringValue(message["model"]), true
}

func extractDataUsageMap(payload map[string]interface{}) (map[string]interface{}, string, bool) {
	return extractNestedUsageMap(payload, "data")
}

func extractResponseUsageMap(payload map[string]interface{}) (map[string]interface{}, string, bool) {
	return extractNestedUsageMap(payload, "response")
}

func extractResultUsageMap(payload map[string]interface{}) (map[string]interface{}, string, bool) {
	return extractNestedUsageMap(payload, "result")
}

func extractNestedUsageMap(payload map[string]interface{}, key string) (map[string]interface{}, string, bool) {
	nestedRaw, ok := payload[key]
	if !ok {
		return nil, "", false
	}
	nested, ok := nestedRaw.(map[string]interface{})
	if !ok {
		return nil, "", false
	}
	if usageRaw, ok := nested["usage"]; ok {
		if usage, ok := usageRaw.(map[string]interface{}); ok {
			modelName := getStringValue(nested["model"])
			if modelName == "" {
				modelName = getStringValue(payload["model"])
			}
			return usage, modelName, true
		}
	}
	return nil, "", false
}

func extractChoicesUsageMap(payload map[string]interface{}) (map[string]interface{}, string, bool) {
	choicesRaw, ok := payload["choices"]
	if !ok {
		return nil, "", false
	}
	choices, ok := choicesRaw.([]interface{})
	if !ok {
		return nil, "", false
	}
	for _, item := range choices {
		choice, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		usageRaw, ok := choice["usage"]
		if !ok {
			continue
		}
		usage, ok := usageRaw.(map[string]interface{})
		if !ok {
			continue
		}
		modelName := getStringValue(choice["model"])
		if modelName == "" {
			modelName = getStringValue(payload["model"])
		}
		return usage, modelName, true
	}
	return nil, "", false
}

func extractTopLevelTokenUsageMap(payload map[string]interface{}) (map[string]interface{}, string, bool) {
	raw, ok := payload["token_usage"]
	if !ok {
		return nil, "", false
	}
	usage, ok := raw.(map[string]interface{})
	if !ok {
		return nil, "", false
	}
	return usage, getStringValue(payload["model"]), true
}

func extractTopLevelUsageFields(payload map[string]interface{}) (map[string]interface{}, string, bool) {
	if !hasAnyUsageSignal(payload) {
		return nil, "", false
	}
	return payload, getStringValue(payload["model"]), true
}

func extractRecursiveUsageMap(payload map[string]interface{}) (map[string]interface{}, string, bool) {
	if usage, ok := findUsageMapRecursive(payload, 0); ok {
		return usage, getStringValue(payload["model"]), true
	}
	return nil, "", false
}

func findUsageMapRecursive(node interface{}, depth int) (map[string]interface{}, bool) {
	if depth > 6 || node == nil {
		return nil, false
	}
	switch current := node.(type) {
	case map[string]interface{}:
		if hasAnyUsageSignal(current) {
			return current, true
		}
		for _, value := range current {
			if nested, ok := findUsageMapRecursive(value, depth+1); ok {
				return nested, true
			}
		}
	case []interface{}:
		for _, value := range current {
			if nested, ok := findUsageMapRecursive(value, depth+1); ok {
				return nested, true
			}
		}
	}
	return nil, false
}

func getStringValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	default:
		return ""
	}
}

func getIntValue(value interface{}) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		v = strings.TrimSpace(v)
		if v == "" {
			return 0
		}
		var parsed int
		_, err := fmt.Sscanf(v, "%d", &parsed)
		if err != nil {
			return 0
		}
		return parsed
	default:
		return 0
	}
}

func getFloatValue(value interface{}) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		v = strings.TrimSpace(v)
		if v == "" {
			return 0
		}
		var parsed float64
		_, err := fmt.Sscanf(v, "%f", &parsed)
		if err != nil {
			return 0
		}
		return parsed
	default:
		return 0
	}
}

func getIntFromMap(parent map[string]interface{}, key string, subKey string) int {
	nestedRaw, ok := parent[key]
	if !ok {
		return 0
	}
	nested, ok := nestedRaw.(map[string]interface{})
	if !ok {
		return 0
	}
	return getIntValue(nested[subKey])
}

func extractUsageMap(payload map[string]interface{}) (map[string]interface{}, string) {
	if payload == nil {
		return nil, ""
	}
	if usageRaw, ok := payload["usage"]; ok {
		if usage, ok := usageRaw.(map[string]interface{}); ok {
			return usage, getStringValue(payload["model"])
		}
	}
	messageRaw, ok := payload["message"]
	if !ok {
		return nil, ""
	}
	message, ok := messageRaw.(map[string]interface{})
	if !ok {
		return nil, ""
	}
	usageRaw, ok := message["usage"]
	if !ok {
		return nil, getStringValue(message["model"])
	}
	usage, ok := usageRaw.(map[string]interface{})
	if !ok {
		return nil, getStringValue(message["model"])
	}
	return usage, getStringValue(message["model"])
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func estimateUsageCostUSD(providerId int, modelName string, promptTokens int, completionTokens int) float64 {
	if modelName == "" {
		return 0
	}
	pricing, err := model.GetModelPricingByProviderAndModel(providerId, modelName)
	if err != nil || pricing == nil {
		return 0
	}
	if pricing.ModelPrice > 0 && promptTokens == 0 && completionTokens == 0 {
		return pricing.ModelPrice
	}
	modelRatio := pricing.ModelRatio
	if modelRatio <= 0 {
		return 0
	}
	completionRatio := pricing.CompletionRatio
	if completionRatio <= 0 {
		completionRatio = 1
	}
	inputCost := float64(promptTokens) * modelRatio / 500000.0
	outputCost := float64(completionTokens) * modelRatio * completionRatio / 500000.0
	return inputCost + outputCost
}
