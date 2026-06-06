# System & maintenance

## Settings (Owner)

**`/settings`** — workspace, media, streaming, storage, security, maintenance (`system_settings`).

API: `GET/PATCH /api/settings`

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
| `POST /api/system/cleanup/orphans` | Quét orphan storage |

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
