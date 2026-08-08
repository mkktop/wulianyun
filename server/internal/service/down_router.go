package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/redis/go-redis/v9"
)

// DownMessage 跨实例下行消息
type DownMessage struct {
	ProductKey string `json:"productId"`
	DeviceName string `json:"deviceName"`
	Payload    []byte `json:"payload"`
}

const downChannel = "tcp:down"

// downRDB Redis 客户端（nil 时退化为单实例模式）
var downRDB *redis.Client

// DownLocal 由 main 注入：仅向本实例 TCP 连接发送（gateway.Send），
// 本实例无该设备连接时返回错误。订阅端专用，避免经 DownPublisher 递归扇出
var DownLocal func(productKey, deviceName string, payload []byte) error

// InitDownRouter 初始化跨实例下行路由器，传入 Redis 客户端和本实例的发送函数
func InitDownRouter(rdb *redis.Client) {
	downRDB = rdb
	if downRDB == nil {
		return
	}
	go downSubscribe()
	slog.Info("down router started", "channel", downChannel)
}

// PublishDown 跨实例下发 TCP 设备：广播到 Redis，各实例收到后检查本地 TCP 连接投递
func PublishDown(productKey, deviceName string, payload []byte) error {
	if downRDB == nil {
		return errors.New("跨实例下行通道未初始化（单实例模式）")
	}
	msg := DownMessage{ProductKey: productKey, DeviceName: deviceName, Payload: payload}
	data, _ := json.Marshal(msg)
	return downRDB.Publish(context.Background(), downChannel, data).Err()
}

func downSubscribe() {
	pubsub := downRDB.Subscribe(context.Background(), downChannel)
	defer pubsub.Close()
	ch := pubsub.Channel()
	for msg := range ch {
		var dm DownMessage
		if err := json.Unmarshal([]byte(msg.Payload), &dm); err != nil {
			continue
		}
		// 仅当本实例有该设备连接时才发送（DownLocal 内部判断）；
		// 不经 DownPublisher，避免递归扇出死循环
		if DownLocal != nil {
			if err := DownLocal(dm.ProductKey, dm.DeviceName, dm.Payload); err != nil {
				slog.Debug("down fanout: local connection not found", "device", dm.ProductKey+"."+dm.DeviceName)
			}
		}
	}
}
