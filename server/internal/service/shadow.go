package service

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"iot-platform/internal/model"
	"iot-platform/internal/repository"
)

// DownPublisher 由 mqtt 包注入，避免循环依赖
var DownPublisher func(productKey, deviceName string, payload []byte) error

// DownRetainedPublisher 发布 Retained 期望值（设备订阅时必达）；payload 为空表示清除
var DownRetainedPublisher func(productKey, deviceName string, payload []byte) error

// GetShadow 获取设备影子（无则初始化）。返回深拷贝，避免调用方并发 marshal 缓存活指针产生 data race（#30）
func GetShadow(deviceID uint) *model.DeviceShadow {
	if s := CachedGetShadow(deviceID); s != nil {
		return cloneShadow(s)
	}
	// 缓存未命中，从 DB 加载并初始化
	var s model.DeviceShadow
	if err := repository.DB.Where("device_id = ?", deviceID).First(&s).Error; err != nil {
		s = model.DeviceShadow{DeviceID: deviceID, Desired: []byte("{}"), Reported: []byte("{}")}
		repository.DB.Create(&s)
	}
	return &s
}

// cloneShadow 深拷贝影子的 Desired/Reported 字节切片，隔离调用方对缓存内容的并发读写
func cloneShadow(s *model.DeviceShadow) *model.DeviceShadow {
	cp := *s
	if s.Desired != nil {
		cp.Desired = append([]byte(nil), s.Desired...)
	}
	if s.Reported != nil {
		cp.Reported = append([]byte(nil), s.Reported...)
	}
	return &cp
}

// UpdateShadowDesired 合并期望值；设备在线立即下发 delta，离线等上线补发
func UpdateShadowDesired(d *model.Device, desired map[string]interface{}) (*model.DeviceShadow, error) {
	entry := getShadowEntry(d.ID)
	if entry == nil {
		// fallback: 直接查 DB
		s := GetShadow(d.ID)
		oldDesired := map[string]interface{}{}
		if len(s.Desired) > 0 {
			json.Unmarshal(s.Desired, &oldDesired)
		}
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
		// retained 同步：无论设备在线与否都刷新，设备下次订阅必然拿到
		pushDesiredRetained(d, merged)
		if d.Status == model.DeviceStatusOnline {
			delta := computeDelta(oldDesired, merged)
			if len(delta) > 0 {
				pushDesired(d, delta)
			}
		}
		return s, nil
	}

	entry.mu.Lock()
	s := entry.shadow
	oldDesired := map[string]interface{}{}
	if len(s.Desired) > 0 {
		json.Unmarshal(s.Desired, &oldDesired)
	}
	merged := map[string]interface{}{}
	json.Unmarshal(s.Desired, &merged)
	for k, v := range desired {
		merged[k] = v
	}
	data, _ := json.Marshal(merged)
	s.Desired = data
	s.Version++
	entry.dirty = true
	// retained 必须在持锁内发布：解锁后发布会与并发 desired-set 竞态，
	// 后者可能先把新期望写库、本处再用旧 merged 覆盖性发布（甚至清空）（#24）
	pushDesiredRetained(d, merged)
	entry.mu.Unlock()

	// 立即刷盘（不等定时 flush）
	shadowFlushAll()

	if d.Status == model.DeviceStatusOnline {
		delta := computeDelta(oldDesired, merged)
		if len(delta) > 0 {
			pushDesired(d, delta)
		}
	}
	return s, nil
}

// computeDelta 计算 old vs new 的差异，仅返回变化或新增的项
func computeDelta(oldDesired, newDesired map[string]interface{}) map[string]interface{} {
	delta := map[string]interface{}{}
	for k, v := range newDesired {
		oldV, exists := oldDesired[k]
		if !exists || fmt.Sprintf("%v", oldV) != fmt.Sprintf("%v", v) {
			delta[k] = v
		}
	}
	return delta
}

// mergeShadowReported 上行遥测同步到影子 reported，并清除已达成的 desired
func mergeShadowReported(d *model.Device, data map[string]interface{}) {
	entry := getShadowEntry(d.ID)
	if entry == nil {
		// fallback: 直接走 DB
		s := GetShadow(d.ID)
		reported := map[string]interface{}{}
		json.Unmarshal(s.Reported, &reported)
		for k, v := range data {
			reported[k] = v
		}
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
			// 期望达成：刷新 retained（剩余期望或清除）
			pushDesiredRetained(d, desired)
		}
		repository.DB.Model(s).Updates(updates)
		return
	}

	entry.mu.Lock()
	s := entry.shadow
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
	s.Reported = rb
	if changed {
		db, _ := json.Marshal(desired)
		s.Desired = db
		// retained 持锁发布（解锁后发布会与并发 desired-set 竞态，#24）
		pushDesiredRetained(d, desired)
	}
	entry.dirty = true
	entry.mu.Unlock()
}

// syncShadowOnConnect 设备上线时补发未达成的期望值（普通下发 + retained 兜底）
func syncShadowOnConnect(d *model.Device) {
	s := GetShadow(d.ID)
	desired := map[string]interface{}{}
	json.Unmarshal(s.Desired, &desired)
	if len(desired) > 0 {
		pushDesired(d, desired)
		pushDesiredRetained(d, desired)
	}
}

func pushDesired(d *model.Device, desired map[string]interface{}) {
	if DownPublisher == nil {
		return
	}
	msg, _ := json.Marshal(map[string]interface{}{
		"method": "property.set",
		"params": desired,
		"delta":  true,
		"ts":     time.Now().UnixMilli(),
	})
	if err := DownPublisher(d.ProductKey, d.Name, msg); err != nil {
		slog.Warn("push shadow desired failed", "device", d.Name, "err", err)
	}
}

// pushDesiredRetained 把当前期望值发布为 Retained 消息：设备订阅完成时由 broker 直接送达，
// 规避"重连瞬间普通下行早于订阅被丢弃"的竞态；desired 为空时发布空 payload 清除 retained
func pushDesiredRetained(d *model.Device, desired map[string]interface{}) {
	if DownRetainedPublisher == nil {
		return
	}
	var payload []byte
	if len(desired) > 0 {
		msg, _ := json.Marshal(map[string]interface{}{
			"method": "property.set",
			"params": desired,
			"delta":  true,
			"ts":     time.Now().UnixMilli(),
		})
		payload = msg
	}
	if err := DownRetainedPublisher(d.ProductKey, d.Name, payload); err != nil {
		slog.Warn("push shadow desired retained failed", "device", d.Name, "err", err)
	}
}

func jsonEqual(a, b interface{}) bool {
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return string(ab) == string(bb)
}
