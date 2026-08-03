package api

import (
	"errors"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"iot-platform/internal/config"
	"iot-platform/internal/model"
	"iot-platform/internal/repository"
	"iot-platform/internal/service"
)

type authReq struct {
	Username string `json:"username" binding:"required,min=3,max=32"`
	Password string `json:"password" binding:"required,min=6,max=64"`
	Nickname string `json:"nickname"`
}

func Register(c *gin.Context) {
	var req authReq
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
	hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	user := model.User{Username: req.Username, PasswordHash: string(hash), Nickname: req.Nickname, Role: "user"}
	if user.Nickname == "" {
		user.Nickname = req.Username
	}
	if err := repository.DB.Create(&user).Error; err != nil {
		Fail(c, 500, "注册失败")
		return
	}
	OK(c, gin.H{"id": user.ID})
}

func Login(c *gin.Context) {
	var req authReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, 400, "参数错误")
		return
	}
	var user model.User
	err := repository.DB.Where("username = ?", req.Username).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) ||
		(err == nil && bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil) {
		Fail(c, 400, "用户名或密码错误")
		return
	}
	if err != nil {
		Fail(c, 500, "登录失败")
		return
	}
	token, err := GenToken(user.ID, user.Username, user.Role)
	if err != nil {
		Fail(c, 500, "生成令牌失败")
		return
	}
	OK(c, gin.H{"token": token, "user": user})
}

func Profile(c *gin.Context) {
	var user model.User
	if err := repository.DB.First(&user, UID(c)).Error; err != nil {
		Fail(c, 404, "用户不存在")
		return
	}
	OK(c, user)
}

// ChangePassword 修改当前用户登录密码
// POST /api/v1/auth/change-password  Body: {"oldPassword":"...","newPassword":"..."}
func ChangePassword(c *gin.Context) {
	var req struct {
		OldPassword string `json:"oldPassword" binding:"required"`
		NewPassword string `json:"newPassword" binding:"required,min=6,max=64"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, 400, "原密码必填，新密码至少6位")
		return
	}
	var user model.User
	if err := repository.DB.First(&user, UID(c)).Error; err != nil {
		Fail(c, 404, "用户不存在")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.OldPassword)) != nil {
		Fail(c, 400, "原密码错误")
		return
	}
	if req.NewPassword == req.OldPassword {
		Fail(c, 400, "新密码不能与原密码相同")
		return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err := repository.DB.Model(&user).Update("password_hash", string(hash)).Error; err != nil {
		Fail(c, 500, "修改失败")
		return
	}
	OK(c, nil)
}

// EnsureAdmin 初始化默认管理员；首次无 admin 用户时生效一次。
// 密码取 config.C.AdminPassword，为空则回退默认 admin123。
func EnsureAdmin() {
	var cnt int64
	repository.DB.Model(&model.User{}).Where("username = ?", "admin").Count(&cnt)
	if cnt == 0 {
		pwd := config.C.AdminPassword
		if pwd == "" {
			pwd = "admin123"
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.DefaultCost)
		repository.DB.Create(&model.User{Username: "admin", PasswordHash: string(hash), Nickname: "管理员", Role: "admin"})
	}
}

// DeviceToken 设备动态获取 MQTT 连接 Token
// POST /api/v1/auth/token  Body: {"productKey":"...","deviceName":"...","secret":"..."}
func DeviceToken(c *gin.Context) {
	var req struct {
		ProductKey string `json:"productKey" binding:"required"`
		DeviceName string `json:"deviceName" binding:"required"`
		Secret     string `json:"secret" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(200, gin.H{"code": 1, "msg": "参数错误"})
		return
	}
	token, ttl, err := service.GenerateDeviceToken(req.ProductKey, req.DeviceName, req.Secret)
	if err != nil {
		c.JSON(200, gin.H{"code": 1, "msg": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": gin.H{"token": token, "ttl": ttl}})
}
