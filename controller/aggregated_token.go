package controller

import (
	"NewAPI-Gateway/model"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func GetAggTokens(c *gin.Context) {
	userId := c.GetInt("id")
	pagination := parsePaginationParams(c)
	tokens, total, err := model.QueryUserAggTokens(userId, pagination.Offset, pagination.PageSize)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": buildPaginatedData(tokens, pagination, total)})
}

func CreateAggToken(c *gin.Context) {
	userId := c.GetInt("id")
	var token model.AggregatedToken
	if err := json.NewDecoder(c.Request.Body).Decode(&token); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的参数"})
		return
	}
	token.UserId = userId
	if token.Name == "" {
		token.Name = "default"
	}
	if err := token.Insert(); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": "ag-" + token.Key})
}

func UpdateAggToken(c *gin.Context) {
	userId := c.GetInt("id")
	var token model.AggregatedToken
	if err := json.NewDecoder(c.Request.Body).Decode(&token); err != nil || token.Id == 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的参数"})
		return
	}
	// Verify ownership
	existing, err := model.GetAggTokenById(token.Id, userId)
	if err != nil || existing == nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "令牌不存在或无权操作"})
		return
	}
	token.UserId = userId
	if err := token.Update(); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func DeleteAggToken(c *gin.Context) {
	userId := c.GetInt("id")
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的 ID"})
		return
	}
	// Verify ownership
	existing, err := model.GetAggTokenById(id, userId)
	if err != nil || existing == nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "令牌不存在或无权操作"})
		return
	}
	if err := existing.Delete(); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}
