# MQTT 接入协议

> 平台通过 **EMQX 5.x** 提供 MQTT 设备接入。设备连接由 EMQX 回调平台后端 `/api/v1/emqx/auth` 做连接鉴权，每个 Topic 的发布/订阅由 `/api/v1/emqx/acl` 做主题级授权（未命中一律 `deny`）。
>
> 相关：物模型 → [物模型TSL](/guide/tsl) · 下行 → [下行控制与设备影子](/guide/downlink-shadow) · 动态令牌 → [认证与安全](/guide/auth-security)

## 一、连接参数

| 项 | 值 | 说明 |
|---|---|---|
| Broker | `tcp://<平台地址>:1883` | 生产环境为公网地址；本地开发 `127.0.0.1:1883` |
| MQTT over WebSocket | `ws://<平台地址>:8083` | 浏览器/弱网设备 |
| ClientID | `{productId}.{deviceName}` | **必须**，产品ID 固定 12 字符，按字符数解析（设备名可含 `.`） |
| Username | `{deviceName}` | 仅设备名，不含产品前缀 |
| Password | `{deviceSecret}` | 一机一密下的设备密钥；或 `tk:{token}` 动态令牌 |
| Clean Session | 建议 `false` | 持久会话，重连后补发离线消息 |
| Keep Alive | 建议 ≤ 60s | 配合遗嘱实现秒级离线 |

> 产品ID 固定 **12 字符**（2 位大写字母 + 10 位数字，如 `AB1234567890`），平台取 ClientID 前 12 字符作为产品ID、其余部分作为设备名，**设备名可以包含点号**。

### 鉴权流程

```
设备 ──CONNECT(clientid, username, password)──► EMQX
        EMQX ──POST /api/v1/emqx/auth──► 平台后端
        （clientid/username/password 三字段）
        平台 === 校验通过 ===► {"result":"allow","username":"{pk}/{dn}"}
        平台 === 校验失败 ===► {"result":"deny"}   → EMQX 拒绝连接
```

- 密码以 `tk:` 开头 → 走动态令牌校验（`ValidateDeviceToken`，令牌绑定签发时的 clientid 与设备密钥哈希），否则按固定密钥校验（`FindDeviceForAuth`，兼容一机一密 / 一型一密）
- 设备被禁用 → `{"result":"deny","comment":"device disabled"}`；动态令牌在设备密钥轮转后也立即失效
- 鉴权成功时平台会把 `username` 重写为 `{productId}/{deviceName}`（斜杠分隔）

## 二、Topic 全表

| 方向 | Topic | QoS | Retained | 语义 |
|---|---|---|---|---|
| 设备→平台 | `thing/up/{pk}/{dn}` | 1 | 否 | 遥测属性上报 / 事件上报 |
| 设备→平台 | `thing/up/{pk}/{dn}/ota` | 1 | 否 | OTA 进度上报 |
| 设备→平台 | `thing/up/{pk}/{dn}/reply` | 1 | 否 | 指令应答 `{messageId, code, data}` |
| 平台→设备 | `thing/down/{pk}/{dn}` | 1 | 否 | 下行指令 / 通知 |
| 平台→设备 | `thing/down/{pk}/{dn}/retained` | 1 | **是** | 影子期望值 / 配置（上线即取） |
| 平台→设备 | `thing/broadcast/{pk}` | 1 | 否 | 产品级广播 |
| 设备→平台 | `thing/offline/{pk}/{dn}` | 1 | — | **LWT 遗嘱**（连接时设置，断线自动发布） |
| 网关设备 | `thing/gateway/{pk}/{dn}/sub/{subId}/login\|logout` | 1 | 否 | 子设备登录 / 登出 |

> 平台内部同时订阅 `thing/up/#`、`thing/offline/+`、`thing/gateway/+/+/sub/+/+`、`thing/up/+/+/reply` 及 `$SYS/brokers/+/clients/+/connected`、`$SYS/brokers/+/clients/+/disconnected`（上下线事件）。

## 三、设备 Topic 权限（ACL）

设备只能操作自己的主题，**任何通配符（`+`/`#`）一律禁止**（仅网关子设备订阅例外）：

| Action | 允许的 Topic | 说明 |
|---|---|---|
| publish | `thing/up/{pk}/{dn}` 及子级（如 `/ota`） | |
| publish | `thing/offline/{pk}/{dn}` | 自己的遗嘱 |
| publish | `thing/gateway/{pk}/{dn}/sub/{subId}/login` / `logout` | 子设备管理 |
| subscribe | `thing/down/{pk}/{dn}` 及子级（含 `/retained`） | |
| subscribe | `thing/broadcast/{pk}` | 产品广播 |
| subscribe | `thing/gateway/{pk}/{dn}/sub/+` | **唯一允许 `+` 的场景** |

## 四、上行报文格式（设备→平台）

### 4.1 遥测属性上报

发布到 `thing/up/{pk}/{dn}`，payload 为**扁平 JSON 对象**，顶层键即物模型属性 `identifier`：

```json
{
  "temperature": 25.5,
  "humidity": 60.2,
  "switch": 1
}
```

- 无需携带 `ts` 字段，平台统一打时间戳
- 属性按产品物模型逐字段校验（类型/取值范围）；校验失败的报文仍写入历史（供回溯），但**不进入最新值/影子/规则**，避免污染实时数据与触发误告警
- 系统方法键（`method`/`messageId`/`code`/`params`/`id` 等）无条件剔除，不进入最新值/影子；产品有物模型时仅保留已定义属性
- 产品无物模型时，属性全部进入最新值/影子（系统方法键仍剔除）

### 4.2 系统方法（method 分流）

payload 中带字符串 `method` 字段时，平台按方法分流：

| 方法 | 上行报文 | 平台行为 |
|---|---|---|
| `event.post` | `{"method":"event.post","identifier":"highTemp","type":"alert","params":{...}}` | 事件入库并实时推送；`type` 仅 `alert`/`fault` 保留，否则归为 `info` |
| `ntp.request` | `{"method":"ntp.request","deviceSendTime":<ms>}` | 下行 `ntp.response` 对时 |
| `config.get` | `{"method":"config.get"}` | 下行产品级远程配置 `config.push` |
| `ota.progress` | `{"method":"ota.progress","taskId":1,"progress":50,"status":"upgrading"}` | 经 `thing/up/{pk}/{dn}/ota` 上报，见 [OTA升级](/guide/ota) |

> 指令应答（非 `method` 分流）：设备在 `thing/up/{pk}/{dn}/reply` 发布 `{"messageId":"...","code":0,"data":...}`，平台匹配 pending 的指令置为 `acked`。

### 4.3 事件上报示例

```json
{
  "method": "event.post",
  "identifier": "highTemp",
  "type": "alert",
  "params": { "temperature": 35.2 }
}
```

## 五、下行报文格式（平台→设备，`thing/down/{pk}/{dn}`）

| 场景 | 报文 | 说明 |
|---|---|---|
| 属性设置 | `{"method":"property.set","params":{...},"delta":true,"ts":<ms>}` | 仅下发影子 delta 差异项 |
| 影子期望值（Retained） | `{"method":"property.set","params":{...},"retained":true,"ts":"<ms>"}` | 发布到 `…/retained`，上线即取 |
| 服务调用 | `{"method":"service.invoke","messageId":"<纳秒>","service":"reset","params":{...},"ts":<ms>}` | |
| 透传命令 | 请求体原始 JSON 原样下发，无包裹 | 由管理端/OpenAPI 下发 |
| 远程配置 | `{"method":"config.push","version":N,"params":{...}}` | 走广播 `thing/broadcast/{pk}` |
| NTP 对时 | `{"method":"ntp.response","deviceSendTime":<ms>,"serverRecvTime":<ms>,"serverSendTime":<ms>}` | |
| OTA 升级 | `{"method":"ota.push","version":"...","url":"...","size":N,"sha256":"...","taskId":N,"ts":<ms>}` | 见 [OTA升级](/guide/ota) |

> 所有下行均为 QoS 1、非 retained（`…/retained` 主题除外）。设备收到 `property.set` 后应**立即上报新值**，平台据此清除影子 desired。

## 六、动态令牌（tk:）

设备可先用三元组换取短期令牌（默认 **1 小时**），再用令牌作为 MQTT password 连接，避免长期 Secret 暴露：

```http
POST /api/v1/auth/token
Content-Type: application/json

{ "productId": "pk...", "deviceName": "dev1", "secret": "..." }
```

```json
{ "code": 0, "data": { "token": "tk:1f2e...", "ttl": 3600 } }
```

- 令牌格式：`tk:` + 32 位十六进制（16 随机字节），存于 Redis（`device:token:{token}`），过期自动失效
- 令牌绑定签发时的设备密钥哈希：**轮转设备 Secret 后旧令牌立即失效**；设备被禁用令牌也立即失效
- 令牌也可通过管理端撤销（`RevokeDeviceToken`）

## 七、最小接入示例（Node.js）

```js
const mqtt = require('mqtt')

const productId = 'pk...'
const deviceName = 'dev1'
const secret = '...'

const client = mqtt.connect('mqtt://<平台地址>:1883', {
  clientId: `${productId}.${deviceName}`,
  username: deviceName,
  password: secret,
  reconnectPeriod: 3000,
})

client.on('connect', () => {
  // 订阅下行
  client.subscribe(`thing/down/${productId}/${deviceName}`, { qos: 1 })
  // 上报遥测
  client.publish(`thing/up/${productId}/${deviceName}`, JSON.stringify({
    temperature: 25.5, humidity: 60.2,
  }), { qos: 1 })
})

client.on('message', (topic, payload) => {
  const msg = JSON.parse(payload.toString())
  if (msg.method === 'property.set') {
    // 应用属性并回报新值
    client.publish(`thing/up/${productId}/${deviceName}`, JSON.stringify(msg.params), { qos: 1 })
  }
})
```

> 完整示例见 [示例代码](/guide/examples) 与内置模拟器 `tools/simulator/simulator.js`。