package api

import (
	"strings"
	"testing"
)

// TestOpenAPISignCoversRequest 验证签名绑定 method/path/body/nonce：
// 篡改任一部分都会导致签名不一致（防"捕获签名→改写请求"式重放）。
func TestOpenAPISignCoversRequest(t *testing.T) {
	const (
		secret = "c9d2f1e8aabbccdd1122334455667788"
		appKey = "ak3f8a1c2d9e4b"
		ts     = "1785712345"
		nonce  = "n-8f3a2b1c9d7e4f6a"
	)
	body := []byte(`{"key":"value"}`)
	base := openapiSignString(secret, "POST", "/openapi/v1/devices/3/property", body, appKey, ts, nonce)

	cases := []struct {
		name  string
		mut   func() string
		same  bool // 篡改后签名是否仍与 base 相同（期望 false）
	}{
		{"原样", func() string { return base }, true},
		{"改 method", func() string { return openapiSignString(secret, "GET", "/openapi/v1/devices/3/property", body, appKey, ts, nonce) }, false},
		{"改 path", func() string { return openapiSignString(secret, "POST", "/openapi/v1/devices/3/command", body, appKey, ts, nonce) }, false},
		{"改 query", func() string { return openapiSignString(secret, "POST", "/openapi/v1/devices/3/property?x=1", body, appKey, ts, nonce) }, false},
		{"改 body", func() string { return openapiSignString(secret, "POST", "/openapi/v1/devices/3/property", []byte(`{"key":"hack"}`), appKey, ts, nonce) }, false},
		{"改 timestamp", func() string { return openapiSignString(secret, "POST", "/openapi/v1/devices/3/property", body, appKey, "1785719999", nonce) }, false},
		{"改 appKey", func() string { return openapiSignString(secret, "POST", "/openapi/v1/devices/3/property", body, "ak000000000000", ts, nonce) }, false},
		{"改 secret", func() string { return openapiSignString("deadbeef", "POST", "/openapi/v1/devices/3/property", body, appKey, ts, nonce) }, false},
		{"改 nonce", func() string { return openapiSignString(secret, "POST", "/openapi/v1/devices/3/property", body, appKey, ts, "n-replayed-0001") }, false},
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
	a := openapiSignString("s", "GET", "/openapi/v1/devices", nil, "k", "1000", "n12345678")
	b := openapiSignString("s", "GET", "/openapi/v1/devices", []byte{}, "k", "1000", "n12345678")
	if a != b {
		t.Error("nil body 与空 body 应得到相同签名")
	}
}

// TestOpenAPINonceDedup nonce 重放去重（内存降级路径）：同一 (appKey,nonce) 第二次判定为重复。
// Redis 不可用的测试环境下 nonceDuplicate 走内存兜底，行为与 Redis SETNX 等价。
func TestOpenAPINonceDedup(t *testing.T) {
	nonceSeenMu.Lock()
	nonceSeen = map[string]int64{} // 清场（Redis 不可用时才会走内存，见 nonceDuplicate）
	nonceSeenMu.Unlock()

	appKey, nonce := "akTestNonce0001", "n-abcdef0123456789"
	if nonceDuplicate(appKey, nonce) {
		t.Fatal("首次使用的 nonce 不应判定为重复")
	}
	if !nonceDuplicate(appKey, nonce) {
		t.Fatal("同一 nonce 第二次应判定为重复（重放）")
	}
	// 不同 appKey 的相同 nonce 不互斥
	if nonceDuplicate("akOtherAccount2", nonce) {
		t.Fatal("不同 AppKey 的 nonce 命名空间应隔离")
	}
}
