package gateway

import (
	"bytes"
	"testing"

	"iot-platform/internal/model"
)

// collect 把一段字节流按给定分片喂入 framer，返回切出的所有帧
func collect(f *framer, chunks ...[]byte) [][]byte {
	var out [][]byte
	for _, ch := range chunks {
		f.append(ch)
		for {
			frame, ok := f.next()
			if !ok {
				break
			}
			out = append(out, frame)
		}
	}
	return out
}

func TestFramerNone(t *testing.T) {
	f := newFramer(&model.Product{AccessMode: model.AccessModePassthrough, FrameMode: model.FrameModeNone})
	frames := collect(f, []byte("hello"))
	if len(frames) != 1 || string(frames[0]) != "hello" {
		t.Fatalf("none 模式应原样返回一帧, got %v", frames)
	}
}

func TestFramerDelimiter(t *testing.T) {
	p := &model.Product{AccessMode: model.AccessModePassthrough, FrameMode: model.FrameModeDelimiter, FrameDelimiter: "0D0A"}
	f := newFramer(p)
	// 粘包 + 拆包：两帧粘在一起，且第二帧被拆成两次到达
	frames := collect(f,
		[]byte("AAA\r\nBB"),
		[]byte("B\r\n"),
	)
	if len(frames) != 2 || string(frames[0]) != "AAA" || string(frames[1]) != "BBB" {
		t.Fatalf("定界符组帧错误, got %q", frames)
	}
}

func TestFramerLength(t *testing.T) {
	// 长度字段在偏移 0，1 字节，值=负载长度，帧总长 = 值 + 1(长度字节自身)
	p := &model.Product{
		AccessMode: model.AccessModePassthrough, FrameMode: model.FrameModeLength,
		FrameLenOffset: 0, FrameLenSize: 1, FrameLenAdjust: 1,
	}
	f := newFramer(p)
	// 帧1: len=3 + 3字节负载; 帧2: len=2 + 2字节负载；分多次拆包到达
	frames := collect(f,
		[]byte{0x03, 'a', 'b'},
		[]byte{'c', 0x02, 'd'},
		[]byte{'e'},
	)
	if len(frames) != 2 {
		t.Fatalf("长度组帧应切出2帧, got %d: %q", len(frames), frames)
	}
	if !bytes.Equal(frames[0], []byte{0x03, 'a', 'b', 'c'}) {
		t.Fatalf("帧1错误: %v", frames[0])
	}
	if !bytes.Equal(frames[1], []byte{0x02, 'd', 'e'}) {
		t.Fatalf("帧2错误: %v", frames[1])
	}
}

func TestFramerModbusSplitAndConcat(t *testing.T) {
	f := newFramer(&model.Product{AccessMode: model.AccessModeModbus})
	// 构造一个读保持寄存器应答: slave=1 fn=3 bytecount=2 data=0x00 0x0A + CRC
	resp := buildModbusReadResp(1, 3, []byte{0x00, 0x0A})
	// 另一帧: slave=1 fn=3 bytecount=4
	resp2 := buildModbusReadResp(1, 3, []byte{0x00, 0x01, 0x00, 0x02})

	// 粘包（resp+resp2）后再拆包
	stream := append(append([]byte{}, resp...), resp2...)
	frames := collect(f, stream[:3], stream[3:len(resp)+1], stream[len(resp)+1:])
	if len(frames) != 2 {
		t.Fatalf("Modbus 组帧应切出2帧, got %d", len(frames))
	}
	if !bytes.Equal(frames[0], resp) || !bytes.Equal(frames[1], resp2) {
		t.Fatalf("Modbus 帧内容错误")
	}
}

func TestFramerModbusResync(t *testing.T) {
	f := newFramer(&model.Product{AccessMode: model.AccessModeModbus})
	resp := buildModbusReadResp(1, 3, []byte{0x00, 0x0A})
	// 前面混入 1 字节噪声，framer 应重新同步并切出完整帧
	frames := collect(f, append([]byte{0xFF}, resp...))
	if len(frames) != 1 || !bytes.Equal(frames[0], resp) {
		t.Fatalf("Modbus 重新同步失败, got %q", frames)
	}
}

// buildModbusReadResp 构造读应答帧（含 CRC）用于测试
func buildModbusReadResp(slave, fn byte, data []byte) []byte {
	frame := []byte{slave, fn, byte(len(data))}
	frame = append(frame, data...)
	crc := crc16(frame)
	return append(frame, byte(crc&0xFF), byte(crc>>8))
}

// crc16 与 modbus 包一致的实现，避免测试跨包耦合
func crc16(data []byte) uint16 {
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
