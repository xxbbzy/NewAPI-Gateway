package controller

import (
	"NewAPI-Gateway/common"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type PaginationParams struct {
	P        int
	PageSize int
	Offset   int
}

func parsePaginationParams(c *gin.Context) PaginationParams {
	p, err := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("p", "0")))
	if err != nil || p < 0 {
		p = 0
	}

	pageSize, err := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("page_size", strconv.Itoa(common.ItemsPerPage))))
	if err != nil || pageSize <= 0 {
		pageSize = common.ItemsPerPage
	}

	maxPageSize := common.MaxItemsPerPage
	if maxPageSize <= 0 {
		maxPageSize = common.ItemsPerPage
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	return PaginationParams{
		P:        p,
		PageSize: pageSize,
		Offset:   p * pageSize,
	}
}

func buildPaginatedData(items interface{}, pagination PaginationParams, total int64) gin.H {
	totalPages := 0
	if total > 0 && pagination.PageSize > 0 {
		totalPages = int((total + int64(pagination.PageSize) - 1) / int64(pagination.PageSize))
	}
	hasMore := totalPages > 0 && pagination.P+1 < totalPages

	return gin.H{
		"items":       items,
		"p":           pagination.P,
		"page":        pagination.P, // Backward compatibility for old clients expecting `page`.
		"page_size":   pagination.PageSize,
		"total":       total,
		"total_pages": totalPages,
		"has_more":    hasMore,
	}
}
