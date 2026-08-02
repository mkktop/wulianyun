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
}

var telBuf *TelemetryBuffer

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
	}
	go telBuf.flushLoop()
}

func (b *TelemetryBuffer) flushLoop() {
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

func ShutdownTelemetryBuffer() {
	if telBuf == nil {
		return
	}
	close(telBuf.quit)
}
