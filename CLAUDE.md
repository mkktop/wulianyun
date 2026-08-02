# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

**KK物联云 (KK IoT Cloud)** — an IoT device management platform benchmarked against OneNET / CTWing(天翼) / 有人云. Monorepo with three parts:

- `server/` — Go backend (module `iot-platform`), the core platform
- `web/` — Vue3 management console
- `tools/simulator/` — Node.js device simulators (MQTT + DTU/TCP)

Code comments, log messages, and user-facing strings are predominantly **Chinese** — match this convention in new code. The project deliberately mirrors the feature sets of the three reference platforms; recent commit history is the clearest record of feature parity work.

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
cd tools\simulator; node simulator.js <productKey> <deviceName> <deviceSecret> [broker]
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
- **MQTT client convention**: `clientid = {productKey}.{deviceName}` (parsed by `service.ParseClientID`), `username = {deviceName}`, `password = device secret` **or** a `tk:` dynamic token. Topics: `thing/up|down|broadcast|offline/{pk}/{dn}`, plus `thing/gateway/{pk}/{dn}/sub/...` for sub-devices. See `mqtt/client.go` constants.
- **Secret modes**: `device` (一机一密, per-device secret) vs `product` (一型一密, shared `ProductSecret` that auto-registers unknown devices — `FindDeviceForAuth`).
- **Auth layers** (all in `internal/api`): `JWTAuth` (web/admin, JWT in `Authorization: Bearer` or `?token=` for WS), `OpenAPIAuth` (HMAC-SHA256 signing on `/openapi/v1`, `±5 min` timestamp window), and EMQX callbacks `EmqxAuth`/`EmqxACL`. OpenAPI calls reuse the same handlers as the admin API, scoped to the app owner's `uid`.
- **Frontend**: single axios client at `/api/v1`; WebSocket at `/api/v1/ws` — send `{type:"subscribe",deviceId}` to filter telemetry to subscribed devices. `/screen` (BigScreen) is outside the auth-protected layout. Routes are lazy-loaded in `web/src/router/index.ts`.
- **Graceful shutdown**: `main.go` traps SIGINT/SIGTERM, tears down perf components in reverse order (shadow cache → telemetry buffer flush), then `srv.Shutdown(5s)`.
- `server/server.exe`, `server/bin/`, `web/dist/`, `.tools/`, and `node_modules/` are build artifacts / vendored tooling — gitignored, do not edit.

## Test Deployment (Aliyun ECS)

The platform is deployed on an Aliyun ECS used as the **ongoing test server**:

- **Host**: 阿里云 ECS 测试机（2 核 / 1.6 GB RAM / Ubuntu 26.04，已装 Docker 29 + Compose v5）。**公网 IP / 内网 IP / SSH 密码均不写入仓库**——见本地 `memory/prod-deployment.md` 或询问用户。
- **SSH**: `root@<test-server>`；密码见本地 gitignored `.claude/` 或 `memory/prod-deployment.md`。本机无 `sshpass`，用 `paramiko`（Python）驱动，而非 shell `ssh`。
- **On server**: code at `/opt/wulianyun`; generated secrets at `/opt/wulianyun/deploy/prod/.env` (run `bash setup.sh` to generate, `--force` to regenerate). Stack runs via `docker compose` from `/opt/wulianyun/deploy/prod`.
- **Public access**: web `:80`、MQTT `:1883`、DTU/TCP `:9100`——三者在阿里云安全组均放行。EMQX dashboard `:18083` 仅绑 `127.0.0.1`（经 SSH 隧道访问）。`8080/5432/6379` 仅内部。

### ⚠️ Thin-image deploy (this server can NOT run the repo's multi-stage build)

The repo's `server/Dockerfile` / `web/Dockerfile` are self-contained **multi-stage** builds — fine on a 4 GB+ machine, but on this 1.6 GB box with rate-limited CN registry mirrors, `docker compose up --build` **hangs on buildkit metadata resolution and OOMs** (npm + Go compile in parallel). Deploy with **prebuilt artifacts + thin single-stage Dockerfiles** instead:

1. **Local prebuild** (on the dev box):
   ```powershell
   cd server; GOOS=linux GOARCH=amd64 CGO_ENABLED=0 .tools\go\bin\go.exe build -trimpath -ldflags="-s -w" -o ..\_kk\server-linux .\cmd\server
   cd ..\web; npm run build   # → web/dist
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

- **Code change** → local prebuild (Go binary / Vue dist) → upload → `docker compose up -d --build server` (or `web`). Don't run the multi-stage build on this server.
- **Config change** → edit `/opt/wulianyun/deploy/prod/config.prod.yaml` → `docker compose restart server`.
- **Logs/status**: `docker compose logs -f server`, `docker compose ps`. `iot-emqx` will show `unhealthy` — ignore (see above); check real MQTT health by publishing as a test device.
