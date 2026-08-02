package api

import (
	"iot-platform/internal/model"
	"iot-platform/internal/repository"

	"github.com/gin-gonic/gin"
)

func ListDeviceLogs(c *gin.Context) {
	deviceID := c.Param("id")
	var params struct {
		Category string `form:"category"`
		Page     int    `form:"page,default=1"`
		Size     int    `form:"size,default=20"`
	}
	c.ShouldBindQuery(&params)
	if params.Size <= 0 {
		params.Size = 20
	}
	if params.Page <= 0 {
		params.Page = 1
	}

	q := repository.DB.Where("device_id = ?", deviceID)
	if params.Category != "" {
		q = q.Where("category = ?", params.Category)
	}

	var total int64
	q.Model(&model.DeviceLog{}).Count(&total)

	var list []model.DeviceLog
	q.Order("id desc").Offset((params.Page - 1) * params.Size).Limit(params.Size).Find(&list)

	OK(c, gin.H{"list": list, "total": total})
}

func ListAllDeviceLogs(c *gin.Context) {
	var params struct {
		DeviceID uint   `form:"deviceId"`
		Category string `form:"category"`
		Page     int    `form:"page,default=1"`
		Size     int    `form:"size,default=20"`
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
	if params.Category != "" {
		q = q.Where("category = ?", params.Category)
	}

	var total int64
	q.Model(&model.DeviceLog{}).Count(&total)

	var list []model.DeviceLog
	q.Order("id desc").Offset((params.Page - 1) * params.Size).Limit(params.Size).Find(&list)

	OK(c, gin.H{"list": list, "total": total})
}
