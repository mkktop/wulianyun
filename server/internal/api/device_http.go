package api

import (
	"encoding/base64"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"iot-platform/internal/model"
	"iot-platform/internal/service"

	"github.com/gin-gonic/gin"
)

// HTTPDeviceTelemetry 设备通过HTTP上报遥测数据
// POST /api/v1/http/telemetry
// Header: X-Device-Token: Base64(productKey:deviceName:secret)
func HTTPDeviceTelemetry(c *gin.Context) {
	token := c.GetHeader("X-Device-Token")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "missing X-Device-Token"})
		return
	}

	// 解码 Token
	decoded, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "invalid token encoding"})
		return
	}
	parts := strings.SplitN(string(decoded), ":", 3)
	if len(parts) != 3 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "invalid token format"})
		return
	}
	productKey, deviceName, secret := parts[0], parts[1], parts[2]

	// 验证设备（与 MQTT/TCP 一致：禁用设备的密钥未轮转仍拒绝上报）
	d, err := service.FindDeviceForAuth(productKey, deviceName, secret)
	if err != nil {
		slog.Warn("http device auth failed", "pk", productKey, "dn", deviceName, "error", err)
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "auth failed"})
		return
	}
	if d.Status == model.DeviceStatusDisabled {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "msg": "device disabled"})
		return
	}

	// 读取 body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "read body failed"})
		return
	}

	// 调用数据摄入
	go service.HandleTelemetry(productKey, deviceName, body)

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok"})
}
