package model

import "time"

// 公告状态
const (
	AnnouncementStatusDraft     = "draft"     // 草稿（未发布，仅超管可见）
	AnnouncementStatusPublished = "published" // 已发布（控制台内所有登录账号可见）
)

// 公告级别
const (
	AnnouncementLevelNormal    = "normal"
	AnnouncementLevelImportant = "important"
)

// Announcement 平台公告（超管发布，控制台内所有登录账号可见）
type Announcement struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	UserID    uint       `gorm:"index" json:"userId"` // 发布者
	Title     string     `gorm:"size:128;not null" json:"title"`
	Content   string     `gorm:"type:text" json:"content"` // markdown
	Level     string     `gorm:"size:16;default:normal" json:"level"`
	Status    string     `gorm:"size:16;default:draft" json:"status"`
	PublishAt *time.Time `json:"publishAt"` // 发布时间（发布时置为当前时间）
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

// HelpDoc 帮助中心文档（超管在线编辑，markdown 存库、前端渲染）
type HelpDoc struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Key       string    `gorm:"size:64;uniqueIndex;not null" json:"key"` // 唯一标识（用于前端路由）
	Title     string    `gorm:"size:128;not null" json:"title"`
	Content   string    `gorm:"type:text" json:"content"` // markdown
	UpdatedBy uint      `json:"updatedBy"`
	UpdatedAt time.Time `json:"updatedAt"`
	CreatedAt time.Time `json:"createdAt"`
}

// SystemSetting 热更新系统参数（覆盖 config.yaml 默认值，改后立即生效）
type SystemSetting struct {
	Key         string    `gorm:"size:64;primaryKey" json:"key"`
	Value       string    `gorm:"size:512" json:"value"` // 字符串存储，按 key 类型解析
	Description string    `gorm:"size:255" json:"description"`
	UpdatedBy   uint      `json:"updatedBy"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// StorageConfig 对象存储配置（单行记录，id 固定为 1；超管后台可改并热重载，覆盖 config.yaml）
type StorageConfig struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	Type         string `gorm:"size:16;default:local" json:"type"` // local | s3
	LocalDir     string `gorm:"size:128;default:uploads" json:"localDir"`
	Endpoint     string `gorm:"size:255" json:"endpoint"`
	Region       string `gorm:"size:64" json:"region"`
	Bucket       string `gorm:"size:128" json:"bucket"`
	AccessKey    string `gorm:"size:255" json:"accessKey"`
	SecretKey    string `gorm:"size:255" json:"secretKey"` // 返回时打码，写空表示不变
	UseSSL       bool   `json:"useSSL"`
	PublicDomain string `gorm:"size:255" json:"publicDomain"`
	UpdatedBy    uint   `json:"updatedBy"`
	UpdatedAt    time.Time `json:"updatedAt"`
}
