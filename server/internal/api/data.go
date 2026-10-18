package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strings"
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
		// 上限 1 万：每行都要 json.Unmarshal 进内存，5 万行大 payload 单请求即可占数十 MB
		if n := atoi(v); n > 0 && n <= 10000 {
			limit = n
		}
	}
	var rows []model.Telemetry
	if err := repository.DB.
		Where("device_id = ? AND ts >= ? AND ts <= ? AND valid = ?", d.ID, start, end, true).
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

// DeviceHistoryAgg 历史遥测聚合查询 ?start=毫秒&end=毫秒&fields=a,b,c&agg=avg&interval=5m
// 长时间范围查询原始点既慢又超出前端可渲染量级，按时间桶聚合（avg/min/max/sum/count）。
// 桶对齐用 epoch 取整（floor(extract(epoch)/N)*N），普通表与 TimescaleDB 超表行为一致。
func DeviceHistoryAgg(c *gin.Context) {
	var d model.Device
	if err := repository.DB.Scopes(ownedScope(c, "")).Where("id = ?", c.Param("id")).First(&d).Error; err != nil {
		Fail(c, 404, "设备不存在")
		return
	}
	end := time.Now()
	start := end.Add(-24 * time.Hour)
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
	if !end.After(start) {
		Fail(c, 400, "end 必须晚于 start")
		return
	}

	// 聚合函数白名单（防注入）；fields 最多 10 个，每个 ≤64 字符
	agg := c.DefaultQuery("agg", "avg")
	aggFn, ok := map[string]string{
		"avg": "avg", "min": "min", "max": "max", "sum": "sum", "count": "count",
	}[agg]
	if !ok {
		Fail(c, 400, "agg 仅支持 avg/min/max/sum/count")
		return
	}
	fields := []string{}
	for _, f := range strings.Split(c.Query("fields"), ",") {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		if len(f) > 64 || len(fields) >= 10 {
			Fail(c, 400, "fields 非法（最多 10 个，单个 ≤64 字符）")
			return
		}
		fields = append(fields, f)
	}
	if len(fields) == 0 {
		Fail(c, 400, "fields 必填（逗号分隔的属性标识符）")
		return
	}

	// 桶宽：缺省按时间范围自动取 ≤600 桶的"整"步长；显式传入则校验 [10s, 30d]
	rangeSec := end.Sub(start).Seconds()
	intervalSec := 0
	if v := c.Query("interval"); v != "" {
		intervalSec = parseIntervalSeconds(v)
		if intervalSec <= 0 {
			Fail(c, 400, "interval 非法（如 30s/1m/5m/1h/1d）")
			return
		}
		if intervalSec < 10 || intervalSec > 30*24*3600 {
			Fail(c, 400, "interval 须在 10s ~ 30d 之间")
			return
		}
	} else {
		intervalSec = niceIntervalSeconds(int(rangeSec/600) + 1)
	}
	if rangeSec/float64(intervalSec) > 5000 {
		Fail(c, 400, "时间范围相对 interval 过大（桶数 >5000），请增大 interval 或缩短范围")
		return
	}

	// 动态列：CASE 正则守卫防止非数值（bool/字符串/日期）进 ::numeric 转换报错；
	// 正则与字段名均以参数绑定——GORM 的 Raw 占位符替换不感知 SQL 引号，
	// 正则里的字面 '?' 必须作为参数传入，不能内联进 SQL 文本；别名用固定 f0..fn，回填时映射回原字段名
	const numericRegex = `^-?[0-9]+(\.[0-9]+)?([eE][+-]?[0-9]+)?$`
	var sb strings.Builder
	args := []interface{}{intervalSec, intervalSec}
	sb.WriteString("SELECT (floor(extract(epoch FROM ts) / ?) * ? * 1000)::bigint AS bucket")
	for i, f := range fields {
		sb.WriteString(fmt.Sprintf(", %s(CASE WHEN data->> ? ~ ? THEN (data->> ?)::numeric END)::float8 AS f%d", aggFn, i))
		args = append(args, f, numericRegex, f)
	}
	sb.WriteString(" FROM telemetries WHERE device_id = ? AND ts >= ? AND ts <= ? AND valid = true GROUP BY 1 ORDER BY 1 LIMIT 5000")
	args = append(args, d.ID, start, end)

	rows, err := repository.DB.WithContext(c.Request.Context()).Raw(sb.String(), args...).Rows()
	if err != nil {
		Fail(c, 500, "聚合查询失败")
		return
	}
	defer rows.Close()

	type point struct {
		Ts   int64                  `json:"ts"`
		Data map[string]interface{} `json:"data"`
	}
	points := make([]point, 0, 512)
	vals := make([]interface{}, 1+len(fields))
	for rows.Next() {
		bucket := int64(0)
		vals[0] = &bucket
		cols := make([]*float64, len(fields))
		for i := range cols {
			vals[i+1] = &cols[i]
		}
		if err := rows.Scan(vals...); err != nil {
			Fail(c, 500, "聚合结果读取失败")
			return
		}
		data := make(map[string]interface{}, len(fields))
		for i, f := range fields {
			if cols[i] != nil {
				data[f] = *cols[i]
			}
		}
		points = append(points, point{Ts: bucket, Data: data})
	}
	OK(c, gin.H{"agg": agg, "intervalSeconds": intervalSec, "points": points})
}

// parseIntervalSeconds 解析 30s / 5m / 1h / 7d 形式的桶宽
func parseIntervalSeconds(s string) int {
	if s == "" || len(s) > 8 {
		return 0
	}
	n := 0
	for _, ch := range s[:len(s)-1] {
		if ch < '0' || ch > '9' {
			return 0
		}
		n = n*10 + int(ch-'0')
	}
	switch s[len(s)-1] {
	case 's':
		return n
	case 'm':
		return n * 60
	case 'h':
		return n * 3600
	case 'd':
		return n * 86400
	}
	return 0
}

// niceIntervalSeconds 把"期望桶宽秒数"取整到常用的整步长（10s~1d），
// 保证时间轴刻度对齐且桶数不爆炸
func niceIntervalSeconds(sec int) int {
	steps := []int{10, 30, 60, 5 * 60, 15 * 60, 30 * 60, 3600, 3 * 3600, 6 * 3600, 12 * 3600, 86400}
	for _, s := range steps {
		if sec <= s {
			return s
		}
	}
	return 86400
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

	// 总消息量：hypertable 全量 COUNT 在大数据量下是分钟级重查询，
	// 加 8s 超时防止一个概览页请求长时间占用连接池；超时/失败返回 0 而非整体 500
	msgTotalCtx, cancelMsgTotal := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancelMsgTotal()
	var msgTotal int64
	if err := repository.DB.WithContext(msgTotalCtx).Model(&model.Telemetry{}).
		Joins("JOIN devices ON devices.id = telemetries.device_id").
		Scopes(ownedScope(c, "devices.user_id")).Count(&msgTotal).Error; err != nil {
		slog.Warn("overview msg total count failed (large table?)", "err", err)
		msgTotal = 0
	}

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
// 认证方式：升级后首帧必须发送 {type:"auth", token}（token 不放进 URL，防泄露进访问日志）
func WSHandler(c *gin.Context) {
	ws.H.Serve(c.Writer, c.Request)
}

// ValidateWSToken WS 首帧 token 校验：解析 JWT 并回查账号状态（禁用/删除/改密即时失效）
func ValidateWSToken(token string) (uint, error) {
	claims, err := ParseToken(token)
	if err != nil {
		return 0, err
	}
	var u model.User
	if err := repository.DB.Select("id, status, password_version").First(&u, claims.UserID).Error; err != nil {
		return 0, fmt.Errorf("账号不存在")
	}
	if u.Status == model.AccountStatusDisabled {
		return 0, fmt.Errorf("账号已禁用")
	}
	if claims.PwdVer != u.PasswordVersion {
		return 0, fmt.Errorf("凭证已失效")
	}
	return u.ID, nil
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
