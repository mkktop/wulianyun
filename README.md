# KK 物联云

> 对标 OneNET / CTWing(天翼) / 有人云 的物联网设备管理平台 —— 一个用 Go + Vue3 构建的开箱即用型 IoT PaaS，覆盖设备接入、物模型、遥测时序存储、规则引擎、告警、OTA、开放平台与可视化大屏。

> 📖 **第三方开发者请阅读 [docs/ 设备接入协议文档](docs/README.md)**（MQTT / TCP-DTU / HTTP 接入、物的模型、OpenAPI 签名、OTA 的完整报文格式与示例代码）。

---

## 目录

- [面向开发者：设备接入协议文档](docs/README.md)
- [一、架构总览](#一架构总览)
- [二、核心特性](#二核心特性)
- [三、快速开始](#三快速开始)
- [四、设备接入](#四设备接入)
- [五、物模型与接入模式](#五物模型与接入模式)
- [六、设备控制与下行分发](#六设备控制与下行分发)
- [七、规则引擎与告警](#七规则引擎与告警)
- [八、开放平台 OpenAPI](#八开放平台-openapi)
- [九、开发者工具](#九开发者工具)
- [十、配置参考](#十配置参考)
- [十一、部署与运维](#十一部署与运维)
- [十二、数据存储与性能](#十二数据存储与性能)
- [十三、目录结构](#十三目录结构)
- [十四、安全注意事项](#十四安全注意事项)

---

## 一、架构总览

```
                          ┌─────────────────────────────────────────────┐
   MQTT 设备 ──► EMQX 5.x ─┤ HTTP 认证/授权回调 (/emqx/auth /emqx/acl)       │
                          │   内部客户端订阅 thing/up/# 与 $SYS 事件         │
   DTU/TCP 设备 ──► :9100 ─┤ TCP 透传网关(组帧/心跳/限流/Modbus 半双工)        │
   HTTP 设备 ────► :8080 ──┤ POST /http/telemetry (Token 认证)              │
                          └───────────────┬─────────────────────────────┘
                                          │ service.HandleTelemetry (统一入口)
            ┌─────────────────────────────▼──────────────────────────────┐
            │  method 分流(event.post / ntp / config.get) → 遥测管线：        │
            │  TSL 校验 → TimescaleDB 批量写 → Redis 原子合并最新值 →          │
            │  WebSocket 推送 → 设备影子合并 → 规则引擎评估 → 轨迹/日志         │
            └─────┬──────────────────┬──────────────────┬─────────────────┘
                  │                  │                  │
        PostgreSQL/TimescaleDB    Redis            WebSocket
        (遥测超表 + 业务表)      (缓存/PubSub/锁)   (Vue3 管理后台)
```

**技术栈**

| 层 | 选型 |
|---|---|
| 后端 | Go 1.25 · Gin · GORM · paho.mqtt · goja(JS 引擎) · gorilla/websocket |
| 接入 | EMQX 5.x（MQTT，HTTP 认证回调）· 自研 TCP 网关（:9100，DTU/透传/Modbus）· HTTP |
| 存储 | PostgreSQL 16 + TimescaleDB（遥测超表）· Redis 7（最新值缓存 / Pub/Sub / 分布式锁） |
| 前端 | Vue 3 · Vite 5 · TypeScript · Element Plus 2 · ECharts 5 · Pinia · vue-router 4 |

平台以**模块化单体**组织，但通过 Redis 实现了**多实例水平扩展**能力（WebSocket 扇出、Modbus 轮询分布式锁、规则静默期共享、TCP 下行跨实例路由）。

---

## 二、核心特性

**设备接入**
- 三协议接入：MQTT（EMQX，HTTP 认证 + 主题级 ACL）、TCP 透传网关（DTU，:9100）、HTTP 直传
- 三种数据模式：标准物模型 `thingmodel`、透传脚本解析 `passthrough`、Modbus 云端轮询 `modbus`
- 两种密钥模式：一机一密 `device`、一型一密 `product`（支持免预注册动态注册）
- 动态 Token（`tk:` 前缀）换取 MQTT 连接凭证，替代长期 Secret
- LWT 遗嘱 + `$SYS` 系统事件双通道秒级感知设备上下线

**数据与物模型**
- 物模型 TSL（properties / events / services），支持 JSON 导入导出
- TimescaleDB 超表存储遥测，自动保留（默认 30 天）与压缩（7 天后）
- Redis Lua 原子合并最新值，避免并发竞态
- 自定义协议 goja JS 脚本 `decode(bytes)` / `encode(obj)`，200ms 执行超时

**设备控制**
- 下行通道自动决策：TCP 在线 → Modbus 点位写 / 网关透传；否则走 MQTT
- 设备影子（desired / reported），上线补发期望值、达成自动清除
- 指令 messageId 应答机制（pending → acked）
- OTA 升级（固件 SHA-256、批量任务、进度上报）
- 远程配置版本化下发、产品广播、NTP 对时、网关子设备协议

**规则与告警**
- 三类规则：阈值告警 `alarm`、离线告警 `offline`、数据转发 `forward`
- 复合条件（and/or 递归）、静默期、告警自动恢复
- 转发目标：Webhook（SSRF 防护 + 重试退避）/ Kafka / MQTT 桥接

**开放与运维**
- OpenAPI（`/openapi/v1`）HMAC-SHA256 签名鉴权，复用管理端能力
- 平台内置设备模拟器、MQTT 调试台、消息轨迹（分阶段耗时）、设备日志
- 可视化大屏（KPI + 实时告警）、Vue3 管理后台
- EMQX 规则引擎直写 TimescaleDB 快路径，后端跳过 DB 写入

---

## 三、快速开始

> 需要 Docker Desktop（用于拉起基础设施）。后端构建使用 Go 1.25（仓库内 `.tools/go/` 提供了 vendored 工具链，可执行 `.tools\go\bin\go.exe`）。

### 1. 启动基础设施

```powershell
cd server\deploy
docker compose up -d
```

| 服务 | 镜像 | 端口 | 凭证 |
|---|---|---|---|
| PostgreSQL + TimescaleDB | `timescale/timescaledb:latest-pg16` | 5432 | `iot / iot123456`，库 `iot` |
| Redis | `redis:7-alpine` | 6379 | 无密码 |
| EMQX | `emqx/emqx:5.8` | 1883 / 8083 / 18083 | Dashboard `admin / public` |

> 三个容器统一使用 `network_mode: host`，便于 Windows/WSL localhost 直连。

### 2. 启动后端

```powershell
cd server
go mod tidy
go run .\cmd\server
```

- 监听 `:8080`（HTTP）与 `:9100`（TCP 网关）
- 启动时自动建表、启用 TimescaleDB 超表、创建管理员 `admin / admin123`
- 用 `-conf` 指定其它配置：`go run .\cmd\server -conf configs\staging.yaml`

### 3. 启动前端

```powershell
cd web
npm install
npm run dev
```

访问 http://localhost:5173 ，使用 `admin / admin123` 登录。Vite 已配置代理：`/api` → `http://127.0.0.1:8080`（含 WebSocket）。

### 4. 模拟设备

在平台创建产品和设备，拿到三元组（ProductKey / DeviceName / DeviceSecret），运行模拟器：

```powershell
cd tools\simulator
npm install
# MQTT 设备：每 5 秒上报温湿度，接收下行命令与影子
node simulator.js <productKey> <deviceName> <deviceSecret>
# TCP 透传(DTU)设备
node dtu.js <productKey> <deviceName> <deviceSecret>
# TCP + Modbus 从机模拟
node dtu-modbus.js <productKey> <deviceName> <deviceSecret>
```

设备详情页可看到实时数据、历史曲线、上下线事件，并可下发 JSON 命令。

---

## 四、设备接入

### MQTT 接入约定

| 项 | 约定 |
|---|---|
| Broker | `tcp://<host>:1883` |
| ClientID | `{productKey}.{deviceName}` |
| Username | `{deviceName}` |
| Password | 设备 Secret，或动态 Token（`tk:` 开头） |
| 遗嘱(LWT) | 发布到 `thing/offline/{productKey}/{deviceName}`，TCP 断开秒级离线 |

### MQTT 主题

| 方向 | 主题 | QoS | Retained | 语义 |
|---|---|---|---|---|
| 设备→平台 | `thing/up/{pk}/{dn}` | 1 | 否 | 遥测属性上报 |
| 设备→平台 | `thing/up/{pk}/{dn}/ota` | 1 | 否 | OTA 进度上报 |
| 设备→平台 | `thing/up/{pk}/{dn}/reply` | 1 | 否 | 指令应答 `{messageId,code,data}` |
| 平台→设备 | `thing/down/{pk}/{dn}` | 1 | 否 | 下行指令 / 通知 |
| 平台→设备 | `thing/down/{pk}/{dn}/retained` | 1 | 是 | Retained 期望值/配置（上线即取） |
| 平台→设备 | `thing/broadcast/{pk}` | 1 | 否 | 产品级广播 |
| 设备→平台 | `thing/offline/{pk}/{dn}` | 1 | — | LWT 遗嘱 |
| 网关 | `thing/gateway/{pk}/{dn}/sub/{subId}/login\|logout` | 1 | 否 | 子设备登录/登出 |
| 平台→网关 | `thing/gateway/{pk}/{gwName}/sub/{subDn}` | 1 | 否 | 子设备下行 |

> 平台定义 QoS 0（至多一次）/ 1（至少一次，默认）/ 2（精确一次）三级常量，上下行默认 QoS 1。

### EMQX 鉴权与授权

EMQX 通过 HTTP 回调后端做设备身份校验与主题级 ACL：

- **认证** `POST /api/v1/emqx/auth`：返回 `{"result":"allow"|"deny"}`，放行平台内部超级账号、固定 Secret 与 `tk:` 动态 Token 两种模式
- **授权** `POST /api/v1/emqx/acl`：按 publish/subscribe 做主题级授权

设备主题级授权矩阵：

| Action | 允许的主题 | 通配符 |
|---|---|---|
| publish | `thing/up/{pk}/{dn}` 及子级 | 禁止 `+/#` |
| publish | `thing/offline/{pk}/{dn}`（自己的遗嘱） | 禁止 |
| publish | `thing/gateway/{pk}/{dn}/sub/{subId}/login\|logout` | 禁止 |
| subscribe | `thing/down/{pk}/{dn}` 及子级（含 `/retained`） | 禁止 `+/#` |
| subscribe | `thing/broadcast/{pk}` | 禁止 |
| subscribe | `thing/gateway/{pk}/{dn}/sub/+` | **允许 `+`** |

### 动态 Token

设备可先用三元组换取短期 Token（默认 1 小时），再用 Token 作为 MQTT password 连接：

```http
POST /api/v1/auth/token
{ "productKey": "...", "deviceName": "...", "secret": "..." }
→ { "code": 0, "data": { "token": "tk:...", "ttl": 3600 } }
```

Token 存于 Redis（`device:token:{token}`），过期/撤销即失效。

### TCP 透传接入（DTU，:9100）

| 阶段 | 约定 |
|---|---|
| 注册包 | 连接后 10 秒内发送：三元组 `{productKey},{deviceName},{secret}\n` 或自定义注册码（单行，匹配设备 `regCode`，如 IMEI/ICCID） |
| 鉴权 | 成功回复 `OK\n`，失败回复 `ERR\n` 并断开 |
| 组帧 | 按产品配置切分：`none`（不组帧）/`delimiter`（定界符）/`length`（长度字段）；Modbus 产品固定按 RTU 帧组帧 |
| 心跳 | 默认 `PING`→`PONG`；或产品自定义（支持文本或 `0x` HEX） |
| 上行 | 有解析脚本则 `decode(bytes)` 解码，否则尝试 JSON |
| 下行 | 脚本有 `encode(obj)` 则编码为二进制，否则透传 JSON |
| 容错 | 空闲 300 秒断开；单 IP 连接数上限（默认 10）+ 令牌桶速率限制；断开时记录重连引导 |

### HTTP 直传

```http
POST /api/v1/http/telemetry
Header: X-Device-Token: Base64(productKey:deviceName:secret)
Body:   { "temperature": 25.5, "humidity": 60 }
```

> 注意：HTTP 直传的 `X-Device-Token` 仅支持固定 Secret，不支持 `tk:` 动态 Token。

---

## 五、物模型与接入模式

### 接入数据模式（accessMode）

| accessMode | 含义 | 协议约束 | 解析方式 |
|---|---|---|---|
| `thingmodel` | 标准物模型（JSON，默认） | mqtt/tcp/http | 平台直接按 TSL 校验 |
| `passthrough` | 透传解析 | 通常 tcp | 产品级 goja JS 脚本 `decode/encode` |
| `modbus` | Modbus 云端轮询 | **强制 tcp** | 平台按点位表主动轮询寄存器 |

### 密钥模式（secretMode）

| secretMode | 含义 | 动态注册行为 |
|---|---|---|
| `device` | 一机一密（默认） | 设备须预先创建，校验设备级 Secret |
| `product` | 一型一密 | 创建产品时生成 `ProductSecret`；设备不存在且 `secret==ProductSecret` 时**自动建设备**并签发独立 Secret |

### 物模型 TSL

物模型以 JSON 描述三类能力，按产品整体覆盖保存，支持导入导出：

| 类别 | 字段 |
|---|---|
| properties 属性 | `identifier, name, dataType(int32/float/double/bool/enum/text/date), unit, min, max, step, accessMode(r/rw), enumSpec[{value,label}], desc` |
| events 事件 | `identifier, name, type(info/alert/fault), outputs[...], desc` |
| services 服务 | `identifier, name, async(bool), inputs[...], outputs[...], desc` |

> 标识符禁用保留字：`set / get / post / property / event / time / value`。

### 自定义协议 JS 脚本（goja）

```javascript
// 上行解码（必填）：bytes 为 0-255 整数数组，必须返回对象
function decode(bytes) {
  return { temperature: bytes[0] + bytes[1] / 10 }
}
// 下行编码（可选）：对象 -> 0-255 整数数组
function encode(obj) {
  return [obj.switch ? 0x01 : 0x00]
}
```

- 单脚本执行超时 **200ms**（`vm.Interrupt`）
- 编译结果按 `productID + sha256(script)` 缓存，热执行不重编译
- 可用 hex 报文在线测试：`POST /api/v1/products/:id/codec/test`

### Modbus 接入

**点位表**（单产品上限 100 点位）：`identifier / name / groupId / slaveId / functionCode / address / rawType / bitPosition / scale / offset / swapByte / swapWord / accessMode / unit`

**功能码**：`1`读线圈 · `2`读离散量 · `3`读保持寄存器 · `4`读输入寄存器 · `5`写单线圈 · `6`写单寄存器 · `15`写多线圈 · `16`写多寄存器

**原始类型 rawType**：`int16 / uint16(默认) / int32 / uint32 / float`（占 2 寄存器，大端）/ `bool / bits`

**采集组**（分频）：同组点位共享 `pollInterval` 与 `reportMode`（`periodic` 全量 / `onchange` 仅变更），未分组点位归入默认组（id=0，使用产品 `PollInterval`）。

**轮询引擎**：合并连续寄存器批量读（`maxGap=8`、单次读寄存器上限 120）、变更过滤、并发信号量限流（默认 50）、Redis SETNX 分布式锁防多实例重复采集。

---

## 六、设备控制与下行分发

### 下行通道决策（在 `main.go` 中注入）

| 条件 | 通道 | 实际调用 |
|---|---|---|
| TCP 在线 且 产品为 Modbus | `modbus` | `poller.WriteProperty`（按点位写寄存器/线圈） |
| TCP 在线 且 非 Modbus | `tcp` | `gateway.Send`（有 `encode` 则编码为二进制，否则透传 JSON） |
| TCP 不在线 | `mqtt` | `mqtt.PublishDown`（`thing/down/{pk}/{dn}`，QoS 1） |

每次下发后异步记录 `CommandLog`（channel: mqtt/tcp/modbus）与设备日志。多实例下，TCP 下行通过 Redis 频道 `tcp:down` 路由到持有连接的实例兜底。

### 设备影子（desired / reported）

| 行为 | 说明 |
|---|---|
| 写期望值 | `UpdateShadowDesired` 按 key 合并，`version+1`，立即落库 |
| 在线下发 | 仅推送 `computeDelta` 差异项（`method:property.set, delta:true`） |
| 离线累积 | 写入影子，待上线时 `syncShadowOnConnect` 全量补发 |
| 自动清除 | `reported` 达到 `desired` 值时从 desired 删除该 key |

### 指令应答状态机

下发指令携带 `messageId`（纳秒时间戳），设备在 `thing/up/{pk}/{dn}/reply` 回应 `{messageId, code, data}`：

| 阶段 | Status | 触发 |
|---|---|---|
| 下发时创建 | `pending` | 属性设置 / 服务调用 |
| 设备应答 | `acked` | 收到匹配 `messageId` 的 reply（仅 pending 可翻转，幂等） |
| 超时 | `timeout` | （状态定义存在，目前无自动扫描） |

### 其它下行协议

| 协议 | 上行报文 | 下行报文 |
|---|---|---|
| 服务调用 | — | `{method:"service.invoke", messageId, service, params, ts}` |
| 事件上报 | `{method:"event.post", identifier, type, params}` | — |
| 远程配置拉取 | `{method:"config.get"}` | `{method:"config.push", version, params}` |
| NTP 对时 | `{method:"ntp.request", deviceSendTime}` | `{method:"ntp.response", deviceSendTime, serverRecvTime, serverSendTime}` |
| OTA 下发 | — | `{method:"ota.push", version, url, size, sha256, taskId, ts}` |
| OTA 进度 | `{method:"ota.progress", taskId, progress, status}` | — |

### OTA 升级

1. 上传固件（`POST /api/v1/firmwares`，multipart，自动计算 SHA-256）
2. 创建升级任务（`POST /api/v1/ota-tasks`，选择固件 + 设备列表）
3. 平台下发 `ota.push`；设备通过 `thing/up/{pk}/{dn}/ota` 上报进度
4. 任务状态：`running` →（`success`）`completed` /（`failed`）`failed`

### 网关子设备

网关设备（`isGateway=true`）通过 `thing/gateway/{pk}/{dn}/sub/{subId}/login|logout` 上下线其子设备，载荷含三元组，经鉴权（一型一密可动态注册）后绑定 `gateway_id`。

---

## 七、规则引擎与告警

| 规则类型 | 常量 | 触发机制 | 条件语法 |
|---|---|---|---|
| 阈值告警 | `alarm` | 遥测上行实时评估 | `{field,op,value}` 或复合 `{logic:and\|or, conditions:[...]}` |
| 离线告警 | `offline` | 定时器每分钟巡检 | `{minutes:N}` 比对 `last_offline_at` |
| 数据转发 | `forward` | 遥测上行实时转发 | 无条件，直接转发 |

- **比较运算符**：`> < >= <= == !=`（数值优先，回退字符串/布尔）
- **静默期**：默认 5 分钟，防止告警风暴（多实例走 Redis）
- **自动恢复**：阈值条件不再满足时，`firing` 告警自动置为 `resolved`
- **告警级别**：`warning` / `critical`；状态：`firing` / `resolved`（确认 `confirmed` 为独立时间戳）
- **规则缓存**：按 userID 分组，30 秒 TTL

**转发目标**：Webhook（SSRF 防护 + 指数退避重试，默认 3 次）/ Kafka / MQTT 桥接（QoS 1）

---

## 八、开放平台 OpenAPI

第三方应用通过 HMAC-SHA256 签名访问 `/openapi/v1`，鉴权后以应用归属用户身份复用管理端处理器。

**签名规范**

| 请求头 | 取值 |
|---|---|
| `X-App-Key` | 应用 AppKey |
| `X-Timestamp` | Unix 秒级时间戳（±5 分钟有效） |
| `X-Sign` | `hex(HMAC-SHA256(AppSecret, AppKey + Timestamp))` |

**开放端点**（`/openapi/v1`）

| 方法 | 路径 |
|---|---|
| GET | `/devices` / `/devices/:id` |
| GET | `/devices/:id/latest` / `/devices/:id/history` / `/devices/:id/shadow` |
| POST | `/devices/:id/property` / `/devices/:id/command` |

> 签名仅覆盖 `appKey + timestamp`，不含请求体；持有 AppSecret 即可对应用归属资源签名。

---

## 九、开发者工具

所有开发者工具端点位于 JWT 鉴权的 `/api/v1` 下：

| 端点 | 说明 |
|---|---|
| `POST /simulator/connect` `/publish` `/disconnect` · `GET /simulator/sessions` | 平台内置设备模拟器（内存态会话），以指定设备身份上报遥测/事件 |
| `GET /mqtt-debug/ws` | MQTT 调试台 WebSocket，复用平台内部客户端，支持 publish/subscribe/unsubscribe（QoS 0/1/2，支持 `+/#` 通配） |
| `GET /traces` · `GET /traces/:traceId` | 消息轨迹查询（支持按 traceId/产品/设备/状态/时间过滤，详情含 ingest/decode/store/rule 分阶段耗时） |
| `GET /devices/:id/logs` · `GET /device-logs` | 设备运行日志（按设备维度 / 用户维度，支持 category 过滤） |

**设备日志 category**：`data_up`（上行）/ `data_down`（下行）/ `connection`（上下线/TCP 断开）/ `event`（事件）/ `error`（下行失败）

> 消息轨迹默认保留 7 天、设备日志默认保留 30 天，后台每 6 小时清理一次。

---

## 十、配置参考

配置文件 `server/configs/config.yaml`，可用 `-conf` 覆盖路径。

| 键 | 默认值 | 单位 | 说明 |
|---|---|---|---|
| `server.addr` | `:8080` | - | HTTP 服务监听地址 |
| `gateway.addr` | `:9100` | - | TCP 透传网关监听地址 |
| `gateway.idle_timeout` | `300` | 秒 | 空闲断开并置离线 |
| `gateway.max_conns_per_ip` | `10` | 个 | 单 IP 最大连接数 |
| `gateway.conn_rate_limit` / `conn_rate_burst` | `5` / `10` | 个/秒、个 | 令牌桶速率 / 容量 |
| `gateway.tls.*` | `false` | - | 网关 TLS（cert_file / key_file） |
| `jwt.secret` | `iot-platform-jwt-secret-change-me` | - | JWT 签名密钥（**生产须改**） |
| `jwt.expire_hours` | `72` | 小时 | JWT 有效期 |
| `database.dsn` | `host=127.0.0.1 ... dbname=iot port=5432 ...` | - | PostgreSQL/TimescaleDB 连接串 |
| `database.max_open_conns` / `max_idle_conns` | `50` / `10` | - | 连接池大小 |
| `database.conn_max_lifetime` / `conn_max_idle_time` | `1800` / `600` | 秒 | 连接存活/空闲时间 |
| `database.retention_days` | `30` | 天 | 遥测保留 |
| `database.compress_after_days` | `7` | 天 | 压缩启动 |
| `redis.addr` / `password` / `db` | `127.0.0.1:6379` / `""` / `0` | - | Redis 连接 |
| `redis.pool_size` / `min_idle_conns` | `20` / `5` | - | 连接池 |
| `mqtt.broker` | `tcp://127.0.0.1:1883` | - | EMQX broker |
| `mqtt.client_id` / `username` | `iot-platform-internal` | - | 平台内部客户端（EMQX 放行） |
| `mqtt.password` | `internal-secret-2026` | - | 内部账号密码 |
| `mqtt.tls.*` | `enabled:false` / `insecure_skip_verify:true` | - | MQTT TLS / mTLS |
| `telemetry_buffer.max_batch` | `100` | - | 批量入库最大条数 |
| `telemetry_buffer.flush_interval` | `1` | 秒 | 缓冲 flush 间隔 |
| `cache.device_ttl` | `300` | 秒 | 设备缓存 TTL |
| `cache.shadow_flush_interval` | `5` | 秒 | 设备影子落库间隔 |
| `poller.max_concurrent` | `50` | - | Modbus 最大并发轮询数 |
| `log.trace_retention_days` | `7` | 天 | 消息轨迹保留 |
| `log.device_log_retention_days` | `30` | 天 | 设备日志保留 |
| `emqx_rule.enabled` | `false` | - | EMQX 规则引擎接管遥测入库（快路径） |

---

## 十一、部署与运维

### 多实例水平扩展

以下能力依赖 Redis；Redis 不可用时自动退化为单实例内存模式：

| 能力 | Redis 用途 |
|---|---|
| WebSocket 跨实例扇出 | Pub/Sub 频道 `ws:broadcast` |
| Modbus 轮询去重 | SETNX 分布式锁 `poller:lock:{pid}_{gid}_{dev}` |
| 规则静默期共享 | `silence:{ruleID}:{deviceID}` |
| TCP 下行跨实例路由 | Pub/Sub 频道 `tcp:down` |

### EMQX 规则引擎快路径

启用 `emqx_rule.enabled=true` 后，EMQX 直写 TimescaleDB（`telemetries` 表），后端跳过遥测 DB 写入，仅保留缓存/推送/影子/规则/轨迹。

> **启用前**必须先在 EMQX Dashboard 手动创建 PostgreSQL Connector（`tsdb_telemetry`）与规则（订阅 `thing/up/#`，过滤 `payload.method` 为空），否则遥测无人入库。

### 构建

```powershell
# Windows
cd server; go build -o bin\server.exe .\cmd\server
# Linux 交叉编译
$env:GOOS="linux"; go build -o bin\server-linux .\cmd\server
```

### 测试

```powershell
cd server
go test ./...                              # 全部
go test ./internal/modbus -run TestCRC16   # 单个
```

测试集中在 `internal/gateway`（组帧）与 `internal/modbus`（编解码/批量读）。

### 默认端口一览

| 服务 | 端口 |
|---|---|
| 后端 HTTP | `:8080` |
| TCP 网关 | `:9100` |
| MQTT | `1883` |
| MQTT over WebSocket | `8083` |
| EMQX Dashboard | `18083`（admin/public） |
| PostgreSQL | `5432` |
| Redis | `6379` |
| 前端 dev | `5173` |

---

## 十二、数据存储与性能

### 存储结构

- **PostgreSQL + TimescaleDB**：业务表（User/Product/Device/...）由 GORM AutoMigrate；遥测表 `telemetries` 提升为 TimescaleDB 超表（时间列 `ts`，**无主键**，复合索引 `idx_dev_ts(device_id, ts desc)`），自动保留与压缩（`segmentby=device_id`）。TimescaleDB 扩展不可用时**非致命降级**为普通表。
- **Redis**：最新值缓存、设备令牌、静默期、分布式锁、跨实例 Pub/Sub。

### Redis 键 / 频道一览

| 键/频道 | 类型 | 用途 | TTL |
|---|---|---|---|
| `device:latest:{id}` | String(JSON) | 设备最新遥测（Lua 原子合并） | 常驻 |
| `device:token:{token}` | String(JSON) | 设备换签令牌 | 3600 秒 |
| `silence:{ruleID}:{deviceID}` | String | 告警静默期 | Silence 分钟（默认 5） |
| `poller:lock:{pid}_{gid}_{dev}` | SETNX | Modbus 轮询分布式锁 | 60 秒 |
| `ws:broadcast` | Pub/Sub | WebSocket 跨实例扇出 | — |
| `tcp:down` | Pub/Sub | TCP 下行跨实例路由 | — |

### 性能组件

| 组件 | 作用 |
|---|---|
| TelemetryBuffer | 遥测批量缓冲（满 100 条或每 1 秒 flush），减少 DB 写次数 |
| DeviceCache | 设备内存缓存（`sync.Map`，默认 TTL 5 分钟），降低接入鉴权 DB 压力 |
| ShadowCache | 设备影子定时落库节流（默认 5 秒） |
| Poller 信号量 | Modbus 轮询并发上限（默认 50） |

---

## 十三、目录结构

```
server/                  Go 后端（module: iot-platform）
  cmd/server/            入口（main.go：组件装配、下行分发注入、优雅关闭）
  internal/api/          HTTP 接口 + 路由 + JWT/OpenAPI/EMQX 中间件
  internal/mqtt/         EMQX 对接（订阅上行/系统事件、下行发布）
  internal/gateway/      TCP 透传网关（DTU/组帧/心跳/限流/Modbus 半双工）
  internal/poller/       Modbus 云端轮询引擎（分频/批量读/onchange/分布式锁）
  internal/modbus/       Modbus RTU 编解码与批量读规划
  internal/codec/        goja JS 自定义协议解析（decode/encode）
  internal/service/      业务核心（遥测入库/影子/规则/OTA/缓存/下行路由）
  internal/rule/         规则引擎（告警/转发/离线巡检）
  internal/ws/           WebSocket 推送中心（Redis 跨实例扇出）
  internal/model/        GORM 数据模型
  internal/repository/   DB/Redis 初始化、Timescale 超表、Pub/Sub
  internal/config/       YAML 配置加载
  configs/config.yaml    默认配置
  deploy/                docker-compose + EMQX 配置 + 规则引擎
web/                     Vue3 管理后台
  src/views/             页面（概览/产品/设备/规则/告警/应用/OTA/大屏/工具）
  src/api/index.ts       统一 axios 客户端 + 全部接口定义
  src/utils/realtime.ts  WebSocket 实时客户端（自动重连 + 设备订阅）
  src/components/        物模型/脚本/点位编辑器
tools/simulator/        设备模拟器（MQTT / TCP透传 / TCP+Modbus）
```

---

## 十四、安全注意事项

- **JWT 密钥**默认为占位值 `iot-platform-jwt-secret-change-me`，部署前必须更改。
- **默认管理员** `admin/admin123`、**MQTT 内部账号** `iot-platform-internal/internal-secret-2026`、**数据库** `iot/iot123456` 均为演示值，生产环境务必替换。
- **EMQX 鉴权回调**（`/emqx/auth`、`/emqx/acl`）注册在公开路由组，须通过内网/白名单保护，避免外网伪造 `allow`。
- **Webhook SSRF 防护**仅校验主机名字面量（禁止 localhost、`169.254.169.254`、回环与链路本地地址），不解析 DNS，无法防御域名解析到内网的场景。
- **OTA 通知多实例盲区**：OTA 下发经单实例 `DownPublisher`，若目标 TCP 连接在其它实例会 fallback 到 MQTT，纯 TCP/透传设备可能收不到 `ota.push`。
- `mqtt.tls.insecure_skip_verify` 默认 `true`，生产启用 TLS 时应关闭并配置 CA 证书。

---

## 路线图

- [x] **一期**：用户/产品/设备管理、MQTT 接入、遥测存储、实时监控、命令下发
- [x] **二期**：物模型(TSL)、设备影子（离线补发）、规则引擎（阈值/离线告警、数据转发、静默期）、告警中心
- [x] **三期**：TCP 透传网关(DTU)、JS 脚本协议解析、OpenAPI(HMAC 签名)、可视化大屏
- [x] **四期**：Modbus 云端轮询（分组分频 + 批量读取 + 变更上报）
- [x] **五期**：OTA 升级、远程配置、网关子设备、设备模拟器、MQTT 调试台、消息轨迹
- [x] **六期**：多实例支持、EMQX 规则引擎集成、性能优化、开发者工具完善
