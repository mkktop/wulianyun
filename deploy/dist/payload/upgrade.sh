#!/usr/bin/env bash
# ============================================================================
# KK物联云 升级（客户侧）
# 用法：bash upgrade.sh <新版本交付包目录>
# 只替换镜像 + 切版本号，postgres/redis/emqx 数据卷不动。
# ============================================================================
set -euo pipefail
[ "$(id -u)" = 0 ] || exec sudo "$0" "$@"

NEW_DIR="${1:-}"
[ -n "$NEW_DIR" ] || { echo "用法: bash upgrade.sh <新版本交付包目录>"; exit 1; }
HERE="$(cd "$(dirname "$0")" && pwd)"; cd "$HERE"
NEW_DIR="$(cd "$NEW_DIR" && pwd)"

{ [ -f "$NEW_DIR/VERSION" ] && [ -d "$NEW_DIR/images" ]; } || { echo "❌ $NEW_DIR 不是交付包（需 VERSION + images/）"; exit 1; }
[ -f .env ] || { echo "❌ 未安装（无 .env），先 bash install.sh"; exit 1; }

NEW_VER="$(tr -d '[:space:]' < "$NEW_DIR/VERSION")"
. ./.env
OLD_VER="${VER:-unknown}"
[ "$NEW_VER" != "$OLD_VER" ] || echo "⚠️  新版本号与当前相同（$OLD_VER）"

echo "▶ 升级 $OLD_VER → $NEW_VER"

echo "▶ 加载新镜像 ..."
for f in "$NEW_DIR"/images/*.tar; do
  [ -f "$f" ] || continue
  echo "  · $(basename "$f")"
  docker load -i "$f" >/dev/null
done

# 备份当前 .env 并切版本号
cp .env ".env.bak-$OLD_VER"
sed -i "s/^VER=.*/VER=${NEW_VER}/" .env

echo "▶ 重新启动（compose 优雅重建 server/web，数据卷不动）..."
docker compose -p kk-iot --env-file .env -f compose/docker-compose.yml up -d

echo "▶ 等待后端就绪（最长 90s）"
ok=0
for _ in $(seq 1 90); do
  if command -v curl >/dev/null 2>&1; then
    curl -fsS http://127.0.0.1/api/v1/readyz >/dev/null 2>&1 && { ok=1; break; }
  else
    docker compose -p kk-iot -f compose/docker-compose.yml exec -T server \
      wget -qO- http://127.0.0.1:8080/api/v1/readyz >/dev/null 2>&1 && { ok=1; break; }
  fi
  printf '.'; sleep 1
done
echo ""

if [ "$ok" = 1 ]; then
  echo "✅ 升级成功：$OLD_VER → $NEW_VER"
  echo "   回滚（如需）：sed -i 's/^VER=.*/VER=${OLD_VER}/' .env && docker compose -p kk-iot -f compose/docker-compose.yml up -d"
else
  echo "❌ 升级后未就绪！建议回滚："
  echo "   sed -i 's/^VER=.*/VER=${OLD_VER}/' .env"
  echo "   docker compose -p kk-iot -f compose/docker-compose.yml up -d"
  echo "   排障：bash diag.sh"
  exit 1
fi
