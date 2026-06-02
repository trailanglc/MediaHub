# Hướng dẫn sử dụng MediaHub

Tài liệu vận hành và tính năng chính. Cài đặt: [INSTALL.md](INSTALL.md).

## Đăng nhập và phiên

- Cookie HttpOnly `access_token` + `refresh_token`
- `POST /api/auth/login`, `/refresh`, `/logout`, `GET /api/auth/me`
- Production: `JWT_SECRET`, mã hóa mật khẩu RSA (`GET /api/auth/crypto/public-key`)

Biến môi trường: xem `backend/.env.example`.

## Members và phân quyền

Owner: **`/members`**, **`/permissions`**.

- Role: `manager`, `viewer` (owner toàn quyền)
- Quyền resource: `read`, `upload`, `update`, `delete`, `manage`, …
- Chia sẻ folder con không bắt buộc share cả cây cha; member chỉ thấy nhánh được cấp
- Folder Root seed: `00000000-0000-4000-8000-000000000001`
- Member: `GET /api/permissions/mine`, Dashboard riêng

## File Manager (`/files`)

Cần migration `000006` trở lên. Chạy `make scheduler` để xóa S3 / purge thùng rác tự động.

Tính năng chính: upload multipart S3, folder tree, thùng rác, tìm kiếm, bulk rename, preview, share quyền.

API chính: `GET/POST /api/objects`, `/api/folders`, `/api/upload/*`, trash / restore / purge.

## Settings (Owner)

**`/settings`** — workspace, media, streaming, storage, security, maintenance (`system_settings`).

## Dashboard

- **Owner**: tổng quan + System Health
- **Member**: thư mục được chia sẻ + liên kết File Manager

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
