package model

import (
	"time"

	"gorm.io/datatypes"
)

// EventReport 设备事件上报记录（对应物模型事件定义）
// 设备通过上行 {"method":"event.post","identifier":"xx","params":{...}} 上报
type EventReport struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	UserID     uint           `gorm:"index;not null" json:"userId"`
	ProductID  uint           `gorm:"index" json:"productId"`
	DeviceID   uint           `gorm:"index" json:"deviceId"`
	DeviceName string         `gorm:"size:64" json:"deviceName"`
	Identifier string         `gorm:"size:50" json:"identifier"`
	Type       string         `gorm:"size:16;default:info" json:"type"` // info/alert/fault
	Params     datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"params"`
	CreatedAt  time.Time      `gorm:"index" json:"createdAt"`
}

// CommandLog 指令下发日志（属性设置/服务调用/透传命令）
type CommandLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     uint      `gorm:"index;not null" json:"userId"`
	ProductID  uint      `gorm:"index" json:"productId"`
	DeviceID   uint      `gorm:"index" json:"deviceId"`
	DeviceName string    `gorm:"size:64" json:"deviceName"`
	Channel    string    `gorm:"size:16" json:"channel"` // mqtt / tcp / modbus
	Payload    string    `gorm:"type:text" json:"payload"`
	Success    bool      `json:"success"`
	Error      string    `gorm:"size:255" json:"error"`
	CreatedAt  time.Time `gorm:"index" json:"createdAt"`
}

// DeviceGroup 设备分组（扁平结构）
type DeviceGroup struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `gorm:"index;not null" json:"userId"`
	Name        string    `gorm:"size:64;not null" json:"name"`
	Description string    `gorm:"size:255" json:"description"`
	CreatedAt   time.Time `json:"createdAt"`

	DeviceCount int64 `gorm:"-" json:"deviceCount"`
}

// DeviceAlarmStat 设备告警统计（按设备分组聚合）
type DeviceAlarmStat struct {
	DeviceID     uint   `json:"deviceId"`
	DeviceName   string `json:"deviceName"`
	GroupName    string `json:"groupName"`
	TotalAlarms  int64  `json:"totalAlarms"`
	FiringCount  int64  `json:"firingCount"`
	ResolvedCount int64 `json:"resolvedCount"`
}

// CommandRequest 下行指令的请求-响应追踪
type CommandRequest struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	MessageID  string     `gorm:"size:36;uniqueIndex;not null" json:"messageId"`
	UserID     uint       `gorm:"index" json:"userId"`
	DeviceID   uint       `gorm:"index" json:"deviceId"`
	DeviceName string     `gorm:"size:64" json:"deviceName"`
	Method     string     `gorm:"size:50" json:"method"`     // property.set / service.invoke
	Service    string     `gorm:"size:50" json:"service"`
	Payload    string     `gorm:"type:text" json:"payload"`
	Status     string     `gorm:"size:16;default:pending" json:"status"` // pending / acked / timeout
	Response   string     `gorm:"type:text" json:"response,omitempty"`
	TimeoutMs  int        `gorm:"default:10000" json:"timeoutMs"`
	CreatedAt  time.Time  `json:"createdAt"`
	AckedAt    *time.Time `json:"ackedAt,omitempty"`
}

// Firmware 固件版本
type Firmware struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `gorm:"index" json:"userId"`
	ProductID   uint      `gorm:"index" json:"productId"`
	Version     string    `gorm:"size:32;not null" json:"version"`
	FileURL     string    `gorm:"size:255" json:"fileUrl"`
	FileSize    int64     `json:"fileSize"`
	Checksum    string    `gorm:"size:64" json:"checksum"`
	Description string    `gorm:"size:512" json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
}

// OTATask OTA升级任务
type OTATask struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     uint      `gorm:"index" json:"userId"`
	FirmwareID uint      `gorm:"index" json:"firmwareId"`
	ProductID  uint      `gorm:"index" json:"productId"`
	DeviceIDs  string    `gorm:"type:text" json:"deviceIds"` // JSON array of device IDs
	Status     string    `gorm:"size:16;default:pending" json:"status"` // pending/running/completed/failed
	Progress   int       `gorm:"default:0" json:"progress"` // 0-100
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}
