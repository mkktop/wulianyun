# 协议总览

> 本文档面向**第三方开发者**，详细说明如何把设备接入 KK 物联云，包括设备接入所需的 Topic、报文格式、鉴权方式与错误码。

---

## 一、接入方式总览

平台提供三种设备接入方式，按设备的硬件能力与场景选择：

| 接入方式 | 传输层 | 适用场景 | 数据模式 | 文档 |
|---|---|---|---|---|
| **MQTT** | TCP :1883 / WebSocket :8083 | 主流智能硬件，推荐 | 物模型 / 透传 | [MQTT接入协议](/guide/mqtt) |
| **TCP 透传 (DTU)** | Raw TCP :9100 | 串口设备 / DTU 透传 / 私有协议 | 物模型 / 透传脚本 / Modbus | [TCP-DTU接入协议](/guide/tcp-dtu) |
| **HTTP 直传** | HTTPS :80 | 弱网络设备、一次性上报 | 物模型 | [HTTP接入协议](/guide/http) |

> 三种方式汇入同一条数据处理管线：**物模型校验 → 时序入库 → 实时推送 → 规则触发**，因此**上行报文格式完全一致**，仅传输与鉴权方式不同。校验未通过的报文仍写入历史供回溯，但不进入实时数据/影子/规则。

## 二、快速开始（10 分钟接入一台设备）

1. **登录平台** → 产品管理 → 创建产品（记录 `ProductID`，选择接入数据模式）
2. 进入产品 → 设备管理 → 添加设备（记录 `DeviceName` 与 `DeviceSecret`）
3. 得到设备**三元组**：`ProductID / DeviceName / DeviceSecret`
4. 按 [MQTT接入协议](/guide/mqtt) 接入，或直接运行内置模拟器：

```bash
cd tools/simulator
npm install
node simulator.js <productId> <deviceName> <deviceSecret> [broker]
```

5. 平台「设备详情」页实时看到数据、上下线事件，并可下发命令。

## 三、核心概念

| 概念 | 说明 |
|---|---|
| **三板斧（三元组）** | `ProductID`（产品唯一标识）+ `DeviceName`（产品内设备名）+ `DeviceSecret`（设备密钥），设备连接与鉴权的唯一凭证 |
| **接入数据模式** | `thingmodel`（标准物模型 JSON）/ `passthrough`（透传脚本 / 二进制）/ `modbus`（Modbus 云端轮询） |
| **密钥模式** | `device`（一机一密，默认）/ `product`（一型一密，`ProductSecret` 可免预注册动态建号） |
| **物模型 TSL** | 产品的能力描述：`properties`（属性）/ `events`（事件）/ `services`（服务），见 [物模型TSL](/guide/tsl) |
| **设备影子** | `desired`（期望值）/ `reported`（实际值）双副本，离线补发、达成自动清除，见 [下行控制与设备影子](/guide/downlink-shadow) |

> **关于 `productId` 语义**：在设备三元组、MQTT ClientID、TSL 导出文件中，`productId` 指**产品标识字符串**（12 位 `AB1234567890` 格式）；而在管理端/OpenAPI 的请求参数（如 `?productId=`、固件上传字段、OTA 任务）中，`productId` 指**产品在平台内的数字 ID**。各端点文档会明确标注其类型，对接时请注意区分。

## 四、连接参数速查

| 项 | 值 |
|---|---|
| MQTT Broker | `tcp://<平台地址>:1883`（WebSocket：`ws://<平台地址>:8083`） |
| MQTT ClientID | `{productId}.{deviceName}` |
| MQTT Username | `{deviceName}` |
| MQTT Password | `{deviceSecret}` 或 `tk:{token}` 动态令牌 |
| TCP 网关 | `<平台地址>:9100` |
| HTTP 上报 | `POST /api/v1/http/telemetry`，头 `X-Device-Token: Base64(pk:dn:secret)` |
| 开放 API | `/openapi/v1`，HMAC-SHA256 签名 |

## 五、文档索引

| 文档 | 内容 |
|---|---|
| [MQTT接入协议](/guide/mqtt) | 连接/鉴权、Topic 全表、上下行报文格式、ACL 权限、动态令牌 |
| [TCP-DTU接入协议](/guide/tcp-dtu) | 注册包、组帧、心跳、透传脚本、Modbus 规格 |
| [HTTP接入协议](/guide/http) | 直传端点、鉴权、报文、错误码、动态令牌 |
| [物模型TSL](/guide/tsl) | 物模型结构与字段、数据类型、校验规则、导入导出 |
| [下行控制与设备影子](/guide/downlink-shadow) | 透传命令/属性设置/服务调用、影子、指令应答、广播 |
| [OTA升级](/guide/ota) | 固件上传、任务下发、设备下载与进度回报 |
| [网关子设备](/guide/gateway) | 网关登录/登出子设备、子设备下行 |
| [开放平台OpenAPI](/guide/openapi) | 应用创建、HMAC 签名、端点列表 |
| [OpenAPI 字段参考](/guide/api-reference) | OpenAPI 全部端点请求/响应字段与排障 |
| [错误码与响应参考](/guide/errors) | 统一信封、业务码、各模块错误消息 |
| [认证与安全](/guide/auth-security) | JWT / 设备密钥 / 动态令牌 / 响应信封 |
| [示例代码](/guide/examples) | 各协议最小可运行示例 |

## 六、通用约定

### 响应信封

所有管理端 / OpenAPI 接口统一返回：

```json
{ "code": 0, "msg": "ok", "data": { } }
```

- `code === 0` 表示成功；业务失败时 HTTP 状态码仍为 200，以 `code` 区分（如 400/404/500）
- 鉴权失败（未登录 / 签名错误）返回 **HTTP 401** + `{code: 401, msg: "..."}`
- 分页结果统一为 `{ "total": N, "list": [...] }`，位于 `data` 下

### 时间

- 报文内 `ts` 一律为 **Unix 毫秒时间戳**（`time.Now().UnixMilli()`）
- 消息 ID `messageId` 为 **纳秒时间戳**（`UnixNano()`）
- 设备上行报文**无需携带时间戳**，由平台统一打点

### QoS

| QoS | 含义 | 平台使用 |
|---|---|---|
| 0 | 至多一次（可丢弃） | — |
| 1 | 至少一次 | **上下行默认**（设备遥测、平台下行指令/属性设置/OTA 通知均使用） |

### 分页约定

- 设备列表等分页接口：`page`（页码，从 1 起）+ `size`（每页条数，默认 10，**上限 100**）
- 历史遥测查询：`start`/`end`（毫秒时间戳）+ `limit`（条数，默认 2000，**上限 50000**，无分页）
- 响应统一为 `data: { total, list: [...] }`

---

- 如对接过程中遇到问题，请联系平台技术支持获取协助。