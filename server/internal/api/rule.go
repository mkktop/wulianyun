package api

import (
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"

	"iot-platform/internal/model"
	"iot-platform/internal/repository"
	"iot-platform/internal/rule"
)

type ruleReq struct {
	Name      string         `json:"name" binding:"required,max=64"`
	Type      string         `json:"type" binding:"required,oneof=alarm offline forward"`
	ProductID uint           `json:"productId"`
	DeviceID  uint           `json:"deviceId"`
	Condition datatypes.JSON `json:"condition"`
	Action    datatypes.JSON `json:"action"`
	Silence   int            `json:"silence"`
	Enabled   *bool          `json:"enabled"`
}

func CreateRule(c *gin.Context) {
	var req ruleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, 400, "规则名称和类型必填")
		return
	}
	r := model.Rule{
		UserID: UID(c), Name: req.Name, Type: req.Type,
		ProductID: req.ProductID, DeviceID: req.DeviceID,
		Condition: req.Condition, Action: req.Action,
		Silence: req.Silence, Enabled: true,
	}
	if r.Silence <= 0 {
		r.Silence = 5
	}
	if req.Enabled != nil {
		r.Enabled = *req.Enabled
	}
	if err := repository.DB.Create(&r).Error; err != nil {
		Fail(c, 500, "创建失败")
		return
	}
	rule.InvalidateRuleCache(UID(c))
	OK(c, r)
}

func ListRules(c *gin.Context) {
	q := repository.DB.Model(&model.Rule{}).Where("user_id = ?", UID(c))
	if t := c.Query("type"); t != "" {
		q = q.Where("type = ?", t)
	}
	var total int64
	q.Count(&total)
	page, size := pageArgs(c)
	var list []model.Rule
	q.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&list)
	// 补充产品/设备名
	for i := range list {
		if list[i].ProductID > 0 {
			var p model.Product
			if repository.DB.Select("name").First(&p, list[i].ProductID).Error == nil {
				list[i].ProductName = p.Name
			}
		}
		if list[i].DeviceID > 0 {
			var d model.Device
			if repository.DB.Select("name").First(&d, list[i].DeviceID).Error == nil {
				list[i].DeviceName = d.Name
			}
		}
	}
	OK(c, PageData{Total: total, List: list})
}

func UpdateRule(c *gin.Context) {
	var r model.Rule
	if err := repository.DB.Where("id = ? AND user_id = ?", c.Param("id"), UID(c)).First(&r).Error; err != nil {
		Fail(c, 404, "规则不存在")
		return
	}
	var req ruleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, 400, "参数错误")
		return
	}
	updates := map[string]interface{}{
		"name": req.Name, "type": req.Type,
		"product_id": req.ProductID, "device_id": req.DeviceID,
		"condition": req.Condition, "action": req.Action,
	}
	if req.Silence > 0 {
		updates["silence"] = req.Silence
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	repository.DB.Model(&r).Updates(updates)
	rule.InvalidateRuleCache(UID(c))
	OK(c, r)
}

func DeleteRule(c *gin.Context) {
	res := repository.DB.Where("id = ? AND user_id = ?", c.Param("id"), UID(c)).Delete(&model.Rule{})
	if res.RowsAffected == 0 {
		Fail(c, 404, "规则不存在")
		return
	}
	rule.InvalidateRuleCache(UID(c))
	OK(c, nil)
}

// ListAlarms 告警记录列表
func ListAlarms(c *gin.Context) {
	q := repository.DB.Model(&model.Alarm{}).Where("user_id = ?", UID(c))
	if st := c.Query("status"); st != "" {
		q = q.Where("status = ?", st)
	}
	if lv := c.Query("level"); lv != "" {
		q = q.Where("level = ?", lv)
	}
	var total int64
	q.Count(&total)
	page, size := pageArgs(c)
	var list []model.Alarm
	q.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&list)
	OK(c, PageData{Total: total, List: list})
}

// ResolveAlarm 处理告警（标记已解决）
func ResolveAlarm(c *gin.Context) {
	var a model.Alarm
	if err := repository.DB.Where("id = ? AND user_id = ?", c.Param("id"), UID(c)).First(&a).Error; err != nil {
		Fail(c, 404, "告警不存在")
		return
	}
	now := time.Now()
	repository.DB.Model(&a).Updates(map[string]interface{}{
		"status": model.AlarmStatusResolved, "resolved_at": now,
	})
	OK(c, nil)
}

// ConfirmAlarm 确认告警（标记已确认，不改状态）
func ConfirmAlarm(c *gin.Context) {
	var a model.Alarm
	if err := repository.DB.Where("id = ? AND user_id = ?", c.Param("id"), UID(c)).First(&a).Error; err != nil {
		Fail(c, 404, "告警不存在")
		return
	}
	now := time.Now()
	repository.DB.Model(&a).Update("confirmed_at", now)
	OK(c, nil)
}

// AlarmStats 告警统计（概览用）
func AlarmStats(c *gin.Context) {
	uid := UID(c)
	var firing, resolved, total, todayCount int64
	repository.DB.Model(&model.Alarm{}).Where("user_id = ? AND status = ?", uid, model.AlarmStatusFiring).Count(&firing)
	repository.DB.Model(&model.Alarm{}).Where("user_id = ? AND status = ?", uid, model.AlarmStatusResolved).Count(&resolved)
	repository.DB.Model(&model.Alarm{}).Where("user_id = ?", uid).Count(&total)
	today := time.Now().Truncate(24 * time.Hour)
	repository.DB.Model(&model.Alarm{}).Where("user_id = ? AND created_at >= ?", uid, today).Count(&todayCount)
	OK(c, gin.H{"total": total, "firing": firing, "resolved": resolved, "today": todayCount})
}

// AlarmTrend 近7日告警趋势
func AlarmTrend(c *gin.Context) {
	uid := UID(c)
	type dayCount struct {
		Day   string `json:"day"`
		Count int64  `json:"count"`
	}
	var trend []dayCount
	today := time.Now().Truncate(24 * time.Hour)
	repository.DB.Raw(`
		SELECT to_char(date_trunc('day', created_at), 'MM-DD') AS day, count(*) AS count
		FROM alarms
		WHERE user_id = ? AND created_at >= ?
		GROUP BY 1 ORDER BY 1
	`, uid, today.AddDate(0, 0, -6)).Scan(&trend)
	OK(c, trend)
}
