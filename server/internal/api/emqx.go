package api

import (
	"strings"

	"github.com/gin-gonic/gin"

	"iot-platform/internal/config"
	"iot-platform/internal/model"
	"iot-platform/internal/service"
)

// EmqxAuth EMQX HTTP 认证回调
// 设备约定：clientid = {productKey}.{deviceName}，password = 设备 Secret 或动态 Token
// 返回 {"result":"allow"} / {"result":"deny"}
func EmqxAuth(c *gin.Context) {
	var req struct {
		ClientID  string `json:"clientid"`
		Username  string `json:"username"`
		Password  string `json:"password"`
		Peerhost  string `json:"peerhost"`
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

	// 支持：固定 Secret 或动态 Token（tk:开头）
	var d *model.Device
	var err error
	if strings.HasPrefix(req.Password, "tk:") {
		// 动态 Token 认证
		d, err = service.ValidateDeviceToken(req.Password)
		if err != nil || d == nil {
			c.JSON(200, gin.H{"result": "deny"})
			return
		}
		// token 与 clientid 绑定：token 只允许其签发设备使用，
		// 防止设备 A 持 token 冒认设备 B 的 clientid（跨设备/跨租户截获下行）
		if d.ProductKey != productKey || d.Name != deviceName {
			c.JSON(200, gin.H{"result": "deny"})
			return
		}
	} else {
		// 固定 Secret 认证
		d, err = service.FindDeviceForAuth(productKey, deviceName, req.Password)
		if err != nil || d == nil {
			c.JSON(200, gin.H{"result": "deny"})
			return
		}
	}
	if d.Status == model.DeviceStatusDisabled {
		c.JSON(200, gin.H{"result": "deny", "comment": "device disabled"})
		return
	}

	c.JSON(200, gin.H{
		"result":       "allow",
		"clientid":     req.ClientID,
		"username":     productKey + "/" + deviceName,
		"expire_time":  "", // 不过期
	})
}

// EmqxACL EMQX HTTP 授权回调（Topic 级权限）
// 设备允许的 Topic：
//   发布: thing/up/{pk}/{dn}[/...], thing/gateway/{pk}/{dn}/sub/+/login, thing/gateway/{pk}/{dn}/sub/+/logout
//   订阅: thing/down/{pk}/{dn}[/...], thing/broadcast/{pk}, thing/gateway/{pk}/{dn}/sub/+
//   遗嘱: thing/offline/{pk}/{dn}（设备发布自己的遗嘱）
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
	// 平台内部客户端直接放行
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

	switch req.Action {
	case "publish":
		// 上行数据 thing/up/{pk}/{dn}[/...]
		allow = matchOwnTopic(req.Topic, "thing/up/", own)
		// 遗嘱 topic thing/offline/{pk}/{dn}
		if !allow {
			allow = req.Topic == "thing/offline/"+own
		}
		// 子设备管理 thing/gateway/{pk}/{dn}/sub/{subId}/login|logout
		if !allow {
			allow = matchOwnTopic(req.Topic, "thing/gateway/", own+"/sub/")
		}
	case "subscribe":
		// 下行指令 thing/down/{pk}/{dn}[/...]
		allow = matchOwnTopic(req.Topic, "thing/down/", own)
		// 产品广播 thing/broadcast/{pk}
		if !allow {
			allow = req.Topic == "thing/broadcast/"+productKey
		}
		// 网关子设备下行 thing/gateway/{pk}/{dn}/sub/+
		if !allow {
			allow = req.Topic == "thing/gateway/"+own+"/sub/+"
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
