package ws

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Client 一个前端 WebSocket 连接
type Client struct {
	conn      *websocket.Conn
	userID    uint
	send      chan []byte
	deviceIDs map[uint]bool // 订阅的设备，空表示只收全局事件
	mu        sync.RWMutex
}

// Hub 推送中心
type Hub struct {
	mu      sync.RWMutex
	clients map[uint]map[*Client]bool // userID -> set of clients
}

var H = &Hub{clients: map[uint]map[*Client]bool{}}

type inMsg struct {
	Type     string `json:"type"` // subscribe / unsubscribe
	DeviceID uint   `json:"deviceId"`
}

type OutMsg struct {
	Type     string      `json:"type"` // telemetry / device_status / alarm
	DeviceID uint        `json:"deviceId,omitempty"`
	Payload  interface{} `json:"payload"`
}

// Serve 升级 HTTP 连接并托管读写
func (h *Hub) Serve(w http.ResponseWriter, r *http.Request, userID uint) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Warn("ws upgrade failed", "err", err)
		return
	}
	c := &Client{conn: conn, userID: userID, send: make(chan []byte, 64), deviceIDs: map[uint]bool{}}
	h.mu.Lock()
	if h.clients[c.userID] == nil {
		h.clients[c.userID] = map[*Client]bool{}
	}
	h.clients[c.userID][c] = true
	h.mu.Unlock()

	go c.writeLoop()
	c.readLoop(h)
}

func (c *Client) readLoop(h *Hub) {
	defer func() {
		h.mu.Lock()
		if m, ok := h.clients[c.userID]; ok {
			delete(m, c)
			if len(m) == 0 {
				delete(h.clients, c.userID)
			}
		}
		h.mu.Unlock()
		close(c.send)
		c.conn.Close()
	}()
	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		var m inMsg
		if json.Unmarshal(data, &m) != nil {
			continue
		}
		c.mu.Lock()
		switch m.Type {
		case "subscribe":
			c.deviceIDs[m.DeviceID] = true
		case "unsubscribe":
			delete(c.deviceIDs, m.DeviceID)
		}
		c.mu.Unlock()
	}
}

func (c *Client) writeLoop() {
	for data := range c.send {
		if c.conn.WriteMessage(websocket.TextMessage, data) != nil {
			return
		}
	}
}

// PushTelemetry 推送遥测给订阅了该设备的连接
func (h *Hub) PushTelemetry(userID, deviceID uint, payload interface{}) {
	h.push(userID, &deviceID, OutMsg{Type: "telemetry", DeviceID: deviceID, Payload: payload})
}

// PushDeviceStatus 推送设备状态变化给该用户所有连接
func (h *Hub) PushDeviceStatus(userID, deviceID uint, payload interface{}) {
	h.push(userID, nil, OutMsg{Type: "device_status", DeviceID: deviceID, Payload: payload})
}

// PushAlarm 推送告警给该用户所有连接
func (h *Hub) PushAlarm(userID uint, payload interface{}) {
	h.push(userID, nil, OutMsg{Type: "alarm", Payload: payload})
}

// PushEvent 推送设备事件上报给该用户所有连接
func (h *Hub) PushEvent(userID uint, payload interface{}) {
	h.push(userID, nil, OutMsg{Type: "event", Payload: payload})
}

func (h *Hub) push(userID uint, onlyDevice *uint, msg OutMsg) {
	data, _ := json.Marshal(msg)
	h.mu.RLock()
	userClients := h.clients[userID]
	h.mu.RUnlock()
	for c := range userClients {
		if onlyDevice != nil {
			c.mu.RLock()
			sub := c.deviceIDs[*onlyDevice]
			c.mu.RUnlock()
			if !sub {
				continue
			}
		}
		select {
		case c.send <- data:
		default: // 队列满丢弃，避免阻塞
		}
	}
}
