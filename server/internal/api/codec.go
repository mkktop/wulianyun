package api

import (
	"encoding/hex"
	"strings"

	"github.com/gin-gonic/gin"

	"iot-platform/internal/codec"
	"iot-platform/internal/model"
	"iot-platform/internal/repository"
)

// GetCodec 查询产品解析脚本
func GetCodec(c *gin.Context) {
	var p model.Product
	if err := repository.DB.Where("id = ? AND user_id = ?", c.Param("id"), UID(c)).First(&p).Error; err != nil {
		Fail(c, 404, "产品不存在")
		return
	}
	OK(c, gin.H{"script": p.CodecScript})
}

// SaveCodec 保存产品解析脚本（空脚本表示关闭脚本解析）
func SaveCodec(c *gin.Context) {
	var p model.Product
	if err := repository.DB.Where("id = ? AND user_id = ?", c.Param("id"), UID(c)).First(&p).Error; err != nil {
		Fail(c, 404, "产品不存在")
		return
	}
	var req struct {
		Script string `json:"script"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, 400, "参数错误")
		return
	}
	// 非空脚本先做一次编译验证（用空报文跑 decode，只报编译类错误）
	if req.Script != "" && !strings.Contains(req.Script, "function decode") {
		Fail(c, 400, "脚本必须包含 decode 函数")
		return
	}
	repository.DB.Model(&p).Update("codec_script", req.Script)
	OK(c, nil)
}

// TestCodec 用十六进制报文测试解析脚本
func TestCodec(c *gin.Context) {
	var p model.Product
	if err := repository.DB.Where("id = ? AND user_id = ?", c.Param("id"), UID(c)).First(&p).Error; err != nil {
		Fail(c, 404, "产品不存在")
		return
	}
	var req struct {
		Script string `json:"script" binding:"required"`
		Hex    string `json:"hex" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, 400, "脚本和测试报文(hex)必填")
		return
	}
	data, err := hex.DecodeString(strings.ReplaceAll(req.Hex, " ", ""))
	if err != nil {
		Fail(c, 400, "hex 报文格式错误")
		return
	}
	obj, err := codec.Decode(p.ID, req.Script, data)
	if err != nil {
		Fail(c, 400, err.Error())
		return
	}
	OK(c, obj)
}
