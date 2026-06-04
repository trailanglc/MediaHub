#!/usr/bin/env bash
# Dừng stack dev sót sau Ctrl+C (go-build binary, go run, next dev).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

kill_sig() {
  local sig=$1
  pkill "-$sig" -f "${ROOT}/scripts/dev-all.sh" 2>/dev/null || true
  pkill "-$sig" -f 'mediahub-dev' 2>/dev/null || true
  pkill "-$sig" -f 'go-build.*/b001/exe/api' 2>/dev/null || true
  pkill "-$sig" -f 'go-build.*/b001/exe/scheduler' 2>/dev/null || true
  pkill "-$sig" -f 'go-build.*/b001/exe/worker' 2>/dev/null || true
  pkill "-$sig" -f 'Library/Caches/go-build.*/worker' 2>/dev/null || true
  pkill "-$sig" -f "${ROOT}/frontend.*next dev" 2>/dev/null || true
  pkill "-$sig" -f "${ROOT}/frontend.*next-server" 2>/dev/null || true
  if command -v pgrep >/dev/null 2>&1; then
    local pids
    pids=$(pgrep -f 'mediahub-convert' 2>/dev/null || true)
    while IFS= read -r pid; do
      [[ -n "$pid" ]] || continue
      kill "-$sig" "$pid" 2>/dev/null || true
    done <<<"$pids"
  fi
}

echo "Đang dừng process dev sót..."
kill_sig TERM
sleep 0.6
kill_sig KILL
echo "Xong."
