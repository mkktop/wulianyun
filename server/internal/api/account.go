package api

import (
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"iot-platform/internal/model"
	"iot-platform/internal/repository"
)

// 本文件实现一级账号对其名下二级账号的管理，以及产品下放（授权给二级使用）。
//   - 账号管理（/accounts）仅一级主账号可访问（router 中用 PrimaryAuth 保护）
//   - 产品下放（/products/:id/grants）仅产品 owner/超管可操作（mustOwnProduct）

type accountCreateReq struct {
	Username   string `json:"username" binding:"required,min=3,max=32"`
	Password   string `json:"password" binding:"required,min=6,max=64"`
	Nickname   string `json:"nickname"`
	Permission string `json:"permission"` // operate(可操作) / view(只读)，默认 operate
}

// ListAccounts 一级列出自己名下的二级账号（附设备数/下放产品数）
func ListAccounts(c *gin.Context) {
	var list []model.User
	repository.DB.Where("parent_id = ?", UID(c)).Order("id asc").Find(&list)
	type accountWithStats struct {
		model.User
		DeviceCount int64 `json:"deviceCount"`
		GrantCount  int64 `json:"grantCount"`
	}
	out := make([]accountWithStats, 0, len(list))
	for _, u := range list {
		var dc, gc int64
		repository.DB.Model(&model.Device{}).Where("user_id = ?", u.ID).Count(&dc)
		repository.DB.Model(&model.ProductGrant{}).Where("secondary_id = ?", u.ID).Count(&gc)
		out = append(out, accountWithStats{User: u, DeviceCount: dc, GrantCount: gc})
	}
	OK(c, out)
}

// CreateAccount 一级创建二级账号
func CreateAccount(c *gin.Context) {
	var req accountCreateReq
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
	perm := req.Permission
	if perm != model.AccountPermissionView {
		perm = model.AccountPermissionOperate
	}
	u := model.User{
		Username: req.Username, PasswordHash: string(hash),
		Nickname: req.Nickname, Role: "user",
		ParentID: uintPtr(UID(c)), Status: model.AccountStatusActive,
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

type accountUpdateReq struct {
	Nickname   *string `json:"nickname"`
	Password   *string `json:"password"`   // 重置密码
	Status     *string `json:"status"`     // active / disabled
	Permission *string `json:"permission"` // operate / view
}

// UpdateAccount 一级修改二级账号（昵称/重置密码/禁用启用）
func UpdateAccount(c *gin.Context) {
	var u model.User
	if err := repository.DB.Where("id = ? AND parent_id = ?", c.Param("id"), UID(c)).First(&u).Error; err != nil {
		Fail(c, 404, "子账号不存在")
		return
	}
	var req accountUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, 400, "参数错误")
		return
	}
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
	if req.Status != nil {
		if *req.Status != model.AccountStatusActive && *req.Status != model.AccountStatusDisabled {
			Fail(c, 400, "状态非法")
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
	repository.DB.Model(&u).Updates(updates)
	OK(c, u)
}

// DeleteAccount 一级删除二级账号（仅当其名下无设备时）
func DeleteAccount(c *gin.Context) {
	var u model.User
	if err := repository.DB.Where("id = ? AND parent_id = ?", c.Param("id"), UID(c)).First(&u).Error; err != nil {
		Fail(c, 404, "子账号不存在")
		return
	}
	var dc int64
	repository.DB.Model(&model.Device{}).Where("user_id = ?", u.ID).Count(&dc)
	if dc > 0 {
		Fail(c, 400, "该子账号下还有设备，请先迁移或删除设备")
		return
	}
	repository.DB.Where("secondary_id = ?", u.ID).Delete(&model.ProductGrant{}) // 清理下放授权
	repository.DB.Delete(&u)
	OK(c, nil)
}

// ---- 产品下放 ----

// ListGrants 查看产品已下放给哪些二级账号
func ListGrants(c *gin.Context) {
	p, err := mustOwnProduct(c, c.Param("id"))
	if err != nil {
		Fail(c, 404, "产品不存在")
		return
	}
	var grants []model.ProductGrant
	repository.DB.Where("product_id = ?", p.ID).Order("id asc").Find(&grants)
	type grantWithUser struct {
		model.ProductGrant
		SecondaryName string `json:"secondaryName"`
		Nickname      string `json:"nickname"`
	}
	out := make([]grantWithUser, 0, len(grants))
	for _, g := range grants {
		var u model.User
		repository.DB.Select("username, nickname").First(&u, g.SecondaryID)
		out = append(out, grantWithUser{ProductGrant: g, SecondaryName: u.Username, Nickname: u.Nickname})
	}
	OK(c, out)
}

// CreateGrant 把产品下放给一个二级账号
func CreateGrant(c *gin.Context) {
	p, err := mustOwnProduct(c, c.Param("id"))
	if err != nil {
		Fail(c, 404, "产品不存在")
		return
	}
	var req struct {
		SecondaryID uint `json:"secondaryId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, 400, "secondaryId 必填")
		return
	}
	// 校验该二级确属当前一级
	var child model.User
	if err := repository.DB.Where("id = ? AND parent_id = ?", req.SecondaryID, UID(c)).First(&child).Error; err != nil {
		Fail(c, 400, "目标子账号不存在")
		return
	}
	g := model.ProductGrant{
		ProductID: p.ID, SecondaryID: child.ID,
		GrantedBy: UID(c), Permission: "operate",
	}
	if err := repository.DB.Create(&g).Error; err != nil {
		Fail(c, 500, "下放失败（可能已下放给该账号）")
		return
	}
	OK(c, g)
}

// DeleteGrant 撤销产品对某二级账号的下放
func DeleteGrant(c *gin.Context) {
	p, err := mustOwnProduct(c, c.Param("id"))
	if err != nil {
		Fail(c, 404, "产品不存在")
		return
	}
	res := repository.DB.Where("product_id = ? AND secondary_id = ?", p.ID, c.Param("sid")).Delete(&model.ProductGrant{})
	if res.RowsAffected == 0 {
		Fail(c, 404, "下放记录不存在")
		return
	}
	OK(c, nil)
}

// uintPtr 取 uint 的指针（用于 ParentID 赋值）
func uintPtr(v uint) *uint { return &v }
