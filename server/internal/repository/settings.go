package repository

import (
	"errors"
	"log/slog"
	"strconv"

	"iot-platform/internal/model"
)

// ---- 热更新系统参数 ----
//
// 参数存 system_settings 表，覆盖 config.yaml 默认值；无记录时回退到调用方传入的默认值。
// 读取频率低（注册、登录、清理循环），直接查库不做缓存，改后立即生效。

// ErrSettingKeyNotAllowed 参数 key 不在白名单内
var ErrSettingKeyNotAllowed = errors.New("setting key not allowed")

// SettingKeys 热更新参数白名单（超管后台可修改的 key）
var SettingKeys = map[string]struct {
	Type        string // bool | int
	Default     string
	Description string
}{
	"register_enabled":          {Type: "bool", Default: "true", Description: "开放注册开关（false 时注册接口拒绝新账号）"},
	"jwt_expire_hours":          {Type: "int", Default: "", Description: "登录令牌有效期（小时），空则用 config.yaml 的 jwt.expire_hours"},
	"trace_retention_days":      {Type: "int", Default: "", Description: "消息轨迹保留天数，空则用 config.yaml 的 log.trace_retention_days"},
	"device_log_retention_days": {Type: "int", Default: "", Description: "设备日志保留天数，空则用 config.yaml 的 log.device_log_retention_days"},
}

// GetSettingString 读取参数原始字符串；未设置返回 fallback
func GetSettingString(key, fallback string) string {
	var s model.SystemSetting
	if err := DB.First(&s, "key = ?", key).Error; err != nil {
		return fallback
	}
	return s.Value
}

// GetSettingBool 读取 bool 参数；未设置或解析失败返回 fallback
func GetSettingBool(key string, fallback bool) bool {
	v := GetSettingString(key, "")
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		slog.Warn("invalid bool setting", "key", key, "value", v)
		return fallback
	}
	return b
}

// GetSettingInt 读取 int 参数；未设置或解析失败返回 fallback
func GetSettingInt(key string, fallback int) int {
	v := GetSettingString(key, "")
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		slog.Warn("invalid int setting", "key", key, "value", v)
		return fallback
	}
	return n
}

// SaveSetting 写入参数（key 需在白名单内）
func SaveSetting(key, value string, updatedBy uint) error {
	if _, ok := SettingKeys[key]; !ok {
		return ErrSettingKeyNotAllowed
	}
	s := model.SystemSetting{Key: key, Value: value, Description: SettingKeys[key].Description, UpdatedBy: updatedBy}
	return DB.Save(&s).Error
}

// ListSettings 返回全部热更新参数记录
func ListSettings() []model.SystemSetting {
	var list []model.SystemSetting
	DB.Order("key").Find(&list)
	return list
}
