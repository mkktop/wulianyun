# KK物联云 离线交付包 —— 构建指南（开发侧）

本目录用于在**构建机**上产出私有化交付的离线安装包。客户机只需 `docker load` + `docker compose up -d`，**绝不在客户机构建**。

## 构建机前提

- Docker + buildx（Docker Desktop 自带；Linux 装 `docker-buildx-plugin`）
- Node.js（用于 web/docs 构建，`npm ci`）
- 仓库自带 Go 1.25 工具链（`.tools/go`），无需系统 go
- （仅 arm64）一次性注册 QEMU：
  ```bash
  docker run --rm --privileged multiarch/qemu-user-static --reset -p yes
  ```

## 用法

在仓库根：

```bash
# 默认只构建 amd64（适配绝大多数 x86 客户）
cd deploy/dist && bash build.sh

# 显式单架构
bash build.sh amd64

# 多架构（arm64 需先注册 QEMU）
bash build.sh amd64 arm64
```

## CI 自动构建（GitHub Actions）

`.github/workflows/build-release.yml` 已配好，**推荐用 CI 构建**而非本地（环境一致、不占本机、免装 Docker）：

- **打 tag `v*` 自动构建 + 发 Release**：`git tag v1.0.0 && git push --tags` → CI 构建 amd64 包，发布到 GitHub Releases（含 `.tar.gz` + `.sha256`）。tag 号必须与 `VERSION` 一致，否则 CI 失败。
- **手动触发**：仓库 Actions 页 → *构建离线交付包* → Run workflow → 选架构（amd64 / arm64 / 双架构）→ 产物在该次运行的 Artifact 里下载。
- arm64 构建会自动注册 QEMU，无需手动配置。

## 产物

```
deploy/dist/kk-iot-<VER>-<ARCH>.tar.gz         # 交付包
deploy/dist/kk-iot-<VER>-<ARCH>.tar.gz.sha256  # 校验和
```

包内结构：

```
kk-iot-<VER>-<ARCH>/
  install.sh upgrade.sh backup.sh restore.sh diag.sh   # 客户侧脚本
  README.md                                            # 客户实施手册
  compose/
    docker-compose.yml          # 交付版 compose（纯 image，无 build）
    config.prod.template.yaml   # 配置模板（6 个占位符）
    emqx.conf                   # 生产 EMQX 配置（回调 server:8080）
    nginx.conf                  # 参考，便于客户改 nginx（镜像已内置）
  images/
    server.tar web.tar postgres.tar redis.tar emqx.tar  # docker save 的离线镜像
```

## 版本号

仓库根 `VERSION` 文件（semver，如 `1.0.0`）。`build.sh` 读取它作为镜像 tag 与包名。
发版前更新 `VERSION`。后续会加 `git tag` 对齐与 ldflags 注入（第二期）。

## 工作原理（为什么不会在客户机卡住）

1. **构建机**编译 server 二进制（Go 交叉编译）+ 前端 dist（npm），用**薄单阶段 Dockerfile** 打成 `kk-iot/server`、`kk-iot/web` 镜像；
2. 连同基础镜像（timescaledb / redis / emqx，固定 tag）一起 `docker save` 成离线 tar；
3. **客户机** `docker load` 这些 tar，`docker compose up -d` 全用 `image:` 启动——不触发任何 `build:`。

这正是对「服务器弱、不能构建」的根治：弱机只 load 不 build。

## 验证（构建后自检）

```bash
# 1. 包结构
tar tzf kk-iot-<VER>-amd64.tar.gz | head

# 2. 镜像架构正确
mkdir /tmp/kk && tar xzf kk-iot-<VER>-amd64.tar.gz -C /tmp/kk
docker load -i /tmp/kk/kk-iot-*/images/server.tar
docker image inspect "kk-iot/server:<VER>-amd64" --format '{{.Architecture}}'   # → amd64
```

完整端到端验证（干净 Linux VM 断网模拟客户）见 `payload/README.md`。

## 与 deploy/prod 的关系

`deploy/prod/` 是**在线构建**路径（compose 带 `build:`，构建机即服务器），适合内部开发/staging。私有化交付一律用本目录 `deploy/dist/`。`deploy/prod/README.md` 已标注弃用引导。
