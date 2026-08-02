package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server struct {
		Addr string `yaml:"addr"`
	} `yaml:"server"`
	Gateway struct {
		Addr           string `yaml:"addr"`
		IdleTimeout    int    `yaml:"idle_timeout"`
		MaxConnsPerIP  int    `yaml:"max_conns_per_ip"`
		ConnRateLimit  int    `yaml:"conn_rate_limit"`  // 每秒令牌补充数
		ConnRateBurst  int    `yaml:"conn_rate_burst"`  // 桶容量
		TLS            struct {
			Enabled  bool   `yaml:"enabled"`
			CertFile string `yaml:"cert_file"`
			KeyFile  string `yaml:"key_file"`
		} `yaml:"tls"`
	} `yaml:"gateway"`
	JWT struct {
		Secret      string `yaml:"secret"`
		ExpireHours int    `yaml:"expire_hours"`
	} `yaml:"jwt"`
	Database struct {
		DSN               string `yaml:"dsn"`
		MaxOpenConns      int    `yaml:"max_open_conns"`
		MaxIdleConns      int    `yaml:"max_idle_conns"`
		ConnMaxLifetime   int    `yaml:"conn_max_lifetime"`   // 秒
		ConnMaxIdleTime   int    `yaml:"conn_max_idle_time"`  // 秒
		RetentionDays     int    `yaml:"retention_days"`
		CompressAfterDays int    `yaml:"compress_after_days"`
	} `yaml:"database"`
	Redis struct {
		Addr         string `yaml:"addr"`
		Password     string `yaml:"password"`
		DB           int    `yaml:"db"`
		PoolSize     int    `yaml:"pool_size"`
		MinIdleConns int    `yaml:"min_idle_conns"`
	} `yaml:"redis"`
	MQTT struct {
		Broker   string `yaml:"broker"`
		ClientID string `yaml:"client_id"`
		Username string `yaml:"username"`
		Password string `yaml:"password"`
		TLS      struct {
			Enabled            bool   `yaml:"enabled"`
			CACert             string `yaml:"ca_cert"`
			ClientCert         string `yaml:"client_cert"`
			ClientKey          string `yaml:"client_key"`
			InsecureSkipVerify bool   `yaml:"insecure_skip_verify"`
		} `yaml:"tls"`
	} `yaml:"mqtt"`
	TelemetryBuffer struct {
		MaxBatch      int `yaml:"max_batch"`
		FlushInterval int `yaml:"flush_interval"` // 秒
	} `yaml:"telemetry_buffer"`
	Cache struct {
		DeviceTTL           int `yaml:"device_ttl"`            // 秒
		ShadowFlushInterval int `yaml:"shadow_flush_interval"` // 秒
	} `yaml:"cache"`
	Poller struct {
		MaxConcurrent int `yaml:"max_concurrent"` // 最大并发轮询数
	} `yaml:"poller"`
	Log struct {
		TraceRetentionDays int `yaml:"trace_retention_days"` // 消息轨迹保留天数
		DeviceLogRetentionDays int `yaml:"device_log_retention_days"` // 设备日志保留天数
	} `yaml:"log"`
}

var C Config

func Load(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, &C)
}
