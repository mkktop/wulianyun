package service

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"iot-platform/internal/model"
	"iot-platform/internal/repository"
	"iot-platform/internal/ws"
)

// handleEventReport 设备事件上报入库并实时推送
// 报文格式: {"method":"event.post","identifier":"highTemp","type":"alert","params":{...}}
func handleEventReport(d *model.Device, data map[string]interface{}) {
	identifier, _ := data["identifier"].(string)
	if identifier == "" {
		slog.Warn("event.post missing identifier", "device", d.Name)
		return
	}
	etype, _ := data["type"].(string)
	if etype != "alert" && etype != "fault" {
		etype = "info"
	}
	params := []byte("{}")
	if p, ok := data["params"]; ok {
		if b, err := json.Marshal(p); err == nil {
			params = b
		}
	}
	ev := model.EventReport{
		UserID: d.UserID, ProductID: d.ProductID, DeviceID: d.ID, DeviceName: d.Name,
		Identifier: identifier, Type: etype, Params: params,
	}
	if err := repository.DB.Create(&ev).Error; err != nil {
		slog.Error("save event report failed", "err", err)
		return
	}
	// 实时推送：与 telemetry/status 一致，走 PushRecipients fan-out（一级账号实时看二级设备事件）
	eventMsg := map[string]interface{}{
		"deviceId": d.ID, "deviceName": d.Name,
		"identifier": identifier, "type": etype,
		"params": json.RawMessage(params), "ts": ev.CreatedAt.UnixMilli(),
	}
	for _, uid := range PushRecipients(d.UserID) {
		ws.H.PushEvent(uid, eventMsg)
	}
	slog.Info("event reported", "device", d.Name, "identifier", identifier, "type", etype)

	// 写入设备日志（批量化 worker 异步落库）
	writeDeviceLog(d.UserID, d.ID, d.Name, "event",
		fmt.Sprintf("事件上报[%s] 类型:%s", identifier, etype), string(params), "")
}

// LogCommand 记录一次指令下发（异步调用）
func LogCommand(productKey, deviceName, channel string, payload []byte, sendErr error) {
	d, err := FindDevice(productKey, deviceName)
	if err != nil {
		return
	}
	logEntry := model.CommandLog{
		UserID: d.UserID, ProductID: d.ProductID, DeviceID: d.ID, DeviceName: d.Name,
		Channel: channel, Payload: string(payload), Success: sendErr == nil,
	}
	if sendErr != nil {
		logEntry.Error = sendErr.Error()
	}
	if err := repository.DB.Create(&logEntry).Error; err != nil {
		slog.Warn("save command log failed", "err", err)
	}

	// 同步写入设备日志
	logSummary := "下行指令[" + channel + "]"
	logCategory := "data_down"
	if sendErr != nil {
		logCategory = "error"
		logSummary = "下行失败[" + channel + "]: " + sendErr.Error()
	}
	writeDeviceLog(d.UserID, d.ID, d.Name, logCategory, logSummary, string(payload), "")
}

// LogTCPDisconnect 记录 TCP 设备断连事件（含重连引导信息）
func LogTCPDisconnect(productKey, deviceName, reason string) {
	d, err := FindDevice(productKey, deviceName)
	if err != nil {
		return
	}
	writeDeviceLog(d.UserID, d.ID, d.Name, "connection",
		"TCP 断开("+reason+")，设备将自动重连", "", "")
}

// Broadcaster 由 main 注入：向产品下所有设备广播（MQTT 广播主题 + TCP 逐连接）
var Broadcaster func(productKey string, payload []byte) error

// Broadcast 向产品下所有设备广播消息
func Broadcast(productKey string, payload []byte) error {
	if Broadcaster == nil {
		return nil
	}
	return Broadcaster(productKey, payload)
}

// handleNTPRequest 处理设备 NTP 对时请求
// 上行: {"method":"ntp.request","deviceSendTime":<ms>}
// 下行: {"method":"ntp.response","deviceSendTime","serverRecvTime","serverSendTime"}
func handleNTPRequest(d *model.Device, data map[string]interface{}) {
	if DownPublisher == nil {
		return
	}
	recv := time.Now().UnixMilli()
	var deviceSend int64
	if v, ok := data["deviceSendTime"].(float64); ok {
		deviceSend = int64(v)
	}
	resp, _ := json.Marshal(map[string]interface{}{
		"method":         "ntp.response",
		"deviceSendTime": deviceSend,
		"serverRecvTime": recv,
		"serverSendTime": time.Now().UnixMilli(),
	})
	if err := DownPublisher(d.ProductKey, d.Name, resp); err != nil {
		slog.Warn("ntp response failed", "device", d.Name, "err", err)
	}
}

// handleConfigGet 处理设备拉取远程配置：下发产品级 RemoteConfig
// 上行: {"method":"config.get"}；下行: {"method":"config.push","version","params"}
func handleConfigGet(d *model.Device) {
	if DownPublisher == nil {
		return
	}
	var p model.Product
	if err := repository.DB.Select("remote_config, config_version").First(&p, d.ProductID).Error; err != nil {
		return
	}
	cfg := map[string]interface{}{}
	if len(p.RemoteConfig) > 0 {
		json.Unmarshal(p.RemoteConfig, &cfg)
	}
	resp, _ := json.Marshal(map[string]interface{}{
		"method":  "config.push",
		"version": p.ConfigVersion,
		"params":  cfg,
	})
	if err := DownPublisher(d.ProductKey, d.Name, resp); err != nil {
		slog.Warn("config push failed", "device", d.Name, "err", err)
	}
}
