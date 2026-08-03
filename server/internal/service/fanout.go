package service

import (
	"sync"

	"iot-platform/internal/model"
	"iot-platform/internal/repository"
)

// parentCache 用户父账号缓存：uid -> *uint（nil 表示无父账号，即一级/超管）。
// 父子关系在 MVP 不可变（二级账号不会 reparent），故缓存无需失效。
var parentCache sync.Map

// PushRecipients 返回应接收某设备实时数据/告警的用户集合：
// 设备归属者本身 + 其一级父账号（当归属者是二级账号时）。
// 供 ingest 的 WS 推送与 rule 告警 fan-out 复用（经 main.go 注入到 rule.RecipientResolver）。
func PushRecipients(ownerID uint) []uint {
	var parent *uint
	if v, ok := parentCache.Load(ownerID); ok {
		parent, _ = v.(*uint)
	} else {
		var u model.User
		if err := repository.DB.Select("id", "parent_id").First(&u, ownerID).Error; err == nil {
			parent = u.ParentID
			parentCache.Store(ownerID, parent)
		}
	}
	if parent != nil {
		return []uint{ownerID, *parent}
	}
	return []uint{ownerID}
}
