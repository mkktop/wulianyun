package api

import (
	"strings"
	"testing"
)

// TestOpenAPISignCoversRequest 验证签名绑定 method/path/body：
// 篡改任一部分都会导致签名不一致（防"捕获签名→改写请求"式重放）。
func TestOpenAPISignCoversRequest(t *testing.T) {
	const (
		secret = "c9d2f1e8aabbccdd1122334455667788"
		appKey = "ak3f8a1c2d9e4b"
		ts     = "1785712345"
	)
	body := []byte(`{"key":"value"}`)
	base := openapiSignString(secret, "POST", "/openapi/v1/devices/3/property", body, appKey, ts)

	cases := []struct {
		name  string
		mut   func() string
		same  bool // 篡改后签名是否仍与 base 相同（期望 false）
	}{
		{"原样", func() string { return base }, true},
		{"改 method", func() string { return openapiSignString(secret, "GET", "/openapi/v1/devices/3/property", body, appKey, ts) }, false},
		{"改 path", func() string { return openapiSignString(secret, "POST", "/openapi/v1/devices/3/command", body, appKey, ts) }, false},
		{"改 query", func() string { return openapiSignString(secret, "POST", "/openapi/v1/devices/3/property?x=1", body, appKey, ts) }, false},
		{"改 body", func() string { return openapiSignString(secret, "POST", "/openapi/v1/devices/3/property", []byte(`{"key":"hack"}`), appKey, ts) }, false},
		{"改 timestamp", func() string { return openapiSignString(secret, "POST", "/openapi/v1/devices/3/property", body, appKey, "1785719999") }, false},
		{"改 appKey", func() string { return openapiSignString(secret, "POST", "/openapi/v1/devices/3/property", body, "ak000000000000", ts) }, false},
		{"改 secret", func() string { return openapiSignString("deadbeef", "POST", "/openapi/v1/devices/3/property", body, appKey, ts) }, false},
	}
	for _, tc := range cases {
		got := tc.mut()
		if (got == base) != tc.same {
			t.Errorf("%s: 签名绑定被破坏（got==base 应为 %v，实际 %v）", tc.name, tc.same, got == base)
		}
	}
	if base == "" || strings.ContainsAny(base, "\n") {
		t.Error("签名应为非空 hex 且不含换行")
	}
}

// TestOpenAPISignEmptyBodyStable GET 无 body 时 BodyHash 固定为空串哈希。
func TestOpenAPISignEmptyBodyStable(t *testing.T) {
	a := openapiSignString("s", "GET", "/openapi/v1/devices", nil, "k", "1000")
	b := openapiSignString("s", "GET", "/openapi/v1/devices", []byte{}, "k", "1000")
	if a != b {
		t.Error("nil body 与空 body 应得到相同签名")
	}
}
