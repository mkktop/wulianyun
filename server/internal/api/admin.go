package api

import (
	"context"
	"log/slog"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"iot-platform/internal/config"
	"iot-platform/internal/gateway"
	"iot-platform/internal/model"
	"iot-platform/internal/mqtt"
	"iot-platform/internal/repository"
	"iot-platform/internal/storage"
)

// 本文件实现平台超管（admin 角色）专属后台接口：系统状态 / 配置查看 / 热更新参数 /
// 公告管理 / 帮助中心管理 / 全量用户管理。路由统一挂在 AdminAuth 组（router.go）。

// Version 服务版本（build.sh 用 -ldflags "-X iot-platform/internal/api.Version=..." 注入）
var Version = "dev"

// startTime 进程启动时间（系统状态页显示运行时长）
var startTime = time.Now()

// ---- 系统状态 ----

// SystemStatus 系统运行状态（超管视角，全局统计不过滤账号）
// @Summary      系统运行状态
// @Description  服务进程信息、各组件健康、全局统计（超管专属）
// @Tags         系统管理
// @Produce      json
// @Success      200  {object}  Resp
// @Router       /admin/system/status [get]
// @Security     BearerAuth
func SystemStatus(c *gin.Context) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	dbStatus, dbErr := "ok", ""
	if err := repository.DB.Exec("SELECT 1").Error; err != nil {
		dbStatus, dbErr = "error", err.Error()
	}

	redisStatus, redisErr := "ok", ""
	if repository.RDB == nil {
		redisStatus, redisErr = "error", "redis 未初始化"
	} else if _, err := repository.RDB.Ping(context.Background()).Result(); err != nil {
		redisStatus, redisErr = "error", err.Error()
	}

	mqttConnected := false
	if cli := mqtt.Client(); cli != nil {
		mqttConnected = cli.IsConnected()
	}

	var userCount, productCount, deviceCount, onlineCount, msgToday int64
	repository.DB.Model(&model.User{}).Count(&userCount)
	repository.DB.Model(&model.Product{}).Count(&productCount)
	repository.DB.Model(&model.Device{}).Count(&deviceCount)
	repository.DB.Model(&model.Device{}).Where("status = ?", model.DeviceStatusOnline).Count(&onlineCount)
	repository.DB.Model(&model.Telemetry{}).Where("ts >= ?", time.Now().Truncate(24*time.Hour)).Count(&msgToday)

	OK(c, gin.H{
		"version":       Version,
		"uptimeSeconds": int64(time.Since(startTime).Seconds()),
		"goVersion":     runtime.Version(),
		"goroutines":    runtime.NumGoroutine(),
		"memAllocMB":    mem.Alloc >> 20,
		"memSysMB":      mem.Sys >> 20,
		"db":            gin.H{"status": dbStatus, "error": dbErr},
		"redis":         gin.H{"status": redisStatus, "error": redisErr},
		"mqtt":          gin.H{"connected": mqttConnected},
		"gateway":       gin.H{"tcpConnections": gateway.ConnCount()},
		"emqxRule":      config.C.EMQXRule.Enabled,
		"counts": gin.H{
			"users": userCount, "products": productCount,
			"devices": deviceCount, "online": onlineCount, "msgToday": msgToday,
		},
	})
}

// ---- 配置查看（只读，敏感项打码） ----

var dsnPwdRe = regexp.MustCompile(`password=\S+`)

// maskSecret 敏感值打码：保留前 4 位，其余替换
func maskSecret(s string) string {
	if len(s) <= 4 {
		return "****"
	}
	return s[:4] + "****"
}

// SystemConfig 当前生效配置（只读展示；基础设施参数需改 config.yaml 重启生效）
// @Summary      系统配置
// @Description  返回当前生效配置（敏感字段打码），仅超管可读
// @Tags         系统管理
// @Produce      json
// @Success      200  {object}  Resp
// @Router       /admin/system/config [get]
// @Security     BearerAuth
func SystemConfig(c *gin.Context) {
	cfg := config.C
	OK(c, gin.H{
		"server": gin.H{"addr": cfg.Server.Addr},
		"gateway": gin.H{
			"addr": cfg.Gateway.Addr, "idle_timeout": cfg.Gateway.IdleTimeout,
			"max_conns_per_ip": cfg.Gateway.MaxConnsPerIP,
			"conn_rate_limit":  cfg.Gateway.ConnRateLimit, "conn_rate_burst": cfg.Gateway.ConnRateBurst,
			"tls_enabled": cfg.Gateway.TLS.Enabled,
		},
		"jwt": gin.H{"expire_hours": cfg.JWT.ExpireHours, "secret": maskSecret(cfg.JWT.Secret)},
		"database": gin.H{
			"dsn": dsnPwdRe.ReplaceAllString(cfg.Database.DSN, "password=****"),
			"max_open_conns": cfg.Database.MaxOpenConns, "max_idle_conns": cfg.Database.MaxIdleConns,
			"conn_max_lifetime": cfg.Database.ConnMaxLifetime, "conn_max_idle_time": cfg.Database.ConnMaxIdleTime,
			"retention_days": cfg.Database.RetentionDays, "compress_after_days": cfg.Database.CompressAfterDays,
		},
		"redis": gin.H{
			"addr": cfg.Redis.Addr, "db": cfg.Redis.DB, "password": maskSecret(cfg.Redis.Password),
		},
		"mqtt": gin.H{
			"broker": cfg.MQTT.Broker, "client_id": cfg.MQTT.ClientID, "username": cfg.MQTT.Username,
			"password": maskSecret(cfg.MQTT.Password), "tls_enabled": cfg.MQTT.TLS.Enabled,
		},
		"telemetry_buffer": gin.H{"max_batch": cfg.TelemetryBuffer.MaxBatch, "flush_interval": cfg.TelemetryBuffer.FlushInterval},
		"cache":            gin.H{"device_ttl": cfg.Cache.DeviceTTL, "shadow_flush_interval": cfg.Cache.ShadowFlushInterval},
		"poller":           gin.H{"max_concurrent": cfg.Poller.MaxConcurrent},
		"log": gin.H{
			"trace_retention_days": cfg.Log.TraceRetentionDays,
			"device_log_retention_days": cfg.Log.DeviceLogRetentionDays,
		},
		"emqx_rule":        gin.H{"enabled": cfg.EMQXRule.Enabled},
		"admin_password_set": cfg.AdminPassword != "",
		"storage": gin.H{
			"type": cfg.Storage.Type, "local_dir": cfg.Storage.LocalDir,
			"endpoint": cfg.Storage.Endpoint, "region": cfg.Storage.Region, "bucket": cfg.Storage.Bucket,
			"access_key": maskSecret(cfg.Storage.AccessKey), "secret_key": maskSecret(cfg.Storage.SecretKey),
			"use_ssl": cfg.Storage.UseSSL, "public_domain": cfg.Storage.PublicDomain,
		},
	})
}

// ---- 对象存储配置（可热重载）----

// StorageView 返回给前端的 storage 配置（敏感字段打码）
type StorageView struct {
	Type         string `json:"type"`
	LocalDir     string `json:"localDir"`
	Endpoint     string `json:"endpoint"`
	Region       string `json:"region"`
	Bucket       string `json:"bucket"`
	AccessKey    string `json:"accessKey"` // 打码
	HasSecretKey bool   `json:"hasSecretKey"`
	UseSSL       bool   `json:"useSSL"`
	PublicDomain string `json:"publicDomain"`
	UpdatedAt    string `json:"updatedAt"`
}

// loadStorageConfig 读取生效的对象存储配置：优先 DB（超管后台改过），回退 config.yaml
func loadStorageConfig() model.StorageConfig {
	var sc model.StorageConfig
	if err := repository.DB.First(&sc, 1).Error; err == nil {
		return sc
	}
	// DB 无记录，用 config.yaml 初始化默认行
	sc = model.StorageConfig{
		ID: 1, Type: config.C.Storage.Type, LocalDir: config.C.Storage.LocalDir,
		Endpoint: config.C.Storage.Endpoint, Region: config.C.Storage.Region,
		Bucket: config.C.Storage.Bucket, AccessKey: config.C.Storage.AccessKey,
		SecretKey: config.C.Storage.SecretKey, UseSSL: config.C.Storage.UseSSL,
		PublicDomain: config.C.Storage.PublicDomain,
	}
	if sc.Type == "" {
		sc.Type = "local"
	}
	if sc.LocalDir == "" {
		sc.LocalDir = "uploads"
	}
	return sc
}

// GetStorageConfig 读取对象存储配置（敏感字段打码）
// @Summary      对象存储配置
// @Tags         系统管理
// @Router       /admin/system/storage [get]
// @Security     BearerAuth
func GetStorageConfig(c *gin.Context) {
	sc := loadStorageConfig()
	OK(c, StorageView{
		Type: sc.Type, LocalDir: sc.LocalDir, Endpoint: sc.Endpoint, Region: sc.Region,
		Bucket: sc.Bucket, AccessKey: maskSecret(sc.AccessKey),
		HasSecretKey: sc.SecretKey != "", UseSSL: sc.UseSSL, PublicDomain: sc.PublicDomain,
		UpdatedAt: formatTime(sc.UpdatedAt),
	})
}

// storageUpdateReq 前端提交的配置（SecretKey 为空表示不修改）
type storageUpdateReq struct {
	Type         string `json:"type"`
	LocalDir     string `json:"localDir"`
	Endpoint     string `json:"endpoint"`
	Region       string `json:"region"`
	Bucket       string `json:"bucket"`
	AccessKey    string `json:"accessKey"`
	SecretKey    string `json:"secretKey"`
	UseSSL       bool   `json:"useSSL"`
	PublicDomain string `json:"publicDomain"`
}

// UpdateStorageConfig 更新对象存储配置并热重载（新上传固件走新配置，已存固件 URL 不变）
// @Summary      更新对象存储配置
// @Tags         系统管理
// @Router       /admin/system/storage [put]
// @Security     BearerAuth
func UpdateStorageConfig(c *gin.Context) {
	var req storageUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, 400, "参数错误")
		return
	}
	if req.Type != "local" && req.Type != "s3" {
		Fail(c, 400, "类型仅支持 local / s3")
		return
	}

	// 读当前配置作为基底（DB 有则用 DB，否则用 yaml 默认行）
	sc := loadStorageConfig()
	sc.ID = 1
	sc.Type = req.Type
	sc.LocalDir = req.LocalDir
	sc.Endpoint = req.Endpoint
	sc.Region = req.Region
	sc.Bucket = req.Bucket
	sc.AccessKey = req.AccessKey
	// SecretKey 为空表示不改（避免前端回显打码值覆盖真实密钥）
	if req.SecretKey != "" {
		sc.SecretKey = req.SecretKey
	}
	sc.UseSSL = req.UseSSL
	sc.PublicDomain = req.PublicDomain
	sc.UpdatedBy = UID(c)

	// s3 模式校验必填项
	if sc.Type == "s3" {
		if sc.Endpoint == "" || sc.Bucket == "" || sc.AccessKey == "" || sc.SecretKey == "" {
			Fail(c, 400, "s3 模式需填写端点、桶名、AccessKey、SecretKey")
			return
		}
	}

	// 先热重载 storage（验证新配置可用，失败则不落库）
	if err := storage.Reinit(storage.Options{
		Type: sc.Type, LocalDir: sc.LocalDir, Endpoint: sc.Endpoint, Region: sc.Region,
		Bucket: sc.Bucket, AccessKey: sc.AccessKey, SecretKey: sc.SecretKey,
		UseSSL: sc.UseSSL, PublicDomain: sc.PublicDomain,
	}); err != nil {
		slog.Error("reinit storage failed", "type", sc.Type, "err", err)
		Fail(c, 500, "存储配置校验失败："+err.Error())
		return
	}

	// 落库（upsert 单行）
	if err := repository.DB.Save(&sc).Error; err != nil {
		slog.Error("save storage config failed", "err", err)
		Fail(c, 500, "保存失败")
		return
	}
	slog.Info("storage config updated and reloaded", "type", sc.Type, "bucket", sc.Bucket, "by", UID(c))
	OK(c, nil)
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

// ---- 热更新参数 ----

// ListSystemSettings 热更新参数列表（含默认值/说明，value 空表示回退 config.yaml）
// @Summary      热更新参数列表
// @Tags         系统管理
// @Router       /admin/system/settings [get]
// @Security     BearerAuth
func ListSystemSettings(c *gin.Context) {
	stored := make(map[string]model.SystemSetting)
	for _, s := range repository.ListSettings() {
		stored[s.Key] = s
	}
	type item struct {
		Key         string `json:"key"`
		Value       string `json:"value"`
		Type        string `json:"type"`
		Description string `json:"description"`
		UpdatedAt   time.Time `json:"updatedAt"`
	}
	// 按 key 排序保证展示顺序稳定（map 遍历无序）
	keys := make([]string, 0, len(repository.SettingKeys))
	for key := range repository.SettingKeys {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]item, 0, len(keys))
	for _, key := range keys {
		meta := repository.SettingKeys[key]
		it := item{Key: key, Type: meta.Type, Description: meta.Description}
		if s, ok := stored[key]; ok {
			it.Value, it.UpdatedAt = s.Value, s.UpdatedAt
		}
		out = append(out, it)
	}
	OK(c, out)
}

// UpdateSystemSetting 更新热更新参数（key 白名单 + 类型校验，立即生效）
// @Summary      更新热更新参数
// @Tags         系统管理
// @Router       /admin/system/settings [put]
// @Security     BearerAuth
func UpdateSystemSetting(c *gin.Context) {
	var req struct {
		Key   string `json:"key" binding:"required"`
		Value string `json:"value"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, 400, "参数错误")
		return
	}
	meta, ok := repository.SettingKeys[req.Key]
	if !ok {
		Fail(c, 400, "参数不存在或不可修改")
		return
	}
	// 类型校验：空值表示恢复默认（回退 yaml）；非空需能解析
	if req.Value != "" {
		switch meta.Type {
		case "bool":
			if _, err := strconv.ParseBool(req.Value); err != nil {
				Fail(c, 400, "布尔参数仅支持 true/false")
				return
			}
		case "int":
			if n, err := strconv.Atoi(req.Value); err != nil || n <= 0 {
				Fail(c, 400, "整数参数需为正整数")
				return
			}
		}
	}
	if err := repository.SaveSetting(req.Key, req.Value, UID(c)); err != nil {
		slog.Error("save setting failed", "key", req.Key, "err", err)
		Fail(c, 500, "保存失败")
		return
	}
	slog.Info("system setting updated", "key", req.Key, "value", req.Value, "by", UID(c))
	OK(c, nil)
}

// ---- 公告管理（超管） ----

type announcementReq struct {
	Title   string `json:"title" binding:"required,max=128"`
	Content string `json:"content"`
	Level   string `json:"level"`
	Status  string `json:"status"` // draft 草稿 / published 发布
}

// ListAdminAnnouncements 公告管理列表（含草稿）
func ListAdminAnnouncements(c *gin.Context) {
	var list []model.Announcement
	repository.DB.Order("id desc").Find(&list)
	// 关联发布者
	names := make(map[uint]string)
	for i := range list {
		if _, ok := names[list[i].UserID]; !ok {
			var u model.User
			repository.DB.Select("username").First(&u, list[i].UserID)
			names[list[i].UserID] = u.Username
		}
	}
	type item struct {
		model.Announcement
		Publisher string `json:"publisher"`
	}
	out := make([]item, 0, len(list))
	for _, a := range list {
		out = append(out, item{Announcement: a, Publisher: names[a.UserID]})
	}
	OK(c, out)
}

// CreateAnnouncement 新建公告（默认草稿）
func CreateAnnouncement(c *gin.Context) {
	var req announcementReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, 400, "标题必填（≤128字）")
		return
	}
	if req.Level != model.AnnouncementLevelImportant {
		req.Level = model.AnnouncementLevelNormal
	}
	status := req.Status
	if status != model.AnnouncementStatusPublished {
		status = model.AnnouncementStatusDraft
	}
	a := model.Announcement{
		UserID: UID(c), Title: req.Title, Content: req.Content,
		Level: req.Level, Status: status,
	}
	if status == model.AnnouncementStatusPublished {
		now := time.Now()
		a.PublishAt = &now
	}
	if err := repository.DB.Create(&a).Error; err != nil {
		Fail(c, 500, "创建失败")
		return
	}
	OK(c, a)
}

// UpdateAnnouncement 修改公告 / 发布 / 下线（发布置 PublishAt=now）
func UpdateAnnouncement(c *gin.Context) {
	var a model.Announcement
	if err := repository.DB.First(&a, c.Param("id")).Error; err != nil {
		Fail(c, 404, "公告不存在")
		return
	}
	var req announcementReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, 400, "参数错误")
		return
	}
	updates := map[string]interface{}{
		"title": req.Title, "content": req.Content,
		"level": req.Level, "status": req.Status,
	}
	if req.Level != model.AnnouncementLevelImportant {
		updates["level"] = model.AnnouncementLevelNormal
	}
	// 发布（草稿→published）时记录发布时间
	if req.Status == model.AnnouncementStatusPublished {
		if a.PublishAt == nil {
			now := time.Now()
			updates["publish_at"] = now
		}
	}
	repository.DB.Model(&a).Updates(updates)
	OK(c, a)
}

// DeleteAnnouncement 删除公告
func DeleteAnnouncement(c *gin.Context) {
	res := repository.DB.Delete(&model.Announcement{}, c.Param("id"))
	if res.RowsAffected == 0 {
		Fail(c, 404, "公告不存在")
		return
	}
	OK(c, nil)
}

// ListPublishedAnnouncements 用户侧：已发布公告列表（控制台铃铛/概览页）
func ListPublishedAnnouncements(c *gin.Context) {
	var list []model.Announcement
	repository.DB.Where("status = ?", model.AnnouncementStatusPublished).
		Order("publish_at desc").Find(&list)
	names := make(map[uint]string)
	for i := range list {
		if _, ok := names[list[i].UserID]; !ok {
			var u model.User
			repository.DB.Select("username").First(&u, list[i].UserID)
			names[list[i].UserID] = u.Username
		}
	}
	type item struct {
		model.Announcement
		Publisher string `json:"publisher"`
	}
	out := make([]item, 0, len(list))
	for _, a := range list {
		out = append(out, item{Announcement: a, Publisher: names[a.UserID]})
	}
	OK(c, out)
}

// ---- 帮助中心管理（超管） ----

type helpDocReq struct {
	Key     string `json:"key" binding:"required,max=64"`
	Title   string `json:"title" binding:"required,max=128"`
	Content string `json:"content"`
}

// ListAdminHelpDocs 帮助文档列表
func ListAdminHelpDocs(c *gin.Context) {
	var list []model.HelpDoc
	repository.DB.Order("id asc").Find(&list)
	OK(c, list)
}

// CreateHelpDoc 新建帮助文档
func CreateHelpDoc(c *gin.Context) {
	var req helpDocReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, 400, "key 与标题必填")
		return
	}
	var cnt int64
	repository.DB.Model(&model.HelpDoc{}).Where("key = ?", req.Key).Count(&cnt)
	if cnt > 0 {
		Fail(c, 400, "key 已存在（应为英文标识）")
		return
	}
	doc := model.HelpDoc{Key: req.Key, Title: req.Title, Content: req.Content, UpdatedBy: UID(c)}
	if err := repository.DB.Create(&doc).Error; err != nil {
		Fail(c, 500, "创建失败")
		return
	}
	OK(c, doc)
}

// UpdateHelpDoc 更新帮助文档
func UpdateHelpDoc(c *gin.Context) {
	var doc model.HelpDoc
	if err := repository.DB.First(&doc, c.Param("id")).Error; err != nil {
		Fail(c, 404, "文档不存在")
		return
	}
	var req helpDocReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, 400, "参数错误")
		return
	}
	updates := map[string]interface{}{
		"key": req.Key, "title": req.Title, "content": req.Content, "updated_by": UID(c),
	}
	repository.DB.Model(&doc).Updates(updates)
	OK(c, doc)
}

// DeleteHelpDoc 删除帮助文档
func DeleteHelpDoc(c *gin.Context) {
	res := repository.DB.Delete(&model.HelpDoc{}, c.Param("id"))
	if res.RowsAffected == 0 {
		Fail(c, 404, "文档不存在")
		return
	}
	OK(c, nil)
}

// ListHelpDocs 用户侧：帮助文档列表
func ListHelpDocs(c *gin.Context) {
	var list []model.HelpDoc
	repository.DB.Select("id, key, title, updated_at").Order("id asc").Find(&list)
	OK(c, list)
}

// GetHelpDoc 用户侧：按 key 取帮助文档内容
func GetHelpDoc(c *gin.Context) {
	var doc model.HelpDoc
	if err := repository.DB.Where("key = ?", c.Param("key")).First(&doc).Error; err != nil {
		Fail(c, 404, "文档不存在")
		return
	}
	OK(c, doc)
}

// ---- 全量用户管理（超管） ----

type adminUserCreateReq struct {
	Username   string `json:"username" binding:"required,min=3,max=32"`
	Password   string `json:"password" binding:"required,min=6,max=64"`
	Nickname   string `json:"nickname"`
	Role       string `json:"role"`       // user / admin（超管），默认 user
	Permission string `json:"permission"` // operate / view，默认 operate
}

// ListAdminUsers 全量用户列表（超管视角，含一级/二级/超管）
func ListAdminUsers(c *gin.Context) {
	q := repository.DB.Model(&model.User{})
	if kw := c.Query("keyword"); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("username ILIKE ? OR nickname ILIKE ?", like, like)
	}
	if role := c.Query("role"); role != "" {
		q = q.Where("role = ?", role)
	}
	if status := c.Query("status"); status != "" {
		q = q.Where("status = ?", status)
	}
	var total int64
	q.Count(&total)
	page, size := pageArgs(c)
	var users []model.User
	q.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&users)

	type userItem struct {
		model.User
		Tier        string `json:"tier"`
		DeviceCount int64  `json:"deviceCount"`
		ParentName  string `json:"parentName"`
	}
	out := make([]userItem, 0, len(users))
	parentNames := make(map[uint]string)
	for _, u := range users {
		it := userItem{User: u, Tier: "primary"}
		switch {
		case u.Role == "admin":
			it.Tier = "platform"
		case u.ParentID != nil:
			it.Tier = "secondary"
		}
		var dc int64
		repository.DB.Model(&model.Device{}).Where("user_id = ?", u.ID).Count(&dc)
		it.DeviceCount = dc
		if u.ParentID != nil {
			name, ok := parentNames[*u.ParentID]
			if !ok {
				var p model.User
				repository.DB.Select("username").First(&p, *u.ParentID)
				name = p.Username
				parentNames[*u.ParentID] = name
			}
			it.ParentName = name
		}
		out = append(out, it)
	}
	OK(c, PageData{Total: total, List: out})
}

// CreateAdminUser 超管创建账号（一级账号或超管；可指定归属父账号）
func CreateAdminUser(c *gin.Context) {
	var req adminUserCreateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, 400, "用户名至少3位，密码至少6位")
		return
	}
	var cnt int64
	repository.DB.Model(&model.User{}).Where("username = ?", req.Username).Count(&cnt)
	if cnt > 0 {
		Fail(c, 400, "用户名已存在")
		return
	}
	role := req.Role
	if role != "admin" {
		role = "user"
	}
	perm := req.Permission
	if perm != model.AccountPermissionView {
		perm = model.AccountPermissionOperate
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	u := model.User{
		Username: req.Username, PasswordHash: string(hash),
		Nickname: req.Nickname, Role: role, Status: model.AccountStatusActive,
		Permission: perm,
	}
	if u.Nickname == "" {
		u.Nickname = req.Username
	}
	if err := repository.DB.Create(&u).Error; err != nil {
		Fail(c, 500, "创建失败")
		return
	}
	OK(c, u)
}

type adminUserUpdateReq struct {
	Nickname   *string `json:"nickname"`
	Password   *string `json:"password"` // 重置密码
	Role       *string `json:"role"`     // user / admin
	Status     *string `json:"status"`   // active / disabled
	Permission *string `json:"permission"`
}

// UpdateAdminUser 超管修改用户（不能禁用/降级自己，防止把自己锁在后台外）
func UpdateAdminUser(c *gin.Context) {
	var u model.User
	if err := repository.DB.First(&u, c.Param("id")).Error; err != nil {
		Fail(c, 404, "用户不存在")
		return
	}
	var req adminUserUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, 400, "参数错误")
		return
	}
	self := u.ID == UID(c)
	updates := map[string]interface{}{}
	if req.Nickname != nil {
		updates["nickname"] = *req.Nickname
	}
	if req.Password != nil {
		if len(*req.Password) < 6 || len(*req.Password) > 64 {
			Fail(c, 400, "密码长度 6-64 位")
			return
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
		updates["password_hash"] = string(hash)
	}
	if req.Role != nil {
		if *req.Role != "admin" && *req.Role != "user" {
			Fail(c, 400, "角色非法")
			return
		}
		if self && *req.Role != "admin" {
			Fail(c, 400, "不能将自己的超管角色降级")
			return
		}
		updates["role"] = *req.Role
	}
	if req.Status != nil {
		if *req.Status != model.AccountStatusActive && *req.Status != model.AccountStatusDisabled {
			Fail(c, 400, "状态非法")
			return
		}
		if self && *req.Status == model.AccountStatusDisabled {
			Fail(c, 400, "不能禁用自己的账号")
			return
		}
		updates["status"] = *req.Status
	}
	if req.Permission != nil {
		if *req.Permission != model.AccountPermissionOperate && *req.Permission != model.AccountPermissionView {
			Fail(c, 400, "权限非法")
			return
		}
		updates["permission"] = *req.Permission
	}
	if len(updates) > 0 {
		repository.DB.Model(&u).Updates(updates)
	}
	OK(c, u)
}

// DeleteAdminUser 超管删除用户（不能删自己；名下还有设备/子账号时拒绝）
func DeleteAdminUser(c *gin.Context) {
	var u model.User
	if err := repository.DB.First(&u, c.Param("id")).Error; err != nil {
		Fail(c, 404, "用户不存在")
		return
	}
	if u.ID == UID(c) {
		Fail(c, 400, "不能删除自己的账号")
		return
	}
	var dc, cc int64
	repository.DB.Model(&model.Device{}).Where("user_id = ?", u.ID).Count(&dc)
	repository.DB.Model(&model.User{}).Where("parent_id = ?", u.ID).Count(&cc)
	if dc > 0 {
		Fail(c, 400, "该账号下还有设备，请先迁移或删除设备")
		return
	}
	if cc > 0 {
		Fail(c, 400, "该账号下还有子账号，请先处理子账号")
		return
	}
	repository.DB.Where("secondary_id = ?", u.ID).Delete(&model.ProductGrant{})
	repository.DB.Where("granted_by = ?", u.ID).Delete(&model.ProductGrant{})
	repository.DB.Delete(&u)
	OK(c, nil)
}
