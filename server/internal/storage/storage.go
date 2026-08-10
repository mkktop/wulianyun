// Package storage 固件对象存储抽象：local（本地磁盘，默认/开发环境）| s3（S3 兼容：阿里 OSS/腾讯 COS/MinIO）。
// 设备通过公开下载 URL 直连拉取固件，平台只负责上传与删除，不承载下载流量。
package storage

import (
	"context"
	"fmt"
	"io"
)

// ObjectStore 固件存储接口
type ObjectStore interface {
	// Put 写入对象（key 为对象名，如 firmware/xxx.bin）
	Put(ctx context.Context, key string, r io.Reader, size int64) error
	// Delete 删除对象（对象不存在时视为成功）
	Delete(ctx context.Context, key string) error
	// URL 返回对象的公开下载 URL（ota.push 直接下发给设备）
	URL(key string) string
	// KeyFromURL 从下载 URL 反解对象 key（删除时用；非本存储的 URL 返回空串）
	KeyFromURL(url string) string
}

// Options 存储初始化参数
type Options struct {
	Type         string // local | s3
	LocalDir     string // local 模式根目录
	Endpoint     string // s3 端点（不带协议）
	Region       string // s3 区域（AWS 必填）
	Bucket       string // s3 桶名
	AccessKey    string
	SecretKey    string
	UseSSL       bool   // https 访问
	PublicDomain string // 公开下载域名（CDN），空则用 bucket.endpoint
}

// Default 全局默认存储，由 main 启动时 Init 注入
var Default ObjectStore

// Init 按配置初始化默认存储
func Init(opts Options) (err error) {
	Default, err = New(opts)
	return
}

// Reinit 热重载存储（超管在后台改对象存储配置后调用，替换 Default，无需重启）
func Reinit(opts Options) error {
	store, err := New(opts)
	if err != nil {
		return err
	}
	Default = store
	return nil
}

// New 按类型创建存储实现
func New(opts Options) (ObjectStore, error) {
	switch opts.Type {
	case "", "local":
		dir := opts.LocalDir
		if dir == "" {
			dir = "uploads"
		}
		return &LocalStore{Dir: dir}, nil
	case "s3":
		return NewS3Store(opts)
	default:
		return nil, fmt.Errorf("unknown storage type: %q（仅支持 local / s3）", opts.Type)
	}
}
