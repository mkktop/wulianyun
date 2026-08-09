package service

import (
	"log/slog"
	"sync"
	"time"

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
	// 多实例一致性：Redis 可用（多实例）时绕过本地内存缓存，直接走 DB 读改写，
	// 避免跨实例本地缓存陈旧导致 desired/reported 不一致（#29）。单实例仍享缓存加速
	if repository.RDB != nil {
		return nil
	}
	if v, ok := shadowCache.Load(deviceID); ok {
		return v.(*shadowCacheEntry)
	}
	// 从DB加载
	var s model.DeviceShadow
	if err := repository.DB.Where("device_id = ?", deviceID).First(&s).Error; err != nil {
		return nil
	}
	entry := &shadowCacheEntry{shadow: &s}
	actual, _ := shadowCache.LoadOrStore(deviceID, entry)
	return actual.(*shadowCacheEntry)
}

func CachedGetShadow(deviceID uint) *model.DeviceShadow {
	entry := getShadowEntry(deviceID)
	if entry == nil {
		return nil
	}
	entry.mu.Lock()
	defer entry.mu.Unlock()
	return entry.shadow
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
			if err := repository.DB.Save(entry.shadow).Error; err != nil {
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
