package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"iot-platform/internal/config"
)

type Claims struct {
	UserID   uint   `json:"uid"`
	Username string `json:"username"`
	Role     string `json:"role"`
	ParentID *uint  `json:"pid,omitempty"` // nil=一级主账号；非空=二级账号
	jwt.RegisteredClaims
}

func GenToken(uid uint, username, role string, parentID *uint) (string, error) {
	claims := Claims{
		UserID: uid, Username: username, Role: role, ParentID: parentID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(config.C.JWT.ExpireHours) * time.Hour)),
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
		c.Set("uid", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Set("pid", claims.ParentID)
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

// PrimaryAuth 仅一级主账号可通过（账号管理、产品下放）
func PrimaryAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !IsPrimary(c) {
			c.AbortWithStatusJSON(http.StatusForbidden, Resp{Code: 403, Msg: "仅一级主账号可操作"})
			return
		}
		c.Next()
	}
}
