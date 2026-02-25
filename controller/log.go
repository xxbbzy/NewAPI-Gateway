package controller

import (
	"NewAPI-Gateway/model"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func parseLogListQuery(c *gin.Context) (PaginationParams, model.UsageLogQuery) {
	pagination := parsePaginationParams(c)
	query := model.UsageLogQuery{
		Offset:       pagination.Offset,
		Limit:        pagination.PageSize,
		Keyword:      strings.TrimSpace(c.Query("keyword")),
		ProviderName: strings.TrimSpace(c.Query("provider")),
		Status:       strings.TrimSpace(c.DefaultQuery("status", "all")),
		ViewTab:      strings.TrimSpace(c.DefaultQuery("view", "all")),
	}
	return pagination, query
}

func GetSelfLogs(c *gin.Context) {
	userId := c.GetInt("id")
	pagination, query := parseLogListQuery(c)
	query.UserID = &userId

	logs, total, err := model.QueryUsageLogs(query)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	providers, err := model.QueryUsageLogProviders(query)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	summary, err := model.QueryUsageLogSummary(query)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	data := buildPaginatedData(logs, pagination, total)
	data["providers"] = providers
	data["summary"] = summary
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    data,
	})
}

func GetAllLogs(c *gin.Context) {
	pagination, query := parseLogListQuery(c)
	logs, total, err := model.QueryUsageLogs(query)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	providers, err := model.QueryUsageLogProviders(query)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	summary, err := model.QueryUsageLogSummary(query)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	data := buildPaginatedData(logs, pagination, total)
	data["providers"] = providers
	data["summary"] = summary
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    data,
	})
}

func GetDashboard(c *gin.Context) {
	stats, err := model.GetDashboardStats()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": stats})
}
