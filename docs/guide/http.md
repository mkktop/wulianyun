# HTTP 直传接入协议

> 适用于弱网络设备、一次性上报场景：设备通过 HTTP 请求直接把遥测数据 POST 给平台，无需保持长连接。鉴权方式为请求头 `X-Device-Token`（Base64 编码的三元组）。
>
> 相关：报文格式 → [MQTT接入协议](/guide/mqtt) · 动态令牌 → [认证与安全](/guide/auth-security)

## 一、上报端点

```
POST /api/v1/http/telemetry
```

| 项 | 值 |
|---|---|
| Header | `X-Device-Token: Base64(productId:deviceName:secret)` |
| Body | 遥测 JSON（属性名→值，或 `method` 分流报文） |
| Content-Type | `application/json` |

**示例：**

```http
POST /api/v1/http/telemetry HTTP/1.1
Host: <平台地址>
X-Device-Token: cGsxMjM6ZGV2MTpzZWNyZXQxMjM=
Content-Type: application/json

{ "temperature": 25.5, "humidity": 60.2 }
```

其中 `cGsxMjM6ZGV2MTpzZWNyZXQxMjM=` = `base64("pk123:dev1:secret123")`。

> ⚠️ 该头使用的是**设备静态 Secret**（或一型一密的产品 `ProductSecret`），**不支持 `tk:` 动态令牌**。动态令牌只用于 MQTT password。

## 二、鉴权逻辑

1. 取 `X-Device-Token` 头 → Base64 解码 → 按 `:` 拆分为恰好 3 段：`productId / deviceName / secret`
2. 调用 `FindDeviceForAuth` 校验（兼容一机一密 / 一型一密）：
   - 一机一密：仅接受设备独立 Secret
   - 一型一密：接受设备 Secret 或产品 `ProductSecret`；设备不存在且密钥匹配 `ProductSecret` 时**自动注册**新设备
3. 校验通过后异步摄入遥测，立即返回成功

## 三、响应

**成功（HTTP 200）：**

```json
{ "code": 0, "msg": "ok" }
```

> 注意：响应在异步处理**之前**返回，遥测摄入失败（JSON 解析失败、TSL 校验失败等）只记日志，不影响此 200 响应。

**错误：**

| HTTP 状态码 | body | 触发条件 |
|---|---|---|
| 401 | `{"code":401,"msg":"missing X-Device-Token"}` | 缺少请求头 |
| 401 | `{"code":401,"msg":"invalid token encoding"}` | Base64 解码失败 |
| 401 | `{"code":401,"msg":"invalid token format"}` | 三元组非 3 段 |
| 401 | `{"code":401,"msg":"auth failed"}` | 产品不存在 / 设备不存在 / 密钥错误（统一返回） |
| 400 | `{"code":400,"msg":"read body failed"}` | 读取请求体失败 |

## 四、请求体格式

与 MQTT 上行完全一致（同一摄入管线）：

```json
{
  "temperature": 25.5,
  "humidity": 60.2,
  "switch": 1
}
```

支持 `method` 分流报文（事件上报 / NTP 对时 / 配置拉取）：

```json
{ "method": "event.post", "identifier": "highTemp", "type": "alert", "params": { "temperature": 35.2 } }
```

## 五、限流说明

HTTP 上报端点**当前无速率限制**（仅有 TCP 网关限流）。生产环境建议在边缘网关/负载均衡层自行加限流。

## 六、动态令牌换取（可选）

虽然 HTTP 上报不支持 `tk:` 令牌，但设备可用三元组换取 MQTT 令牌：

```http
POST /api/v1/auth/token
Content-Type: application/json

{ "productId": "pk123", "deviceName": "dev1", "secret": "secret123" }
```

```json
{ "code": 0, "data": { "token": "tk:1f2e...", "ttl": 3600 } }
```

## 七、示例（curl）

```bash
curl -X POST http://<平台地址>/api/v1/http/telemetry \
  -H "X-Device-Token: $(printf 'pk123:dev1:secret123' | base64)" \
  -H "Content-Type: application/json" \
  -d '{"temperature":25.5,"humidity":60.2}'
```

> 完整可运行示例见 [示例代码](/guide/examples)。