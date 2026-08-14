package service

import (
	"hash/fnv"
	"log/slog"
	"sync/atomic"
)

// 遥测处理 worker 池：MQTT/HTTP 入口原先每条消息 `go HandleTelemetry(...)`，
// DB 抖动时消费速率骤降而到达速率不变，无界 goroutine 会堆积直至 OOM。
// 这里改为按 (productKey+deviceName) 哈希分片的有界队列 + 固定 worker：
//   - 同一设备的消息总是路由到同一队列，保持单设备内处理顺序；
//   - 队列总容量有界，满时丢弃并计数（遥测可降级，好过拖垮整个进程）；
//   - TCP 网关路径仍同步调用 HandleTelemetry（读循环天然背压），不经此池。
type telemetryJob struct {
	productKey string
	deviceName string
	payload    []byte
}

var (
	telemetryQueues []chan telemetryJob
	telemetryDropped atomic.Uint64
)

// InitTelemetryWorkers 启动遥测处理 worker 池（workers 个队列，每队列 queueCap 容量）
func InitTelemetryWorkers(workers, queueCap int) {
	if workers <= 0 {
		workers = 64
	}
	if queueCap <= 0 {
		queueCap = 512
	}
	telemetryQueues = make([]chan telemetryJob, workers)
	for i := range telemetryQueues {
		telemetryQueues[i] = make(chan telemetryJob, queueCap)
		go telemetryWorker(telemetryQueues[i])
	}
	slog.Info("telemetry worker pool started", "workers", workers, "queueCap", queueCap)
}

func telemetryWorker(ch <-chan telemetryJob) {
	for j := range ch {
		HandleTelemetry(j.productKey, j.deviceName, j.payload)
	}
}

// SubmitTelemetry 提交遥测到 worker 池（非阻塞；满载丢弃并告警计数）
func SubmitTelemetry(productKey, deviceName string, payload []byte) {
	if telemetryQueues == nil {
		// 未初始化（测试/开发）退化为直接处理
		HandleTelemetry(productKey, deviceName, payload)
		return
	}
	h := fnv.New32a()
	h.Write([]byte(productKey))
	h.Write([]byte(deviceName))
	idx := h.Sum32() % uint32(len(telemetryQueues))
	select {
	case telemetryQueues[idx] <- telemetryJob{productKey: productKey, deviceName: deviceName, payload: payload}:
	default:
		n := telemetryDropped.Add(1)
		if n%1000 == 1 { // 告警限频，避免丢弃风暴时刷爆日志
			slog.Warn("telemetry queue full, dropping messages", "droppedTotal", n)
		}
	}
}
