# KK物联云 安装与运维手册

本包是 KK物联云 的**离线交付包**。客户机无需联网、无需构建，按以下步骤部署即可。

## 一、环境要求

| 项 | 要求 |
|---|---|
| 操作系统 | Linux x86_64 / arm64 |
| 内存 | ≥ 2GB（1.5GB 可跑但不推荐；越大越稳） |
| 磁盘 | ≥ 10GB 可用 |
| 软件 | Docker（含 compose v2 插件） |
| 端口 | 80 / 1883 / 9100 / 8083 可用（8080/5432/6379 仅内部） |

未装 Docker：`curl -fsSL https://get.docker.com | sh`（需联网一次）。

## 二、安装

```bash
tar xzf kk-iot-<版本>-<架构>.tar.gz
cd kk-iot-<版本>-<架构>
bash install.sh
```

脚本自动：加载离线镜像 → 生成密钥 → 渲染配置 → 启动 → 等待就绪。
成功后打印访问地址、管理员密码、EMQX 面板密码。**请立即保存密码。**

重新生成密钥：`bash install.sh --force`（会覆盖 `.env`）。

## 三、访问

- 管理后台：`http://<服务器IP>`，账号 `admin` + 安装时打印的管理密码
- 设备接入：MQTT `1883`、DTU/TCP `9100`
- EMQX 面板：仅本机 `18083`，需 SSH 隧道（`ssh -L 18083:127.0.0.1:18083 root@<IP>`），账号 `admin` + 安装时打印的 EMQX 密码

## 四、日常运维

| 操作 | 命令 |
|---|---|
| 备份 | `bash backup.sh` → 生成 `backup-*.tar.gz` |
| 恢复 | `bash restore.sh <备份包>` |
| 升级 | `bash upgrade.sh <新版本交付包目录>` |
| 诊断 | `bash diag.sh` → 生成 `diag-*.tar.gz`（已脱敏）发给技术支持 |
| 重启 | `docker compose -p kk-iot -f compose/docker-compose.yml restart` |
| 改配置 | 编辑 `compose/config.prod.yaml` → 上面的 restart 命令 |
| 查日志 | `docker compose -p kk-iot -f compose/docker-compose.yml logs -f server` |

## 五、内存调优

compose 内置适配弱机的内存上限（postgres 512m / emqx 384m / server 256m / redis 128m / web 48m，合计约 1.5GB）。
若服务器内存充裕（≥4GB），可放大限制：编辑 `compose/docker-compose.yml` 各服务的 `deploy.resources.limits.memory`，再 `docker compose -p kk-iot -f compose/docker-compose.yml up -d`。

## 六、关于 EMQX 面板密码

EMQX 面板密码**仅在首次安装时**写入（emqx-data 卷为空时生效）。之后改 `.env` 的 `EMQX_DASHBOARD_PASSWORD` 不会生效。
如需重置：

```bash
docker compose -p kk-iot -f compose/docker-compose.yml stop emqx
docker volume rm kk-iot_emqx-data        # 丢弃 EMQX 运行状态，不影响设备业务数据
# 编辑 .env 的 EMQX_DASHBOARD_PASSWORD
docker compose -p kk-iot -f compose/docker-compose.yml up -d
```

## 七、端口表

| 端口 | 用途 | 公网 |
|---|---|---|
| 80 | 管理后台（nginx） | 是 |
| 1883 | MQTT 设备接入 | 是 |
| 9100 | DTU/TCP 设备接入 | 是 |
| 8083 | MQTT over WebSocket（可选） | 是 |
| 18083 | EMQX 面板 | 仅本机 |
| 8080 / 5432 / 6379 | 后端 / DB / Redis | 仅内部 |

## 八、排障

1. `bash diag.sh` 打包诊断信息（脱敏），发给技术支持
2. 查后端日志：`docker compose -p kk-iot -f compose/docker-compose.yml logs --tail=200 server`
3. 重启全部：`docker compose -p kk-iot -f compose/docker-compose.yml restart`

## 九、升级回滚

`upgrade.sh` 会保留旧版本号到 `.env.bak-<旧版本>`，并打印回滚命令。
注意：数据库迁移是**向前兼容**的（GORM AutoMigrate），降级到旧版本一般安全，但若新版本有破坏性的表结构变更，降级前请先备份。

---
版本：见包内 `VERSION` 文件。
