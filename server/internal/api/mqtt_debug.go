package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"

	"iot-platform/internal/mqtt"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	paho "github.com/eclipse/paho.mqtt.golang"
)

var wsUpgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

type mqttDebugSession struct {
	conn *websocket.Conn
	subs map[string]paho.Token
	send chan []byte
	mu   sync.Mutex
}

func MqttDebugWS(c *gin.Context) {
	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Error("mqtt debug ws upgrade failed", "error", err)
		return
	}
	sess := &mqttDebugSession{
		conn: conn,
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
