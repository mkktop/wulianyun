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

// ForwardToKafka Kafka 转发（当前为 placeholder，待引入 sarama 依赖后实现）
func ForwardToKafka(brokers []string, topic string, d *model.Device, payload []byte) {
	if len(brokers) == 0 || topic == "" {
		return
	}
	// 注意：Kafka 需要 github.com/IBM/sarama 依赖
	// 当前先用日志占位，待引入依赖后实现
	// 如果项目不想引入 sarama 依赖，可以用 HTTP 代理方式转发到 Kafka
	slog.Info("kafka forward (placeholder)", "brokers", brokers, "topic", topic, "device", d.Name)

	// 实际实现需要：
	// 1. 创建/复用 sarama.SyncProducer
	// 2. 发送 &sarama.ProducerMessage{Topic: topic, Value: sarama.ByteEncoder(payload)}
}

// ---- MQTT 桥接转发 ----

var (
	bridgeMu      sync.Mutex
	bridgeClients = make(map[string]mqtt.Client) // broker URL -> client
)

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

func getBridgeClient(broker, username, password string) mqtt.Client {
	bridgeMu.Lock()
	defer bridgeMu.Unlock()

	if c, ok := bridgeClients[broker]; ok && c.IsConnected() {
		return c
	}

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
	if !token.WaitTimeout(5*time.Second) || token.Error() != nil {
		slog.Error("mqtt bridge connect failed", "broker", broker, "error", token.Error())
		return nil
	}

	bridgeClients[broker] = c
	return c
}
