package api

import (
	"github.com/gin-gonic/gin"

	"iot-platform/internal/model"
	"iot-platform/internal/repository"
)

// ListModbusGroups 查询产品的 Modbus 采集组
func ListModbusGroups(c *gin.Context) {
	p, err := canViewProduct(c, c.Param("id"))
	if err != nil {
		Fail(c, 404, "产品不存在")
		return
	}
	var groups []model.ModbusGroup
	repository.DB.Where("product_id = ?", p.ID).Order("id asc").Find(&groups)
	for i := range groups {
		repository.DB.Model(&model.ModbusPoint{}).Where("group_id = ?", groups[i].ID).Count(&groups[i].PointCount)
	}
	OK(c, groups)
}

type modbusGroupReq struct {
	Name         string `json:"name" binding:"required,max=64"`
	PollInterval int    `json:"pollInterval"`
	ReportMode   string `json:"reportMode"`
}

func normalizeGroupReq(r *modbusGroupReq) {
	if r.PollInterval < 1 {
		r.PollInterval = 60
	}
	if r.ReportMode != model.ReportModeOnChange {
		r.ReportMode = model.ReportModePeriodic
	}
}

// CreateModbusGroup 创建采集组
func CreateModbusGroup(c *gin.Context) {
	p, err := mustOwnProduct(c, c.Param("id"))
	if err != nil {
		Fail(c, 404, "产品不存在")
		return
	}
	var req modbusGroupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, 400, "分组名称必填")
		return
	}
	normalizeGroupReq(&req)
	g := model.ModbusGroup{
		ProductID: p.ID, Name: req.Name,
		PollInterval: req.PollInterval, ReportMode: req.ReportMode,
	}
	if err := repository.DB.Create(&g).Error; err != nil {
		Fail(c, 500, "创建失败")
		return
	}
	refreshModbusDevice(p) // 新组立即生效（#22）
	OK(c, g)
}

// UpdateModbusGroup 更新采集组
func UpdateModbusGroup(c *gin.Context) {
	var g model.ModbusGroup
	if err := joinGroupOwner(c, &g); err != nil {
		Fail(c, 404, "分组不存在")
		return
	}
	var req modbusGroupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, 400, "参数错误")
		return
	}
	normalizeGroupReq(&req)
	repository.DB.Model(&g).Updates(map[string]interface{}{
		"name": req.Name, "poll_interval": req.PollInterval, "report_mode": req.ReportMode,
	})
	refreshModbusDevice(loadProductForRefresh(g.ProductID))
	OK(c, g)
}

// DeleteModbusGroup 删除采集组，组内点位归入默认组(0)
func DeleteModbusGroup(c *gin.Context) {
	var g model.ModbusGroup
	if err := joinGroupOwner(c, &g); err != nil {
		Fail(c, 404, "分组不存在")
		return
	}
	repository.DB.Delete(&g)
	repository.DB.Model(&model.ModbusPoint{}).Where("group_id = ?", g.ID).Update("group_id", 0)
	refreshModbusDevice(loadProductForRefresh(g.ProductID))
	OK(c, nil)
}

// loadProductForRefresh 加载产品（组变更后刷新轮询用）
func loadProductForRefresh(productID uint) *model.Product {
	var p model.Product
	repository.DB.First(&p, productID)
	return &p
}

// joinGroupOwner 校验分组归属当前用户（经产品关联）。采集组属产品定义，仅 owner/超管可改。
func joinGroupOwner(c *gin.Context, g *model.ModbusGroup) error {
	q := repository.DB.
		Joins("JOIN products ON products.id = modbus_groups.product_id").
		Where("modbus_groups.id = ?", c.Param("gid"))
	if !IsAdmin(c) {
		q = q.Where("products.user_id = ?", UID(c))
	}
	return q.First(g).Error
}
