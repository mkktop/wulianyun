package repository

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// redisPubSub 实现 Redis Pub/Sub，供 ws 包跨实例广播
type redisPubSub struct {
	rdb *redis.Client
}

// NewRedisPubSub 创建 Redis Pub/Sub 适配器
func NewRedisPubSub() *redisPubSub {
	return &redisPubSub{rdb: RDB}
}

// Publish 发布消息到指定频道
func (p *redisPubSub) Publish(ctx context.Context, channel string, message []byte) error {
	return p.rdb.Publish(ctx, channel, message).Err()
}

// Subscribe 订阅指定频道，返回消息 channel（每条消息为 []byte payload）
func (p *redisPubSub) Subscribe(ctx context.Context, channel string) <-chan []byte {
	pubsub := p.rdb.Subscribe(ctx, channel)
	out := make(chan []byte, 128)

	go func() {
		defer close(out)
		defer pubsub.Close()
		ch := pubsub.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				out <- []byte(msg.Payload)
			}
		}
	}()

	return out
}
