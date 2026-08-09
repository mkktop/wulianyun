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
- 令牌存于服务端，TTL 默认 3600 秒，过期即失效
- **绑定签发时设备**：令牌记录 clientid 与设备密钥，仅该设备可用；轮转设备 Secret 或禁用设备后旧令牌立即失效
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

`POST /api/v1/http/telemetry`，头 `X-Device-Token: Base64(productId:deviceName:secret)`（仅静态 Secret，不支持 `tk:`）。禁用设备的密钥未轮转也会被拒绝（与 MQTT/TCP 一致）。

## 二、管理端认证（JWT）

| 项 | 值 |
|---|---|
| Header | `Authorization: Bearer <token>` |
| 算法 | HS256 |
| 有效期 | 默认 72 小时（`jwt.expire_hours`） |
| Claims | `uid` / `username` / `role` / `pid` |

> 中间件每个请求回查数据库：**账号被禁用/删除即时失效，角色/层级变更立即生效**（不依赖 token 内可能陈旧的 claim）。

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
- 登录对"用户不存在"与"密码错误"采用等耗时比较（固定 bcrypt 对照），防用户名枚举时序攻击

### 2.1 WebSocket 认证

实时推送 WS（`/api/v1/ws`）采用**首帧认证**：升级后 5 秒内必须发送 `{"type":"auth","token":"<JWT>"}`，认证失败返回 `{"type":"auth_failed"}` 并关闭。token 不放进 URL（避免泄露进 nginx/代理访问日志），客户端收到 `auth_failed` 应跳转登录。MQTT 调试台 WS（`/api/v1/mqtt-debug/ws`）仅平台超管可用，支持 `?token=` 传参。

## 三、开放平台认证（HMAC-SHA256）

见 [开放平台OpenAPI](/guide/openapi)：

| 头 | 值 |
|---|---|
| `X-App-Key` | AppKey |
| `X-Timestamp` | Unix 秒级（±5 分钟） |
| `X-Sign` | `hex(HMAC-SHA256(AppSecret, Method\nPathAndQuery\nBodyHash\nAppKey\nTimestamp))` |

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
| **JWT 密钥** | 默认 `iot-platform-jwt-secret-change-me`（仓库公开占位符，可据此伪造 admin 令牌）；加载时对默认/过短密钥告警，**生产必须更换**。交付包 `install.sh` 会生成随机强密钥 |
| **默认账号** | `admin/admin123` 为首次启动演示值（可由 `admin_password` 覆盖）；MQTT 内部账号、DB 密码、EMQX 面板密码在交付包安装时随机生成——请勿在生产沿用任何仓库内明文 |
| **EMQX 回调** | `/emqx/auth`、`/emqx/acl` 注册在公开路由组，须经内网/白名单保护 |
| **OTA /uploads** | 公开静态下载（文件名随机化，强制 `application/octet-stream` + 附件方式下载 + `nosniff`，杜绝存储型 XSS；扩展名白名单 bin/hex/img/zip 等）；如需保密请加鉴权 |
| **Webhook SSRF 防护** | URL 校验 + DialContext 解析后拦截私网/回环/云元数据地址 + 重定向逐跳校验 |
| **HTTP 上报限流** | 生产环境建议边缘加限流；服务端校验禁用设备 |
| **OpenAPI 签名** | 签名覆盖 Method+PathAndQuery+BodyHash+AppKey+Timestamp，防篡改重放；同一请求在窗口内可原样重放，无 nonce |

> 完整错误码与响应结构见 [错误码与响应参考](/guide/errors)。