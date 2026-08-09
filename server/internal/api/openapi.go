package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strconv"
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
//   X-Sign:      hex(HMAC-SHA256(appSecret, 待签名串))
//
// 待签名串 = Method + "\n" + PathAndQuery + "\n" + hex(SHA256(Body)) + "\n" + AppKey + "\n" + Timestamp
// 覆盖 method/path/body 后，捕获的签名无法被改写为其他请求（防重放篡改）

// openapiSignString 计算 OpenAPI 签名。
// 待签名串 = Method + "\n" + PathAndQuery + "\n" + hex(SHA256(Body)) + "\n" + AppKey + "\n" + Timestamp
// 覆盖 method/path/body 后，捕获的签名无法被改写为其他请求（防重放篡改）。
// 提取为纯函数便于单元测试。
func openapiSignString(appSecret, method, uri string, body []byte, appKey, ts string) string {
	bodyHash := sha256.Sum256(body)
	mac := hmac.New(sha256.New, []byte(appSecret))
	mac.Write([]byte(method + "\n" + uri + "\n" + hex.EncodeToString(bodyHash[:]) + "\n" + appKey + "\n" + ts))
	return hex.EncodeToString(mac.Sum(nil))
}

func OpenAPIAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		appKey := c.GetHeader("X-App-Key")
		tsStr := c.GetHeader("X-Timestamp")
		sign := c.GetHeader("X-Sign")
		if appKey == "" || tsStr == "" || sign == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, Resp{Code: 401, Msg: "缺少签名头 X-App-Key/X-Timestamp/X-Sign"})
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
		// 读取并还原请求体，计算 body 哈希参与签名
		bodyBytes, _ := io.ReadAll(c.Request.Body)
		c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		expect := openapiSignString(app.AppSecret, c.Request.Method, c.Request.URL.RequestURI(), bodyBytes, appKey, tsStr)
		if !hmac.Equal([]byte(expect), []byte(sign)) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, Resp{Code: 401, Msg: "签名错误"})
			return
		}
		// 以应用归属用户身份访问，复用管理端处理器
		c.Set("uid", app.UserID)
		c.Next()
	}
}

func abs64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
