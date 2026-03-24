package controller

import (
	"NewAPI-Gateway/common"
	"NewAPI-Gateway/model"
	"NewAPI-Gateway/service"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func GetProviders(c *gin.Context) {
	pagination := parsePaginationParams(c)
	keyword := strings.TrimSpace(c.Query("keyword"))
	providers, total, err := model.QueryProviders(keyword, pagination.Offset, pagination.PageSize)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	// Clean sensitive fields
	for _, provider := range providers {
		provider.CleanForResponse()
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": buildPaginatedData(providers, pagination, total)})
}

func GetProviderSummary(c *gin.Context) {
	summary, err := model.GetProviderSummary()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": summary})
}

func GetProviderDetail(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的 ID"})
		return
	}
	provider, err := model.GetProviderById(id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "供应商不存在"})
		return
	}
	if item, itemErr := model.GetLatestCheckinRunItemByProviderId(id); itemErr == nil && item != nil {
		provider.LastCheckinStatus = item.Status
		provider.LastCheckinMessage = item.Message
		provider.LastCheckinQuotaAwarded = item.QuotaAwarded
		provider.LastCheckinResultAt = item.CheckedAt
	}
	provider.CleanForResponse()
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": provider})
}

func GetCheckinRunSummaries(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit <= 0 {
		limit = 20
	}
	runs, err := model.GetRecentCheckinRuns(limit)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": runs})
}

func GetCheckinRunMessages(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit <= 0 {
		limit = 50
	}
	items, err := model.GetRecentCheckinRunItems(limit)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": items})
}

func GetUncheckinProviders(c *gin.Context) {
	providers, dayStart, timezone, err := service.GetUncheckinProviders(time.Now())
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	for _, provider := range providers {
		provider.CleanForResponse()
	}
	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "",
		"data":      providers,
		"day_start": dayStart,
		"timezone":  timezone,
	})
}

func TriggerFullCheckinRunHandler(c *gin.Context) {
	run, err := service.TriggerUncheckedCheckinRun("manual")
	if err != nil {
		if errors.Is(err, service.ErrCheckinRunInProgress) {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "签到任务正在执行中，请稍后再试"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "触发签到任务失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": run.Message,
		"data":    run,
	})
}

func GetProviderModelAliasMapping(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的 ID"})
		return
	}
	provider, err := model.GetProviderById(id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "供应商不存在"})
		return
	}
	mapping := model.ParseProviderAliasMapping(provider.ModelAliasMapping)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    mapping,
	})
}

func UpdateProviderModelAliasMapping(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的 ID"})
		return
	}
	provider, err := model.GetProviderById(id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "供应商不存在"})
		return
	}

	var req struct {
		ModelAliasMapping map[string]string `json:"model_alias_mapping"`
	}
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的参数"})
		return
	}

	payload, err := model.MarshalProviderAliasMapping(req.ModelAliasMapping)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "模型映射 JSON 无效"})
		return
	}
	provider.UpdateModelAliasMapping(payload)
	service.MarkBackupDirty(fmt.Sprintf("provider_alias_mapping_update:%d", provider.Id))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    model.ParseProviderAliasMapping(payload),
	})
}

func ExportProviders(c *gin.Context) {
	providers, err := model.GetAllProviders(0, 1000)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	// Export format: include access_token for re-import
	type ExportItem struct {
		Name              string `json:"name"`
		BaseURL           string `json:"base_url"`
		AccessToken       string `json:"access_token"`
		UserID            int    `json:"user_id,omitempty"`
		Status            int    `json:"status,omitempty"`
		Priority          int    `json:"priority,omitempty"`
		Weight            int    `json:"weight,omitempty"`
		CheckinEnabled    bool   `json:"checkin_enabled,omitempty"`
		ProxyEnabled      bool   `json:"proxy_enabled,omitempty"`
		ProxyURL          string `json:"proxy_url,omitempty"`
		ProxyURLRedacted  string `json:"proxy_url_redacted,omitempty"`
		ModelAliasMapping string `json:"model_alias_mapping,omitempty"`
		Remark            string `json:"remark,omitempty"`
	}
	var items []ExportItem
	for _, p := range providers {
		items = append(items, ExportItem{
			Name:              p.Name,
			BaseURL:           p.BaseURL,
			AccessToken:       p.AccessToken,
			UserID:            p.UserID,
			Status:            p.Status,
			Priority:          p.Priority,
			Weight:            p.Weight,
			CheckinEnabled:    p.CheckinEnabled,
			ProxyEnabled:      p.ProxyEnabled,
			ProxyURL:          p.ProxyURL,
			ProxyURLRedacted:  model.RedactProxyURL(p.ProxyURL),
			ModelAliasMapping: p.ModelAliasMapping,
			Remark:            p.Remark,
		})
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": items})
}

func ImportProviders(c *gin.Context) {
	// Use a flexible import struct to accept user_id as string or int
	type ImportItem struct {
		Name              string            `json:"name"`
		BaseURL           string            `json:"base_url"`
		AccessToken       string            `json:"access_token"`
		UserID            json.Number       `json:"user_id"`
		Status            int               `json:"status"`
		Priority          int               `json:"priority"`
		Weight            int               `json:"weight"`
		CheckinEnabled    bool              `json:"checkin_enabled"`
		ProxyEnabled      bool              `json:"proxy_enabled"`
		ProxyURL          string            `json:"proxy_url"`
		ModelAliasMapping map[string]string `json:"model_alias_mapping"`
		Remark            string            `json:"remark"`
	}
	var items []ImportItem
	if err := json.NewDecoder(c.Request.Body).Decode(&items); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "JSON 格式错误: " + err.Error()})
		return
	}
	created := 0
	updated := 0
	skipped := 0
	for _, item := range items {
		if item.Name == "" || item.BaseURL == "" || item.AccessToken == "" {
			skipped++
			continue
		}
		uid, _ := item.UserID.Int64()
		userID := int(uid)

		aliasMapping := ""
		if payload, err := model.MarshalProviderAliasMapping(item.ModelAliasMapping); err == nil {
			aliasMapping = payload
		}

		status := item.Status
		if status == 0 {
			status = 1
		}
		weight := item.Weight
		if weight == 0 {
			weight = 10
		}
		if err := model.ValidateProviderProxyConfig(item.ProxyEnabled, item.ProxyURL); err != nil {
			skipped++
			continue
		}

		// Conflict detection: use (base_url, user_id) as logical unique key
		// - Same base_url + same user_id → update existing record
		// - Same base_url + different user_id → create new (multi-account coexistence)
		// - New base_url → create new
		existing, err := model.FindProviderByBaseURLAndUserID(item.BaseURL, userID)
		if err != nil {
			skipped++
			continue
		}

		if existing != nil {
			// Update existing provider
			existing.Name = item.Name
			existing.AccessToken = item.AccessToken
			existing.Status = status
			existing.Priority = item.Priority
			existing.Weight = weight
			existing.CheckinEnabled = item.CheckinEnabled
			existing.ProxyEnabled = item.ProxyEnabled
			if strings.TrimSpace(item.ProxyURL) != "" {
				existing.ProxyURL = item.ProxyURL
			}
			existing.Remark = item.Remark
			if aliasMapping != "" {
				existing.ModelAliasMapping = aliasMapping
			}
			if err := existing.Update(); err != nil {
				skipped++
			} else {
				updated++
			}
		} else {
			// Create new provider
			p := model.Provider{
				Name:              item.Name,
				BaseURL:           item.BaseURL,
				AccessToken:       item.AccessToken,
				UserID:            userID,
				Status:            status,
				Priority:          item.Priority,
				Weight:            weight,
				CheckinEnabled:    item.CheckinEnabled,
				ProxyEnabled:      item.ProxyEnabled,
				ProxyURL:          item.ProxyURL,
				ModelAliasMapping: aliasMapping,
				Remark:            item.Remark,
			}
			if err := p.Insert(); err != nil {
				skipped++
			} else {
				created++
			}
		}
	}
	msg := fmt.Sprintf("新增 %d 个，更新 %d 个，跳过 %d 个", created, updated, skipped)
	if created > 0 || updated > 0 {
		service.MarkBackupDirty("provider_import")
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": msg})
}

func CreateProvider(c *gin.Context) {
	var provider model.Provider
	if err := json.NewDecoder(c.Request.Body).Decode(&provider); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的参数"})
		return
	}
	if provider.Name == "" || provider.BaseURL == "" || provider.AccessToken == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "名称、地址和 AccessToken 不能为空"})
		return
	}
	if err := model.ValidateProviderProxyConfig(provider.ProxyEnabled, provider.ProxyURL); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	if err := provider.Insert(); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	service.MarkBackupDirty(fmt.Sprintf("provider_create:%d", provider.Id))
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func UpdateProvider(c *gin.Context) {
	rawBody, readErr := io.ReadAll(c.Request.Body)
	if readErr != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的参数"})
		return
	}

	var provider model.Provider
	if err := json.Unmarshal(rawBody, &provider); err != nil || provider.Id == 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的参数"})
		return
	}
	existing, existingErr := model.GetProviderById(provider.Id)
	if existingErr != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "供应商不存在"})
		return
	}

	var checkinPayload struct {
		CheckinEnabled *bool `json:"checkin_enabled"`
		ProxyEnabled   *bool `json:"proxy_enabled"`
	}
	if err := json.Unmarshal(rawBody, &checkinPayload); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的参数"})
		return
	}
	if strings.TrimSpace(provider.AccessToken) == "" {
		provider.AccessToken = existing.AccessToken
	}
	if strings.TrimSpace(provider.ProxyURL) == "" {
		provider.ProxyURL = existing.ProxyURL
	}
	if err := model.ValidateProviderProxyConfig(provider.ProxyEnabled, provider.ProxyURL); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	if err := provider.Update(); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	if checkinPayload.CheckinEnabled != nil {
		if err := (&model.Provider{Id: provider.Id}).UpdateCheckinEnabled(*checkinPayload.CheckinEnabled); err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
			return
		}
	}
	if checkinPayload.ProxyEnabled != nil {
		if err := model.DB.Model(&model.Provider{}).Where("id = ?", provider.Id).Update("proxy_enabled", *checkinPayload.ProxyEnabled).Error; err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
			return
		}
	}
	service.MarkBackupDirty(fmt.Sprintf("provider_update:%d", provider.Id))
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func DeleteProvider(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的 ID"})
		return
	}
	provider := &model.Provider{Id: id}
	if err := provider.Delete(); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	service.MarkBackupDirty(fmt.Sprintf("provider_delete:%d", id))
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func SyncProviderHandler(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的 ID"})
		return
	}
	provider, err := model.GetProviderById(id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "供应商不存在"})
		return
	}
	go func() {
		if err := service.SyncProvider(provider); err != nil {
			common.SysLog("sync provider failed: " + err.Error())
		}
	}()
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "同步任务已启动，请稍后在令牌列表查看 key_status 恢复结果"})
}

func CheckinProviderHandler(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的 ID"})
		return
	}
	provider, err := model.GetProviderById(id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "供应商不存在"})
		return
	}
	run, item, err := service.RunProviderCheckin(provider)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "签到失败: " + err.Error(),
			"data": gin.H{
				"run":  run,
				"item": item,
			},
		})
		return
	}
	message := strings.TrimSpace(item.Message)
	if message == "" {
		message = "签到成功"
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": message,
		"data": gin.H{
			"run":  run,
			"item": item,
		},
	})
}

func GetProviderTokens(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的 ID"})
		return
	}
	tokens, err := model.GetProviderTokensByProviderId(id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	for _, t := range tokens {
		t.CleanForResponse()
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": tokens})
}

type tokenGroupOption struct {
	GroupName string  `json:"group_name"`
	Ratio     float64 `json:"ratio"`
}

func parseEnableGroupsJSON(enableGroups string) []string {
	if strings.TrimSpace(enableGroups) == "" {
		return nil
	}
	var groups []string
	if err := json.Unmarshal([]byte(enableGroups), &groups); err != nil {
		return nil
	}
	result := make([]string, 0, len(groups))
	for _, group := range groups {
		normalized := strings.TrimSpace(group)
		if normalized == "" {
			continue
		}
		result = append(result, normalized)
	}
	return result
}

func collectProviderAvailableGroups(pricing []*model.ModelPricing) map[string]struct{} {
	availableGroups := make(map[string]struct{})
	for _, item := range pricing {
		for _, group := range parseEnableGroupsJSON(item.EnableGroups) {
			availableGroups[group] = struct{}{}
		}
	}
	return availableGroups
}

func buildTokenGroupOptions(availableGroups map[string]struct{}, groupRatio map[string]float64) []tokenGroupOption {
	options := make([]tokenGroupOption, 0, len(availableGroups))
	for groupName := range availableGroups {
		ratio := groupRatio[groupName]
		if ratio <= 0 {
			ratio = 1
		}
		options = append(options, tokenGroupOption{
			GroupName: groupName,
			Ratio:     ratio,
		})
	}
	sort.Slice(options, func(i, j int) bool {
		if options[i].Ratio == options[j].Ratio {
			return options[i].GroupName < options[j].GroupName
		}
		return options[i].Ratio < options[j].Ratio
	})
	return options
}

func pickAvailableGroup(candidate string, availableGroups map[string]struct{}) string {
	normalized := strings.TrimSpace(candidate)
	if normalized == "" {
		return ""
	}
	if _, ok := availableGroups[normalized]; !ok {
		return ""
	}
	return normalized
}

func resolveDefaultGroupFromUsable(usableGroup map[string]string, userID int, availableGroups map[string]struct{}, options []tokenGroupOption) string {
	if len(availableGroups) == 0 {
		return ""
	}
	if len(usableGroup) > 0 {
		keyCandidates := []string{
			strconv.Itoa(userID),
			"user_id",
			"userid",
			"default",
			"default_group",
			"group",
		}
		for _, key := range keyCandidates {
			if value, ok := usableGroup[key]; ok {
				if group := pickAvailableGroup(value, availableGroups); group != "" {
					return group
				}
				if group := pickAvailableGroup(key, availableGroups); group != "" {
					return group
				}
			}
		}

		for key, value := range usableGroup {
			if strings.Contains(strings.ToLower(strings.TrimSpace(key)), "default") {
				if group := pickAvailableGroup(value, availableGroups); group != "" {
					return group
				}
			}
		}
		for _, value := range usableGroup {
			if group := pickAvailableGroup(value, availableGroups); group != "" {
				return group
			}
		}
		for key := range usableGroup {
			if group := pickAvailableGroup(key, availableGroups); group != "" {
				return group
			}
		}
	}
	if len(options) == 0 {
		return ""
	}
	return options[0].GroupName
}

func getProviderAvailableGroups(providerId int) (map[string]struct{}, error) {
	pricing, err := model.GetModelPricingByProvider(providerId)
	if err != nil {
		return nil, err
	}
	return collectProviderAvailableGroups(pricing), nil
}

func GetProviderPricing(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的 ID"})
		return
	}
	provider, err := model.GetProviderById(id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "供应商不存在"})
		return
	}
	pricing, err := model.GetModelPricingByProvider(id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	groupRatio := map[string]float64{}
	if provider.PricingGroupRatio != "" {
		_ = json.Unmarshal([]byte(provider.PricingGroupRatio), &groupRatio)
	}

	usableGroup := map[string]string{}
	if provider.PricingUsableGroup != "" {
		_ = json.Unmarshal([]byte(provider.PricingUsableGroup), &usableGroup)
	}

	supportedEndpoint := map[string]map[string]string{}
	if provider.PricingSupportedEndpoint != "" {
		_ = json.Unmarshal([]byte(provider.PricingSupportedEndpoint), &supportedEndpoint)
	}

	availableGroups := collectProviderAvailableGroups(pricing)
	tokenGroupOptions := buildTokenGroupOptions(availableGroups, groupRatio)
	defaultGroup := resolveDefaultGroupFromUsable(usableGroup, provider.UserID, availableGroups, tokenGroupOptions)

	c.JSON(http.StatusOK, gin.H{
		"success":             true,
		"message":             "",
		"data":                pricing,
		"group_ratio":         groupRatio,
		"usable_group":        usableGroup,
		"default_group":       defaultGroup,
		"token_group_options": tokenGroupOptions,
		"supported_endpoint":  supportedEndpoint,
	})
}

func buildProviderTokenCreateMessage(outcome *service.ProviderTokenCreateOutcome) string {
	if outcome == nil {
		return "Token 已在上游创建，但明文密钥暂未恢复，请稍后同步后重试"
	}
	if outcome.KeyStatus == model.ProviderTokenKeyStatusReady {
		return "Token 已在上游创建，密钥已同步，可在列表中复制"
	}
	switch strings.TrimSpace(outcome.KeyUnresolvedReason) {
	case model.ProviderTokenKeyUnresolvedReasonKeyEndpointUnavailable:
		return "Token 已在上游创建，但上游未开放明文恢复接口（POST /api/token/{id}/key）"
	case model.ProviderTokenKeyUnresolvedReasonKeyEndpointUnauthorized:
		return "Token 已在上游创建，但明文恢复鉴权失败，请检查 Authorization 与 New-Api-User 是否匹配"
	case model.ProviderTokenKeyUnresolvedReasonKeyEndpointRequestFailed:
		return "Token 已在上游创建，但明文恢复请求失败，请检查网络连通性后重试"
	default:
		return "Token 已在上游创建，但明文密钥暂未恢复，请稍后同步后重试"
	}
}

func CreateProviderToken(c *gin.Context) {
	providerId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的供应商 ID"})
		return
	}
	provider, err := model.GetProviderById(providerId)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "供应商不存在"})
		return
	}
	var req struct {
		Name           string `json:"name"`
		GroupName      string `json:"group_name"`
		UnlimitedQuota bool   `json:"unlimited_quota"`
		RemainQuota    int64  `json:"remain_quota"`
		ModelLimits    string `json:"model_limits"`
	}
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的参数"})
		return
	}
	req.GroupName = strings.TrimSpace(req.GroupName)
	if req.GroupName == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "分组不能为空"})
		return
	}
	availableGroups, err := getProviderAvailableGroups(providerId)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	if len(availableGroups) == 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "未获取到可用分组，请先同步供应商数据"})
		return
	}
	if _, ok := availableGroups[req.GroupName]; !ok {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "分组不属于该渠道可用分组，请先同步后重试"})
		return
	}
	outcome, err := service.CreateProviderTokenWithReconciliation(provider, req.Name, req.GroupName, req.UnlimitedQuota, req.RemainQuota, req.ModelLimits)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "上游创建 Token 失败: " + err.Error()})
		return
	}
	service.MarkBackupDirty(fmt.Sprintf("provider_token_create_upstream:%d", providerId))
	c.JSON(http.StatusOK, gin.H{"success": true, "message": buildProviderTokenCreateMessage(outcome), "data": outcome})
}

func UpdateProviderToken(c *gin.Context) {
	tokenId, err := strconv.Atoi(c.Param("token_id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的 Token ID"})
		return
	}
	var token model.ProviderToken
	if err := json.NewDecoder(c.Request.Body).Decode(&token); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的参数"})
		return
	}
	token.Id = tokenId
	// Use UpdateMetadataOnly to prevent sk_key overwrite from frontend
	if err := token.UpdateMetadataOnly(); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	service.MarkBackupDirty(fmt.Sprintf("provider_token_update:%d", token.Id))
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func DeleteProviderToken(c *gin.Context) {
	tokenId, err := strconv.Atoi(c.Param("token_id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的 Token ID"})
		return
	}
	token, err := model.GetProviderTokenById(tokenId)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Token 不存在"})
		return
	}
	// If this token came from upstream, delete upstream first; otherwise it will be re-synced later.
	if token.UpstreamTokenId > 0 {
		provider, err := model.GetProviderById(token.ProviderId)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "供应商不存在"})
			return
		}
		client := service.NewUpstreamClient(provider.BaseURL, provider.AccessToken, provider.UserID)
		if err := client.DeleteUpstreamToken(token.UpstreamTokenId); err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "上游删除 Token 失败: " + err.Error()})
			return
		}
	}
	if err := token.Delete(); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	service.MarkBackupDirty(fmt.Sprintf("provider_token_delete:%d", token.Id))
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}
