package api

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"iot-platform/internal/model"
	"iot-platform/internal/repository"
	"iot-platform/internal/service"
)

// ExportDeviceHistory 按时间范围导出设备历史数据为 CSV（可按参数过滤）
// ?start=毫秒&end=毫秒&fields=a,b,c（fields 为空 = 物模型全部属性）
func ExportDeviceHistory(c *gin.Context) {
	var d model.Device
	if err := repository.DB.Scopes(ownedScope(c, "")).Where("id = ?", c.Param("id")).First(&d).Error; err != nil {
		Fail(c, 404, "设备不存在")
		return
	}
	startMs := atoi64(c.Query("start"))
	endMs := atoi64(c.Query("end"))
	if startMs <= 0 || endMs <= startMs {
		Fail(c, 400, "请指定有效的时间范围（start/end 毫秒时间戳）")
		return
	}
	// 导出范围保护：最多 31 天
	if endMs-startMs > 31*24*3600*1000 {
		Fail(c, 400, "导出时间范围不能超过 31 天")
		return
	}

	// 列：fields 参数为空 = 物模型全部属性；否则按指定标识符
	fields := []string{}
	if f := c.Query("fields"); f != "" {
		for _, s := range strings.Split(f, ",") {
			if s = strings.TrimSpace(s); s != "" {
				fields = append(fields, s)
			}
		}
	} else {
		if props, err := service.LoadThingModelProps(d.ProductID); err == nil {
			for _, p := range props {
				if id, ok := p["identifier"].(string); ok {
					fields = append(fields, id)
				}
			}
		}
	}
	if len(fields) == 0 {
		Fail(c, 400, "没有可导出的参数（产品未定义物模型属性）")
		return
	}

	filename := fmt.Sprintf("history_%s_%s_%d.csv", d.Name, time.Now().Format("20060102_150405"), d.ID)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "text/csv; charset=utf-8")
	w := c.Writer
	// UTF-8 BOM：Excel 直接打开不乱码
	w.Write([]byte{0xEF, 0xBB, 0xBF})
	w.Write([]byte("time"))
	for _, f := range fields {
		w.Write([]byte(","))
		w.Write([]byte(f))
	}
	w.Write([]byte("\n"))

	// 游标分页读取（数据量大时避免一次全量加载）
	start := time.UnixMilli(startMs)
	end := time.UnixMilli(endMs)
	lastTs := start
	for {
		var rows []model.Telemetry
		if err := repository.DB.
			Where("device_id = ? AND ts > ? AND ts <= ?", d.ID, lastTs, end).
			Order("ts asc").Limit(1000).Find(&rows).Error; err != nil || len(rows) == 0 {
			break
		}
		for _, r := range rows {
			var data map[string]interface{}
			if json.Unmarshal(r.Data, &data) != nil {
				continue
			}
			var sb strings.Builder
			sb.WriteString(r.Ts.Format("2006-01-02 15:04:05"))
			for _, f := range fields {
				sb.WriteString(",")
				if v, ok := data[f]; ok {
					sb.WriteString(csvValue(v))
				}
			}
			sb.WriteString("\n")
			w.WriteString(sb.String())
		}
		lastTs = rows[len(rows)-1].Ts
	}
}

// csvValue CSV 单元格：数值直写，字符串加引号转义
func csvValue(v interface{}) string {
	switch val := v.(type) {
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		if val {
			return "1"
		}
		return "0"
	case string:
		return `"` + strings.ReplaceAll(val, `"`, `""`) + `"`
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", val)
	}
}
