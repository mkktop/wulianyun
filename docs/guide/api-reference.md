# OpenAPI 端点字段参考

> 本文档逐一列出 `/openapi/v1` 全部端点的请求参数与响应结构，是第三方应用对接的完整字段参考。
>
> 签名鉴权与创建应用见 [开放平台 OpenAPI](/guide/openapi)；错误码见 [错误码与响应参考](/guide/errors)。

## 一、通用约定

- **Base path**：`/openapi/v1`（不带 `/api/v1` 前缀）
- **请求头**（每个请求必带）：`X-App-Key` / `X-Timestamp`（Unix 秒级，±5 分钟）/ `X-Sign`
- **签名**：`hex(HMAC-SHA256(AppSecret, Method + "\n" + PathAndQuery + "\n" + hex(SHA256(Body)) + "\n" + AppKey + "\n" + Timestamp))`；POST 的 BodyHash 必须与实际发送字节一致
- **标识**：路径 `:id` 与 query 中 `productId`/`groupId` 均为**数字 ID**；设备名/产品标识字符串只在设备三元组与 MQTT 中使用
- **作用域**：请求以应用归属用户身份执行，只能访问其名下资源；越权访问统一返回 `404 设备不存在`
- **写端点**（property/command）受应用归属账号权限约束：查看者（view）权限账号不可写
- **分页**：`page`（默认 1）+ `size`（默认 10，上限 100）

## 二、端点详情

### 2.1 设备列表

```
GET /openapi/v1/devices
```

| Query | 类型 | 必填 | 说明 |
|---|---|---|---|
| `productId` | number | 否 | 按产品过滤（产品数字 ID） |
| `keyword` | string | 否 | 按设备名模糊匹配 |
| `status` | string | 否 | `inactive` / `online` / `offline` / `disabled` |
| `groupId` | number | 否 | 按分组过滤 |
| `page` | number | 否 | 页码（默认 1） |
| `size` | number | 否 | 每页条数（默认 10，上限 100） |

响应 `data`：

```json
{
  "total": 2,
  "list": [
    {
      "id": 3,
      "productDbId": 7,
      "productId": "AB1234567890",
      "name": "dev1",
      "status": "online",
      "lastOnlineAt": "2026-08-09T03:00:00Z",
      "lastOfflineAt": null
    }
  ]
}
```

> `productId`（响应内）为**产品标识字符串**；`productDbId` 为产品数字 ID。

### 2.2 设备详情

```
GET /openapi/v1/devices/:id
```

响应 `data` 为单个设备对象（字段同列表项，另含 `secret`、`regCode`、`groupId`、`isGateway`、`tags`、`remark`、`createdAt` 等）。

### 2.3 设备最新遥测

```
GET /openapi/v1/devices/:id/latest
```

响应 `data`：

```json
{
  "ts": 1786243503278,
  "data": { "temperature": 25.5, "humidity": 60.2, "switch": 1 }
}
```

- `ts`：毫秒时间戳
- `data`：最新属性值（仅包含物模型已定义属性；无数据时 `data` 为 `null`）

### 2.4 历史遥测

```
GET /openapi/v1/devices/:id/history
```

| Query | 类型 | 必填 | 说明 |
|---|---|---|---|
| `start` | number | 否 | 起始毫秒时间戳（默认 1 小时前） |
| `end` | number | 否 | 结束毫秒时间戳（默认当前） |
| `limit` | number | 否 | 最大条数（默认 2000，上限 **50000**，无分页） |

响应 `data`（按时间升序）：

```json
[
  { "ts": 1786243440000, "data": { "temperature": 25.4, "humidity": 60.1 } },
  { "ts": 1786243500000, "data": { "temperature": 25.5, "humidity": 60.2 } }
]
```

> 仅返回物模型校验通过（`valid=true`）的报文。

### 2.5 设备影子

```
GET /openapi/v1/devices/:id/shadow
```

响应 `data`：

```json
{
  "id": 9,
  "deviceId": 3,
  "desired": { "switch": 1 },
  "reported": { "temperature": 25.5, "switch": 1 },
  "version": 3,
  "updatedAt": "2026-08-09T03:00:00Z"
}
```

- `desired`：期望值（待设备达成的目标状态）
- `reported`：实际上报值（设备达成 desired 后对应键自动从 desired 移除）
- `version`：每次写入自增

### 2.6 属性设置

```
POST /openapi/v1/devices/:id/property
Content-Type: application/json
```

请求体（两种格式任选）：

```json
{ "params": { "switch": 1 }, "expireSec": 0 }
```

```json
{ "switch": 1 }
```

| 字段 | 类型 | 说明 |
|---|---|---|
| `params` | object | 期望设置的属性键值（有物模型时仅可写 `rw` 属性生效） |
| `expireSec` | number | 过期秒数（0=不过期）；仅在 `params` 格式下有效 |

行为：

1. 写入影子 `desired`（合并，不覆盖未涉及的键）
2. 设备在线 → 立即下发 `property.set`（QoS 1，仅推送差异项）；设备离线 → 写入影子待上线补发
3. 设备应答后，平台将 `messageId` 对应的指令记录置为 `acked`

响应 `data`：

```json
{
  "shadow": { "desired": { "switch": 1 }, "reported": {}, "version": 2 },
  "delivered": true,
  "note": "已下发",
  "messageId": "1785712345678901234"
}
```

| 字段 | 说明 |
|---|---|
| `delivered` | 设备当前是否在线并已下发 |
| `note` | `已下发` 或 `设备离线，已写入影子待上线补发` |
| `messageId` | 本次指令标识；设备应答时在 `thing/up/{pk}/{dn}/reply` 回带 |

### 2.7 透传命令

```
POST /openapi/v1/devices/:id/command
Content-Type: application/json
```

请求体为**任意合法 JSON 对象，原样透传**：

```json
{ "action": "reboot", "timeout": 30 }
```

- 要求设备在线，否则 `400 设备不在线`
- 不生成 messageId、无应答状态机（设备收到后自行处理）
- 响应：`{"code":0,"msg":"ok"}`

## 三、签名计算速查

```text
BodyHash     = hex(SHA256(请求体))            // GET 无请求体：空串哈希
PathAndQuery = 请求路径含 query               // 如 /openapi/v1/devices?size=10
待签名串      = Method + "\n" + PathAndQuery + "\n" + BodyHash + "\n" + AppKey + "\n" + Timestamp
X-Sign       = hex(HMAC-SHA256(AppSecret, 待签名串))
```

Node.js / Python 完整示例（含 POST 带请求体）见 [示例代码](/guide/examples)。

## 四、常见排障

| 现象 | 原因 |
|---|---|
| 401 `签名错误` | body 与签名时字节不一致（含空白）、时间戳不是秒级、路径漏 query |
| 401 `时间戳无效或已过期` | 设备时钟偏差 > 5 分钟 |
| 404 `设备不存在` | 设备 ID 不属于该应用归属用户（越权统一按不存在返回） |
| 400 `设备不在线` | 命令/属性下发时设备离线（属性设置可离线，命令不可以） |
| CORS 预检失败 | 浏览器跨域不被支持；请使用服务端 / 设备端调用 |
