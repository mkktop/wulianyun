package service

import (
	"context"
	"encoding/json"
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

// InitDownRouter 初始化跨实例下行路由器，传入 Redis 客户端和本实例的发送函数
func InitDownRouter(rdb *redis.Client) {
	downRDB = rdb
	if downRDB == nil {
		return
	}
	go downSubscribe()
	slog.Info("down router started", "channel", downChannel)
}

// PublishDown 跨实例发送下行消息：本实例有 TCP 连接则直接发送，同时广播到 Redis 让其他实例也尝试
func PublishDown(productKey, deviceName string, payload []byte) error {
	// 本实例先尝试直接发送（DownPublisher 已在 main.go 中注入）
	if DownPublisher != nil {
		if err := DownPublisher(productKey, deviceName, payload); err == nil {
			return nil
		}
	}

	// 广播到 Redis，其他实例收到后检查本地 TCP 连接
	if downRDB != nil {
		msg := DownMessage{ProductKey: productKey, DeviceName: deviceName, Payload: payload}
		data, _ := json.Marshal(msg)
		_ = downRDB.Publish(context.Background(), downChannel, data).Err()
	}
	return nil
}

// downSubscriber 监听 Redis 下行频道，收到消息后通过本实例的 DownPublisher 发送
var downSubscriber *redis.Client

func downSubscribe() {
	pubsub := downRDB.Subscribe(context.Background(), downChannel)
	defer pubsub.Close()
	ch := pubsub.Channel()
	for msg := range ch {
		var dm DownMessage
		if err := json.Unmarshal([]byte(msg.Payload), &dm); err != nil {
			continue
		}
		// 仅当本实例有该设备连接时才发送（DownPublisher 内部判断）
		if DownPublisher != nil {
			DownPublisher(dm.ProductKey, dm.DeviceName, dm.Payload)
		}
	}
}
