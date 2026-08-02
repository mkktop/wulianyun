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
