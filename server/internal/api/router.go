package api

import (
	"net/http"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// onlyFilesFS 拒绝目录访问（与 gin.Static 行为一致），避免 /uploads 目录列举
type onlyFilesFS struct{ fs http.FileSystem }

func (fs onlyFilesFS) Open(name string) (http.File, error) {
	f, err := fs.fs.Open(name)
	if err != nil {
		return nil, err
	}
	if stat, err := f.Stat(); err == nil && stat.IsDir() {
		f.Close()
		return nil, os.ErrNotExist
	}
	return f, nil
}

// uploadsDownload /uploads 静态文件下载：
// 固定 Content-Type: application/octet-stream + Content-Disposition: attachment + nosniff，
// 使任何上传内容（即使扩展名为 .html）都只能被下载、不能被浏览器同源渲染
func uploadsDownload() gin.HandlerFunc {
	fs := http.StripPrefix("/uploads", http.FileServer(onlyFilesFS{http.Dir("./uploads")}))
	return func(c *gin.Context) {
		c.Header("Content-Type", "application/octet-stream")
		c.Header("Content-Disposition", "attachment")
		c.Header("X-Content-Type-Options", "nosniff")
		fs.ServeHTTP(c.Writer, c.Request)
	}
}

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
		// 健康检查探针（无需鉴权，供 Docker healthcheck / 部署脚本就绪探测）
		v1.GET("/healthz", Healthz)
		v1.GET("/readyz", Readyz)

		// 公开接口
		v1.POST("/auth/register", Register)
		v1.POST("/auth/login", Login)
		v1.POST("/auth/token", DeviceToken) // 设备动态获取 MQTT Token
		v1.POST("/emqx/auth", EmqxAuth) // EMQX 认证回调（内网）
		v1.POST("/emqx/acl", EmqxACL)   // EMQX 授权回调（内网）
		v1.POST("/http/telemetry", HTTPDeviceTelemetry) // 设备HTTP上报（Token认证）

		// WebSocket 实时推送：公开升级，首帧 {type:"auth",token} 认证（token 不进 URL）
		v1.GET("/ws", WSHandler)

		// 需登录
		auth := v1.Group("", JWTAuth())
		{
			auth.GET("/auth/profile", Profile)
			auth.POST("/auth/change-password", ChangePassword)
			auth.GET("/overview", Overview)

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
			auth.GET("/devices/:id/export", ExportDeviceHistory)
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
			auth.GET("/mqtt-debug/ws", AdminAuth(), MqttDebugWS)

			// 用户侧：公告与帮助中心（只读）
			auth.GET("/announcements", ListPublishedAnnouncements)
			auth.GET("/help-docs", ListHelpDocs)
			auth.GET("/help-docs/:key", GetHelpDoc)

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

			// 平台超管专属后台（系统状态/配置/热更新参数/公告/帮助中心/全量用户管理/全局日志）
			admin := auth.Group("", AdminAuth())
			{
				admin.GET("/admin/system/status", SystemStatus)
				admin.GET("/admin/system/config", SystemConfig)
				admin.GET("/admin/system/settings", ListSystemSettings)
				admin.PUT("/admin/system/settings", UpdateSystemSetting)
				admin.GET("/admin/system/storage", GetStorageConfig)
				admin.PUT("/admin/system/storage", UpdateStorageConfig)

				admin.GET("/admin/announcements", ListAdminAnnouncements)
				admin.POST("/admin/announcements", CreateAnnouncement)
				admin.PUT("/admin/announcements/:id", UpdateAnnouncement)
				admin.DELETE("/admin/announcements/:id", DeleteAnnouncement)

				admin.GET("/admin/help-docs", ListAdminHelpDocs)
				admin.POST("/admin/help-docs", CreateHelpDoc)
				admin.PUT("/admin/help-docs/:id", UpdateHelpDoc)
				admin.DELETE("/admin/help-docs/:id", DeleteHelpDoc)

				admin.GET("/admin/users", ListAdminUsers)
				admin.POST("/admin/users", CreateAdminUser)
				admin.PUT("/admin/users/:id", UpdateAdminUser)
				admin.DELETE("/admin/users/:id", DeleteAdminUser)
			}

			// 开发者工具（模拟器会产生下行/上报，属写操作）
			sim := auth.Group("/simulator")
			sim.GET("/sessions", ListSimulatorSessions)
			sim.POST("/connect", RequireOperate(), ConnectSimulator)
			sim.POST("/publish", RequireOperate(), PublishSimulator)
			sim.POST("/disconnect", RequireOperate(), DisconnectSimulator)
		}
	}

	// OpenAPI：第三方应用签名访问，复用管理端处理器；写端点复用 RequireOperate（viewer 账号不能写设备）
	open := r.Group("/openapi/v1", OpenAPIAuth())
	{
		open.GET("/devices", ListDevices)
		open.GET("/devices/:id", GetDevice)
		open.GET("/devices/:id/latest", DeviceLatest)
		open.GET("/devices/:id/history", DeviceHistory)
		open.GET("/devices/:id/shadow", GetDeviceShadow)
		open.POST("/devices/:id/property", RequireOperate(), SetDeviceProperty)
		open.POST("/devices/:id/command", RequireOperate(), SendCommand)
	}
	// 静态资源：OTA 固件下载（fileURL 形如 /uploads/firmware/...）；文件名含时间戳+产品ID随机化
	// 强制以附件方式下载（octet-stream + attachment + nosniff），防止上传文件被同源渲染（存储型 XSS）
	r.GET("/uploads/*filepath", uploadsDownload())
	// 上传目录本身也禁止列目录/访问目录
	r.GET("/uploads", func(c *gin.Context) { c.Status(http.StatusNotFound) })

	return r
}
