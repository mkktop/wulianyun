package codec

import (
	"strings"
	"testing"
)

const goodScript = `
function decode(bytes) { return {temperature: bytes[0] / 10} }
function encode(obj)   { return [obj.temperature * 10] }
`

// TestValidate 坏脚本拒绝；好脚本通过且可复用缓存。
func TestValidate(t *testing.T) {
	if err := Validate(1, "function decode("); err == nil {
		t.Error("语法错误的脚本应编译失败")
	}
	if err := Validate(1, goodScript); err != nil {
		t.Errorf("合法脚本应通过: %v", err)
	}
}

// TestDecodeInputCap 超长报文直接拒绝（防 16× 装箱放大 OOM）。
func TestDecodeInputCap(t *testing.T) {
	big := make([]byte, maxInputLen+1)
	_, err := Decode(2, goodScript, big)
	if err == nil || !strings.Contains(err.Error(), "过大") {
		t.Errorf("超长报文应被拒绝，得到 %v", err)
	}
	// 测试入口同样受限
	_, err = TestDecode(goodScript, big)
	if err == nil || !strings.Contains(err.Error(), "过大") {
		t.Errorf("TestDecode 超长报文应被拒绝，得到 %v", err)
	}
}

// TestScriptCap 超大脚本拒绝（防编译卡死全局解码）。
func TestScriptCap(t *testing.T) {
	huge := "function decode(bytes){return {}}\n" + strings.Repeat("var x=1;\n", maxScriptLen)
	if err := Validate(3, huge); err == nil {
		t.Error("超大脚本应被拒绝")
	}
	if _, err := TestDecode(huge, []byte{1}); err == nil {
		t.Error("TestDecode 超大脚本应被拒绝")
	}
}

// TestTestDecodeNoCachePollution 测试解析不应污染产品缓存：坏测试脚本之后，线上好脚本仍正常。
func TestTestDecodeNoCachePollution(t *testing.T) {
	if _, err := TestDecode("function decode(broken", []byte{1, 2}); err == nil {
		t.Fatal("坏测试脚本应解析失败")
	}
	// 同一产品 ID 的线上解析不受测试脚本影响
	obj, err := Decode(9, goodScript, []byte{255})
	if err != nil {
		t.Fatalf("线上解析失败: %v", err)
	}
	if v, _ := obj["temperature"].(float64); v != 25.5 {
		t.Errorf("temperature = %v, want 25.5", obj["temperature"])
	}
}

// TestDecodeRoundTrip 正常解析与编码。
func TestDecodeRoundTrip(t *testing.T) {
	obj, err := Decode(10, goodScript, []byte{100})
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	// JS 整数结果（100/10=10）导出为 int64，浮点结果（255/10=25.5）导出为 float64
	var got float64
	switch v := obj["temperature"].(type) {
	case float64:
		got = v
	case int64:
		got = float64(v)
	}
	if got != 10 {
		t.Errorf("temperature = %v, want 10", obj["temperature"])
	}
	out, ok, err := Encode(10, goodScript, map[string]interface{}{"temperature": 25.5})
	if err != nil || !ok {
		t.Fatalf("encode: ok=%v err=%v", ok, err)
	}
	if len(out) != 1 || out[0] != 255 {
		t.Errorf("encode out = %v, want [255]", out)
	}
}
