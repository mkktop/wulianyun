package rule

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"iot-platform/internal/model"
	"iot-platform/internal/repository"
	"iot-platform/internal/ws"
)

// silenceCache 静默期缓存 ruleID:deviceID -> 上次触发时间（内存级，单实例）
var silenceCache sync.Map

// redisSilenceRDB Redis 客户端（多实例模式下使用，nil 时退化为内存缓存）
var redisSilenceRDB *redis.Client

// UseRedisSilence 启用 Redis 静默缓存（多实例模式）
func UseRedisSilence(rdb *redis.Client) {
	redisSilenceRDB = rdb
}

// RecipientResolver 由 main 注入：把一个用户展开为"应接收其实时数据/告警"的用户集合
// （含其一级父账号，便于一级实时看二级设备的告警）。nil 时只推该用户本身。
var RecipientResolver func(uint) []uint

// pushAlarmTo 把告警推送给 userIDs 及其父账号（去重）。
func pushAlarmTo(userIDs []uint, payload map[string]interface{}) {
	if RecipientResolver == nil {
		for _, uid := range userIDs {
			ws.H.PushAlarm(uid, payload)
		}
		return
	}
	seen := make(map[uint]bool, len(userIDs)*2)
	for _, uid := range userIDs {
		for _, r := range RecipientResolver(uid) {
			if !seen[r] {
				seen[r] = true
				ws.H.PushAlarm(r, payload)
			}
		}
	}
}

// ruleCache 规则缓存：按 userID 分组，TTL 30s
var (
	ruleCacheMu   sync.RWMutex
	ruleCache     map[uint][]model.Rule // userID -> rules
	ruleCacheTime time.Time
	ruleCacheTTL  = 30 * time.Second
)

func loadRulesForUser(userID uint) []model.Rule {
	ruleCacheMu.RLock()
	if time.Since(ruleCacheTime) < ruleCacheTTL && ruleCache != nil {
		if rules, ok := ruleCache[userID]; ok {
			ruleCacheMu.RUnlock()
			return rules
		}
	}
	ruleCacheMu.RUnlock()

	ruleCacheMu.Lock()
	defer ruleCacheMu.Unlock()
	// double check
	if time.Since(ruleCacheTime) < ruleCacheTTL && ruleCache != nil {
		if rules, ok := ruleCache[userID]; ok {
			return rules
		}
	}
	// 重新加载所有用户的规则
	var allRules []model.Rule
	repository.DB.Where("enabled = ?", true).Find(&allRules)
	newCache := make(map[uint][]model.Rule)
	for _, r := range allRules {
		newCache[r.UserID] = append(newCache[r.UserID], r)
	}
	ruleCache = newCache
	ruleCacheTime = time.Now()
	return ruleCache[userID]
}

// InvalidateRuleCache 使规则缓存失效
func InvalidateRuleCache(userID uint) {
	ruleCacheMu.Lock()
	ruleCacheTime = time.Time{} // 强制过期
	ruleCacheMu.Unlock()
}

// EvalTelemetry 遥测数据进入规则引擎（由 ingest 调用，异步）
func EvalTelemetry(d *model.Device, data map[string]interface{}) {
	allRules := loadRulesForUser(d.UserID)
	// 内存过滤：product_id/device_id 匹配
	var rules []model.Rule
	for _, r := range allRules {
		if !r.Enabled {
			continue
		}
		if r.Type != model.RuleTypeAlarm && r.Type != model.RuleTypeForward {
			continue
		}
		if r.ProductID > 0 && r.ProductID != d.ProductID {
			continue
		}
		if r.DeviceID > 0 && r.DeviceID != d.ID {
			continue
		}
		rules = append(rules, r)
	}

	for i := range rules {
		r := &rules[i]
		switch r.Type {
		case model.RuleTypeAlarm:
			evalAlarmRule(r, d, data)
		case model.RuleTypeForward:
			doForward(r, d, data)
		}
	}
}

type alarmAction struct {
	Level      string   `json:"level"`
	Notify     []string `json:"notify"` // ws / webhook
	WebhookURL string   `json:"webhookUrl"`
}

// evalCondition 递归评估条件表达式
// 支持两种格式：
// 1. 旧格式（向后兼容）: {"field":"temperature","op":">","value":35}
// 2. 新格式（复合条件）: {"logic":"and","conditions":[...]}
func evalCondition(cond json.RawMessage, data map[string]interface{}) bool {
	// 尝试解析为复合条件
	var compound struct {
		Logic      string            `json:"logic"`
		Conditions []json.RawMessage `json:"conditions"`
	}
	if err := json.Unmarshal(cond, &compound); err == nil && compound.Logic != "" {
		switch strings.ToLower(compound.Logic) {
		case "and":
			for _, c := range compound.Conditions {
				if !evalCondition(c, data) {
					return false
				}
			}
			return len(compound.Conditions) > 0
		case "or":
			for _, c := range compound.Conditions {
				if evalCondition(c, data) {
					return true
				}
			}
			return false
		}
	}

	// 旧格式单条件
	var single struct {
		Field string      `json:"field"`
		Op    string      `json:"op"`
		Value interface{} `json:"value"`
	}
	if err := json.Unmarshal(cond, &single); err != nil || single.Field == "" {
		return false
	}
	v, ok := data[single.Field]
	if !ok {
		return false
	}
	return compare(v, single.Op, single.Value)
}

func evalAlarmRule(r *model.Rule, d *model.Device, data map[string]interface{}) {
	if !evalCondition(json.RawMessage(r.Condition), data) {
		// 条件不满足时，检查是否有 firing 状态的告警需要自动恢复
		autoResolveAlarm(r, d)
		return
	}
	if inSilence(r, d.ID) {
		return
	}

	var act alarmAction
	json.Unmarshal(r.Action, &act)
	if act.Level == "" {
		act.Level = "warning"
	}
	// 构建告警消息（提取条件摘要）
	var single struct {
		Field string      `json:"field"`
		Op    string      `json:"op"`
		Value interface{} `json:"value"`
	}
	msg := ""
	if json.Unmarshal(r.Condition, &single) == nil && single.Field != "" {
		v := data[single.Field]
		msg = fmt.Sprintf("设备[%s] %s=%v 触发条件 %s %v", d.Name, single.Field, v, single.Op, single.Value)
	} else {
		msg = fmt.Sprintf("设备[%s] 触发规则[%s]", d.Name, r.Name)
	}
	fireAlarm(r, d, act, msg, data)
}

// autoResolveAlarm 条件不满足时自动恢复 firing 状态的告警
func autoResolveAlarm(r *model.Rule, d *model.Device) {
	var alarm model.Alarm
	err := repository.DB.Where("rule_id = ? AND device_id = ? AND status = ?",
		r.ID, d.ID, model.AlarmStatusFiring).
		Order("id desc").First(&alarm).Error
	if err != nil {
		return // 没有 firing 告警
	}

	now := time.Now()
	repository.DB.Model(&alarm).Updates(map[string]interface{}{
		"status":      model.AlarmStatusResolved,
		"resolved_at": now,
	})

	// 发送恢复通知
	var act alarmAction
	json.Unmarshal(r.Action, &act)
	for _, n := range act.Notify {
		if n == "webhook" && act.WebhookURL != "" {
			go postJSONWithRetry(act.WebhookURL, map[string]interface{}{
				"type":    "alarm_resolve",
				"rule":    r.Name,
				"device":  d.Name,
				"message": "告警已自动恢复",
				"ts":      now.UnixMilli(),
			}, r.RetryCount)
		}
	}
	// WebSocket 推送（fan-out 给规则归属者及其一级父账号）
	pushAlarmTo([]uint{r.UserID}, map[string]interface{}{
		"type":     "alarm_resolve",
		"ruleName": r.Name,
		"device":   d.Name,
		"ts":       now.UnixMilli(),
	})
	slog.Info("alarm auto resolved", "rule", r.Name, "device", d.Name)
}

// EvalOffline 离线巡检（定时器调用）：设备离线超过 N 分钟触发
func EvalOffline() {
	var rules []model.Rule
	repository.DB.Where("enabled = true AND type = ?", model.RuleTypeOffline).Find(&rules)
	for i := range rules {
		r := &rules[i]
		var cond struct {
			Minutes int `json:"minutes"`
		}
		if json.Unmarshal(r.Condition, &cond) != nil || cond.Minutes <= 0 {
			continue
		}
		deadline := time.Now().Add(-time.Duration(cond.Minutes) * time.Minute)

		q := repository.DB.Model(&model.Device{}).
			Where("user_id = ? AND status = ? AND last_offline_at IS NOT NULL AND last_offline_at < ?",
				r.UserID, model.DeviceStatusOffline, deadline)
		if r.ProductID > 0 {
			q = q.Where("product_id = ?", r.ProductID)
		}
		if r.DeviceID > 0 {
			q = q.Where("id = ?", r.DeviceID)
		}
		var devices []model.Device
		q.Find(&devices)

		var act alarmAction
		json.Unmarshal(r.Action, &act)
		if act.Level == "" {
			act.Level = "warning"
		}
		for j := range devices {
			d := &devices[j]
			if inSilence(r, d.ID) {
				continue
			}
			msg := fmt.Sprintf("设备[%s] 已离线超过 %d 分钟", d.Name, cond.Minutes)
			fireAlarm(r, d, act, msg, nil)
		}
	}
}

// StartOfflineChecker 启动离线巡检定时器
func StartOfflineChecker() {
	go func() {
		ticker := time.NewTicker(time.Minute)
		for range ticker.C {
			EvalOffline()
		}
	}()
}

func fireAlarm(r *model.Rule, d *model.Device, act alarmAction, msg string, data map[string]interface{}) {
	alarm := model.Alarm{
		UserID: r.UserID, RuleID: r.ID, RuleName: r.Name,
		DeviceID: d.ID, DeviceName: d.Name,
		Level: act.Level, Message: msg, Status: model.AlarmStatusFiring,
	}
	if err := repository.DB.Create(&alarm).Error; err != nil {
		slog.Error("save alarm failed", "err", err)
		return
	}
	markFired(r, d.ID)

	// 站内实时通知（默认始终推送）—— fan-out 给规则归属者及其一级父账号
	pushAlarmTo([]uint{r.UserID}, map[string]interface{}{
		"id": alarm.ID, "ruleName": r.Name, "deviceId": d.ID, "deviceName": d.Name,
		"level": act.Level, "message": msg, "ts": alarm.CreatedAt.UnixMilli(),
	})

	// Webhook 通知
	for _, n := range act.Notify {
		if n == "webhook" && act.WebhookURL != "" {
			go postJSONWithRetry(act.WebhookURL, map[string]interface{}{
				"type": "alarm", "rule": r.Name, "device": d.Name,
				"level": act.Level, "message": msg, "data": data,
				"ts": time.Now().UnixMilli(),
			}, r.RetryCount)
		}
	}
	slog.Info("alarm fired", "rule", r.Name, "device", d.Name, "msg", msg)
}

type forwardAction struct {
	Type       string   `json:"type"` // webhook / kafka / mqtt_bridge
	WebhookURL string   `json:"webhookUrl"`
	Brokers    []string `json:"brokers"`
	Topic      string   `json:"topic"`
	Broker     string   `json:"broker"`
	Username   string   `json:"username"`
	Password   string   `json:"password"`
}

func doForward(r *model.Rule, d *model.Device, data map[string]interface{}) {
	var actCheck struct{ Type string `json:"type"` }
	json.Unmarshal(r.Action, &actCheck)
	if actCheck.Type == "kafka" || actCheck.Type == "mqtt_bridge" {
		ForwardData(r, d, data)
		return
	}

	var act forwardAction
	if err := json.Unmarshal(r.Action, &act); err != nil {
		slog.Error("parse forward action failed", "ruleID", r.ID, "error", err)
		return
	}

	// webhook（默认，向后兼容旧数据）
	if act.WebhookURL == "" {
		return
	}
	go postJSONWithRetry(act.WebhookURL, map[string]interface{}{
		"type": "telemetry", "productId": d.ProductKey, "device": d.Name,
		"data": data, "ts": time.Now().UnixMilli(),
	}, r.RetryCount)
}

func inSilence(r *model.Rule, deviceID uint) bool {
	if r.Silence <= 0 {
		return false
	}
	key := "silence:" + strconv.FormatUint(uint64(r.ID), 10) + ":" + strconv.FormatUint(uint64(deviceID), 10)
	ttl := time.Duration(r.Silence) * time.Minute

	// 多实例模式：Redis 检查
	if redisSilenceRDB != nil {
		ctx := context.Background()
		val, err := redisSilenceRDB.Get(ctx, key).Result()
		return err == nil && val != "" // key 存在说明在静默期内
	}

	// 单实例退化：内存检查
	keyMem := strconv.FormatUint(uint64(r.ID), 10) + ":" + strconv.FormatUint(uint64(deviceID), 10)
	if v, ok := silenceCache.Load(keyMem); ok {
		if time.Since(v.(time.Time)) < ttl {
			return true
		}
	}
	return false
}

func markFired(r *model.Rule, deviceID uint) {
	key := "silence:" + strconv.FormatUint(uint64(r.ID), 10) + ":" + strconv.FormatUint(uint64(deviceID), 10)
	ttl := time.Duration(r.Silence) * time.Minute

	// 多实例模式：Redis 设置带 TTL
	if redisSilenceRDB != nil {
		ctx := context.Background()
		redisSilenceRDB.Set(ctx, key, "1", ttl)
		return
	}

	// 单实例退化：内存存储
	keyMem := strconv.FormatUint(uint64(r.ID), 10) + ":" + strconv.FormatUint(uint64(deviceID), 10)
	silenceCache.Store(keyMem, time.Now())
}

func compare(v interface{}, op string, target interface{}) bool {
	// 数值比较优先
	vf, vok := toFloat(v)
	tf, tok := toFloat(target)
	if vok && tok {
		switch op {
		case ">":
			return vf > tf
		case "<":
			return vf < tf
		case ">=":
			return vf >= tf
		case "<=":
			return vf <= tf
		case "==":
			return vf == tf
		case "!=":
			return vf != tf
		}
		return false
	}
	// 字符串/布尔比较
	vs := fmt.Sprintf("%v", v)
	ts := fmt.Sprintf("%v", target)
	switch op {
	case "==":
		return vs == ts
	case "!=":
		return vs != ts
	}
	return false
}

func toFloat(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(n, 64)
		return f, err == nil
	case bool:
		if n {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}

// validateWebhookURL 基础 SSRF 防护：仅允许 http/https，禁止回环与云元数据地址
func validateWebhookURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("仅支持 http/https")
	}
	host := u.Hostname()
	if host == "localhost" || host == "169.254.169.254" {
		return fmt.Errorf("目标地址不允许")
	}
	if ip := net.ParseIP(host); ip != nil && (ip.IsLoopback() || ip.IsLinkLocalUnicast()) {
		return fmt.Errorf("目标地址不允许")
	}
	return nil
}

func postJSONWithRetry(rawURL string, body interface{}, maxRetries int) {
	if err := validateWebhookURL(rawURL); err != nil {
		slog.Warn("webhook url rejected", "url", rawURL, "err", err)
		return
	}
	if maxRetries <= 0 {
		maxRetries = 3
	}
	backoff := time.Second
	for attempt := 0; attempt <= maxRetries; attempt++ {
		err := doPostJSON(rawURL, body)
		if err == nil {
			return
		}
		if attempt < maxRetries {
			slog.Warn("webhook failed, retrying", "url", rawURL, "attempt", attempt+1, "error", err)
			time.Sleep(backoff)
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
		} else {
			slog.Error("webhook failed after retries", "url", rawURL, "attempts", maxRetries+1, "error", err)
		}
	}
}

func doPostJSON(rawURL string, body interface{}) error {
	data, _ := json.Marshal(body)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(rawURL, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}
