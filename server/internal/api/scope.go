package api

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"iot-platform/internal/model"
	"iot-platform/internal/repository"
)

// 本文件集中实现"一/二级账号"的可见性解析。两类资源、两套规则：
//
//  1. 设备类资源（设备/告警/规则/事件/轨迹/日志/OTA/OpenApp/分组）按"归属者"隔离：
//     - 平台超管：看全部
//     - 一级主账号：{自己} ∪ {名下所有二级账号}（即看得到二级的所有设备）
//     - 二级账号：仅自己
//
//  2. 产品：按"所有权 + 下放授权"：
//     - 平台超管：全部
//     - 一级：自己拥有的产品
//     - 二级：自己拥有的 ∪ 一级下放给自己的
//     - 产品定义（物模型/编解码/删除）仅 owner/超管可改；二级对下放产品只读。

// visibleOwnerIDs 返回设备类资源的可见归属者集合。
// admin → nil（调用方据此跳过过滤）；一级 → {自己∪名下二级}；二级 → {自己}。
func visibleOwnerIDs(c *gin.Context) []uint {
	if IsAdmin(c) {
		return nil
	}
	uid := UID(c)
	ids := []uint{uid}
	if ParentID(c) == nil { // 一级：含所有名下二级账号（不论启用/禁用，便于管理其设备）
		var children []uint
		repository.DB.Model(&model.User{}).
			Where("parent_id = ?", uid).
			Pluck("id", &children)
		ids = append(ids, children...)
	}
	return ids
}

// ownedScope 设备类资源可见性 GORM scope。admin 不过滤；否则 col IN visibleOwnerIDs。
// col 默认 "user_id"；联表查询传 "devices.user_id" 等带表名的列。
func ownedScope(c *gin.Context, col string) func(*gorm.DB) *gorm.DB {
	return func(q *gorm.DB) *gorm.DB {
		ids := visibleOwnerIDs(c)
		if ids == nil {
			return q
		}
		if col == "" {
			col = "user_id"
		}
		return q.Where(col+" IN ?", ids)
	}
}

// productScope 产品可见性 GORM scope（admin 全部；一级自有；二级 自有∪下放）。
func productScope(c *gin.Context) func(*gorm.DB) *gorm.DB {
	return func(q *gorm.DB) *gorm.DB {
		if IsAdmin(c) {
			return q
		}
		uid := UID(c)
		return q.Where(
			"user_id = ? OR id IN (SELECT product_id FROM product_grants WHERE secondary_id = ?)",
			uid, uid,
		)
	}
}

// canViewProduct 取一个"可读"的产品（owner / 被下放 / 超管）。不可见返回 ErrRecordNotFound。
func canViewProduct(c *gin.Context, id any) (*model.Product, error) {
	var p model.Product
	err := repository.DB.Scopes(productScope(c)).Where("id = ?", id).First(&p).Error
	return &p, err
}

// mustOwnProduct 取一个"可写定义"的产品（仅 owner / 超管）。二级对下放产品只读 → ErrRecordNotFound。
func mustOwnProduct(c *gin.Context, id any) (*model.Product, error) {
	var p model.Product
	err := repository.DB.Where("id = ?", id).First(&p).Error
	if err != nil {
		return &p, err
	}
	if IsAdmin(c) || p.UserID == UID(c) {
		return &p, nil
	}
	return &p, gorm.ErrRecordNotFound
}

// productGranted 判断产品 productID 是否已下放给 uid。
func productGranted(uid, productID uint) bool {
	var cnt int64
	repository.DB.Model(&model.ProductGrant{}).
		Where("secondary_id = ? AND product_id = ?", uid, productID).
		Count(&cnt)
	return cnt > 0
}

// isSecondary 当前账号是否为二级账号
func isSecondary(c *gin.Context) bool { return Tier(c) == "secondary" }
