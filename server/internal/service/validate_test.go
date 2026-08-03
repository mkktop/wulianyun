package service

import (
	"testing"
)

// def 构造属性定义：键名与存储/导入一致（dataType）
func def(dataType string, extra map[string]interface{}) map[string]interface{} {
	m := map[string]interface{}{"dataType": dataType}
	for k, v := range extra {
		m[k] = v
	}
	return m
}

func TestValidateField_Int32(t *testing.T) {
	d := def("int32", map[string]interface{}{"min": float64(0), "max": float64(100)})
	if errs := validateField("temp", float64(50), d); len(errs) != 0 {
		t.Fatalf("valid int32 should pass, got %v", errs)
	}
	if errs := validateField("temp", "abc", d); len(errs) == 0 {
		t.Fatal("non-numeric int32 should fail")
	}
	if errs := validateField("temp", float64(-1), d); len(errs) == 0 {
		t.Fatal("below min should fail")
	}
	if errs := validateField("temp", float64(101), d); len(errs) == 0 {
		t.Fatal("above max should fail")
	}
}

func TestValidateField_Float(t *testing.T) {
	d := def("float", map[string]interface{}{"min": float64(-40), "max": float64(80)})
	if errs := validateField("v", float64(25.5), d); len(errs) != 0 {
		t.Fatalf("valid float should pass, got %v", errs)
	}
	if errs := validateField("v", float64(81), d); len(errs) == 0 {
		t.Fatal("above max should fail")
	}
}

func TestValidateField_Bool(t *testing.T) {
	d := def("bool", nil)
	if errs := validateField("sw", true, d); len(errs) != 0 {
		t.Fatalf("true should pass, got %v", errs)
	}
	if errs := validateField("sw", float64(1), d); len(errs) != 0 {
		t.Fatalf("1 (0/1) should pass, got %v", errs)
	}
	if errs := validateField("sw", "on", d); len(errs) == 0 {
		t.Fatal("string should fail for bool")
	}
}

func TestValidateField_Enum(t *testing.T) {
	d := def("enum", map[string]interface{}{
		"enumSpec": []interface{}{
			map[string]interface{}{"value": float64(0), "label": "normal"},
			map[string]interface{}{"value": float64(1), "label": "eco"},
		},
	})
	if errs := validateField("mode", float64(1), d); len(errs) != 0 {
		t.Fatalf("valid enum should pass, got %v", errs)
	}
	if errs := validateField("mode", float64(9), d); len(errs) == 0 {
		t.Fatal("value not in enum should fail")
	}
}

func TestValidateField_Text(t *testing.T) {
	d := def("text", nil)
	if errs := validateField("name", "hello", d); len(errs) != 0 {
		t.Fatalf("string should pass, got %v", errs)
	}
	if errs := validateField("name", float64(1), d); len(errs) == 0 {
		t.Fatal("non-string should fail for text")
	}
}

func TestValidateField_Date(t *testing.T) {
	d := def("date", nil)
	if errs := validateField("ts", float64(1785712345678), d); len(errs) != 0 {
		t.Fatalf("numeric timestamp should pass, got %v", errs)
	}
	if errs := validateField("ts", "2026-01-01", d); len(errs) == 0 {
		t.Fatal("string should fail for date")
	}
}

// 回归：定义的键名必须是 dataType（曾误读为 type 导致校验全部旁路）
func TestValidateField_DataTypeKey(t *testing.T) {
	d := def("text", nil)
	if errs := validateField("name", float64(1), d); len(errs) == 0 {
		t.Fatal("校验必须按 dataType 键读取定义，否则 type 检查形同虚设")
	}
}