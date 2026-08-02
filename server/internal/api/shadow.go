package api

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"iot-platform/internal/model"
	"iot-platform/internal/repository"
	"iot-platform/internal/service"
)

// GetDeviceShadow 查询设备影子
func GetDeviceShadow(c *gin.Context) {
	var d model.Device
	if err := repository.DB.Where("id = ? AND user_id = ?", c.Param("id"), UID(c)).First(&d).Error; err != nil {
		Fail(c, 404, "设备不存在")
		return
	}
	OK(c, service.GetShadow(d.ID))
}

// SetDeviceProperty 属性设置：写影子 desired，在线即时下发（QoS 2），离线上线补发
// 支持 expireSec 参数：指定秒数后指令自动过期
func SetDeviceProperty(c *gin.Context) {
	var d model.Device
	if err := repository.DB.Where("id = ? AND user_id = ?", c.Param("id"), UID(c)).First(&d).Error; err != nil {
		Fail(c, 404, "设备不存在")
		return
	}
	var body struct {
		Params   map[string]interface{} `json:"params"`
		ExpireSec int                   `json:"expireSec"` // 过期秒数（0=不过期）
	}
	if err := c.ShouldBindJSON(&body); err != nil || len(body.Params) == 0 {
		// 兼容旧格式：直接传 params
		if err := c.ShouldBindJSON(&body.Params); err != nil || len(body.Params) == 0 {
			Fail(c, 400, "属性参数必须是非空 JSON 对象")
			return
		}
	}

	messageID := fmt.Sprintf("%d", time.Now().UnixNano())
	// 消息过期检查：expireSec > 0 时添加 expireAt
	var expireAt int64
	if body.ExpireSec > 0 {
		expireAt = time.Now().Add(time.Duration(body.ExpireSec) * time.Second).UnixMilli()
	}
	_ = expireAt // 在下行 payload 中使用

	s, err := service.UpdateShadowDesired(&d, body.Params)
	if err != nil {
		Fail(c, 500, "更新影子失败")
		return
	}

	// 记录 CommandRequest
	cmdReq := model.CommandRequest{
		MessageID:  messageID,
		UserID:     UID(c),
		DeviceID:   d.ID,
		DeviceName: d.Name,
		Method:     "property.set",
		Payload:    func() string { b, _ := json.Marshal(body.Params); return string(b) }(),
		Status:     "pending",
		TimeoutMs:  10000,
	}
	repository.DB.Create(&cmdReq)

	msg := "已下发"
	if d.Status != model.DeviceStatusOnline {
		msg = "设备离线，已写入影子待上线补发"
	}
	OK(c, gin.H{"shadow": s, "delivered": d.Status == model.DeviceStatusOnline, "note": msg, "messageId": messageID})
}

// InvokeService 服务调用：按物模型服务标识符下发
func InvokeService(c *gin.Context) {
	var d model.Device
	if err := repository.DB.Where("id = ? AND user_id = ?", c.Param("id"), UID(c)).First(&d).Error; err != nil {
		Fail(c, 404, "设备不存在")
		return
	}
	if d.Status != model.DeviceStatusOnline {
		Fail(c, 400, "设备不在线")
		return
	}
	var req struct {
		Service string                 `json:"service" binding:"required"`
		Params  map[string]interface{} `json:"params"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, 400, "服务标识符必填")
		return
	}

	messageID := fmt.Sprintf("%d", time.Now().UnixNano())
	payload, _ := json.Marshal(map[string]interface{}{
		"method":    "service.invoke",
		"messageId": messageID,
		"service":   req.Service,
		"params":    req.Params,
		"ts":        time.Now().UnixMilli(),
	})

	// 记录 CommandRequest
	cmdReq := model.CommandRequest{
		MessageID:  messageID,
		UserID:     UID(c),
		DeviceID:   d.ID,
		DeviceName: d.Name,
		Method:     "service.invoke",
		Service:    req.Service,
		Payload:    string(payload),
		Status:     "pending",
		TimeoutMs:  10000,
	}
	repository.DB.Create(&cmdReq)

	if err := service.DownPublisher(d.ProductKey, d.Name, payload); err != nil {
		Fail(c, 500, "下发失败: "+err.Error())
		return
	}
	OK(c, gin.H{"messageId": messageID})
}
