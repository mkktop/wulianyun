// TCP 字节流组帧：把连续的 TCP 字节流切分为完整的应用层帧，解决粘包/拆包。
//
// 组帧模式（按产品配置）：
//   - modbus    ：按 Modbus RTU 响应帧头推断长度 + CRC 校验（Modbus 产品固定使用）
//   - delimiter ：按定界符切分（如 0D0A）
//   - length    ：按帧内长度字段切分
//   - none      ：不组帧，读到多少算一帧（兼容旧行为）
package gateway

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"strings"

	"iot-platform/internal/model"
	"iot-platform/internal/modbus"
)

const maxFrameSize = 8192

type framer struct {
	mode      string
	buf       []byte
	delim     []byte
	lenOffset int
	lenSize   int
	lenAdjust int
}

// newFramer 按产品配置构造组帧器；Modbus 产品固定走 modbus 模式
func newFramer(p *model.Product) *framer {
	f := &framer{}
	if p.AccessMode == model.AccessModeModbus {
		f.mode = "modbus"
		return f
	}
	switch p.FrameMode {
	case model.FrameModeDelimiter:
		f.delim = parseHex(p.FrameDelimiter)
		if len(f.delim) == 0 {
			f.mode = model.FrameModeNone
		} else {
			f.mode = model.FrameModeDelimiter
		}
	case model.FrameModeLength:
		f.mode = model.FrameModeLength
		f.lenOffset = p.FrameLenOffset
		f.lenSize = p.FrameLenSize
		if f.lenSize != 2 {
			f.lenSize = 1
		}
		f.lenAdjust = p.FrameLenAdjust
	default:
		f.mode = model.FrameModeNone
	}
	return f
}

// append 追加新读入的字节；缓冲异常膨胀时丢弃头部，防止内存无限增长
func (f *framer) append(b []byte) {
	f.buf = append(f.buf, b...)
	if len(f.buf) > maxFrameSize*4 {
		f.buf = f.buf[len(f.buf)-maxFrameSize:]
	}
}

// next 从缓冲切出下一个完整帧；无完整帧时返回 (nil,false)
func (f *framer) next() ([]byte, bool) {
	switch f.mode {
	case "modbus":
		return f.nextModbus()
	case model.FrameModeDelimiter:
		return f.nextDelimiter()
	case model.FrameModeLength:
		return f.nextLength()
	default:
		return f.nextNone()
	}
}

func (f *framer) nextNone() ([]byte, bool) {
	if len(f.buf) == 0 {
		return nil, false
	}
	out := f.buf
	f.buf = nil
	return out, true
}

func (f *framer) nextDelimiter() ([]byte, bool) {
	for {
		i := bytes.Index(f.buf, f.delim)
		if i < 0 {
			return nil, false
		}
		frame := f.buf[:i]
		f.buf = f.buf[i+len(f.delim):]
		if len(frame) == 0 {
			continue // 跳过空帧（连续定界符）
		}
		out := make([]byte, len(frame))
		copy(out, frame)
		return out, true
	}
}

func (f *framer) nextLength() ([]byte, bool) {
	for {
		if len(f.buf) < f.lenOffset+f.lenSize {
			return nil, false
		}
		var lv int
		if f.lenSize == 2 {
			lv = int(binary.BigEndian.Uint16(f.buf[f.lenOffset:]))
		} else {
			lv = int(f.buf[f.lenOffset])
		}
		total := lv + f.lenAdjust
		if total <= 0 || total > maxFrameSize {
			f.buf = f.buf[1:] // 长度非法，丢 1 字节重新同步
			continue
		}
		if len(f.buf) < total {
			return nil, false
		}
		out := make([]byte, total)
		copy(out, f.buf[:total])
		f.buf = f.buf[total:]
		return out, true
	}
}

// nextModbus 按 Modbus RTU 响应帧头推断长度并做 CRC 校验
func (f *framer) nextModbus() ([]byte, bool) {
	for {
		n := modbusFrameLen(f.buf)
		if n == 0 {
			return nil, false // 头部信息不足，等待更多字节
		}
		if n < 0 || n > maxFrameSize {
			f.buf = f.buf[1:] // 功能码非法，丢 1 字节重新同步
			continue
		}
		if len(f.buf) < n {
			return nil, false
		}
		if !modbus.CheckCRC(f.buf[:n]) {
			f.buf = f.buf[1:] // CRC 不过，丢 1 字节重新同步
			continue
		}
		out := make([]byte, n)
		copy(out, f.buf[:n])
		f.buf = f.buf[n:]
		return out, true
	}
}

// modbusFrameLen 由响应帧头推断整帧长度：
// 返回 0=信息不足需等待，-1=功能码非法，>0=帧总长
func modbusFrameLen(buf []byte) int {
	if len(buf) < 2 {
		return 0
	}
	fn := buf[1]
	if fn&0x80 != 0 {
		return 5 // 异常响应：地址+功能码+异常码+CRC
	}
	switch fn {
	case model.FuncReadCoils, model.FuncReadDiscreteInputs,
		model.FuncReadHoldingRegisters, model.FuncReadInputRegisters:
		if len(buf) < 3 {
			return 0 // 还没读到字节数
		}
		return 3 + int(buf[2]) + 2
	case model.FuncWriteSingleCoil, model.FuncWriteSingleRegister,
		model.FuncWriteMultipleCoils, model.FuncWriteMultipleRegs:
		return 8 // 写应答固定 8 字节
	}
	return -1
}

// parseHex 解析 HEX 字符串（可选 0x 前缀）为字节；非法返回 nil
func parseHex(s string) []byte {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "0x")
	s = strings.TrimPrefix(s, "0X")
	b, err := hex.DecodeString(s)
	if err != nil || len(b) == 0 {
		return nil
	}
	return b
}

// parseBytes 解析心跳等配置：0x 开头按 HEX，否则按文本
func parseBytes(s string) []byte {
	if s == "" {
		return nil
	}
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		if b := parseHex(s); b != nil {
			return b
		}
	}
	return []byte(s)
}
