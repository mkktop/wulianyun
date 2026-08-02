package api

import (
	"strings"

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
	d, err := service.FindDeviceForAuth(productKey, deviceName, req.Password)
	if err != nil || d.Status == model.DeviceStatusDisabled {
		c.JSON(200, gin.H{"result": "deny"})
		return
	}
	c.JSON(200, gin.H{"result": "allow"})
}

// EmqxACL EMQX HTTP 授权回调（Topic 级权限）
// 设备仅允许：发布 thing/up/{pk}/{dn}[/...]；订阅 thing/down/{pk}/{dn}[/...] 与 thing/broadcast/{pk}
// 平台内部客户端在认证时已标记 is_superuser，EMQX 不会回调到这里
func EmqxACL(c *gin.Context) {
	var req struct {
		ClientID string `json:"clientid"`
		Username string `json:"username"`
		Topic    string `json:"topic"`
		Action   string `json:"action"` // publish / subscribe
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(200, gin.H{"result": "deny"})
		return
	}
	// 平台内部客户端直接放行（正常路径走 superuser 不会到这里）
	if req.Username == config.C.MQTT.Username {
		c.JSON(200, gin.H{"result": "allow"})
		return
	}
	productKey, deviceName, ok := service.ParseClientID(req.ClientID)
	if !ok {
		c.JSON(200, gin.H{"result": "deny"})
		return
	}
	own := productKey + "/" + deviceName
	allow := false
	// 允许设备发布 reply 主题（指令应答）
	if req.Action == "publish" && strings.HasPrefix(req.Topic, "thing/up/") && strings.HasSuffix(req.Topic, "/reply") {
		rest, ok := strings.CutPrefix(req.Topic, "thing/up/")
		if ok && (rest == own || strings.HasPrefix(rest, own+"/")) {
			allow = true
		}
	}
	if !allow {
		switch req.Action {
		case "publish":
			allow = matchOwnTopic(req.Topic, "thing/up/", own)
		case "subscribe":
			allow = matchOwnTopic(req.Topic, "thing/down/", own) ||
				req.Topic == "thing/broadcast/"+productKey
		}
	}
	if allow {
		c.JSON(200, gin.H{"result": "allow"})
		return
	}
	c.JSON(200, gin.H{"result": "deny"})
}

// matchOwnTopic topic 必须是 {prefix}{pk}/{dn} 本身或其子级，且不含通配符
func matchOwnTopic(topic, prefix, own string) bool {
	if strings.ContainsAny(topic, "+#") {
		return false
	}
	rest, ok := strings.CutPrefix(topic, prefix)
	if !ok {
		return false
	}
	return rest == own || strings.HasPrefix(rest, own+"/")
}