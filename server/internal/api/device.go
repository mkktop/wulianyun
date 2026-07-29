package api

import (
	"github.com/gin-gonic/gin"

	"iot-platform/internal/model"
	"iot-platform/internal/repository"
)

type deviceReq struct {
	ProductID uint   `json:"productId" binding:"required"`
	Name      string `json:"name" binding:"required,max=64"`
	Remark    string `json:"remark"`
}

func CreateDevice(c *gin.Context) {
	var req deviceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, 400, "产品和设备名称必填")
		return
	}
	var p model.Product
	if err := repository.DB.Where("id = ? AND user_id = ?", req.ProductID, UID(c)).First(&p).Error; err != nil {
		Fail(c, 404, "产品不存在")
		return
	}
	var cnt int64
	repository.DB.Model(&model.Device{}).Where("product_key = ? AND name = ?", p.ProductKey, req.Name).Count(&cnt)
	if cnt > 0 {
		Fail(c, 400, "该产品下设备名称已存在")
		return
	}
	d := model.Device{
		UserID: UID(c), ProductID: p.ID, ProductKey: p.ProductKey,
		Name: req.Name, Secret: randHex(16),
		Status: model.DeviceStatusInactive, Remark: req.Remark,
	}
	if err := repository.DB.Create(&d).Error; err != nil {
		Fail(c, 500, "创建失败")
		return
	}
	d.ProductName = p.Name
	OK(c, d)
}

func ListDevices(c *gin.Context) {
	q := repository.DB.Model(&model.Device{}).Where("devices.user_id = ?", UID(c))
	if pid := c.Query("productId"); pid != "" {
		q = q.Where("product_id = ?", pid)
	}
	if kw := c.Query("keyword"); kw != "" {
		q = q.Where("devices.name ILIKE ?", "%"+kw+"%")
	}
	if st := c.Query("status"); st != "" {
		q = q.Where("status = ?", st)
	}
	var total int64
	q.Count(&total)
	page, size := pageArgs(c)
	var list []model.Device
	if err := q.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&list).Error; err != nil {
		Fail(c, 500, "查询失败")
		return
	}
	// 补充产品名称
	pidNames := map[uint]string{}
	for i := range list {
		if name, ok := pidNames[list[i].ProductID]; ok {
			list[i].ProductName = name
			continue
		}
		var p model.Product
		if repository.DB.Select("name").First(&p, list[i].ProductID).Error == nil {
			pidNames[list[i].ProductID] = p.Name
			list[i].ProductName = p.Name
		}
	}
	OK(c, PageData{Total: total, List: list})
}

func GetDevice(c *gin.Context) {
	var d model.Device
	if err := repository.DB.Where("id = ? AND user_id = ?", c.Param("id"), UID(c)).First(&d).Error; err != nil {
		Fail(c, 404, "设备不存在")
		return
	}
	var p model.Product
	if repository.DB.First(&p, d.ProductID).Error == nil {
		d.ProductName = p.Name
	}
	OK(c, d)
}

func UpdateDevice(c *gin.Context) {
	var d model.Device
	if err := repository.DB.Where("id = ? AND user_id = ?", c.Param("id"), UID(c)).First(&d).Error; err != nil {
		Fail(c, 404, "设备不存在")
		return
	}
	var req struct {
		Remark  string `json:"remark"`
		Disable *bool  `json:"disable"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, 400, "参数错误")
		return
	}
	updates := map[string]interface{}{"remark": req.Remark}
	if req.Disable != nil {
		if *req.Disable {
			updates["status"] = model.DeviceStatusDisabled
		} else if d.Status == model.DeviceStatusDisabled {
			updates["status"] = model.DeviceStatusOffline
		}
	}
	repository.DB.Model(&d).Updates(updates)
	OK(c, d)
}

func DeleteDevice(c *gin.Context) {
	var d model.Device
	if err := repository.DB.Where("id = ? AND user_id = ?", c.Param("id"), UID(c)).First(&d).Error; err != nil {
		Fail(c, 404, "设备不存在")
		return
	}
	repository.DB.Delete(&d)
	repository.DB.Where("device_id = ?", d.ID).Delete(&model.DeviceEvent{})
	OK(c, nil)
}

// ListDeviceEvents 设备上下线事件日志
func ListDeviceEvents(c *gin.Context) {
	var d model.Device
	if err := repository.DB.Where("id = ? AND user_id = ?", c.Param("id"), UID(c)).First(&d).Error; err != nil {
		Fail(c, 404, "设备不存在")
		return
	}
	var list []model.DeviceEvent
	repository.DB.Where("device_id = ?", d.ID).Order("id desc").Limit(50).Find(&list)
	OK(c, list)
}
