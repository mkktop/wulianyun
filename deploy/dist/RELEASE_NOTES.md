# KK物联云 私有化离线交付包 v@VERSION@

KK物联云是一套企业级物联网设备管理平台（Go 后端 + Vue3 控制台 + EMQX MQTT + TimescaleDB 时序存储）。
本交付包为**离线自包含安装包**：全部组件镜像打包在内，客户机**无需联网、无需构建**，解压后一键安装。

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

- 操作系统：Linux（amd64 或 arm64）
- 已安装 **Docker** + **docker compose v2**（install.sh 会检测；完全离线机需提前备好离线 Docker 安装包）
- 内存 ≥1.5GB（推荐 ≥2GB）、磁盘剩余 ≥10GB
- 目标端口：80(Web) / 1883(MQTT) / 9100(DTU-TCP) / 8083(MQTT-WS)，占用会警告

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
| 查看日志 | `docker compose -p kk-iot logs -f server` |
| 修改配置 | 编辑 `compose/config.prod.yaml` 后 `docker compose -p kk-iot -f compose/docker-compose.yml up -d` |

## 升级

```bash
# 下载新版本包并解压，在当前安装目录执行：
bash upgrade.sh <新版本包目录>
```

- 只替换 server/web 镜像并切换版本号，**postgres / redis / emqx / uploads 数据卷不动**
- 升级前先 `bash backup.sh`
- 升级失败回滚：`sed -i 's/^VER=.*/VER=<旧版本>/' .env && docker compose -p kk-iot -f compose/docker-compose.yml up -d`

## 迁移（换服务器）

新机器安装同版本 → `bash restore.sh <旧服务器备份包>` → 数据库、固件上传、配置完整迁移。

## 注意事项

- 交付包离线自包含：客户机全程不访问公网，无外网依赖
- `.env` / `compose/config.prod.yaml` 含密钥（600 权限），请妥善保管、勿外传
- `diag.sh` 产物已脱敏，可放心发给技术支持排障
- EMQX 面板密码仅首次引导设置；修改需删除 `emqx-data` 卷重建（先备份）
