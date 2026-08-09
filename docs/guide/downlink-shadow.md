# 下行控制与设备影子

> 平台提供三类下行能力，按设备接入通道自动选路：设备通过 TCP 网关在线时走网关（Modbus 产品写寄存器、透传产品按脚本编码或透传），否则走 MQTT `thing/down` 主题（QoS 1）。每次下发异步记录指令日志。
>
> 相关：MQTT 下行报文 → [MQTT接入协议](/guide/mqtt) · 开放 API → [开放平台OpenAPI](/guide/openapi)

## 一、三类下行能力对比

| 能力 | API | 是否走影子 | 离线处理 | 应答状态机 | messageId |
|---|---|---|---|---|---|
| **透传命令** | `POST /devices/:id/command` | 否 | 拒绝（需在线） | 无 | 无 |
| **属性设置** | `POST /devices/:id/property` | 是（写 desired） | 缓存待上线补发 | 有（pending→acked） | 有 |
| **服务调用** | `POST /devices/:id/service` | 否 | 拒绝（需在线） | 有（pending→acked） | 有 |

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
- **不生成 messageId、不走应答状态机**（如需应答请用属性设置或服务调用）
- 成功返回 `{"code":0,"msg":"ok"}`

### 2.2 属性设置（写影子 + 在线下发）

```
POST /api/v1/devices/:id/property
Content-Type: application/json

{ "params": { "switch": 1 }, "expireSec": 0 }
```

行为：

1. 写入/合并影子 `desired`（`expireSec` 为过期秒数，0=不过期）
2. 设备在线 → 立即下发 **差异项**：`{"method":"property.set","params":{...},"delta":true,"ts":<ms>}`（QoS 1）
3. 设备离线 → 写入影子，待设备上线时全量补发未达成的期望值
4. 平台记录一条待应答指令（状态 `pending`，超时 10s），等待设备应答

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

下行报文（QoS 1）：

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

> 服务调用仅在管理端 API 提供，**未开放给 OpenAPI 第三方应用**。

## 三、指令应答（messageId 机制）

设备收到 `service.invoke` / `property.set` 后，向 `thing/up/{pk}/{dn}/reply` 发布应答：

```json
{ "messageId": "1785712345678901234", "code": 0, "data": { } }
```

平台匹配 `messageId` 对应的待应答指令，置为 `acked` 并记录应答内容：

| 阶段 | 状态 | 触发 |
|---|---|---|
| 下发时 | `pending` | 属性设置 / 服务调用 |
| 设备应答 | `acked` | 收到匹配 `messageId` 的 reply（仅 pending 可翻转，幂等） |
| 超时 | `timeout` | 状态值已定义，当前无自动扫描将其置为 timeout |

查询指令记录：`GET /api/v1/command-logs`（支持按产品/设备过滤）。

> `messageId` 为平台生成的纳秒时间戳字符串。**透传命令（`/devices/:id/command`）不生成 messageId，也不进入应答状态机。**

## 四、设备影子（desired / reported）

| 行为 | 说明 |
|---|---|
| 写期望值 | 按 key 合并到 `desired`，`version` 自增，立即持久化 |
| 在线下发 | 仅推送与当前 `reported` 不同的差异项 |
| 离线累积 | 写入影子，待设备上线时全量补发未达成的期望值 |
| 自动清除 | 设备上报值达到 `desired` 值时，从 desired 移除该 key |

**设备侧闭环**：收到 `property.set` → 应用属性 → **立即上报新值**（普通遥测 `thing/up/{pk}/{dn}`）→ 平台合并到 `reported` 并清除对应 desired：

```json
{ "switch": 1 }
```

> 影子 Retained 主题 `thing/down/{pk}/{dn}/retained`（retained=true）用于期望值的持久下发，设备上线订阅即可取到。该主题的 `ts` 字段为**字符串**形态，普通 `thing/down` 下行为**数字**形态，解析时请兼容。

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