// Package codec 自定义协议解析：产品级 JS 脚本，把二进制报文与物模型 JSON 互转
//
// 脚本契约：
//
//	function decode(bytes) { return {temperature: 25.5} }  // 上行：字节数组 -> 属性对象
//	function encode(obj)   { return [0x01, 0x02] }         // 下行(可选)：对象 -> 字节数组
package codec

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/dop251/goja"
)

type cached struct {
	hash string
	prog *goja.Program
}

var (
	mu    sync.Mutex
	cache = map[uint]*cached{}
)

const (
	execTimeout  = 200 * time.Millisecond
	maxScriptLen = 64 << 10 // 脚本上限 64KB（防超大脚本编译卡死/内存放大）
	maxInputLen  = 64 << 10 // 解码输入上限 64KB（防 16× 装箱放大 OOM）
)

func getProgram(productID uint, script string) (*goja.Program, error) {
	h := sha256.Sum256([]byte(script))
	hash := hex.EncodeToString(h[:8])

	mu.Lock()
	defer mu.Unlock()
	if c, ok := cache[productID]; ok && c.hash == hash {
		return c.prog, nil
	}
	prog, err := goja.Compile("codec.js", script, true)
	if err != nil {
		return nil, fmt.Errorf("脚本编译失败: %w", err)
	}
	cache[productID] = &cached{hash: hash, prog: prog}
	return prog, nil
}

func newVM(productID uint, script string) (*goja.Runtime, error) {
	prog, err := getProgram(productID, script)
	if err != nil {
		return nil, err
	}
	vm := goja.New()
	timer := time.AfterFunc(execTimeout, func() { vm.Interrupt("执行超时") })
	defer timer.Stop()
	if _, err := vm.RunProgram(prog); err != nil {
		return nil, fmt.Errorf("脚本执行失败: %w", err)
	}
	return vm, nil
}

// Validate 编译校验脚本（SaveCodec 保存前调用，坏脚本拒绝落库）。
// 编译结果按 productID 进入缓存，线上解析直接复用。
func Validate(productID uint, script string) error {
	if len(script) > maxScriptLen {
		return fmt.Errorf("脚本过大（上限 %dKB）", maxScriptLen>>10)
	}
	_, err := getProgram(productID, script)
	return err
}

// bytesToJSArr 把字节数组转换为 JS 数组（避免 goja 对 []byte 的映射差异）
func bytesToJSArr(data []byte) []interface{} {
	arr := make([]interface{}, len(data))
	for i, b := range data {
		arr[i] = int(b)
	}
	return arr
}

// Decode 上行解码：bytes -> 属性 map
// 带 recover：TCP 透传路径不经 gin.Recovery，goja 异常 panic 会崩整个进程
func Decode(productID uint, script string, data []byte) (out map[string]interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			out, err = nil, fmt.Errorf("decode 脚本异常: %v", r)
		}
	}()
	if len(data) > maxInputLen {
		return nil, fmt.Errorf("报文过大（上限 %dKB）", maxInputLen>>10)
	}
	vm, err := newVM(productID, script)
	if err != nil {
		return nil, err
	}
	fn, ok := goja.AssertFunction(vm.Get("decode"))
	if !ok {
		return nil, fmt.Errorf("脚本缺少 decode 函数")
	}
	timer := time.AfterFunc(execTimeout, func() { vm.Interrupt("执行超时") })
	defer timer.Stop()
	res, err := fn(goja.Undefined(), vm.ToValue(bytesToJSArr(data)))
	if err != nil {
		return nil, fmt.Errorf("decode 调用失败: %w", err)
	}
	obj, ok := res.Export().(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("decode 必须返回对象")
	}
	return obj, nil
}

// TestDecode 测试解析：给定脚本直接编译执行，不进产品缓存
// （TestCodec 等测试入口传入的脚本不应污染线上产品缓存）
func TestDecode(script string, data []byte) (out map[string]interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			out, err = nil, fmt.Errorf("decode 脚本异常: %v", r)
		}
	}()
	if len(script) > maxScriptLen {
		return nil, fmt.Errorf("脚本过大（上限 %dKB）", maxScriptLen>>10)
	}
	if len(data) > maxInputLen {
		return nil, fmt.Errorf("报文过大（上限 %dKB）", maxInputLen>>10)
	}
	prog, err := goja.Compile("codec.js", script, true)
	if err != nil {
		return nil, fmt.Errorf("脚本编译失败: %w", err)
	}
	vm := goja.New()
	timer := time.AfterFunc(execTimeout, func() { vm.Interrupt("执行超时") })
	defer timer.Stop()
	if _, err := vm.RunProgram(prog); err != nil {
		return nil, fmt.Errorf("脚本执行失败: %w", err)
	}
	fn, ok := goja.AssertFunction(vm.Get("decode"))
	if !ok {
		return nil, fmt.Errorf("脚本缺少 decode 函数")
	}
	res, err := fn(goja.Undefined(), vm.ToValue(bytesToJSArr(data)))
	if err != nil {
		return nil, fmt.Errorf("decode 调用失败: %w", err)
	}
	obj, ok := res.Export().(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("decode 必须返回对象")
	}
	return obj, nil
}

// Encode 下行编码：属性 map -> bytes；脚本无 encode 函数时返回 (nil, false, nil)
func Encode(productID uint, script string, params map[string]interface{}) (out []byte, ok bool, err error) {
	defer func() {
		if r := recover(); r != nil {
			out, ok, err = nil, false, fmt.Errorf("encode 脚本异常: %v", r)
		}
	}()
	vm, err := newVM(productID, script)
	if err != nil {
		return nil, false, err
	}
	fn, ok := goja.AssertFunction(vm.Get("encode"))
	if !ok {
		return nil, false, nil
	}
	timer := time.AfterFunc(execTimeout, func() { vm.Interrupt("执行超时") })
	defer timer.Stop()
	res, err := fn(goja.Undefined(), vm.ToValue(params))
	if err != nil {
		return nil, false, fmt.Errorf("encode 调用失败: %w", err)
	}
	raw, ok := res.Export().([]interface{})
	if !ok {
		return nil, false, fmt.Errorf("encode 必须返回字节数组")
	}
	out = make([]byte, len(raw))
	for i, v := range raw {
		n, ok := v.(int64)
		if !ok || n < 0 || n > 255 {
			return nil, false, fmt.Errorf("encode 返回值必须是 0-255 的整数数组")
		}
		out[i] = byte(n)
	}
	return out, true, nil
}
