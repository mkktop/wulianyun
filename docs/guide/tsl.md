# 物模型（TSL）协议

> 物模型（Thing Model）描述产品的能力，是设备上报数据与平台校验、存储的契约。每个产品一份，按 `properties / events / services` 三类 JSON 数组整体保存，支持导入导出。

## 一、顶层结构

```json
{
  "properties": [],
  "events": [],
  "services": []
}
```

### properties（属性）

表示设备的状态 / 遥测数据，设备上报的 JSON 顶层键即属性 `identifier`。

```json
{
  "identifier": "temperature",
  "name": "温度",
  "dataType": "float",
  "unit": "℃",
  "min": -40,
  "max": 80,
  "step": 0.1,
  "accessMode": "r",
  "enumSpec": [],
  "desc": ""
}
```

| 字段 | 类型 | 说明 |
|---|---|---|
| `identifier` | string | 属性标识符（产品内唯一，禁保留字） |
| `name` | string | 显示名称（必填） |
| `dataType` | string | 数据类型，见下表 |
| `unit` | string | 单位 |
| `min` / `max` / `step` | number | 取值范围 / 步长（float 型） |
| `accessMode` | string | `r`（只读）/ `rw`（可写，可下发） |
| `enumSpec` | array | 枚举项 `[{value, label}]`，`dataType=enum` 时必填 |
| `desc` | string | 描述 |

### events（事件）

```json
{
  "identifier": "highTemp",
  "name": "高温告警",
  "type": "alert",
  "outputs": [{ "identifier": "temperature", "name": "温度", "dataType": "float" }],
  "desc": ""
}
```

`type` 取值：`info`（信息）/ `alert`（告警）/ `fault`（故障）。

### services（服务）

```json
{
  "identifier": "reset",
  "name": "复位",
  "async": false,
  "inputs": [{ "identifier": "code", "name": "复位码", "dataType": "int32" }],
  "outputs": [],
  "desc": ""
}
```

## 二、数据类型

| 类型 | 说明 | 校验规则 |
|---|---|---|
| `int32` | 32 位整数 | 数值 + min/max 范围 |
| `float` / `double` | 浮点数 | 数值 + min/max 范围 |
| `bool` | 布尔 | `true/false` 或 `0/1` |
| `enum` | 枚举 | 值必须在 `enumSpec` 中 |
| `text` | 字符串 | 必须为 string |
| `date` | 时间戳 | 数值（毫秒时间戳） |

> 无 `struct` / `string` 类型，`text` 即字符串。

## 三、保留字

标识符禁用：`set / get / post / property / event / time / value`。

## 四、保存校验规则

- `identifier` 与 `name` 必填、产品内唯一、不含保留字
- `dataType` 必须合法
- `min` 不能大于 `max`
- `enum` 类型必须定义 `enumSpec`
- 事件 `type` 仅 `info / alert / fault`

## 五、设备上报与校验

设备上报 JSON（MQTT / TCP / HTTP 同构）以顶层键为属性 `identifier` 逐字段校验：

```json
{
  "temperature": 25.5,
  "humidity": 60.2,
  "switch": 1
}
```

校验规则（`ValidateTelemetry`）：

- 已定义属性 → 类型 / 范围校验，失败则整条报文标记 `valid=false` 并记录 `validationErrors`
- 未定义字段 → 放行（warning 级）
- 产品无物模型 / 无属性定义 → 全部放行

> ✅ 校验器按 `dataType` 键读取定义（曾误读为 `type` 导致校验旁路，已修复并补单测）。

## 六、物模型 API

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/v1/products/:id/thing-model` | 获取物模型 |
| PUT | `/api/v1/products/:id/thing-model` | 保存物模型（整体覆盖） |
| GET | `/api/v1/products/:id/tsl/export` | 导出（含 productKey/productName） |
| POST | `/api/v1/products/:id/tsl/import` | 导入 |

## 七、完整示例

```json
{
  "properties": [
    { "identifier": "temperature", "name": "温度", "dataType": "float", "unit": "℃", "min": -40, "max": 80, "step": 0.1, "accessMode": "r", "enumSpec": [], "desc": "" },
    { "identifier": "switch", "name": "开关", "dataType": "bool", "unit": "", "min": null, "max": null, "step": null, "accessMode": "rw", "enumSpec": [], "desc": "" },
    { "identifier": "mode", "name": "模式", "dataType": "enum", "unit": "", "min": null, "max": null, "step": null, "accessMode": "rw", "enumSpec": [{ "value": 0, "label": "normal" }, { "value": 1, "label": "eco" }], "desc": "" }
  ],
  "events": [
    { "identifier": "highTemp", "name": "高温告警", "type": "alert", "outputs": [{ "identifier": "temperature", "name": "温度", "dataType": "float" }], "desc": "" }
  ],
  "services": [
    { "identifier": "reset", "name": "复位", "async": false, "inputs": [{ "identifier": "code", "name": "复位码", "dataType": "int32" }], "outputs": [], "desc": "" }
  ]
}
```

## 八、接入数据模式

| accessMode | 含义 | 协议约束 | 解析方式 |
|---|---|---|---|
| `thingmodel` | 标准物模型（默认） | mqtt / tcp / http | 平台直接按 TSL 校验 |
| `passthrough` | 透传解析 | 通常 tcp | 产品级 goja JS 脚本 `decode/encode`（见 [TCP-DTU接入协议](/guide/tcp-dtu)） |
| `modbus` | Modbus 云端轮询 | **强制 tcp** | 平台按点位表主动轮询寄存器 |

## 九、密钥模式

| secretMode | 含义 | 动态注册行为 |
|---|---|---|
| `device` | 一机一密（默认） | 设备须预先创建，校验设备级 Secret |
| `product` | 一型一密 | 创建产品时生成 `ProductSecret`；设备不存在且 `secret == ProductSecret` 时**自动建设备**并签发独立 Secret |

> 详见 [认证与安全](/guide/auth-security)。