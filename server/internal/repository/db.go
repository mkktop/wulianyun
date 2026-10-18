package repository

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"iot-platform/internal/config"
	"iot-platform/internal/model"
)

var (
	DB  *gorm.DB
	RDB *redis.Client
)

// queryTimeout 热路径查询/写入的统一短超时。
// 背景：热路径大量运行在无请求上下文的后台协程（遥测 ingest/规则/影子/缓冲 flush），
// context.Background() 下 DB 网络挂起会无限占用连接（ConnMaxLifetime 1800s 才回收），
// 池耗尽后所有请求排进 database/sql 的无界等待队列，与遥测协程堆积叠加放大成全局瘫痪。
// 批量写入等长操作可按需用 CtxWith 自定时长。
const queryTimeout = 5 * time.Second

// Ctx 返回带热路径超时的 context（调用方须 defer cancel 释放）
func Ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), queryTimeout)
}

// CtxWith 自定超时时长的热路径 context
func CtxWith(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}

func Init() error {
	db, err := gorm.Open(postgres.Open(config.C.Database.DSN), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
	})
	if err != nil {
		return err
	}
	DB = db

	// 配置数据库连接池
	sqlDB, err := db.DB()
	if err == nil {
		sqlDB.SetMaxOpenConns(config.C.Database.MaxOpenConns)
		sqlDB.SetMaxIdleConns(config.C.Database.MaxIdleConns)
		sqlDB.SetConnMaxLifetime(time.Duration(config.C.Database.ConnMaxLifetime) * time.Second)
		sqlDB.SetConnMaxIdleTime(time.Duration(config.C.Database.ConnMaxIdleTime) * time.Second)
	}

	if err := DB.AutoMigrate(
		&model.User{}, &model.Product{}, &model.Device{},
		&model.Telemetry{}, &model.DeviceEvent{},
		&model.ThingModel{}, &model.DeviceShadow{}, &model.Rule{}, &model.Alarm{},
		&model.OpenApp{}, &model.ModbusPoint{},
		&model.EventReport{}, &model.CommandLog{}, &model.DeviceGroup{},
		&model.ModbusGroup{}, &model.CommandRequest{},
		&model.MessageTrace{}, &model.DeviceLog{},
		&model.Firmware{}, &model.OTATask{},
		&model.ProductGrant{},
		&model.Announcement{}, &model.HelpDoc{}, &model.SystemSetting{},
		&model.StorageConfig{},
	); err != nil {
		return err
	}

	// 标签过滤（tags @>）与指令超时扫描的热路径索引；已存在时幂等
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_devices_tags ON devices USING gin (tags)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_command_requests_pending ON command_requests (created_at) WHERE status = 'pending'")

	// 尝试启用 TimescaleDB 并把 telemetries 转为超表；失败不阻塞（退化为普通表）
	if err := DB.Exec("CREATE EXTENSION IF NOT EXISTS timescaledb").Error; err != nil {
		slog.Warn("timescaledb extension unavailable, telemetry falls back to plain table", "err", err)
	} else if err := DB.Exec(
		"SELECT create_hypertable('telemetries', 'ts', if_not_exists => TRUE, migrate_data => TRUE)",
	).Error; err != nil {
		slog.Warn("create_hypertable failed", "err", err)
	}

	// TimescaleDB 数据保留策略
	retentionDays := config.C.Database.RetentionDays
	if retentionDays <= 0 {
		retentionDays = 30
	}
	DB.Exec(fmt.Sprintf(
		"SELECT add_retention_policy('telemetries', INTERVAL '%d days', if_not_exists => TRUE)",
		retentionDays,
	))

	// TimescaleDB 压缩策略
	compressAfter := config.C.Database.CompressAfterDays
	if compressAfter <= 0 {
		compressAfter = 7
	}
	DB.Exec("ALTER TABLE telemetries SET (timescaledb.compress, timescaledb.compress_segmentby = 'device_id')")
	DB.Exec(fmt.Sprintf(
		"SELECT add_compression_policy('telemetries', INTERVAL '%d days', if_not_exists => TRUE)",
		compressAfter,
	))

	RDB = redis.NewClient(&redis.Options{
		Addr:         config.C.Redis.Addr,
		Password:     config.C.Redis.Password,
		DB:           config.C.Redis.DB,
		PoolSize:     config.C.Redis.PoolSize,
		MinIdleConns: config.C.Redis.MinIdleConns,
	})
	if err := RDB.Ping(context.Background()).Err(); err != nil {
		slog.Warn("redis unavailable, latest-value cache disabled", "err", err)
	}

	// 启动定期清理例程
	go cleanupLoop()
	return nil
}

// cleanupLoop 定期清理消息轨迹和设备日志
func cleanupLoop() {
	// 启动后立即执行一次
	doCleanup()

	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		doCleanup()
	}
}

// doCleanup 按当前生效的保留天数清理（热更新参数可覆盖 yaml 默认值，每次执行时读取）
func doCleanup() {
	traceDays := GetSettingInt("trace_retention_days", config.C.Log.TraceRetentionDays)
	if traceDays <= 0 {
		traceDays = 7
	}
	logDays := GetSettingInt("device_log_retention_days", config.C.Log.DeviceLogRetentionDays)
	if logDays <= 0 {
		logDays = 30
	}

	cutoff := time.Now().AddDate(0, 0, -traceDays)
	if result := DB.Where("created_at < ?", cutoff).Delete(&model.MessageTrace{}); result.Error != nil {
		slog.Warn("cleanup old traces failed", "err", result.Error)
	} else if result.RowsAffected > 0 {
		slog.Info("cleaned old traces", "count", result.RowsAffected, "older_than", cutoff)
	}

	cutoff2 := time.Now().AddDate(0, 0, -logDays)
	if result := DB.Where("created_at < ?", cutoff2).Delete(&model.DeviceLog{}); result.Error != nil {
		slog.Warn("cleanup old device logs failed", "err", result.Error)
	} else if result.RowsAffected > 0 {
		slog.Info("cleaned old device logs", "count", result.RowsAffected, "older_than", cutoff2)
	}
}
