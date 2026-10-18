package service

import (
	"log/slog"
	"time"

	"iot-platform/internal/model"
	"iot-platform/internal/repository"
)

// StartCommandTimeoutSweeper 指令超时扫描：每分钟把超过自身 TimeoutMs 仍未应答的
// pending 指令置为 timeout。此前 timeout 状态只定义不落地，pending 指令永久挂起，
// 指令日志无法区分"设备未回"与"尚未超时"。
func StartCommandTimeoutSweeper() {
	go func() {
		sweepCommandTimeouts()
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			sweepCommandTimeouts()
		}
	}()
}

// sweepCommandTimeouts 按每条指令自带的 timeout_ms 判定（默认 10s），
// 部分索引 idx_command_requests_pending 保证扫描只触碰存量 pending 行
func sweepCommandTimeouts() {
	ctx, cancel := repository.CtxWith(5 * time.Second)
	defer cancel()
	result := repository.DB.WithContext(ctx).
		Model(&model.CommandRequest{}).
		Where("status = ? AND created_at + make_interval(secs => timeout_ms / 1000.0) < now()", model.CommandStatusPending).
		Update("status", model.CommandStatusTimeout)
	if result.Error != nil {
		slog.Warn("sweep command timeouts failed", "err", result.Error)
		return
	}
	if result.RowsAffected > 0 {
		slog.Info("command requests timed out", "count", result.RowsAffected)
	}
}
