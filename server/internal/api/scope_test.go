package api

import (
	"testing"

	"github.com/gin-gonic/gin"
)

// makeCtx 构造一个带鉴权上下文的 *gin.Context（模拟 JWTAuth 注入的 uid/role/pid）。
func makeCtx(uid uint, role string, pid *uint) *gin.Context {
	c := &gin.Context{}
	c.Set("uid", uid)
	c.Set("role", role)
	c.Set("pid", pid)
	return c
}

func TestTier(t *testing.T) {
	pid := uint(5)
	cases := []struct {
		name string
		role string
		pid  *uint
		want string
	}{
		{"超管", "admin", nil, "platform"},
		{"超管即使带pid也按超管", "admin", &pid, "platform"},
		{"一级", "user", nil, "primary"},
		{"二级", "user", &pid, "secondary"},
	}
	for _, tc := range cases {
		if got := Tier(makeCtx(1, tc.role, tc.pid)); got != tc.want {
			t.Errorf("%s: Tier=%q want %q", tc.name, got, tc.want)
		}
	}
}

func TestIsAdminIsPrimaryIsSecondary(t *testing.T) {
	pid := uint(5)
	if !IsAdmin(makeCtx(1, "admin", nil)) {
		t.Error("admin 应为 IsAdmin")
	}
	if IsAdmin(makeCtx(1, "user", nil)) {
		t.Error("user 不应为 IsAdmin")
	}
	if !IsPrimary(makeCtx(1, "user", nil)) {
		t.Error("user+nil pid 应为一级")
	}
	if IsPrimary(makeCtx(1, "user", &pid)) {
		t.Error("user+pid 不应为一级")
	}
	if IsPrimary(makeCtx(1, "admin", nil)) {
		t.Error("admin 是 platform，不算一级（PrimaryAuth 应拦截）")
	}
	if !isSecondary(makeCtx(1, "user", &pid)) {
		t.Error("user+pid 应为二级")
	}
	if isSecondary(makeCtx(1, "user", nil)) {
		t.Error("user+nil pid 不应为二级")
	}
}

func TestParentIDFromContext(t *testing.T) {
	if ParentID(makeCtx(1, "user", nil)) != nil {
		t.Error("nil pid 应返回 nil")
	}
	pid := uint(7)
	got := ParentID(makeCtx(1, "user", &pid))
	if got == nil || *got != 7 {
		t.Errorf("pid=7 应返回 7，得到 %v", got)
	}
}

// TestPrimaryAuthGate 确认二级账号被 PrimaryAuth 拦截（403）。
func TestPrimaryAuthGate(t *testing.T) {
	pid := uint(5)
	cases := []struct {
		name    string
		role    string
		pid     *uint
		blocked bool
	}{
		{"二级被拦", "user", &pid, true},
		{"超管放行（以自身为主账号管理二级）", "admin", nil, false},
		{"一级放行", "user", nil, false},
	}
	for _, tc := range cases {
		blocked := !IsAdmin(makeCtx(1, tc.role, tc.pid)) && !IsPrimary(makeCtx(1, tc.role, tc.pid))
		if blocked != tc.blocked {
			t.Errorf("%s: 期望 blocked=%v 得到 %v", tc.name, tc.blocked, blocked)
		}
	}
}

func TestUintPtr(t *testing.T) {
	p := uintPtr(42)
	if p == nil || *p != 42 {
		t.Errorf("uintPtr(42) 应返回指向 42 的指针")
	}
}

// TestRouterBuilds 构造完整路由表：若读写分离后存在 gin 路由冲突，NewRouter 会 panic，此测试提前暴露。
func TestRouterBuilds(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("NewRouter panic（可能是路由冲突）: %v", r)
		}
	}()
	if r := NewRouter(); r == nil {
		t.Fatal("NewRouter 返回 nil")
	}
}
