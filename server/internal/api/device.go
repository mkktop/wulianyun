package api

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"iot-platform/internal/gateway"
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
	// 设备名不能以点号开头：ClientID 为产品ID+设备名纯拼接（无分隔符），
	// 前导点会让 ClientID 形如 {产品ID}.{设备名}，易被误认为是分隔符
	if strings.HasPrefix(req.Name, ".") {
		Fail(c, 400, "设备名称不能以点号开头")
		return
	}
	if msg := validateDeviceName(req.Name); msg != "" {
		Fail(c, 400, msg)
		return
	}
	p, err := canViewProduct(c, req.ProductID)
	if err != nil {
		Fail(c, 404, "产品不存在或未下放")
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

// validateDeviceName 设备名字符校验：禁止 MQTT 主题通配符与路径分隔符——
// 设备名会进入 topic（thing/up/{pk}/{dn}），含 / 会让同产品设备 A("x") 的 ACL
// 放行设备 B("x/y") 的主题前缀，造成跨设备主题混淆
func validateDeviceName(name string) string {
	if strings.ContainsAny(name, "/+#") {
		return "设备名称不能包含 / + # 字符"
	}
	return ""
}

// @Summary      设备列表
// @Description  分页查询当前账号可见的设备，支持按产品/状态/分组过滤并补全产品与分组名
// @Tags         设备
// @Produce      json
// @Param        productId query int false "产品ID"
// @Param        keyword query string false "设备名称模糊关键字"
// @Param        status query string false "设备状态(online/offline/inactive/disabled)"
// @Param        groupId query int false "分组ID"
// @Param        page query int false "页码"
// @Param        size query int false "每页数量"
// @Success      200  {object}  Resp
// @Failure      400  {object}  Resp
// @Router       /devices [get]
// @Security     BearerAuth
func ListDevices(c *gin.Context) {
	q := repository.DB.Model(&model.Device{}).Scopes(ownedScope(c, "devices.user_id"))
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
	// 标签过滤：tags 为 jsonb 数组，包含匹配（tags @> ["tag"]）；字段名走参数绑定防注入
	if tag := c.Query("tag"); tag != "" {
		tagJSON, _ := json.Marshal([]string{tag})
		q = q.Where("tags @> CAST(? AS jsonb)", string(tagJSON))
	}
	var total int64
	q.Count(&total)
	page, size := pageArgs(c)
	var list []model.Device
	if err := q.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&list).Error; err != nil {
		Fail(c, 500, "查询失败")
		return
	}
	// 批量补充产品与分组名称（IN 查询，替代逐行 First 的 N+1）
	if len(list) > 0 {
		pidSet := map[uint]bool{}
		gidSet := map[uint]bool{}
		for i := range list {
			pidSet[list[i].ProductID] = true
			if list[i].GroupID > 0 {
				gidSet[list[i].GroupID] = true
			}
		}
		pids := make([]uint, 0, len(pidSet))
		for id := range pidSet {
			pids = append(pids, id)
		}
		var products []model.Product
		repository.DB.Select("id, name").Where("id IN ?", pids).Find(&products)
		pidNames := make(map[uint]string, len(products))
		for _, p := range products {
			pidNames[p.ID] = p.Name
		}
		gidNames := map[uint]string{}
		if len(gidSet) > 0 {
			gids := make([]uint, 0, len(gidSet))
			for id := range gidSet {
				gids = append(gids, id)
			}
			var groups []model.DeviceGroup
			repository.DB.Select("id, name").Where("id IN ?", gids).Find(&groups)
			for _, g := range groups {
				gidNames[g.ID] = g.Name
			}
		}
		for i := range list {
			list[i].ProductName = pidNames[list[i].ProductID]
			list[i].GroupName = gidNames[list[i].GroupID]
			// 列表不打码设备密钥会被整页导出（浏览器缓存/截图扩大泄露面）；
			// 密钥在详情页"显示密钥"操作中按需获取
			list[i].Secret = ""
		}
	}
	OK(c, PageData{Total: total, List: list})
}

// @Summary      设备详情
// @Description  获取指定设备（附带产品名）
// @Tags         设备
// @Produce      json
// @Param        id path int true "设备ID"
// @Success      200  {object}  Resp
// @Failure      400  {object}  Resp
// @Router       /devices/{id} [get]
// @Security     BearerAuth
func GetDevice(c *gin.Context) {
	var d model.Device
	if err := repository.DB.Scopes(ownedScope(c, "")).Where("id = ?", c.Param("id")).First(&d).Error; err != nil {
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
	if err := repository.DB.Scopes(ownedScope(c, "")).Where("id = ?", c.Param("id")).First(&d).Error; err != nil {
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
	if req.GroupID != nil && *req.GroupID > 0 {
		// 分组归属校验：只能把设备移入自己可见的分组（防跨租户分组名泄露/移入他人分组）
		var g model.DeviceGroup
		if err := repository.DB.Scopes(ownedScope(c, "")).Where("id = ?", *req.GroupID).First(&g).Error; err != nil {
			Fail(c, 400, "分组不存在或无权访问")
			return
		}
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
	if err := repository.DB.Scopes(ownedScope(c, "")).Where("id = ?", c.Param("id")).First(&d).Error; err != nil {
		Fail(c, 404, "设备不存在")
		return
	}
	// 先踢 TCP 会话：连接关闭后 handleConn 的 defer 走既有下线路径
	// （置离线、停 Modbus 轮询、记录日志），否则已删设备的会话与轮询会一直残留空转
	gateway.KickDevice(d.ProductKey, d.Name)
	repository.DB.Delete(&d)
	repository.DB.Where("device_id = ?", d.ID).Delete(&model.DeviceEvent{})
	repository.DB.Where("device_id = ?", d.ID).Delete(&model.DeviceShadow{})
	service.InvalidateDeviceCache(d.ProductKey, d.Name)
	service.InvalidateShadowCache(d.ID)
	// 清理 Redis 最新值缓存
	if repository.RDB != nil {
		_ = repository.RDB.Del(context.Background(), "device:latest:"+strconv.FormatUint(uint64(d.ID), 10)).Err()
	}
	OK(c, nil)
}

// ListDeviceEvents 设备上下线事件日志
// @Summary      设备事件列表
// @Description  分页查询指定设备的上下线事件日志
// @Tags         设备
// @Produce      json
// @Param        id path int true "设备ID"
// @Param        page query int false "页码"
// @Param        size query int false "每页数量"
// @Success      200  {object}  Resp
// @Failure      400  {object}  Resp
// @Router       /devices/{id}/events [get]
// @Security     BearerAuth
func ListDeviceEvents(c *gin.Context) {
	var d model.Device
	if err := repository.DB.Scopes(ownedScope(c, "")).Where("id = ?", c.Param("id")).First(&d).Error; err != nil {
		Fail(c, 404, "设备不存在")
		return
	}
	q := repository.DB.Model(&model.DeviceEvent{}).Where("device_id = ?", d.ID)
	var total int64
	q.Count(&total)
	page, size := pageArgs(c)
	var list []model.DeviceEvent
	q.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&list)
	OK(c, PageData{Total: total, List: list})
}

// ListSubDevices 获取网关的子设备列表
func ListSubDevices(c *gin.Context) {
	gatewayID := c.Param("id")
	var list []model.Device
	repository.DB.Scopes(ownedScope(c, "")).Where("gateway_id = ?", gatewayID).Find(&list)
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
	if err := repository.DB.Scopes(ownedScope(c, "")).Where("id = ?", gatewayID).First(&gw).Error; err != nil {
		Fail(c, 404, "网关设备不存在")
		return
	}
	repository.DB.Model(&model.Device{}).Scopes(ownedScope(c, "")).Where("id = ?", req.DeviceID).Update("gateway_id", gw.ID)
	OK(c, nil)
}

// RemoveSubDevice 从网关移除子设备
func RemoveSubDevice(c *gin.Context) {
	gatewayID := c.Param("id")
	subID := c.Param("subId")
	repository.DB.Model(&model.Device{}).Scopes(ownedScope(c, "")).Where("id = ? AND gateway_id = ?", subID, gatewayID).Update("gateway_id", nil)
	OK(c, nil)
}

// BatchUpdateDevices 批量操作设备：{ids:[], action: disable|enable|delete|moveGroup, groupId?}
// 逐台按 ownedScope 圈定后处理，返回成功/失败清单；单次上限 200 台。
func BatchUpdateDevices(c *gin.Context) {
	var req struct {
		IDs     []uint  `json:"ids" binding:"required"`
		Action  string  `json:"action" binding:"required,oneof=disable enable delete moveGroup"`
		GroupID *uint   `json:"groupId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 {
		Fail(c, 400, "参数错误：ids 与 action 必填")
		return
	}
	if len(req.IDs) > 200 {
		Fail(c, 400, "单次最多操作 200 台设备")
		return
	}
	if req.Action == "moveGroup" {
		if req.GroupID == nil || *req.GroupID == 0 {
			Fail(c, 400, "moveGroup 需要 groupId")
			return
		}
		var g model.DeviceGroup
		if err := repository.DB.Scopes(ownedScope(c, "")).Where("id = ?", *req.GroupID).First(&g).Error; err != nil {
			Fail(c, 400, "分组不存在或无权访问")
			return
		}
	}
	var devices []model.Device
	if err := repository.DB.Scopes(ownedScope(c, "")).Where("id IN ?", req.IDs).Find(&devices).Error; err != nil {
		Fail(c, 500, "查询失败")
		return
	}
	owned := make(map[uint]bool, len(devices))
	for _, d := range devices {
		owned[d.ID] = true
	}
	skipped := make([]uint, 0)
	for _, id := range req.IDs {
		if !owned[id] {
			skipped = append(skipped, id) // 不存在或越权：跳过不报错，结果里如实体现
		}
	}
	failed := make(map[uint]string, 0)
	done := make([]uint, 0, len(devices))
	for i := range devices {
		d := &devices[i]
		var err error
		switch req.Action {
		case "disable":
			err = repository.DB.Model(d).Update("status", model.DeviceStatusDisabled).Error
		case "enable":
			if d.Status != model.DeviceStatusDisabled {
				done = append(done, d.ID) // 非禁用态启用是幂等操作，直接计成功
				continue
			}
			err = repository.DB.Model(d).Update("status", model.DeviceStatusOffline).Error
		case "moveGroup":
			err = repository.DB.Model(d).Update("group_id", *req.GroupID).Error
		case "delete":
			// 与 DeleteDevice 相同的清理序列：踢 TCP 会话→级联删事件/影子→清缓存
			gateway.KickDevice(d.ProductKey, d.Name)
			err = repository.DB.Delete(d).Error
			if err == nil {
				repository.DB.Where("device_id = ?", d.ID).Delete(&model.DeviceEvent{})
				repository.DB.Where("device_id = ?", d.ID).Delete(&model.DeviceShadow{})
				service.InvalidateShadowCache(d.ID)
				if repository.RDB != nil {
					_ = repository.RDB.Del(context.Background(), "device:latest:"+strconv.FormatUint(uint64(d.ID), 10)).Err()
				}
			}
		}
		if err != nil {
			failed[d.ID] = err.Error()
			continue
		}
		service.InvalidateDeviceCache(d.ProductKey, d.Name)
		done = append(done, d.ID)
	}
	OK(c, gin.H{"success": len(done), "skipped": skipped, "failed": failed})
}
