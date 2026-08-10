package storage

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// S3Store S3 兼容对象存储（阿里 OSS / 腾讯 COS / MinIO）。
// 桶需设为公开读：设备直连下载固件，平台不承载下载流量；
// 对象名含随机串不可猜测，URL 短且永久有效，适配 4G 模组/嵌入式 HTTP 栈。
type S3Store struct {
	client    *minio.Client
	bucket    string
	publicURL string // 公开下载前缀，如 http://bucket.oss-cn-hangzhou.aliyuncs.com
}

// NewS3Store 连接并校验桶存在（不存在则创建）
func NewS3Store(opts Options) (*S3Store, error) {
	client, err := minio.New(opts.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(opts.AccessKey, opts.SecretKey, ""),
		Secure: opts.UseSSL,
		Region: opts.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("init s3 client failed: %w", err)
	}
	ctx := context.Background()
	exists, err := client.BucketExists(ctx, opts.Bucket)
	if err != nil {
		return nil, fmt.Errorf("check bucket %q failed: %w", opts.Bucket, err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, opts.Bucket, minio.MakeBucketOptions{Region: opts.Region}); err != nil {
			return nil, fmt.Errorf("create bucket %q failed: %w", opts.Bucket, err)
		}
		slog.Info("s3 bucket created", "bucket", opts.Bucket)
	}

	scheme := "http"
	if opts.UseSSL {
		scheme = "https"
	}
	host := opts.PublicDomain
	if host == "" {
		// 虚拟托管风格：bucket.endpoint（阿里 OSS / MinIO / 腾讯 COS 均支持）
		host = opts.Bucket + "." + opts.Endpoint
	}

	return &S3Store{
		client:    client,
		bucket:    opts.Bucket,
		publicURL: scheme + "://" + host,
	}, nil
}

func (s *S3Store) Put(ctx context.Context, key string, r io.Reader, size int64) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, r, size, minio.PutObjectOptions{
		ContentType: "application/octet-stream",
	})
	return err
}

func (s *S3Store) Delete(ctx context.Context, key string) error {
	return s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
}

func (s *S3Store) URL(key string) string {
	return s.publicURL + "/" + key
}

func (s *S3Store) KeyFromURL(url string) string {
	prefix := s.publicURL + "/"
	if strings.HasPrefix(url, prefix) {
		return strings.TrimPrefix(url, prefix)
	}
	return ""
}
