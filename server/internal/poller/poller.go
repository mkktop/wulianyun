// Package poller Modbus 云端轮询引擎：设备（DTU）通过 9100 长连接主动接入后，
// 平台按产品采集周期在该长连接上下发 Modbus RTU 读请求，解析应答为物模型属性并入库。
package poller

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"iot-platform/internal/gateway"
	"iot-platform/internal/model"
	"iot-platform/internal/modbus"
	"iot-platform/internal/repository"
	"iot-platform/internal/service"
)

const requestTimeout = 3 * time.Second

type task struct {
	cancel chan struct{}
}

var (
	mu    sync.Mutex
	tasks = map[string]*task{} // key: productKey.deviceName

	// 变更上报缓存：key = productKey.deviceName.identifier -> 上次上报值
	lastValues sync.Map

	// 点位缓存：key = "productID_groupID" -> []model.ModbusPoint
	pointCache     sync.Map
	pointCacheTTL  = 5 * time.Minute
	pointCacheTime sync.Map

	// 并发限制信号量
	pollSemaphore chan struct{}
)

// 多实例分布式锁：Redis SETNX 防止多实例重复采集同一设备组
var rdb *redis.Client

// InitDistributedLock 初始化 Redis 客户端用于分布式锁
func InitDistributedLock() {
	rdb = repository.RDB
}

// InitPollerSemaphore 初始化轮询并发信号量
func InitPollerSemaphore(maxConcurrent int) {
	if maxConcurrent <= 0 {
		maxConcurrent = 50
	}
	pollSemaphore = make(chan struct{}, maxConcurrent)
}

func getCachedPoints(productID, groupID uint) ([]model.ModbusPoint, bool) {
	key := fmt.Sprintf("%d_%d", productID, groupID)
	if t, ok := pointCacheTime.Load(key); ok {
		if time.Since(t.(time.Time)) < pointCacheTTL {
			if pts, ok := pointCache.Load(key); ok {
				return pts.([]model.ModbusPoint), true
			}
		}
	}
	return nil, false
}

func cachePoints(productID, groupID uint, pts []model.ModbusPoint) {
	key := fmt.Sprintf("%d_%d", productID, groupID)
	pointCache.Store(key, pts)
	pointCacheTime.Store(key, time.Now())
}

// InvalidatePointCache 使指定产品的点位缓存失效
func InvalidatePointCache(productID uint) {
	// 简单实现：清空所有（产品数量不多）
	pointCache.Range(func(k, v any) bool {
		pointCache.Delete(k)
		pointCacheTime.Delete(k)
		return true
	})
}

// Init 注册到 gateway 的上下线钩子
func Init() {
	gateway.OnDeviceConnect = onConnect
	gateway.OnDeviceDisconnect = onDisconnect
}

func onConnect(productKey, deviceName string, productID uint) {
	var p model.Product
	if err := repository.DB.First(&p, productID).Error; err != nil {
		return
	}
	if p.AccessMode != model.AccessModeModbus {
		return
	}
	startDevice(&p, deviceName)
}

func onDisconnect(productKey, deviceName string) {
	key := productKey + "." + deviceName
	mu.Lock()
	if t, ok := tasks[key]; ok {
		close(t.cancel)
		delete(tasks, key)
	}
	mu.Unlock()
	// 清理该设备 on-change 去重缓存（key 前缀 productKey.deviceName.），避免断连/删设备留孤儿键长期内存泄漏（#28）
	prefix := key + "."
	lastValues.Range(func(k, v any) bool {
		if ks, ok := k.(string); ok && len(ks) > len(prefix) && ks[:len(prefix)] == prefix {
			lastValues.Delete(k)
		}
		return true
	})
}

// RefreshDevice 采集组/点位变更后对在线设备重启轮询（#22）：
// 设备连接后经 API 新建的采集组原本要到重连才被采集，刷新后立即生效
func RefreshDevice(productKey, deviceName string) {
	var p model.Product
	if err := repository.DB.Where("product_key = ?", productKey).First(&p).Error; err != nil {
		return
	}
	if p.AccessMode != model.AccessModeModbus {
		return
	}
	if !gateway.Has(productKey, deviceName) {
		return // 离线设备由下次 onConnect 启动
	}
	startDevice(&p, deviceName)
}

func startDevice(p *model.Product, deviceName string) {
	key := p.ProductKey + "." + deviceName
	cancel := make(chan struct{})
	mu.Lock()
	if old, ok := tasks[key]; ok {
		close(old.cancel)
	}
	tasks[key] = &task{cancel: cancel}
	mu.Unlock()

	// 按采集组分别调度（分频）；未分组点位归入默认组（产品周期）
	groups := loadGroups(p)
	for i := range groups {
		g := groups[i]
		go runGroup(p, deviceName, g, cancel)
	}
	slog.Info("modbus poller started", "device", key, "groups", len(groups))
}

// groupPlan 采集组调度单元
type groupPlan struct {
	id         uint
	name       string
	interval   time.Duration
	reportMode string
}

// loadGroups 返回该产品的采集组（含一个虚拟默认组 id=0 承载未分组点位）
func loadGroups(p *model.Product) []groupPlan {
	var gs []model.ModbusGroup
	repository.DB.Where("product_id = ?", p.ID).Find(&gs)
	plans := []groupPlan{{
		id: 0, name: "默认组",
		interval: clampInterval(p.PollInterval), reportMode: model.ReportModePeriodic,
	}}
	for _, g := range gs {
		plans = append(plans, groupPlan{
			id: g.ID, name: g.Name,
			interval: clampInterval(g.PollInterval), reportMode: g.ReportMode,
		})
	}
	return plans
}

func clampInterval(sec int) time.Duration {
	if sec < 1 {
		sec = 60
	}
	return time.Duration(sec) * time.Second
}

// runGroup 单个采集组的独立调度循环
func runGroup(p *model.Product, deviceName string, g groupPlan, cancel chan struct{}) {
	// 首次执行前加随机延迟，避免多设备同时轮询
	jitter := time.Duration(rand.Int63n(int64(g.interval)))
	select {
	case <-cancel:
		return
	case <-time.After(jitter):
	}
	pollGroup(p, deviceName, g)
	ticker := time.NewTicker(g.interval)
	defer ticker.Stop()
	for {
		select {
		case <-cancel:
			return
		case <-ticker.C:
			pollGroup(p, deviceName, g)
		}
	}
}

// pollGroup 采集一个组：合并连续寄存器批量读取 → 变更过滤 → 上报
func pollGroup(p *model.Product, deviceName string, g groupPlan) {
	// 多实例分布式锁：获取锁才执行采集，防止多实例重复轮询同一设备组
	lockKey := fmt.Sprintf("poller:lock:%d_%d_%s", p.ID, g.id, deviceName)
	if rdb != nil {
		ctx := context.Background()
		ok, err := rdb.SetNX(ctx, lockKey, "1", 60*time.Second).Result()
		if err != nil {
			// Redis 抖动/不可用：告警并降级为单实例轮询。
			// 直接 return 会导致该设备组所有采集停止（违反"Redis 不可用降级"约定）
			slog.Warn("poller lock error, degrade to single-instance polling", "key", lockKey, "err", err)
		} else if !ok {
			return // 其他实例正在采集，跳过
		} else {
			defer rdb.Del(ctx, lockKey)
		}
	}

	// 获取信号量
	if pollSemaphore != nil {
		select {
		case pollSemaphore <- struct{}{}:
			defer func() { <-pollSemaphore }()
		case <-time.After(3 * time.Second):
			slog.Warn("poll semaphore timeout", "device", deviceName)
			return
		}
	}

	var points []model.ModbusPoint
	if pts, ok := getCachedPoints(p.ID, g.id); ok {
		points = pts
	} else {
		repository.DB.Where("product_id = ? AND group_id = ?", p.ID, g.id).Order("address asc").Find(&points)
		cachePoints(p.ID, g.id, points)
	}
	if len(points) == 0 {
		return
	}
	ptrs := make([]*model.ModbusPoint, 0, len(points))
	for i := range points {
		ptrs = append(ptrs, &points[i])
	}

	// 合并为最少的批量读请求
	blocks := modbus.PlanReadBlocks(ptrs)
	data := map[string]interface{}{}
	for _, b := range blocks {
		req := modbus.BuildBlockRequest(b)
		resp, err := gateway.Request(p.ProductKey, deviceName, req, requestTimeout)
		if err != nil {
			slog.Warn("modbus block poll failed", "device", deviceName, "group", g.name,
				"slave", b.SlaveID, "start", b.Start, "qty", b.Quantity, "err", err)
			continue
		}
		vals, err := modbus.ParseBlockResponse(b, resp)
		if err != nil {
			slog.Warn("modbus block parse failed", "device", deviceName, "group", g.name, "err", err)
			continue
		}
		for id, v := range vals {
			data[id] = round2(v)
		}
	}
	if len(data) == 0 {
		return
	}

	// 变更上报：只保留与上次不同的点位
	if g.reportMode == model.ReportModeOnChange {
		changed := map[string]interface{}{}
		for id, v := range data {
			ck := p.ProductKey + "." + deviceName + "." + id
			if old, ok := lastValues.Load(ck); ok && old == v {
				continue
			}
			lastValues.Store(ck, v)
			changed[id] = v
		}
		if len(changed) == 0 {
			return
		}
		data = changed
	}

	payload, _ := json.Marshal(data)
	service.HandleTelemetry(p.ProductKey, deviceName, payload)
}

// WriteProperty 写控制：解析 property.set 载荷，按点位表写寄存器/线圈
// payload 形如 {"method":"property.set","params":{"switch":1}} 或直接 {"switch":1}
func WriteProperty(productKey, deviceName string, payload []byte) error {
	var msg struct {
		Params map[string]interface{} `json:"params"`
	}
	json.Unmarshal(payload, &msg)
	params := msg.Params
	if len(params) == 0 {
		json.Unmarshal(payload, &params)
	}
	if len(params) == 0 {
		return fmt.Errorf("无可写参数")
	}

	var p model.Product
	if err := repository.DB.Where("product_key = ?", productKey).First(&p).Error; err != nil {
		return err
	}
	var points []model.ModbusPoint
	repository.DB.Where("product_id = ?", p.ID).Find(&points)
	byID := map[string]*model.ModbusPoint{}
	for i := range points {
		byID[points[i].Identifier] = &points[i]
	}

	var lastErr error
	wrote := 0
	for k, val := range params {
		pt, ok := byID[k]
		if !ok || pt.AccessMode != "rw" {
			continue
		}
		writeFn := writeFuncFor(pt.FunctionCode, pt.RawType)
		if writeFn == 0 {
			lastErr = fmt.Errorf("点位 %s 不可写", k)
			continue
		}
		wp := *pt
		wp.FunctionCode = writeFn
		frame, err := modbus.BuildWriteRequest(&wp, toFloat(val))
		if err != nil {
			lastErr = err
			continue
		}
		// 写操作也等待应答（回显帧），失败不阻塞其他点位
		if _, err := gateway.Request(productKey, deviceName, frame, requestTimeout); err != nil {
			lastErr = err
			continue
		}
		wrote++
	}
	if wrote == 0 && lastErr != nil {
		return lastErr
	}
	return nil
}

// writeFuncFor 由读功能码推导写功能码；离散量输入/输入寄存器只读返回 0
func writeFuncFor(fn int, rawType string) int {
	switch fn {
	case model.FuncReadCoils:
		return model.FuncWriteSingleCoil
	case model.FuncReadHoldingRegisters:
		// 32-bit 点位（int32/uint32/float，占 2 寄存器）必须用 FC16 多寄存器写，
		// FC6 单寄存器写会丢弃高字、第二寄存器不动，静默截断数值
		if rawType == "int32" || rawType == "uint32" || rawType == "float" {
			return model.FuncWriteMultipleRegs
		}
		return model.FuncWriteSingleRegister
	case model.FuncWriteSingleCoil, model.FuncWriteSingleRegister,
		model.FuncWriteMultipleCoils, model.FuncWriteMultipleRegs:
		return fn
	}
	return 0
}

func toFloat(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case bool:
		if n {
			return 1
		}
		return 0
	case string:
		f, _ := strconv.ParseFloat(n, 64)
		return f
	}
	return 0
}

func round2(v float64) float64 {
	// math.Round 对正负数对称舍入（四舍五入到最近的偶数方向取最近整数）；
	// 原实现 int64(v*100+0.5) 对负数向零偏置（-0.51→0 而非 -1），掩盖冷链冻融阈值
	return math.Round(v*100) / 100
}
