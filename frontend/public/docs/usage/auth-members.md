# Auth & members

## Đăng nhập và phiên

- Cookie HttpOnly `access_token` + `refresh_token`
- `POST /api/auth/login`, `/refresh`, `/logout`, `GET /api/auth/me`
- Production: `JWT_SECRET`, mã hóa mật khẩu RSA (`GET /api/auth/crypto/public-key`)

Biến môi trường: [Backend env](../configuration/backend-env.md).

## Redis (cache auth)

| Lớp cache | TTL | Invalidate khi |
|-----------|-----|----------------|
| User auth (`cache:auth:user:*`) | 60s | Đổi role/status member |
| Settings (`cache:settings:v1`) | 30s | Owner `PATCH /api/settings` |
| Stream metadata (`cache:stream:video:*`) | 45s | Đổi stream policy, convert, xóa HLS |

Tune: `REDIS_POOL_SIZE`, `REDIS_MIN_IDLE_CONNS` trong `backend/.env`.

## Members và phân quyền

Owner: **`/members`**, **`/permissions`**.

- Role: `manager`, `viewer` (owner toàn quyền)
- Quyền resource: `read`, `upload`, `update`, `delete`, `manage`, …
- Chia sẻ folder con — member chỉ thấy nhánh được cấp
- Folder Root seed: `00000000-0000-4000-8000-000000000001`
- Member: `GET /api/permissions/mine`, Dashboard riêng

API chính:

| Method | Path |
|--------|------|
| GET/POST | `/api/members` |
| GET/PATCH/DELETE | `/api/members/{public_id}` |
| GET/POST | `/api/permissions` |
| DELETE | `/api/permissions/{id}` |
