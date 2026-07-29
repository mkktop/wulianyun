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

const execTimeout = 200 * time.Millisecond

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

// Decode 上行解码：bytes -> 属性 map
func Decode(productID uint, script string, data []byte) (map[string]interface{}, error) {
	vm, err := newVM(productID, script)
	if err != nil {
		return nil, err
	}
	fn, ok := goja.AssertFunction(vm.Get("decode"))
	if !ok {
		return nil, fmt.Errorf("脚本缺少 decode 函数")
	}
	// []byte -> JS 数组
	arr := make([]interface{}, len(data))
	for i, b := range data {
		arr[i] = int(b)
	}
	timer := time.AfterFunc(execTimeout, func() { vm.Interrupt("执行超时") })
	defer timer.Stop()
	res, err := fn(goja.Undefined(), vm.ToValue(arr))
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
func Encode(productID uint, script string, params map[string]interface{}) ([]byte, bool, error) {
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
	out := make([]byte, len(raw))
	for i, v := range raw {
		n, ok := v.(int64)
		if !ok || n < 0 || n > 255 {
			return nil, false, fmt.Errorf("encode 返回值必须是 0-255 的整数数组")
		}
		out[i] = byte(n)
	}
	return out, true, nil
}
