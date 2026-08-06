package api

import (
	"time"

	"github.com/gin-gonic/gin"

	"iot-platform/internal/model"
	"iot-platform/internal/repository"
)

// ProductStats 产品概况统计
func ProductStats(c *gin.Context) {
	p, err := canViewProduct(c, c.Param("id"))
	if err != nil {
		Fail(c, 404, "产品不存在")
		return
	}
	var total, activated, online, todayNew int64
	base := repository.DB.Model(&model.Device{}).Where("product_id = ?", p.ID)
	base.Count(&total)
	repository.DB.Model(&model.Device{}).Where("product_id = ? AND status <> ?", p.ID, model.DeviceStatusInactive).Count(&activated)
	repository.DB.Model(&model.Device{}).Where("product_id = ? AND status = ?", p.ID, model.DeviceStatusOnline).Count(&online)
	today := time.Now().Truncate(24 * time.Hour)
	repository.DB.Model(&model.Device{}).Where("product_id = ? AND created_at >= ?", p.ID, today).Count(&todayNew)

	// 今日消息量
	var msgToday int64
	repository.DB.Model(&model.Telemetry{}).
		Joins("JOIN devices ON devices.id = telemetries.device_id").
		Where("devices.product_id = ? AND telemetries.ts >= ?", p.ID, today).Count(&msgToday)

	// 近14天每日新增设备
	type dayCount struct {
		Day   string `json:"day"`
		Count int64  `json:"count"`
	}
	var deviceTrend []dayCount
	repository.DB.Raw(`
		SELECT to_char(date_trunc('day', created_at), 'MM-DD') AS day, count(*) AS count
		FROM devices WHERE product_id = ? AND created_at >= ?
		GROUP BY 1 ORDER BY 1`, p.ID, today.AddDate(0, 0, -13)).Scan(&deviceTrend)

	// 近7天每日消息量
	var msgTrend []dayCount
	repository.DB.Raw(`
		SELECT to_char(date_trunc('day', t.ts), 'MM-DD') AS day, count(*) AS count
		FROM telemetries t JOIN devices d ON d.id = t.device_id
		WHERE d.product_id = ? AND t.ts >= ?
		GROUP BY 1 ORDER BY 1`, p.ID, today.AddDate(0, 0, -6)).Scan(&msgTrend)

	OK(c, gin.H{
		"total": total, "activated": activated, "online": online,
		"todayNew": todayNew, "msgToday": msgToday,
		"deviceTrend": deviceTrend, "msgTrend": msgTrend,
	})
}

// ListEventReports 事件上报记录（产品或设备维度）
func ListEventReports(c *gin.Context) {
	q := repository.DB.Model(&model.EventReport{}).Scopes(ownedScope(c, ""))
	if pid := c.Query("productId"); pid != "" {
		q = q.Where("product_id = ?", pid)
	}
	if did := c.Query("deviceId"); did != "" {
		q = q.Where("device_id = ?", did)
	}
	if t := c.Query("type"); t != "" {
		q = q.Where("type = ?", t)
	}
	var total int64
	q.Count(&total)
	page, size := pageArgs(c)
	var list []model.EventReport
	q.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&list)
	OK(c, PageData{Total: total, List: list})
}

// ListCommandLogs 指令下发日志（产品或设备维度）
func ListCommandLogs(c *gin.Context) {
	q := repository.DB.Model(&model.CommandLog{}).Scopes(ownedScope(c, ""))
	if pid := c.Query("productId"); pid != "" {
		q = q.Where("product_id = ?", pid)
	}
	if did := c.Query("deviceId"); did != "" {
		q = q.Where("device_id = ?", did)
	}
	var total int64
	q.Count(&total)
	page, size := pageArgs(c)
	var list []model.CommandLog
	q.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&list)
	OK(c, PageData{Total: total, List: list})
}

// ---- 设备分组 ----

func CreateGroup(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required,max=64"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, 400, "分组名称必填")
		return
	}
	g := model.DeviceGroup{UserID: UID(c), Name: req.Name, Description: req.Description}
	if err := repository.DB.Create(&g).Error; err != nil {
		Fail(c, 500, "创建失败")
		return
	}
	OK(c, g)
}

// @Summary      设备分组列表
// @Description  查询当前账号名下全部分组，并批量聚合每组设备数
// @Tags         分组
// @Produce      json
// @Success      200  {object}  Resp
// @Failure      400  {object}  Resp
// @Router       /groups [get]
// @Security     BearerAuth
func ListGroups(c *gin.Context) {
	var list []model.DeviceGroup
	repository.DB.Scopes(ownedScope(c, "")).Order("id desc").Find(&list)
	// 批量聚合 deviceCount，避免 N+1（原先每行一次 Count → 现仅 1 条 GROUP BY 查询）
	if len(list) > 0 {
		ids := make([]uint, len(list))
		for i, g := range list {
			ids[i] = g.ID
		}
		var rows []struct {
			GroupID uint
			Cnt     int64
		}
		repository.DB.Model(&model.Device{}).Select("group_id, count(*) as cnt").
			Where("group_id IN ?", ids).Group("group_id").Scan(&rows)
		m := make(map[uint]int64)
		for _, r := range rows {
			m[r.GroupID] = r.Cnt
		}
		for i := range list {
			list[i].DeviceCount = m[list[i].ID]
		}
	}
	OK(c, list)
}

func UpdateGroup(c *gin.Context) {
	var g model.DeviceGroup
	if err := repository.DB.Scopes(ownedScope(c, "")).Where("id = ?", c.Param("id")).First(&g).Error; err != nil {
		Fail(c, 404, "分组不存在")
		return
	}
	var req struct {
		Name        string `json:"name" binding:"required,max=64"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, 400, "参数错误")
		return
	}
	repository.DB.Model(&g).Updates(map[string]interface{}{"name": req.Name, "description": req.Description})
	OK(c, g)
}

func DeleteGroup(c *gin.Context) {
	res := repository.DB.Scopes(ownedScope(c, "")).Where("id = ?", c.Param("id")).Delete(&model.DeviceGroup{})
	if res.RowsAffected == 0 {
		Fail(c, 404, "分组不存在")
		return
	}
	// 组内设备置为未分组
	repository.DB.Model(&model.Device{}).Where("group_id = ?", c.Param("id")).Update("group_id", 0)
	OK(c, nil)
}

// BatchCreateDevices 批量创建设备
func BatchCreateDevices(c *gin.Context) {
	p, err := canViewProduct(c, c.Param("id"))
	if err != nil {
		Fail(c, 404, "产品不存在")
		return
	}
	var req struct {
		Names []string `json:"names" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Names) == 0 {
		Fail(c, 400, "设备名称列表必填")
		return
	}
	if len(req.Names) > 500 {
		Fail(c, 400, "单次最多导入 500 个设备")
		return
	}
	type failItem struct {
		Name   string `json:"name"`
		Reason string `json:"reason"`
	}
	created := 0
	var failed []failItem
	seen := map[string]bool{}
	for _, name := range req.Names {
		if name == "" {
			continue
		}
		if seen[name] {
			failed = append(failed, failItem{Name: name, Reason: "列表内重复"})
			continue
		}
		seen[name] = true
		var cnt int64
		repository.DB.Model(&model.Device{}).Where("product_key = ? AND name = ?", p.ProductKey, name).Count(&cnt)
		if cnt > 0 {
			failed = append(failed, failItem{Name: name, Reason: "设备已存在"})
			continue
		}
		d := model.Device{
			UserID: UID(c), ProductID: p.ID, ProductKey: p.ProductKey,
			Name: name, Secret: randHex(16), Status: model.DeviceStatusInactive,
		}
		if err := repository.DB.Create(&d).Error; err != nil {
			failed = append(failed, failItem{Name: name, Reason: "创建失败"})
			continue
		}
		created++
	}
	OK(c, gin.H{"created": created, "failed": failed})
}

// DeviceAlarmStats 产品下设备告警统计（按设备+分组聚合）
func DeviceAlarmStats(c *gin.Context) {
	p, err := canViewProduct(c, c.Param("id"))
	if err != nil {
		Fail(c, 404, "产品不存在")
		return
	}

	var stats []model.DeviceAlarmStat
	repository.DB.Raw(`
		SELECT
			d.id AS device_id,
			d.name AS device_name,
			COALESCE(dg.name, '未分组') AS group_name,
			COUNT(a.id) AS total_alarms,
			COUNT(a.id) FILTER (WHERE a.status = 'firing') AS firing_count,
			COUNT(a.id) FILTER (WHERE a.status = 'resolved') AS resolved_count
		FROM devices d
		LEFT JOIN device_groups dg ON dg.id = d.group_id
		LEFT JOIN alarms a ON a.device_id = d.id
		WHERE d.product_id = ?
		GROUP BY d.id, dg.name
		ORDER BY total_alarms DESC
	`, p.ID).Scan(&stats)

	OK(c, stats)
}
