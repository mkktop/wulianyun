package api

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gin-gonic/gin"

	"iot-platform/internal/model"
	"iot-platform/internal/repository"
)

func randHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

type productReq struct {
	Name         string `json:"name" binding:"required,max=64"`
	Protocol     string `json:"protocol" binding:"omitempty,oneof=mqtt tcp http"`
	DataFormat   string `json:"dataFormat"`
	AccessMode   string `json:"accessMode" binding:"omitempty,oneof=thingmodel passthrough modbus"`
	SecretMode   string `json:"secretMode" binding:"omitempty,oneof=device product"`
	PollInterval int    `json:"pollInterval"`
	Description  string `json:"description"`
}

func CreateProduct(c *gin.Context) {
	var req productReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, 400, "产品名称必填")
		return
	}
	if req.Protocol == "" {
		req.Protocol = "mqtt"
	}
	if req.DataFormat == "" {
		req.DataFormat = "json"
	}
	if req.AccessMode == "" {
		req.AccessMode = model.AccessModeThingModel
	}
	if req.SecretMode == "" {
		req.SecretMode = model.SecretModeDevice
	}
	if req.PollInterval < 60 {
		req.PollInterval = 60
	}
	// Modbus 接入必须走 TCP（MQTT/HTTP 不支持 Modbus）
	if req.AccessMode == model.AccessModeModbus {
		req.Protocol = "tcp"
	} else if req.Protocol == "" {
		req.Protocol = "mqtt"
	}
	p := model.Product{
		UserID: UID(c), Name: req.Name, ProductKey: "pk" + randHex(8),
		Protocol: req.Protocol, DataFormat: req.DataFormat, Description: req.Description,
		AccessMode: req.AccessMode, SecretMode: req.SecretMode, PollInterval: req.PollInterval,
	}
	// 一型一密：生成产品级密钥
	if req.SecretMode == model.SecretModeProduct {
		p.ProductSecret = randHex(16)
	}
	if err := repository.DB.Create(&p).Error; err != nil {
		Fail(c, 500, "创建失败")
		return
	}
	OK(c, p)
}

func ListProducts(c *gin.Context) {
	var list []model.Product
	q := repository.DB.Where("user_id = ?", UID(c)).Order("id desc")
	if kw := c.Query("keyword"); kw != "" {
		q = q.Where("name ILIKE ?", "%"+kw+"%")
	}
	var total int64
	q.Model(&model.Product{}).Count(&total)
	page, size := pageArgs(c)
	if err := q.Offset((page - 1) * size).Limit(size).Find(&list).Error; err != nil {
		Fail(c, 500, "查询失败")
		return
	}
	for i := range list {
		repository.DB.Model(&model.Device{}).Where("product_id = ?", list[i].ID).Count(&list[i].DeviceCount)
	}
	OK(c, PageData{Total: total, List: list})
}

func GetProduct(c *gin.Context) {
	var p model.Product
	if err := repository.DB.Where("id = ? AND user_id = ?", c.Param("id"), UID(c)).First(&p).Error; err != nil {
		Fail(c, 404, "产品不存在")
		return
	}
	repository.DB.Model(&model.Device{}).Where("product_id = ?", p.ID).Count(&p.DeviceCount)
	OK(c, p)
}

func UpdateProduct(c *gin.Context) {
	var p model.Product
	if err := repository.DB.Where("id = ? AND user_id = ?", c.Param("id"), UID(c)).First(&p).Error; err != nil {
		Fail(c, 404, "产品不存在")
		return
	}
	var req productReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, 400, "参数错误")
		return
	}
	// 接入方式/协议/密钥模式创建后不可变；仅允许改名称/描述/采集周期
	updates := map[string]interface{}{"name": req.Name, "description": req.Description}
	if p.AccessMode == model.AccessModeModbus && req.PollInterval >= 60 {
		updates["poll_interval"] = req.PollInterval
	}
	repository.DB.Model(&p).Updates(updates)
	OK(c, p)
}

func DeleteProduct(c *gin.Context) {
	var p model.Product
	if err := repository.DB.Where("id = ? AND user_id = ?", c.Param("id"), UID(c)).First(&p).Error; err != nil {
		Fail(c, 404, "产品不存在")
		return
	}
	var cnt int64
	repository.DB.Model(&model.Device{}).Where("product_id = ?", p.ID).Count(&cnt)
	if cnt > 0 {
		Fail(c, 400, "产品下还有设备，请先删除设备")
		return
	}
	repository.DB.Delete(&p)
	OK(c, nil)
}

func pageArgs(c *gin.Context) (page, size int) {
	page, size = 1, 10
	if v := c.Query("page"); v != "" {
		if n := atoi(v); n > 0 {
			page = n
		}
	}
	if v := c.Query("size"); v != "" {
		if n := atoi(v); n > 0 && n <= 100 {
			size = n
		}
	}
	return
}

func atoi(s string) int {
	n := 0
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0
		}
		n = n*10 + int(ch-'0')
	}
	return n
}
