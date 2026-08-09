# 开放平台 OpenAPI

> 第三方应用通过 **HMAC-SHA256 签名**访问 `/openapi/v1`，以应用归属用户的身份复用管理端能力（设备查询、实时数据、命令下发）。无需登录用户名密码，凭 AppKey/AppSecret 即可对接。
>
> 相关：响应信封 → [认证与安全](/guide/auth-security)

## 一、创建应用

1. 登录平台 Web 控制台 → **应用管理** → 新建应用（仅需填名称）
2. 创建后获得：

| 凭据 | 示例 | 说明 |
|---|---|---|
| `appKey` | `ak3f8a1c2d9e4b...` | 固定以 `ak` 开头 + 16 位 hex |
| `appSecret` | `c9d2f1e8...`（32 位 hex） | **保密**，仅创建时展示 |

> 管理端 API：`POST /api/v1/apps`（创建）、`GET /api/v1/apps`（列表）、`PUT /api/v1/apps/:id`（改名称/启停）、`DELETE /api/v1/apps/:id`（删除）。

## 二、签名鉴权

每个请求携带三个签名头：

| 请求头 | 取值 |
|---|---|
| `X-App-Key` | 应用 AppKey |
| `X-Timestamp` | Unix **秒级**时间戳（±5 分钟有效） |
| `X-Sign` | `hex(HMAC-SHA256(AppSecret, 待签名串))` |

### 签名计算

待签名串覆盖 **Method + PathAndQuery + BodyHash**，防止捕获的签名被改写为其他请求：

```text
BodyHash   = hex(SHA256(请求体))                      （GET 无请求体时为空串的哈希）
待签名串    = Method + "\n" + PathAndQuery + "\n" + BodyHash + "\n" + AppKey + "\n" + Timestamp
PathAndQuery = 请求路径（含 query，如 /openapi/v1/devices/3/latest）
签名        = hex(HMAC-SHA256(AppSecret, 待签名串))
```

示例：`GET /openapi/v1/devices/3/latest?size=10`、`appKey=ak3f8a1c...`、`ts=1785712345`：

```text
待签名串 = GET\n/openapi/v1/devices/3/latest?size=10\ne3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855\nak3f8a1c...\n1785712345
```

### 校验流程

1. 缺任一签名头 → 401
2. 时间戳超出 ±5 分钟 → 401
3. AppKey 不存在或 `enabled=false` → 401
4. 签名不匹配（常量时间比较）→ 401

### 示例（Node.js）

```js
const crypto = require('crypto')

// 待签名串 = Method\nPathAndQuery\nBodyHash\nAppKey\nTimestamp
function sign(method, pathAndQuery, body, appKey, appSecret, ts) {
  const bodyHash = crypto.createHash('sha256').update(body).digest('hex')
  const mac = crypto.createHmac('sha256', appSecret)
  mac.update(`${method}\n${pathAndQuery}\n${bodyHash}\n${appKey}\n${ts}`)
  return mac.digest('hex')
}

const appKey = 'ak3f8a1c...'
const appSecret = 'c9d2f1e8...'
const ts = Math.floor(Date.now() / 1000)
const path = '/openapi/v1/devices'

const res = await fetch(`http://<平台地址>${path}`, {
  headers: {
    'X-App-Key': appKey,
    'X-Timestamp': String(ts),
    'X-Sign': sign('GET', path, '', appKey, appSecret, ts),
  },
})
```

> POST 请求时 `pathAndQuery` 需包含 query 字符串、`body` 传请求体原始字符串，且两端必须与发出请求完全一致（签名绑定 method/path/body）。

### 示例（Python）

```python
import time, hmac, hashlib, requests

app_key = "ak3f8a1c..."
app_secret = "c9d2f1e8..."
ts = str(int(time.time()))
path = "/openapi/v1/devices"
body_hash = hashlib.sha256(b"").hexdigest()
raw = f"GET\n{path}\n{body_hash}\n{app_key}\n{ts}"
sign = hmac.new(app_secret.encode(), raw.encode(), hashlib.sha256).hexdigest()

r = requests.get(
    "http://<平台地址>" + path,
    headers={"X-App-Key": app_key, "X-Timestamp": ts, "X-Sign": sign},
)
print(r.json())
```

> 签名覆盖 **Method + PathAndQuery + BodyHash + AppKey + Timestamp**，捕获的签名无法被改写为其他请求（不同 method/path/body 会得到不同签名）。同一请求在 ±5 分钟时间窗口内可被原样重放，当前无 nonce 防重放机制。

## 三、端点列表

Base path：`/openapi/v1`（**不带** `/api/v1` 前缀）。各端点的完整字段、请求/响应结构见 [OpenAPI 字段参考](/guide/api-reference)。

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/openapi/v1/devices` | 分页设备列表；query：`productId`（产品数字 ID）、`keyword`、`status`、`groupId`、`page`、`size`（上限 100） |
| GET | `/openapi/v1/devices/:id` | 设备详情（`:id` 为设备数字 ID） |
| GET | `/openapi/v1/devices/:id/latest` | 设备最新遥测 |
| GET | `/openapi/v1/devices/:id/history` | 历史遥测；query：`start`（毫秒）、`end`（毫秒）、`limit`（默认 2000，上限 **50000**，无分页） |
| GET | `/openapi/v1/devices/:id/shadow` | 设备影子 |
| POST | `/openapi/v1/devices/:id/property` | 属性设置：`{"params": {...}, "expireSec": 0}` |
| POST | `/openapi/v1/devices/:id/command` | 透传命令：body 为**任意原始 JSON**（不生成 messageId，无应答状态机） |

> 设备/产品标识：路径中的 `:id` 与 query 中的 `productId` 均为**数字 ID**（数据库主键），不是产品标识字符串。

### 属性设置示例

```http
POST /openapi/v1/devices/3/property
X-App-Key: ak3f8a1c...
X-Timestamp: 1785712345
X-Sign: ...

{ "params": { "switch": 1 }, "expireSec": 0 }
```

```json
{
  "code": 0,
  "data": {
    "shadow": { "desired": { "switch": 1 }, "reported": {} },
    "delivered": true,
    "note": "已下发",
    "messageId": "1785712345678901234"
  }
}
```

### 透传命令示例

```http
POST /openapi/v1/devices/3/command
X-App-Key: ak3f8a1c...
X-Timestamp: 1785712345
X-Sign: ...

{ "action": "reboot" }
```

> 要求设备在线，否则 `400 设备不在线`。

## 四、作用域

- 所有请求以**应用归属用户**的身份执行，只能访问该用户名下的设备/资源
- 不属于该用户的设备 ID 返回 `404 设备不存在`

## 五、注意事项

- **CORS**：`AllowHeaders` 不含 `X-App-Key/X-Timestamp/X-Sign`，浏览器跨域直接调用会被预检拒绝；服务端 / curl / 设备端不受影响
- 端点不支持 `?token=` 方式，仅接受三个签名头
- 设备 ID 为数据库主键（uint），非 DeviceName

## 六、错误码

| HTTP | code | 触发条件 |
|---|---|---|
| 401 | 401 | 缺少签名头 / 时间戳无效或过期 / AppKey 无效或已禁用 / 签名错误 |
| 404 | 404 | 设备不存在（或不属于该应用） |
| 400 | 400 | 属性参数非法 / 设备不在线 / 命令非合法 JSON |