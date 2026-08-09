package service

import (
	"log/slog"
	"sync"
	"time"

	"iot-platform/internal/model"
	"iot-platform/internal/repository"
)

// logBuffer 轨迹/设备日志批量化写入，避免每条遥测各 1 次 INSERT 抢占连接池（#17）。
// 与 TelemetryBuffer 同构：有界批量 worker + flush 失败重入队。
type logBuffer struct {
	mu            sync.Mutex
	traces        []model.MessageTrace
	logs          []model.DeviceLog
	maxBatch      int
	flushInterval time.Duration
	quit          chan struct{}
	done          chan struct{}
}

var logBuf *logBuffer

// InitLogBuffer 启动日志批量化 worker
func InitLogBuffer(maxBatch, intervalSeconds int) {
	if maxBatch <= 0 {
		maxBatch = 200
	}
	if intervalSeconds <= 0 {
		intervalSeconds = 2
	}
	logBuf = &logBuffer{
		maxBatch:      maxBatch,
		flushInterval: time.Duration(intervalSeconds) * time.Second,
		quit:          make(chan struct{}),
		done:          make(chan struct{}),
	}
	go logBuf.flushLoop()
}

func (b *logBuffer) flushLoop() {
	defer close(b.done)
	ticker := time.NewTicker(b.flushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			b.flush()
		case <-b.quit:
			b.flush()
			return
		}
	}
}

const logBufferedFactor = 10

func (b *logBuffer) flush() {
	b.mu.Lock()
	if len(b.traces) == 0 && len(b.logs) == 0 {
		b.mu.Unlock()
		return
	}
	traces := b.traces
	logs := b.logs
	b.traces = make([]model.MessageTrace, 0, b.maxBatch)
	b.logs = make([]model.DeviceLog, 0, b.maxBatch)
	b.mu.Unlock()

	cap := b.maxBatch * logBufferedFactor
	if len(traces) > 0 {
		if err := repository.DB.CreateInBatches(traces, b.maxBatch).Error; err != nil {
			slog.Error("batch trace flush failed", "count", len(traces), "error", err)
			b.mu.Lock()
			b.traces = append(traces, b.traces...)
			if len(b.traces) > cap {
				b.traces = b.traces[len(b.traces)-cap:]
			}
			b.mu.Unlock()
		}
	}
	if len(logs) > 0 {
		if err := repository.DB.CreateInBatches(logs, b.maxBatch).Error; err != nil {
			slog.Error("batch device-log flush failed", "count", len(logs), "error", err)
			b.mu.Lock()
			b.logs = append(logs, b.logs...)
			if len(b.logs) > cap {
				b.logs = b.logs[len(b.logs)-cap:]
			}
			b.mu.Unlock()
		}
	}
}

func (b *logBuffer) pushTrace(t model.MessageTrace) {
	b.mu.Lock()
	b.traces = append(b.traces, t)
	full := len(b.traces) >= b.maxBatch
	b.mu.Unlock()
	if full {
		b.flush()
	}
}

func (b *logBuffer) pushLog(l model.DeviceLog) {
	b.mu.Lock()
	b.logs = append(b.logs, l)
	full := len(b.logs) >= b.maxBatch
	b.mu.Unlock()
	if full {
		b.flush()
	}
}

// ShutdownLogBuffer 停止日志缓冲并等待最后一次 flush 完成
func ShutdownLogBuffer() {
	if logBuf == nil {
		return
	}
	close(logBuf.quit)
	select {
	case <-logBuf.done:
	case <-time.After(10 * time.Second):
		slog.Warn("log buffer shutdown timeout")
	}
}
