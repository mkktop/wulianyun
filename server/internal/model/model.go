package model

import (
	"time"

	"gorm.io/datatypes"
)

// 设备状态
const (
	DeviceStatusInactive = "inactive" // 未激活（从未连接）
	DeviceStatusOnline   = "online"
	DeviceStatusOffline  = "offline"
	DeviceStatusDisabled = "disabled"
)

type User struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"size:64;uniqueIndex;not null" json:"username"`
	PasswordHash string    `gorm:"size:128;not null" json:"-"`
	Nickname     string    `gorm:"size:64" json:"nickname"`
	Role         string    `gorm:"size:16;default:user" json:"role"` // admin / user
	CreatedAt    time.Time `json:"createdAt"`
}

// 产品接入数据模式
const (
	AccessModeThingModel  = "thingmodel"  // 标准物模型(JSON)
	AccessModePassthrough = "passthrough" // 透传解析(脚本)
	AccessModeModbus      = "modbus"      // Modbus 云端轮询
)

// 产品密钥模式
const (
	SecretModeDevice  = "device"  // 一机一密
	SecretModeProduct = "product" // 一型一密
)

type Product struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `gorm:"index;not null" json:"userId"`
	Name        string    `gorm:"size:64;not null" json:"name"`
	ProductKey  string    `gorm:"size:32;uniqueIndex;not null" json:"productKey"`
	Protocol    string    `gorm:"size:16;default:mqtt" json:"protocol"` // mqtt / tcp / http
	DataFormat  string    `gorm:"size:16;default:json" json:"dataFormat"`
	AccessMode  string    `gorm:"size:16;default:thingmodel" json:"accessMode"` // thingmodel/passthrough/modbus
	SecretMode  string    `gorm:"size:16;default:device" json:"secretMode"`     // device/product
	ProductSecret string  `gorm:"size:64" json:"productSecret"`                 // 一型一密的产品级密钥
	PollInterval  int     `gorm:"default:60" json:"pollInterval"`               // Modbus 采集周期(秒)
	CodecScript string    `gorm:"type:text" json:"codecScript"` // 自定义协议解析脚本(JS)
	Description string    `gorm:"size:255" json:"description"`
	CreatedAt   time.Time `json:"createdAt"`

	DeviceCount int64 `gorm:"-" json:"deviceCount"`
}

type Device struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	UserID        uint       `gorm:"index;not null" json:"userId"`
	ProductID     uint       `gorm:"index;not null" json:"productId"`
	ProductKey    string     `gorm:"size:32;not null;uniqueIndex:idx_product_device,priority:1" json:"productKey"`
	Name          string     `gorm:"size:64;not null;uniqueIndex:idx_product_device,priority:2" json:"name"`
	Secret        string     `gorm:"size:64;not null" json:"secret"`
	Status        string     `gorm:"size:16;default:inactive;index" json:"status"`
	GroupID       uint       `gorm:"index;default:0" json:"groupId"`             // 设备分组（0=未分组）
	Tags          datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"tags"`     // 标签数组
	Remark        string     `gorm:"size:255" json:"remark"`
	LastOnlineAt  *time.Time `json:"lastOnlineAt"`
	LastOfflineAt *time.Time `json:"lastOfflineAt"`
	CreatedAt     time.Time  `json:"createdAt"`

	ProductName string `gorm:"-" json:"productName"`
	GroupName   string `gorm:"-" json:"groupName"`
}

// Telemetry 遥测数据（TimescaleDB 超表，无主键）
type Telemetry struct {
	Ts         time.Time      `gorm:"index:idx_dev_ts,priority:2,sort:desc;not null" json:"ts"`
	DeviceID   uint           `gorm:"index:idx_dev_ts,priority:1;not null" json:"deviceId"`
	ProductKey string         `gorm:"size:32" json:"productKey"`
	DeviceName string         `gorm:"size:64" json:"deviceName"`
	Data       datatypes.JSON `gorm:"type:jsonb" json:"data"`
}

// DeviceEvent 设备上下线等事件日志
type DeviceEvent struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	DeviceID  uint      `gorm:"index" json:"deviceId"`
	Type      string    `gorm:"size:32" json:"type"` // online / offline
	Detail    string    `gorm:"size:255" json:"detail"`
	CreatedAt time.Time `json:"createdAt"`
}

// OpenApp OpenAPI 应用（第三方调用凭证）
type OpenApp struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"userId"`
	Name      string    `gorm:"size:64;not null" json:"name"`
	AppKey    string    `gorm:"size:32;uniqueIndex;not null" json:"appKey"`
	AppSecret string    `gorm:"size:64;not null" json:"appSecret"`
	Enabled   bool      `gorm:"default:true" json:"enabled"`
	CreatedAt time.Time `json:"createdAt"`
}
