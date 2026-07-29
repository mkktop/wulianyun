package mqtt

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"

	"iot-platform/internal/config"
	"iot-platform/internal/service"
)

var client paho.Client

const (
	TopicUpPrefix   = "thing/up/"   // thing/up/{productKey}/{deviceName}
	TopicDownPrefix = "thing/down/" // thing/down/{productKey}/{deviceName}
)

// Start 连接 EMQX，订阅设备上行与系统上下线事件；broker 不可用时后台自动重连
func Start() {
	opts := paho.NewClientOptions().
		AddBroker(config.C.MQTT.Broker).
		SetClientID(config.C.MQTT.ClientID).
		SetUsername(config.C.MQTT.Username).
		SetPassword(config.C.MQTT.Password).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(5 * time.Second).
		SetOnConnectHandler(func(c paho.Client) {
			slog.Info("mqtt connected", "broker", config.C.MQTT.Broker)
			subscribe(c)
		}).
		SetConnectionLostHandler(func(c paho.Client, err error) {
			slog.Warn("mqtt connection lost", "err", err)
		})

	client = paho.NewClient(opts)
	if token := client.Connect(); token.WaitTimeout(10*time.Second) && token.Error() != nil {
		slog.Warn("mqtt initial connect failed, retrying in background", "err", token.Error())
	}
}

func subscribe(c paho.Client) {
	// 设备遥测上行
	c.Subscribe(TopicUpPrefix+"#", 1, func(_ paho.Client, m paho.Message) {
		parts := strings.Split(m.Topic(), "/")
		// thing/up/{productKey}/{deviceName}[/...]
		if len(parts) < 4 {
			return
		}
		go service.HandleTelemetry(parts[2], parts[3], m.Payload())
	})

	// EMQX 系统事件：上下线
	c.Subscribe("$SYS/brokers/+/clients/+/connected", 1, func(_ paho.Client, m paho.Message) {
		handleSysEvent(m.Payload(), true)
	})
	c.Subscribe("$SYS/brokers/+/clients/+/disconnected", 1, func(_ paho.Client, m paho.Message) {
		handleSysEvent(m.Payload(), false)
	})
}

func handleSysEvent(payload []byte, online bool) {
	var evt struct {
		ClientID string `json:"clientid"`
		Username string `json:"username"`
	}
	if err := json.Unmarshal(payload, &evt); err != nil || evt.ClientID == "" {
		return
	}
	// 跳过平台内部连接
	if evt.ClientID == config.C.MQTT.ClientID {
		return
	}
	go service.HandleDeviceStatus(evt.ClientID, online)
}

// PublishDown 向设备下行主题发布消息
func PublishDown(productKey, deviceName string, payload []byte) error {
	if client == nil || !client.IsConnected() {
		return fmt.Errorf("mqtt broker 未连接")
	}
	topic := TopicDownPrefix + productKey + "/" + deviceName
	token := client.Publish(topic, 1, false, payload)
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("发布超时")
	}
	return token.Error()
}
