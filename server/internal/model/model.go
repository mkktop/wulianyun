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

// 账号状态
const (
	AccountStatusActive   = "active"
	AccountStatusDisabled = "disabled"
)

// 账号权限级别（P2 账号内 RBAC）：一级/超管恒为可写；二级账号可选 operate/view
const (
	AccountPermissionOperate = "operate" // 可操作：创建设备、下发指令、管理规则等
	AccountPermissionView    = "view"    // 只读：仅查看，写操作被拒绝
)

type User struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"size:64;uniqueIndex;not null" json:"username"`
	PasswordHash string    `gorm:"size:128;not null" json:"-"`
	Nickname     string    `gorm:"size:64" json:"nickname"`
	Role         string    `gorm:"size:16;default:user" json:"role"` // admin=平台超管 / user=普通账号
	ParentID     *uint     `gorm:"index" json:"parentId,omitempty"`          // 父账号 ID；nil=一级（独立主账号）；非空=二级
	Status       string    `gorm:"size:16;default:active" json:"status"`     // active / disabled
	Permission   string    `gorm:"size:16;default:operate" json:"permission"` // operate / view（二级账号权限级别）
	CreatedAt    time.Time `json:"createdAt"`
}

// ProductGrant 产品下放授权：一级把某个产品下放给某个二级账号使用。
type ProductGrant struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	ProductID   uint      `gorm:"uniqueIndex:idx_product_grant;not null" json:"productId"`
	SecondaryID uint      `gorm:"uniqueIndex:idx_product_grant;not null" json:"secondaryId"` // 接收的二级账号
	GrantedBy   uint      `gorm:"index" json:"grantedBy"` // 下放操作人（一级）
	Permission  string    `gorm:"size:16;default:operate" json:"permission"`                 // 默认 operate；后续可细化
	CreatedAt   time.Time `json:"createdAt"`
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

// TCP 组帧模式（透传/自定义协议产品）
const (
	FrameModeNone      = "none"      // 不组帧：每次读取视为一帧（兼容旧行为）
	FrameModeDelimiter = "delimiter" // 定界符分帧
	FrameModeLength    = "length"    // 长度字段分帧
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

	// TCP 组帧配置（Modbus 产品固定按 RTU 帧组帧，无需配置）
	FrameMode      string `gorm:"size:16;default:none" json:"frameMode"` // none/delimiter/length
	FrameDelimiter string `gorm:"size:32" json:"frameDelimiter"`         // 定界符(HEX，如 0D0A)
	FrameLenOffset int    `gorm:"default:0" json:"frameLenOffset"`       // 长度字段字节偏移
	FrameLenSize   int    `gorm:"default:1" json:"frameLenSize"`         // 长度字段字节数(1/2，2 为大端)
	FrameLenAdjust int    `gorm:"default:0" json:"frameLenAdjust"`       // 帧总长 = 长度值 + 调整值

	// TCP 自定义心跳（空 = 默认 PING/PONG；内容支持文本或 0x 开头 HEX）
	HeartbeatPacket string `gorm:"size:128" json:"heartbeatPacket"`
	HeartbeatReply  string `gorm:"size:128" json:"heartbeatReply"`

	// 远程配置（config.get / config.push 下发内容）
	RemoteConfig  datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"remoteConfig"`
	ConfigVersion int            `gorm:"default:0" json:"configVersion"`

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
	RegCode       string     `gorm:"size:64;index" json:"regCode"` // TCP 自定义注册码(IMEI/ICCID 等)，非空时可免三元组注册
	Status        string     `gorm:"size:16;default:inactive;index" json:"status"`
	GroupID       uint       `gorm:"index;default:0" json:"groupId"`             // 设备分组（0=未分组）
	GatewayID     *uint      `gorm:"index" json:"gatewayId,omitempty"`
	IsGateway     bool       `gorm:"default:false" json:"isGateway"`
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
	Ts               time.Time      `gorm:"index:idx_dev_ts,priority:2,sort:desc;not null" json:"ts"`
	DeviceID         uint           `gorm:"index:idx_dev_ts,priority:1;not null" json:"deviceId"`
	ProductKey       string         `gorm:"size:32" json:"productKey"`
	DeviceName       string         `gorm:"size:64" json:"deviceName"`
	Data             datatypes.JSON `gorm:"type:jsonb" json:"data"`
	Valid            bool           `gorm:"default:true" json:"valid"`
	ValidationErrors datatypes.JSON `gorm:"type:jsonb" json:"validationErrors,omitempty"`
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
