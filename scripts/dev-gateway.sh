#!/usr/bin/env bash
# Wrapper dev: chạy gateway Go (kiểm tra môi trường + API + scheduler + worker).
# Frontend chạy riêng: make fe
# Ctrl+C dừng toàn bộ backend.
#
# Yêu cầu: make infra-up && make migrate-up (hoặc make dev trước đó)
# Tùy chọn: SKIP_WORKER=1 make dev-gateway

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT/backend"

if [[ ! -f .env ]] && [[ -f .env.example ]]; then
  echo "Gợi ý: cp .env.example .env"
fi

echo "MediaHub gateway — Ctrl+C để dừng"
echo "  Frontend: make fe (terminal khác)"
echo ""

exec go run ./cmd/gateway
