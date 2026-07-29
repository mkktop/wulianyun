# KK物联云

对标 OneNET / CTWing / 有人云的物联网设备管理平台（第一期 MVP）。

## 架构

```
MQTT 设备 ──► EMQX ──► Go 后端(Gin) ──► PostgreSQL/TimescaleDB + Redis
                            │
                            └─► WebSocket ──► Vue3 管理后台
```

- **后端**: Go + Gin + GORM，模块化单体（`server/`）
- **MQTT 接入**: EMQX 5.x，HTTP 认证回调对接平台，后端以内部客户端订阅上行/系统事件
- **存储**: PostgreSQL 16 + TimescaleDB（遥测超表），Redis（最新值缓存）
- **前端**: Vue3 + Vite + TypeScript + Element Plus + ECharts（`web/`）

## 快速启动

### 1. 启动基础设施（需 Docker Desktop）

```powershell
cd server\deploy
docker compose up -d
```

启动 PostgreSQL(5432)、Redis(6379)、EMQX(1883/18083，Dashboard 账号 admin/public)。

### 2. 启动后端

```powershell
cd server
go mod tidy
go run .\cmd\server
```

监听 `:8080`，自动建表并初始化管理员 `admin / admin123`。

### 3. 启动前端

```powershell
cd web
npm install
npm run dev
```

访问 http://localhost:5173 。

### 4. 模拟设备

在平台上创建产品和设备后，拿到三元组（ProductKey / DeviceName / DeviceSecret）：

```powershell
cd tools\simulator
npm install mqtt
node simulator.js <productKey> <deviceName> <deviceSecret>
```

模拟器每 5 秒上报温湿度，设备详情页可看到实时数据、历史曲线，并可下发 JSON 命令。

## 设备接入约定（MQTT）

| 项 | 约定 |
|---|---|
| ClientID | `{productKey}.{deviceName}` |
| Username | `{deviceName}` |
| Password | 设备 Secret |
| 上报主题 | `thing/up/{productKey}/{deviceName}`，Payload 为 JSON 对象 |
| 下发主题 | `thing/down/{productKey}/{deviceName}` |

## 目录结构

```
server/            Go 后端
  cmd/server/      入口
  internal/api/    HTTP 接口 + 路由 + JWT
  internal/mqtt/   EMQX 对接（订阅上行/系统事件、下行发布）
  internal/service/遥测入库、状态处理
  internal/ws/     WebSocket 推送中心
  internal/model/  数据模型
  deploy/          docker-compose + EMQX 配置
web/               Vue3 管理后台
tools/simulator/   MQTT 设备模拟器
```

## 路线图

- [x] 一期：用户/产品/设备管理、MQTT 接入、遥测存储、实时监控、命令下发
- [x] 二期：物模型(TSL)、设备影子（离线补发）、规则引擎（阈值/离线告警、数据转发、静默期）、告警中心
- [x] 三期：TCP 透传网关(DTU，端口 9100)、JS 脚本协议解析(上行 decode/下行 encode)、OpenAPI(HMAC 签名)、可视化大屏

## DTU 接入约定（TCP 透传）

| 项 | 约定 |
|---|---|
| 网关端口 | 9100 |
| 注册包 | `{productKey},{deviceName},{secret}\n`，成功回复 `OK` |
| 心跳 | 发送 `PING` 回复 `PONG`，空闲 300 秒断开 |
| 上行 | 产品配置了解析脚本则按 `decode(bytes)` 解码，否则按 JSON 解析 |
| 下行 | 脚本有 `encode(obj)` 则编码为二进制，否则透传 JSON |

## OpenAPI 签名

请求头：`X-App-Key` + `X-Timestamp`(Unix秒) + `X-Sign` = hex(HMAC-SHA256(AppSecret, AppKey+Timestamp))，时间窗口 ±5 分钟，接口前缀 `/openapi/v1`。
