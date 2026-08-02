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

// SetDeviceProperty 属性设置：写影子 desired，在线即时下发，离线上线补发
func SetDeviceProperty(c *gin.Context) {
	var d model.Device
	if err := repository.DB.Where("id = ? AND user_id = ?", c.Param("id"), UID(c)).First(&d).Error; err != nil {
		Fail(c, 404, "设备不存在")
		return
	}
	var params map[string]interface{}
	if err := c.ShouldBindJSON(&params); err != nil || len(params) == 0 {
		Fail(c, 400, "属性参数必须是非空 JSON 对象")
		return
	}

	messageID := fmt.Sprintf("%d", time.Now().UnixNano())
	s, err := service.UpdateShadowDesired(&d, params)
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
		Payload:    func() string { b, _ := json.Marshal(params); return string(b) }(),
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
