# 下行控制与设备影子

> 平台通过三条管理端 API 下发指令，统一汇入 `DownPublisher` 按设备所在通道自动选路（TCP 网关 → Modbus 写点 / 透传；否则 MQTT `thing/down`）。每次下发异步记录指令日志与设备日志。
>
> 相关：MQTT 下行报文 → [MQTT接入协议](/guide/mqtt) · 开放 API → [开放平台OpenAPI](/guide/openapi)

## 一、下行通道决策

| 条件 | 通道 | 实际调用 |
|---|---|---|
| TCP 在线 且 产品为 Modbus | `modbus` | `poller.WriteProperty`（按点位表写寄存器/线圈） |
| TCP 在线 且 非 Modbus | `tcp` | `gateway.Send`（有 `encode` 脚本则编码为二进制，否则透传 JSON） |
| TCP 不在线 | `mqtt` | `mqtt.PublishDown`（`thing/down/{pk}/{dn}`，QoS 1） |

> 多实例下，TCP 下行通过 Redis 频道 `tcp:down` 路由到持有连接的实例兜底。

## 二、三条下发 API

### 2.1 透传命令

```
POST /api/v1/devices/:id/command
```

请求体为**任意合法 JSON，原样透传**（不做任何包裹）：

```json
{ "action": "reboot", "timeout": 30 }
```

- 要求设备 `online`，否则 `400 设备不在线`
- 成功返回 `{"code":0,"msg":"ok"}`

### 2.2 属性设置（写影子 + 在线下发）

```
POST /api/v1/devices/:id/property
Content-Type: application/json

{ "params": { "switch": 1 }, "expireSec": 0 }
```

行为：

1. 写入/合并影子 `desired`（`expireSec` 为过期秒数，0=不过期）
2. 设备在线 → 立即下发 **delta** 差异项：`{"method":"property.set","params":{...},"delta":true,"ts":<ms>}`
3. 设备离线 → 写入影子，待上线时 `syncShadowOnConnect` 全量补发
4. 创建 `CommandRequest`（status=`pending`，超时 10s），等待设备应答

响应：

```json
{
  "code": 0,
  "data": {
    "shadow": {},
    "delivered": true,
    "note": "已下发",
    "messageId": "1785712345678901234"
  }
}
```

### 2.3 服务调用

```
POST /api/v1/devices/:id/service
Content-Type: application/json

{ "service": "reset", "params": { "code": 123 } }
```

下行报文：

```json
{
  "method": "service.invoke",
  "messageId": "1785712345678901234",
  "service": "reset",
  "params": { "code": 123 },
  "ts": 1785712345678
}
```

- 要求设备 `online`（离线直接 400，不像属性设置可缓存）
- 成功返回 `{"code":0,"data":{"messageId":"..."}}`

## 三、指令应答（messageId 机制）

设备收到 `service.invoke` / `property.set` 后，向 `thing/up/{pk}/{dn}/reply` 发布应答：

```json
{ "messageId": "1785712345678901234", "code": 0, "data": { } }
```

平台匹配 `messageId` 且状态为 `pending` 的 `CommandRequest`，置为 `acked` 并记录应答：

| 阶段 | 状态 | 触发 |
|---|---|---|
| 下发时创建 | `pending` | 属性设置 / 服务调用 |
| 设备应答 | `acked` | 收到匹配 `messageId` 的 reply（仅 pending 可翻转，幂等） |
| 超时 | `timeout` | （状态定义存在，当前无自动扫描） |

查询：`GET /api/v1/command-logs`（支持 productId/deviceId 过滤）。

> `messageId` 为纳秒时间戳字符串。MQTT 透传命令（`SendCommand`）不生成 messageId。

## 四、设备影子（desired / reported）

| 行为 | 说明 |
|---|---|
| 写期望值 | `UpdateShadowDesired` 按 key 合并，`version+1`，立即落库 |
| 在线下发 | 仅推送 `computeDelta` 差异项（`method:property.set, delta:true`） |
| 离线累积 | 写入影子，待上线时 `syncShadowOnConnect` 全量补发 |
| 自动清除 | `reported` 达到 `desired` 值时从 desired 删除该 key |

**设备侧闭环**：收到 `property.set` → 应用属性 → **立即上报新值**（普通遥测 `thing/up/{pk}/{dn}`）→ 平台合并到 `reported` 并清除 desired：

```json
{ "switch": 1 }
```

> 影子 Retained 主题：`thing/down/{pk}/{dn}/retained`（`retained=true`），用于期望值/配置的持久下发，设备上线即可取到。

## 五、产品广播

```
POST /api/v1/products/:id/broadcast
Content-Type: application/json

{ "payload": { "notice": "系统维护" } }
```

同时下发到：

- 所有在线 TCP 连接（逐连接写入）
- MQTT 主题 `thing/broadcast/{pk}`（QoS 1）

设备订阅 `thing/broadcast/{pk}` 即可收到。远程配置推送（`POST /api/v1/products/:id/config/push`）同样走广播：`{"method":"config.push","version":N,"params":{...}}`。

## 六、其它下行报文

| 协议 | 报文 |
|---|---|
| NTP 对时 | `{"method":"ntp.response","deviceSendTime":<ms>,"serverRecvTime":<ms>,"serverSendTime":<ms>}` |
| OTA 升级 | `{"method":"ota.push","version":"...","url":"...","size":N,"sha256":"...","taskId":N,"ts":<ms>}` |

> 详见 [OTA升级](/guide/ota)。