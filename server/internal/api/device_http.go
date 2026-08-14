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

	// 读取 body（上限 1MB：设备遥测远小于此，无上限时恶意/异常设备可灌 GB 级 body 撑爆内存）
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"code": 413, "msg": "payload too large"})
		return
	}

	// 提交到有界 worker 池处理（非阻塞，满载丢弃计数；防止 DB 抖动时无界 goroutine 堆积 OOM）
	service.SubmitTelemetry(productKey, deviceName, body)

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok"})
}
