package api

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
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

	// Swagger 接口文档（生产环境通过 nginx 不暴露 /swagger，仅内网可访问，避免接口细节泄露）
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

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

			// ---- 只读接口 ----
			auth.GET("/products", ListProducts)
			auth.GET("/products/:id", GetProduct)
			auth.GET("/products/:id/thing-model", GetThingModel)
			auth.GET("/products/:id/tsl/export", ExportThingModel)
			auth.POST("/products/:id/codec/test", TestCodec) // 测试解析，不产生变更
			auth.GET("/products/:id/codec", GetCodec)
			auth.GET("/products/:id/modbus-points", GetModbusPoints)
			auth.POST("/products/:id/modbus-points/test", TestModbusPoint) // 测试解析，不产生变更
			auth.GET("/products/:id/modbus-groups", ListModbusGroups)
			auth.GET("/products/:id/stats", ProductStats)
			auth.GET("/products/:id/device-alarm-stats", DeviceAlarmStats)
			auth.GET("/products/:id/config", GetRemoteConfig)

			auth.GET("/devices", ListDevices)
			auth.GET("/devices/:id", GetDevice)
			auth.GET("/devices/:id/events", ListDeviceEvents)
			auth.GET("/devices/:id/latest", DeviceLatest)
			auth.GET("/devices/:id/history", DeviceHistory)
			auth.GET("/devices/:id/shadow", GetDeviceShadow)

			auth.GET("/rules", ListRules)

			auth.GET("/alarms", ListAlarms)
			auth.GET("/alarms/stats", AlarmStats)
			auth.GET("/alarms/trend", AlarmTrend)

			auth.GET("/apps", ListOpenApps)
			auth.GET("/event-reports", ListEventReports)
			auth.GET("/command-logs", ListCommandLogs)
			auth.GET("/groups", ListGroups)

			auth.GET("/traces", ListTraces)
			auth.GET("/traces/:traceId", GetTrace)
			auth.GET("/devices/:id/logs", ListDeviceLogs)
			auth.GET("/device-logs", ListAllDeviceLogs)
			auth.GET("/devices/:id/sub-devices", ListSubDevices)
			auth.GET("/firmwares", ListFirmwares)
			auth.GET("/ota-tasks", ListOTATasks)
			auth.GET("/mqtt-debug/ws", MqttDebugWS)

			// ---- 写操作（查看者账号被 RequireOperate 拦截）----
			write := auth.Group("", RequireOperate())
			{
				write.POST("/products", CreateProduct)
				write.PUT("/products/:id", UpdateProduct)
				write.DELETE("/products/:id", DeleteProduct)
				write.PUT("/products/:id/thing-model", SaveThingModel)
				write.POST("/products/:id/tsl/import", ImportThingModel)
				write.PUT("/products/:id/codec", SaveCodec)
				write.PUT("/products/:id/modbus-points", SaveModbusPoints)
				write.POST("/products/:id/modbus-groups", CreateModbusGroup)
				write.PUT("/products/:id/modbus-groups/:gid", UpdateModbusGroup)
				write.DELETE("/products/:id/modbus-groups/:gid", DeleteModbusGroup)
				write.PUT("/products/:id/config", SaveRemoteConfig)
				write.POST("/products/:id/config/push", PushRemoteConfig)
				write.POST("/products/:id/broadcast", BroadcastProduct)
				write.POST("/products/:id/devices/batch", BatchCreateDevices)

				write.POST("/devices", CreateDevice)
				write.PUT("/devices/:id", UpdateDevice)
				write.DELETE("/devices/:id", DeleteDevice)
				write.POST("/devices/:id/command", SendCommand)
				write.POST("/devices/:id/property", SetDeviceProperty)
				write.POST("/devices/:id/service", InvokeService)

				write.POST("/rules", CreateRule)
				write.PUT("/rules/:id", UpdateRule)
				write.DELETE("/rules/:id", DeleteRule)

				write.POST("/alarms/:id/resolve", ResolveAlarm)
				write.POST("/alarms/:id/confirm", ConfirmAlarm)

				write.POST("/apps", CreateOpenApp)
				write.PUT("/apps/:id", UpdateOpenApp)
				write.DELETE("/apps/:id", DeleteOpenApp)

				write.POST("/groups", CreateGroup)
				write.PUT("/groups/:id", UpdateGroup)
				write.DELETE("/groups/:id", DeleteGroup)

				write.POST("/devices/:id/sub-devices", AddSubDevice)
				write.DELETE("/devices/:id/sub-devices/:subId", RemoveSubDevice)

				write.POST("/firmwares", CreateFirmware)
				write.DELETE("/firmwares/:id", DeleteFirmware)
				write.POST("/ota-tasks", CreateOTATask)
			}

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

			// 开发者工具（模拟器会产生下行/上报，属写操作）
			sim := auth.Group("/simulator")
			sim.GET("/sessions", ListSimulatorSessions)
			sim.POST("/connect", RequireOperate(), ConnectSimulator)
			sim.POST("/publish", RequireOperate(), PublishSimulator)
			sim.POST("/disconnect", RequireOperate(), DisconnectSimulator)
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
