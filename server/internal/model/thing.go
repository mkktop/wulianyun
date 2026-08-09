package model

import (
	"time"

	"gorm.io/datatypes"
)

// ThingModel 物模型（TSL）：每个产品一份，JSON 定义属性/事件/服务
// properties: [{identifier,name,dataType(int32/float/double/bool/enum/text/date),
//               unit,min,max,step,accessMode(r/rw),enumSpec:[{value,label}],desc}]
// events:     [{identifier,name,type(info/alert/fault),outputs:[{identifier,name,dataType}],desc}]
// services:   [{identifier,name,async,inputs:[{identifier,name,dataType}],outputs:[...],desc}]
type ThingModel struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	ProductID  uint           `gorm:"uniqueIndex;not null" json:"productId"`
	Properties datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"properties"`
	Events     datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"events"`
	Services   datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"services"`
	UpdatedAt  time.Time      `json:"updatedAt"`
}

// DeviceShadow 设备影子：desired 期望值 / reported 上报值
type DeviceShadow struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	DeviceID  uint           `gorm:"uniqueIndex;not null" json:"deviceId"`
	Desired   datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"desired"`
	Reported  datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"reported"`
	Version   int64          `gorm:"default:0" json:"version"`
	UpdatedAt time.Time      `json:"updatedAt"`
}

// 规则类型
const (
	RuleTypeAlarm   = "alarm"   // 阈值告警
	RuleTypeOffline = "offline" // 离线告警
	RuleTypeForward = "forward" // 数据转发
)

// Rule 规则：条件(JSON) + 动作(JSON)
// condition(alarm):   {"field":"temperature","op":">","value":35}
// condition(offline): {"minutes":10}
// action(alarm):   {"level":"warning|critical","notify":["ws","webhook"],"webhookUrl":"..."}
// action(forward): {"webhookUrl":"..."}
type Rule struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	UserID    uint           `gorm:"index;not null" json:"userId"`
	ProductID uint           `gorm:"index" json:"productId"` // 0 表示全部产品
	DeviceID  uint           `gorm:"index" json:"deviceId"`  // 0 表示产品下全部设备
	Name      string         `gorm:"size:64;not null" json:"name"`
	Type      string         `gorm:"size:16;not null" json:"type"`
	Condition datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"condition"`
	Action    datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"action"`
	Silence    int            `gorm:"default:5" json:"silence"`     // 静默期(分钟)，防告警风暴
	RetryCount int            `gorm:"default:3" json:"retryCount"` // Webhook重试次数
	// 注意：不能加 gorm:"default:true"——false 会被 GORM 零值省略，草稿规则会被存成启用（与 OpenApp.Enabled 同类）
	Enabled    bool           `json:"enabled"`
	CreatedAt time.Time      `json:"createdAt"`

	ProductName string `gorm:"-" json:"productName"`
	DeviceName  string `gorm:"-" json:"deviceName"`
}

// 告警状态
const (
	AlarmStatusFiring   = "firing"
	AlarmStatusResolved = "resolved"
)

// Alarm 告警记录
type Alarm struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	UserID     uint       `gorm:"index;not null" json:"userId"`
	RuleID     uint       `gorm:"index" json:"ruleId"`
	RuleName   string     `gorm:"size:64" json:"ruleName"`
	DeviceID   uint       `gorm:"index" json:"deviceId"`
	DeviceName string     `gorm:"size:64" json:"deviceName"`
	Level      string     `gorm:"size:16;default:warning" json:"level"`
	Message    string     `gorm:"size:512" json:"message"`
	Status     string     `gorm:"size:16;default:firing;index" json:"status"`
	CreatedAt  time.Time  `json:"createdAt"`
	ResolvedAt *time.Time `json:"resolvedAt"`
	ConfirmedAt *time.Time `json:"confirmedAt"`
}
