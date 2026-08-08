package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"iot-platform/internal/model"
	"iot-platform/internal/repository"
)

// TSL 数据类型与保留字
var tslDataTypes = map[string]bool{
	"int32": true, "float": true, "double": true, "bool": true,
	"enum": true, "text": true, "date": true,
}
var tslReserved = map[string]bool{
	"set": true, "get": true, "post": true, "property": true,
	"event": true, "time": true, "value": true,
}

// tslEnumItem 枚举项
type tslEnumItem struct {
	Value int    `json:"value"`
	Label string `json:"label"`
}

// tslProperty 完整 TSL 属性
type tslProperty struct {
	Identifier string        `json:"identifier"`
	Name       string        `json:"name"`
	DataType   string        `json:"dataType"` // int32/float/double/bool/enum/text/date
	Unit       string        `json:"unit"`
	Min        *float64      `json:"min"`
	Max        *float64      `json:"max"`
	Step       *float64      `json:"step"`
	AccessMode string        `json:"accessMode"` // r / rw
	EnumSpec   []tslEnumItem `json:"enumSpec"`
	Desc       string        `json:"desc"`
}

type tslParam struct {
	Identifier string `json:"identifier"`
	Name       string `json:"name"`
	DataType   string `json:"dataType"`
}

// tslEvent 事件（信息/告警/故障）
type tslEvent struct {
	Identifier string     `json:"identifier"`
	Name       string     `json:"name"`
	Type       string     `json:"type"` // info / alert / fault
	Outputs    []tslParam `json:"outputs"`
	Desc       string     `json:"desc"`
}

// tslService 服务（含入参/出参、同步/异步）
type tslService struct {
	Identifier string     `json:"identifier"`
	Name       string     `json:"name"`
	Async      bool       `json:"async"`
	Inputs     []tslParam `json:"inputs"`
	Outputs    []tslParam `json:"outputs"`
	Desc       string     `json:"desc"`
}

// GetThingModel 查询产品物模型（无则返回空模型）
func GetThingModel(c *gin.Context) {
	p, err := canViewProduct(c, c.Param("id"))
	if err != nil {
		Fail(c, 404, "产品不存在")
		return
	}
	var tm model.ThingModel
	if err := repository.DB.Where("product_id = ?", p.ID).First(&tm).Error; err != nil {
		tm = model.ThingModel{ProductID: p.ID, Properties: []byte("[]"), Events: []byte("[]"), Services: []byte("[]")}
	}
	OK(c, tm)
}

// SaveThingModel 保存产品物模型（整体覆盖）+ 完整 TSL 校验
func SaveThingModel(c *gin.Context) {
	p, err := mustOwnProduct(c, c.Param("id"))
	if err != nil {
		Fail(c, 404, "产品不存在")
		return
	}
	var req struct {
		Properties []tslProperty `json:"properties"`
		Events     []tslEvent    `json:"events"`
		Services   []tslService  `json:"services"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, 400, "参数错误")
		return
	}

	// 校验属性
	seen := map[string]bool{}
	for _, prop := range req.Properties {
		if msg := validIdentifier(prop.Identifier, prop.Name, "属性", seen); msg != "" {
			Fail(c, 400, msg)
			return
		}
		if !tslDataTypes[prop.DataType] {
			Fail(c, 400, "属性["+prop.Identifier+"]数据类型非法: "+prop.DataType)
			return
		}
		if prop.Min != nil && prop.Max != nil && *prop.Min > *prop.Max {
			Fail(c, 400, "属性["+prop.Identifier+"]取值范围 min 不能大于 max")
			return
		}
		if prop.DataType == "enum" && len(prop.EnumSpec) == 0 {
			Fail(c, 400, "枚举属性["+prop.Identifier+"]必须定义枚举项")
			return
		}
	}
	// 校验事件
	eseen := map[string]bool{}
	for _, e := range req.Events {
		if msg := validIdentifier(e.Identifier, e.Name, "事件", eseen); msg != "" {
			Fail(c, 400, msg)
			return
		}
		if e.Type != "" && e.Type != "info" && e.Type != "alert" && e.Type != "fault" {
			Fail(c, 400, "事件["+e.Identifier+"]类型非法，应为 info/alert/fault")
			return
		}
	}
	// 校验服务
	sseen := map[string]bool{}
	for _, s := range req.Services {
		if msg := validIdentifier(s.Identifier, s.Name, "服务", sseen); msg != "" {
			Fail(c, 400, msg)
			return
		}
	}

	props, _ := json.Marshal(req.Properties)
	events, _ := json.Marshal(req.Events)
	svcs, _ := json.Marshal(req.Services)

	var tm model.ThingModel
	if err := repository.DB.Where("product_id = ?", p.ID).First(&tm).Error; err != nil {
		tm = model.ThingModel{ProductID: p.ID, Properties: props, Events: events, Services: svcs}
		repository.DB.Create(&tm)
	} else {
		repository.DB.Model(&tm).Updates(map[string]interface{}{
			"properties": props, "events": events, "services": svcs,
		})
	}
	OK(c, tm)
}

// validIdentifier 校验标识符与名称：非空、唯一、不含保留字
func validIdentifier(id, name, kind string, seen map[string]bool) string {
	if id == "" || name == "" {
		return kind + "标识符和名称必填"
	}
	if tslReserved[id] {
		return kind + "标识符不能使用保留字: " + id
	}
	if seen[id] {
		return kind + "标识符重复: " + id
	}
	seen[id] = true
	return ""
}

// ExportThingModel 导出产品物模型为 JSON 文件
func ExportThingModel(c *gin.Context) {
	p, err := canViewProduct(c, c.Param("id"))
	if err != nil {
		Fail(c, 404, "产品不存在")
		return
	}
	var tm model.ThingModel
	if err := repository.DB.Where("product_id = ?", p.ID).First(&tm).Error; err != nil {
		tm = model.ThingModel{ProductID: p.ID, Properties: []byte("[]"), Events: []byte("[]"), Services: []byte("[]")}
	}

	exportData := gin.H{
		"productId": p.ProductKey,
		"productName": p.Name,
		"properties": json.RawMessage(tm.Properties),
		"events":     json.RawMessage(tm.Events),
		"services":   json.RawMessage(tm.Services),
	}
	body, _ := json.MarshalIndent(exportData, "", "  ")
	c.Header("Content-Disposition", "attachment; filename=tsl_"+p.ProductKey+".json")
	c.Data(http.StatusOK, "application/json", body)
}

// ImportThingModel 从上传的 JSON 文件导入物模型
func ImportThingModel(c *gin.Context) {
	p, err := mustOwnProduct(c, c.Param("id"))
	if err != nil {
		Fail(c, 404, "产品不存在")
		return
	}

	file, _, err := c.Request.FormFile("file")
	if err != nil {
		Fail(c, 400, "请上传文件")
		return
	}
	defer file.Close()
	body, err := io.ReadAll(file)
	if err != nil {
		Fail(c, 400, "读取文件失败")
		return
	}

	var req struct {
		Properties []tslProperty `json:"properties"`
		Events     []tslEvent    `json:"events"`
		Services   []tslService  `json:"services"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		Fail(c, 400, "JSON 格式错误: "+err.Error())
		return
	}

	// 复用 SaveThingModel 的校验逻辑
	seen := map[string]bool{}
	for _, prop := range req.Properties {
		if msg := validIdentifier(prop.Identifier, prop.Name, "属性", seen); msg != "" {
			Fail(c, 400, msg)
			return
		}
		if !tslDataTypes[prop.DataType] {
			Fail(c, 400, "属性["+prop.Identifier+"]数据类型非法: "+prop.DataType)
			return
		}
	}
	eseen := map[string]bool{}
	for _, e := range req.Events {
		if msg := validIdentifier(e.Identifier, e.Name, "事件", eseen); msg != "" {
			Fail(c, 400, msg)
			return
		}
	}
	sseen := map[string]bool{}
	for _, s := range req.Services {
		if msg := validIdentifier(s.Identifier, s.Name, "服务", sseen); msg != "" {
			Fail(c, 400, msg)
			return
		}
	}

	props, _ := json.Marshal(req.Properties)
	events, _ := json.Marshal(req.Events)
	svcs, _ := json.Marshal(req.Services)

	var tm model.ThingModel
	if err := repository.DB.Where("product_id = ?", p.ID).First(&tm).Error; err != nil {
		tm = model.ThingModel{ProductID: p.ID, Properties: props, Events: events, Services: svcs}
		repository.DB.Create(&tm)
	} else {
		repository.DB.Model(&tm).Updates(map[string]interface{}{
			"properties": props, "events": events, "services": svcs,
		})
	}
	OK(c, tm)
}
