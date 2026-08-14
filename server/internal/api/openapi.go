package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"iot-platform/internal/model"
	"iot-platform/internal/repository"
)

// ---- 应用管理（管理后台） ----

func CreateOpenApp(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required,max=64"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, 400, "应用名称必填")
		return
	}
	app := model.OpenApp{
		UserID: UID(c), Name: req.Name,
		AppKey: "ak" + randHex(8), AppSecret: randHex(16), Enabled: true,
	}
	if err := repository.DB.Create(&app).Error; err != nil {
		Fail(c, 500, "创建失败")
		return
	}
	OK(c, app)
}

// @Summary      应用列表
// @Description  分页查询当前账号名下的开放平台应用
// @Tags         应用
// @Produce      json
// @Param        page query int false "页码"
// @Param        size query int false "每页数量"
// @Success      200  {object}  Resp
// @Failure      400  {object}  Resp
// @Router       /apps [get]
// @Security     BearerAuth
func ListOpenApps(c *gin.Context) {
	q := repository.DB.Model(&model.OpenApp{}).Scopes(ownedScope(c, ""))
	var total int64
	q.Count(&total)
	page, size := pageArgs(c)
	var list []model.OpenApp
	q.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&list)
	OK(c, PageData{Total: total, List: list})
}

func UpdateOpenApp(c *gin.Context) {
	var app model.OpenApp
	if err := repository.DB.Scopes(ownedScope(c, "")).Where("id = ?", c.Param("id")).First(&app).Error; err != nil {
		Fail(c, 404, "应用不存在")
		return
	}
	var req struct {
		Name    string `json:"name"`
		Enabled *bool  `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, 400, "参数错误")
		return
	}
	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	repository.DB.Model(&app).Updates(updates)
	OK(c, app)
}

func DeleteOpenApp(c *gin.Context) {
	res := repository.DB.Scopes(ownedScope(c, "")).Where("id = ?", c.Param("id")).Delete(&model.OpenApp{})
	if res.RowsAffected == 0 {
		Fail(c, 404, "应用不存在")
		return
	}
	OK(c, nil)
}

// ---- OpenAPI 签名鉴权 ----
//
// 请求头：
//   X-App-Key:   应用 AppKey
//   X-Timestamp: Unix 秒级时间戳（±5 分钟有效）
//   X-Nonce:     一次性随机串（8-64 字符，建议 UUID/hex；同一 AppKey 5 分钟内不可重复）
//   X-Sign:      hex(HMAC-SHA256(appSecret, 待签名串))
//
// 待签名串 = Method + "\n" + PathAndQuery + "\n" + hex(SHA256(Body)) + "\n" + AppKey + "\n" + Timestamp + "\n" + Nonce
// 覆盖 method/path/body/nonce 后：捕获的签名既不能改写为其他请求，也不能原样重放
//（nonce 经 Redis SETNX 去重，TTL 覆盖时间戳有效窗口）

// nonceTTL nonce 去重键存活时间（须 > 时间戳 ±5 分钟窗口，保证窗口内全程去重）
const nonceTTL = 6 * time.Minute

// openapiSignString 计算 OpenAPI 签名。
// 待签名串 = Method + "\n" + PathAndQuery + "\n" + hex(SHA256(Body)) + "\n" + AppKey + "\n" + Timestamp + "\n" + Nonce
// 提取为纯函数便于单元测试。
func openapiSignString(appSecret, method, uri string, body []byte, appKey, ts, nonce string) string {
	bodyHash := sha256.Sum256(body)
	mac := hmac.New(sha256.New, []byte(appSecret))
	mac.Write([]byte(method + "\n" + uri + "\n" + hex.EncodeToString(bodyHash[:]) + "\n" + appKey + "\n" + ts + "\n" + nonce))
	return hex.EncodeToString(mac.Sum(nil))
}

// nonce 内存去重（Redis 不可用时的单实例降级兜底）
var (
	nonceSeenMu sync.Mutex
	nonceSeen   = map[string]int64{} // key -> 过期时间（Unix 秒）
)

// nonceDuplicate true=重复（重放）。先查 Redis SETNX；Redis 异常时降级内存去重
func nonceDuplicate(appKey, nonce string) bool {
	key := "openapi:nonce:" + appKey + ":" + nonce
	if repository.RDB != nil {
		ok, err := repository.RDB.SetNX(context.Background(), key, "1", nonceTTL).Result()
		if err == nil {
			return !ok // 设置失败=已存在=重放
		}
		slog.Warn("openapi nonce redis error, degrade to in-memory dedup", "err", err)
	}
	// 内存降级：惰性清扫过期项，防止 map 无限增长
	now := time.Now().Unix()
	nonceSeenMu.Lock()
	defer nonceSeenMu.Unlock()
	if len(nonceSeen) > 65536 {
		for k, exp := range nonceSeen {
			if exp < now {
				delete(nonceSeen, k)
			}
		}
	}
	if exp, ok := nonceSeen[key]; ok && exp >= now {
		return true
	}
	nonceSeen[key] = now + int64(nonceTTL.Seconds())
	return false
}

func OpenAPIAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		appKey := c.GetHeader("X-App-Key")
		tsStr := c.GetHeader("X-Timestamp")
		nonce := c.GetHeader("X-Nonce")
		sign := c.GetHeader("X-Sign")
		if appKey == "" || tsStr == "" || sign == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, Resp{Code: 401, Msg: "缺少签名头 X-App-Key/X-Timestamp/X-Sign"})
			return
		}
		// nonce 必填：没有 nonce 的时间戳窗口内可原样重放（签名绑定 nonce 后缺失即拒绝）
		if len(nonce) < 8 || len(nonce) > 64 || strings.ContainsAny(nonce, " \t\r\n") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, Resp{Code: 401, Msg: "X-Nonce 必填（8-64 位随机串，且不可重复）"})
			return
		}
		ts, err := strconv.ParseInt(tsStr, 10, 64)
		if err != nil || abs64(time.Now().Unix()-ts) > 300 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, Resp{Code: 401, Msg: "时间戳无效或已过期"})
			return
		}
		var app model.OpenApp
		if err := repository.DB.Where("app_key = ? AND enabled = true", appKey).First(&app).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, Resp{Code: 401, Msg: "AppKey 无效或已禁用"})
			return
		}
		// 应用归属用户必须存在且未被禁用（禁用账号的 token/应用即时失效）
		var owner model.User
		if err := repository.DB.Select("id, role, parent_id, status").First(&owner, app.UserID).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, Resp{Code: 401, Msg: "应用归属账号不存在"})
			return
		}
		if owner.Status == model.AccountStatusDisabled {
			c.AbortWithStatusJSON(http.StatusUnauthorized, Resp{Code: 401, Msg: "应用归属账号已禁用"})
			return
		}
		// 读取并还原请求体，计算 body 哈希参与签名（上限 1MB，防恶意大 body 撑爆内存）
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, Resp{Code: 413, Msg: "请求体过大"})
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		expect := openapiSignString(app.AppSecret, c.Request.Method, c.Request.URL.RequestURI(), bodyBytes, appKey, tsStr, nonce)
		if !hmac.Equal([]byte(expect), []byte(sign)) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, Resp{Code: 401, Msg: "签名错误"})
			return
		}
		// nonce 去重在签名校验之后：否则攻击者可在验签前刷垃圾 nonce，阻断合法请求（DoS）
		if nonceDuplicate(appKey, nonce) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, Resp{Code: 401, Msg: "重复的请求（nonce 已使用）"})
			return
		}
		// 以应用归属用户身份访问，复用管理端处理器；
		// role/pid 以库为准，下游 RequireOperate/Tier 据此判定写权限（viewer 账号不能经 OpenAPI 写设备）
		c.Set("uid", owner.ID)
		c.Set("role", owner.Role)
		c.Set("pid", owner.ParentID)
		c.Next()
	}
}

func abs64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
