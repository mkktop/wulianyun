#!/usr/bin/env bash
# ============================================================================
# KK物联云 备份（在线，不中断服务）
# 产物：backup-<VER>-<时间戳>.tar.gz（含 db.dump / uploads / redis 快照 / 配置）
# ============================================================================
set -euo pipefail
[ "$(id -u)" = 0 ] || exec sudo "$0" "$@"
HERE="$(cd "$(dirname "$0")" && pwd)"; cd "$HERE"
[ -f .env ] || { echo "❌ 未安装（无 .env）"; exit 1; }

. ./.env
P="${COMPOSE_PROJECT_NAME:-kk-iot}"
CF="compose/docker-compose.yml"
TS="$(date +%Y%m%d-%H%M%S)"
WORK="$(mktemp -d)"; trap 'rm -rf "$WORK"' EXIT

echo "▶ 备份数据库（pg_dump 在线，自定义格式）..."
docker compose -p "$P" -f "$CF" exec -T postgres pg_dump -U iot -Fc iot > "$WORK/db.dump"

echo "▶ 备份 uploads 卷 ..."
docker run --rm -v "${P}_uploads":/data -v "$WORK":/backup alpine \
  tar czf /backup/uploads.tar.gz -C /data .

echo "▶ 备份 redis 快照（缓存，非致命）..."
if docker compose -p "$P" -f "$CF" exec -T redis \
  sh -c 'redis-cli -a "$REDIS_PASSWORD" --rdb /data/dump-backup.rdb' >/dev/null 2>&1; then
  docker run --rm -v "${P}_redis-data":/data -v "$WORK":/backup alpine \
    cp /data/dump-backup.rdb /backup/redis.rdb 2>/dev/null || true
else
  echo "  ⚠️  redis 快照失败（缓存可从 DB 重建，非致命）"
fi

echo "▶ 备份配置 ..."
cp .env "$WORK/env"
cp compose/config.prod.yaml "$WORK/config.prod.yaml"

OUT="backup-${VER:-unknown}-$TS.tar.gz"
tar -czf "$OUT" -C "$WORK" .
sha256sum "$OUT" > "$OUT.sha256"
echo "✅ 备份完成: $OUT (+ .sha256)"
echo "   恢复: bash restore.sh $OUT"
