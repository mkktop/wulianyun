package api

import (
	"encoding/json"

	"github.com/gin-gonic/gin"

	"iot-platform/internal/model"
	"iot-platform/internal/repository"
)

// TSL 属性定义
type tslProperty struct {
	Identifier string   `json:"identifier"`
	Name       string   `json:"name"`
	DataType   string   `json:"dataType"` // int / float / bool / enum / string
	Unit       string   `json:"unit"`
	Min        *float64 `json:"min"`
	Max        *float64 `json:"max"`
	AccessMode string   `json:"accessMode"` // r / rw
}

type tslService struct {
	Identifier string `json:"identifier"`
	Name       string `json:"name"`
	Desc       string `json:"desc"`
	Params     []struct {
		Identifier string `json:"identifier"`
		Name       string `json:"name"`
		DataType   string `json:"dataType"`
	} `json:"params"`
}

// GetThingModel 查询产品物模型（无则返回空模型）
func GetThingModel(c *gin.Context) {
	var p model.Product
	if err := repository.DB.Where("id = ? AND user_id = ?", c.Param("id"), UID(c)).First(&p).Error; err != nil {
		Fail(c, 404, "产品不存在")
		return
	}
	var tm model.ThingModel
	if err := repository.DB.Where("product_id = ?", p.ID).First(&tm).Error; err != nil {
		tm = model.ThingModel{ProductID: p.ID, Properties: []byte("[]"), Services: []byte("[]")}
	}
	OK(c, tm)
}

// SaveThingModel 保存产品物模型（整体覆盖）
func SaveThingModel(c *gin.Context) {
	var p model.Product
	if err := repository.DB.Where("id = ? AND user_id = ?", c.Param("id"), UID(c)).First(&p).Error; err != nil {
		Fail(c, 404, "产品不存在")
		return
	}
	var req struct {
		Properties []tslProperty `json:"properties"`
		Services   []tslService  `json:"services"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, 400, "参数错误")
		return
	}
	// 校验标识符唯一且非空
	seen := map[string]bool{}
	for _, prop := range req.Properties {
		if prop.Identifier == "" || prop.Name == "" {
			Fail(c, 400, "属性标识符和名称必填")
			return
		}
		if seen[prop.Identifier] {
			Fail(c, 400, "属性标识符重复: "+prop.Identifier)
			return
		}
		seen[prop.Identifier] = true
	}
	props, _ := json.Marshal(req.Properties)
	svcs, _ := json.Marshal(req.Services)

	var tm model.ThingModel
	if err := repository.DB.Where("product_id = ?", p.ID).First(&tm).Error; err != nil {
		tm = model.ThingModel{ProductID: p.ID, Properties: props, Services: svcs}
		repository.DB.Create(&tm)
	} else {
		repository.DB.Model(&tm).Updates(map[string]interface{}{"properties": props, "services": svcs})
	}
	OK(c, tm)
}
