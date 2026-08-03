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
		v1.POST("/auth/token", DeviceToken) // 设备动态获取 MQTT Token
		v1.POST("/emqx/auth", EmqxAuth) // EMQX 认证回调（内网）
		v1.POST("/emqx/acl", EmqxACL)   // EMQX 授权回调（内网）
		v1.POST("/http/telemetry", HTTPDeviceTelemetry) // 设备HTTP上报（Token认证）

		// 需登录
		auth := v1.Group("", JWTAuth())
		{
			auth.GET("/auth/profile", Profile)
			auth.POST("/auth/change-password", ChangePassword)
			auth.GET("/overview", Overview)
			auth.GET("/ws", WSHandler)

			auth.POST("/products", CreateProduct)
			auth.GET("/products", ListProducts)
			auth.GET("/products/:id", GetProduct)
			auth.PUT("/products/:id", UpdateProduct)
			auth.DELETE("/products/:id", DeleteProduct)
			auth.GET("/products/:id/thing-model", GetThingModel)
			auth.PUT("/products/:id/thing-model", SaveThingModel)
			auth.GET("/products/:id/tsl/export", ExportThingModel)
			auth.POST("/products/:id/tsl/import", ImportThingModel)
			auth.GET("/products/:id/codec", GetCodec)
			auth.PUT("/products/:id/codec", SaveCodec)
			auth.POST("/products/:id/codec/test", TestCodec)
			auth.GET("/products/:id/modbus-points", GetModbusPoints)
			auth.PUT("/products/:id/modbus-points", SaveModbusPoints)
			auth.POST("/products/:id/modbus-points/test", TestModbusPoint)
			auth.GET("/products/:id/modbus-groups", ListModbusGroups)
			auth.POST("/products/:id/modbus-groups", CreateModbusGroup)
			auth.PUT("/products/:id/modbus-groups/:gid", UpdateModbusGroup)
			auth.DELETE("/products/:id/modbus-groups/:gid", DeleteModbusGroup)
			auth.GET("/products/:id/stats", ProductStats)
			auth.GET("/products/:id/device-alarm-stats", DeviceAlarmStats)
			auth.GET("/products/:id/config", GetRemoteConfig)
			auth.PUT("/products/:id/config", SaveRemoteConfig)
			auth.POST("/products/:id/config/push", PushRemoteConfig)
			auth.POST("/products/:id/broadcast", BroadcastProduct)
			auth.POST("/products/:id/devices/batch", BatchCreateDevices)

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
			auth.POST("/alarms/:id/confirm", ConfirmAlarm)
			auth.GET("/alarms/stats", AlarmStats)
			auth.GET("/alarms/trend", AlarmTrend)

			auth.POST("/apps", CreateOpenApp)
			auth.GET("/apps", ListOpenApps)
			auth.PUT("/apps/:id", UpdateOpenApp)
			auth.DELETE("/apps/:id", DeleteOpenApp)

			auth.GET("/event-reports", ListEventReports)
			auth.GET("/command-logs", ListCommandLogs)

			auth.POST("/groups", CreateGroup)
			auth.GET("/groups", ListGroups)
			auth.PUT("/groups/:id", UpdateGroup)
			auth.DELETE("/groups/:id", DeleteGroup)

			// 账号管理 + 产品下放（仅一级主账号）
			primary := auth.Group("", PrimaryAuth())
			{
				primary.GET("/accounts", ListAccounts)
				primary.POST("/accounts", CreateAccount)
				primary.PUT("/accounts/:id", UpdateAccount)
				primary.DELETE("/accounts/:id", DeleteAccount)
				primary.GET("/products/:id/grants", ListGrants)
				primary.POST("/products/:id/grants", CreateGrant)
				primary.DELETE("/products/:id/grants/:sid", DeleteGrant)
			}

			// 开发者工具
			sim := auth.Group("/simulator")
			sim.POST("/connect", ConnectSimulator)
			sim.POST("/publish", PublishSimulator)
			sim.POST("/disconnect", DisconnectSimulator)
			sim.GET("/sessions", ListSimulatorSessions)

			auth.GET("/mqtt-debug/ws", MqttDebugWS)

			auth.GET("/traces", ListTraces)
			auth.GET("/traces/:traceId", GetTrace)

			auth.GET("/devices/:id/logs", ListDeviceLogs)
			auth.GET("/device-logs", ListAllDeviceLogs)

			auth.GET("/devices/:id/sub-devices", ListSubDevices)
			auth.POST("/devices/:id/sub-devices", AddSubDevice)
			auth.DELETE("/devices/:id/sub-devices/:subId", RemoveSubDevice)

			auth.GET("/firmwares", ListFirmwares)
			auth.POST("/firmwares", CreateFirmware)
			auth.DELETE("/firmwares/:id", DeleteFirmware)
			auth.POST("/ota-tasks", CreateOTATask)
			auth.GET("/ota-tasks", ListOTATasks)
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
	// 静态资源：OTA 固件下载（fileURL 形如 /uploads/firmware/...）；文件名含时间戳+产品ID随机化
	r.Static("/uploads", "./uploads")

	return r
}
