package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"iot-platform/internal/model"
	"iot-platform/internal/repository"
)

// Token 认证：设备动态获取 MQTT 连接 Token（有效期可配，默认 1 小时）
// 设备通过 HTTP 接口换取 Token，然后用 Token 作为 MQTT password 连接 EMQX

const tokenPrefix = "device:token:"
const defaultTokenTTL = 3600 // 秒（1 小时）

// GenerateDeviceToken 为设备生成动态 Token
// 设备通过 POST /api/v1/auth/token 携带三元组换 Token
func GenerateDeviceToken(productKey, deviceName, secret string) (string, int, error) {
	d, err := FindDeviceForAuth(productKey, deviceName, secret)
	if err != nil {
		return "", 0, fmt.Errorf("认证失败")
	}
	if d.Status == model.DeviceStatusDisabled {
		return "", 0, fmt.Errorf("设备已禁用")
	}

	// 生成随机 Token
	b := make([]byte, 16)
	rand.Read(b)
	token := "tk:" + hex.EncodeToString(b)

	// 存入 Redis，带 TTL；绑定签发时设备密钥的哈希——密钥轮转后旧 token 立即失效
	ttl := defaultTokenTTL
	ctx := context.Background()
	secretHash := sha256.Sum256([]byte(secret))
	tokenData, _ := json.Marshal(map[string]interface{}{
		"deviceId":   d.ID,
		"productId":  productKey,
		"deviceName": deviceName,
		"secretHash": hex.EncodeToString(secretHash[:]),
		"issuedAt":   time.Now().Unix(),
	})
	repository.RDB.Set(ctx, tokenPrefix+token, tokenData, time.Duration(ttl)*time.Second)

	slog.Info("device token generated", "device", deviceName, "ttl", ttl)
	return token, ttl, nil
}

// ValidateDeviceToken 验证设备 Token（EMQX 认证回调中使用）
// 如果 Token 有效，返回设备信息；否则返回 nil
func ValidateDeviceToken(token string) (*model.Device, error) {
	if len(token) < 3 || token[:3] != "tk:" {
		return nil, fmt.Errorf("非 Token 格式")
	}
	ctx := context.Background()
	data, err := repository.RDB.Get(ctx, tokenPrefix+token).Bytes()
	if err == redis.Nil {
		return nil, fmt.Errorf("Token 已过期或不存在")
	}
	if err != nil {
		return nil, err
	}
	var info struct {
		DeviceID   uint   `json:"deviceId"`
		ProductKey string `json:"productId"`
		DeviceName string `json:"deviceName"`
		SecretHash string `json:"secretHash"`
	}
	json.Unmarshal(data, &info)
	d, err := FindDevice(info.ProductKey, info.DeviceName)
	if err != nil {
		return nil, fmt.Errorf("设备不存在")
	}
	// 设备被禁用：token 立即失效
	if d.Status == model.DeviceStatusDisabled {
		return nil, fmt.Errorf("设备已禁用")
	}
	// 密钥轮转：签发时的密钥哈希与当前不一致 → 旧 token 失效
	if info.SecretHash != "" {
		cur := sha256.Sum256([]byte(d.Secret))
		if info.SecretHash != hex.EncodeToString(cur[:]) {
			return nil, fmt.Errorf("设备密钥已变更，token 失效")
		}
	}
	return d, nil
}

// RevokeDeviceToken 撤销设备 Token
func RevokeDeviceToken(token string) {
	if len(token) < 3 {
		return
	}
	ctx := context.Background()
	repository.RDB.Del(ctx, tokenPrefix+token)
}
