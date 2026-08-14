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
	TopicUpPrefix    = "thing/up/"        // thing/up/{productKey}/{deviceName}
	TopicDownPrefix  = "thing/down/"      // thing/down/{productKey}/{deviceName}
	TopicBroadcast   = "thing/broadcast/" // thing/broadcast/{productKey}
	TopicOffline     = "thing/offline/"   // thing/offline/{productKey}/{deviceName} — LWT 遗嘱
	TopicGateway     = "thing/gateway/"   // thing/gateway/{pk}/{dn}/sub/{subId}/login|logout
)

// QoS 级别
const (
	QoS0 = 0 // 至多一次（火忘）
	QoS1 = 1 // 至少一次（默认）
	QoS2 = 2 // 精确一次（关键指令）
)

// Start 连接 EMQX，订阅设备上行与系统上下线事件；broker 不可用时后台自动重连
func Start() {
	clientID := config.C.MQTT.ClientID
	opts := paho.NewClientOptions().
		AddBroker(config.C.MQTT.Broker).
		SetClientID(clientID).
		SetUsername(config.C.MQTT.Username).
		SetPassword(config.C.MQTT.Password).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(5 * time.Second).
		SetCleanSession(false). // 持久会话：重连后补发离线消息
		SetOnConnectHandler(func(c paho.Client) {
			slog.Info("mqtt connected", "broker", config.C.MQTT.Broker)
			subscribe(c)
			SubscribeReply() // 重连时重订 reply 主题（会话被 broker 清理时兜底）
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

// maxTelemetryPayload MQTT 上行 payload 上限（与 codec/HTTP 入口对齐）：
// paho 不限制消息大小，超大 payload 的 JSON 解析会内存放大
const maxTelemetryPayload = 256 << 10

func subscribe(c paho.Client) {
	// 设备遥测上行
	c.Subscribe(TopicUpPrefix+"#", QoS1, func(_ paho.Client, m paho.Message) {
		parts := strings.Split(m.Topic(), "/")
		// thing/up/{productKey}/{deviceName}[/...]
		if len(parts) < 4 {
			return
		}
		// 指令应答主题由 SubscribeReply 专门处理，不按遥测合并进最新值
		if len(parts) >= 5 && parts[4] == "reply" {
			return
		}
		// OTA 进度上报：thing/up/{pk}/{dn}/ota
		if len(parts) >= 5 && parts[4] == "ota" {
			// 设备查询移出 paho 分发协程：万级设备升级风暴时，
			// 同步查库会串行阻塞整个 thing/up/# 的消息分发（含全部遥测）
			pk, dn := parts[2], parts[3]
			go func() {
				var d model.Device
				if err := repository.DB.Where("product_key = ? AND name = ?", pk, dn).First(&d).Error; err != nil {
					return
				}
				service.HandleOTAProgress(d.ID, m.Payload())
			}()
			return
		}
		if len(m.Payload()) > maxTelemetryPayload {
			slog.Warn("mqtt telemetry payload too large, dropped", "topic", m.Topic(), "size", len(m.Payload()))
			return
		}
		// 有界 worker 池（按设备哈希分片保序），替代无界 go，防 DB 抖动时协程堆积
		service.SubmitTelemetry(parts[2], parts[3], m.Payload())
	})

	// 设备遗嘱消息（LWT）：thing/offline/{pk}/{dn}
	// 仅记录日志；状态以上下线 $SYS 事件为准（带时间戳可去重），
	// LWT 无时间戳且与 $SYS disconnected 重复，参与状态处理会并发乱序覆盖新状态
	c.Subscribe(TopicOffline+"+", QoS1, func(_ paho.Client, m paho.Message) {
		parts := strings.Split(m.Topic(), "/")
		// thing/offline/{pk}/{dn}
		if len(parts) != 4 {
			return
		}
		clientID := parts[2] + "." + parts[3]
		slog.Info("lwt offline detected", "device", clientID)
	})

	// 子设备网关协议：thing/gateway/{pk}/{dn}/sub/{subId}/login|logout
	c.Subscribe(TopicGateway+"+/+/sub/+/+", QoS1, func(_ paho.Client, m paho.Message) {
		go service.HandleGatewaySubDevice(m.Topic(), m.Payload())
	})

	// EMQX 系统事件：上下线（作为 LWT 的补充，处理 clean disconnect 场景）
	c.Subscribe("$SYS/brokers/+/clients/+/connected", QoS1, func(_ paho.Client, m paho.Message) {
		handleSysEvent(m.Payload(), true)
	})
	c.Subscribe("$SYS/brokers/+/clients/+/disconnected", QoS1, func(_ paho.Client, m paho.Message) {
		handleSysEvent(m.Payload(), false)
	})
}

func handleSysEvent(payload []byte, online bool) {
	var evt struct {
		ClientID string `json:"clientid"`
		Username string `json:"username"`
		Ts       int64  `json:"ts"`
	}
	if err := json.Unmarshal(payload, &evt); err != nil || evt.ClientID == "" {
		return
	}
	// 跳过平台内部连接
	if evt.ClientID == config.C.MQTT.ClientID {
		return
	}
	// 时间戳单位兼容：部分 EMQX 版本/事件的 ts 为秒级。统一归一为毫秒——
	// 若把秒当毫秒，last_online/offline 会被写成 1970 年，离线告警判定整体失效
	ts := evt.Ts
	if ts > 0 && ts < 1e11 {
		ts *= 1000
	}
	service.QueueStatus(evt.ClientID, online, ts)
}

// Disconnect 主动断开 broker 连接（优雅关闭时停止接收设备上行）
func Disconnect() {
	if client != nil && client.IsConnected() {
		client.Disconnect(1000)
	}
}

// PublishDown 向设备下行主题发布消息（QoS 1）
func PublishDown(productKey, deviceName string, payload []byte) error {
	return PublishDownWithQoS(productKey, deviceName, payload, QoS1)
}

// PublishDownWithQoS 向设备下行主题发布消息（指定 QoS）
func PublishDownWithQoS(productKey, deviceName string, payload []byte, qos int) error {
	if client == nil || !client.IsConnected() {
		return fmt.Errorf("mqtt broker 未连接")
	}
	topic := TopicDownPrefix + productKey + "/" + deviceName
	token := client.Publish(topic, byte(qos), false, payload)
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("发布超时")
	}
	return token.Error()
}

// PublishDownRetained 发布 Retained 消息（设备上线后从 broker 立即获取最新期望值/配置）
func PublishDownRetained(productKey, deviceName string, payload []byte) error {
	if client == nil || !client.IsConnected() {
		return fmt.Errorf("mqtt broker 未连接")
	}
	topic := TopicDownPrefix + productKey + "/" + deviceName + "/retained"
	token := client.Publish(topic, QoS1, true, payload) // retained=true
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
	token := client.Publish(TopicBroadcast+productKey, QoS1, false, payload)
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("发布超时")
	}
	return token.Error()
}

// PublishGatewayDown 向网关子设备下行主题发布消息
func PublishGatewayDown(productKey, gatewayName, subDeviceName string, payload []byte) error {
	if client == nil || !client.IsConnected() {
		return fmt.Errorf("mqtt broker 未连接")
	}
	topic := TopicGateway + productKey + "/" + gatewayName + "/sub/" + subDeviceName
	token := client.Publish(topic, QoS1, false, payload)
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("发布超时")
	}
	return token.Error()
}

// ClearRetained 清除指定设备的 Retained 消息（发布空 payload 到 Retained topic）
func ClearRetained(productKey, deviceName string) error {
	if client == nil || !client.IsConnected() {
		return fmt.Errorf("mqtt broker 未连接")
	}
	topic := TopicDownPrefix + productKey + "/" + deviceName + "/retained"
	token := client.Publish(topic, QoS1, true, []byte(""))
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("发布超时")
	}
	return token.Error()
}

// Client 暴露内部 paho.Client
func Client() paho.Client {
	return client
}

// SubscribeReply 订阅设备指令应答主题。
// 在 subscribe（重连即重订）中一并调用：持久会话被 broker 清理（EMQX 重启/迁移）时，
// 仅在进程启动时订阅一次会丢失 reply 订阅，指令应答将永远无人处理
func SubscribeReply() {
	if client == nil || !client.IsConnected() {
		return
	}
	topic := "thing/up/+/+/reply"
	client.Subscribe(topic, QoS1, func(c paho.Client, msg paho.Message) {
		go service.HandleDeviceReply(msg.Topic(), msg.Payload())
	})
	slog.Info("subscribed to reply topic", "topic", topic)
}
