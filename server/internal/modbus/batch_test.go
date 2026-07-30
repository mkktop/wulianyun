package modbus

import (
	"testing"

	"iot-platform/internal/model"
)

func p(id string, addr uint16, raw string) *model.ModbusPoint {
	return &model.ModbusPoint{
		Identifier: id, SlaveID: 1, FunctionCode: model.FuncReadHoldingRegisters,
		Address: addr, RawType: raw, Scale: 1,
	}
}

func TestPlanReadBlocks_MergeContiguous(t *testing.T) {
	// 地址 0,1,2 三个连续 uint16 → 合并为 1 个块，quantity=3
	pts := []*model.ModbusPoint{p("a", 0, "uint16"), p("b", 1, "uint16"), p("c", 2, "uint16")}
	blocks := PlanReadBlocks(pts)
	if len(blocks) != 1 {
		t.Fatalf("期望合并为 1 个块，实际 %d", len(blocks))
	}
	if blocks[0].Start != 0 || blocks[0].Quantity != 3 {
		t.Fatalf("块范围错误: start=%d qty=%d", blocks[0].Start, blocks[0].Quantity)
	}
}

func TestPlanReadBlocks_GapWithinThreshold(t *testing.T) {
	// 地址 0 和 5，间隔 4 <= maxGap(8) → 仍合并为一块 (0..5, qty=6)
	pts := []*model.ModbusPoint{p("a", 0, "uint16"), p("b", 5, "uint16")}
	blocks := PlanReadBlocks(pts)
	if len(blocks) != 1 || blocks[0].Quantity != 6 {
		t.Fatalf("间隔内应合并: blocks=%d qty=%d", len(blocks), blocks[0].Quantity)
	}
}

func TestPlanReadBlocks_GapExceedsThreshold(t *testing.T) {
	// 地址 0 和 100，间隔远超阈值 → 拆成 2 个块
	pts := []*model.ModbusPoint{p("a", 0, "uint16"), p("b", 100, "uint16")}
	blocks := PlanReadBlocks(pts)
	if len(blocks) != 2 {
		t.Fatalf("超阈值应拆分: blocks=%d", len(blocks))
	}
}

func TestPlanReadBlocks_DifferentSlaveOrFunc(t *testing.T) {
	// 不同从机不合并
	a := p("a", 0, "uint16")
	b := p("b", 1, "uint16")
	b.SlaveID = 2
	blocks := PlanReadBlocks([]*model.ModbusPoint{a, b})
	if len(blocks) != 2 {
		t.Fatalf("不同从机应拆分: blocks=%d", len(blocks))
	}
}

func TestBuildAndParseBlockResponse(t *testing.T) {
	// 温度(addr0,uint16,x0.1) + 湿度(addr1,uint16,x0.1) 合并读
	temp := &model.ModbusPoint{Identifier: "temp", SlaveID: 1, FunctionCode: 3, Address: 0, RawType: "uint16", Scale: 0.1}
	hum := &model.ModbusPoint{Identifier: "hum", SlaveID: 1, FunctionCode: 3, Address: 1, RawType: "uint16", Scale: 0.1}
	blocks := PlanReadBlocks([]*model.ModbusPoint{temp, hum})
	if len(blocks) != 1 {
		t.Fatalf("应合并为一块")
	}
	req := BuildBlockRequest(blocks[0])
	// 01 03 0000 0002 CRC
	if req[0] != 0x01 || req[1] != 0x03 || req[5] != 0x02 {
		t.Fatalf("批量读请求错误: %X", req)
	}
	// 构造应答: 01 03 04 00FA 0226 CRC  → temp=25.0, hum=55.0
	body := []byte{0x01, 0x03, 0x04, 0x00, 0xFA, 0x02, 0x26}
	resp := appendCRC(body)
	vals, err := ParseBlockResponse(blocks[0], resp)
	if err != nil {
		t.Fatal(err)
	}
	if vals["temp"] < 24.99 || vals["temp"] > 25.01 {
		t.Fatalf("temp 解析错误: %f", vals["temp"])
	}
	if vals["hum"] < 54.99 || vals["hum"] > 55.01 {
		t.Fatalf("hum 解析错误: %f", vals["hum"])
	}
}

func TestParseBlockResponse_Int32InMiddle(t *testing.T) {
	// addr0 uint16 + addr1..2 int32(x1) 合并，验证偏移定位正确
	a := &model.ModbusPoint{Identifier: "a", SlaveID: 1, FunctionCode: 3, Address: 0, RawType: "uint16", Scale: 1}
	b := &model.ModbusPoint{Identifier: "b", SlaveID: 1, FunctionCode: 3, Address: 1, RawType: "int32", Scale: 1}
	blocks := PlanReadBlocks([]*model.ModbusPoint{a, b})
	if len(blocks) != 1 || blocks[0].Quantity != 3 {
		t.Fatalf("应合并 qty=3: %+v", blocks[0])
	}
	// 应答 3 个寄存器 = 6 字节: a=0x0007, b=0x00000009
	body := []byte{0x01, 0x03, 0x06, 0x00, 0x07, 0x00, 0x00, 0x00, 0x09}
	resp := appendCRC(body)
	vals, err := ParseBlockResponse(blocks[0], resp)
	if err != nil {
		t.Fatal(err)
	}
	if vals["a"] != 7 {
		t.Fatalf("a 错误: %f", vals["a"])
	}
	if vals["b"] != 9 {
		t.Fatalf("b 错误: %f", vals["b"])
	}
}
