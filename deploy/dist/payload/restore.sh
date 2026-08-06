#!/usr/bin/env bash
# ============================================================================
# KK物联云 恢复
# 用法：bash restore.sh <备份包.tar.gz>
# 会停 server/web/emqx/redis（保留 postgres 运行以执行 pg_restore），恢复后重启。
# ============================================================================
set -euo pipefail
[ "$(id -u)" = 0 ] || exec sudo "$0" "$@"
HERE="$(cd "$(dirname "$0")" && pwd)"; cd "$HERE"

BK="${1:-}"
{ [ -n "$BK" ] && [ -f "$BK" ]; } || { echo "用法: bash restore.sh <备份包.tar.gz>"; exit 1; }
[ -f .env ] || { echo "❌ 未安装（无 .env）"; exit 1; }

. ./.env
P="${COMPOSE_PROJECT_NAME:-kk-iot}"
CF="compose/docker-compose.yml"
WORK="$(mktemp -d)"; trap 'rm -rf "$WORK"' EXIT
tar xzf "$BK" -C "$WORK"

echo "▶ 停止 server/web/emqx/redis（保留 postgres）..."
docker compose -p "$P" -f "$CF" stop server web emqx redis

echo "▶ 恢复数据库（pg_restore --clean）..."
docker compose -p "$P" -f "$CF" exec -T postgres \
  pg_restore -U iot -d iot --clean --if-exists --no-owner < "$WORK/db.dump"

echo "▶ 恢复 uploads 卷 ..."
docker run --rm -v "${P}_uploads":/data -v "$WORK":/backup alpine \
  sh -c 'rm -rf /data/* && tar xzf /backup/uploads.tar.gz -C /data'

if [ -f "$WORK/redis.rdb" ]; then
  echo "▶ 恢复 redis 快照 ..."
  docker run --rm -v "${P}_redis-data":/data -v "$WORK":/backup alpine \
    sh -c 'cp /backup/redis.rdb /data/dump.rdb' 2>/dev/null || true
fi

[ -f "$WORK/config.prod.yaml" ] && cp "$WORK/config.prod.yaml" compose/config.prod.yaml

echo "▶ 启动服务 ..."
docker compose -p "$P" -f "$CF" up -d

echo "▶ 等待就绪（最长 90s）"
ok=0
for _ in $(seq 1 90); do
  if command -v curl >/dev/null 2>&1; then
    curl -fsS http://127.0.0.1/api/v1/readyz >/dev/null 2>&1 && { ok=1; break; }
  else
    docker compose -p "$P" -f "$CF" exec -T server \
      wget -qO- http://127.0.0.1:8080/api/v1/readyz >/dev/null 2>&1 && { ok=1; break; }
  fi
  printf '.'; sleep 1
done
echo ""
[ "$ok" = 1 ] && echo "✅ 恢复完成" || { echo "❌ 恢复后未就绪，运行 bash diag.sh"; exit 1; }
