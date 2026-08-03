package api

import (
	"encoding/json"
	"time"

	"github.com/gin-gonic/gin"

	"iot-platform/internal/repository"
	"iot-platform/internal/service"
)

// GetRemoteConfig 获取产品远程配置
func GetRemoteConfig(c *gin.Context) {
	p, err := canViewProduct(c, c.Param("id"))
	if err != nil {
		Fail(c, 404, "产品不存在")
		return
	}
	cfg := map[string]interface{}{}
	if len(p.RemoteConfig) > 0 {
		json.Unmarshal(p.RemoteConfig, &cfg)
	}
	OK(c, gin.H{"version": p.ConfigVersion, "config": cfg})
}

// SaveRemoteConfig 保存产品远程配置（版本号自增）
func SaveRemoteConfig(c *gin.Context) {
	p, err := mustOwnProduct(c, c.Param("id"))
	if err != nil {
		Fail(c, 404, "产品不存在")
		return
	}
	var req struct {
		Config map[string]interface{} `json:"config"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, 400, "参数错误")
		return
	}
	data, _ := json.Marshal(req.Config)
	repository.DB.Model(p).Updates(map[string]interface{}{
		"remote_config":  data,
		"config_version": p.ConfigVersion + 1,
	})
	OK(c, gin.H{"version": p.ConfigVersion + 1})
}

// PushRemoteConfig 主动把远程配置广播给产品下所有设备
func PushRemoteConfig(c *gin.Context) {
	p, err := mustOwnProduct(c, c.Param("id"))
	if err != nil {
		Fail(c, 404, "产品不存在")
		return
	}
	cfg := map[string]interface{}{}
	if len(p.RemoteConfig) > 0 {
		json.Unmarshal(p.RemoteConfig, &cfg)
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"method":  "config.push",
		"version": p.ConfigVersion,
		"params":  cfg,
		"ts":      time.Now().UnixMilli(),
	})
	if err := service.Broadcast(p.ProductKey, payload); err != nil {
		Fail(c, 500, "下发失败: "+err.Error())
		return
	}
	OK(c, nil)
}

// BroadcastProduct 向产品下所有设备广播自定义消息
func BroadcastProduct(c *gin.Context) {
	p, err := mustOwnProduct(c, c.Param("id"))
	if err != nil {
		Fail(c, 404, "产品不存在")
		return
	}
	var req struct {
		Payload map[string]interface{} `json:"payload"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Payload) == 0 {
		Fail(c, 400, "广播内容不能为空")
		return
	}
	payload, _ := json.Marshal(req.Payload)
	if err := service.Broadcast(p.ProductKey, payload); err != nil {
		Fail(c, 500, "广播失败: "+err.Error())
		return
	}
	OK(c, nil)
}
