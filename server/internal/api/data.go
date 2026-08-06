package api

import (
	"encoding/json"
	"math"
	"time"

	"github.com/gin-gonic/gin"

	"iot-platform/internal/model"
	"iot-platform/internal/repository"
	"iot-platform/internal/service"
	"iot-platform/internal/ws"
)

// DeviceLatest 设备最新遥测
func DeviceLatest(c *gin.Context) {
	var d model.Device
	if err := repository.DB.Scopes(ownedScope(c, "")).Where("id = ?", c.Param("id")).First(&d).Error; err != nil {
		Fail(c, 404, "设备不存在")
		return
	}
	OK(c, service.GetLatest(d.ID))
}

// DeviceHistory 历史遥测查询 ?start=毫秒&end=毫秒&limit=n
func DeviceHistory(c *gin.Context) {
	var d model.Device
	if err := repository.DB.Scopes(ownedScope(c, "")).Where("id = ?", c.Param("id")).First(&d).Error; err != nil {
		Fail(c, 404, "设备不存在")
		return
	}
	end := time.Now()
	start := end.Add(-time.Hour)
	if v := c.Query("start"); v != "" {
		if ms := atoi64(v); ms > 0 {
			start = time.UnixMilli(ms)
		}
	}
	if v := c.Query("end"); v != "" {
		if ms := atoi64(v); ms > 0 {
			end = time.UnixMilli(ms)
		}
	}
	limit := 2000
	if v := c.Query("limit"); v != "" {
		if n := atoi(v); n > 0 && n <= 5000 {
			limit = n
		}
	}
	var rows []model.Telemetry
	if err := repository.DB.
		Where("device_id = ? AND ts >= ? AND ts <= ?", d.ID, start, end).
		Order("ts asc").Limit(limit).Find(&rows).Error; err != nil {
		Fail(c, 500, "查询失败")
		return
	}
	type point struct {
		Ts   int64                  `json:"ts"`
		Data map[string]interface{} `json:"data"`
	}
	points := make([]point, 0, len(rows))
	for _, r := range rows {
		var data map[string]interface{}
		if json.Unmarshal(r.Data, &data) == nil {
			points = append(points, point{Ts: r.Ts.UnixMilli(), Data: data})
		}
	}
	OK(c, points)
}

// SendCommand 向设备下发消息（透传 JSON）
func SendCommand(c *gin.Context) {
	var d model.Device
	if err := repository.DB.Scopes(ownedScope(c, "")).Where("id = ?", c.Param("id")).First(&d).Error; err != nil {
		Fail(c, 404, "设备不存在")
		return
	}
	if d.Status != model.DeviceStatusOnline {
		Fail(c, 400, "设备不在线")
		return
	}
	raw, err := c.GetRawData()
	if err != nil || len(raw) == 0 || !json.Valid(raw) {
		Fail(c, 400, "命令必须是合法 JSON")
		return
	}
	if err := service.DownPublisher(d.ProductKey, d.Name, raw); err != nil {
		Fail(c, 500, "下发失败: "+err.Error())
		return
	}
	OK(c, nil)
}

// Overview 平台概览统计
func Overview(c *gin.Context) {
	var productCount int64
	repository.DB.Model(&model.Product{}).Scopes(productScope(c)).Count(&productCount)

	var deviceCount, onlineCount int64
	repository.DB.Model(&model.Device{}).Scopes(ownedScope(c, "")).Count(&deviceCount)
	repository.DB.Model(&model.Device{}).Scopes(ownedScope(c, "")).Where("status = ?", model.DeviceStatusOnline).Count(&onlineCount)

	// 设备状态分布（大屏用）
	type statusCount struct {
		Status string `json:"status"`
		Count  int64  `json:"count"`
	}
	var statusDist []statusCount
	repository.DB.Model(&model.Device{}).Select("status, count(*) as count").
		Scopes(ownedScope(c, "")).Group("status").Scan(&statusDist)

	// 今日消息量
	today := time.Now().Truncate(24 * time.Hour)
	var msgToday int64
	repository.DB.Model(&model.Telemetry{}).
		Joins("JOIN devices ON devices.id = telemetries.device_id").
		Scopes(ownedScope(c, "devices.user_id")).
		Where("telemetries.ts >= ?", today).Count(&msgToday)

	// 近7日每日消息量
	type dayCount struct {
		Day   string `json:"day"`
		Count int64  `json:"count"`
	}
	var trend []dayCount
	repository.DB.Model(&model.Telemetry{}).
		Joins("JOIN devices ON devices.id = telemetries.device_id").
		Scopes(ownedScope(c, "devices.user_id")).
		Where("telemetries.ts >= ?", today.AddDate(0, 0, -6)).
		Select("to_char(date_trunc('day', telemetries.ts), 'MM-DD') AS day, count(*) AS count").
		Group("day").Order("day").Scan(&trend)

	// 设备在线率
	onlineRate := 0.0
	if deviceCount > 0 {
		onlineRate = float64(onlineCount) / float64(deviceCount) * 100
	}

	// 最近一小时平均每分钟消息量（吞吐量指标）
	oneHourAgo := time.Now().Add(-time.Hour)
	var msgLastHour int64
	repository.DB.Model(&model.Telemetry{}).
		Joins("JOIN devices ON devices.id = telemetries.device_id").
		Scopes(ownedScope(c, "devices.user_id")).
		Where("telemetries.ts >= ?", oneHourAgo).Count(&msgLastHour)
	msgRateMin := msgLastHour / 60 // 每分钟平均

	// 总消息量
	var msgTotal int64
	repository.DB.Model(&model.Telemetry{}).
		Joins("JOIN devices ON devices.id = telemetries.device_id").
		Scopes(ownedScope(c, "devices.user_id")).Count(&msgTotal)

	OK(c, gin.H{
		"productCount": productCount,
		"deviceCount":  deviceCount,
		"onlineCount":  onlineCount,
		"onlineRate":   math.Round(onlineRate*10) / 10, // 保留一位小数
		"msgToday":     msgToday,
		"msgTotal":     msgTotal,
		"msgRateMin":   msgRateMin,
		"msgTrend":     trend,
		"statusDist":   statusDist,
	})
}

// WSHandler WebSocket 实时推送入口
func WSHandler(c *gin.Context) {
	ws.H.Serve(c.Writer, c.Request, UID(c))
}

func atoi64(s string) int64 {
	var n int64
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0
		}
		n = n*10 + int64(ch-'0')
	}
	return n
}
