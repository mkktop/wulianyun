#!/usr/bin/env bash
# KK物联云 生产部署初始化脚本
# 功能：预检依赖 → 生成强密钥(.env) → 渲染 config.prod.yaml
# 用法：bash setup.sh [--force]
set -euo pipefail

FORCE=0
[[ "${1:-}" == "--force" ]] && FORCE=1

# ---------- 1. 预检依赖 ----------
for cmd in docker openssl envsubst; do
    if ! command -v "$cmd" &>/dev/null; then
        echo "❌ 缺少依赖: $cmd"
        if [ "$cmd" = "envsubst" ]; then
            echo "   安装 gettext:  apt-get install -y gettext   (Debian/Ubuntu)"
        else
            echo "   安装 Docker:  curl -fsSL https://get.docker.com | sh"
        fi
        exit 1
    fi
done
if ! docker compose version &>/dev/null; then
    echo "❌ 缺少 docker compose（Compose v2 插件）"
    echo "   apt-get install -y docker-compose-plugin"
    exit 1
fi

# ---------- 2. 生成密钥（幂等，--force 覆盖） ----------
if [ -f .env ] && [ "$FORCE" -ne 1 ]; then
    echo "⚠️  .env 已存在，跳过密钥生成（使用 --force 强制重新生成）"
else
    echo "🔑 生成强密钥..."
    POSTGRES_PASSWORD=$(openssl rand -hex 16)
    REDIS_PASSWORD=$(openssl rand -hex 24)
    JWT_SECRET=$(openssl rand -base64 48)
    MQTT_INTERNAL_PASSWORD=$(openssl rand -hex 24)
    ADMIN_PASSWORD=$(openssl rand -base64 18 | tr -d '/+=' | head -c 24)
    EMQX_DASHBOARD_PASSWORD=$(openssl rand -base64 18 | tr -d '/+=' | head -c 24)

    cat > .env <<EOF
POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
REDIS_PASSWORD=${REDIS_PASSWORD}
JWT_SECRET=${JWT_SECRET}
MQTT_INTERNAL_PASSWORD=${MQTT_INTERNAL_PASSWORD}
ADMIN_PASSWORD=${ADMIN_PASSWORD}
EMQX_DASHBOARD_PASSWORD=${EMQX_DASHBOARD_PASSWORD}
EOF
    chmod 600 .env
    echo "✅ .env 已生成"
fi

# 加载密钥
set -a
# shellcheck disable=SC1091
. ./.env
set +a

# ---------- 3. 渲染配置 ----------
envsubst < config.prod.template.yaml > config.prod.yaml
chmod 600 config.prod.yaml
echo "✅ config.prod.yaml 已渲染"

# ---------- 4. 校验 ----------
if [ ! -s config.prod.yaml ]; then
    echo "❌ config.prod.yaml 为空，渲染失败"
    exit 1
fi
if grep -q '\${' config.prod.yaml; then
    echo "❌ config.prod.yaml 仍有未替换的占位符，检查 .env 是否完整"
    exit 1
fi

# ---------- 5. 输出摘要 ----------
echo ""
echo "=============================================="
echo "  KK物联云 生产部署就绪"
echo "=============================================="
echo "  管理员密码:  ${ADMIN_PASSWORD}"
echo "  EMQX 面板:   ${EMQX_DASHBOARD_PASSWORD}"
echo "  (admin 用户名固定为 admin)"
echo "=============================================="
echo ""
echo "下一步："
echo "  docker compose up -d --build"
echo "  docker compose logs -f server   # 观察后端启动日志"
echo "  浏览器访问: http://<服务器IP>"
