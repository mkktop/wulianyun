# ===== 构建阶段 =====
FROM golang:1.25-alpine AS build
WORKDIR /src
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# 静态构建（pgx 为纯 Go，无 cgo），适配 linux/amd64
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# ===== 运行阶段 =====
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 10001 app
WORKDIR /app
COPY --from=build /out/server /app/server
# 默认配置（生产环境由 compose 挂载 config.prod.yaml 覆盖）
COPY configs /app/configs
RUN mkdir -p /app/uploads/firmware && chown -R app:app /app
USER app
EXPOSE 8080 9100
# exec-form：Go 进程即 PID 1，直接接收 SIGTERM，配合 main.go 的优雅关闭
ENTRYPOINT ["/app/server"]
CMD ["-conf", "configs/config.yaml"]
