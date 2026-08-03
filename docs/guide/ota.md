# OTA 升级协议

> OTA 升级采用「管理端上传固件 → 创建任务 → MQTT/TCP 下行推送通知 → 设备 HTTP 下载 → 设备 MQTT 上报进度」四段式流程。
>
> 相关：下行通道 → [下行控制与设备影子](/guide/downlink-shadow)

## 一、流程总览

```
 管理员 ──POST /api/v1/firmwares──► 上传固件（multipart，自动计算 SHA-256）
 管理员 ──POST /api/v1/ota-tasks──► 创建升级任务（选固件 + 设备列表）
 平台   ──thing/down/{pk}/{dn}──► 设备  {"method":"ota.push", version, url, size, sha256, taskId}
 设备   ──HTTP GET url──────────► 平台  /uploads/firmware/...（公开下载，无鉴权）
 设备   ──thing/up/{pk}/{dn}/ota─► 平台  {"method":"ota.progress", taskId, progress, status}
 平台   任务状态：running → completed / failed
```

## 二、固件上传

```
POST /api/v1/firmwares
Content-Type: multipart/form-data
```

| 字段 | 必填 | 说明 |
|---|---|---|
| `productId` | ✔ | 产品 ID |
| `version` | ✔ | 版本号（字符串，仅校验非空，≤32 字符） |
| `description` | ✘ | 描述 |
| `file` | ✔ | 固件文件（multipart 文件字段） |

上传时自动计算 **SHA-256** 校验和。响应返回 `model.Firmware`：

```json
{
  "code": 0,
  "data": {
    "id": 1,
    "productId": 1,
    "version": "1.0.3",
    "fileUrl": "/uploads/firmware/1_1.0.3_1720000000_app.bin",
    "fileSize": 102400,
    "checksum": "a1b2c3...",
    "createdAt": "..."
  }
}
```

### 文件 URL 格式

```
/uploads/firmware/{productID}_{version}_{unixTimestamp}_{原文件名(安全化)}
```

例：`/uploads/firmware/1_1.0.3_1720000000_app.bin`

- 原文件名只保留字母数字 `.` `_` `-`，其余替换为 `_`
- 下载为**公开静态路由**（`/uploads`），无鉴权，设备可用 `sha256` 校验完整性

## 三、创建升级任务

```
POST /api/v1/ota-tasks
Content-Type: application/json

{ "firmwareId": 1, "deviceIds": [3, 5, 7] }
```

- 任务按**设备 ID 列表**指定目标设备（非按产品）
- 任务状态初始为 `running`
- 创建后立即向每个目标设备下发 `ota.push` 通知（优先 TCP 通道，回退 MQTT）

响应：

```json
{ "code": 0, "data": { "task": { "id": 1, "status": "running" }, "pushedCount": 3, "totalDevices": 3 } }
```

## 四、设备侧协议

### 4.1 接收升级通知（下行）

```json
{
  "method": "ota.push",
  "version": "1.0.3",
  "url": "/uploads/firmware/1_1.0.3_1720000000_app.bin",
  "size": 102400,
  "sha256": "a1b2c3...",
  "taskId": 1,
  "ts": 1785712345678
}
```

### 4.2 下载固件

设备直接 HTTP GET `url`（完整地址为 `http://<平台地址> + url`），下载后校验 SHA-256。

### 4.3 上报进度（上行）

发布到 `thing/up/{pk}/{dn}/ota`：

```json
{ "method": "ota.progress", "taskId": 1, "progress": 50, "status": "upgrading" }
```

| 字段 | 说明 |
|---|---|
| `taskId` | 任务 ID（下行通知中的 taskId） |
| `progress` | 0–100 整数 |
| `status` | `upgrading`（进行中）/ `success`（成功）/ `failed`（失败） |

平台处理：

- `status == "success"` → 任务置 `completed`、progress=100
- `status == "failed"` → 任务置 `failed`
- 否则仅更新 progress

> ⚠️ `status` 判等只认字面量 `"success"` / `"failed"`，其它写法（如 `"succeeded"`）无效。

## 五、任务状态

| 状态 | 含义 |
|---|---|
| `running` | 升级中 |
| `completed` | 全部完成（收到 success 上报） |
| `failed` | 失败（收到 failed 上报） |

查询：`GET /api/v1/ota-tasks`。

## 六、当前限制（已知问题）

- **无设备主动拉取**：设备只能被动等 `ota.push` 通知（非 retained，无重发机制），错过推送即漏升级；无 `ota.get` / `ota.query` 类方法
- **无设备确认（ack）**：任务只统计推送成功数，不跟踪设备是否收到/下载成功
- **无按设备粒度结果**：多设备任务中单个设备进度会覆盖整体进度，无法区分各设备结果
- **无版本比较**：不校验设备当前版本 vs 目标版本，不拒绝重复版本号
- **无删除/取消任务接口**

> 以上为当前实现边界，均在演进计划中。