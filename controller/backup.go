package controller

import (
	"NewAPI-Gateway/model"
	"NewAPI-Gateway/service"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func GetBackupStatusHandler(c *gin.Context) {
	status, err := service.GetBackupStatusView()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	preflight, preflightErr := service.BackupSnapshotPreflight(status.Config)
	if preflightErr != nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": status, "preflight": gin.H{"ready": false, "message": preflightErr.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": status, "preflight": preflight})
}

func GetBackupRunsHandler(c *gin.Context) {
	limit, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("limit", "50")))
	runs, err := model.GetRecentBackupRuns(limit)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": runs})
}

func GetBackupRetryQueueHandler(c *gin.Context) {
	limit, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("limit", "100")))
	rows, err := model.GetRecentBackupUploadRetries(limit)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": rows})
}

func TriggerBackupHandler(c *gin.Context) {
	trigger := strings.TrimSpace(c.DefaultQuery("trigger", "manual"))
	if trigger == "" {
		trigger = "manual"
	}
	run, err := service.TriggerBackupNow(trigger)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": run})
}

func ValidateBackupRestoreHandler(c *gin.Context) {
	var req service.BackupRestoreRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的参数"})
		return
	}
	req.DryRun = true
	result, err := service.ValidateBackupRestoreRequest(req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": result})
}

func ExecuteBackupRestoreHandler(c *gin.Context) {
	var req service.BackupRestoreRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的参数"})
		return
	}
	if req.DryRun {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "执行恢复时 dry_run 必须为 false"})
		return
	}
	if !req.Confirm {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "恢复操作需要 confirm=true。建议先调用 /api/backup/restore/validate 进行 dry-run 校验"})
		return
	}
	result, err := service.ExecuteBackupRestore(req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": result})
}
