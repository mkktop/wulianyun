// Package poller Modbus 云端轮询引擎：设备（DTU）通过 9100 长连接主动接入后，
// 平台按产品采集周期在该长连接上下发 Modbus RTU 读请求，解析应答为物模型属性并入库。
package poller

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"iot-platform/internal/gateway"
	"iot-platform/internal/model"
	"iot-platform/internal/modbus"
	"iot-platform/internal/repository"
	"iot-platform/internal/service"
)

const requestTimeout = 3 * time.Second

type task struct {
	cancel chan struct{}
}

var (
	mu    sync.Mutex
	tasks = map[string]*task{} // key: productKey.deviceName
)

// Init 注册到 gateway 的上下线钩子
func Init() {
	gateway.OnDeviceConnect = onConnect
	gateway.OnDeviceDisconnect = onDisconnect
}

func onConnect(productKey, deviceName string, productID uint) {
	var p model.Product
	if err := repository.DB.First(&p, productID).Error; err != nil {
		return
	}
	if p.AccessMode != model.AccessModeModbus {
		return
	}
	startDevice(&p, deviceName)
}

func onDisconnect(productKey, deviceName string) {
	key := productKey + "." + deviceName
	mu.Lock()
	if t, ok := tasks[key]; ok {
		close(t.cancel)
		delete(tasks, key)
	}
	mu.Unlock()
}

func startDevice(p *model.Product, deviceName string) {
	key := p.ProductKey + "." + deviceName
	interval := time.Duration(p.PollInterval) * time.Second
	if interval < 60*time.Second {
		interval = 60 * time.Second
	}
	cancel := make(chan struct{})
	mu.Lock()
	if old, ok := tasks[key]; ok {
		close(old.cancel)
	}
	tasks[key] = &task{cancel: cancel}
	mu.Unlock()

	go func() {
		// 上线立即采集一次，随后按周期轮询
		pollOnce(p, deviceName)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-cancel:
				return
			case <-ticker.C:
				pollOnce(p, deviceName)
			}
		}
	}()
	slog.Info("modbus poller started", "device", key, "interval", interval.String())
}

// pollOnce 遍历读点位，逐条下发请求并解析，聚合为一条遥测
func pollOnce(p *model.Product, deviceName string) {
	var points []model.ModbusPoint
	repository.DB.Where("product_id = ?", p.ID).Order("id asc").Find(&points)
	if len(points) == 0 {
		return
	}
	data := map[string]interface{}{}
	for i := range points {
		pt := &points[i]
		// 仅轮询读功能码(1-4)；纯写点位跳过
		if pt.FunctionCode < model.FuncReadCoils || pt.FunctionCode > model.FuncReadInputRegisters {
			continue
		}
		req, err := modbus.BuildReadRequest(pt)
		if err != nil {
			continue
		}
		resp, err := gateway.Request(p.ProductKey, deviceName, req, requestTimeout)
		if err != nil {
			slog.Warn("modbus poll failed", "device", deviceName, "point", pt.Identifier, "err", err)
			continue
		}
		v, err := modbus.ParseResponse(pt, resp)
		if err != nil {
			slog.Warn("modbus parse failed", "device", deviceName, "point", pt.Identifier, "err", err)
			continue
		}
		data[pt.Identifier] = round2(v)
	}
	if len(data) > 0 {
		payload, _ := json.Marshal(data)
		service.HandleTelemetry(p.ProductKey, deviceName, payload)
	}
}

// WriteProperty 写控制：解析 property.set 载荷，按点位表写寄存器/线圈
// payload 形如 {"method":"property.set","params":{"switch":1}} 或直接 {"switch":1}
func WriteProperty(productKey, deviceName string, payload []byte) error {
	var msg struct {
		Params map[string]interface{} `json:"params"`
	}
	json.Unmarshal(payload, &msg)
	params := msg.Params
	if len(params) == 0 {
		json.Unmarshal(payload, &params)
	}
	if len(params) == 0 {
		return fmt.Errorf("无可写参数")
	}

	var p model.Product
	if err := repository.DB.Where("product_key = ?", productKey).First(&p).Error; err != nil {
		return err
	}
	var points []model.ModbusPoint
	repository.DB.Where("product_id = ?", p.ID).Find(&points)
	byID := map[string]*model.ModbusPoint{}
	for i := range points {
		byID[points[i].Identifier] = &points[i]
	}

	var lastErr error
	wrote := 0
	for k, val := range params {
		pt, ok := byID[k]
		if !ok || pt.AccessMode != "rw" {
			continue
		}
		writeFn := writeFuncFor(pt.FunctionCode)
		if writeFn == 0 {
			lastErr = fmt.Errorf("点位 %s 不可写", k)
			continue
		}
		wp := *pt
		wp.FunctionCode = writeFn
		frame, err := modbus.BuildWriteRequest(&wp, toFloat(val))
		if err != nil {
			lastErr = err
			continue
		}
		// 写操作也等待应答（回显帧），失败不阻塞其他点位
		if _, err := gateway.Request(productKey, deviceName, frame, requestTimeout); err != nil {
			lastErr = err
			continue
		}
		wrote++
	}
	if wrote == 0 && lastErr != nil {
		return lastErr
	}
	return nil
}

// writeFuncFor 由读功能码推导写功能码；离散量输入/输入寄存器只读返回 0
func writeFuncFor(fn int) int {
	switch fn {
	case model.FuncReadCoils:
		return model.FuncWriteSingleCoil
	case model.FuncReadHoldingRegisters:
		return model.FuncWriteSingleRegister
	case model.FuncWriteSingleCoil, model.FuncWriteSingleRegister,
		model.FuncWriteMultipleCoils, model.FuncWriteMultipleRegs:
		return fn
	}
	return 0
}

func toFloat(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case bool:
		if n {
			return 1
		}
		return 0
	case string:
		f, _ := strconv.ParseFloat(n, 64)
		return f
	}
	return 0
}

func round2(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}
