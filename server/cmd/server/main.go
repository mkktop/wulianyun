package main

import (
	"flag"
	"log/slog"
	"os"

	"iot-platform/internal/api"
	"iot-platform/internal/config"
	"iot-platform/internal/gateway"
	"iot-platform/internal/mqtt"
	"iot-platform/internal/repository"
	"iot-platform/internal/rule"
	"iot-platform/internal/service"
)

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
	api.EnsureAdmin()

	// MQTT 后台连接（EMQX 未启动时自动重连，不阻塞 HTTP 服务）
	mqtt.Start()

	// TCP 透传网关（DTU）
	gateway.Start()

	// 下行分发：TCP 在线设备走网关，否则走 EMQX
	service.DownPublisher = func(productKey, deviceName string, payload []byte) error {
		if gateway.Has(productKey, deviceName) {
			return gateway.Send(productKey, deviceName, payload)
		}
		return mqtt.PublishDown(productKey, deviceName, payload)
	}

	// 离线告警巡检
	rule.StartOfflineChecker()

	r := api.NewRouter()
	slog.Info("iot-platform server started", "addr", config.C.Server.Addr)
	if err := r.Run(config.C.Server.Addr); err != nil {
		slog.Error("server exited", "err", err)
		os.Exit(1)
	}
}
