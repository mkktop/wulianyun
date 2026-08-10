package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"iot-platform/internal/api"
	"iot-platform/internal/config"
	_ "iot-platform/internal/docs" // Swagger 文档（swag init 生成，import 触发 SwaggerInfo 注册）
	"iot-platform/internal/gateway"
	"iot-platform/internal/model"
	"iot-platform/internal/mqtt"
	"iot-platform/internal/poller"
	"iot-platform/internal/repository"
	"iot-platform/internal/rule"
	"iot-platform/internal/service"
	"iot-platform/internal/storage"
	"iot-platform/internal/ws"
)

// @title           KK 物联云 API
// @version         1.0
// @description     企业 IoT 设备管理平台接口文档（设备 / 产品 / 物模型 / 规则 / 告警 / OTA / 开放平台）
// @host            localhost:8080
// @BasePath        /api/v1
// @securityDefinitions.apikey BearerAuth
// @in   header
// @name Authorization
func main() {
	confPath := flag.String("conf", "configs/config.yaml", "配置文件路径")
	flag.Parse()

	if err := config.Load(*confPath); err != nil {
		slog.Error("load config failed", "err", err)
		os.Exit(1)
	}
	if err := repository.Init(); err != nil {
		slog.Error("init database failed", "err", err)
		os.Exit(1)
	}
	// 固件对象存储（local 本地磁盘 | s3 对象存储），启动即校验桶可用
	if err := storage.Init(storage.Options{
		Type:         config.C.Storage.Type,
		LocalDir:     config.C.Storage.LocalDir,
		Endpoint:     config.C.Storage.Endpoint,
		Region:       config.C.Storage.Region,
		Bucket:       config.C.Storage.Bucket,
		AccessKey:    config.C.Storage.AccessKey,
		SecretKey:    config.C.Storage.SecretKey,
		UseSSL:       config.C.Storage.UseSSL,
		PublicDomain: config.C.Storage.PublicDomain,
	}); err != nil {
		slog.Error("init storage failed", "err", err)
		os.Exit(1)
	}

	// 初始化性能组件
	service.InitDeviceCache(config.C.Cache.DeviceTTL)
	service.InitTelemetryBuffer(config.C.TelemetryBuffer.MaxBatch, config.C.TelemetryBuffer.FlushInterval)
	service.InitLogBuffer(200, 2) // 轨迹/设备日志批量化，避免每包 INSERT 抢占连接池（#17）
	service.InitShadowCache(config.C.Cache.ShadowFlushInterval)
	poller.InitPollerSemaphore(config.C.Poller.MaxConcurrent)

	// 多实例：Redis Pub/Sub 跨实例广播 WebSocket 消息
	ws.RedisPubSub = repository.NewRedisPubSub()
	ws.StartPubSub()

	// 多实例：轮询分布式锁初始化
	poller.InitDistributedLock()

	// 多实例：规则静默缓存迁移到 Redis
	rule.UseRedisSilence(repository.RDB)

	// 一级账号实时接收二级账号设备的告警（fan-out 注入）
	rule.RecipientResolver = service.PushRecipients

	// WebSocket 首帧认证注入（token 不放进 URL，防泄露进访问日志）
	ws.ValidateToken = api.ValidateWSToken

	// 多实例：TCP 下行通道跨实例路由
	service.InitDownRouter(repository.RDB)

	api.EnsureAdmin()

	// MQTT 后台连接（EMQX 未启动时自动重连，不阻塞 HTTP 服务）
	mqtt.Start()
	mqtt.SubscribeReply()

	// TCP 透传网关（DTU）
	gateway.Start()

	// Modbus 云端轮询引擎（注册 gateway 上下线钩子）
	poller.Init()

	// 下行分发：本实例 TCP 连接（Modbus 点位写入 / 网关透传）→ 直发；
	// 无本实例连接时按产品协议分流：TCP 设备走 Redis 扇出（其他实例的 gateway 投递），
	// MQTT 设备走 EMQX；统一记录下发日志
	service.DownLocal = gateway.Send
	service.DownPublisher = func(productKey, deviceName string, payload []byte) error {
		var err error
		channel := "mqtt"
		if gateway.Has(productKey, deviceName) {
			var p model.Product
			if repository.DB.Select("access_mode").Where("product_key = ?", productKey).First(&p).Error == nil &&
				p.AccessMode == model.AccessModeModbus {
				channel = "modbus"
				err = poller.WriteProperty(productKey, deviceName, payload)
			} else {
				channel = "tcp"
				err = gateway.Send(productKey, deviceName, payload)
			}
		} else {
			// 无本实例 TCP 连接：按产品协议分流
			var p model.Product
			if repository.DB.Select("protocol").Where("product_key = ?", productKey).First(&p).Error == nil &&
				p.Protocol == "tcp" {
				// TCP/DTU 设备（可能连接在其他实例）：Redis 扇出
				channel = "tcp"
				err = service.PublishDown(productKey, deviceName, payload)
			} else {
				// MQTT 设备：EMQX 下发
				err = mqtt.PublishDown(productKey, deviceName, payload)
			}
		}
		go service.LogCommand(productKey, deviceName, channel, payload, err)
		return err
	}

	// 广播分发：TCP 在线设备逐连接下发（codec 编码）+ MQTT 广播主题
	service.Broadcaster = func(productKey string, payload []byte) error {
		tcpErr := gateway.Broadcast(productKey, payload)
		mqttErr := mqtt.PublishBroadcast(productKey, payload)
		if tcpErr != nil {
			return tcpErr
		}
		return mqttErr
	}

	// 影子期望值 retained 下发（设备订阅时必达；空 payload 清除）
	service.DownRetainedPublisher = func(productKey, deviceName string, payload []byte) error {
		return mqtt.PublishDownRetained(productKey, deviceName, payload)
	}

	// 离线告警巡检
	rule.StartOfflineChecker()

	r := api.NewRouter()
	srv := &http.Server{
		Addr:    config.C.Server.Addr,
		Handler: r,
	}

	// 监听系统信号，优雅关闭
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("iot-platform server started", "addr", config.C.Server.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server exited", "err", err)
			os.Exit(1)
		}
	}()

	// 等待中断信号
	<-ctx.Done()
	slog.Info("shutting down server...")

	// 关闭顺序（数据可靠性）：
	// 1. 先停止接收新 HTTP 请求（在途请求最多 5s 内完成，期间遥测入口仍在写缓冲，不丢数据）
	// 2. 再停止遥测入口（MQTT 断开 + TCP 网关停止监听），避免新数据进入已停止的缓冲
	// 3. 最后逆序关闭性能组件：缓冲最后关，且等待最后一次 flush 完成
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown failed", "err", err)
	}

	mqtt.Disconnect()
	gateway.Stop()

	service.ShutdownShadowCache()
	service.ShutdownLogBuffer()
	service.ShutdownTelemetryBuffer()

	slog.Info("server exited gracefully")
}
