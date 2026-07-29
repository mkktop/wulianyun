package repository

import (
	"context"
	"log/slog"

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

func Init() error {
	db, err := gorm.Open(postgres.Open(config.C.Database.DSN), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
	})
	if err != nil {
		return err
	}
	DB = db

	if err := DB.AutoMigrate(
		&model.User{}, &model.Product{}, &model.Device{},
		&model.Telemetry{}, &model.DeviceEvent{},
		&model.ThingModel{}, &model.DeviceShadow{}, &model.Rule{}, &model.Alarm{},
		&model.OpenApp{},
	); err != nil {
		return err
	}

	// 尝试启用 TimescaleDB 并把 telemetries 转为超表；失败不阻塞（退化为普通表）
	if err := DB.Exec("CREATE EXTENSION IF NOT EXISTS timescaledb").Error; err != nil {
		slog.Warn("timescaledb extension unavailable, telemetry falls back to plain table", "err", err)
	} else if err := DB.Exec(
		"SELECT create_hypertable('telemetries', 'ts', if_not_exists => TRUE, migrate_data => TRUE)",
	).Error; err != nil {
		slog.Warn("create_hypertable failed", "err", err)
	}

	RDB = redis.NewClient(&redis.Options{
		Addr:     config.C.Redis.Addr,
		Password: config.C.Redis.Password,
		DB:       config.C.Redis.DB,
	})
	if err := RDB.Ping(context.Background()).Err(); err != nil {
		slog.Warn("redis unavailable, latest-value cache disabled", "err", err)
	}
	return nil
}
