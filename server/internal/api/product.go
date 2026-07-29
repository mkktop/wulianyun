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
	Name        string `json:"name" binding:"required,max=64"`
	Protocol    string `json:"protocol" binding:"omitempty,oneof=mqtt tcp http"`
	DataFormat  string `json:"dataFormat"`
	Description string `json:"description"`
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
	p := model.Product{
		UserID: UID(c), Name: req.Name, ProductKey: "pk" + randHex(8),
		Protocol: req.Protocol, DataFormat: req.DataFormat, Description: req.Description,
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
	updates := map[string]interface{}{"name": req.Name, "description": req.Description}
	if req.Protocol != "" {
		updates["protocol"] = req.Protocol
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
