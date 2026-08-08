package ws

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// instanceID 本进程实例唯一标识：Redis pub/sub 会把广播回送给发送者自身，
// 接收端据此跳过自己发出的消息，避免单实例下本地直推+回环各推一次造成重复。
var instanceID = func() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}()

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Client 一个前端 WebSocket 连接
type Client struct {
	conn      *websocket.Conn
	userID    uint
	send      chan []byte
	done      chan struct{} // 连接关闭信号，广播发送 select 保护
	deviceIDs map[uint]bool // 订阅的设备，空表示只收全局事件
	mu        sync.RWMutex
}

// Hub 推送中心
type Hub struct {
	mu      sync.RWMutex
	clients map[uint]map[*Client]bool // userID -> set of clients
}

var H = &Hub{clients: map[uint]map[*Client]bool{}}

// RedisPubSub Redis 发布订阅接口（由 repository 注入，nil 时退化为单实例模式）
var RedisPubSub RedisPubSuber

// RedisPubSuber Redis Pub/Sub 抽象接口
type RedisPubSuber interface {
	Publish(ctx context.Context, channel string, message []byte) error
	Subscribe(ctx context.Context, channel string) <-chan []byte
}

const wsChannel = "ws:broadcast"

type inMsg struct {
	Type     string `json:"type"` // subscribe / unsubscribe
	DeviceID uint   `json:"deviceId"`
}

type OutMsg struct {
	Type     string      `json:"type"` // telemetry / device_status / alarm
	DeviceID uint        `json:"deviceId,omitempty"`
	Payload  interface{} `json:"payload"`
}

// StartPubSub 启动 Redis 订阅，收到消息后推给本实例的 WebSocket 客户端
func StartPubSub() {
	if RedisPubSub == nil {
		return
	}
	go func() {
		ch := RedisPubSub.Subscribe(context.Background(), wsChannel)
		for data := range ch {
			H.localDispatch(data)
		}
	}()
	slog.Info("ws pubsub started", "channel", wsChannel)
}

// Serve 升级 HTTP 连接并托管读写
func (h *Hub) Serve(w http.ResponseWriter, r *http.Request, userID uint) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Warn("ws upgrade failed", "err", err)
		return
	}
	c := &Client{conn: conn, userID: userID, send: make(chan []byte, 64), done: make(chan struct{}), deviceIDs: map[uint]bool{}}
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
		// 先关 done 再关 send：广播发送的 select 会优先命中 done，避免 send-on-closed panic
		close(c.done)
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

// wsBroadcastMsg 跨实例广播消息（包含目标用户和完整 OutMsg）
type wsBroadcastMsg struct {
	InstanceID string `json:"instanceId"` // 发送方实例；接收方跳过自身回声
	UserID     uint   `json:"userId"`
	OnlyDevice *uint  `json:"onlyDevice,omitempty"`
	Msg        OutMsg `json:"msg"`
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

// push 本地推送 + 跨实例广播
func (h *Hub) push(userID uint, onlyDevice *uint, msg OutMsg) {
	data, _ := json.Marshal(msg)
	// 1. 本实例直接推送
	h.localPush(userID, onlyDevice, data)
	// 2. 通过 Redis Pub/Sub 广播给其他实例（带实例 ID，接收端跳过自身回声）
	if RedisPubSub != nil {
		bm := wsBroadcastMsg{InstanceID: instanceID, UserID: userID, OnlyDevice: onlyDevice, Msg: msg}
		bmData, _ := json.Marshal(bm)
		_ = RedisPubSub.Publish(context.Background(), wsChannel, bmData)
	}
}

// localPush 仅推送给本实例的 WebSocket 客户端
func (h *Hub) localPush(userID uint, onlyDevice *uint, data []byte) {
	// 全程持 RLock 遍历：readLoop 在锁内 delete 客户端，锁外遍历会并发 map 读写崩溃
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients[userID] {
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
		case <-c.done: // 连接已关闭，跳过
		default: // 队列满丢弃，避免阻塞
		}
	}
}

// localDispatch 处理从 Redis 收到的广播消息
func (h *Hub) localDispatch(bmData []byte) {
	var bm wsBroadcastMsg
	if err := json.Unmarshal(bmData, &bm); err != nil {
		return
	}
	if bm.InstanceID == instanceID {
		return // 自己发出的广播回环：本地已直推过，跳过避免重复
	}
	data, _ := json.Marshal(bm.Msg)
	h.localPush(bm.UserID, bm.OnlyDevice, data)
}
