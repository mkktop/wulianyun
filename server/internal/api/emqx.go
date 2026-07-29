package api

import (
	"github.com/gin-gonic/gin"

	"iot-platform/internal/config"
	"iot-platform/internal/model"
	"iot-platform/internal/service"
)

// EmqxAuth EMQX HTTP 认证回调
// 设备约定：clientid = {productKey}.{deviceName}，password = 设备 Secret
// 返回 {"result":"allow"} / {"result":"deny"}
func EmqxAuth(c *gin.Context) {
	var req struct {
		ClientID string `json:"clientid"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(200, gin.H{"result": "deny"})
		return
	}

	// 平台内部客户端放行
	if req.Username == config.C.MQTT.Username && req.Password == config.C.MQTT.Password {
		c.JSON(200, gin.H{"result": "allow", "is_superuser": true})
		return
	}

	productKey, deviceName, ok := service.ParseClientID(req.ClientID)
	if !ok {
		c.JSON(200, gin.H{"result": "deny"})
		return
	}
	d, err := service.FindDevice(productKey, deviceName)
	if err != nil || d.Secret != req.Password || d.Status == model.DeviceStatusDisabled {
		c.JSON(200, gin.H{"result": "deny"})
		return
	}
	c.JSON(200, gin.H{"result": "allow"})
}
