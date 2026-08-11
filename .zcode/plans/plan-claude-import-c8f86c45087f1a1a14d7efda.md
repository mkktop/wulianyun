## 目标
给 ESP32-S3 固件（D:\work\mqtt）增加一套 **TCP 透传网关接入客户端**，作为与现有 MQTT 客户端并行的传输方式；然后用物理板子真实接入平台 TCP 网关 `:9100`，**逐项验证 TCP 网关的全部功能**，并给出每一步的具体测试方法与预期。编译时切换传输，覆盖三类数据模式（透传 JSON / 脚本解码 / Modbus）。

## 一、前提与已核实事实

| 项 | 结论（已核实） |
|---|---|
| 现有产品 #7 `MQTT全功能测试产品` | protocol=mqtt，**accessMode/protocol 创建后不可变**（product.go:193），无法改 TCP → **必须新建一个 TCP 产品** |
| TCP 网关 :9100 | 从设备 WiFi 可达 `39.105.129.96:9100`（已实测 OK） |
| 平台 TCP 网关协议 | 注册包 `{pk},{dn},{secret}\n` 或注册码（tcp.go:331）；回复 `OK\n`/`ERR\n`（10s 超时） |
| 组帧 | none/delimiter/length（model.go:65-69）；Modbus 由 accessMode 强制（framer.go:34） |
| 心跳 | 默认 `PING`→`PONG\n`；或产品自定义 heartbeatPacket/heartbeatReply（tcp.go:433） |
| 空闲超时 | 默认 300s（config.yaml），无数据断开+离线 |
| 下行路由 | 由 `gateway.Has(pk,dn)` 决定走 TCP；无脚本透传原始 JSON，有脚本走 `codec.Encode`；Modbus 走 `poller.WriteProperty` |
| 下行来源 | property.set（shadow.go:203）、service.invoke（shadow.go:135）、`POST /devices/:id/command`（data.go:74，**字节原样透传，最干净的测试端点**） |
| 重连踢人 | 同 key 新连接 `old.conn.Close()`（tcp.go:364）；重连触发 OnDeviceConnect→poller 重启 |
| ESP-IDF | v5.4.4，lwIP socket 可用（MAX_SOCKETS=10），FreeRTOS 100Hz |
| 三元组 secret | 设备创建时 `randHex(16)`（device.go:61）生成 |
| 二级账号 mkk_s3 | **不能创建产品**（product.go:64，"二级账号无法创建产品"）→ **新建 TCP 产品用主账号 W17284539mkk**，再回到 mkk_s3 测试不适用。→ 改为：用主账号建产品+设备，三元组写进固件 |

## 二、方案：编译时传输切换（最简、互不干扰）

遵循现有固件「硬编码三元组 + WiFi 就绪后启动」的风格，加一个 `#define`：

### 2.1 新文件 `main/bsp/tcp.h` / `main/bsp/tcp.c`
- `void tcp_start(void);` / `bool tcp_is_connected(void);` —— 与 `mqtt.h` 对称的对外接口
- 顶部同样 `#define` TCP 三元组（新 TCP 产品，独立于 MQTT 产品的 GY7638063548/mkk_S3）
- 内部：
  - **注册鉴权任务**：socket 连 `39.105.129.96:9100` → 发 `{pk},{dn},{secret}\n` → 读 `OK\n`/`ERR\n`；失败指数退避重连（1→2→4→…→60s）
  - **心跳任务**：周期发 `PING`，应答检查；同时维持空闲超时
  - **遥测任务**：周期上行（与 mqtt.c 的 telemetry_task 对应）
  - **接收任务**：读 socket，按当前数据模式处理下行（透传 JSON 打印 / Modbus 应答投递）
  - **重连**：close→重连→重注册，覆盖"踢人/空闲超时/网络抖动"

### 2.2 修改 `main/main.c`
```c
#include "mqtt.h"
#include "tcp.h"
#define APP_TRANSPORT_MQTT 1   /* 1=MQTT客户端  0=TCP透传客户端 */
...
#if APP_TRANSPORT_MQTT
    mqtt_start();
#else
    tcp_start();
#endif
```
`bsp_wifi_web.c` 里的 `mqtt_flag = true`（IP 拿到后）改为通用名 `net_ready_flag`，含义不变（WiFi 已拿到 IP，可启动平台连接）。

### 2.3 修改 `main/CMakeLists.txt`
`SRCS` 追加 `"bsp/tcp.c"`（已含 mqtt.c，两者编译期互不调用，仅 main.c 按 `#define` 选一个 `start`）。

## 三、测试范围（三类数据模式全测，固件按层增量实现）

TCP 客户端的注册/心跳/重连/接收骨架**三类共用**，只"上行帧构造 + 下行处理"不同。先实现骨架（即透传 JSON 全链路），再增量加脚本解码与 Modbus 从机。建 3 个 TCP 产品逐个测：

| 产品 | protocol/accessMode/frameMode/codec | 固件对应 | 测什么 |
|---|---|---|---|
| **P1 透传JSON** | tcp / thingmodel / **delimiter=0D0A** / 无 | 发 `{"temperature":..}\r\n` | 注册/心跳/组帧/JSON上行/下行透传/重连踢人/鉴权失败 |
| **P2 脚本解码** | tcp / passthrough / length / JS脚本 | 发二进制帧 | codec.Decode 上行、codec.Encode 下行、在线 codec 测试 |
| **P3 Modbus** | tcp / **modbus** / —— | 软件RTU从机 | 云端轮询、批量读、scale/offset、写线圈/寄存器、onchange |

## 四、逐项测试方法（每个测试都给出：动作 + 观测点 + 预期 + 失败排查）

### P1 透传 JSON —— 网关主干验证

**准备**：主账号建 P1（protocol=tcp, accessMode=thingmodel, frameMode=delimiter, frameDelimiter=0D0A, heartbeatPacket=PING, heartbeatReply=PONG\n），建设备 D1，取三元组写进 `tcp.c`。固件 `APP_TRANSPORT_MQTT=0` 烧录。

| # | 测试 | 动作 | 观测 | 预期 | 失败排查 |
|---|---|---|---|---|---|
| 1.1 | **注册成功** | 上电→连 WiFi→连 :9100 | `_monitor.log`、前端设备状态 | 收到 `OK\n`，设备置 online | 注册行不带 `\n`?secret 错?产品 accessMode 非 thingmodel? |
| 1.2 | **鉴权失败** | 故意改错 secret 烧录 | 串口日志 | 收到 `ERR\n` 后断开；前端仍 offline | — |
| 1.3 | **三元组3段格式** | 切换为注册码方式（设备设 RegCode）发裸行 | 设备状态 | `OK\n`（注册码匹配） | RegCode 重复/空? |
| 1.4 | **心跳** | 让固件周期发 `PING` | 串口收到 `PONG\n` | 持续在线，不被空闲超时踢 | heartbeatReply 没配? |
| 1.5 | **JSON 上行** | 固件发 `{"temperature":25.6,"humidity":60.2}\r\n` | 前端设备详情-实时数据 | 属性值刷新，历史曲线有点 | delimiter 不匹配→组帧卡住?JSON 非 TSL 合法字段? |
| 1.6 | **下行 property.set** | 前端设属性 switch | 串口接收任务日志 | **收到裸 JSON** `{"method":"property.set","params":{"switch":1},...}` | 设备显示 online?走的是 MQTT 还是 TCP（看 gateway.Has）? |
| 1.7 | **下行 command（最干净）** | `POST /devices/:id/command` body `{"led":1}` | 串口 | **原样收到 `{"led":1}`** | 该端点是验证下行透传的最佳手段 |
| 1.8 | **重连踢人** | 用同三元组再连一个 socket | 第一连接断开 | 旧连 close，新连 OK；前端保持 online（无离线闪） | — |
| 1.9 | **空闲超时** | 固件停发心跳与数据 300s | 串口、前端 | 连接断开，设备 offline | 配置 IdleTimeout 值? |
| 1.10 | **断线重连** | 拔 WiFi 60s 再恢复 | 串口 | 指数退避重连，恢复后重新注册+online | — |

### P2 脚本解码 —— codec 全链路

**准备**：主账号建 P2（protocol=tcp, accessMode=passthrough, frameMode=length, frameLenOffset=0/size=1/adjust=2），`PUT /products/:id/codec` 存脚本：
```js
function decode(b){return{temperature:(b[0]<<8|b[1])/10, humidity:(b[2]<<8|b[3])/10}}
function encode(o){return o.led?[0xA1]:[0xA0]}
```
固件发 5 字节帧 `[len][tempHi][tempLo][humHi][humLo]`（len=3）。

| # | 测试 | 动作 | 预期 |
|---|---|---|---|
| 2.1 | **codec.Decode 上行** | 发二进制帧 | 前端看到 temperature/humidity 已解析为数值 |
| 2.2 | **codec 在线测试（不污染）** | `POST /products/:id/codec/test {script, hex}` | 返回解码对象，DB 不变（codec.go:48） |
| 2.3 | **坏脚本拒绝** | `PUT /codec` 存语法错脚本 | 返回 400，不入库（codec.Validate） |
| 2.4 | **下行 codec.Encode** | `POST /devices/:id/command {"led":1}` | 串口**收到 0xA1**（脚本编码后的二进制） |
| 2.5 | **脚本超时** | 脚本写死循环 | 该帧丢弃 + 平台告警（200ms 超时） |

### P3 Modbus —— 云端轮询 + 写控制

**准备**：主账号建 P3（accessMode=modbus，protocol 自动 tcp），配点位表（参考 dtu-modbus.js：addr0=温度x10 rawType=uint16 scale=0.1, addr1=湿度, addr16=开关 rw）。固件实现软件 RTU 从机（内存寄存器 + CRC16 + 0x03/0x04/0x05/0x06 应答），逻辑直接搬 `tools/simulator/dtu-modbus.js`。

| # | 测试 | 动作 | 预期 |
|---|---|---|---|
| 3.1 | **轮询上报** | 等待 pollInterval | 平台主动发 0x03 读帧，固件应答；前端属性周期刷新 |
| 3.2 | **批量合并读** | 连续寄存器 addr0-2 | 平台**单请求**读 3 个寄存器（maxGap=8 合并） |
| 3.3 | **scale/offset** | 寄存器=250 scale=0.1 | 前端 temperature=25.0 |
| 3.4 | **写线圈 0x05** | 前端设 switch=1 | 平台发 0x05 帧，固件回显，寄存器 addr16 更新 |
| 3.5 | **写寄存器 0x06** | 写 int32 点位 | 平台发 0x10 多寄存器帧 |
| 3.6 | **onchange 抑制** | 配组 reportMode=onchange | 数值不变时不上报、变化时才上报 |
| 3.7 | **CRC 错帧** | 故意回错 CRC | 平台丢弃该帧（framer.CheckCRC） |
| 3.8 | **超时请求迟到** | 模拟慢应答 | 不错配给下个请求（generation 校验 #19） |

## 五、固件增量实现顺序（对应测试阶段）

1. **骨架**（一次写完，P1/P2/P3 共用）：`tcp.c` 的 socket 连接 + 注册鉴权 + 心跳 + 重连 + 接收任务 + 周期任务抽象。可发可收，收到就打印。
2. **透传 JSON 层**（测 P1）：周期发 `{"temperature":...}\r\n`；下行打印。完成 1.x 全测。
3. **脚本二进制层**（测 P2）：帧改为 `[len][...]` 长度组帧上行；下行收 5 字节。完成 2.x。
4. **Modbus 从机层**（测 P3）：移植 dtu-modbus.js 的 CRC16 + 寄存器表 + 0x03/04/05/06 应答到 C；对平台下发的 RTU 帧应答。完成 3.x。

每层实现完即烧录实测，通过再进下一层，避免一次改太多难定位。

## 六、验证产出

- 串口日志（每项测试 `_monitor*.log`）证据
- 前端设备状态/实时数据/历史曲线截图或 API 返回
- 三个 TCP 产品在平台上的配置快照（accessMode/frameMode/codec/点位表）
- 每项测试的 通过/失败 结论 + 失败时的根因

## 七、不做的事

- 不改平台代码（本轮纯固件 + 测试，平台侧只建产品/设备/配脚本点位）
- 不改现有 MQTT 客户端逻辑（`#define` 编译期二选一，mqtt.c 原样保留）
- 不做 NVS 动态切换传输（编译时切换足够测试用，避免动配网页）
- 不接真实 RS485 Modbus 从机（软件 RTU 从机即可完整测云端轮询逻辑；物理总线验证不在本轮）