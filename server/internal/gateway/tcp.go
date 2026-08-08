// Package gateway TCP 透传接入网关
//
// 接入流程：
//  1. 设备连接后 10 秒内发送注册包：三元组 {productKey},{deviceName},{secret}\n
//     或自定义注册码（单行，匹配设备 RegCode，如 IMEI/ICCID）
//  2. 鉴权通过回复 OK\n，失败回复 ERR\n 并断开
//  3. 之后按产品组帧配置切分数据帧：脚本 decode 或 JSON 解析
//  4. 心跳（默认 "PING"→"PONG"，或产品自定义）；空闲超时断开并置离线
package gateway

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
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

	fr       *framer // 组帧器（鉴权后按产品配置构造）
	hbPacket []byte  // 自定义心跳请求（空=默认 PING）
	hbReply  []byte  // 自定义心跳应答（空=默认 PONG）

	// Modbus 请求-响应：有待响应请求时，匹配的帧路由到 waitCh
	mu       sync.Mutex
	waitCh   chan []byte
	expSlave byte // 期望应答的从机地址
	expFunc  byte // 期望应答的功能码
	// reqMu 串行化同一连接上的请求-响应（Modbus 半双工，多采集组/写操作并发时必须排队）
	reqMu  sync.Mutex
}

// setWait 开启一次等待响应；记录期望的从机地址/功能码用于应答匹配
func (s *session) setWait(slave, fn byte) (chan []byte, func()) {
	ch := make(chan []byte, 1)
	s.mu.Lock()
	s.waitCh = ch
	s.expSlave = slave
	s.expFunc = fn
	s.mu.Unlock()
	return ch, func() {
		s.mu.Lock()
		if s.waitCh == ch {
			s.waitCh = nil
		}
		s.mu.Unlock()
	}
}

// deliver 尝试把收到的帧投递给等待中的请求；仅当从机地址与功能码匹配时消费
// 返回 true 表示已消费（不再作为普通上行处理）
func (s *session) deliver(data []byte) bool {
	s.mu.Lock()
	ch := s.waitCh
	slave, fn := s.expSlave, s.expFunc
	s.mu.Unlock()
	if ch == nil {
		return false
	}
	// 校验应答与请求匹配：从机地址一致，功能码一致或对应异常码(fn|0x80)
	// 不匹配的帧（迟到应答/主动帧）不消费，交由普通上行处理
	if len(data) >= 2 && (data[0] != slave || (data[1] != fn && data[1] != fn|0x80)) {
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

var ipConnCount sync.Map // ip string -> *int64 (atomic counter)

func getIPCounter(ip string) *int64 {
	if v, ok := ipConnCount.Load(ip); ok {
		return v.(*int64)
	}
	var counter int64
	actual, _ := ipConnCount.LoadOrStore(ip, &counter)
	return actual.(*int64)
}

type tokenBucket struct {
	tokens   float64
	capacity float64
	rate     float64 // tokens per second
	lastFill time.Time
	mu       sync.Mutex
}

func (b *tokenBucket) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	elapsed := now.Sub(b.lastFill).Seconds()
	b.tokens += elapsed * b.rate
	if b.tokens > b.capacity {
		b.tokens = b.capacity
	}
	b.lastFill = now
	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

var (
	rateLimiterMu sync.Mutex
	rateLimiters  = make(map[string]*tokenBucket)
)

func getRateLimiter(ip string) *tokenBucket {
	rateLimiterMu.Lock()
	defer rateLimiterMu.Unlock()
	if lb, ok := rateLimiters[ip]; ok {
		return lb
	}
	rate := float64(config.C.Gateway.ConnRateLimit)
	if rate <= 0 {
		rate = 5
	}
	capacity := float64(config.C.Gateway.ConnRateBurst)
	if capacity <= 0 {
		capacity = 10
	}
	lb := &tokenBucket{
		tokens:   capacity,
		capacity: capacity,
		rate:     rate,
		lastFill: time.Now(),
	}
	rateLimiters[ip] = lb
	return lb
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
	var ln net.Listener
	if config.C.Gateway.TLS.Enabled {
		cert, err := tls.LoadX509KeyPair(config.C.Gateway.TLS.CertFile, config.C.Gateway.TLS.KeyFile)
		if err != nil {
			slog.Error("load gateway tls cert failed", "error", err)
			return
		}
		tlsConfig := &tls.Config{Certificates: []tls.Certificate{cert}}
		ln, err = tls.Listen("tcp", addr, tlsConfig)
		if err != nil {
			slog.Error("tls listen failed", "error", err)
			return
		}
		slog.Info("tls tcp gateway started", "addr", addr)
	} else {
		var err error
		ln, err = net.Listen("tcp", addr)
		if err != nil {
			slog.Error("tcp gateway listen failed", "addr", addr, "err", err)
			return
		}
		slog.Info("tcp gateway started", "addr", addr)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				slog.Warn("tcp accept failed", "err", err)
				continue
			}
			remoteIP := ""
			if addr, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
				remoteIP = addr.IP.String()
			}
			if remoteIP != "" {
				lb := getRateLimiter(remoteIP)
				if !lb.allow() {
					conn.Close()
					slog.Warn("conn rate limited", "ip", remoteIP)
					continue
				}
			}
			go handleConn(conn)
		}
	}()
}

func handleConn(conn net.Conn) {
	defer conn.Close()

	remoteIP := ""
	if addr, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
		remoteIP = addr.IP.String()
	}
	if remoteIP == "" {
		return
	}

	counter := getIPCounter(remoteIP)
	maxConns := int64(config.C.Gateway.MaxConnsPerIP)
	if maxConns <= 0 {
		maxConns = 10
	}
	if atomic.AddInt64(counter, 1) > maxConns {
		atomic.AddInt64(counter, -1)
		slog.Warn("ip connection limit exceeded", "ip", remoteIP, "max", maxConns)
		return
	}
	defer atomic.AddInt64(counter, -1)

	// 注册包鉴权
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	reader := bufio.NewReaderSize(conn, 1024)
	line, err := reader.ReadString('\n')
	if err != nil {
		return
	}
	parts := strings.Split(strings.TrimSpace(line), ",")
	raw := strings.TrimSpace(line)
	var d *model.Device
	if len(parts) == 3 {
		d, err = service.FindDeviceForAuth(parts[0], parts[1], parts[2])
	} else if raw != "" {
		d, err = service.FindDeviceByRegCode(raw) // 自定义注册码（IMEI/ICCID 等）
	} else {
		conn.Write([]byte("ERR\n"))
		return
	}
	if err != nil || d == nil || d.Status == model.DeviceStatusDisabled {
		conn.Write([]byte("ERR\n"))
		return
	}
	conn.Write([]byte("OK\n"))

	productKey, deviceName := d.ProductKey, d.Name
	key := productKey + "." + deviceName

	// 加载产品组帧/心跳配置
	var p model.Product
	if err := repository.DB.First(&p, d.ProductID).Error; err != nil {
		return
	}
	s := &session{
		conn: conn, productKey: productKey, deviceName: deviceName, productID: d.ProductID,
		fr:       newFramer(&p),
		hbPacket: parseBytes(p.HeartbeatPacket),
		hbReply:  parseBytes(p.HeartbeatReply),
	}

	// 同设备重复连接：踢掉旧连接
	mu.Lock()
	if old, ok := sessions[key]; ok {
		old.conn.Close()
	}
	sessions[key] = s
	mu.Unlock()

	service.QueueStatus(key, true, 0)
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
			service.QueueStatus(key, false, 0)
			if OnDeviceDisconnect != nil {
				OnDeviceDisconnect(productKey, deviceName)
			}
			// TCP 断线重连引导：记录断开原因，设备端固件可据此执行指数退避重连
			writeReconnectGuidance(productKey, deviceName, conn)
		} else {
			mu.Unlock()
		}
		slog.Info("dtu disconnected", "device", key)
	}()

	// 数据循环：按产品组帧配置切分完整帧
	buf := make([]byte, 4096)
	for {
		conn.SetReadDeadline(time.Now().Add(idleTimeout()))
		n, err := reader.Read(buf)
		if err != nil {
			return
		}
		s.fr.append(buf[:n])
		for {
			frame, ok := s.fr.next()
			if !ok {
				break
			}
			s.handleFrame(frame)
		}
	}
}

// handleFrame 处理一个完整帧：Modbus 应答投递 / 心跳 / 普通上行
func (s *session) handleFrame(frame []byte) {
	// Modbus 请求-响应：投递给等待者（framer 已保证完整且 CRC 通过）
	if s.deliver(frame) {
		return
	}
	data := bytes.TrimRight(frame, "\r\n")
	if len(data) == 0 {
		return
	}
	// 心跳
	if s.isHeartbeat(data) {
		s.replyHeartbeat()
		return
	}
	handleUplink(s, data)
}

// isHeartbeat 判断是否心跳帧（产品自定义优先，否则默认 PING）
func (s *session) isHeartbeat(data []byte) bool {
	if len(s.hbPacket) > 0 {
		return bytes.Equal(data, s.hbPacket)
	}
	return bytes.Equal(data, []byte("PING"))
}

// replyHeartbeat 回复心跳（自定义心跳未配回复则不回）
func (s *session) replyHeartbeat() {
	s.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if len(s.hbPacket) > 0 {
		if len(s.hbReply) > 0 {
			s.conn.Write(s.hbReply)
		}
		return
	}
	s.conn.Write([]byte("PONG\n"))
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

// Broadcast 向某产品下所有在线 TCP 设备下发（复用 Send 的 codec 编码逻辑，收集写错误）
func Broadcast(productKey string, payload []byte) error {
	mu.RLock()
	var targets []string
	prefix := productKey + "."
	for key := range sessions {
		if strings.HasPrefix(key, prefix) {
			targets = append(targets, key)
		}
	}
	mu.RUnlock()

	var errs []string
	for _, key := range targets {
		dn := strings.TrimPrefix(key, prefix)
		if err := Send(productKey, dn, payload); err != nil {
			errs = append(errs, dn+": "+err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("broadcast partial failure: %s", strings.Join(errs, "; "))
	}
	return nil
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

	var slave, fn byte
	if len(reqFrame) >= 2 {
		slave, fn = reqFrame[0], reqFrame[1]
	}
	ch, cleanup := s.setWait(slave, fn)
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

// writeReconnectGuidance TCP 断线重连引导：记录断开原因到设备日志
// 设备端固件建议实现指数退避重连：1s → 2s → 4s → ... → 60s cap
func writeReconnectGuidance(productKey, deviceName string, conn net.Conn) {
	reason := "未知"
	if conn != nil {
		if conn.RemoteAddr() != nil {
			reason = "远程断开: " + conn.RemoteAddr().String()
		}
	}
	// 写入设备日志，前端在设备详情页可看到断连记录
	go service.LogTCPDisconnect(productKey, deviceName, reason)
}
