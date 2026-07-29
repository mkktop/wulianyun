package main

import (
	"flag"
	"log/slog"
	"os"

	"iot-platform/internal/api"
	"iot-platform/internal/config"
	"iot-platform/internal/gateway"
	"iot-platform/internal/model"
	"iot-platform/internal/mqtt"
	"iot-platform/internal/poller"
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

	// Modbus 云端轮询引擎（注册 gateway 上下线钩子）
	poller.Init()

	// 下行分发：Modbus 产品走点位写入，其他 TCP 在线设备走网关透传，否则走 EMQX
	service.DownPublisher = func(productKey, deviceName string, payload []byte) error {
		if gateway.Has(productKey, deviceName) {
			var p model.Product
			if repository.DB.Select("access_mode").Where("product_key = ?", productKey).First(&p).Error == nil &&
				p.AccessMode == model.AccessModeModbus {
				return poller.WriteProperty(productKey, deviceName, payload)
			}
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
