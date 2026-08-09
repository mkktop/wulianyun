package api

import (
	"encoding/hex"
	"strings"

	"github.com/gin-gonic/gin"

	"iot-platform/internal/codec"
	"iot-platform/internal/repository"
)

// GetCodec 查询产品解析脚本
func GetCodec(c *gin.Context) {
	p, err := canViewProduct(c, c.Param("id"))
	if err != nil {
		Fail(c, 404, "产品不存在")
		return
	}
	OK(c, gin.H{"script": p.CodecScript})
}

// SaveCodec 保存产品解析脚本（空脚本表示关闭脚本解析）
func SaveCodec(c *gin.Context) {
	p, err := mustOwnProduct(c, c.Param("id"))
	if err != nil {
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
	// 非空脚本先做一次真实编译校验（坏脚本拒绝落库，否则线上每包都解析失败）
	if req.Script != "" {
		if err := codec.Validate(p.ID, req.Script); err != nil {
			Fail(c, 400, err.Error())
			return
		}
	}
	repository.DB.Model(p).Update("codec_script", req.Script)
	OK(c, nil)
}

// TestCodec 用十六进制报文测试解析脚本
func TestCodec(c *gin.Context) {
	_, err := canViewProduct(c, c.Param("id"))
	if err != nil {
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
	// 测试解析：不污染产品缓存；脚本/报文大小由 codec 层限制
	obj, err := codec.TestDecode(req.Script, data)
	if err != nil {
		Fail(c, 400, err.Error())
		return
	}
	OK(c, obj)
}
