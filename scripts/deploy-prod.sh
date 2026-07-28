#!/usr/bin/env bash
# Build từ repo source → đồng bộ runtime riêng (không chạy production trong Projects/).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DEPLOY_ROOT="${MEDIAHUB_DEPLOY_ROOT:-$HOME/mediahub}"
export PATH="/usr/local/go/bin:${HOME}/.nvm/versions/node/v24.18.0/bin:${PATH}"

echo "==> Deploy root: ${DEPLOY_ROOT}"
mkdir -p "${DEPLOY_ROOT}/backend" "${DEPLOY_ROOT}/frontend" "${DEPLOY_ROOT}/logs"

echo "==> Build backend gateway"
mkdir -p "${ROOT}/backend/bin"
(
  cd "${ROOT}/backend"
  CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/mediahub ./cmd/gateway
)
install -m 0755 "${ROOT}/backend/bin/mediahub" "${DEPLOY_ROOT}/backend/mediahub"
install -m 0600 "${ROOT}/backend/.env" "${DEPLOY_ROOT}/backend/.env"

echo "==> Sync docs + build frontend (standalone)"
(
  cd "${ROOT}"
  make docs-sync
)
(
  cd "${ROOT}/frontend"
  pnpm install --frozen-lockfile
  pnpm build
)

STANDALONE="${ROOT}/frontend/.next/standalone"
if [[ ! -d "${STANDALONE}" ]]; then
  echo "ERROR: thiếu ${STANDALONE} — kiểm tra next.config output: 'standalone'" >&2
  exit 1
fi

# Next monorepo/workspace có thể đặt server.js trong standalone/frontend/
if [[ -f "${STANDALONE}/server.js" ]]; then
  FE_SRC="${STANDALONE}"
elif [[ -f "${STANDALONE}/frontend/server.js" ]]; then
  FE_SRC="${STANDALONE}/frontend"
else
  echo "ERROR: không tìm thấy server.js trong standalone output" >&2
  find "${STANDALONE}" -maxdepth 3 -name server.js -print >&2 || true
  exit 1
fi

echo "==> Sync frontend runtime từ ${FE_SRC}"
rsync -a --delete \
  --exclude='.env*' \
  "${FE_SRC}/" "${DEPLOY_ROOT}/frontend/"
mkdir -p "${DEPLOY_ROOT}/frontend/.next" "${DEPLOY_ROOT}/frontend/public"
rsync -a --delete "${ROOT}/frontend/.next/static/" "${DEPLOY_ROOT}/frontend/.next/static/"
rsync -a --delete "${ROOT}/frontend/public/" "${DEPLOY_ROOT}/frontend/public/"
install -m 0600 "${ROOT}/frontend/.env.local" "${DEPLOY_ROOT}/frontend/.env.local"

echo "==> Write PM2 ecosystem"
cat > "${DEPLOY_ROOT}/ecosystem.config.cjs" <<EOF
/**
 * MediaHub production — runtime tại ${DEPLOY_ROOT}
 * Source code: ${ROOT}
 *
 *   pm2 start ${DEPLOY_ROOT}/ecosystem.config.cjs
 *   pm2 save
 */
module.exports = {
  apps: [
    {
      name: "mediahub-gateway",
      cwd: "${DEPLOY_ROOT}/backend",
      script: "${DEPLOY_ROOT}/backend/mediahub",
      interpreter: "none",
      instances: 1,
      exec_mode: "fork",
      autorestart: true,
      watch: false,
      max_memory_restart: "2G",
      kill_timeout: 15000,
      error_file: "${DEPLOY_ROOT}/logs/gateway-error.log",
      out_file: "${DEPLOY_ROOT}/logs/gateway-out.log",
      env: { NODE_ENV: "production" },
    },
    {
      name: "mediahub-frontend",
      cwd: "${DEPLOY_ROOT}/frontend",
      script: "server.js",
      instances: 1,
      exec_mode: "fork",
      autorestart: true,
      watch: false,
      max_memory_restart: "1G",
      error_file: "${DEPLOY_ROOT}/logs/frontend-error.log",
      out_file: "${DEPLOY_ROOT}/logs/frontend-out.log",
      env: {
        NODE_ENV: "production",
        PORT: "3000",
        HOSTNAME: "0.0.0.0",
      },
    },
  ],
};
EOF

echo "==> Done. Binary + standalone đã vào ${DEPLOY_ROOT}"
echo "    Restart: pm2 startOrReload ${DEPLOY_ROOT}/ecosystem.config.cjs && pm2 save"
