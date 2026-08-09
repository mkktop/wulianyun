package api

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gin-gonic/gin"

	"iot-platform/internal/model"
	"iot-platform/internal/repository"
)

func randHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// genProductKey 生成产品标识：2 位随机大写字母 + 10 位随机数字（如 AB1234567890）
func genProductKey() string {
	const letters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	const digits = "0123456789"
	b := make([]byte, 12)
	rand.Read(b)
	for i := 0; i < 2; i++ {
		b[i] = letters[int(b[i])%len(letters)]
	}
	for i := 2; i < 12; i++ {
		b[i] = digits[int(b[i])%len(digits)]
	}
	return string(b)
}

type productReq struct {
	Name         string `json:"name" binding:"required,max=64"`
	Protocol     string `json:"protocol" binding:"omitempty,oneof=mqtt tcp http"`
	DataFormat   string `json:"dataFormat"`
	AccessMode   string `json:"accessMode" binding:"omitempty,oneof=thingmodel passthrough modbus"`
	SecretMode   string `json:"secretMode" binding:"omitempty,oneof=device product"`
	PollInterval  int    `json:"pollInterval"`
	RequestTimeout int  `json:"requestTimeout"` // Modbus 单次 RTU 请求超时(秒)，钳制[3,30]，默认3
	Description  string `json:"description"`

	// TCP 组帧/心跳（透传产品可配；Modbus 固定按 RTU 组帧）
	FrameMode       string `json:"frameMode" binding:"omitempty,oneof=none delimiter length"`
	FrameDelimiter  string `json:"frameDelimiter"`
	FrameLenOffset  int    `json:"frameLenOffset"`
	FrameLenSize    int    `json:"frameLenSize"`
	FrameLenAdjust  int    `json:"frameLenAdjust"`
	HeartbeatPacket string `json:"heartbeatPacket"`
	HeartbeatReply  string `json:"heartbeatReply"`
}

// @Summary      创建产品
// @Description  新建设备产品（自动生成 ProductKey；一型一密时生成 ProductSecret）
// @Tags         产品
// @Produce      json
// @Param        body body object true "产品信息 {name, protocol, accessMode, secretMode, pollInterval, description, frameMode, ...}"
// @Success      200  {object}  Resp
// @Failure      400  {object}  Resp
// @Router       /products [post]
// @Security     BearerAuth
func CreateProduct(c *gin.Context) {
	if isSecondary(c) {
		Fail(c, 403, "二级账号无法创建产品，请使用主账号下放的产品")
		return
	}
	var req productReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, 400, "产品名称必填")
		return
	}
	if req.Protocol == "" {
		req.Protocol = "mqtt"
	}
	if req.DataFormat == "" {
		req.DataFormat = "json"
	}
	if req.AccessMode == "" {
		req.AccessMode = model.AccessModeThingModel
	}
	if req.SecretMode == "" {
		req.SecretMode = model.SecretModeDevice
	}
	if req.PollInterval < 60 {
		req.PollInterval = 60
	}
	// Modbus 单次 RTU 请求超时钳制：最低 3s（防过短误超时），上限 30s（防拖垮轮询节奏）
	if req.RequestTimeout < 3 {
		req.RequestTimeout = 3
	}
	if req.RequestTimeout > 30 {
		req.RequestTimeout = 30
	}
	// Modbus 接入必须走 TCP（MQTT/HTTP 不支持 Modbus）
	if req.AccessMode == model.AccessModeModbus {
		req.Protocol = "tcp"
	} else if req.Protocol == "" {
		req.Protocol = "mqtt"
	}
	p := model.Product{
		UserID: UID(c), Name: req.Name, ProductKey: genProductKey(),
		Protocol: req.Protocol, DataFormat: req.DataFormat, Description: req.Description,
		AccessMode: req.AccessMode, SecretMode: req.SecretMode,
		PollInterval: req.PollInterval, RequestTimeout: req.RequestTimeout,
		FrameMode: req.FrameMode, FrameDelimiter: req.FrameDelimiter,
		FrameLenOffset: req.FrameLenOffset, FrameLenSize: req.FrameLenSize, FrameLenAdjust: req.FrameLenAdjust,
		HeartbeatPacket: req.HeartbeatPacket, HeartbeatReply: req.HeartbeatReply,
	}
	if p.FrameMode == "" {
		p.FrameMode = model.FrameModeNone
	}
	// 产品名称在同一拥有者名下唯一（区分大小写）
	var dup int64
	repository.DB.Model(&model.Product{}).Where("user_id = ? AND name = ?", p.UserID, p.Name).Count(&dup)
	if dup > 0 {
		Fail(c, 400, "产品名称已存在，请更换")
		return
	}
	// 一型一密：生成产品级密钥
	if req.SecretMode == model.SecretModeProduct {
		p.ProductSecret = randHex(16)
	}
	if err := repository.DB.Create(&p).Error; err != nil {
		Fail(c, 500, "创建失败")
		return
	}
	OK(c, p)
}

// @Summary      产品列表
// @Description  分页查询当前账号可见的产品，附带每产品设备数
// @Tags         产品
// @Produce      json
// @Param        keyword query string false "名称模糊关键字"
// @Param        page query int false "页码"
// @Param        size query int false "每页数量"
// @Success      200  {object}  Resp
// @Failure      400  {object}  Resp
// @Router       /products [get]
// @Security     BearerAuth
func ListProducts(c *gin.Context) {
	var list []model.Product
	q := repository.DB.Model(&model.Product{}).Scopes(productScope(c))
	if kw := c.Query("keyword"); kw != "" {
		q = q.Where("name ILIKE ?", "%"+kw+"%")
	}
	var total int64
	q.Count(&total)
	page, size := pageArgs(c)
	if err := q.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&list).Error; err != nil {
		Fail(c, 500, "查询失败")
		return
	}
	for i := range list {
		repository.DB.Model(&model.Device{}).Where("product_id = ?", list[i].ID).Count(&list[i].DeviceCount)
	}
	OK(c, PageData{Total: total, List: list})
}

// @Summary      产品详情
// @Description  获取指定产品（含设备数），需有查看权限
// @Tags         产品
// @Produce      json
// @Param        id path int true "产品ID"
// @Success      200  {object}  Resp
// @Failure      400  {object}  Resp
// @Router       /products/{id} [get]
// @Security     BearerAuth
func GetProduct(c *gin.Context) {
	p, err := canViewProduct(c, c.Param("id"))
	if err != nil {
		Fail(c, 404, "产品不存在")
		return
	}
	repository.DB.Model(&model.Device{}).Where("product_id = ?", p.ID).Count(&p.DeviceCount)
	OK(c, p)
}

func UpdateProduct(c *gin.Context) {
	p, err := mustOwnProduct(c, c.Param("id"))
	if err != nil {
		Fail(c, 404, "产品不存在")
		return
	}
	var req productReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, 400, "参数错误")
		return
	}
	// 改名时校验同一拥有者名下名称唯一（排除自身）
	if req.Name != p.Name {
		var dup int64
		repository.DB.Model(&model.Product{}).
			Where("user_id = ? AND name = ? AND id <> ?", p.UserID, req.Name, p.ID).
			Count(&dup)
		if dup > 0 {
			Fail(c, 400, "产品名称已存在，请更换")
			return
		}
	}
	// 接入方式/协议/密钥模式创建后不可变；允许改名称/描述/采集周期/请求超时/组帧与心跳配置
	updates := map[string]interface{}{"name": req.Name, "description": req.Description}
	if p.AccessMode == model.AccessModeModbus && req.PollInterval >= 60 {
		updates["poll_interval"] = req.PollInterval
	}
	// Modbus 请求超时：钳制 [3,30]，仅在 modbus 产品生效
	if p.AccessMode == model.AccessModeModbus && req.RequestTimeout > 0 {
		rt := req.RequestTimeout
		if rt < 3 {
			rt = 3
		}
		if rt > 30 {
			rt = 30
		}
		updates["request_timeout"] = rt
	}
	// 非 Modbus 的 TCP 产品：允许调整组帧/心跳配置
	if p.Protocol == "tcp" && p.AccessMode != model.AccessModeModbus {
		if req.FrameMode != "" {
			updates["frame_mode"] = req.FrameMode
		}
		updates["frame_delimiter"] = req.FrameDelimiter
		updates["frame_len_offset"] = req.FrameLenOffset
		updates["frame_len_size"] = req.FrameLenSize
		updates["frame_len_adjust"] = req.FrameLenAdjust
		updates["heartbeat_packet"] = req.HeartbeatPacket
		updates["heartbeat_reply"] = req.HeartbeatReply
	}
	repository.DB.Model(&p).Updates(updates)
	OK(c, p)
}

func DeleteProduct(c *gin.Context) {
	p, err := mustOwnProduct(c, c.Param("id"))
	if err != nil {
		Fail(c, 404, "产品不存在")
		return
	}
	var cnt int64
	repository.DB.Model(&model.Device{}).Where("product_id = ?", p.ID).Count(&cnt)
	if cnt > 0 {
		Fail(c, 400, "产品下还有设备，请先删除设备")
		return
	}
	repository.DB.Where("product_id = ?", p.ID).Delete(&model.ProductGrant{}) // 清理下放授权
	repository.DB.Delete(&p)
	OK(c, nil)
}

func pageArgs(c *gin.Context) (page, size int) {
	page, size = 1, 10
	if v := c.Query("page"); v != "" {
		if n := atoi(v); n > 0 {
			page = n
		}
	}
	if v := c.Query("size"); v != "" {
		if n := atoi(v); n > 0 && n <= 100 {
			size = n
		}
	}
	return
}

func atoi(s string) int {
	n := 0
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0
		}
		n = n*10 + int(ch-'0')
	}
	return n
}
