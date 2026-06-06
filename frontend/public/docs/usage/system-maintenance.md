# System & maintenance

## Owner dashboard

**`/dashboard`** (Owner) — tóm tắt vận hành: nội dung, queue, storage, stream, sức khỏe dịch vụ. Một request API:

| API | Mô tả |
|-----|--------|
| `GET /api/system/overview` | Gom health, queue (tối đa 5 job running/failed), storage quota, stream analytics |

Chi tiết và thao tác (pause queue, cleanup, …) vẫn ở **`/system/health`**, **`/system/queue`**, **`/system/storage`**.

## Settings (Owner)

**`/settings`** — workspace, media, streaming, storage, security, maintenance (`system_settings`).

API: `GET/PATCH /api/settings`

## Storage quota (Owner)

**Settings → Storage → Quota storage (GB)** (`storage.quota_bytes`). Khi > 0, upload init/complete bị chặn nếu vượt dung lượng bucket + upload đang pending.

Dashboard: **`/system/storage`** — dung lượng MinIO/S3, quota workspace, dọn temp/orphan (dry-run trước khi xóa).

| API | Mô tả |
|-----|--------|
| `GET /api/system/storage` | Stats bucket, `quota_bytes`, `quota_remaining_bytes`, `stats_partial` |

## Queue (Owner)

Dashboard: **`/system/queue`** — độ sâu Asynq, job running/failed, pause/resume, hủy convert, backlog xóa S3.

| API | Mô tả |
|-----|--------|
| `GET /api/system/queue` | `queue_depth`, `queue_max_depth`, `queue_paused`, `running_jobs`, `storage_deletion_jobs` |
| `POST /api/system/queue/pause` | Tạm dừng enqueue convert mới |
| `POST /api/system/queue/resume` | Tiếp tục queue |
| `POST /api/videos/{id}/convert/cancel` | Hủy job pending/running (cooperative cancel khi FFmpeg đang chạy) |

Scheduler tự recovery job `running` quá hạn (`CONVERT_JOB_TIMEOUT` + 5 phút) → `failed`.

## Audit logs (Owner)

Dashboard: **`/system/audit-logs`** — xem lịch sử hành động (login, convert, phân quyền, settings, …).

| API | Mô tả |
|-----|--------|
| `GET /api/system/audit-logs` | Danh sách — `cursor`, `limit`, `actions` (csv), `action_prefix`, `actor_public_id`, `from`, `to` |
| `GET /api/system/audit-logs/actions` | Danh sách action distinct cho bộ lọc UI |

Thời gian giữ audit trong DB: **Settings → Maintenance → Giữ audit log (ngày)** (`maintenance.audit_retention_days`).

App logs (zap) ra stdout — cấu hình `LOG_LEVEL` trong [backend-env](../configuration/backend-env.md). Request `/stream/*`, `/assets/*` không log khi thành công (dùng `/metrics`).

## System cleanup (Owner)

| API | Mô tả |
|-----|--------|
| `POST /api/system/cleanup/temp` | Dọn file tạm |
| `POST /api/system/cleanup/orphans` | Quét (`dry_run: true`) hoặc xóa orphan storage |

CLI orphan: `make orphan-cleanup` / `ORPHAN_CLEANUP_CONFIRM=1 make orphan-cleanup-apply`

## Kiểm tra bảo mật

```bash
export TEST_OWNER_PASSWORD='...'
cd backend && go test ./internal/handler/... -run TestSecurity_ -v
```

```bash
export API_URL=http://localhost:8080 OWNER_EMAIL=... OWNER_PASSWORD=...
./scripts/security-smoke.sh
```

## E2E

```bash
cd frontend
pnpm exec playwright install chromium
E2E_EMAIL=owner@example.com E2E_PASSWORD=secret pnpm test:e2e
```

## Webhooks (Owner)

Cấu hình webhook dashboard — events gửi tới CMS: [Webhooks](../integration/05-webhooks.md)

## Production

[Production tuning](../configuration/production-tuning.md)
