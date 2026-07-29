package api

import (
	"errors"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"iot-platform/internal/model"
	"iot-platform/internal/repository"
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

// EnsureAdmin 初始化默认管理员 admin/admin123
func EnsureAdmin() {
	var cnt int64
	repository.DB.Model(&model.User{}).Where("username = ?", "admin").Count(&cnt)
	if cnt == 0 {
		hash, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
		repository.DB.Create(&model.User{Username: "admin", PasswordHash: string(hash), Nickname: "管理员", Role: "admin"})
	}
}
