// batch.go Modbus 寄存器批量读取优化：把同从机、同功能码、地址连续（或间隔在阈值内）的
// 点位合并成一次 Modbus 请求，大幅减少 RS485 总线往返次数。
package modbus

import (
	"encoding/binary"
	"fmt"
	"math"
	"sort"

	"iot-platform/internal/model"
)

// maxGapRegisters 合并时允许的最大地址空洞（跨过少量无用寄存器仍合并，减少请求数）
const maxGapRegisters = 8

// maxQuantityRegisters Modbus 单次读寄存器上限（协议 125），保守取 120
const maxQuantityRegisters = 120

// maxQuantityCoils 单次读线圈上限（协议 2000），保守取 2000
const maxQuantityCoils = 2000

// ReadBlock 一个合并后的读请求块：覆盖一段连续地址，包含若干点位
type ReadBlock struct {
	SlaveID      uint8
	FunctionCode int
	Start        uint16
	Quantity     uint16
	Points       []*model.ModbusPoint
}

// PlanReadBlocks 将点位按(从机+功能码)分桶，桶内按地址排序后合并为最少的读请求块
func PlanReadBlocks(points []*model.ModbusPoint) []*ReadBlock {
	// 仅保留读功能码点位
	type key struct {
		slave uint8
		fn    int
	}
	buckets := map[key][]*model.ModbusPoint{}
	for _, p := range points {
		if p.FunctionCode < model.FuncReadCoils || p.FunctionCode > model.FuncReadInputRegisters {
			continue
		}
		k := key{p.SlaveID, p.FunctionCode}
		buckets[k] = append(buckets[k], p)
	}

	var blocks []*ReadBlock
	// 稳定顺序：按从机、功能码排序桶
	var keys []key
	for k := range buckets {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].slave != keys[j].slave {
			return keys[i].slave < keys[j].slave
		}
		return keys[i].fn < keys[j].fn
	})

	for _, k := range keys {
		pts := buckets[k]
		sort.Slice(pts, func(i, j int) bool { return pts[i].Address < pts[j].Address })
		isCoil := pts[0].IsCoilFunc()
		maxQty := uint16(maxQuantityRegisters)
		if isCoil {
			maxQty = maxQuantityCoils
		}

		var cur *ReadBlock
		for _, p := range pts {
			if cur == nil {
				cur = newBlock(k.slave, k.fn, p)
				continue
			}
			end := cur.Start + cur.Quantity - 1
			// 与当前块的间隔（负数表示重叠/包含）
			gap := int(p.Address) - int(end) - 1
			newEnd := p.EndAddress()
			newQty := int(newEnd) - int(cur.Start) + 1
			if gap <= maxGapRegisters && newQty <= int(maxQty) {
				// 并入当前块
				if newEnd > end {
					cur.Quantity = uint16(newQty)
				}
				cur.Points = append(cur.Points, p)
			} else {
				blocks = append(blocks, cur)
				cur = newBlock(k.slave, k.fn, p)
			}
		}
		if cur != nil {
			blocks = append(blocks, cur)
		}
	}
	return blocks
}

func newBlock(slave uint8, fn int, p *model.ModbusPoint) *ReadBlock {
	qty := p.RegisterCount()
	if p.IsCoilFunc() {
		qty = 1
	}
	return &ReadBlock{
		SlaveID: slave, FunctionCode: fn,
		Start: p.Address, Quantity: qty, Points: []*model.ModbusPoint{p},
	}
}

// BuildBlockRequest 构造合并块的读请求帧
func BuildBlockRequest(b *ReadBlock) []byte {
	frame := []byte{b.SlaveID, byte(b.FunctionCode)}
	frame = append(frame, byte(b.Start>>8), byte(b.Start&0xFF))
	frame = append(frame, byte(b.Quantity>>8), byte(b.Quantity&0xFF))
	return appendCRC(frame)
}

// ParseBlockResponse 解析合并块应答，按各点位相对偏移提取缩放后的值
// 返回 identifier -> value
func ParseBlockResponse(b *ReadBlock, frame []byte) (map[string]float64, error) {
	if len(frame) < 5 {
		return nil, fmt.Errorf("应答帧过短(%d字节)", len(frame))
	}
	if !checkCRC(frame) {
		return nil, fmt.Errorf("CRC 校验失败")
	}
	if frame[0] != b.SlaveID {
		return nil, fmt.Errorf("从机地址不匹配: 期望 %d 收到 %d", b.SlaveID, frame[0])
	}
	if frame[1]&0x80 != 0 {
		code := byte(0)
		if len(frame) >= 3 {
			code = frame[2]
		}
		return nil, fmt.Errorf("Modbus 异常响应，异常码 %d", code)
	}
	if frame[1] != byte(b.FunctionCode) {
		return nil, fmt.Errorf("功能码不匹配: 期望 %d 收到 %d", b.FunctionCode, frame[1])
	}
	byteCount := int(frame[2])
	if len(frame) < 3+byteCount+2 {
		return nil, fmt.Errorf("数据长度不足")
	}
	data := frame[3 : 3+byteCount]

	result := map[string]float64{}
	coil := b.Points[0].IsCoilFunc()
	for _, p := range b.Points {
		if coil {
			// 线圈：按位偏移取 bit
			bitIdx := int(p.Address) - int(b.Start)
			byteIdx := bitIdx / 8
			if byteIdx >= len(data) {
				continue
			}
			bit := (data[byteIdx] >> uint(bitIdx%8)) & 1
			result[p.Identifier] = float64(bit)
			continue
		}
		// 寄存器：按寄存器偏移 *2 定位字节
		regOff := int(p.Address) - int(b.Start)
		start := regOff * 2
		need := int(p.RegisterCount()) * 2
		if start < 0 || start+need > len(data) {
			continue
		}
		raw, err := decodeRegisterBytes(p, data[start:start+need])
		if err != nil {
			continue
		}
		result[p.Identifier] = raw*p.Scale + p.Offset
	}
	return result, nil
}

// decodeRegisterBytes 与 decodeRegisters 同逻辑，但入参为已切好的定长字节
func decodeRegisterBytes(p *model.ModbusPoint, seg []byte) (float64, error) {
	data := normalize(seg, p.SwapByte, p.SwapWord)
	switch p.RawType {
	case "int16":
		return float64(int16(binary.BigEndian.Uint16(data))), nil
	case "uint16":
		return float64(binary.BigEndian.Uint16(data)), nil
	case "int32":
		return float64(int32(binary.BigEndian.Uint32(data))), nil
	case "uint32":
		return float64(binary.BigEndian.Uint32(data)), nil
	case "float":
		return float64(math.Float32frombits(binary.BigEndian.Uint32(data))), nil
	case "bits":
		v := binary.BigEndian.Uint16(data)
		return float64((v >> uint(p.BitPosition)) & 1), nil
	case "bool":
		return float64(binary.BigEndian.Uint16(data) & 1), nil
	default:
		return float64(binary.BigEndian.Uint16(data)), nil
	}
}
