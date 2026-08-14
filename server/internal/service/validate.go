package service

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"iot-platform/internal/model"
	"iot-platform/internal/repository"
)

// 物模型属性缓存：物模型变更频率极低（管理操作），而遥测热路径每条消息都要
// 加载属性定义做校验与白名单过滤——无缓存时 5k msg/s 就是 10k QPS 的纯浪费查询。
// 按 productID 分区 + TTL 5 分钟 + 写路径主动失效（InvalidateThingModelCache）。
var (
	tmCacheMu   sync.RWMutex
	tmCache     = map[uint][]map[string]interface{}{} // productID -> 属性定义（含空切片负缓存）
	tmCacheTime = map[uint]time.Time{}
	tmCacheTTL  = 5 * time.Minute
)

// InvalidateThingModelCache 物模型保存/导入后使缓存失效
func InvalidateThingModelCache(productID uint) {
	tmCacheMu.Lock()
	delete(tmCache, productID)
	delete(tmCacheTime, productID)
	tmCacheMu.Unlock()
}

// LoadThingModelProps 加载产品物模型属性定义（带 TTL 缓存）；无物模型或解析失败时返回错误
func LoadThingModelProps(productID uint) ([]map[string]interface{}, error) {
	tmCacheMu.RLock()
	if t, ok := tmCacheTime[productID]; ok && time.Since(t) < tmCacheTTL {
		if props, ok2 := tmCache[productID]; ok2 {
			tmCacheMu.RUnlock()
			return props, nil
		}
	}
	tmCacheMu.RUnlock()

	var tm model.ThingModel
	if err := repository.DB.Where("product_id = ?", productID).First(&tm).Error; err != nil {
		// 无物模型：负缓存空切片，避免每个无物模型产品的遥测都打一次空查询
		tmCacheMu.Lock()
		tmCache[productID] = nil
		tmCacheTime[productID] = time.Now()
		tmCacheMu.Unlock()
		return nil, err
	}
	var props []map[string]interface{}
	if len(tm.Properties) > 0 {
		if err := json.Unmarshal(tm.Properties, &props); err != nil {
			return nil, err
		}
	}
	tmCacheMu.Lock()
	tmCache[productID] = props
	tmCacheTime[productID] = time.Now()
	tmCacheMu.Unlock()
	return props, nil
}

// ValidateTelemetry 按产品物模型校验上行遥测数据（缓存加载属性定义）
// 返回 (是否全部合法, 错误列表)
func ValidateTelemetry(productID uint, data map[string]interface{}) (bool, []string) {
	props, err := LoadThingModelProps(productID)
	if err != nil || len(props) == 0 {
		return true, nil // 无物模型定义，放行
	}
	return ValidateTelemetryProps(props, data)
}

// ValidateTelemetryProps 按已加载的属性定义校验遥测数据（热路径复用同一次加载，避免二次查询）
func ValidateTelemetryProps(props []map[string]interface{}, data map[string]interface{}) (bool, []string) {
	if len(props) == 0 {
		return true, nil
	}

	// 建立属性定义映射
	propMap := make(map[string]map[string]interface{})
	for _, p := range props {
		if id, ok := p["identifier"].(string); ok {
			propMap[id] = p
		}
	}

	var errors []string
	for key, val := range data {
		def, ok := propMap[key]
		if !ok {
			continue // 未定义字段，放行（warning级别）
		}
		if errs := validateField(key, val, def); len(errs) > 0 {
			errors = append(errors, errs...)
		}
	}

	return len(errors) == 0, errors
}

func validateField(name string, val interface{}, def map[string]interface{}) []string {
	var errs []string
	dataType, _ := def["dataType"].(string)

	switch dataType {
	case "int32":
		v, ok := toFloat64(val)
		if !ok {
			errs = append(errs, fmt.Sprintf("%s: expected int32, got %T", name, val))
			break
		}
		if min, ok := def["min"]; ok {
			if minV, ok := toFloat64(min); ok && v < minV {
				errs = append(errs, fmt.Sprintf("%s: value %v below min %v", name, v, minV))
			}
		}
		if max, ok := def["max"]; ok {
			if maxV, ok := toFloat64(max); ok && v > maxV {
				errs = append(errs, fmt.Sprintf("%s: value %v above max %v", name, v, maxV))
			}
		}
	case "float", "double":
		v, ok := toFloat64(val)
		if !ok {
			errs = append(errs, fmt.Sprintf("%s: expected %s, got %T", name, dataType, val))
			break
		}
		if min, ok := def["min"]; ok {
			if minV, ok := toFloat64(min); ok && v < minV {
				errs = append(errs, fmt.Sprintf("%s: value %v below min %v", name, v, minV))
			}
		}
		if max, ok := def["max"]; ok {
			if maxV, ok := toFloat64(max); ok && v > maxV {
				errs = append(errs, fmt.Sprintf("%s: value %v above max %v", name, v, maxV))
			}
		}
	case "bool":
		switch val.(type) {
		case bool:
			// ok
		case float64:
			// ok (0/1)
		default:
			errs = append(errs, fmt.Sprintf("%s: expected bool, got %T", name, val))
		}
	case "enum":
		// 检查值是否在枚举列表中
		if enumSpec, ok := def["enumSpec"]; ok {
			if enumList, ok := enumSpec.([]interface{}); ok {
				found := false
				for _, e := range enumList {
					if em, ok := e.(map[string]interface{}); ok {
						if fmt.Sprintf("%v", em["value"]) == fmt.Sprintf("%v", val) {
							found = true
							break
						}
					}
				}
				if !found {
					errs = append(errs, fmt.Sprintf("%s: value %v not in enum", name, val))
				}
			}
		}
	case "text":
		if _, ok := val.(string); !ok {
			errs = append(errs, fmt.Sprintf("%s: expected text, got %T", name, val))
		}
	case "date":
		// 时间戳，数值类型
		if _, ok := toFloat64(val); !ok {
			errs = append(errs, fmt.Sprintf("%s: expected date timestamp, got %T", name, val))
		}
	}
	return errs
}

func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case json.Number:
		f, err := val.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}
