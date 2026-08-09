package rule

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"iot-platform/internal/model"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// ForwardData 根据 action type 分发到对应的转发函数
func ForwardData(r *model.Rule, d *model.Device, data map[string]interface{}) {
	var act struct {
		Type     string   `json:"type"`
		Topic    string   `json:"topic"`
		Brokers  []string `json:"brokers"`
		Broker   string   `json:"broker"`
		Username string   `json:"username"`
		Password string   `json:"password"`
	}
	if err := json.Unmarshal(r.Action, &act); err != nil {
		slog.Error("parse forward action failed", "error", err)
		return
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"device": d.Name, "product": d.ProductKey, "data": data, "ts": time.Now().UnixMilli(),
	})
	switch act.Type {
	case "kafka":
		go ForwardToKafka(act.Brokers, act.Topic, d, payload)
	case "mqtt_bridge":
		go ForwardToMqttBridge(act.Broker, act.Topic, act.Username, act.Password, payload)
	}
}

// ForwardToKafka Kafka 转发（未实现）。为避免静默丢数据，明确告警并不做任何"成功"伪装；
// 真正实现需引入 sarama，在此之前规则不应允许 type=kafka（CreateRule/UpdateRule 已在 API 层拒绝）
func ForwardToKafka(brokers []string, topic string, d *model.Device, payload []byte) {
	slog.Error("kafka forward not implemented (rule engine accepted kafka type unexpectedly)",
		"brokers", brokers, "topic", topic, "device", d.Name)
}

// ---- MQTT 桥接转发 ----

var (
	bridgeMu      sync.Mutex
	bridgeClients = make(map[string]mqtt.Client) // broker URL -> client
	bridgePending = make(map[string]*pendingBridge)
)

// pendingBridge singleflight：同一 broker 的并发连接请求只发起一次 Connect，其余等待结果
type pendingBridge struct {
	done chan struct{}
	c    mqtt.Client
	ok   bool
}

// ForwardToMqttBridge MQTT 桥接转发（懒初始化连接，自动复用）
func ForwardToMqttBridge(broker, topic, username, password string, payload []byte) {
	if broker == "" || topic == "" {
		return
	}

	client := getBridgeClient(broker, username, password)
	if client == nil {
		slog.Error("mqtt bridge client not available", "broker", broker)
		return
	}

	token := client.Publish(topic, 1, false, payload)
	if token.WaitTimeout(5*time.Second) && token.Error() != nil {
		slog.Error("mqtt bridge publish failed", "broker", broker, "topic", topic, "error", token.Error())
	}
}

// getBridgeClient 复用 broker 连接；锁内只读写 map，阻塞 Connect 移到锁外（per-broker singleflight）
func getBridgeClient(broker, username, password string) mqtt.Client {
	// 快路径：锁内查 map，命中且连接健康直接返回
	bridgeMu.Lock()
	if c, ok := bridgeClients[broker]; ok && c.IsConnected() {
		bridgeMu.Unlock()
		return c
	}
	// 已有 pending 的 singleflight，登记等待
	if p, ok := bridgePending[broker]; ok {
		bridgeMu.Unlock()
		<-p.done
		return p.c
	}
	p := &pendingBridge{done: make(chan struct{})}
	bridgePending[broker] = p
	bridgeMu.Unlock()

	// 锁外阻塞连接（一个不可达 broker 最多阻塞一次 Connect，不拖慢其他 broker/租户）
	opts := mqtt.NewClientOptions().
		AddBroker(broker).
		SetClientID(fmt.Sprintf("kk-iot-bridge-%d", time.Now().UnixNano())).
		SetAutoReconnect(true).
		SetConnectTimeout(5 * time.Second)
	if username != "" {
		opts.SetUsername(username)
		opts.SetPassword(password)
	}
	c := mqtt.NewClient(opts)
	token := c.Connect()
	ok := token.WaitTimeout(5*time.Second) && token.Error() == nil
	if !ok {
		slog.Error("mqtt bridge connect failed", "broker", broker, "error", token.Error())
		c = nil
	}

	bridgeMu.Lock()
	// 覆盖前先 Disconnect 旧客户端，防止 AutoReconnect 后台协程与 socket 泄漏（#27）
	if ok {
		if old, exists := bridgeClients[broker]; exists && old != c {
			old.Disconnect(500)
		}
		bridgeClients[broker] = c
	}
	delete(bridgePending, broker)
	bridgeMu.Unlock()

	close(p.done)
	p.c, p.ok = c, ok
	return c
}
