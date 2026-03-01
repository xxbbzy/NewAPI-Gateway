package controller

import (
	"NewAPI-Gateway/common"
	"NewAPI-Gateway/model"
	"NewAPI-Gateway/service"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func GetOptions(c *gin.Context) {
	var options []*model.Option
	common.OptionMapRWMutex.Lock()
	for k, v := range common.OptionMap {
		if model.IsSensitiveOptionKey(k) {
			continue
		}
		options = append(options, &model.Option{
			Key:   k,
			Value: common.Interface2String(v),
		})
	}
	common.OptionMapRWMutex.Unlock()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    options,
	})
	return
}

func UpdateOption(c *gin.Context) {
	var option model.Option
	err := json.NewDecoder(c.Request.Body).Decode(&option)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}
	const loginMethodGuardMessage = "至少保留一种登录方式（密码登录或 GitHub 登录）"
	switch option.Key {
	case "PasswordLoginEnabled":
		normalized := strings.TrimSpace(strings.ToLower(option.Value))
		if normalized != "true" && normalized != "false" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "密码登录开关必须是 true 或 false",
			})
			return
		}
		option.Value = normalized
		if normalized == "false" && !common.GitHubOAuthEnabled {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": loginMethodGuardMessage,
			})
			return
		}
	case "GitHubOAuthEnabled":
		normalized := strings.TrimSpace(strings.ToLower(option.Value))
		if normalized != "true" && normalized != "false" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "GitHub OAuth 开关必须是 true 或 false",
			})
			return
		}
		option.Value = normalized
		if normalized == "false" && !common.PasswordLoginEnabled {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": loginMethodGuardMessage,
			})
			return
		}
		if option.Value == "true" && common.GitHubClientId == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 GitHub OAuth，请先填入 GitHub Client ID 以及 GitHub Client Secret！",
			})
			return
		}
	case "WeChatAuthEnabled":
		if option.Value == "true" && common.WeChatServerAddress == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用微信登录，请先填入微信登录相关配置信息！",
			})
			return
		}
	case "TurnstileCheckEnabled":
		if option.Value == "true" && common.TurnstileSiteKey == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 Turnstile 校验，请先填入 Turnstile 校验相关配置信息！",
			})
			return
		}
	case "CheckinScheduleEnabled":
		normalized := strings.TrimSpace(strings.ToLower(option.Value))
		if normalized != "true" && normalized != "false" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "签到定时任务开关必须是 true 或 false",
			})
			return
		}
	case "CheckinScheduleTime":
		if _, err := time.Parse("15:04", strings.TrimSpace(option.Value)); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "签到时间格式必须是 HH:mm（24 小时制）",
			})
			return
		}
	case "CheckinScheduleTimezone":
		if _, err := time.LoadLocation(strings.TrimSpace(option.Value)); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "签到时区必须是有效的 IANA 时区，例如 Asia/Shanghai",
			})
			return
		}
	case model.RelayResponseValidityGuardModeOptionKey,
		model.RelayResponseValidityGuardEnabledOptionKey,
		model.RoutingInvalidResponseSuppressionEnabledOptionKey,
		model.RoutingInvalidResponseSuppressionThresholdOptionKey,
		model.RoutingInvalidResponseSuppressionWindowMinutesOptionKey,
		model.RoutingInvalidResponseSuppressionCooldownMinutesOptionKey:
		if message, ok := model.ValidateRelayReliabilityOption(option.Key, option.Value); ok && message != "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": message,
			})
			return
		}
	case model.BackupEnabledOptionKey,
		model.BackupTriggerModeOptionKey,
		model.BackupScheduleCronOptionKey,
		model.BackupMinIntervalSecondsOptionKey,
		model.BackupDebounceSecondsOptionKey,
		model.BackupWebDAVURLOptionKey,
		model.BackupWebDAVUsernameOptionKey,
		model.BackupWebDAVPasswordOptionKey,
		model.BackupWebDAVBasePathOptionKey,
		model.BackupEncryptEnabledOptionKey,
		model.BackupEncryptPassphraseOptionKey,
		model.BackupRetentionDaysOptionKey,
		model.BackupRetentionMaxFilesOptionKey,
		model.BackupSpoolDirOptionKey,
		model.BackupCommandTimeoutSecondsOptionKey,
		model.BackupMaxRetriesOptionKey,
		model.BackupRetryBaseSecondsOptionKey,
		model.BackupMySQLDumpCommandOptionKey,
		model.BackupPostgresDumpCommandOptionKey,
		model.BackupMySQLRestoreCommandOptionKey,
		model.BackupPostgresRestoreCommandOptionKey:
		if message, ok := model.ValidateBackupOption(option.Key, option.Value); ok && message != "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": message,
			})
			return
		}
	case "RoutingUsageWindowHours":
		value, err := strconv.Atoi(strings.TrimSpace(option.Value))
		if err != nil || value < 1 || value > 24*30 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "路由统计窗口必须是 1 到 720 小时的整数",
			})
			return
		}
	case "RoutingHealthAdjustmentEnabled":
		normalized := strings.TrimSpace(strings.ToLower(option.Value))
		if normalized != "true" && normalized != "false" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "健康调节开关必须是 true 或 false",
			})
			return
		}
	case "RoutingBaseWeightFactor", "RoutingValueScoreFactor":
		value, err := strconv.ParseFloat(strings.TrimSpace(option.Value), 64)
		if err != nil || value < 0 || value > 10 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "路由系数必须是 0 到 10 之间的数字",
			})
			return
		}
	case "RoutingHealthWindowHours":
		value, err := strconv.Atoi(strings.TrimSpace(option.Value))
		if err != nil || value < 1 || value > 24*30 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "健康统计窗口必须是 1 到 720 小时的整数",
			})
			return
		}
	case "RoutingFailurePenaltyAlpha":
		value, err := strconv.ParseFloat(strings.TrimSpace(option.Value), 64)
		if err != nil || value < 0 || value > 20 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "故障惩罚系数必须是 0 到 20 之间的数字",
			})
			return
		}
	case "RoutingHealthRewardBeta":
		value, err := strconv.ParseFloat(strings.TrimSpace(option.Value), 64)
		if err != nil || value < 0 || value > 2 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "健康奖励系数必须是 0 到 2 之间的数字",
			})
			return
		}
	case "RoutingHealthMinMultiplier", "RoutingHealthMaxMultiplier":
		value, err := strconv.ParseFloat(strings.TrimSpace(option.Value), 64)
		if err != nil || value < 0 || value > 10 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "健康倍率阈值必须是 0 到 10 之间的数字",
			})
			return
		}
	case "RoutingHealthMinSamples":
		value, err := strconv.Atoi(strings.TrimSpace(option.Value))
		if err != nil || value < 1 || value > 1000 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "健康最小样本数必须是 1 到 1000 的整数",
			})
			return
		}
	}
	err = model.UpdateOption(option.Key, option.Value)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
	service.MarkBackupDirty("option_update:" + option.Key)
	return
}
