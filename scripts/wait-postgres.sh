#!/usr/bin/env bash
# Chờ Postgres trong Docker Compose sẵn sàng nhận kết nối.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
ENV_FILE="${ROOT}/deployments/.env"
COMPOSE_FILE="${ROOT}/deployments/docker-compose.yml"
TIMEOUT="${WAIT_TIMEOUT:-90}"

if [[ ! -f "$ENV_FILE" ]]; then
  echo "Thiếu $ENV_FILE — chạy: cp deployments/.env.example deployments/.env" >&2
  exit 1
fi

# shellcheck disable=SC1090
source "$ENV_FILE"

CONTAINER="${POSTGRES_CONTAINER:-mediahub-postgres}"
elapsed=0

echo "Đợi Postgres ($CONTAINER)..."
until docker exec "$CONTAINER" pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB" -q 2>/dev/null; do
  if (( elapsed >= TIMEOUT )); then
    echo "Timeout sau ${TIMEOUT}s — kiểm tra: docker logs $CONTAINER" >&2
    exit 1
  fi
  sleep 2
  elapsed=$((elapsed + 2))
done

echo "Postgres sẵn sàng."
