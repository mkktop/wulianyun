# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

**KK物联云 (KK IoT Cloud)** — an enterprise IoT device management platform. Monorepo with three parts:

- `server/` — Go backend (module `iot-platform`), the core platform
- `web/` — Vue3 management console
- `tools/simulator/` — Node.js device simulators (MQTT + DTU/TCP)

Code comments, log messages, and user-facing strings are predominantly **Chinese** — match this convention in new code. The project targets comprehensive enterprise IoT feature coverage; recent commit history is the clearest record of feature work.

## Commands

All backend commands run from `server/`. The vendored Go 1.25 toolchain lives at `.tools/go/bin/go.exe` (gitignored); use it or any system `go` ≥ 1.25.

```powershell
# Infrastructure (Docker Desktop required) — Postgres/TimescaleDB, Redis, EMQX
cd server\deploy; docker compose up -d

# Backend — auto-migrates schema, creates admin/admin123, listens :8080
cd server; go run .\cmd\server
go run .\cmd\server -conf configs\config.yaml   # explicit config (default)
go run .\cmd\server -conf configs\staging.yaml   # alternate config

# Frontend — :5173, proxies /api → :8080 (ws: true)
cd web; npm install; npm run dev
cd web; npm run build                             # → web/dist

# Tests (live under internal/gateway, internal/modbus)
cd server; go test ./...
go test ./internal/modbus -run TestCRC16          # single test
go test -v ./internal/gateway                     # one package, verbose

# Build (artifacts land in server/bin or server/server.exe)
cd server; go build -o bin\server.exe .\cmd\server
$env:GOOS="linux"; go build -o bin\server-linux .\cmd\server  # cross-compile

# Device simulator
cd tools\simulator; node simulator.js <productId> <deviceName> <deviceSecret> [broker]
```

Default ports: HTTP `:8080` · TCP gateway `:9100` · MQTT `1883` (EMQX dashboard `18083`, admin/public) · Postgres `5432` · Redis `6379` · frontend `5173`. Docker uses `network_mode: host` so Windows/WSL localhost reaches the containers directly.

## High-Level Architecture

### Three device access paths converge on one ingest pipeline

A `Product` declares how its devices connect via `AccessMode` + `Protocol`:

| Protocol | Transport | How it reaches the backend |
|---|---|---|
| **MQTT** | EMQX 5.x | Backend is an *internal subscriber client* (paho); devices publish to `thing/up/{pk}/{dn}`. EMQX calls back to `/api/v1/emqx/auth` & `/emqx/acl` for per-connection auth/authorization. |
| **TCP (DTU)** | Raw TCP `:9100` (`internal/gateway`) | Registration packet (三元组 *or* a custom `RegCode` like IMEI/ICCID) → `OK`/`ERR`, then framing + heartbeat. Passthrough products run a per-product JS codec; Modbus products are polled by the engine. |
| **HTTP** | `POST /api/v1/http/telemetry` | Device authenticates with a `tk:` dynamic token, posts JSON directly. |

All paths funnel into `service.HandleTelemetry` (`internal/service/ingest.go`), which is the single telemetry entry point. Its pipeline:

1. Parse JSON → `FindDevice` (cached) → **method dispatch** (`event.post` / `ntp.request` / `config.get` short-circuit out)
2. **TSL validation** against the product's thing-model (`ValidateTelemetry`)
3. **Buffered write** to TimescaleDB via `AppendTelemetry` — **skipped when `emqx_rule.enabled`** (EMQX rule engine writes the hypertable directly; backend then runs the "fast path" = business logic only)
4. **Latest-value cache**: Redis Lua atomic merge (`mergeLatestScript`) keyed `device:latest:{id}`
5. **WebSocket push** → shadow merge → **rule engine eval** (`rule.EvalTelemetry`)
6. Async writes to `MessageTrace` and `DeviceLog`

### Downlink dispatch is wired in `main.go`, not in any package

`service.DownPublisher` and `service.Broadcaster` are injected function fields set in `cmd/server/main.go`. This is the single place that decides the down-channel and is worth reading before touching anything device→out:

- `DownPublisher(pk, dn, payload)`: if the device is on the **TCP gateway** → Modbus product ⇒ `poller.WriteProperty` (register/coil writes); passthrough product ⇒ `gateway.Send` (encode via codec or raw JSON). Otherwise ⇒ `mqtt.PublishDown`. Always records a `CommandLog` + device log.
- `Broadcaster(pk, payload)`: `gateway.Broadcast` (per TCP connection) + `mqtt.PublishBroadcast` simultaneously.

### Multi-instance is first-class

The platform runs horizontally; several subsystems use Redis and degrade to single-instance memory when Redis is unavailable:

- **WebSocket fan-out**: `ws.H.push` publishes to Redis channel `ws:broadcast`; every instance's subscriber (`ws.StartPubSub`) re-dispatches to its local connections. Outbound msg carries `userId` + optional `onlyDevice`.
- **Modbus polling**: `poller` takes a Redis SETNX lock per `(product, group, device)` to stop two instances polling the same device.
- **Rule silence windows**: `rule.UseRedisSilence` (memory `sync.Map` fallback).
- **TCP down cross-instance**: `service.InitDownRouter` + channel `tcp:down` — if the owning instance lacks the TCP connection it publishes to Redis so the instance holding the connection delivers.

### Supporting subsystems

- **Rule engine** (`internal/rule`): types `alarm` / `forward` / `offline`. Rules cached per-user (30s TTL; `InvalidateRuleCache` after writes). Conditions support compound `and`/`or` or legacy single `{field,op,value}`. Silence periods, auto-resolve on condition clearing, SSRF-guarded webhooks with exponential-backoff retry. Forward targets: webhook / kafka / mqtt_bridge.
- **Modbus poller** (`internal/poller`): hooks into `gateway.OnDeviceConnect/Disconnect`. Schedules per **collection group** (分频) at its own interval; `PlanReadBlocks` merges contiguous registers into batch reads; supports on-change reporting; bounded by `poller.max_concurrent` semaphore.
- **TCP gateway** (`internal/gateway`): per-IP connection cap + token-bucket rate limit; framing modes `none`/`delimiter`/`length` (Modbus products use RTU framing); Modbus request/response is half-duplex — `session.reqMu` serializes concurrent requests and `deliver()` matches replies by slave+function code.
- **Codec** (`internal/codec`): per-product goja JS, contract `function decode(bytes)` / `function encode(obj)`. Compiled programs SHA256-cached per product; 200ms execution timeout via `vm.Interrupt`.

### Data layer

- **PostgreSQL 16 + TimescaleDB**: `repository.Init` runs GORM `AutoMigrate` over the full model list, then promotes `telemetries` to a hypertable on `ts` and adds retention + compression policies. If the TimescaleDB extension is absent it **non-fatally** falls back to a plain table. GORM logger is fixed at `Warn`, so SQL is quiet by default.
- **Redis**: latest-value cache, `tk:` device tokens, WS/distributed-lock/silence/down pub-sub, IP rate-limit state. A down Redis disables latest-value caching and reverts multi-instance features to single-instance mode.
- Retention: `MessageTrace` and `DeviceLog` are pruned by a 6-hour cleanup loop using `log.trace_retention_days` / `device_log_retention_days`.

## Key Conventions

- **API envelope**: every JSON response is `{code, msg, data}` where `code === 0` means success. The frontend axios interceptor (`web/src/api/index.ts`) unwraps `data` and surfaces non-zero `msg` via `ElMessage`; `401` clears the token and redirects to `/login`.
- **MQTT client convention**: `clientid = {productId}.{deviceName}` (parsed by `service.ParseClientID`), `username = {deviceName}`, `password = device secret` **or** a `tk:` dynamic token. Topics: `thing/up|down|broadcast|offline/{pk}/{dn}`, plus `thing/gateway/{pk}/{dn}/sub/...` for sub-devices. See `mqtt/client.go` constants.
- **Secret modes**: `device` (一机一密, per-device secret) vs `product` (一型一密, shared `ProductSecret` that auto-registers unknown devices — `FindDeviceForAuth`).
- **Auth layers** (all in `internal/api`): `JWTAuth` (web/admin, JWT in `Authorization: Bearer` or `?token=` for WS), `OpenAPIAuth` (HMAC-SHA256 signing on `/openapi/v1`, `±5 min` timestamp window), and EMQX callbacks `EmqxAuth`/`EmqxACL`. OpenAPI calls reuse the same handlers as the admin API, scoped to the app owner's `uid`.
- **Frontend**: single axios client at `/api/v1`; WebSocket at `/api/v1/ws` — send `{type:"subscribe",deviceId}` to filter telemetry to subscribed devices. `/screen` (BigScreen) is outside the auth-protected layout. Routes are lazy-loaded in `web/src/router/index.ts`.
- **Graceful shutdown**: `main.go` traps SIGINT/SIGTERM, tears down perf components in reverse order (shadow cache → telemetry buffer flush), then `srv.Shutdown(5s)`.
- `server/server.exe`, `server/bin/`, `web/dist/`, `.tools/`, and `node_modules/` are build artifacts / vendored tooling — gitignored, do not edit.

## 离线交付包 (deploy/dist/)

商业化路径 = **私有化交付 (on-prem)**。`deploy/dist/` 是标准化**离线自包含交付包**——构建机产镜像、客户机 `docker load` + `docker compose up -d`，**绝不在客户机构建**（弱机/离线友好）。CI 打 `v*` tag 自动构建发 Release。这是商业化的交付门槛，应用层基本不动代码。

**构建 `deploy/dist/build.sh`（需 Docker + Go ≥1.25 + Node）：**

```bash
cd deploy/dist && bash build.sh amd64     # 产 kk-iot-<VERSION>-amd64.tar.gz + .sha256
```

build.sh 流程：go 交叉编译 server（`CGO_ENABLED=0`）→ web/docs `npm build`（docs 产物并入 web/dist/developer）→ 薄单阶段 `Dockerfile.server`/`Dockerfile.web` 打 `kk-iot/server`、`kk-iot/web` → 连同 `timescale/timescaledb:2.17.2-pg16` / `redis:7-alpine` / `emqx/emqx:5.8`（**固定 tag，不用 latest**）`docker save` → tar.gz + sha256。Go 多源探测：`$GO` → `.tools/go` → `go`；arm64 走 QEMU（本期只实测 amd64）。

**版本与发布：** 仓库根 `VERSION`（纯 semver，当前 `1.0.4`）是唯一版本源——build.sh 读取、CI 校验 tag==VERSION（不一致 CI fail）。`git tag v<VERSION>` 推送 → [`.github/workflows/build-release.yml`](.github/workflows/build-release.yml) 构建并 `gh release create`，Release 说明由 `RELEASE_NOTES.md`（`@VERSION@` 占位符）渲染（也作为包内 README 单一来源）。客户从 Releases 下载 tar.gz + sha256。

**客户侧脚本（`deploy/dist/payload/`，打进交付包）：**

- `install.sh`：root 预检（OS/架构/资源/端口）→ `docker load images/*.tar` → 生成 6 个密钥写 `.env`（幂等，`--force` 重生）→ envsubst 渲染 `compose/config.prod.yaml`（缺 gettext 走纯 bash 回退）→ `compose up -d` → 轮询 `/readyz`
- `upgrade.sh <新包目录>`：load 新镜像 + 切 `.env` 的 `VER`（留 `.env.bak-<旧版>`）+ compose 优雅重建 server/web（**pg/redis/emqx 数据卷不动**）→ 轮询 readyz → 打印回滚命令。**注意从旧包目录运行、传新包目录**
- `backup.sh` / `restore.sh <备份包>`：`pg_dump -Fc`（在线，不中断）+ uploads 卷 tar + redis RDB（非致命）+ config/.env → `backup-<VER>-<ts>.tar.gz` + sha256。卷操作用**包内自带的 `kk-iot/server` 镜像**（alpine 底，`--entrypoint '' --user 0` 跑 busybox tar/cp），离线不拉 alpine
- `diag.sh`：收集 compose ps / logs / stats / 配置（**脱敏** password/secret）/ images → tar.gz

**交付包关键约定（踩过的坑）：**

- compose 顶层 `name: kk-iot`（+ `.env` 的 `COMPOSE_PROJECT_NAME`）固定卷名 `kk-iot_pgdata` 等——换解压目录不影响，否则会"空库附挂=数据丢失"假象
- **server 容器 uid 10001**：bind-mount 的 `config.prod.yaml` 必须 `644`（`install.sh`/`restore.sh` 已做；`600` → `permission denied` 崩溃循环）。`Dockerfile.server` 必须 `mkdir -p /app/uploads/firmware && chown -R app:app /app`，否则 named volume 首次复制继承 root → OTA 固件上传 EPERM
- 探针 `server/internal/api/health.go`：`/api/v1/healthz`（存活，无依赖，compose healthcheck 用）+ `/readyz`（DB ping，install/upgrade 轮询用）——拆双端点避免 DB 抖动触发重启
- EMQX 的 `emqx ctl status` 超时 / compose `unhealthy` 标签是**假阴性**（dist 协议 ping）；EMQX 实际正常，看设备 auth 回调 200 + server 内部 MQTT 客户端连接 `mqtt connected`
- 第二期未做（位置已标记）：版本化 migration（替裸 AutoMigrate）、ldflags 版本注入、license 授权、捆绑 Docker 离线安装、arm64 实测

## Test Deployment (Aliyun ECS)

The platform is deployed on an Aliyun ECS used as the **ongoing test server**:

- **Host**: 阿里云 ECS 测试机（2 核 / 1.6 GB RAM / Ubuntu 26.04，已装 Docker 29 + Compose v5）。**公网 IP / 内网 IP / SSH 密码均不写入仓库**——见本地 `memory/prod-deployment.md` 或询问用户。
- **SSH**: `root@<test-server>`；密码见本地 gitignored `.claude/` 或 `memory/prod-deployment.md`。本机无 `sshpass`，用 `paramiko`（Python）驱动，而非 shell `ssh`（连接慢，需 `banner_timeout=30`）。
- **On server**: 测试机**现跑离线交付包**（当前 v1.0.4），装在 `/root/pkg/kk-iot-<VER>-amd64/`——`docker compose -p kk-iot -f compose/docker-compose.yml`（容器 `iot-*`，项目名 `kk-iot`，密钥在该目录 `.env`）。用 `upgrade.sh` 升级、`backup.sh` 备份。遗留在线开发路径仍在 `/opt/wulianyun/deploy/prod`（`setup.sh` 生成 `.env`），但已被交付包取代。
- **Public access**: web `:80`、MQTT `:1883`、DTU/TCP `:9100`——三者在阿里云安全组均放行。EMQX dashboard `:18083` 仅绑 `127.0.0.1`（经 SSH 隧道访问）。`8080/5432/6379` 仅内部。

### ⚠️ Thin-image deploy (this server can NOT run the repo's multi-stage build)

The repo's `server/Dockerfile` / `web/Dockerfile` are self-contained **multi-stage** builds — fine on a 4 GB+ machine, but on this 1.6 GB box with rate-limited CN registry mirrors, `docker compose up --build` **hangs on buildkit metadata resolution and OOMs** (npm + Go compile in parallel). Deploy with **prebuilt artifacts + thin single-stage Dockerfiles** instead:

1. **Local prebuild** (on the dev box):
   ```powershell
   cd server; GOOS=linux GOARCH=amd64 CGO_ENABLED=0 .tools\go\bin\go.exe build -trimpath -ldflags="-s -w" -o ..\_kk\server-linux .\cmd\server
   cd ..\web; npm run build          # → web/dist
   cd ..\docs; npm ci; npm run docs:build   # → docs/.vitepress/dist（VitePress 开发文档）
   # 文档产物并入前端 dist，随 web/dist 一起上传；nginx 经 /developer/ 子路径提供（compose 已 bind-mount）
   if (Test-Path ..\web\dist\developer) { Remove-Item -Recurse -Force ..\web\dist\developer }
   Copy-Item -Recurse .vitepress\dist ..\web\dist\developer   # 先清后拷，避免重复运行时 developer/dist 套娃
   cd ..\web
   ```
2. **Thin Dockerfiles** (single stage): `FROM alpine:3.20` + `COPY server-linux /app/server` (+ `COPY configs`, `ca-certificates`, `tzdata`, non-root `app`); `FROM nginx:1.27-alpine` + `COPY dist` + `COPY nginx.conf`. The web one needs `web/.dockerignore` to **keep `dist/`** (the repo's excludes it for multi-stage).
3. **Upload** via SFTP — but the server's SFTP (paramiko + OpenSSH 10.3) **rejects absolute remote paths** (ENOENT on open-for-write); use **relative paths** (SFTP CWD is `/root`), or tar + relative put + `tar xzf` over `/opt/wulianyun` (preserves `.env`/`config.prod.yaml`).
4. **On server**: `cd /opt/wulianyun/deploy/prod && docker compose build && docker compose up -d`.

### Server-specific gotchas (already handled, recorded to avoid regressions)

- **No swap by default** → `fallocate -l 2G /swapfile && mkswap && swapon` (+ `/etc/fstab`). Required: MySQL + EMQX + TimescaleDB coexist in 1.6 GB.
- **Redis healthcheck**: the redis service must declare `environment: REDIS_PASSWORD: ${REDIS_PASSWORD}` or the in-container `$REDIS_PASSWORD` is empty and `redis-cli -a` fails → unhealthy → server never starts.
- **config.prod.yaml perms**: bind-mounted read-only, server runs as uid `10001` — the host file (root:root 600) needs `chmod 644` or the binary exits `permission denied`.
- **EMQX `unhealthy` label is a false negative**: `emqx ctl status` flake-times out on the dist protocol; EMQX actually serves MQTT (verified by device auth + telemetry round-trip). Don't chase it.
- **nginx upstream `host not found`**: only fires while `server` is crash-looping; resolves once server is stably up.

### Redeploy checklist

- **代码变更（交付包路径，当前推荐）** → 本地 `bash deploy/dist/build.sh amd64` 出新包（或 CI 打 `v*` tag 出 Release）→ 传服务器 → `cd /root/pkg/kk-iot-<旧VER>-amd64 && bash upgrade.sh /root/pkg/kk-iot-<新VER>-amd64`（数据卷不动）。
- **配置变更** → 编辑 `/root/pkg/kk-iot-<VER>-amd64/compose/config.prod.yaml`（**保持 644**）→ `docker compose -p kk-iot -f compose/docker-compose.yml restart server`。
- **遗留手动路径**（仅在线开发/staging）：本地 prebuild Go/Vue → 上传 → `/opt/wulianyun/deploy/prod` 下 `docker compose up -d --build server`。**勿在此 1.6G 服务器跑多阶段构建**。
- **日志/状态**：`docker compose -p kk-iot -f compose/docker-compose.yml logs -f server` / `ps`。`iot-emqx` 显示 `unhealthy`——忽略（见上）；真实 MQTT 健康看设备 auth 回调 200 + `mqtt connected`。
