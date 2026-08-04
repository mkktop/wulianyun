package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// echoPubSub 模拟 Redis pub/sub 回环：Published 的消息会回送到本实例自己的订阅通道
// （真实 Redis PUB/SUB 即此行为——订阅者包含发布者自身所在进程）。
type echoPubSub struct {
	ch chan []byte
}

func (e *echoPubSub) Publish(ctx context.Context, channel string, message []byte) error {
	e.ch <- message
	return nil
}

func (e *echoPubSub) Subscribe(ctx context.Context, channel string) <-chan []byte {
	return e.ch
}

// TestPushNoDuplicateWithEcho 单实例场景：本地直推一次 + Redis 回环再送达一次，
// 修复后客户端应只收到 1 份（instanceID 跳过自身回声）。
func TestPushNoDuplicateWithEcho(t *testing.T) {
	h := &Hub{clients: map[uint]map[*Client]bool{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.Serve(w, r, 1)
	}))
	defer srv.Close()

	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// 订阅设备 5
	if err := conn.WriteJSON(inMsg{Type: "subscribe", DeviceID: 5}); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	time.Sleep(200 * time.Millisecond) // 等 readLoop 处理订阅

	ps := &echoPubSub{ch: make(chan []byte, 16)}
	go func() {
		for data := range ps.ch {
			h.localDispatch(data)
		}
	}()

	// 推送一条遥测：本地直推 1 次 + publish 到"Redis"回环再送回 1 次
	h.PushTelemetry(1, 5, map[string]interface{}{"x": 1})

	count := 0
	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
		count++
	}
	if count != 1 {
		t.Fatalf("期望收到 1 份，实际 %d 份（回环去重失效）", count)
	}
}

// TestDispatchFromOtherInstance 另一实例发来的广播（不同 instanceID）应正常投递。
func TestDispatchFromOtherInstance(t *testing.T) {
	h := &Hub{clients: map[uint]map[*Client]bool{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.Serve(w, r, 1)
	}))
	defer srv.Close()

	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(inMsg{Type: "subscribe", DeviceID: 7}); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	time.Sleep(200 * time.Millisecond)

	// 伪造一条来自"其他实例"的广播（遥测带 onlyDevice=7）
	other := wsBroadcastMsg{
		InstanceID: "other-instance",
		UserID:     1,
		OnlyDevice: uintPtrTest(7),
		Msg:        OutMsg{Type: "telemetry", DeviceID: 7, Payload: map[string]interface{}{"y": 2}},
	}
	bmData, _ := json.Marshal(other)
	h.localDispatch(bmData)

	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	if _, _, err := conn.ReadMessage(); err != nil {
		t.Fatalf("其他实例的广播应被投递: %v", err)
	}
}

func uintPtrTest(v uint) *uint { return &v }
