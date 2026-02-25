package controller

import (
	"NewAPI-Gateway/model"
	"NewAPI-Gateway/service"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Relay is the main proxy handler for all OpenAI-compatible API calls
func Relay(c *gin.Context) {
	aggToken := c.MustGet("agg_token").(*model.AggregatedToken)

	// 1. Extract and resolve model identity.
	requestedModel := extractRequestedModel(c)
	if requestedModel == "" {
		requestedModel = "unknown"
	}
	canonicalModel := requestedModel
	if resolvedCanonical, ok, err := model.ResolveCanonicalModelName(requestedModel); err == nil && ok {
		canonicalModel = resolvedCanonical
	}
	c.Set("request_model_original", requestedModel)
	c.Set("request_model_canonical", canonicalModel)
	c.Set("request_model", canonicalModel)

	// 2. Check model whitelist
	if !aggToken.IsModelAllowed(canonicalModel) {
		c.JSON(http.StatusForbidden, gin.H{
			"error": gin.H{
				"message": "model not allowed: " + requestedModel,
				"type":    "permission_error",
				"code":    "model_not_allowed",
			},
		})
		return
	}

	// 3. Build retry plan: retry all routes within one priority first, then downgrade.
	planModel := canonicalModel
	plan, err := model.BuildRouteAttemptsByPriority(planModel)
	if err != nil && !strings.EqualFold(planModel, requestedModel) {
		planModel = requestedModel
		plan, err = model.BuildRouteAttemptsByPriority(planModel)
	}
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": gin.H{
				"message": "no available provider for model: " + requestedModel,
				"type":    "server_error",
				"code":    "service_unavailable",
			},
		})
		return
	}

	var lastErr *service.ProxyAttemptError
	for _, priorityGroup := range plan {
		for _, attempt := range priorityGroup {
			if attempt.Route.ModelName != "" {
				c.Set("request_model_resolved", attempt.Route.ModelName)
				c.Set("request_model", attempt.Route.ModelName)
			}

			if proxyErr := service.ProxyToUpstream(c, attempt.Token, attempt.Provider); proxyErr == nil {
				return
			} else {
				lastErr = proxyErr
				if !proxyErr.Retryable {
					statusCode := proxyErr.StatusCode
					if statusCode <= 0 {
						statusCode = http.StatusBadGateway
					}
					c.JSON(statusCode, gin.H{
						"error": gin.H{
							"message": proxyErr.Message,
							"type":    "server_error",
							"code":    "upstream_request_failed",
						},
					})
					return
				}
			}
		}
	}

	message := "no available provider for model: " + requestedModel
	if lastErr != nil && lastErr.Message != "" {
		message = "all providers failed for model: " + requestedModel
	}
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error": gin.H{
			"message": message,
			"type":    "server_error",
			"code":    "service_unavailable",
		},
	})
}

// ListModels returns all available models across all providers
func ListModels(c *gin.Context) {
	aggToken := c.MustGet("agg_token").(*model.AggregatedToken)
	models, err := model.GetDistinctModels()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"object": "list",
			"data":   []gin.H{},
		})
		return
	}

	var modelList []gin.H
	for _, m := range models {
		if !aggToken.IsModelAllowed(m) {
			continue
		}
		modelList = append(modelList, gin.H{
			"id":       m,
			"object":   "model",
			"owned_by": "aggregated-gateway",
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   modelList,
	})
}

func GetModel(c *gin.Context) {
	aggToken := c.MustGet("agg_token").(*model.AggregatedToken)
	modelName := strings.TrimSpace(c.Param("model"))
	if modelName == "" {
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"message": "model not found: " + modelName,
				"type":    "invalid_request_error",
			},
		})
		return
	}

	resolvedModel := modelName
	if canonical, ok, err := model.ResolveCanonicalModelName(modelName); err == nil && ok {
		resolvedModel = canonical
	}
	if !aggToken.IsModelAllowed(resolvedModel) {
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"message": "model not found: " + modelName,
				"type":    "invalid_request_error",
			},
		})
		return
	}

	models, err := model.GetDistinctModels()
	if err == nil {
		for _, m := range models {
			if m != resolvedModel {
				continue
			}
			c.JSON(http.StatusOK, gin.H{
				"id":       m,
				"object":   "model",
				"owned_by": "aggregated-gateway",
			})
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{
		"error": gin.H{
			"message": "model not found: " + modelName,
			"type":    "invalid_request_error",
		},
	})
}

// BillingSubscription returns a fake billing subscription for compatibility
func BillingSubscription(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"object":                "billing_subscription",
		"has_payment_method":    true,
		"hard_limit_usd":        999999,
		"soft_limit_usd":        999999,
		"system_hard_limit_usd": 999999,
		"access_until":          4102444800, // 2100-01-01
	})
}

// BillingUsage returns usage data for compatibility
func BillingUsage(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"object":      "list",
		"total_usage": 0,
	})
}

func extractRequestedModel(c *gin.Context) string {
	modelName := extractModelFromBody(c)
	if modelName != "" {
		return modelName
	}
	return extractModelFromGeminiPath(c.Request.URL.Path)
}

func extractModelFromBody(c *gin.Context) string {
	// Read body and restore it
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return ""
	}
	// Restore body for later use by proxy
	c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	var body struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		return ""
	}
	return body.Model
}

func extractModelFromGeminiPath(path string) string {
	rawPath := strings.TrimSpace(path)
	if rawPath == "" {
		return ""
	}

	const prefix = "/v1beta/models/"
	if !strings.Contains(rawPath, prefix) {
		return ""
	}
	idx := strings.Index(rawPath, prefix)
	if idx < 0 {
		return ""
	}
	rest := strings.TrimSpace(rawPath[idx+len(prefix):])
	if rest == "" {
		return ""
	}
	if colonIdx := strings.Index(rest, ":"); colonIdx >= 0 {
		rest = rest[:colonIdx]
	}
	rest = strings.Trim(rest, "/")
	if rest == "" {
		return ""
	}
	if slashIdx := strings.LastIndex(rest, "/"); slashIdx >= 0 && slashIdx < len(rest)-1 {
		rest = rest[slashIdx+1:]
	}
	return strings.TrimSpace(rest)
}
