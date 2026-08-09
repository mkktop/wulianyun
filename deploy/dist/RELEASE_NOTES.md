# KK物联云 私有化离线交付包 v@VERSION@

KK物联云是一套企业级物联网设备管理平台（Go 后端 + Vue3 控制台 + EMQX MQTT + TimescaleDB 时序存储）。
本交付包为**离线自包含安装包**：全部组件镜像打包在内，客户机**无需联网、无需构建**，解压后一键安装。

## 版本变更

- **v1.0.7**：安全与可靠性加固——JWT 中间件每请求回查 DB（账号禁用/降级即时生效）；WebSocket 首帧认证（JWT 不进 URL，防泄露进访问日志）+ 指数退避重连；规则引擎幂等（条件三态求值、firing 状态缓存 + 原子去重、离线告警不重复、上线自动恢复）；遥测批量写入失败重试不丢数据；优雅关闭顺序保证关闭窗口不丢遥测；校验失败的遥测不污染实时数据/影子/规则；系统方法键无条件剔除；HTTP 上报校验设备禁用状态；轨迹/日志批量化写入；WebSocket 心跳防半开连接泄漏；Modbus 超时帧隔离防串设备；影子 retained 持锁发布 + 多实例一致性 + 深拷贝；Kafka 转发规则拒绝（未实现）+ MQTT 桥接锁外连接 + 重连不泄漏；数值对称舍入；点位/采集组变更即时生效；脚本长度上限 + 编译校验 + panic 防护。
- **v1.0.6**：安全加固——MQTT 调试台仅平台超管可用并限制 `$SYS`/通配主题；设备模拟器归属校验；固件上传扩展名白名单 + `/uploads` 强制附件下载；TCP 注册包有界读取防内存 DoS；EMQX 动态 Token 绑定 clientid；OpenAPI 签名覆盖 Method+Path+BodyHash + nginx `/openapi/` 代理；WebSocket 广播并发安全、状态队列非阻塞、跨实例下行路由、Redis 抖动降级、点位保存事务化、32 位寄存器写入、广播编码、Webhook SSRF 防护。功能：产品 ID 固定 12 字符（2 字母+10 数字）、ClientID 按字符数解析（设备名可含点号）；属性设置面板展示各可写属性期望状态、运行日志显示上下行详情、历史曲线单图切换 + 时间范围查询 + CSV 导出；设备状态事件 FIFO 串行 + 时间戳守卫、影子期望值 retained 必达。
- **v1.0.5**：产品名称在同一拥有者名下唯一校验（创建/改名去重）；修复设备属性下发请求体二次读流失败、兼容裸属性对象格式；前端路由切换动画改为纯淡入、列表操作列按钮居中不换行。
- **v1.0.4**：修复安装后 `config.prod.yaml` 权限（server 以非 root 用户运行，改 644 避免 `permission denied`）；备份/恢复改用它自带镜像操作卷，**彻底离线**（不再依赖联网拉取 alpine）。
- **v1.0.3**：交付包全部镜像化（timescaledb 2.17.2 / redis 7 / emqx 5.8 固定版本），新增 `healthz/readyz` 探针；说明文档合并为单一来源。

## 下载与校验

| 文件 | 适用 |
|---|---|
| `kk-iot-@VERSION@-amd64.tar.gz` | x86_64 服务器 |
| `kk-iot-@VERSION@-amd64.tar.gz.sha256` | 校验和 |

```bash
sha256sum -c kk-iot-@VERSION@-amd64.tar.gz.sha256   # 输出 OK 表示完整
```

> arm64 / 信创（飞腾、鲲鹏）版本：在仓库 Actions 页手动触发「构建离线交付包」选择 `arm64`，构建完成后在 Artifact 或 Releases 下载。

## 系统要求

| 项 | 要求 |
|---|---|
| 操作系统 | Linux x86_64 / arm64 |
| 内存 | ≥2GB（1.5GB 可跑但推荐更高） |
| 磁盘 | ≥10GB 可用 |
| 软件 | Docker（含 compose v2 插件）；未装需先联网 `curl -fsSL https://get.docker.com \| sh` |
| 端口 | 80 / 1883 / 9100 / 8083 可用（8080/5432/6379 仅内部） |

**端口表**

| 端口 | 用途 | 公网 |
|---|---|---|
| 80 | 管理后台（nginx） | 是 |
| 1883 | MQTT 设备接入 | 是 |
| 9100 | DTU/TCP 设备接入 | 是 |
| 8083 | MQTT over WebSocket（可选） | 是 |
| 18083 | EMQX 面板 | 仅本机 |
| 8080 / 5432 / 6379 | 后端 / 数据库 / Redis | 仅内部 |

## 安装

```bash
# 上传到目标服务器
scp kk-iot-@VERSION@-amd64.tar.gz root@<服务器IP>:~/

# 解压并安装（全程离线；非 root 自动 sudo 重提）
tar xzf kk-iot-@VERSION@-amd64.tar.gz
cd kk-iot-@VERSION@-amd64
bash install.sh
```

安装过程自动完成：环境预检 → 加载离线镜像 → 生成强密钥 → 渲染配置 → 启动服务 → 就绪探测（最长 90 秒）。

安装成功输出：

```
访问地址 : http://<服务器IP>
管理员   : admin / <随机密码>      ← 请立即保存，登录后修改
设备接入 : 1883(MQTT) / 9100(DTU-TCP)
EMQX 面板: 18083 仅本机（经 SSH 隧道访问）
```

EMQX 面板经 SSH 隧道访问：

```bash
ssh -L 18083:127.0.0.1:18083 root@<服务器IP>
# 浏览器打开 http://127.0.0.1:18083 ，账号 admin / 安装时打印的 EMQX 密码
```

> **重装注意**：`bash install.sh --force` 会重新生成密钥，旧数据将无法连接——重装前务必先备份。

## 首次验证

1. 登录控制台，修改 admin 密码
2. 「产品」新建 MQTT 设备（一机一密），记录 `pk / dn / secret`
3. 用设备模拟器（`tools/simulator`）推遥测，确认设备上线、数据入库、WebSocket 实时刷新
4. 上传固件（OTA，上限 512MB）验证下载
5. 执行 `bash backup.sh` 建立**基线备份**

## 日常运维

| 操作 | 命令 |
|---|---|
| 备份（在线，不中断服务） | `bash backup.sh` |
| 恢复 | `bash restore.sh <备份包>` |
| 升级 | `bash upgrade.sh <新版本包目录>` |
| 诊断包（脱敏日志+配置） | `bash diag.sh` |
| 重启 | `docker compose -p kk-iot -f compose/docker-compose.yml restart` |
| 查看日志 | `docker compose -p kk-iot -f compose/docker-compose.yml logs -f server` |
| 修改配置 | 编辑 `compose/config.prod.yaml` → 上面的 restart 命令 |

## 升级

```bash
# 下载新版本包并解压，在当前安装目录执行：
bash upgrade.sh <新版本包目录>
```

- 只替换 server/web 镜像并切换版本号，**postgres / redis / emqx / uploads 数据卷不动**
- 升级前先 `bash backup.sh`
- 升级失败回滚：`sed -i 's/^VER=.*/VER=<旧版本>/' .env && docker compose -p kk-iot -f compose/docker-compose.yml up -d`
- 数据库迁移向前兼容（GORM AutoMigrate），降级旧版本一般安全；但若新版本含破坏性表结构变更，降级前务必先备份

## 内存调优

compose 内置适配弱机的内存上限（postgres 512m / emqx 384m / server 256m / redis 128m / web 48m，合计约 1.5GB）。
若服务器内存充裕（≥4GB），可放大限制：编辑 `compose/docker-compose.yml` 各服务的 `deploy.resources.limits.memory`，再 `docker compose -p kk-iot -f compose/docker-compose.yml up -d`。

## EMQX 面板密码

面板密码**仅在首次安装时**写入（`emqx-data` 卷为空时生效），之后改 `.env` 的 `EMQX_DASHBOARD_PASSWORD` 不会生效。重置步骤：

```bash
docker compose -p kk-iot -f compose/docker-compose.yml stop emqx
docker volume rm kk-iot_emqx-data        # 丢弃 EMQX 运行状态，不影响设备业务数据
# 编辑 .env 的 EMQX_DASHBOARD_PASSWORD
docker compose -p kk-iot -f compose/docker-compose.yml up -d
```

## 迁移（换服务器）

新机器安装同版本 → `bash restore.sh <旧服务器备份包>` → 数据库、固件上传、配置完整迁移。

## 排障

1. `bash diag.sh` 打包诊断信息（已脱敏），发给技术支持
2. 查后端日志：`docker compose -p kk-iot -f compose/docker-compose.yml logs --tail=200 server`
3. 重启全部：`docker compose -p kk-iot -f compose/docker-compose.yml restart`

## 注意事项

- 交付包离线自包含：客户机全程不访问公网，无外网依赖
- `.env` / `compose/config.prod.yaml` 含密钥（600 权限），请妥善保管、勿外传
- `diag.sh` 产物已脱敏，可放心发给技术支持排障
