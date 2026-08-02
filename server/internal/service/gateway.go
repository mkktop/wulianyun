package service

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"iot-platform/internal/model"
	"iot-platform/internal/repository"
	"iot-platform/internal/ws"
)

// HandleGatewaySubDevice 处理网关代理的子设备上下线
// Topic: thing/gateway/{pk}/{dn}/sub/{subId}/login   或 thing/gateway/{pk}/{dn}/sub/{subId}/logout
// Payload: {"productKey":"...","deviceName":"...","secret":"...","timestamp":...}
func HandleGatewaySubDevice(topic string, payload []byte) {
	parts := strings.Split(topic, "/")
	// thing/gateway/{pk}/{gatewayDn}/sub/{subId}/{login|logout}
	if len(parts) != 7 {
		return
	}
	gatewayPK := parts[2]
	gatewayDN := parts[3]
	subID := parts[5]
	action := parts[6] // login / logout

	var req struct {
		ProductKey string `json:"productKey"`
		DeviceName string `json:"deviceName"`
		Secret     string `json:"secret"`
		Timestamp  int64  `json:"timestamp"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		slog.Warn("gateway sub-device parse failed", "topic", topic, "error", err)
		return
	}

	// 查找网关设备
	gw, err := FindDevice(gatewayPK, gatewayDN)
	if err != nil {
		slog.Warn("gateway device not found", "pk", gatewayPK, "dn", gatewayDN)
		return
	}
	if !gw.IsGateway {
		slog.Warn("device is not a gateway", "pk", gatewayPK, "dn", gatewayDN)
		return
	}

	switch action {
	case "login":
		// 子设备上线：验证 + 绑定到网关
		sub, err := FindDeviceForAuth(req.ProductKey, req.DeviceName, req.Secret)
		if err != nil {
			slog.Warn("gateway sub-device auth failed", "subId", subID, "error", err)
			return
		}
		gwID := gw.ID
		repository.DB.Model(sub).Update("gateway_id", gwID)
		repository.DB.Model(&model.Device{}).Where("id = ?", sub.ID).Update("status", model.DeviceStatusOnline)

		slog.Info("gateway sub-device online", "gateway", gatewayDN, "subDevice", req.DeviceName, "subId", subID)
		ws.H.PushDeviceStatus(gw.UserID, sub.ID, map[string]interface{}{
			"deviceId": sub.ID, "name": req.DeviceName, "status": "online",
			"via": "gateway", "gateway": gatewayDN,
		})

	case "logout":
		sub, err := FindDevice(req.ProductKey, req.DeviceName)
		if err != nil {
			return
		}
		repository.DB.Model(&model.Device{}).Where("id = ?", sub.ID).Update("status", model.DeviceStatusOffline)
		slog.Info("gateway sub-device offline", "gateway", gatewayDN, "subDevice", req.DeviceName, "subId", subID)
		ws.H.PushDeviceStatus(gw.UserID, sub.ID, map[string]interface{}{
			"deviceId": sub.ID, "name": req.DeviceName, "status": "offline",
			"via": "gateway", "gateway": gatewayDN,
		})
	}
}

// PublishShadowRetained 将设备影子期望值作为 Retained 消息发布到 MQTT
// 设备上线后从 broker 立即获取最新期望值，无需等待后端主动推送
func PublishShadowRetained(d *model.Device, desired map[string]interface{}) error {
	if DownPublisher == nil {
		return nil
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"method":   "property.set",
		"params":   desired,
		"retained": true,
		"ts":       fmt.Sprintf("%d", time.Now().UnixMilli()),
	})
	return DownPublisher(d.ProductKey, d.Name, payload)
}
