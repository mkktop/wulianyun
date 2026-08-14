package config

import (
	"log/slog"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	// MultiInstance 是否多实例水平部署。影响单/多实例行为选择：
	// true 时影子走 DB 读改写（本地缓存跨实例会陈旧）；false（默认）时影子享本地缓存
	MultiInstance bool `yaml:"multi_instance"`
	Server        struct {
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
	// AdminPassword 首次启动创建 admin 用户的密码；为空时回退默认 admin123（仅首次无 admin 时生效）
	AdminPassword string `yaml:"admin_password"`
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
		// CallbackSecret EMQX 认证/授权回调的共享密钥：配置后 EMQX 回调必须携带
		// 同值的 X-Callback-Secret 头，防止回调端点被第三方直接调用充当口令暴破 oracle。
		// 需在 EMQX 认证器（HTTP Auth）的 headers 中配置同一密钥；留空则跳过校验（兼容存量）
		CallbackSecret string `yaml:"callback_secret"`
		TLS            struct {
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
		// Workers 遥测处理 worker 数（按设备哈希分片保序）；0 用默认 64
		Workers int `yaml:"workers"`
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
	EMQXRule struct {
		Enabled bool `yaml:"enabled"` // EMQX 规则引擎接管遥测入库，后端跳过 DB 写入
	} `yaml:"emqx_rule"`
	Storage struct {
		Type         string `yaml:"type"`          // 固件对象存储：local（本地磁盘，默认）| s3（S3 兼容：阿里 OSS/腾讯 COS/MinIO）
		LocalDir     string `yaml:"local_dir"`     // local 模式存储根目录，默认 uploads
		Endpoint     string `yaml:"endpoint"`      // s3 模式：服务端点（如 oss-cn-hangzhou.aliyuncs.com，不带协议）
		Region       string `yaml:"region"`        // s3 模式：区域（AWS 必填，阿里 OSS 可留空）
		Bucket       string `yaml:"bucket"`        // s3 模式：桶名（需设为公开读，设备直连下载）
		AccessKey    string `yaml:"access_key"`    // s3 模式：AccessKey
		SecretKey    string `yaml:"secret_key"`    // s3 模式：SecretKey
		UseSSL       bool   `yaml:"use_ssl"`       // s3 模式：https 访问（注意廉价 4G 模组 TLS 兼容性，默认 http）
		PublicDomain string `yaml:"public_domain"` // s3 模式：公开下载域名（CDN/自定义域名），空则用 bucket.endpoint
	} `yaml:"storage"`
}

var C Config

func Load(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(data, &C); err != nil {
		return err
	}
	// 密钥支持环境变量注入（多实例共享同一密钥 / 容器化部署密钥不入配置文件）
	if v := os.Getenv("IOT_JWT_SECRET"); v != "" {
		C.JWT.Secret = v
	}
	// 安全提醒：JWT 密钥为空或为仓库公开的默认占位符时，任何拿到该字符串的人
	// 都能伪造 admin 令牌接管平台——生产环境务必配置随机强密钥（≥32 字节）
	if C.JWT.Secret == "" || len(C.JWT.Secret) < 16 || C.JWT.Secret == "iot-platform-jwt-secret-change-me" {
		slog.Error("⚠ JWT secret 为空或为默认占位符（iot-platform-jwt-secret-change-me），"+
			"可被伪造 admin 令牌接管平台！生产环境必须改为随机强密钥（支持环境变量 IOT_JWT_SECRET 注入）")
	}
	return nil
}
