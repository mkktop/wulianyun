# 部署与烧录工作流（开发笔记，2026-08 实测验证）

> 从 ZCode 本地记忆导出（2026-10-18 重装电脑前备份）。**敏感凭据已脱敏**：`<服务器IP>`、`<root密码>`、`<平台主账号>` / `<平台密码>` 为占位符，真实值不进仓库，需自行留存于安全位置（密码管理器等）。

## 平台热部署（iot-server / iot-web 容器）

**SSH 凭据**：`root@<服务器IP>` / `<root密码>`（paramiko，Windows 本机无 sshpass）
**平台 API 登录**：`<平台主账号>` / `<平台密码>`（主账号，可建产品；注意 SSH 密码与平台密码不是同一个，勿混用）

### 后端 server-linux 热部署（关键步骤）
1. 交叉编译：`cd server && GOOS=linux GOARCH=amd64 ./.tools/go/bin/go.exe build -o bin/server-linux ./cmd/server`
2. paramiko SFTP 上传到 `/tmp/server-linux.x`，宿主 `chmod 755`
3. `docker cp /tmp/server-linux.x iot-server:/tmp/server.x`
4. **必须 `docker exec -u 0:0`**（容器用户 uid 10001 无权 mv root 文件）：
   `docker exec -u 0:0 iot-server sh -c 'cp -f /tmp/server.x /app/server && chmod 755 /app/server && chown 10001:10001 /app/server'`
5. `docker restart iot-server`
6. 验证：容器内 `md5sum /app/server` 与本地一致；`docker logs` 看启动；API 登录测试

### 前端 web/dist 部署
1. `cd web && npm run build`（vue-tsc + vite）
2. 打包 `tar czf`（**排除 developer/，保留其旧版**）：`index.html assets stats.html`
3. `docker cp` 到 iot-web 容器 → `docker exec iot-web sh -c 'cd /usr/share/nginx/html && tar xzf /tmp/... && rm ...'`
4. nginx 静态文件即时生效，无需重启
5. 验证：`docker exec iot-web grep -l 新字段名 assets/*.js` 或 curl 页面

### SSH 坑位
- **频繁连接会被 sshd 限速**：banner_timeout 调大(60-90s) + 重试循环(5次×15s)，或等 30-60s 冷却
- **别留僵尸会话**：脚本被杀会留下 sshd 会话 → MaxSessions 耗尽 → "Timeout opening channel"。宁可单连接多命令，勿反复连
- Python 读本地文件用 **Windows 路径**（`C:\...`），Git Bash 的 `/tmp` 在 Python 里不存在
- 主机曾因 docker cp 大文件 + 并发 SSH 卡死过一次（API 无响应、连 uptime 都跑不出），需重启 ECS/容器恢复

## ESP32-S3 固件（D:\work\mqtt，非 git 仓）

- 串口：**COM6**（用 `powershell Get-CimInstance Win32_SerialPort` 查，会变）
- 构建+烧录：`cd /d/work/mqtt && MSYS_NO_PATHCONV=1 cmd /c "_build.cmd build"` / `_build.cmd -p COM6 flash`
- 监控：`_build.cmd -p COM6 monitor`（后台跑会占 COM6，kill 残留用 `taskkill //F //IM python.exe` 或精确按 cmdline 杀；**别用宽泛 PowerShell kill，会杀到自己的 shell**）
- 传输切换：`main.c` 的 `APP_TRANSPORT_MQTT`（1=MQTT / 0=TCP）；`bsp/tcp.c` 的 `TCP_DATA_MODE`（0=透传JSON / 1=脚本二进制 / 2=Modbus 从机）+ 对应三元组 #define
- 烧录后设备自动重连平台，无需其他操作

## 发布（git）
- bump `VERSION` + `deploy/dist/RELEASE_NOTES.md` 顶部追加条目 → 提交 → `git push origin master` → `git tag -a v1.0.x -m ...` → `git push origin v1.0.x`
- 离线交付包：`deploy/dist/build.sh`（VERSION 驱动，构建 kk-iot 镜像 + tar.gz）

### push 坑位（GitHub 网络不稳定）
- 本机 git config + 环境变量都指向本地代理 `127.0.0.1:7897`；**直连和代理两条路都可能时通时断**（schannel "server closed abruptly" / "Connection reset" / 443 连不上）
- 不可靠信号：`curl https://github.com` 返回 200 ≠ git push 能成功
- 解法：写循环重试（5 次×10s），**两条路都试**——先 `git push`（走代理），失败再 `git -c http.proxy= -c https.proxy= push`（直连）。push 幂等可安全重试
- 判断成败看 `git push` 自身退出码，**别用 `git push | tail`**（tail 恒为 0 会误判成功）

### commit message 中文
- 用 `git commit -F <file>`（写 UTF-8 文件）读入，别用 `-m "中文"`（Git Bash 命令行传中文会打坏 UTF-8）

## 平台关键行为（测试必知）
- 产品 accessMode/protocol 创建后不可变 → TCP/Modbus 测试需**新建产品**
- TCP 设备 codec 脚本**建连时缓存**，改 codec 需设备重连才生效
- length 组帧的帧**含长度字节**，codec.decode 索引从长度字节后算
- 心跳帧必须符合产品组帧格式（delimiter 产品 PING 要带 0D0A）
- Modbus：平台主动轮询（pollInterval），批量合并读 maxGap=8、寄存器上限120，单请求超时 requestTimeout 默认3s（1.0.8 起产品级可配[3,30]）

## 编码教训
- **Git Bash 命令行直接传中文会乱码**（UTF-8 被打坏 → 数据库存 U+FFFD 不可逆）→ 中文写入一律用 Node 脚本（UTF-8 显式）或 psql `convert_from(decode(hex))`
- 已修复案例：产品/设备名、主账号 nickname 显示名（psql 直改）
