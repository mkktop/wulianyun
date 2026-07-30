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
