package rule

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"iot-platform/internal/model"
	"iot-platform/internal/repository"
	"iot-platform/internal/ws"
)

// silenceCache 静默期缓存 ruleID:deviceID -> 上次触发时间
var silenceCache sync.Map

// EvalTelemetry 遥测数据进入规则引擎（由 ingest 调用，异步）
func EvalTelemetry(d *model.Device, data map[string]interface{}) {
	var rules []model.Rule
	repository.DB.Where(
		"user_id = ? AND enabled = true AND type IN ? AND (product_id = 0 OR product_id = ?) AND (device_id = 0 OR device_id = ?)",
		d.UserID, []string{model.RuleTypeAlarm, model.RuleTypeForward}, d.ProductID, d.ID,
	).Find(&rules)

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

type alarmCondition struct {
	Field string      `json:"field"`
	Op    string      `json:"op"` // > < >= <= == !=
	Value interface{} `json:"value"`
}

type alarmAction struct {
	Level      string   `json:"level"`
	Notify     []string `json:"notify"` // ws / webhook
	WebhookURL string   `json:"webhookUrl"`
}

func evalAlarmRule(r *model.Rule, d *model.Device, data map[string]interface{}) {
	var cond alarmCondition
	if json.Unmarshal(r.Condition, &cond) != nil || cond.Field == "" {
		return
	}
	v, ok := data[cond.Field]
	if !ok {
		return
	}
	if !compare(v, cond.Op, cond.Value) {
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
	msg := fmt.Sprintf("设备[%s] %s=%v 触发条件 %s %v", d.Name, cond.Field, v, cond.Op, cond.Value)
	fireAlarm(r, d, act, msg, data)
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

	// 站内实时通知（默认始终推送）
	ws.H.PushAlarm(r.UserID, map[string]interface{}{
		"id": alarm.ID, "ruleName": r.Name, "deviceId": d.ID, "deviceName": d.Name,
		"level": act.Level, "message": msg, "ts": alarm.CreatedAt.UnixMilli(),
	})

	// Webhook 通知
	for _, n := range act.Notify {
		if n == "webhook" && act.WebhookURL != "" {
			go postJSON(act.WebhookURL, map[string]interface{}{
				"type": "alarm", "rule": r.Name, "device": d.Name,
				"level": act.Level, "message": msg, "data": data,
				"ts": time.Now().UnixMilli(),
			})
		}
	}
	slog.Info("alarm fired", "rule", r.Name, "device", d.Name, "msg", msg)
}

type forwardAction struct {
	WebhookURL string `json:"webhookUrl"`
}

func doForward(r *model.Rule, d *model.Device, data map[string]interface{}) {
	var act forwardAction
	if json.Unmarshal(r.Action, &act) != nil || act.WebhookURL == "" {
		return
	}
	go postJSON(act.WebhookURL, map[string]interface{}{
		"type": "telemetry", "productKey": d.ProductKey, "device": d.Name,
		"data": data, "ts": time.Now().UnixMilli(),
	})
}

func inSilence(r *model.Rule, deviceID uint) bool {
	if r.Silence <= 0 {
		return false
	}
	key := strconv.FormatUint(uint64(r.ID), 10) + ":" + strconv.FormatUint(uint64(deviceID), 10)
	if v, ok := silenceCache.Load(key); ok {
		if time.Since(v.(time.Time)) < time.Duration(r.Silence)*time.Minute {
			return true
		}
	}
	return false
}

func markFired(r *model.Rule, deviceID uint) {
	key := strconv.FormatUint(uint64(r.ID), 10) + ":" + strconv.FormatUint(uint64(deviceID), 10)
	silenceCache.Store(key, time.Now())
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

func postJSON(url string, body interface{}) {
	if err := validateWebhookURL(url); err != nil {
		slog.Warn("webhook url rejected", "url", url, "err", err)
		return
	}
	data, _ := json.Marshal(body)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		slog.Warn("webhook post failed", "url", url, "err", err)
		return
	}
	resp.Body.Close()
}
