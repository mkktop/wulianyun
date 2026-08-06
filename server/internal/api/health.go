package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"iot-platform/internal/repository"
)

// Healthz 存活探针：进程在跑即 200，不检查任何依赖。
// 供 Docker compose healthcheck 使用（不因 DB 短暂抖动而重启容器）。
func Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Readyz 就绪探针：DB 可连通才 200，否则 503。
// 供部署/升级脚本轮询，确认 repository.Init（AutoMigrate + TimescaleDB 超表）已执行完成。
func Readyz(c *gin.Context) {
	if repository.DB == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "degraded", "db": "uninitialized"})
		return
	}
	sqlDB, err := repository.DB.DB()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "degraded", "db": "down"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "degraded", "db": "down"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ready", "db": "up"})
}
