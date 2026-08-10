package storage

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// LocalStore 本地磁盘存储（默认模式，保持原有 uploads/ 行为；单机部署适用）
type LocalStore struct {
	Dir string // 根目录，如 uploads
}

func (s *LocalStore) Put(ctx context.Context, key string, r io.Reader, _ int64) error {
	p, err := s.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	out, err := os.Create(p)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, r)
	out.Close()
	return err
}

func (s *LocalStore) Delete(ctx context.Context, key string) error {
	p, err := s.path(key)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *LocalStore) URL(key string) string {
	return "/" + filepath.ToSlash(filepath.Join(s.Dir, key))
}

// KeyFromURL 兼容旧格式（/uploads/firmware/...）
func (s *LocalStore) KeyFromURL(url string) string {
	prefix := "/" + s.Dir + "/"
	if !strings.HasPrefix(url, prefix) {
		return ""
	}
	return strings.TrimPrefix(url, prefix)
}

// path 安全拼接对象 key 到根目录下，防止路径穿越
func (s *LocalStore) path(key string) (string, error) {
	if strings.Contains(key, "..") {
		return "", errors.New("invalid object key")
	}
	base := filepath.Clean(s.Dir)
	p := filepath.Clean(filepath.Join(s.Dir, filepath.FromSlash(key)))
	if !strings.HasPrefix(p, base+string(filepath.Separator)) {
		return "", errors.New("invalid object key")
	}
	return p, nil
}
