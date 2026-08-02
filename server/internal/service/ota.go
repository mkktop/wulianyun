package service

import (
	"encoding/json"
	"log/slog"

	"iot-platform/internal/model"
	"iot-platform/internal/repository"
)

// HandleOTAProgress 处理设备 OTA 升级进度上报
// 设备通过 thing/up/{pk}/{dn}/ota 上报：{"method":"ota.progress","taskId":1,"progress":50,"status":"upgrading"}
func HandleOTAProgress(deviceID uint, payload []byte) {
	var report struct {
		Method   string `json:"method"`
		TaskID   uint   `json:"taskId"`
		Progress int    `json:"progress"`
		Status   string `json:"status"` // upgrading / success / failed
		Message  string `json:"message"`
	}
	if err := json.Unmarshal(payload, &report); err != nil {
		slog.Warn("parse ota progress failed", "deviceID", deviceID, "error", err)
		return
	}
	if report.TaskID == 0 {
		return
	}

	var task model.OTATask
	if err := repository.DB.First(&task, report.TaskID).Error; err != nil {
		slog.Warn("ota task not found", "taskId", report.TaskID)
		return
	}

	// 更新任务进度
	updates := map[string]interface{}{"progress": report.Progress}
	if report.Status == "success" {
		updates["status"] = "completed"
		updates["progress"] = 100
	} else if report.Status == "failed" {
		updates["status"] = "failed"
	}
	repository.DB.Model(&task).Updates(updates)

	slog.Info("ota progress reported", "taskId", report.TaskID, "deviceID", deviceID,
		"progress", report.Progress, "status", report.Status)
}
