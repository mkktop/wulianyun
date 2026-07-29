package model

import "time"

// Modbus 功能码
const (
	FuncReadCoils            = 1  // 0x01 读线圈
	FuncReadDiscreteInputs   = 2  // 0x02 读离散量输入
	FuncReadHoldingRegisters = 3  // 0x03 读保持寄存器
	FuncReadInputRegisters   = 4  // 0x04 读输入寄存器
	FuncWriteSingleCoil      = 5  // 0x05 写单个线圈
	FuncWriteSingleRegister  = 6  // 0x06 写单个寄存器
	FuncWriteMultipleCoils   = 15 // 0x0F 写多个线圈
	FuncWriteMultipleRegs    = 16 // 0x10 写多个寄存器
)

// ModbusPoint Modbus 点位表：产品级配置，平台按此轮询/写入寄存器
type ModbusPoint struct {
	ID           uint    `gorm:"primaryKey" json:"id"`
	ProductID    uint    `gorm:"index;not null" json:"productId"`
	Identifier   string  `gorm:"size:50;not null" json:"identifier"` // 映射到物模型属性标识符
	Name         string  `gorm:"size:64;not null" json:"name"`
	SlaveID      uint8   `gorm:"default:1" json:"slaveId"`      // 从机地址 1-247
	FunctionCode int     `gorm:"not null" json:"functionCode"`  // 1/2/3/4/5/6/15/16
	Address      uint16  `json:"address"`                       // 寄存器地址
	RawType      string  `gorm:"size:16;default:uint16" json:"rawType"` // int16/uint16/int32/uint32/float/bool/bits
	BitPosition  int     `gorm:"default:0" json:"bitPosition"`  // bits 类型时的位偏移
	Scale        float64 `gorm:"default:1" json:"scale"`        // 缩放因子
	Offset       float64 `gorm:"default:0" json:"offset"`       // 偏移量
	SwapByte     bool    `gorm:"default:false" json:"swapByte"` // 寄存器内高低字节互换
	SwapWord     bool    `gorm:"default:false" json:"swapWord"` // 32位双寄存器字序互换
	AccessMode   string  `gorm:"size:8;default:r" json:"accessMode"` // r / rw
	Unit         string  `gorm:"size:16" json:"unit"`
	CreatedAt    time.Time `json:"createdAt"`
}

// RegisterCount 该点位读取需要的寄存器数量
func (p *ModbusPoint) RegisterCount() uint16 {
	switch p.RawType {
	case "int32", "uint32", "float":
		return 2
	default:
		return 1
	}
}

// IsCoilFunc 是否线圈/离散量类操作（按位），否则为寄存器（16 位字）
func (p *ModbusPoint) IsCoilFunc() bool {
	return p.FunctionCode == FuncReadCoils || p.FunctionCode == FuncReadDiscreteInputs ||
		p.FunctionCode == FuncWriteSingleCoil || p.FunctionCode == FuncWriteMultipleCoils
}
