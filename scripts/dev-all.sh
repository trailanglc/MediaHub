#!/usr/bin/env bash
# Chạy API + scheduler + worker + frontend trong một terminal.
# Ctrl+C dừng toàn bộ (API, scheduler, worker, Next.js).
#
# Yêu cầu: make infra-up && make migrate-up (hoặc make dev trước đó)
# Tùy chọn: SKIP_WORKER=1 ./scripts/dev-all.sh  — bỏ worker (không convert HLS)

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if [[ ! -f backend/.env ]] && [[ -f backend/.env.example ]]; then
  echo "Gợi ý: cp backend/.env.example backend/.env"
fi

# Job control: Ctrl+C tới cả job nền trong cùng terminal.
if [[ -t 0 ]]; then
  set -m
fi

pids=()
critical_pids=()
optional_pids=()
names=()
cleaned=0
fe_warned=0

FE_PORT="${FE_PORT:-3000}"
API_PORT="${API_PORT:-8080}"

kill_tree() {
  local sig=$1
  local pid=$2
  [[ -z "$pid" ]] && return 0
  kill "-$sig" -- "-$pid" 2>/dev/null || kill "-$sig" "$pid" 2>/dev/null || true
  pkill "-$sig" -P "$pid" 2>/dev/null || true
}

# go run biên dịch ra binary trong go-build/.../exe/* — parent "go run" chết nhưng binary vẫn sống.
kill_go_dev_binaries() {
  local sig=$1
  pkill "-$sig" -f 'mediahub-dev' 2>/dev/null || true
  pkill "-$sig" -f 'go-build.*/b001/exe/api' 2>/dev/null || true
  pkill "-$sig" -f 'go-build.*/b001/exe/scheduler' 2>/dev/null || true
  pkill "-$sig" -f 'go-build.*/b001/exe/worker' 2>/dev/null || true
  pkill "-$sig" -f 'Library/Caches/go-build.*/worker' 2>/dev/null || true
  pkill "-$sig" -f "${ROOT}/frontend.*next dev" 2>/dev/null || true
  pkill "-$sig" -f "${ROOT}/frontend.*next-server" 2>/dev/null || true
}

# FFmpeg convert dùng temp dir mediahub-convert-* — có thể sót khi worker bị kill đột ngột.
cleanup_ffmpeg_orphans() {
  local sig=$1
  if ! command -v pgrep >/dev/null 2>&1; then
    return 0
  fi
  local pids
  pids=$(pgrep -f 'mediahub-convert' 2>/dev/null || true)
  [[ -z "$pids" ]] && return 0
  echo "⚠ Dừng FFmpeg convert sót (mediahub-convert) PID: $(echo "$pids" | tr '\n' ' ')"
  while IFS= read -r pid; do
    [[ -n "$pid" ]] || continue
    kill "-$sig" "$pid" 2>/dev/null || true
  done <<<"$pids"
}

# Giải phóng cổng dev (Next/go còn sót sau lần chạy trước).
free_tcp_port() {
  local port=$1
  local label=$2
  if ! command -v lsof >/dev/null 2>&1; then
    return 0
  fi
  local pids
  pids=$(lsof -nP -iTCP:"$port" -sTCP:LISTEN -t 2>/dev/null || true)
  [[ -z "$pids" ]] && return 0
  echo "⚠ Cổng $port ($label) đang bận — dừng PID: $(echo "$pids" | tr '\n' ' ')"
  while IFS= read -r pid; do
    [[ -n "$pid" ]] || continue
    kill -TERM "$pid" 2>/dev/null || true
  done <<<"$pids"
  sleep 0.6
  pids=$(lsof -nP -iTCP:"$port" -sTCP:LISTEN -t 2>/dev/null || true)
  while IFS= read -r pid; do
    [[ -n "$pid" ]] || continue
    kill -KILL "$pid" 2>/dev/null || true
  done <<<"$pids"
}

prepare_next_dev() {
  free_tcp_port "$FE_PORT" "Next.js"
  rm -f "$ROOT/frontend/.next/dev/lock" 2>/dev/null || true
}

cleanup() {
  local code=${1:-0}
  [[ "$cleaned" -eq 1 ]] && return 0
  cleaned=1
  trap - INT TERM EXIT

  echo ""
  echo "Đang dừng các process..."

  local job_pid
  while IFS= read -r job_pid; do
    [[ -n "$job_pid" ]] || continue
    kill_tree TERM "$job_pid"
  done < <(jobs -p 2>/dev/null || true)

  for pid in "${pids[@]}"; do
    kill_tree TERM "$pid"
  done

  kill_go_dev_binaries TERM
  cleanup_ffmpeg_orphans TERM

  sleep 0.8

  while IFS= read -r job_pid; do
    [[ -n "$job_pid" ]] || continue
    kill_tree KILL "$job_pid"
  done < <(jobs -p 2>/dev/null || true)

  for pid in "${pids[@]}"; do
    kill_tree KILL "$pid"
  done

  kill_go_dev_binaries KILL
  cleanup_ffmpeg_orphans KILL

  wait 2>/dev/null || true
  exit "$code"
}

on_interrupt() {
  cleanup 130
}

trap on_interrupt INT TERM

# $1: 1 = critical (thoát → dừng cả stack), 0 = optional (chỉ cảnh báo)
run() {
  local critical=$1
  local name=$2
  shift 2
  echo "▶ $name"
  (
    "$@" 2>&1 | while IFS= read -r line; do
      printf '[%s] %s\n' "$name" "$line"
    done
  ) &
  local pid=$!
  pids+=("$pid")
  names+=("$name")
  if [[ "$critical" == "1" ]]; then
    critical_pids+=("$pid")
  else
    optional_pids+=("$pid")
  fi
}

API_URL="${API_URL:-http://localhost:${API_PORT}}"
HEALTH_URL="${API_URL%/}/health"

echo "MediaHub dev — Ctrl+C để dừng tất cả"
echo "  API:       $API_URL"
echo "  Frontend:  http://localhost:${FE_PORT}"
echo ""

free_tcp_port "$API_PORT" "API"
prepare_next_dev

run 1 api bash -c 'cd backend && exec go run ./cmd/api # mediahub-dev'

echo "Đợi API sẵn sàng (go run có thể mất ~30–60s lần đầu)..."
ready=0
for _ in $(seq 1 90); do
  if curl -sf "$HEALTH_URL" >/dev/null 2>&1; then
    ready=1
    echo "API OK ($HEALTH_URL)"
    break
  fi
  sleep 1
done
if [[ "$ready" -ne 1 ]]; then
  echo "Cảnh báo: API chưa phản hồi sau 90s — frontend vẫn khởi động; refresh trình duyệt khi API lên."
fi

run 1 scheduler bash -c 'cd backend && exec go run ./cmd/scheduler # mediahub-dev'

if [[ "${SKIP_WORKER:-}" != "1" ]]; then
  if ! command -v ffmpeg >/dev/null 2>&1; then
    echo "[worker] Cảnh báo: ffmpeg không có trong PATH — bỏ worker (hoặc cài ffmpeg)."
  else
    run 1 worker bash -c 'cd backend && exec go run ./cmd/worker # mediahub-dev'
  fi
else
  echo "[worker] Bỏ qua (SKIP_WORKER=1)"
fi

if command -v pnpm >/dev/null 2>&1; then
  run 0 fe bash -c "cd frontend && exec pnpm dev --port ${FE_PORT}"
else
  echo "[fe] Cần pnpm — chạy: cd frontend && npm run dev"
fi

echo ""
echo "Đã khởi động: ${names[*]}"
echo ""

while true; do
  for pid in "${critical_pids[@]}"; do
    if ! kill -0 "$pid" 2>/dev/null; then
      echo ""
      echo "Service nền tảng đã thoát — dừng các process còn lại."
      cleanup 1
    fi
  done

  if [[ ${#optional_pids[@]} -gt 0 ]]; then
    still=()
    for pid in "${optional_pids[@]}"; do
      if kill -0 "$pid" 2>/dev/null; then
        still+=("$pid")
      fi
    done
    opt_n=${#optional_pids[@]}
    still_n=${#still[@]}
    if [[ "$still_n" -lt "$opt_n" ]] && [[ "$fe_warned" -eq 0 ]]; then
      fe_warned=1
      echo ""
      echo "[fe] Frontend không chạy - API/scheduler/worker vẫn hoạt động."
      echo "      Chạy lại: make dev-all"
    fi
    optional_pids=("${still[@]}")
  fi

  sleep 1
done
