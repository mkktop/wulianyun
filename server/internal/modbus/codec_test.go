package modbus

import (
	"testing"

	"iot-platform/internal/model"
)

func TestCRC16(t *testing.T) {
	// 已知向量：01 03 00 00 00 01 -> CRC 0x0A84 (低字节 0x84 高字节 0x0A)
	frame := []byte{0x01, 0x03, 0x00, 0x00, 0x00, 0x01}
	crc := CRC16(frame)
	if byte(crc&0xFF) != 0x84 || byte(crc>>8) != 0x0A {
		t.Fatalf("CRC16 错误: got %04X, 期望低0x84 高0x0A", crc)
	}
}

func TestBuildReadRequest(t *testing.T) {
	p := &model.ModbusPoint{SlaveID: 1, FunctionCode: model.FuncReadHoldingRegisters, Address: 0, RawType: "int16"}
	frame, err := BuildReadRequest(p)
	if err != nil {
		t.Fatal(err)
	}
	// 期望: 01 03 00 00 00 01 84 0A
	expect := []byte{0x01, 0x03, 0x00, 0x00, 0x00, 0x01, 0x84, 0x0A}
	if len(frame) != len(expect) {
		t.Fatalf("帧长度错误: got %d 期望 %d", len(frame), len(expect))
	}
	for i := range expect {
		if frame[i] != expect[i] {
			t.Fatalf("帧字节[%d]错误: got %02X 期望 %02X", i, frame[i], expect[i])
		}
	}
}

func TestParseResponse_Uint16Scale(t *testing.T) {
	// 温度点位：保持寄存器，uint16，缩放 0.1
	p := &model.ModbusPoint{SlaveID: 1, FunctionCode: model.FuncReadHoldingRegisters, RawType: "uint16", Scale: 0.1}
	// 应答: 01 03 02 00FA CRC，0x00FA=250 -> 250*0.1=25.0
	body := []byte{0x01, 0x03, 0x02, 0x00, 0xFA}
	frame := appendCRC(body)
	v, err := ParseResponse(p, frame)
	if err != nil {
		t.Fatal(err)
	}
	if v < 24.99 || v > 25.01 {
		t.Fatalf("解析值错误: got %f 期望 25.0", v)
	}
}

func TestParseResponse_Int16Negative(t *testing.T) {
	p := &model.ModbusPoint{SlaveID: 1, FunctionCode: model.FuncReadHoldingRegisters, RawType: "int16", Scale: 1}
	// 0xFFFF = -1
	body := []byte{0x01, 0x03, 0x02, 0xFF, 0xFF}
	frame := appendCRC(body)
	v, err := ParseResponse(p, frame)
	if err != nil {
		t.Fatal(err)
	}
	if v != -1 {
		t.Fatalf("负数解析错误: got %f 期望 -1", v)
	}
}

func TestParseResponse_Float32(t *testing.T) {
	p := &model.ModbusPoint{SlaveID: 1, FunctionCode: model.FuncReadInputRegisters, RawType: "float", Scale: 1}
	// float32 25.5 = 0x41CC0000
	body := []byte{0x01, 0x04, 0x04, 0x41, 0xCC, 0x00, 0x00}
	frame := appendCRC(body)
	v, err := ParseResponse(p, frame)
	if err != nil {
		t.Fatal(err)
	}
	if v < 25.49 || v > 25.51 {
		t.Fatalf("float 解析错误: got %f 期望 25.5", v)
	}
}

func TestParseResponse_SwapWord(t *testing.T) {
	// int32 字序互换：原始 0x0001 0x0000 互换后为 0x0000 0x0001 = 1
	p := &model.ModbusPoint{SlaveID: 1, FunctionCode: model.FuncReadHoldingRegisters, RawType: "int32", Scale: 1, SwapWord: true}
	body := []byte{0x01, 0x03, 0x04, 0x00, 0x01, 0x00, 0x00}
	frame := appendCRC(body)
	v, err := ParseResponse(p, frame)
	if err != nil {
		t.Fatal(err)
	}
	if v != 1 {
		t.Fatalf("字序互换解析错误: got %f 期望 1", v)
	}
}

func TestParseResponse_CRCError(t *testing.T) {
	p := &model.ModbusPoint{SlaveID: 1, FunctionCode: model.FuncReadHoldingRegisters, RawType: "uint16", Scale: 1}
	frame := []byte{0x01, 0x03, 0x02, 0x00, 0xFA, 0x00, 0x00} // 错误 CRC
	if _, err := ParseResponse(p, frame); err == nil {
		t.Fatal("期望 CRC 校验失败，但通过了")
	}
}

func TestBuildWriteSingleRegister(t *testing.T) {
	p := &model.ModbusPoint{SlaveID: 1, FunctionCode: model.FuncWriteSingleRegister, Address: 0x10, RawType: "uint16", Scale: 1}
	frame, err := BuildWriteRequest(p, 100)
	if err != nil {
		t.Fatal(err)
	}
	// 01 06 0010 0064 CRC
	if frame[0] != 0x01 || frame[1] != 0x06 || frame[4] != 0x00 || frame[5] != 0x64 {
		t.Fatalf("写寄存器帧错误: %X", frame)
	}
	if !checkCRC(frame) {
		t.Fatal("写帧 CRC 无效")
	}
}

func TestBuildWriteSingleCoil(t *testing.T) {
	p := &model.ModbusPoint{SlaveID: 1, FunctionCode: model.FuncWriteSingleCoil, Address: 0x00, RawType: "bool", Scale: 1}
	frame, err := BuildWriteRequest(p, 1)
	if err != nil {
		t.Fatal(err)
	}
	// 01 05 0000 FF00 CRC
	if frame[1] != 0x05 || frame[4] != 0xFF || frame[5] != 0x00 {
		t.Fatalf("写线圈帧错误: %X", frame)
	}
}
