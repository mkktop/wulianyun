package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"iot-platform/internal/config"
	"iot-platform/internal/model"
	"iot-platform/internal/repository"
	"iot-platform/internal/rule"
	"iot-platform/internal/ws"
)

// mergeLatestScript Redis Lua 原子合并最新值（避免 GET→合并→SET 竞态）
var mergeLatestScript = redis.NewScript(`
local key = KEYS[1]
local newData = ARGV[1]
local newTs = ARGV[2]
local old = redis.call('GET', key)
if old then
    local oldObj = cjson.decode(old)
    local newObj = cjson.decode(newData)
    local oldData = oldObj['data'] or {}
    for k, v in pairs(newObj) do
        oldData[k] = v
    end
    oldObj['data'] = oldData
    oldObj['ts'] = tonumber(newTs)
    redis.call('SET', key, cjson.encode(oldObj))
else
    local result = cjson.encode({ts = tonumber(newTs), data = cjson.decode(newData)})
    redis.call('SET', key, result)
end
return 1
`)

// FindDevice 按三元组标识查设备（带缓存）
func FindDevice(productKey, deviceName string) (*model.Device, error) {
	if d := getCachedDevice(productKey, deviceName); d != nil {
		return d, nil
	}
	var d model.Device
	err := repository.DB.Where("product_key = ? AND name = ?", productKey, deviceName).First(&d).Error
	if err != nil {
		return nil, err
	}
	cacheDevice(&d)
	return &d, nil
}

// FindDeviceForAuth 接入鉴权：兼容一机一密与一型一密
//   - 一机一密：校验设备独立 Secret
//   - 一型一密：secret == 产品 ProductSecret 时，设备不存在则动态注册（自动建设备并生成独立 Secret）
func FindDeviceForAuth(productKey, deviceName, secret string) (*model.Device, error) {
	var p model.Product
	if err := repository.DB.Where("product_key = ?", productKey).First(&p).Error; err != nil {
		return nil, errors.New("产品不存在")
	}

	d, err := FindDevice(productKey, deviceName)
	if err == nil {
		// 设备已存在：一型一密下允许用设备密钥或产品密钥，一机一密仅设备密钥
		if d.Secret == secret {
			return d, nil
		}
		if p.SecretMode == model.SecretModeProduct && p.ProductSecret != "" && secret == p.ProductSecret {
			return d, nil
		}
		return nil, errors.New("密钥错误")
	}

	// 设备不存在：仅一型一密且密钥匹配产品密钥时动态注册
	if p.SecretMode == model.SecretModeProduct && p.ProductSecret != "" && secret == p.ProductSecret {
		nd := model.Device{
			UserID: p.UserID, ProductID: p.ID, ProductKey: productKey,
			Name: deviceName, Secret: randSecret(),
			Status: model.DeviceStatusInactive,
		}
		if err := repository.DB.Create(&nd).Error; err != nil {
			return nil, errors.New("动态注册失败")
		}
		slog.Info("device auto-registered", "productId", productKey, "device", deviceName)
		return &nd, nil
	}
	return nil, errors.New("设备不存在或密钥错误")
}

func randSecret() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// FindDeviceByRegCode 按自定义注册码（IMEI/ICCID 等）查设备，用于 TCP 免三元组接入
func FindDeviceByRegCode(regCode string) (*model.Device, error) {
	if regCode == "" {
		return nil, errors.New("注册码为空")
	}
	var d model.Device
	if err := repository.DB.Where("reg_code = ?", regCode).First(&d).Error; err != nil {
		return nil, err
	}
	return &d, nil
}

// ParseClientID clientid 约定为 {productId}.{deviceName}
// 产品ID 固定 12 字符（2 位字母 + 10 位数字），按字符数解析（设备名可含点号）
func ParseClientID(clientID string) (productKey, deviceName string, ok bool) {
	// 前 12 字符为产品ID，第 13 字符为分隔点号
	if len(clientID) > 13 && isNewProductID(clientID[:12]) && clientID[12] == '.' {
		dn := clientID[13:]
		if dn != "" {
			return clientID[:12], dn, true
		}
	}
	return "", "", false
}

// isNewProductID 判断是否为固定 12 字符产品ID：2 位大写字母 + 10 位数字
func isNewProductID(s string) bool {
	if len(s) != 12 {
		return false
	}
	for i := 0; i < 2; i++ {
		c := s[i]
		if c < 'A' || c > 'Z' {
			return false
		}
	}
	for i := 2; i < 12; i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// HandleTelemetry 处理设备上行数据（JSON 对象）；含 method=event.post 时分流到事件上报
func HandleTelemetry(productKey, deviceName string, payload []byte) {
	traceID := generateTraceID()
	t0 := time.Now()

	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err != nil || len(data) == 0 {
		slog.Warn("invalid telemetry payload", "productId", productKey, "device", deviceName)
		return
	}
	d, err := FindDevice(productKey, deviceName)
	if err != nil {
		slog.Warn("telemetry from unknown device", "productId", productKey, "device", deviceName)
		return
	}

	// 系统方法分流：事件上报 / NTP 对时 / 配置拉取
	if m, ok := data["method"].(string); ok {
		switch m {
		case "event.post":
			handleEventReport(d, data)
			return
		case "ntp.request":
			handleNTPRequest(d, data)
			return
		case "config.get":
			handleConfigGet(d)
			return
		}
	}

	now := time.Now()

	// TSL 物模型校验
	t1 := time.Now()
	valid, validationErrors := ValidateTelemetry(d.ProductID, data)

	// 快慢路径分离：EMQX 规则引擎开启时跳过 DB 写入（快路径），仅做业务逻辑
	if !config.C.EMQXRule.Enabled {
		t := model.Telemetry{
			Ts: now, DeviceID: d.ID, ProductKey: productKey, DeviceName: deviceName,
			Data: payload, Valid: valid,
		}
		if !valid && len(validationErrors) > 0 {
			errJSON, _ := json.Marshal(validationErrors)
			t.ValidationErrors = errJSON
		}
		AppendTelemetry(t)
	}
	t2 := time.Now()

	// 最新值白名单：有物模型时只合并已定义属性，未定义字段（如指令应答 messageId/code）
	// 不进入实时数据/影子/规则，避免污染设备最新值（轨迹与设备日志仍保留原始 payload）
	mergeData := data
	if props, err := LoadThingModelProps(d.ProductID); err == nil && len(props) > 0 {
		allowed := make(map[string]bool, len(props))
		for _, p := range props {
			if id, ok := p["identifier"].(string); ok {
				allowed[id] = true
			}
		}
		filtered := make(map[string]interface{}, len(data))
		for k, v := range data {
			if allowed[k] {
				filtered[k] = v
			}
		}
		mergeData = filtered
	}

	// 最新值缓存：按属性合并（Redis Lua 原子操作，避免竞态）
	mergePayload, _ := json.Marshal(mergeData)
	mergeLatestScript.Run(context.Background(), repository.RDB,
		[]string{latestKey(d.ID)},
		string(mergePayload), fmt.Sprintf("%d", now.UnixMilli()),
	)

	// 实时推送：设备归属者 + 其一级父账号（一级实时看二级设备曲线）
	telemetryMsg := map[string]interface{}{"ts": now.UnixMilli(), "data": mergeData}
	for _, uid := range PushRecipients(d.UserID) {
		ws.H.PushTelemetry(uid, d.ID, telemetryMsg)
	}

	// 同步设备影子 reported + 规则引擎评估
	mergeShadowReported(d, mergeData)
	rule.EvalTelemetry(d, mergeData)
	t3 := time.Now()

	// 异步写入消息轨迹
	traceStatus := "ok"
	if !valid {
		traceStatus = "validation_failed"
	}
	go writeTrace(&model.MessageTrace{
		TraceID: traceID, UserID: d.UserID, ProductKey: productKey, DeviceName: deviceName,
		DeviceID: d.ID, Direction: "up", Stage: "completed", Status: traceStatus,
		Payload: string(payload),
		IngestMs: int(t1.Sub(t0).Milliseconds()),
		DecodeMs: 0,
		StoreMs:  int(t2.Sub(t1).Milliseconds()),
		RuleMs:   int(t3.Sub(t2).Milliseconds()),
	})

	// 异步写入设备日志
	go writeDeviceLog(d.UserID, d.ID, d.Name, "data_up", fmt.Sprintf("上报 %d 个属性", len(data)), string(payload), traceID)
}

// HandleDeviceStatus 处理上下线事件
// evtTs 为事件时间戳（毫秒），0 表示不校验；校验避免 $SYS connected/disconnected
// 事件经并发 goroutine 乱序处理时把新状态覆盖成旧状态。
// 注意：须经 QueueStatus 入队串行处理以保持 EMQX 发布顺序，勿直接并发调用。
func HandleDeviceStatus(clientID string, online bool, evtTs int64) {
	productKey, deviceName, ok := ParseClientID(clientID)
	if !ok {
		return
	}
	d, err := FindDevice(productKey, deviceName)
	if err != nil {
		return
	}

	// 陈旧事件保护：事件时间不晚于最近一次状态变更则忽略（FindDevice 有缓存，时间戳直查 DB）。
	// 平局（evtTs == lastChange）偏好在线：同刻 disconnect 多来自被新会话接管的旧会话
	if evtTs > 0 {
		var lastChange int64
		var fresh model.Device
		if err := repository.DB.Select("last_online_at", "last_offline_at").
			Where("id = ?", d.ID).First(&fresh).Error; err == nil {
			if fresh.LastOnlineAt != nil && fresh.LastOnlineAt.UnixMilli() > lastChange {
				lastChange = fresh.LastOnlineAt.UnixMilli()
			}
			if fresh.LastOfflineAt != nil && fresh.LastOfflineAt.UnixMilli() > lastChange {
				lastChange = fresh.LastOfflineAt.UnixMilli()
			}
		}
		if lastChange > 0 && (evtTs < lastChange || (!online && evtTs == lastChange)) {
			slog.Debug("skip stale status event", "clientID", clientID, "online", online, "evtTs", evtTs, "lastChange", lastChange)
			return
		}
	}

	now := time.Now()
	// 状态时间戳优先用事件时间（更准确；ts=0 的事件用处理时间）
	ts := now
	if evtTs > 0 {
		ts = time.UnixMilli(evtTs)
	}
	status := model.DeviceStatusOffline
	eventType := "offline"
	updates := map[string]interface{}{"last_offline_at": ts}
	if online {
		status = model.DeviceStatusOnline
		eventType = "online"
		updates = map[string]interface{}{"last_online_at": ts}
	}
	// 禁用设备不改状态（EMQX 鉴权阶段已拒绝，这里兜底）
	if d.Status == model.DeviceStatusDisabled {
		return
	}
	updates["status"] = status
	repository.DB.Model(&model.Device{}).Where("id = ?", d.ID).Updates(updates)
	repository.DB.Create(&model.DeviceEvent{DeviceID: d.ID, Type: eventType, Detail: "clientid: " + clientID})

	// 上线时补发影子期望值
	if online {
		syncShadowOnConnect(d)
	}

	// 异步写入设备日志
	logCategory := "connection"
	logSummary := "设备下线"
	if online {
		logSummary = "设备上线"
	}
	go writeDeviceLog(d.UserID, d.ID, d.Name, logCategory, logSummary, "clientid: "+clientID, "")

	statusMsg := map[string]interface{}{
		"deviceId": d.ID, "name": d.Name, "status": status, "ts": now.UnixMilli(),
	}
	for _, uid := range PushRecipients(d.UserID) {
		ws.H.PushDeviceStatus(uid, d.ID, statusMsg)
	}
}

// statusEvent 上下线事件
type statusEvent struct {
	clientID string
	online   bool
	evtTs    int64
}

var statusQueue = make(chan statusEvent, 512)

func init() {
	// 单 worker 按到达顺序串行处理状态事件：
	// $SYS connected/disconnected 与 LWT 遗嘱事件并发 goroutine 处理会乱序，
	// 导致新状态被旧事件覆盖（如 online 被随后处理的 offline 覆盖）
	go func() {
		for e := range statusQueue {
			HandleDeviceStatus(e.clientID, e.online, e.evtTs)
		}
	}()
}

// QueueStatus 提交上下线事件（非阻塞入队，FIFO 串行处理）
// 队列满时丢弃并告警：状态事件可降级（后续心跳/连接事件会再触发），
// 绝不能阻塞调用方——$SYS 回调在 paho 单分发协程上，阻塞会卡死全平台 MQTT 上行
func QueueStatus(clientID string, online bool, evtTs int64) {
	select {
	case statusQueue <- statusEvent{clientID: clientID, online: online, evtTs: evtTs}:
	default:
		slog.Warn("status queue full, drop event", "clientID", clientID, "online", online)
	}
}

// GetLatest 读取设备最新遥测（Redis 缓存优先，回退数据库）
func GetLatest(deviceID uint) map[string]interface{} {
	if b, err := repository.RDB.Get(context.Background(), latestKey(deviceID)).Bytes(); err == nil {
		var m map[string]interface{}
		if json.Unmarshal(b, &m) == nil {
			return m
		}
	}
	var t model.Telemetry
	if err := repository.DB.Where("device_id = ?", deviceID).Order("ts desc").First(&t).Error; err != nil {
		return nil
	}
	var data map[string]interface{}
	json.Unmarshal(t.Data, &data)
	return map[string]interface{}{"ts": t.Ts.UnixMilli(), "data": data}
}

func latestKey(deviceID uint) string {
	return "device:latest:" + strconv.FormatUint(uint64(deviceID), 10)
}

// HandleDeviceReply 处理设备指令应答，更新 CommandRequest 状态
func HandleDeviceReply(topic string, payload []byte) {
	var reply struct {
		MessageID string      `json:"messageId"`
		Code      int         `json:"code"`
		Data      interface{} `json:"data"`
	}
	if err := json.Unmarshal(payload, &reply); err != nil {
		slog.Warn("parse device reply failed", "topic", topic, "error", err)
		return
	}
	if reply.MessageID == "" {
		return
	}

	now := time.Now()
	result := repository.DB.Model(&model.CommandRequest{}).
		Where("message_id = ? AND status = ?", reply.MessageID, "pending").
		Updates(map[string]interface{}{
			"status":   "acked",
			"response": string(payload),
			"acked_at": now,
		})
	if result.RowsAffected == 0 {
		slog.Warn("reply matched no pending command", "messageId", reply.MessageID)
	}
}

// generateTraceID 生成简易唯一 traceId
func generateTraceID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%x%d", b, time.Now().UnixNano())
}

// writeTrace 异步写入消息轨迹
func writeTrace(t *model.MessageTrace) {
	if err := repository.DB.Create(t).Error; err != nil {
		slog.Warn("write trace failed", "traceId", t.TraceID, "err", err)
	}
}

// writeDeviceLog 异步写入设备运行日志
func writeDeviceLog(userID, deviceID uint, deviceName, category, summary, payload, traceID string) {
	log := model.DeviceLog{
		UserID: userID, DeviceID: deviceID, DeviceName: deviceName,
		Category: category, Summary: summary, Payload: payload, TraceID: traceID,
	}
	if err := repository.DB.Create(&log).Error; err != nil {
		slog.Warn("write device log failed", "err", err)
	}
}
