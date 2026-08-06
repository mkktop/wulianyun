#!/usr/bin/env bash
# ============================================================================
# KK物联云 离线交付包 —— 一键安装（客户侧）
# 用法：bash install.sh [--force]
# 流程：预检 → 加载镜像 → 生成密钥 → 渲染配置 → 启动 → 就绪探测 → 输出
# 全程不联网、不构建。
# ============================================================================
set -euo pipefail

# ---------- 0. root ----------
[ "$(id -u)" = 0 ] || { echo "ℹ️  需要 root，自动以 sudo 重提"; exec sudo "$0" "$@"; }

FORCE=0
[[ "${1:-}" == "--force" ]] && FORCE=1

HERE="$(cd "$(dirname "$0")" && pwd)"
cd "$HERE"

# ---------- 1. 预检：OS / 架构 ----------
[ "$(uname -s)" = "Linux" ] || { echo "❌ 仅支持 Linux"; exit 1; }
case "$(uname -m)" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) echo "❌ 不支持的架构: $(uname -m)（仅 amd64/arm64）"; exit 1 ;;
esac

VER="$(tr -d '[:space:]' < VERSION 2>/dev/null || true)"
[ -n "$VER" ] || { echo "❌ 缺少 VERSION 文件（交付包损坏？）"; exit 1; }

echo "================ KK物联云 安装 ================"
echo "  版本: $VER    架构: $ARCH"
echo "==============================================="

# ---------- 2. 预检：Docker ----------
command -v docker >/dev/null 2>&1 || { echo "❌ 缺少 docker（安装：curl -fsSL https://get.docker.com | sh，需联网）"; exit 1; }
docker compose version >/dev/null 2>&1 || { echo "❌ 缺少 docker compose v2（apt-get install -y docker-compose-plugin）"; exit 1; }

# ---------- 3. 预检：资源 / 端口 ----------
mem_mb="$(awk '/MemTotal/{printf "%d", $2/1024}' /proc/meminfo 2>/dev/null || echo 0)"
[ "${mem_mb:-0}" -ge 1500 ] || echo "⚠️  内存 ${mem_mb}MB < 推荐 1500MB，可能不稳定（建议 ≥2GB）"
disk_mb="$(df -m . 2>/dev/null | awk 'NR==2{print $4}')"
[ "${disk_mb:-0}" -ge 10240 ] || echo "⚠️  剩余磁盘 ${disk_mb}MB < 10GB"

port_busy() { ss -tlnH "sport = :$1" 2>/dev/null | grep -q ":$1" || netstat -tln 2>/dev/null | grep -q ":$1 "; }
for p in 80 1883 9100 8083; do port_busy "$p" && echo "⚠️  端口 $p 已被占用，可能冲突" || true; done

# ---------- 4. 加载镜像 ----------
echo "▶ 加载离线镜像 ..."
count=0
for f in images/*.tar; do
  [ -f "$f" ] || continue
  echo "  · $(basename "$f")"
  docker load -i "$f" >/dev/null
  count=$((count+1))
done
[ "$count" -ge 5 ] || echo "⚠️  仅加载 $count 个镜像（预期 5）"
echo "✅ 镜像就绪"

# ---------- 5. 生成密钥（幂等） ----------
rand_hex() { # $1=字节数
  if command -v openssl >/dev/null 2>&1; then openssl rand -hex "$1"
  else head -c "$1" /dev/urandom | od -An -tx1 | tr -d ' \n'; fi
}
rand_b64() { # $1=字节数
  if command -v openssl >/dev/null 2>&1; then openssl rand -base64 "$1" | tr -d '/+=\n'
  else head -c "$1" /dev/urandom | base64 | tr -d '/+=\n'; fi
}

if [ -f .env ] && [ "$FORCE" -ne 1 ]; then
  echo "ℹ️  .env 已存在，复用（--force 重新生成）"
else
  echo "▶ 生成强密钥 ..."
  cat > .env <<EOF
# KK物联云 密钥（install.sh 生成，请勿提交）
POSTGRES_PASSWORD=$(rand_hex 16)
REDIS_PASSWORD=$(rand_hex 24)
JWT_SECRET=$(rand_b64 48)
MQTT_INTERNAL_PASSWORD=$(rand_hex 24)
ADMIN_PASSWORD=$(rand_b64 18)
EMQX_DASHBOARD_PASSWORD=$(rand_b64 18)

# 以下由 install 自动写入，请勿手改
VER=${VER}
ARCH=${ARCH}
COMPOSE_PROJECT_NAME=kk-iot
EOF
  chmod 600 .env
  echo "✅ .env 已生成"
fi

# ---------- 6. 渲染 config.prod.yaml ----------
set -a; . ./.env; set +a

if command -v envsubst >/dev/null 2>&1; then
  envsubst < compose/config.prod.template.yaml > compose/config.prod.yaml
else
  # 纯 bash 回退：白名单变量替换（无 eval，不引入 gettext 依赖）
  content="$(cat compose/config.prod.template.yaml)"
  for var in JWT_SECRET ADMIN_PASSWORD POSTGRES_PASSWORD REDIS_PASSWORD MQTT_INTERNAL_PASSWORD; do
    content="${content//\$\{${var}\}/${!var}}"
  done
  printf '%s\n' "$content" > compose/config.prod.yaml
fi
chmod 600 compose/config.prod.yaml
grep -q '\${' compose/config.prod.yaml && { echo "❌ config.prod.yaml 仍有未替换占位符，检查 .env"; exit 1; }
echo "✅ 配置已渲染"

# ---------- 7. 启动 ----------
echo "▶ 启动服务 ..."
docker compose -p kk-iot --env-file .env -f compose/docker-compose.yml up -d

# ---------- 8. 就绪探测（轮询 /readyz） ----------
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

if [ "$ok" -ne 1 ]; then
  echo "❌ 后端 90s 内未就绪"
  echo "   排障：bash diag.sh"
  echo "   日志：docker compose -p kk-iot -f compose/docker-compose.yml logs --tail=100 server"
  exit 1
fi

# ---------- 9. 输出 ----------
IP="$(hostname -I 2>/dev/null | awk '{print $1}')"
ADMIN="$(grep '^ADMIN_PASSWORD=' .env | cut -d= -f2-)"
EMQXPW="$(grep '^EMQX_DASHBOARD_PASSWORD=' .env | cut -d= -f2-)"

cat <<EOF

============================================================
  🎉 KK物联云 部署成功！
============================================================
  访问地址 : http://${IP:-<服务器IP>}
  管理员   : admin
  管理密码 : ${ADMIN}

  EMQX 面板（仅本机 18083，经 SSH 隧道访问）:
    admin / ${EMQXPW}
    （仅首次引导设置；改密码需删 emqx-data 卷重建，见 README）

  设备接入端口：1883(MQTT) / 9100(DTU-TCP)
============================================================
  ⚠️ 请立即妥善保存以上密码！

  常用命令：
    备份 : bash backup.sh
    恢复 : bash restore.sh <备份包>
    升级 : bash upgrade.sh <新版本交付包目录>
    诊断 : bash diag.sh
============================================================
EOF
