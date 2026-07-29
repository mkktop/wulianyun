package service

import (
	"encoding/json"
	"log/slog"
	"time"

	"iot-platform/internal/model"
	"iot-platform/internal/repository"
)

// DownPublisher 由 mqtt 包注入，避免循环依赖
var DownPublisher func(productKey, deviceName string, payload []byte) error

// GetShadow 获取设备影子（无则初始化）
func GetShadow(deviceID uint) *model.DeviceShadow {
	var s model.DeviceShadow
	if err := repository.DB.Where("device_id = ?", deviceID).First(&s).Error; err != nil {
		s = model.DeviceShadow{DeviceID: deviceID, Desired: []byte("{}"), Reported: []byte("{}")}
		repository.DB.Create(&s)
	}
	return &s
}

// UpdateShadowDesired 合并期望值；设备在线立即下发，离线等上线补发
func UpdateShadowDesired(d *model.Device, desired map[string]interface{}) (*model.DeviceShadow, error) {
	s := GetShadow(d.ID)
	merged := map[string]interface{}{}
	json.Unmarshal(s.Desired, &merged)
	for k, v := range desired {
		merged[k] = v
	}
	data, _ := json.Marshal(merged)
	repository.DB.Model(s).Updates(map[string]interface{}{
		"desired": data, "version": s.Version + 1,
	})
	s.Desired = data
	s.Version++

	if d.Status == model.DeviceStatusOnline {
		pushDesired(d, merged)
	}
	return s, nil
}

// mergeShadowReported 上行遥测同步到影子 reported，并清除已达成的 desired
func mergeShadowReported(d *model.Device, data map[string]interface{}) {
	s := GetShadow(d.ID)
	reported := map[string]interface{}{}
	json.Unmarshal(s.Reported, &reported)
	for k, v := range data {
		reported[k] = v
	}
	// 设备已上报到期望值的项，从 desired 移除
	desired := map[string]interface{}{}
	json.Unmarshal(s.Desired, &desired)
	changed := false
	for k, dv := range desired {
		if rv, ok := data[k]; ok && jsonEqual(rv, dv) {
			delete(desired, k)
			changed = true
		}
	}
	rb, _ := json.Marshal(reported)
	updates := map[string]interface{}{"reported": rb}
	if changed {
		db, _ := json.Marshal(desired)
		updates["desired"] = db
	}
	repository.DB.Model(s).Updates(updates)
}

// syncShadowOnConnect 设备上线时补发未达成的期望值
func syncShadowOnConnect(d *model.Device) {
	s := GetShadow(d.ID)
	desired := map[string]interface{}{}
	json.Unmarshal(s.Desired, &desired)
	if len(desired) > 0 {
		pushDesired(d, desired)
	}
}

func pushDesired(d *model.Device, desired map[string]interface{}) {
	if DownPublisher == nil {
		return
	}
	msg, _ := json.Marshal(map[string]interface{}{
		"method": "property.set",
		"params": desired,
		"ts":     time.Now().UnixMilli(),
	})
	if err := DownPublisher(d.ProductKey, d.Name, msg); err != nil {
		slog.Warn("push shadow desired failed", "device", d.Name, "err", err)
	}
}

func jsonEqual(a, b interface{}) bool {
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return string(ab) == string(bb)
}
