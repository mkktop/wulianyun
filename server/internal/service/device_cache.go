package service

import (
	"sync"
	"time"

	"iot-platform/internal/model"
	"iot-platform/internal/repository"
)

type deviceCacheEntry struct {
	device    *model.Device
	expiresAt time.Time
}

var (
	deviceCache    sync.Map // key: "productKey\x00deviceName" -> *deviceCacheEntry
	deviceCacheTTL = 5 * time.Minute
)

func deviceCacheKey(productKey, deviceName string) string {
	return productKey + "\x00" + deviceName
}

func getCachedDevice(productKey, deviceName string) *model.Device {
	key := deviceCacheKey(productKey, deviceName)
	if v, ok := deviceCache.Load(key); ok {
		entry := v.(*deviceCacheEntry)
		if time.Now().Before(entry.expiresAt) {
			return entry.device
		}
		deviceCache.Delete(key)
	}
	return nil
}

func cacheDevice(d *model.Device) {
	key := deviceCacheKey(d.ProductKey, d.Name)
	deviceCache.Store(key, &deviceCacheEntry{
		device:    d,
		expiresAt: time.Now().Add(deviceCacheTTL),
	})
}

func InvalidateDeviceCache(productKey, deviceName string) {
	deviceCache.Delete(deviceCacheKey(productKey, deviceName))
}

func RefreshDeviceCache(d *model.Device) {
	cacheDevice(d)
}

// CachedFindDevice 先查缓存再查DB
func CachedFindDevice(productKey, deviceName string) (*model.Device, error) {
	if d := getCachedDevice(productKey, deviceName); d != nil {
		return d, nil
	}
	var d model.Device
	err := repository.DB.Where("product_key = ? AND name = ?", productKey, deviceName).First(&d).Error
	if err != nil {
		return nil, err
	}
	cacheDevice(&d)
	return &d, nil
}

func InitDeviceCache(ttlSeconds int) {
	if ttlSeconds > 0 {
		deviceCacheTTL = time.Duration(ttlSeconds) * time.Second
	}
}
