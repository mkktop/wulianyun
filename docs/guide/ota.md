# OTA 升级协议

> OTA 升级采用「管理端上传固件 → 创建任务 → MQTT/TCP 下行推送通知 → 设备 HTTP 下载 → 设备 MQTT 上报进度」四段式流程。
>
> 相关：下行通道 → [下行控制与设备影子](/guide/downlink-shadow)

## 一、流程总览

```
 管理员 ──POST /api/v1/firmwares──► 上传固件（multipart，平台计算 SHA-256）
 管理员 ──POST /api/v1/ota-tasks──► 创建升级任务（选固件 + 设备列表）
 平台   ──thing/down/{pk}/{dn}──► 设备  {"method":"ota.push", version, url, size, sha256, taskId}（QoS 1）
 设备   ──HTTP GET url──────────► 本地模式：平台 /uploads/firmware/...；s3 模式：对象存储直连（公开读桶）
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

上传时平台自动计算 **SHA-256** 校验和。响应返回固件记录：

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
    "crc32": 3735928559,
    "createdAt": "..."
  }
}
```

| 响应字段 | 说明 |
|---|---|
| `fileUrl` | 固件下载路径（`/uploads/firmware/...`） |
| `fileSize` | 文件字节数 |
| `checksum` | SHA-256 校验和（十六进制） |
| `crc32` | CRC32/IEEE 校验值（无符号 32 位整数，0~4294967295）。旧固件为 0 表示无 CRC32 |

### 文件 URL 格式

固件存储由 `storage` 配置决定（`local` 本地磁盘 / `s3` 对象存储），URL 形态随模式不同：

**local 模式**（默认，单机部署）

```
/uploads/firmware/{产品数字ID}_{版本}_{unix时间戳}_{安全化文件名}
```

例：`/uploads/firmware/1_1.0.3_1720000000_app.bin`，设备需拼接平台地址（`http://<平台地址> + url`）下载，为强制附件（`Content-Type: application/octet-stream` + `Content-Disposition: attachment`），任何扩展名都不会被浏览器渲染。

**s3 模式**（S3 兼容对象存储：阿里 OSS / 腾讯 COS / MinIO）

```
http://{bucket}.{endpoint}/firmware/{产品数字ID}_{版本}_{unix时间戳}_{8位随机hex}_{安全化文件名}
```

例：`http://wulian-ota.oss-cn-hangzhou.aliyuncs.com/firmware/1_1.0.3_1720000000_a3f9x7k2_app.bin`

- 桶需设为**公开读**；对象名含随机串不可猜测，防越权下载
- URL **短而永久有效**（无签名、无过期），适配 4G 模组/嵌入式 HTTP 栈；设备直连对象存储（可前置 CDN），平台不承载下载流量
- 多实例部署天然共享；`public_domain` 可配置 CDN/自定义域名

两种模式共同点：

- 原文件名只保留字母数字 `.` `_` `-`，其余替换为 `_`
- 上传扩展名白名单：`.bin .hex .img .dat .zip .tar .gz .pack .rbl`；单文件上限 512 MB
- 上传时平台自动计算 SHA-256 + CRC32/IEEE，随 `ota.push` 下发给设备校验

## 三、创建升级任务

```
POST /api/v1/ota-tasks
Content-Type: application/json

{ "firmwareId": 1, "deviceIds": [3, 5, 7] }
```

- 任务按**设备数字 ID 列表**指定目标设备（非按产品）
- 目标设备必须与固件属于**同一产品**，不属于该产品的设备 ID 会被剔除
- 任务状态初始为 `running`
- 创建后立即向每个目标设备下发 `ota.push` 通知，**按设备接入协议选路**：TCP/DTU 设备走网关（在线直发，多实例走 Redis 扇出），MQTT 设备走 EMQX（QoS 1）。设备离线时该台计入未送达，不再回退到其它协议

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
  "crc32": 3735928559,
  "taskId": 1,
  "ts": 1785712345678
}
```

| 字段 | 说明 |
|---|---|
| `crc32` | CRC32/IEEE 校验值，无符号 32 位整数（0~4294967295）。**值为 0 表示该固件无 CRC32**（旧固件），请仅校验 `sha256`；非 0 时建议先做 CRC32 快速校验，再做 SHA-256 完整性校验 |

### 4.2 下载固件

- **local 模式**：设备直接 HTTP GET `url`（完整地址为 `http://<平台地址> + url`）
- **s3 模式**：`url` 即为对象存储的完整公开地址（或 CDN 域名），设备直接 GET 即可

下载后校验完整性：**`crc32` 非 0 时先做 CRC32 快速校验**（低算力设备友好），再校验 **SHA-256**（与 `ota.push` 中对应字段比对）。`crc32` 为 0 的旧固件仅校验 `sha256`。

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

## 六、当前能力边界

接入开发时请注意以下能力约束：

- **设备被动接收**：设备只能等待 `ota.push` 通知（非 retained，无重发），错过推送则不会自动补送；当前无设备主动查询版本的 `ota.get`/`ota.query` 方法
- **无设备级确认**：任务仅统计推送成功数，不跟踪单台设备是否收到/下载完成；设备成功与否依赖其主动上报 `ota.progress`
- **进度为整任务粒度**：多设备任务中，单台设备的进度上报会覆盖整体 `progress`，无法区分各设备结果
- **无版本比较**：平台不校验设备当前版本与目标版本，不拒绝重复版本号
- **无取消/删除任务**：任务创建后不可取消或删除