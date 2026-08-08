package api

import (
	"encoding/hex"
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"iot-platform/internal/modbus"
	"iot-platform/internal/model"
	"iot-platform/internal/repository"
)

// GetModbusPoints 查询产品 Modbus 点位表
func GetModbusPoints(c *gin.Context) {
	p, err := canViewProduct(c, c.Param("id"))
	if err != nil {
		Fail(c, 404, "产品不存在")
		return
	}
	var points []model.ModbusPoint
	repository.DB.Where("product_id = ?", p.ID).Order("id asc").Find(&points)
	OK(c, points)
}

type modbusPointReq struct {
	Identifier   string  `json:"identifier"`
	Name         string  `json:"name"`
	GroupID      uint    `json:"groupId"`
	SlaveID      uint8   `json:"slaveId"`
	FunctionCode int     `json:"functionCode"`
	Address      uint16  `json:"address"`
	RawType      string  `json:"rawType"`
	BitPosition  int     `json:"bitPosition"`
	Scale        float64 `json:"scale"`
	Offset       float64 `json:"offset"`
	SwapByte     bool    `json:"swapByte"`
	SwapWord     bool    `json:"swapWord"`
	AccessMode   string  `json:"accessMode"`
	Unit         string  `json:"unit"`
}

// SaveModbusPoints 覆盖保存产品点位表
func SaveModbusPoints(c *gin.Context) {
	p, err := mustOwnProduct(c, c.Param("id"))
	if err != nil {
		Fail(c, 404, "产品不存在")
		return
	}
	var req struct {
		Points []modbusPointReq `json:"points"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, 400, "参数错误")
		return
	}
	if len(req.Points) > 100 {
		Fail(c, 400, "点位数量不能超过 100")
		return
	}
	seen := map[string]bool{}
	points := make([]model.ModbusPoint, 0, len(req.Points))
	for _, r := range req.Points {
		if r.Identifier == "" || r.Name == "" {
			Fail(c, 400, "点位标识符和名称必填")
			return
		}
		if seen[r.Identifier] {
			Fail(c, 400, "点位标识符重复: "+r.Identifier)
			return
		}
		seen[r.Identifier] = true
		if r.Scale == 0 {
			r.Scale = 1
		}
		if r.SlaveID == 0 {
			r.SlaveID = 1
		}
		if r.AccessMode == "" {
			r.AccessMode = "r"
		}
		points = append(points, model.ModbusPoint{
			ProductID: p.ID, GroupID: r.GroupID, Identifier: r.Identifier, Name: r.Name,
			SlaveID: r.SlaveID, FunctionCode: r.FunctionCode, Address: r.Address,
			RawType: r.RawType, BitPosition: r.BitPosition, Scale: r.Scale, Offset: r.Offset,
			SwapByte: r.SwapByte, SwapWord: r.SwapWord, AccessMode: r.AccessMode, Unit: r.Unit,
		})
	}
	// 事务覆盖：Delete + Create 必须原子，否则 Create 失败时点位表已被清空（静默全量丢点）
	err = repository.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("product_id = ?", p.ID).Delete(&model.ModbusPoint{}).Error; err != nil {
			return err
		}
		if len(points) > 0 {
			return tx.Create(&points).Error
		}
		return nil
	})
	if err != nil {
		slog.Error("save modbus points failed", "productId", p.ID, "err", err)
		Fail(c, 500, "保存点位失败")
		return
	}
	OK(c, points)
}

// TestModbusPoint 用示例应答帧(hex)验证点位解析
func TestModbusPoint(c *gin.Context) {
	_, err := canViewProduct(c, c.Param("id"))
	if err != nil {
		Fail(c, 404, "产品不存在")
		return
	}
	var req struct {
		Point modbusPointReq `json:"point"`
		Hex   string         `json:"hex"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, 400, "点位和应答帧(hex)必填")
		return
	}
	data, err := hex.DecodeString(strings.ReplaceAll(req.Hex, " ", ""))
	if err != nil {
		Fail(c, 400, "hex 报文格式错误")
		return
	}
	pt := &model.ModbusPoint{
		SlaveID: req.Point.SlaveID, FunctionCode: req.Point.FunctionCode, Address: req.Point.Address,
		RawType: req.Point.RawType, BitPosition: req.Point.BitPosition,
		Scale: req.Point.Scale, Offset: req.Point.Offset,
		SwapByte: req.Point.SwapByte, SwapWord: req.Point.SwapWord,
	}
	if pt.Scale == 0 {
		pt.Scale = 1
	}
	if pt.SlaveID == 0 {
		pt.SlaveID = 1
	}
	v, err := modbus.ParseResponse(pt, data)
	if err != nil {
		Fail(c, 400, err.Error())
		return
	}
	OK(c, gin.H{"value": v})
}
