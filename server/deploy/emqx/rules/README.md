# EMQX 规则引擎配置 — 遥测数据直写 TimescaleDB（快路径）

## 原理

```
设备 → EMQX → ┌─ 快路径：规则引擎直写 TimescaleDB（批量高吞吐）
               └─ 慢路径：转发到 Go 后端（规则评估/影子/WebSocket推送/轨迹日志）
```

## 启用步骤

### 1. 创建 PostgreSQL 数据桥接

在 EMQX Dashboard（http://localhost:18083，admin/public）→ Integration → Connectors 中创建：

- 类型: PostgreSQL
- 名称: `tsdb_telemetry`
- 服务器: `127.0.0.1:5432`
- 数据库: `iot`
- 用户名: `iot`
- 密码: `iot123456`
- 连接池: 8

或通过 REST API:
```bash
curl -X POST http://localhost:18083/api/v5/bridges \
  -u admin:public \
  -H "Content-Type: application/json" \
  -d '{
    "type": "postgresql",
    "name": "tsdb_telemetry",
    "enable": true,
    "server": "127.0.0.1:5432",
    "database": "iot",
    "username": "iot",
    "password": "iot123456",
    "pool_size": 8
  }'
```

### 2. 创建规则 — 遥测入库

在 Dashboard → Integration → Rules 中创建：

**SQL:**
```sql
SELECT
  payload.data as data,
  payload.device_id as device_id,
  payload.product_key as product_key,
  payload.device_name as device_name,
  timestamp as ts
FROM "thing/up/#"
WHERE payload.method IS NULL OR payload.method = ''
```

> 注意：只处理纯遥测消息，过滤掉 `event.post`/`ntp.request`/`config.get` 等系统方法

**Action:** PostgreSQL Bridge → `tsdb_telemetry`

**SQL Template:**
```sql
INSERT INTO telemetries (ts, device_id, product_key, device_name, data, valid)
VALUES (NOW(), ${device_id}, '${product_key}', '${device_name}', '${data}'::jsonb, true)
```

### 3. 后端配置开关

在 `server/configs/config.yaml` 中启用：
```yaml
emqx_rule:
  enabled: true
```

启用后 Go 后端跳过遥测 DB 写入，只负责：
- Redis 最新值缓存
- WebSocket 实时推送
- 设备影子同步
- 规则引擎评估（告警/转发）
- 消息轨迹/设备日志
