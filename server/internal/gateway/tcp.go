// Package gateway TCP 透传接入网关（对标有人云 DTU 场景）
//
// 接入流程：
//  1. 设备连接后 10 秒内发送注册包：{productKey},{deviceName},{secret}\n
//  2. 鉴权通过回复 OK\n，失败回复 ERR\n 并断开
//  3. 之后透传数据：产品配置了解析脚本则按脚本 decode，否则按 JSON 解析
//  4. "PING" 心跳回复 "PONG"；空闲超时断开并置离线
package gateway

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"iot-platform/internal/codec"
	"iot-platform/internal/config"
	"iot-platform/internal/model"
	"iot-platform/internal/repository"
	"iot-platform/internal/service"
)

type session struct {
	conn       net.Conn
	productKey string
	deviceName string
	productID  uint

	// Modbus 请求-响应：有待响应请求时，收到的帧路由到 waitCh
	mu     sync.Mutex
	waitCh chan []byte
	// reqMu 串行化同一连接上的请求-响应（Modbus 半双工，多采集组/写操作并发时必须排队）
	reqMu  sync.Mutex
}

// setWait 开启一次等待响应；返回接收通道与清理函数
func (s *session) setWait() (chan []byte, func()) {
	ch := make(chan []byte, 1)
	s.mu.Lock()
	s.waitCh = ch
	s.mu.Unlock()
	return ch, func() {
		s.mu.Lock()
		if s.waitCh == ch {
			s.waitCh = nil
		}
		s.mu.Unlock()
	}
}

// deliver 尝试把收到的帧投递给等待中的请求；返回 true 表示已消费
func (s *session) deliver(data []byte) bool {
	s.mu.Lock()
	ch := s.waitCh
	s.mu.Unlock()
	if ch == nil {
		return false
	}
	buf := make([]byte, len(data))
	copy(buf, data)
	select {
	case ch <- buf:
	default:
	}
	return true
}

var (
	mu       sync.RWMutex
	sessions = map[string]*session{} // key: productKey.deviceName

	// 上下线钩子（由 poller 注入，用于 Modbus 设备启停轮询；避免循环依赖）
	OnDeviceConnect    func(productKey, deviceName string, productID uint)
	OnDeviceDisconnect func(productKey, deviceName string)
)

func idleTimeout() time.Duration {
	if config.C.Gateway.IdleTimeout <= 0 {
		return 5 * time.Minute
	}
	return time.Duration(config.C.Gateway.IdleTimeout) * time.Second
}

// Start 启动 TCP 网关（异步）
func Start() {
	addr := config.C.Gateway.Addr
	if addr == "" {
		return
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Error("tcp gateway listen failed", "addr", addr, "err", err)
		return
	}
	slog.Info("tcp gateway started", "addr", addr)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				slog.Warn("tcp accept failed", "err", err)
				continue
			}
			go handleConn(conn)
		}
	}()
}

func handleConn(conn net.Conn) {
	defer conn.Close()

	// 注册包鉴权
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	reader := bufio.NewReaderSize(conn, 1024)
	line, err := reader.ReadString('\n')
	if err != nil {
		return
	}
	parts := strings.Split(strings.TrimSpace(line), ",")
	if len(parts) != 3 {
		conn.Write([]byte("ERR\n"))
		return
	}
	productKey, deviceName, secret := parts[0], parts[1], parts[2]
	d, err := service.FindDeviceForAuth(productKey, deviceName, secret)
	if err != nil || d.Status == model.DeviceStatusDisabled {
		conn.Write([]byte("ERR\n"))
		return
	}
	conn.Write([]byte("OK\n"))

	key := productKey + "." + deviceName
	s := &session{conn: conn, productKey: productKey, deviceName: deviceName, productID: d.ProductID}

	// 同设备重复连接：踢掉旧连接
	mu.Lock()
	if old, ok := sessions[key]; ok {
		old.conn.Close()
	}
	sessions[key] = s
	mu.Unlock()

	service.HandleDeviceStatus(key, true)
	if OnDeviceConnect != nil {
		OnDeviceConnect(productKey, deviceName, d.ProductID)
	}
	slog.Info("dtu connected", "device", key, "remote", conn.RemoteAddr().String())

	defer func() {
		mu.Lock()
		// 仅当仍是当前会话时清理（避免被新连接顶替后误置离线）
		if cur, ok := sessions[key]; ok && cur == s {
			delete(sessions, key)
			mu.Unlock()
			service.HandleDeviceStatus(key, false)
			if OnDeviceDisconnect != nil {
				OnDeviceDisconnect(productKey, deviceName)
			}
		} else {
			mu.Unlock()
		}
		slog.Info("dtu disconnected", "device", key)
	}()

	// 数据循环
	buf := make([]byte, 4096)
	for {
		conn.SetReadDeadline(time.Now().Add(idleTimeout()))
		n, err := reader.Read(buf)
		if err != nil {
			return
		}
		// Modbus 请求-响应：有等待中的请求时，投递原始二进制帧（不裁剪）
		if s.deliver(buf[:n]) {
			continue
		}
		data := bytes.TrimRight(buf[:n], "\r\n")
		if len(data) == 0 {
			continue
		}
		// 心跳
		if bytes.Equal(data, []byte("PING")) {
			conn.Write([]byte("PONG\n"))
			continue
		}
		handleUplink(s, data)
	}
}

// handleUplink 上行数据：优先脚本解析，其次尝试 JSON
func handleUplink(s *session, data []byte) {
	var p model.Product
	if err := repository.DB.Select("id, codec_script").First(&p, s.productID).Error; err != nil {
		return
	}
	if p.CodecScript != "" {
		obj, err := codec.Decode(p.ID, p.CodecScript, data)
		if err != nil {
			slog.Warn("codec decode failed", "device", s.deviceName, "err", err)
			return
		}
		payload, _ := json.Marshal(obj)
		service.HandleTelemetry(s.productKey, s.deviceName, payload)
		return
	}
	if json.Valid(data) {
		service.HandleTelemetry(s.productKey, s.deviceName, data)
		return
	}
	slog.Warn("dtu raw data dropped (no codec script)", "device", s.deviceName, "len", len(data))
}

// Has 设备是否通过 TCP 网关在线
func Has(productKey, deviceName string) bool {
	mu.RLock()
	defer mu.RUnlock()
	_, ok := sessions[productKey+"."+deviceName]
	return ok
}

// Send 下行数据：脚本有 encode 则编码为二进制，否则透传原始 JSON
func Send(productKey, deviceName string, payload []byte) error {
	mu.RLock()
	s, ok := sessions[productKey+"."+deviceName]
	mu.RUnlock()
	if !ok {
		return net.ErrClosed
	}
	out := payload
	var p model.Product
	if err := repository.DB.Select("id, codec_script").First(&p, s.productID).Error; err == nil && p.CodecScript != "" {
		var params map[string]interface{}
		if json.Unmarshal(payload, &params) == nil {
			if encoded, ok, err := codec.Encode(p.ID, p.CodecScript, params); err == nil && ok {
				out = encoded
			}
		}
	}
	s.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, err := s.conn.Write(out)
	return err
}

// Request 发送请求帧并等待应答（Modbus 半双工请求-响应），带超时；同一连接上串行执行
func Request(productKey, deviceName string, reqFrame []byte, timeout time.Duration) ([]byte, error) {
	mu.RLock()
	s, ok := sessions[productKey+"."+deviceName]
	mu.RUnlock()
	if !ok {
		return nil, net.ErrClosed
	}
	// 串行化：多个采集组/写操作并发时排队，避免应答错配
	s.reqMu.Lock()
	defer s.reqMu.Unlock()

	ch, cleanup := s.setWait()
	defer cleanup()

	s.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err := s.conn.Write(reqFrame); err != nil {
		return nil, err
	}
	select {
	case resp := <-ch:
		return resp, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("请求超时")
	}
}
