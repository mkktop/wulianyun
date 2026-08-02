# KK 物联云 —— 生产部署手册

> 全 Docker Compose 一键部署（Linux x86_64，IP 访问）。一条命令拉起后端 + 前端 + 数据库 + Redis + EMQX。

---

## 目录

- [快速开始](#快速开始)
- [架构与端口](#架构与端口)
- [首次登录](#首次登录)
- [防火墙](#防火墙)
- [接入设备](#接入设备)
- [日常运维](#日常运维)
- [故障排查](#故障排查)

---

## 快速开始

### 0. 前置要求

- Linux 服务器（x86_64），≥ 2 核 / 2GB 内存（推荐 4GB）/ 20GB 磁盘
- Docker + docker compose v2
- `openssl`、`envsubst`（gettext 包）、`bash`

### 1. 安装 Docker（若未安装）

```bash
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER
# 重新登录 shell 后生效
sudo systemctl enable --now docker
```

验证：

```bash
docker --version
docker compose version
```

### 2. 获取代码

```bash
# 方式 A：git clone
git clone <你的仓库地址> /opt/wulianyun
cd /opt/wulianyun/deploy/prod

# 方式 B：本地 scp 上传
scp -r . user@SERVER_IP:/opt/wulianyun
```

### 3. 生成密钥与配置

```bash
cd /opt/wulianyun/deploy/prod
bash setup.sh
```

脚本会：
- 用 `openssl rand` 生成 6 组强密钥（数据库、Redis、JWT、MQTT 内部账号、管理员、EMQX Dashboard）
- 写入 `.env`（权限 600，gitignore）
- 渲染 `config.prod.template.yaml` → `config.prod.yaml`（gitignore）
- **打印管理员密码和 EMQX Dashboard 密码 —— 请立即保存**

如需重新生成：`bash setup.sh --force`

### 4. 构建并启动

```bash
docker compose up -d --build
```

首次构建约 3-8 分钟（拉取基础镜像 + 编译 Go + 构建 Vue）。观察后端日志：

```bash
docker compose logs -f server
```

看到 `iot-platform server started addr=:8080` 和 `mqtt connected` 即成功。

### 5. 访问

- **管理后台**：`http://<服务器IP>`
- **登录**：`admin` / setup.sh 打印的管理员密码
- **EMQX Dashboard**（可选）：
  ```bash
  ssh -L 18083:127.0.0.1:18083 user@SERVER_IP
  # 本地浏览器打开 http://localhost:18083
  # 账号 admin / setup.sh 打印的 EMQX Dashboard 密码
  ```

---

## 架构与端口

```
公网 ──:80──► web(nginx) ──/api/,/ws──► server:8080（内部）
   :1883──► emqx ──HTTP 回调──► server:8080（内部）
   :9100──► server（TCP 透传网关，DTU 设备）
   127.0.0.1:18083──► emqx dashboard（仅 SSH 隧道）

内部网络 iot-net：
   server ──► postgres:5432 / redis:6379 / emqx:1883（服务名 DNS）
```

| 端口 | 服务 | 暴露范围 |
|---|---|---|
| 80 | web (nginx) | 公网 |
| 1883 | emqx (MQTT) | 公网 |
| 8083 | emqx (MQTT-WS) | 公网（可选） |
| 9100 | server (TCP 网关) | 公网 |
| 18083 | emqx dashboard | 仅 127.0.0.1 |
| 8080 | server (HTTP) | 仅内部 |
| 5432 | postgres | 仅内部 |
| 6379 | redis | 仅内部 |

---

## 首次登录

1. 浏览器打开 `http://<服务器IP>`
2. 输入 `admin` + setup.sh 打印的管理员密码
3. 概览页显示平台统计即部署成功

---

## 防火墙

使用 `ufw`（Ubuntu/Debian）：

```bash
sudo ufw allow 22/tcp      # SSH
sudo ufw allow 80/tcp      # Web 管理后台
sudo ufw allow 1883/tcp    # MQTT 设备接入
sudo ufw allow 9100/tcp    # TCP 透传（DTU）
# 不放行 18083（已绑本机）/ 5432 / 6379 / 8080
sudo ufw enable
```

如需对特定 IP 开放 EMQX Dashboard：

```bash
# 修改 docker-compose.yml 中 emqx 端口映射为 "ADMIN_IP:18083:18083"
sudo ufw allow from ADMIN_IP to any port 18083
```

---

## 接入设备

### MQTT 设备

| 项 | 值 |
|---|---|
| Broker | `tcp://<服务器IP>:1883` |
| ClientID | `{productKey}.{deviceName}` |
| Username | `{deviceName}` |
| Password | 设备 Secret（或动态 Token） |
| 上行 | `thing/up/{productKey}/{deviceName}` |
| 下行 | `thing/down/{productKey}/{deviceName}` |

### 测试设备（模拟器）

```bash
cd tools/simulator
npm install
node simulator.js <productKey> <deviceName> <deviceSecret>
```

### TCP 透传（DTU）

设备连接 `<服务器IP>:9100`，发送注册包：

```
{productKey},{deviceName},{secret}\n
```

成功回复 `OK\n`，之后按产品配置的组帧/心跳工作。

---

## 日常运维

```bash
cd /opt/wulianyun/deploy/prod

# 查看全部容器状态
docker compose ps

# 查看后端日志
docker compose logs -f server

# 修改配置后重启
# 编辑 config.prod.yaml，然后：
docker compose restart server

# 更新代码后重新构建
git pull
docker compose up -d --build server web

# 停止（保留数据卷）
docker compose down

# ⚠️ 停止并删除数据（不可恢复）
docker compose down -v

# 进入数据库
docker compose exec postgres psql -U iot -d iot

# 进入 Redis
docker compose exec redis redis-cli -a $(grep REDIS_PASSWORD .env | cut -d= -f2)
```

---

## 故障排查

### 后端启动失败，日志报数据库连接错误

```bash
# 检查 postgres 是否就绪
docker compose logs postgres | tail -20
docker compose exec postgres pg_isready -U iot -d iot
```

如果 postgres 反复重启，检查 `.env` 中 `POSTGRES_PASSWORD` 是否与首次生成一致（不要手动改 `.env` 后不重建卷）。

### EMQX 设备无法连接

```bash
# 检查 emqx 是否回调后端成功
docker compose logs emqx | grep -i "auth\|acl\|error"

# 从服务器本机测试 MQTT 连接
docker compose exec emqx emqx_ctl clients list
```

常见原因：
- 后端未就绪（等 server healthy 后再试）
- 设备三元组错误
- 防火墙未放行 1883

### Web 页面 404 或白屏

```bash
# 检查 nginx 日志
docker compose logs web

# 确认后端 API 可达
curl http://localhost/api/v1/overview
```

### OTA 固件无法下载

确认 `router.go` 已包含 `r.Static("/uploads", "./uploads")`，且 `uploads` 卷已挂载。测试：

```bash
# 上传固件后，从浏览器访问返回的 fileURL
curl -I http://<服务器IP>/uploads/firmware/xxx
```

### 重置管理员密码

直接更新数据库：

```bash
docker compose exec postgres psql -U iot -d iot -c \
  "UPDATE users SET password_hash = '\$2a\$10\$...' WHERE username = 'admin';"
```

（bcrypt hash 可用在线工具或 `htpasswd -bnBC 10 "" 新密码` 生成）

---

## 安全提醒

- 所有默认密钥已由 `setup.sh` 替换为随机强密钥
- `18083`（EMQX Dashboard）仅绑定 `127.0.0.1`，需 SSH 隧道访问
- `5432`（Postgres）、`6379`（Redis）、`8080`（后端 HTTP）不暴露到宿主机
- 生产环境建议：启用 MQTT TLS、配置域名 + HTTPS、限制 SSH 来源 IP
