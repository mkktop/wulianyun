#!/usr/bin/env bash
# ============================================================================
# KK物联云 离线交付包构建脚本
# 在【构建机】执行（需 Docker + buildx + Node.js + 仓库自带 .tools/go）。
# 产出 deploy/dist/kk-iot-<VER>-<ARCH>.tar.gz，客户机 docker load + compose up 即用，
# 全程不在客户机构建。
#
# 用法：
#   bash build.sh              # 默认只构建 amd64
#   bash build.sh amd64        # 显式单架构
#   bash build.sh amd64 arm64  # 多架构（arm64 需构建机先注册 QEMU，见 README）
# ============================================================================
set -euo pipefail

DIST_DIR="$(cd "$(dirname "$0")" && pwd)"     # deploy/dist
ROOT="$(cd "$DIST_DIR/../.." && pwd)"          # 仓库根
SERVER="$ROOT/server"; WEB="$ROOT/web"; DOCS="$ROOT/docs"
# Go 工具链：优先 $GO（CI 用 setup-go 注入 PATH），否则本地 .tools/go（Windows go.exe / Linux go），最后 PATH 的 go
if [ -n "${GO:-}" ]; then GO_BIN="$GO"
elif [ -x "$ROOT/.tools/go/bin/go" ]; then GO_BIN="$ROOT/.tools/go/bin/go"
elif [ -x "$ROOT/.tools/go/bin/go.exe" ]; then GO_BIN="$ROOT/.tools/go/bin/go.exe"
else GO_BIN="go"; fi

# ---------- 版本 ----------
VERSION="$(tr -d '[:space:]' < "$ROOT/VERSION")"
[ -n "$VERSION" ] || { echo "❌ 仓库根 VERSION 为空"; exit 1; }

# ---------- 目标架构（显式，绝不用 uname —— Git Bash 返回主机架构）----------
ARCHES=()
if [ $# -gt 0 ]; then ARCHES=("$@"); else ARCHES=(amd64); fi

export GOTOOLCHAIN=auto CGO_ENABLED=0

BUILD="$DIST_DIR/.build"
rm -rf "$BUILD"; mkdir -p "$BUILD"

echo "================ KK物联云 交付包构建 ================"
echo "  VERSION : $VERSION"
echo "  ARCHES  : ${ARCHES[*]}"
echo "======================================================"

# ============================================================================
# 1) 前端 + 开发文档（架构无关，只构建一次）
# ============================================================================
echo "▶ [1/2] 前端 + 开发文档 ..."
# 顺序：先 web build 生成 web/dist，再并入 docs 产物。
# （若反过来，CI 干净 checkout 无 web/dist（gitignored）会 cp 失败；且 vite 构建会清空 dist）
( cd "$WEB" && npm ci && npm run build )
( cd "$DOCS" && npm ci && npm run docs:build )
mkdir -p "$WEB/dist"
rm -rf "$WEB/dist/developer"
cp -r "$DOCS/.vitepress/dist" "$WEB/dist/developer"
echo "✅ 前端产物就绪"

# 暂存 web 构建上下文（绕开 web/.dockerignore 对 dist/ 的排除 —— 不动 web/ 目录）
WEB_CTX="$BUILD/web"
mkdir -p "$WEB_CTX"
cp -r "$WEB/dist" "$WEB_CTX/dist"
cp "$WEB/nginx.conf" "$WEB_CTX/nginx.conf"
cp "$DIST_DIR/Dockerfile.web" "$WEB_CTX/Dockerfile.web"
: > "$WEB_CTX/.dockerignore"   # 允许全部
echo "✅ 前端产物就绪"

# ============================================================================
# 2) 每个架构：server 镜像 + web 镜像 + 基础镜像 + 组装交付包
# ============================================================================
for ARCH in "${ARCHES[@]}"; do
  echo ""
  echo "▶ [2/2] 架构 $ARCH"

  # ---- server 二进制（Go 原生交叉编译，无需 QEMU）+ 薄镜像 ----
  echo "  · 编译 server ($ARCH) ..."
  ( cd "$SERVER" && GOOS=linux GOARCH="$ARCH" "$GO_BIN" build \
      -trimpath -ldflags="-s -w" -o "$BUILD/server-$ARCH" ./cmd/server )
  SRV_CTX="$BUILD/ctx-server-$ARCH"
  mkdir -p "$SRV_CTX/configs"
  cp "$BUILD/server-$ARCH" "$SRV_CTX/server"
  cp -r "$SERVER/configs/." "$SRV_CTX/configs/"
  cp "$DIST_DIR/Dockerfile.server" "$SRV_CTX/Dockerfile.server"
  echo "  · 构建 server 镜像 ..."
  docker buildx build --platform "linux/$ARCH" \
    -t "kk-iot/server:$VERSION-$ARCH" --load -f "$SRV_CTX/Dockerfile.server" "$SRV_CTX"

  # ---- web 镜像（按架构 —— nginx 基础镜像有架构分层，COPY 同一份 dist）----
  echo "  · 构建 web 镜像 ($ARCH) ..."
  docker buildx build --platform "linux/$ARCH" \
    -t "kk-iot/web:$VERSION-$ARCH" --load -f "$WEB_CTX/Dockerfile.web" "$WEB_CTX"

  # ---- 基础镜像（固定 tag，不用 latest；目标架构）----
  echo "  · 拉取基础镜像 ($ARCH) ..."
  docker pull --platform "linux/$ARCH" timescale/timescaledb:2.17.2-pg16
  docker pull --platform "linux/$ARCH" redis:7-alpine
  docker pull --platform "linux/$ARCH" emqx/emqx:5.8

  # ---- 组装交付目录 ----
  PKG="$DIST_DIR/kk-iot-$VERSION-$ARCH"
  rm -rf "$PKG"; mkdir -p "$PKG/images" "$PKG/compose"
  cp "$DIST_DIR/payload"/install.sh "$DIST_DIR/payload"/upgrade.sh \
     "$DIST_DIR/payload"/backup.sh "$DIST_DIR/payload"/restore.sh \
     "$DIST_DIR/payload"/diag.sh "$PKG/"
  cp "$DIST_DIR/payload/README.md" "$PKG/"
  echo "$VERSION" > "$PKG/VERSION"   # install.sh 据此写 .env 的 VER
  cp "$DIST_DIR/payload/compose/docker-compose.yml"      "$PKG/compose/"
  cp "$DIST_DIR/payload/compose/config.prod.template.yaml" "$PKG/compose/"
  cp "$DIST_DIR/payload/compose/emqx.conf"               "$PKG/compose/"
  cp "$WEB/nginx.conf" "$PKG/compose/nginx.conf"   # 参考/可选覆盖

  # ---- save 全部镜像（交付 tar 自包含，客户机无需联网）----
  echo "  · 打包镜像 ..."
  docker save "kk-iot/server:$VERSION-$ARCH"     -o "$PKG/images/server.tar"
  docker save "kk-iot/web:$VERSION-$ARCH"        -o "$PKG/images/web.tar"
  docker save timescale/timescaledb:2.17.2-pg16  -o "$PKG/images/postgres.tar"
  docker save redis:7-alpine                     -o "$PKG/images/redis.tar"
  docker save emqx/emqx:5.8                      -o "$PKG/images/emqx.tar"

  # ---- 压缩 + 校验和 ----
  ( cd "$DIST_DIR" && tar -czf "kk-iot-$VERSION-$ARCH.tar.gz" "kk-iot-$VERSION-$ARCH" )
  ( cd "$DIST_DIR" && sha256sum "kk-iot-$VERSION-$ARCH.tar.gz" > "kk-iot-$VERSION-$ARCH.tar.gz.sha256" )
  rm -rf "$PKG"

  echo "✅ 产出: deploy/dist/kk-iot-$VERSION-$ARCH.tar.gz (+ .sha256)"
done

rm -rf "$BUILD"
echo ""
echo "🎉 构建完成。交付包："
echo "   deploy/dist/kk-iot-$VERSION-*.tar.gz"
echo "   交付方式：scp 到客户机 → tar xzf → bash install.sh"
