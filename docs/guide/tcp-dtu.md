# TCP 透传（DTU）接入协议

> 平台在 **:9100** 提供 TCP 透传网关，适用于串口设备经 DTU 以 TCP 长连接接入：连接后先发**注册包**鉴权，再按产品组帧配置收发数据帧。支持三种数据模式：标准物模型 JSON、透传脚本（JS `decode/encode`）、Modbus 云端轮询。
>
> 相关：组帧/脚本 → [物模型TSL](/guide/tsl) · 下行 → [下行控制与设备影子](/guide/downlink-shadow)

## 一、连接流程

```
设备 ──TCP连接 :9100──► 平台
设备 ──注册包(10s内)────► {productId},{deviceName},{secret}\n   或   注册码\n
平台 ──OK\n──► 设备（鉴权成功，进入数据阶段）
平台 ──ERR\n──► 设备（鉴权失败，随即断开）
设备 ──数据帧──► 平台（按组帧配置切分，心跳 `PING`/`PONG`）
平台 ──数据帧──► 设备（下行：透传 JSON 或 encode 后的二进制）
```

## 二、注册包格式

连接建立后** 10 秒内**必须发送一行（以 `\n` 结尾）注册包，两种格式任选其一：

| 格式 | 示例 | 说明 |
|---|---|---|
| **三元组** | `pk123,dev1,secret123\n` | 逗号分隔恰好 3 段，平台校验三元组（兼容一机一密/一型一密） |
| **自定义注册码** | `865207041234567\n` | 整行作为注册码匹配设备（IMEI/ICCID 等，设备管理时可设置） |

**响应：**

| 响应 | 含义 |
|---|---|
| `OK\n` | 鉴权成功，进入数据阶段 |
| `ERR\n` | 鉴权失败 / 设备不存在 / 设备禁用 / 超时，随后断开连接 |

> 同一设备重复连接会**踢掉旧连接**。注册成功后设备置为 `online`。

## 三、组帧模式

TCP 是字节流，需要按产品配置切分数据帧（解决粘包/拆包）。产品字段：

| 字段 | 取值 | 说明 |
|---|---|---|
| `frameMode` | `none` / `delimiter` / `length` | 组帧方式（Modbus 产品固定 `modbus`） |
| `frameDelimiter` | 如 `0D0A` | 定界符（HEX，支持 `0x` 前缀） |
| `frameLenOffset` | 0 | 长度字段字节偏移 |
| `frameLenSize` | 1 / 2 | 长度字段字节数（2 为**大端**） |
| `frameLenAdjust` | 0 | 帧总长 = 长度值 + 调整值 |

| 模式 | 切分规则 |
|---|---|
| `none` | 读到多少算一帧（无组帧，整段作为一帧） |
| `delimiter` | 按定界符切分帧 |
| `length` | 按帧内长度字段切分：`帧总长 = 长度值 + adjust` |
| `modbus` | 按 Modbus RTU 响应帧头推断长度 + CRC 校验 |

## 四、心跳

| 项 | 默认值 | 说明 |
|---|---|---|
| 心跳帧 | `PING` | 产品可自定义（`heartbeatPacket`，支持 `0x` HEX 或文本） |
| 心跳应答 | `PONG\n` | 产品可自定义（`heartbeatReply`）；自定义未配回复则不回 |
| 空闲超时 | **300 秒** | 超时无数据断开并置为离线 |

## 五、上行数据（设备→平台）

每个完整数据帧经 `TrimRight("\r\n")` 后，非心跳帧按以下顺序处理：

1. **有解析脚本** → `codec.decode(bytes)` 解码为属性对象 → JSON 化后进入遥测管线
2. **无脚本** → 直接尝试 JSON 解析，合法则进入遥测管线
3. 非 JSON 且无脚本 → 丢弃并告警

标准物模型 JSON 与 MQTT 完全一致（顶层键=属性 identifier，支持 `method` 分流）：

```json
{ "temperature": 25.5, "humidity": 60.2 }
```

### 透传脚本契约（JS）

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

- 单脚本执行超时 **200ms**，超时即判失败
- 脚本编译结果会缓存，热执行不重复编译；保存脚本时平台会先做编译校验
- 在线测试：`POST /api/v1/products/:id/codec/test`（hex 报文）

## 六、下行数据（平台→设备）

- **透传产品**：平台直接写回 TCP 连接。有 `encode` 脚本时把 JSON 参数编码为二进制，否则原样透传 JSON
- **Modbus 产品**：平台按点位表主动写寄存器/线圈（详见下方 Modbus 规格）
- 下行**不等待设备回执**（非应答式），写超时 5 秒

## 七、Modbus 规格

### 帧格式

```
从机地址(1) + 功能码(1) + 数据(N) + CRC16(2, 低字节在前)
```

- CRC16：多项式 `0xA001`，初值 `0xFFFF`
- 从机地址范围 1–247（默认 1）

### 功能码

| 功能码 | 含义 | | 功能码 | 含义 |
|---|---|---|---|---|
| 0x01 | 读线圈 | | 0x05 | 写单线圈 |
| 0x02 | 读离散量 | | 0x06 | 写单寄存器 |
| 0x03 | 读保持寄存器 | | 0x0F | 写多线圈 |
| 0x04 | 读输入寄存器 | | 0x10 | 写多寄存器 |

### 点位表（单产品上限 100 点位）

`identifier / name / groupId / slaveId / functionCode / address / rawType / bitPosition / scale / offset / swapByte / swapWord / accessMode / unit`

- **rawType**：`int16 / uint16(默认) / int32 / uint32 / float`（占 2 寄存器，大端）、`bool / bits`
- **取值换算**：`raw * scale + offset`
- **采集组**（分频）：同组点位共享 `pollInterval` 与 `reportMode`（`periodic` 全量 / `onchange` 仅变更）；未分组点位归默认组（id=0，用产品 `pollInterval`）
- **轮询引擎**：合并连续寄存器批量读（`maxGap=8`、读寄存器上限 120）、单请求超时 3s、并发上限 50；多实例部署时平台保证同一设备组仅一个实例采集

## 八、限流与容错

| 项 | 默认值 | 说明 |
|---|---|---|
| 单 IP 最大连接数 | 10 | 超限直接断开 |
| 连接速率 | 5 个/秒（burst 10） | 令牌桶，超限断开 |
| 空闲超时 | 300 秒 | 断开并置离线 |
| 写超时 | 5 秒 | |

## 九、最小接入示例（DTU 模拟器）

```js
const net = require('net')

const [productId, deviceName, secret] = [process.argv[2], process.argv[3], process.argv[4]]
const sock = net.connect({ host: '127.0.0.1', port: 9100 })

// 1. 注册包
sock.write(`${productId},${deviceName},${secret}\n`)

// 2. 等待 OK
sock.on('data', (chunk) => {
  const text = chunk.toString()
  if (text.includes('OK')) {
    console.log('注册成功')
    // 3. 上报 4 字节二进制帧（温度x10 + 湿度x10）
    const frame = Buffer.alloc(4)
    frame.writeInt16BE(250, 0)  // 25.0°C
    frame.writeInt16BE(550, 2)  // 55.0%
    sock.write(frame)
  }
})
```

> 完整示例见 `tools/simulator/dtu.js`（透传）与 `tools/simulator/dtu-modbus.js`（Modbus 从机）。