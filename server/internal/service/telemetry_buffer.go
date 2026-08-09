package service

import (
	"log/slog"
	"sync"
	"time"

	"iot-platform/internal/model"
	"iot-platform/internal/repository"
)

type TelemetryBuffer struct {
	mu            sync.Mutex
	items         []model.Telemetry
	maxBatch      int
	flushInterval time.Duration
	quit          chan struct{}
	done          chan struct{} // flushLoop 退出（含最后一次 flush 完成）信号
}

var telBuf *TelemetryBuffer

// maxBuffered 缓冲上限（maxBatch 的倍数）：DB 故障期间重试队列最多保留这些条，
// 超出部分丢弃（防止内存无限膨胀），DB 恢复后继续 flush
const maxBufferedFactor = 10

func InitTelemetryBuffer(maxBatch int, intervalSeconds int) {
	if maxBatch <= 0 {
		maxBatch = 100
	}
	if intervalSeconds <= 0 {
		intervalSeconds = 1
	}
	telBuf = &TelemetryBuffer{
		items:         make([]model.Telemetry, 0, maxBatch),
		maxBatch:      maxBatch,
		flushInterval: time.Duration(intervalSeconds) * time.Second,
		quit:          make(chan struct{}),
		done:          make(chan struct{}),
	}
	go telBuf.flushLoop()
}

func (b *TelemetryBuffer) flushLoop() {
	defer close(b.done)
	ticker := time.NewTicker(b.flushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			b.flush()
		case <-b.quit:
			b.flush() // 最后一次 flush，完成后经 done 通知调用方
			return
		}
	}
}

func (b *TelemetryBuffer) flush() {
	b.mu.Lock()
	if len(b.items) == 0 {
		b.mu.Unlock()
		return
	}
	batch := b.items
	b.items = make([]model.Telemetry, 0, b.maxBatch)
	b.mu.Unlock()

	if err := repository.DB.Create(&batch).Error; err != nil {
		slog.Error("batch telemetry flush failed", "count", len(batch), "error", err)
		// 整批放回队首重试（下次 flush 再试）；超出缓冲上限时丢弃最旧部分，防止内存膨胀
		b.mu.Lock()
		b.items = append(batch, b.items...)
		if len(b.items) > b.maxBatch*maxBufferedFactor {
			drop := len(b.items) - b.maxBatch*maxBufferedFactor
			b.items = b.items[drop:]
		}
		b.mu.Unlock()
	}
}

func AppendTelemetry(t model.Telemetry) {
	if telBuf == nil {
		// fallback: 直接写入（初始化前）
		repository.DB.Create(&t)
		return
	}
	telBuf.mu.Lock()
	telBuf.items = append(telBuf.items, t)
	shouldFlush := len(telBuf.items) >= telBuf.maxBatch
	telBuf.mu.Unlock()

	if shouldFlush {
		telBuf.flush()
	}
}

// ShutdownTelemetryBuffer 停止缓冲并等待最后一次 flush 落库完成
func ShutdownTelemetryBuffer() {
	if telBuf == nil {
		return
	}
	close(telBuf.quit)
	select {
	case <-telBuf.done:
	case <-time.After(10 * time.Second): // 兜底：DB 故障时也不无限阻塞关闭
		slog.Warn("telemetry buffer shutdown timeout")
	}
}
