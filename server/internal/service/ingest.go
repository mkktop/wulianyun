package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"iot-platform/internal/model"
	"iot-platform/internal/repository"
	"iot-platform/internal/rule"
	"iot-platform/internal/ws"
)

// FindDevice 按三元组标识查设备
func FindDevice(productKey, deviceName string) (*model.Device, error) {
	var d model.Device
	err := repository.DB.Where("product_key = ? AND name = ?", productKey, deviceName).First(&d).Error
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// ParseClientID clientid 约定为 {productKey}.{deviceName}
func ParseClientID(clientID string) (productKey, deviceName string, ok bool) {
	parts := strings.SplitN(clientID, ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// HandleTelemetry 处理设备上行遥测（JSON 对象）
func HandleTelemetry(productKey, deviceName string, payload []byte) {
	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err != nil || len(data) == 0 {
		slog.Warn("invalid telemetry payload", "productKey", productKey, "device", deviceName)
		return
	}
	d, err := FindDevice(productKey, deviceName)
	if err != nil {
		slog.Warn("telemetry from unknown device", "productKey", productKey, "device", deviceName)
		return
	}

	now := time.Now()
	t := model.Telemetry{Ts: now, DeviceID: d.ID, ProductKey: productKey, DeviceName: deviceName, Data: payload}
	if err := repository.DB.Create(&t).Error; err != nil {
		slog.Error("save telemetry failed", "err", err)
		return
	}

	// 最新值缓存
	cache := map[string]interface{}{"ts": now.UnixMilli(), "data": data}
	if b, err := json.Marshal(cache); err == nil {
		repository.RDB.Set(context.Background(), latestKey(d.ID), b, 0)
	}

	// 实时推送
	ws.H.PushTelemetry(d.UserID, d.ID, map[string]interface{}{"ts": now.UnixMilli(), "data": data})

	// 同步设备影子 reported + 规则引擎评估
	mergeShadowReported(d, data)
	rule.EvalTelemetry(d, data)
}

// HandleDeviceStatus 处理上下线事件
func HandleDeviceStatus(clientID string, online bool) {
	productKey, deviceName, ok := ParseClientID(clientID)
	if !ok {
		return
	}
	d, err := FindDevice(productKey, deviceName)
	if err != nil {
		return
	}
	now := time.Now()
	status := model.DeviceStatusOffline
	eventType := "offline"
	updates := map[string]interface{}{"last_offline_at": now}
	if online {
		status = model.DeviceStatusOnline
		eventType = "online"
		updates = map[string]interface{}{"last_online_at": now}
	}
	// 禁用设备不改状态（EMQX 鉴权阶段已拒绝，这里兜底）
	if d.Status == model.DeviceStatusDisabled {
		return
	}
	updates["status"] = status
	repository.DB.Model(&model.Device{}).Where("id = ?", d.ID).Updates(updates)
	repository.DB.Create(&model.DeviceEvent{DeviceID: d.ID, Type: eventType, Detail: "clientid: " + clientID})

	// 上线时补发影子期望值
	if online {
		syncShadowOnConnect(d)
	}

	ws.H.PushDeviceStatus(d.UserID, d.ID, map[string]interface{}{
		"deviceId": d.ID, "name": d.Name, "status": status, "ts": now.UnixMilli(),
	})
}

// GetLatest 读取设备最新遥测（Redis 缓存优先，回退数据库）
func GetLatest(deviceID uint) map[string]interface{} {
	if b, err := repository.RDB.Get(context.Background(), latestKey(deviceID)).Bytes(); err == nil {
		var m map[string]interface{}
		if json.Unmarshal(b, &m) == nil {
			return m
		}
	}
	var t model.Telemetry
	if err := repository.DB.Where("device_id = ?", deviceID).Order("ts desc").First(&t).Error; err != nil {
		return nil
	}
	var data map[string]interface{}
	json.Unmarshal(t.Data, &data)
	return map[string]interface{}{"ts": t.Ts.UnixMilli(), "data": data}
}

func latestKey(deviceID uint) string {
	return "device:latest:" + strconv.FormatUint(uint64(deviceID), 10)
}
