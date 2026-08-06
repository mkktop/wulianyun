#!/usr/bin/env bash
# ============================================================================
# KK物联云 诊断导出（脱敏）—— 打包给技术支持
# 产物：diag-<时间戳>.tar.gz
# ============================================================================
set -euo pipefail
[ "$(id -u)" = 0 ] || exec sudo "$0" "$@"
HERE="$(cd "$(dirname "$0")" && pwd)"; cd "$HERE"

. ./.env 2>/dev/null || true
P="${COMPOSE_PROJECT_NAME:-kk-iot}"
CF="compose/docker-compose.yml"
TS="$(date +%Y%m%d-%H%M%S)"
WORK="$(mktemp -d)"; trap 'rm -rf "$WORK"' EXIT

echo "▶ 采集诊断信息 ..."
docker compose -p "$P" -f "$CF" ps -a > "$WORK/01-ps.txt" 2>&1 || true
for svc in postgres redis emqx server web; do
  docker compose -p "$P" -f "$CF" logs --tail=300 "$svc" > "$WORK/logs-$svc.txt" 2>&1 || true
done
docker stats --no-stream > "$WORK/02-stats.txt" 2>&1 || true
docker images > "$WORK/03-images.txt" 2>&1 || true
free -m > "$WORK/04-mem.txt" 2>&1 || true
df -h > "$WORK/05-disk.txt" 2>&1 || true
{ ss -tlnp 2>/dev/null || netstat -tlnp 2>/dev/null; } > "$WORK/06-ports.txt" 2>&1 || true
uname -a > "$WORK/07-uname.txt" 2>&1

# 脱敏配置（屏蔽 password/secret 值）
mask() { sed -E 's/(password|secret)([^:=]*[=:]).*/\1\2***/Ig'; }
[ -f compose/config.prod.yaml ] && mask < compose/config.prod.yaml > "$WORK/08-config.masked.yaml" || true
[ -f .env ] && mask < .env > "$WORK/09-env.masked" || true

OUT="diag-$TS.tar.gz"
tar -czf "$OUT" -C "$WORK" .

# 复查无明文密钥
if tar xzfO "$OUT" 2>/dev/null | grep -Ei '(password|secret)[^=]*=[^*]{6,}' >/dev/null 2>&1; then
  echo "⚠️  诊断包可能含明文密钥，请复查 $OUT 再发出"
else
  echo "✅ 诊断包已生成: $OUT（已脱敏）"
fi
echo "   发给技术支持即可。"
