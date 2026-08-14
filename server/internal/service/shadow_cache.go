package service

import (
	"log/slog"
	"sync"
	"time"

	"iot-platform/internal/config"
	"iot-platform/internal/model"
	"iot-platform/internal/repository"
)

type shadowCacheEntry struct {
	shadow *model.DeviceShadow
	dirty  bool
	mu     sync.Mutex
}

var (
	shadowCache         sync.Map // deviceID -> *shadowCacheEntry
	shadowFlushInterval = 5 * time.Second
	shadowQuit          = make(chan struct{})
)

func InitShadowCache(flushIntervalSeconds int) {
	if flushIntervalSeconds > 0 {
		shadowFlushInterval = time.Duration(flushIntervalSeconds) * time.Second
	}
	go shadowFlushLoop()
}

func ShutdownShadowCache() {
	close(shadowQuit)
	shadowFlushAll()
}

func getShadowEntry(deviceID uint) *shadowCacheEntry {
	// 多实例一致性：显式声明多实例部署（multi_instance: true）时绕过本地内存缓存，
	// 直接走 DB 读改写，避免跨实例本地缓存陈旧导致 desired/reported 不一致（#29）。
	// 单实例（默认）始终享缓存加速——注意不能用"Redis 可用"作判据：
	// RDB 客户端在任何部署形态下都会创建，用它判断会让本地缓存永远不生效
	if repository.RDB != nil && config.C.MultiInstance {
		return nil
	}
	if v, ok := shadowCache.Load(deviceID); ok {
		return v.(*shadowCacheEntry)
	}
	// 从DB加载
	var s model.DeviceShadow
	ctx, cancel := repository.Ctx()
	defer cancel()
	if err := repository.DB.WithContext(ctx).Where("device_id = ?", deviceID).First(&s).Error; err != nil {
		return nil
	}
	entry := &shadowCacheEntry{shadow: &s}
	actual, _ := shadowCache.LoadOrStore(deviceID, entry)
	return actual.(*shadowCacheEntry)
}

// CachedGetShadow 读取缓存的影子；持锁深拷贝后返回，
// 避免调用方在锁外读活指针与并发写（desired-set/reported 合并）构成数据竞争
func CachedGetShadow(deviceID uint) *model.DeviceShadow {
	entry := getShadowEntry(deviceID)
	if entry == nil {
		return nil
	}
	entry.mu.Lock()
	cp := cloneShadow(entry.shadow)
	entry.mu.Unlock()
	return cp
}

func MarkShadowDirty(deviceID uint) {
	if v, ok := shadowCache.Load(deviceID); ok {
		entry := v.(*shadowCacheEntry)
		entry.mu.Lock()
		entry.dirty = true
		entry.mu.Unlock()
	}
}

func shadowFlushLoop() {
	ticker := time.NewTicker(shadowFlushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			shadowFlushAll()
		case <-shadowQuit:
			return
		}
	}
}

func shadowFlushAll() {
	shadowCache.Range(func(key, value any) bool {
		entry := value.(*shadowCacheEntry)
		entry.mu.Lock()
		if entry.dirty {
			ctx, cancel := repository.CtxWith(10 * time.Second) // 批量落库给足时长
			err := repository.DB.WithContext(ctx).Save(entry.shadow).Error
			cancel()
			if err != nil {
				slog.Error("shadow flush failed", "deviceID", key, "error", err)
			} else {
				entry.dirty = false
			}
		}
		entry.mu.Unlock()
		return true
	})
}

func InvalidateShadowCache(deviceID uint) {
	shadowCache.Delete(deviceID)
}
