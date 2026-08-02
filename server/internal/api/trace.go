package api

import (
	"iot-platform/internal/model"
	"iot-platform/internal/repository"
	"time"

	"github.com/gin-gonic/gin"
)

func ListTraces(c *gin.Context) {
	var params struct {
		DeviceID   uint   `form:"deviceId"`
		ProductKey string `form:"productKey"`
		TraceID    string `form:"traceId"`
		Status     string `form:"status"`
		StartTime  string `form:"startTime"`
		EndTime    string `form:"endTime"`
		Page       int    `form:"page,default=1"`
		Size       int    `form:"size,default=20"`
	}
	c.ShouldBindQuery(&params)
	if params.Size <= 0 {
		params.Size = 20
	}
	if params.Page <= 0 {
		params.Page = 1
	}

	q := repository.DB.Where("user_id = ?", UID(c))
	if params.DeviceID > 0 {
		q = q.Where("device_id = ?", params.DeviceID)
	}
	if params.ProductKey != "" {
		q = q.Where("product_key = ?", params.ProductKey)
	}
	if params.TraceID != "" {
		q = q.Where("trace_id = ?", params.TraceID)
	}
	if params.Status != "" {
		q = q.Where("status = ?", params.Status)
	}
	if params.StartTime != "" {
		if t, err := time.Parse(time.RFC3339, params.StartTime); err == nil {
			q = q.Where("created_at >= ?", t)
		}
	}
	if params.EndTime != "" {
		if t, err := time.Parse(time.RFC3339, params.EndTime); err == nil {
			q = q.Where("created_at <= ?", t)
		}
	}

	var total int64
	q.Model(&model.MessageTrace{}).Count(&total)

	var list []model.MessageTrace
	q.Order("id desc").Offset((params.Page - 1) * params.Size).Limit(params.Size).Find(&list)

	OK(c, gin.H{"list": list, "total": total})
}

func GetTrace(c *gin.Context) {
	traceID := c.Param("traceId")
	var trace model.MessageTrace
	if err := repository.DB.Where("trace_id = ? AND user_id = ?", traceID, UID(c)).First(&trace).Error; err != nil {
		Fail(c, 1, "trace not found")
		return
	}
	OK(c, trace)
}
