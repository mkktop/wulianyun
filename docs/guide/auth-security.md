# 认证与安全

> 平台有两套完全独立的认证体系：**管理端/开放 API**（JWT + HMAC 签名）与**设备接入**（Secret / 动态令牌）。本文档汇总所有认证机制与安全约定。

## 一、设备接入认证

### 1.1 连接参数

| 项 | 值 |
|---|---|
| ClientID | `{productId}.{deviceName}` |
| Username | `{deviceName}` |
| Password | 设备 Secret（一机一密）或产品 `ProductSecret`（一型一密）或 `tk:{token}` 动态令牌 |

### 1.2 密钥模式

| secretMode | 说明 |
|---|---|
| `device`（一机一密，默认） | 设备须预先创建，校验设备级 Secret |
| `product`（一型一密） | 创建产品时生成 `ProductSecret`；设备不存在且 `secret == ProductSecret` 时**自动注册**并签发独立 Secret |

### 1.3 动态令牌（tk:）

避免长期 Secret 暴露在链路上，可使用短期令牌（默认 1 小时）：

```http
POST /api/v1/auth/token
Content-Type: application/json

{ "productId": "pk...", "deviceName": "dev1", "secret": "..." }
```

```json
{ "code": 0, "data": { "token": "tk:1f2e...", "ttl": 3600 } }
```

- 格式：`tk:` + 32 位 hex（16 随机字节）
- 存于 Redis（`device:token:{token}`），TTL 默认 3600 秒，过期即失效
- 可撤销（`RevokeDeviceToken`）

### 1.4 EMQX 回调（HTTP 内网）

设备连接时 EMQX 回调平台后端：

```
POST /api/v1/emqx/auth    （连接鉴权）
POST /api/v1/emqx/acl     （Topic 授权）
```

- 需在**内网/白名单**保护，避免外网伪造 `allow`
- 放行平台内部超级账号（`mqtt.username` + `mqtt.password`）
- ACL 未命中一律 `deny`（`no_match=deny`）

### 1.5 HTTP 直传鉴权

`POST /api/v1/http/telemetry`，头 `X-Device-Token: Base64(productId:deviceName:secret)`（仅静态 Secret，不支持 `tk:`）。

## 二、管理端认证（JWT）

| 项 | 值 |
|---|---|
| Header | `Authorization: Bearer <token>`（WebSocket 支持 `?token=`） |
| 算法 | HS256 |
| 有效期 | 默认 72 小时（`jwt.expire_hours`） |
| Claims | `uid` / `username` / `role` |

登录接口：

```http
POST /api/v1/auth/login
Content-Type: application/json

{ "username": "admin", "password": "..." }
```

```json
{ "code": 0, "data": { "token": "eyJ...", "user": { "id": 1, "username": "admin" } } }
```

- 注册：`POST /api/v1/auth/register`（用户名 ≥3 位，密码 ≥6 位，bcrypt 存储）
- 默认管理员：`admin / admin123`（生产部署可通过 `admin_password` 配置覆盖）

## 三、开放平台认证（HMAC-SHA256）

见 [开放平台OpenAPI](/guide/openapi)：

| 头 | 值 |
|---|---|
| `X-App-Key` | AppKey |
| `X-Timestamp` | Unix 秒级（±5 分钟） |
| `X-Sign` | `hex(HMAC-SHA256(AppSecret, AppKey + Timestamp))` |

## 四、统一响应信封

```json
{ "code": 0, "msg": "ok", "data": { } }
```

- `code === 0` 成功；业务失败 HTTP 200 + 非零 `code`
- 鉴权失败 HTTP 401 + `{code: 401, msg: "..."}`
- 分页：`{ "total": N, "list": [...] }`

## 五、安全注意事项

| 项 | 说明 |
|---|---|
| **JWT 密钥** | 默认 `iot-platform-jwt-secret-change-me`，生产必须更换 |
| **默认账号** | `admin/admin123`、MQTT 内部账号 `iot-platform-internal/internal-secret-2026`、DB `iot/iot123456` 均为演示值，生产必须替换 |
| **EMQX 回调** | `/emqx/auth`、`/emqx/acl` 注册在公开路由组，须经内网/白名单保护 |
| **OTA /uploads** | 公开静态下载（文件名随机化），如需保密请加鉴权 |
| **Webhook SSRF 防护** | 仅校验主机名字面量，不解析 DNS，无法防御域名解析到内网场景 |
| **HTTP 上报无限流** | 生产环境建议边缘加限流 |
| **OpenAPI 无防重放** | 签名仅覆盖 appKey+timestamp，无 nonce |

## 六、Redis 键一览

| 键/频道 | 用途 | TTL |
|---|---|---|
| `device:latest:{id}` | 设备最新遥测缓存 | 常驻 |
| `device:token:{token}` | 设备动态令牌 | 3600 秒 |
| `silence:{ruleID}:{deviceID}` | 告警静默期 | 静默分钟数 |
| `poller:lock:{pid}_{gid}_{dev}` | Modbus 轮询分布式锁 | 60 秒 |
| `ws:broadcast` | WebSocket 跨实例扇出 | — |
| `tcp:down` | TCP 下行跨实例路由 | — |