package api

import (
	"encoding/json"

	"github.com/gin-gonic/gin"

	"iot-platform/internal/model"
	"iot-platform/internal/repository"
	"iot-platform/internal/service"
)

type deviceReq struct {
	ProductID uint   `json:"productId" binding:"required"`
	Name      string `json:"name" binding:"required,max=64"`
	Remark    string `json:"remark"`
	RegCode   string `json:"regCode"` // 自定义注册码（IMEI/ICCID 等）
}

// regCodeTaken 注册码是否已被其他设备占用
func regCodeTaken(regCode string, excludeID uint) bool {
	if regCode == "" {
		return false
	}
	var cnt int64
	repository.DB.Model(&model.Device{}).
		Where("reg_code = ? AND id <> ?", regCode, excludeID).Count(&cnt)
	return cnt > 0
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
	if regCodeTaken(req.RegCode, 0) {
		Fail(c, 400, "注册码已被占用")
		return
	}
	d := model.Device{
		UserID: UID(c), ProductID: p.ID, ProductKey: p.ProductKey,
		Name: req.Name, Secret: randHex(16), RegCode: req.RegCode,
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
	if gid := c.Query("groupId"); gid != "" {
		q = q.Where("group_id = ?", gid)
	}
	var total int64
	q.Count(&total)
	page, size := pageArgs(c)
	var list []model.Device
	if err := q.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&list).Error; err != nil {
		Fail(c, 500, "查询失败")
		return
	}
	// 补充产品与分组名称
	pidNames := map[uint]string{}
	gidNames := map[uint]string{}
	for i := range list {
		if name, ok := pidNames[list[i].ProductID]; ok {
			list[i].ProductName = name
		} else {
			var p model.Product
			if repository.DB.Select("name").First(&p, list[i].ProductID).Error == nil {
				pidNames[list[i].ProductID] = p.Name
				list[i].ProductName = p.Name
			}
		}
		if list[i].GroupID > 0 {
			if name, ok := gidNames[list[i].GroupID]; ok {
				list[i].GroupName = name
			} else {
				var g model.DeviceGroup
				if repository.DB.Select("name").First(&g, list[i].GroupID).Error == nil {
					gidNames[list[i].GroupID] = g.Name
					list[i].GroupName = g.Name
				}
			}
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
		Remark  string          `json:"remark"`
		Disable *bool           `json:"disable"`
		GroupID *uint           `json:"groupId"`
		Tags    []string        `json:"tags"`
		RegCode *string         `json:"regCode"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, 400, "参数错误")
		return
	}
	updates := map[string]interface{}{"remark": req.Remark}
	if req.RegCode != nil {
		if regCodeTaken(*req.RegCode, d.ID) {
			Fail(c, 400, "注册码已被占用")
			return
		}
		updates["reg_code"] = *req.RegCode
	}
	if req.GroupID != nil {
		updates["group_id"] = *req.GroupID
	}
	if req.Tags != nil {
		tags, _ := json.Marshal(req.Tags)
		updates["tags"] = tags
	}
	if req.Disable != nil {
		if *req.Disable {
			updates["status"] = model.DeviceStatusDisabled
		} else if d.Status == model.DeviceStatusDisabled {
			updates["status"] = model.DeviceStatusOffline
		}
	}
	repository.DB.Model(&d).Updates(updates)
	service.InvalidateDeviceCache(d.ProductKey, d.Name)
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
	service.InvalidateDeviceCache(d.ProductKey, d.Name)
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

// ListSubDevices 获取网关的子设备列表
func ListSubDevices(c *gin.Context) {
	gatewayID := c.Param("id")
	var list []model.Device
	repository.DB.Where("gateway_id = ? AND user_id = ?", gatewayID, UID(c)).Find(&list)
	OK(c, list)
}

// AddSubDevice 添加子设备到网关
func AddSubDevice(c *gin.Context) {
	gatewayID := c.Param("id")
	var req struct {
		DeviceID uint `json:"deviceId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, 400, err.Error())
		return
	}
	var gw model.Device
	if err := repository.DB.Where("id = ? AND user_id = ?", gatewayID, UID(c)).First(&gw).Error; err != nil {
		Fail(c, 404, "网关设备不存在")
		return
	}
	repository.DB.Model(&model.Device{}).Where("id = ? AND user_id = ?", req.DeviceID, UID(c)).Update("gateway_id", gw.ID)
	OK(c, nil)
}

// RemoveSubDevice 从网关移除子设备
func RemoveSubDevice(c *gin.Context) {
	gatewayID := c.Param("id")
	subID := c.Param("subId")
	repository.DB.Model(&model.Device{}).Where("id = ? AND gateway_id = ? AND user_id = ?", subID, gatewayID, UID(c)).Update("gateway_id", nil)
	OK(c, nil)
}
