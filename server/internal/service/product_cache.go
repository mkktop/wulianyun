package service

import (
	"sync"
	"time"

	"iot-platform/internal/model"
	"iot-platform/internal/repository"
)

// 产品配置缓存：设备鉴权（FindDeviceForAuth）、下行选路（DownPublisher）、
// Modbus 写属性（WriteProperty）都会按 productKey 查询产品表。
// 万级设备重连风暴时这是万级 QPS 的查询脉冲；而产品的接入方式/密钥模式/
// 协议创建后不可变，仅采集周期/组帧等少数字段可变——短 TTL 缓存完全可接受。
type productCacheEntry struct {
	product   *model.Product
	expiresAt time.Time
}

var (
	productCache    sync.Map // productKey -> *productCacheEntry
	productCacheTTL = 60 * time.Second
)

// FindProductByKey 按 productKey 查产品（带 TTL 缓存）
func FindProductByKey(productKey string) (*model.Product, error) {
	if v, ok := productCache.Load(productKey); ok {
		entry := v.(*productCacheEntry)
		if time.Now().Before(entry.expiresAt) {
			return entry.product, nil
		}
		productCache.Delete(productKey)
	}
	var p model.Product
	if err := repository.DB.Where("product_key = ?", productKey).First(&p).Error; err != nil {
		return nil, err
	}
	productCache.Store(productKey, &productCacheEntry{product: &p, expiresAt: time.Now().Add(productCacheTTL)})
	return &p, nil
}

// InvalidateProductCache 产品更新/删除后使缓存失效
func InvalidateProductCache(productKey string) {
	productCache.Delete(productKey)
}

// InitProductCache 配置缓存 TTL（秒）
func InitProductCache(ttlSeconds int) {
	if ttlSeconds > 0 {
		productCacheTTL = time.Duration(ttlSeconds) * time.Second
	}
}
