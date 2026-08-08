package model

import (
	"time"
)

// MessageTrace 消息轨迹追踪
type MessageTrace struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	TraceID    string    `gorm:"size:36;uniqueIndex;not null" json:"traceId"`
	UserID     uint      `gorm:"index;not null" json:"userId"`
	ProductKey string    `gorm:"size:32" json:"productId"`
	DeviceName string    `gorm:"size:64" json:"deviceName"`
	DeviceID   uint      `gorm:"index" json:"deviceId"`
	Direction  string    `gorm:"size:8" json:"direction"`
	Topic      string    `gorm:"size:255" json:"topic"`
	Payload    string    `gorm:"type:text" json:"payload"`
	Stage      string    `gorm:"size:32" json:"stage"`
	Status     string    `gorm:"size:32" json:"status"`
	IngestMs   int       `json:"ingestMs"`
	DecodeMs   int       `json:"decodeMs"`
	StoreMs    int       `json:"storeMs"`
	RuleMs     int       `json:"ruleMs"`
	Error      string    `gorm:"size:512" json:"error,omitempty"`
	CreatedAt  time.Time `gorm:"index" json:"createdAt"`
}

// DeviceLog 设备运行日志
type DeviceLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     uint      `gorm:"index;not null" json:"userId"`
	DeviceID   uint      `gorm:"index;not null" json:"deviceId"`
	DeviceName string    `gorm:"size:64" json:"deviceName"`
	Category   string    `gorm:"size:16;index" json:"category"`
	Summary    string    `gorm:"size:512" json:"summary"`
	Payload    string    `gorm:"type:text" json:"payload,omitempty"`
	TraceID    string    `gorm:"size:36;index" json:"traceId,omitempty"`
	CreatedAt  time.Time `gorm:"index" json:"createdAt"`
}
