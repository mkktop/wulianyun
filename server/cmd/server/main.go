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

	// 初始化性能组件
	service.InitDeviceCache(config.C.Cache.DeviceTTL)
	service.InitTelemetryBuffer(config.C.TelemetryBuffer.MaxBatch, config.C.TelemetryBuffer.FlushInterval)
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

	// 下行分发：Modbus 产品走点位写入，其他 TCP 在线设备走网关透传，否则走 EMQX；统一记录下发日志
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
			err = mqtt.PublishDown(productKey, deviceName, payload)
		}
		go service.LogCommand(productKey, deviceName, channel, payload, err)
		return err
	}

	// 广播分发：TCP 在线设备逐连接下发 + MQTT 广播主题
	service.Broadcaster = func(productKey string, payload []byte) error {
		gateway.Broadcast(productKey, payload)
		return mqtt.PublishBroadcast(productKey, payload)
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

	// shutdown 时逆序关闭性能组件
	service.ShutdownShadowCache()
	service.ShutdownTelemetryBuffer()

	// 优雅关闭 HTTP 服务
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown failed", "err", err)
	}

	slog.Info("server exited gracefully")
}
