package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"iot-platform/internal/config"
	"iot-platform/internal/model"
	"iot-platform/internal/repository"
)

type Claims struct {
	UserID   uint   `json:"uid"`
	Username string `json:"username"`
	Role     string `json:"role"`
	ParentID *uint  `json:"pid,omitempty"` // nil=一级主账号；非空=二级账号
	jwt.RegisteredClaims
}

func GenToken(uid uint, username, role string, parentID *uint) (string, error) {
	// 热更新参数：令牌有效期可被超管后台覆盖（未设置则回退 yaml 配置）
	expireHours := repository.GetSettingInt("jwt_expire_hours", config.C.JWT.ExpireHours)
	claims := Claims{
		UserID: uid, Username: username, Role: role, ParentID: parentID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expireHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(config.C.JWT.Secret))
}

func ParseToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		return []byte(config.C.JWT.Secret), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, jwt.ErrTokenInvalidClaims
}

// JWTAuth 认证中间件；WebSocket 场景支持 query 传 token
func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		if tokenStr == "" {
			tokenStr = c.Query("token")
		}
		if tokenStr == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, Resp{Code: 401, Msg: "未登录"})
			return
		}
		claims, err := ParseToken(tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, Resp{Code: 401, Msg: "登录已过期"})
			return
		}
		// 回查 DB：禁用/删除账号即时失效；角色/层级以库为准（token 内的 claim 可能陈旧，
		// 否则被降级/改父账号的用户在 token 有效期内仍持有旧权限）
		var u model.User
		if err := repository.DB.Select("id, role, parent_id, status").First(&u, claims.UserID).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, Resp{Code: 401, Msg: "账号不存在"})
			return
		}
		if u.Status == model.AccountStatusDisabled {
			c.AbortWithStatusJSON(http.StatusUnauthorized, Resp{Code: 401, Msg: "账号已禁用"})
			return
		}
		c.Set("uid", u.ID)
		c.Set("username", claims.Username)
		c.Set("role", u.Role)
		c.Set("pid", u.ParentID)
		c.Next()
	}
}

func UID(c *gin.Context) uint {
	return c.GetUint("uid")
}

// Role 当前账号角色：admin=平台超管 / user=普通账号
func Role(c *gin.Context) string {
	if r, ok := c.Get("role"); ok {
		if s, ok := r.(string); ok {
			return s
		}
	}
	return "user"
}

// ParentID 当前账号的父账号 ID；nil=一级主账号
func ParentID(c *gin.Context) *uint {
	if v, ok := c.Get("pid"); ok {
		if pid, ok := v.(*uint); ok {
			return pid
		}
	}
	return nil
}

// IsAdmin 平台超管（看全部租户数据）
func IsAdmin(c *gin.Context) bool { return Role(c) == "admin" }

// Tier 账号层级：platform(超管) / primary(一级) / secondary(二级)
func Tier(c *gin.Context) string {
	if IsAdmin(c) {
		return "platform"
	}
	if ParentID(c) == nil {
		return "primary"
	}
	return "secondary"
}

// IsPrimary 一级主账号（可创建二级、下放产品）
func IsPrimary(c *gin.Context) bool { return Tier(c) == "primary" }

// PrimaryAuth 一级主账号或平台超管可通过（账号管理、产品下放）。
// 超管以自身为"主账号"创建/管理二级账号（parent_id=超管ID），便于平台运营与演示。
func PrimaryAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !IsAdmin(c) && !IsPrimary(c) {
			c.AbortWithStatusJSON(http.StatusForbidden, Resp{Code: 403, Msg: "仅一级主账号可操作"})
			return
		}
		c.Next()
	}
}

// AdminAuth 仅平台超管可通过（MQTT 调试台等超级权限工具）。
func AdminAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !IsAdmin(c) {
			c.AbortWithStatusJSON(http.StatusForbidden, Resp{Code: 403, Msg: "仅平台超管可操作"})
			return
		}
		c.Next()
	}
}

// RequireOperate 写操作保护（P2 账号内 RBAC）。
// 平台超管/一级主账号恒放行；二级账号按 Permission 判定，view（只读）拒绝。
// 权限实时从库读取（管理员改权限即时生效，无需二级重新登录）。
func RequireOperate() gin.HandlerFunc {
	return func(c *gin.Context) {
		if IsAdmin(c) || IsPrimary(c) {
			c.Next()
			return
		}
		var perm string
		repository.DB.Model(&model.User{}).Where("id = ?", UID(c)).Pluck("permission", &perm)
		if perm == model.AccountPermissionView {
			c.AbortWithStatusJSON(http.StatusForbidden, Resp{Code: 403, Msg: "查看者账号无操作权限"})
			return
		}
		c.Next()
	}
}
