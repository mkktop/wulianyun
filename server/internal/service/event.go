package service

import (
	"encoding/json"
	"log/slog"

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
	ws.H.PushEvent(d.UserID, map[string]interface{}{
		"deviceId": d.ID, "deviceName": d.Name,
		"identifier": identifier, "type": etype,
		"params": json.RawMessage(params), "ts": ev.CreatedAt.UnixMilli(),
	})
	slog.Info("event reported", "device", d.Name, "identifier", identifier, "type", etype)
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
}
