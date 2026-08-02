// Package modbus 提供 Modbus RTU 帧的编解码：请求帧构造、应答解析、写请求构造。
// 遵循 Modbus RTU 规范：从机地址(1) + 功能码(1) + 数据(N) + CRC16(2, 低字节在前)。
package modbus

import (
	"encoding/binary"
	"fmt"
	"math"

	"iot-platform/internal/model"
)

// CRC16 计算 Modbus CRC16（多项式 0xA001，初值 0xFFFF）
func CRC16(data []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range data {
		crc ^= uint16(b)
		for i := 0; i < 8; i++ {
			if crc&1 != 0 {
				crc = (crc >> 1) ^ 0xA001
			} else {
				crc >>= 1
			}
		}
	}
	return crc
}

// appendCRC 在帧尾追加 CRC16（低字节在前）
func appendCRC(frame []byte) []byte {
	crc := CRC16(frame)
	return append(frame, byte(crc&0xFF), byte(crc>>8))
}

// checkCRC 校验帧尾 CRC16
func checkCRC(frame []byte) bool {
	if len(frame) < 3 {
		return false
	}
	body := frame[:len(frame)-2]
	crc := CRC16(body)
	return frame[len(frame)-2] == byte(crc&0xFF) && frame[len(frame)-1] == byte(crc>>8)
}

// CheckCRC 对外暴露的 CRC16 帧校验（供 TCP 网关组帧使用）
func CheckCRC(frame []byte) bool {
	return checkCRC(frame)
}

// BuildReadRequest 构造读请求帧（功能码 1/2/3/4）
func BuildReadRequest(p *model.ModbusPoint) ([]byte, error) {
	switch p.FunctionCode {
	case model.FuncReadCoils, model.FuncReadDiscreteInputs,
		model.FuncReadHoldingRegisters, model.FuncReadInputRegisters:
	default:
		return nil, fmt.Errorf("功能码 %d 不是读操作", p.FunctionCode)
	}
	qty := p.RegisterCount()
	if p.IsCoilFunc() {
		qty = 1 // 线圈按位读，取 1 位
	}
	frame := []byte{p.SlaveID, byte(p.FunctionCode)}
	frame = append(frame, byte(p.Address>>8), byte(p.Address&0xFF))
	frame = append(frame, byte(qty>>8), byte(qty&0xFF))
	return appendCRC(frame), nil
}

// ParseResponse 解析读应答帧，返回缩放后的数值
// 应答格式：从机地址(1) + 功能码(1) + 字节数(1) + 数据(N) + CRC(2)
func ParseResponse(p *model.ModbusPoint, frame []byte) (float64, error) {
	if len(frame) < 5 {
		return 0, fmt.Errorf("应答帧过短(%d字节)", len(frame))
	}
	if !checkCRC(frame) {
		return 0, fmt.Errorf("CRC 校验失败")
	}
	if frame[0] != p.SlaveID {
		return 0, fmt.Errorf("从机地址不匹配: 期望 %d 收到 %d", p.SlaveID, frame[0])
	}
	// 异常响应：功能码最高位置 1
	if frame[1]&0x80 != 0 {
		code := byte(0)
		if len(frame) >= 3 {
			code = frame[2]
		}
		return 0, fmt.Errorf("Modbus 异常响应，异常码 %d", code)
	}
	if frame[1] != byte(p.FunctionCode) {
		return 0, fmt.Errorf("功能码不匹配: 期望 %d 收到 %d", p.FunctionCode, frame[1])
	}
	byteCount := int(frame[2])
	data := frame[3 : 3+byteCount]
	if len(data) < byteCount {
		return 0, fmt.Errorf("数据长度不足")
	}

	// 线圈/离散量：按位取
	if p.IsCoilFunc() {
		if len(data) < 1 {
			return 0, fmt.Errorf("线圈数据为空")
		}
		bit := (data[0] >> 0) & 1
		return float64(bit), nil
	}

	raw, err := decodeRegisters(p, data)
	if err != nil {
		return 0, err
	}
	return raw*p.Scale + p.Offset, nil
}

// decodeRegisters 按原始类型与字节序解析寄存器数据为原始数值
func decodeRegisters(p *model.ModbusPoint, data []byte) (float64, error) {
	need := int(p.RegisterCount()) * 2
	if len(data) < need {
		return 0, fmt.Errorf("寄存器数据长度不足: 需要 %d 收到 %d", need, len(data))
	}
	data = normalize(data[:need], p.SwapByte, p.SwapWord)

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

// normalize 应用字节序变换：SwapByte 交换每个寄存器内高低字节；SwapWord 交换双寄存器字序
func normalize(data []byte, swapByte, swapWord bool) []byte {
	out := make([]byte, len(data))
	copy(out, data)
	if swapByte {
		for i := 0; i+1 < len(out); i += 2 {
			out[i], out[i+1] = out[i+1], out[i]
		}
	}
	if swapWord && len(out) == 4 {
		out[0], out[1], out[2], out[3] = out[2], out[3], out[0], out[1]
	}
	return out
}

// BuildWriteRequest 构造写请求帧（功能码 5/6/15/16），value 为缩放前的目标业务值
func BuildWriteRequest(p *model.ModbusPoint, value float64) ([]byte, error) {
	// 反向缩放：业务值 -> 原始寄存器值
	raw := value
	if p.Scale != 0 {
		raw = (value - p.Offset) / p.Scale
	}
	frame := []byte{p.SlaveID, byte(p.FunctionCode)}
	frame = append(frame, byte(p.Address>>8), byte(p.Address&0xFF))

	switch p.FunctionCode {
	case model.FuncWriteSingleCoil: // 0x05：ON=0xFF00 OFF=0x0000
		if raw != 0 {
			frame = append(frame, 0xFF, 0x00)
		} else {
			frame = append(frame, 0x00, 0x00)
		}
	case model.FuncWriteSingleRegister: // 0x06：写单个 16 位寄存器
		v := uint16(int64(raw))
		reg := []byte{byte(v >> 8), byte(v & 0xFF)}
		reg = normalize(reg, p.SwapByte, false)
		frame = append(frame, reg...)
	case model.FuncWriteMultipleRegs: // 0x10：按原始类型写 1 或 2 个寄存器
		regs := encodeRegisters(p, raw)
		count := uint16(len(regs) / 2)
		frame = append(frame, byte(count>>8), byte(count&0xFF), byte(len(regs)))
		frame = append(frame, regs...)
	case model.FuncWriteMultipleCoils: // 0x0F：写 1 个线圈
		frame = append(frame, 0x00, 0x01, 0x01)
		if raw != 0 {
			frame = append(frame, 0x01)
		} else {
			frame = append(frame, 0x00)
		}
	default:
		return nil, fmt.Errorf("功能码 %d 不是写操作", p.FunctionCode)
	}
	return appendCRC(frame), nil
}

// encodeRegisters 将原始数值编码为寄存器字节（应用字节序）
func encodeRegisters(p *model.ModbusPoint, raw float64) []byte {
	var buf []byte
	switch p.RawType {
	case "int32", "uint32":
		buf = make([]byte, 4)
		binary.BigEndian.PutUint32(buf, uint32(int64(raw)))
	case "float":
		buf = make([]byte, 4)
		binary.BigEndian.PutUint32(buf, math.Float32bits(float32(raw)))
	default:
		buf = make([]byte, 2)
		binary.BigEndian.PutUint16(buf, uint16(int64(raw)))
	}
	return normalize(buf, p.SwapByte, p.SwapWord)
}
