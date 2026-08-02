package mqtt

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"

	"iot-platform/internal/config"
	"iot-platform/internal/model"
	"iot-platform/internal/repository"
	"iot-platform/internal/service"
)

var client paho.Client

const (
	TopicUpPrefix   = "thing/up/"        // thing/up/{productKey}/{deviceName}
	TopicDownPrefix = "thing/down/"      // thing/down/{productKey}/{deviceName}
	TopicBroadcast  = "thing/broadcast/" // thing/broadcast/{productKey}
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

	// TLS 支持
	if config.C.MQTT.TLS.Enabled {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: config.C.MQTT.TLS.InsecureSkipVerify,
		}
		if config.C.MQTT.TLS.ClientCert != "" && config.C.MQTT.TLS.ClientKey != "" {
			cert, err := tls.LoadX509KeyPair(config.C.MQTT.TLS.ClientCert, config.C.MQTT.TLS.ClientKey)
			if err != nil {
				slog.Error("load mqtt tls cert failed", "error", err)
			} else {
				tlsConfig.Certificates = []tls.Certificate{cert}
			}
		}
		opts.SetTLSConfig(tlsConfig)
	}

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
		// OTA 进度上报单独分流：thing/up/{pk}/{dn}/ota
		if len(parts) >= 5 && parts[4] == "ota" {
			var d model.Device
			if err := repository.DB.Where("product_key = ? AND name = ?", parts[2], parts[3]).First(&d).Error; err != nil {
				return
			}
			go service.HandleOTAProgress(d.ID, m.Payload())
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

// PublishBroadcast 向产品广播主题发布消息（该产品下订阅的设备均收到）
func PublishBroadcast(productKey string, payload []byte) error {
	if client == nil || !client.IsConnected() {
		return fmt.Errorf("mqtt broker 未连接")
	}
	token := client.Publish(TopicBroadcast+productKey, 1, false, payload)
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("发布超时")
	}
	return token.Error()
}

// Client 暴露内部 paho.Client
func Client() paho.Client {
	return client
}

// SubscribeReply 订阅设备指令应答主题
func SubscribeReply() {
	if client == nil || !client.IsConnected() {
		return
	}
	topic := "thing/up/+/+/reply"
	client.Subscribe(topic, 1, func(c paho.Client, msg paho.Message) {
		go service.HandleDeviceReply(msg.Topic(), msg.Payload())
	})
	slog.Info("subscribed to reply topic", "topic", topic)
}
