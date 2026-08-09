package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"iot-platform/internal/mqtt"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	paho "github.com/eclipse/paho.mqtt.golang"
)

var wsUpgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

type mqttDebugSession struct {
	conn *websocket.Conn
	user string
	subs map[string]paho.Token
	send chan []byte
	mu   sync.Mutex
}

// MqttDebugWS 平台内部 MQTT 调试台。
// 安全约束：仅平台超管可用；禁止 $ 系统主题；publish 不允许通配符。
// 该连接使用平台内部超级用户客户端，任意登录用户（含只读 viewer）不得触达。
func MqttDebugWS(c *gin.Context) {
	if !IsAdmin(c) {
		Fail(c, 403, "仅平台超管可调试 MQTT")
		return
	}
	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Error("mqtt debug ws upgrade failed", "error", err)
		return
	}
	sess := &mqttDebugSession{
		conn: conn,
		user: c.GetString("username"),
		subs: make(map[string]paho.Token),
		send: make(chan []byte, 64),
	}
	go sess.writePump()
	go sess.readPump()
}

func (s *mqttDebugSession) readPump() {
	defer s.conn.Close()
	for {
		_, msg, err := s.conn.ReadMessage()
		if err != nil {
			break
		}
		var cmd struct {
			Action  string `json:"action"`
			Topic   string `json:"topic"`
			Payload string `json:"payload"`
			QoS     int    `json:"qos"`
		}
		if json.Unmarshal(msg, &cmd) != nil {
			continue
		}
		client := mqtt.Client()
		if client == nil {
			continue
		}
		// 主题白名单：拒绝 $ 系统主题；publish 拒绝通配符（EMQX 本身也拒绝通配 publish）
		if strings.HasPrefix(cmd.Topic, "$") || (cmd.Action == "publish" && strings.ContainsAny(cmd.Topic, "+#")) {
			s.pushMsg(map[string]interface{}{"direction": "err", "topic": cmd.Topic, "payload": "topic rejected"})
			continue
		}
		slog.Info("mqtt debug op", "username", s.user, "action", cmd.Action, "topic", cmd.Topic)
		switch cmd.Action {
		case "publish":
			client.Publish(cmd.Topic, byte(cmd.QoS), false, cmd.Payload)
			s.pushMsg(map[string]interface{}{"direction": "out", "topic": cmd.Topic, "payload": cmd.Payload})
		case "subscribe":
			token := client.Subscribe(cmd.Topic, byte(cmd.QoS), func(_ paho.Client, m paho.Message) {
				s.pushMsg(map[string]interface{}{"direction": "in", "topic": m.Topic(), "payload": string(m.Payload())})
			})
			s.mu.Lock()
			s.subs[cmd.Topic] = token
			s.mu.Unlock()
		case "unsubscribe":
			client.Unsubscribe(cmd.Topic)
			s.mu.Lock()
			delete(s.subs, cmd.Topic)
			s.mu.Unlock()
		}
	}
}

func (s *mqttDebugSession) writePump() {
	defer s.conn.Close()
	for msg := range s.send {
		if err := s.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			break
		}
	}
}

func (s *mqttDebugSession) pushMsg(v interface{}) {
	data, _ := json.Marshal(v)
	select {
	case s.send <- data:
	default:
	}
}
