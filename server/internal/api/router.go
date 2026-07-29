package api

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func NewRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173", "http://127.0.0.1:5173"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	v1 := r.Group("/api/v1")
	{
		// 公开接口
		v1.POST("/auth/register", Register)
		v1.POST("/auth/login", Login)
		v1.POST("/emqx/auth", EmqxAuth) // EMQX 回调（内网）

		// 需登录
		auth := v1.Group("", JWTAuth())
		{
			auth.GET("/auth/profile", Profile)
			auth.GET("/overview", Overview)
			auth.GET("/ws", WSHandler)

			auth.POST("/products", CreateProduct)
			auth.GET("/products", ListProducts)
			auth.PUT("/products/:id", UpdateProduct)
			auth.DELETE("/products/:id", DeleteProduct)
			auth.GET("/products/:id/thing-model", GetThingModel)
			auth.PUT("/products/:id/thing-model", SaveThingModel)
			auth.GET("/products/:id/codec", GetCodec)
			auth.PUT("/products/:id/codec", SaveCodec)
			auth.POST("/products/:id/codec/test", TestCodec)

			auth.POST("/devices", CreateDevice)
			auth.GET("/devices", ListDevices)
			auth.GET("/devices/:id", GetDevice)
			auth.PUT("/devices/:id", UpdateDevice)
			auth.DELETE("/devices/:id", DeleteDevice)
			auth.GET("/devices/:id/events", ListDeviceEvents)
			auth.GET("/devices/:id/latest", DeviceLatest)
			auth.GET("/devices/:id/history", DeviceHistory)
			auth.POST("/devices/:id/command", SendCommand)
			auth.GET("/devices/:id/shadow", GetDeviceShadow)
			auth.POST("/devices/:id/property", SetDeviceProperty)
			auth.POST("/devices/:id/service", InvokeService)

			auth.POST("/rules", CreateRule)
			auth.GET("/rules", ListRules)
			auth.PUT("/rules/:id", UpdateRule)
			auth.DELETE("/rules/:id", DeleteRule)

			auth.GET("/alarms", ListAlarms)
			auth.POST("/alarms/:id/resolve", ResolveAlarm)
			auth.GET("/alarms/stats", AlarmStats)

			auth.POST("/apps", CreateOpenApp)
			auth.GET("/apps", ListOpenApps)
			auth.PUT("/apps/:id", UpdateOpenApp)
			auth.DELETE("/apps/:id", DeleteOpenApp)
		}
	}

	// OpenAPI：第三方应用签名访问，复用管理端处理器
	open := r.Group("/openapi/v1", OpenAPIAuth())
	{
		open.GET("/devices", ListDevices)
		open.GET("/devices/:id", GetDevice)
		open.GET("/devices/:id/latest", DeviceLatest)
		open.GET("/devices/:id/history", DeviceHistory)
		open.GET("/devices/:id/shadow", GetDeviceShadow)
		open.POST("/devices/:id/property", SetDeviceProperty)
		open.POST("/devices/:id/command", SendCommand)
	}
	return r
}
