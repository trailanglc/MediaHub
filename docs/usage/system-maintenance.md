# System & maintenance

## Settings (Owner)

**`/settings`** — workspace, media, streaming, storage, security, maintenance (`system_settings`).

API: `GET/PATCH /api/settings`

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
