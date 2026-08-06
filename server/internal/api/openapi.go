package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
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
//   X-Sign:      hex(HMAC-SHA256(appSecret, appKey + timestamp))

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
		mac := hmac.New(sha256.New, []byte(app.AppSecret))
		mac.Write([]byte(appKey + tsStr))
		expect := hex.EncodeToString(mac.Sum(nil))
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
