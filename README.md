# MediaHub

Nền tảng quản lý media và streaming HLS tự host. Monorepo tách **backend** (Go + Gin) và **frontend** (Next.js).

## Yêu cầu

- Go 1.22+
- pnpm
- Docker & Docker Compose
- FFmpeg (khi triển khai worker convert — tùy chọn cho skeleton)

## Khởi động nhanh

### 1. Cấu hình môi trường

```bash
cp deployments/.env.example deployments/.env
cp backend/.env.example backend/.env
cp frontend/.env.example frontend/.env.local
```

Chỉnh `deployments/.env` và `backend/.env` cho khớp credential Postgres / MinIO.

### 2. Hạ tầng dev

```bash
make infra-up
```

Service `minio-init` tự tạo bucket `mediahub` (hoặc tên trong `STORAGE_BUCKET` ở `deployments/.env`).

Nếu đổi credential hoặc gặp lỗi auth cũ, reset stack:

```bash
make infra-reset
make migrate-up
```

### 3. Migration database

```bash
make migrate-up
```

### 4. Chạy ứng dụng

Terminal 1 — API:

```bash
make api
```

Terminal 2 — Worker (tùy chọn):

```bash
make worker
```

Terminal 3 — Frontend:

```bash
cd frontend && pnpm install && pnpm dev
```

- API: http://localhost:8080
- Health (Owner, đã đăng nhập): http://localhost:8080/api/system/health
- UI: http://localhost:3000

## Thiết lập Owner lần đầu

Khi chưa có user `owner` trong database:

1. Mở http://localhost:3000 → tự chuyển **`/setup`**
2. Tạo email + mật khẩu (tối thiểu 8 ký tự, có chữ và số)
3. Sau khi tạo xong → **`/login`**

API:

- `GET /api/setup/status` — `{ setup_required, setup_allowed }`
- `POST /api/setup/owner` — body `{ email, password, setup_token? }`

Production (`APP_ENV=production`): bắt buộc `SETUP_TOKEN` trong `backend/.env` và gửi header `X-Setup-Token` (hoặc field `setup_token`).

### Reset hệ thống về trạng thái mới

**Factory reset** (xóa volume Docker Postgres / MinIO / Redis, chạy lại migration):

```bash
RESET_CONFIRM=1 make reset
```

Sau đó mở http://localhost:3000/setup để tạo Owner mới.

**Chỉ xóa dữ liệu ứng dụng** (giữ volume, nhanh hơn khi dev):

```bash
RESET_CONFIRM=1 make reset-data
```

Lệnh `reset-data` truncate bảng Postgres, seed lại folder Root, `FLUSHALL` Redis và xóa object trong bucket MinIO.

Chỉ reset hạ tầng (không migrate): `make infra-reset` rồi `make migrate-up`.

## Kiểm tra bảo mật Auth / phân quyền

### Test tự động (Go)

Cần Postgres + Redis đang chạy (`make infra-up`) và owner đã tạo. Đặt mật khẩu owner:

```bash
export TEST_OWNER_PASSWORD='mật-khẩu-owner-của-bạn'
# tùy chọn: export TEST_OWNER_EMAIL=owner@example.com
cd backend && go test ./internal/handler/... -run TestSecurity_ -v
```

Các case gồm: route không token (401), JWT giả / claim role giả (403), phân quyền owner vs viewer, refresh reuse, rate limit login, CORS, vô hiệu session sau đổi role.

### Smoke script (API đang chạy)

```bash
export API_URL=http://localhost:8080
export OWNER_EMAIL=owner@example.com
export OWNER_PASSWORD='mật-khẩu'
./scripts/security-smoke.sh
```

### Production / staging

- `APP_ENV=production` hoặc `staging`: bắt buộc `JWT_SECRET` và `SETUP_TOKEN` (fail fast khi khởi động).
- `/api/system/*` chỉ Owner (middleware `RequireOwner`).
- Đổi role/status member → revoke refresh + vô hiệu access token đang phát hành (Redis).

## Đăng nhập và phiên làm việc

Sau khi có Owner:

1. Mở **`/login`**, nhập email/mật khẩu
2. API trả cookie HttpOnly `access_token` + `refresh_token` (frontend gọi API với `credentials: include`)
3. Các route `/dashboard`, `/files`, … yêu cầu đăng nhập (`GET /api/auth/me`)

API auth:

- `POST /api/auth/login` — `{ email, password }`
- `POST /api/auth/refresh` — rotate refresh token
- `POST /api/auth/logout` — thu hồi phiên
- `GET /api/auth/me` — thông tin user hiện tại

Biến môi trường backend (xem `backend/.env.example`):

- `JWT_SECRET` — bắt buộc khi `APP_ENV=production`
- `JWT_REFRESH_TTL` — tùy chọn; mặc định trong code `72h` (3 ngày). Access token cố định `15m`.
- `LOGIN_MAX_ATTEMPTS` / `LOGIN_LOCKOUT_WINDOW` — giới hạn đăng nhập sai
- `LOGIN_RSA_PRIVATE_KEY` — PEM khóa RSA (tùy chọn; không set thì API tự sinh khi khởi động)
- `REQUIRE_ENCRYPTED_PASSWORD` — `true` để bắt buộc `encrypted_password` (mặc định bật khi `APP_ENV=production`)

**Mã hóa mật khẩu khi gửi:** Frontend lấy `GET /api/auth/crypto/public-key`, mã hóa mật khẩu RSA-OAEP (SHA-256), gửi field `encrypted_password` (không gửi plaintext). Vẫn nên dùng HTTPS trong production.

**Quan trọng:** Sau khi cập nhật code backend, cần **khởi động lại API** (`make api`). Process cũ sẽ trả `not_implemented` cho `/api/auth/login`.

## Members và phân quyền (Owner)

Owner quản lý tại **`/members`** và **`/permissions`**:

- Tạo member (`manager` / `viewer`)
- Gán quyền resource (`read`, `upload`, …) trên folder/file
- Folder **Root** seed sẵn: `00000000-0000-4000-8000-000000000001` (migration `000004`)

API (yêu cầu cookie Owner):

- `GET/POST/PATCH/DELETE /api/members`
- `GET/POST/DELETE /api/permissions?resource_id={uuid}`

## File Manager (`/files`)

**Bắt buộc** migration `000006`–`000010` trước khi upload — nếu thiếu, API trả `upload failed` / 500 trên `/api/upload/init`:

```bash
make migrate-up
# Khởi động lại API sau migrate
make api
```

Session upload **cũ** (trước `000007`) cần **hủy và upload lại**.

Owner (và member có quyền) dùng **File Manager** để:

- Duyệt cây folder, breadcrumb, xem dạng bảng / lưới
- Upload: mỗi chunk là **S3 UploadPart** thẳng vào `originals/` / `documents/` (API không gộp file trong RAM)
- Tạo folder, đổi tên (đơn + hàng loạt tối đa 20), di chuyển, xóa mềm, thùng rác + khôi phục, tìm toàn workspace
- Xem trước ảnh/file (presigned URL), gán quyền member trên folder (nút Share)

API (cookie đăng nhập):

| Method | Path | Mô tả |
|--------|------|--------|
| GET | `/api/objects?parent_id=&type=&q=&cursor=&limit=` | Liệt kê con của folder |
| POST | `/api/folders` | Tạo folder `{ parent_id, name }` |
| GET | `/api/objects/trash?cursor=&limit=` | Thùng rác (đã xóa mềm) |
| GET | `/api/objects/search?q=&type=&cursor=&limit=` | Tìm theo tên toàn workspace (`q` bắt buộc) |
| POST | `/api/objects/{public_id}/restore` | Khôi phục từ thùng rác (folder → cả cây con, trả `{ restored_count, object }`) |
| DELETE | `/api/objects/{public_id}/purge` | Xóa vĩnh viễn khỏi DB + enqueue S3 (chỉ mục đã xóa mềm, trả `{ purged_count }`) |
| GET/PATCH/DELETE | `/api/objects/{public_id}` | Chi tiết / đổi tên / move / xóa (DELETE → `{ deleted_count }`) |
| POST | `/api/objects/preview-urls` | Presign batch `{ ids: [uuid…] }` → `urls` (grid/preview, tối đa 24) |
| POST | `/api/upload/init` | Bắt đầu session upload |
| PUT | `/api/upload/{session_id}/chunks/{index}` | Gửi chunk (body raw, `Content-Length` bắt buộc) |
| POST | `/api/upload/{session_id}/complete` | `CompleteMultipartUpload` + tạo `media_objects` |
| DELETE | `/api/upload/{session_id}` | Hủy session + `AbortMultipartUpload` |

Nền API (tự động):

| Chu kỳ | Việc làm |
|--------|----------|
| 15 phút | Hủy session upload hết hạn (`AbortMultipart`) |
| 30 giây + ngay sau xóa | Xóa S3 sau **xóa file**: enqueue DB → goroutine xóa key đơn (ảnh/PDF, mọi size); worker batch xóa prefix HLS (`storage_deletion_jobs`) |
| 24 giờ | Dọn object `temp/` cũ; auto-purge thùng rác quá `maintenance.trash_retention_days` (mặc định 30, 0 = tắt); sửa `object_paths` trùng |

List folder: index `parent_id` + **pg_trgm** trên `name`; quyền UI tính **batch** (2 query/list, không 6×N).

Ảnh upload: sinh thumbnail JPEG nền (`thumbnails/{public_id}.jpg`). Giới hạn upload: `UPLOAD_INIT_PER_MINUTE` (mặc định 60), `UPLOAD_MAX_PENDING_PER_USER` (10). Metrics upload trên `GET /api/system/health`.

Upload complete ghi **SHA-256** vào `media_objects.checksum` (đọc stream từ S3, RAM cố định).

Giới hạn upload: **Settings → Media** (`max_upload_bytes`, mặc định **5 GB**, Owner tự chỉnh). Mọi user upload đọc giới hạn qua `GET /api/upload/limits`; dialog chọn file chặn theo giới hạn này. Folder mặc định khi mở `/files` lấy từ `default_root_folder_public_id`.

**Xóa folder:** xóa cả cây con (soft-delete); API trả `{ "deleted_count": N }`.

**Băng thông host thấp:** hạ `UPLOAD_MAX_PENDING_PER_USER` (vd. `2`–`3`); upload chunk **tuần tự**; cân nhắc nginx `limit_rate` — xem `deploy/nginx-upload-rate-limit.example.conf`.

### Kiểm thử nhanh

1. Owner: `/files` → Upload ảnh + PDF → tạo subfolder → move file
2. Member `viewer` có `read` trên subfolder: thấy list, không có nút Upload
3. Upload file > `max_upload_bytes` → HTTP 413
4. `cd backend && go test ./internal/mediautil/... ./internal/repository/... -count=1` (repository cần Postgres)
5. E2E File Manager (API + frontend đang chạy, hoặc để Playwright tự `pnpm dev`):

```bash
cd frontend
pnpm exec playwright install chromium
E2E_EMAIL=owner@example.com E2E_PASSWORD=secret pnpm test:e2e
```

## Settings (Owner)

Trang **`/settings`** là trung tâm cấu hình runtime (lưu bảng `system_settings`, migration `000005`):

| Tab | Có thể chỉnh qua UI | Chỉ đọc / `.env` |
|-----|---------------------|------------------|
| Chung | Tên workspace, public URL, root folder mặc định (Permissions) | — |
| Media | Kích thước upload tối đa | — |
| Streaming | TTL token, domain allowlist global | — |
| Storage | Quota (GB) | Endpoint, bucket, driver |
| Bảo mật | Số lần login sai, thời gian khóa | JWT, encrypted password (production) |
| Bảo trì | Giữ audit log (ngày), giữ thùng rác trước auto-purge (ngày, 0 = tắt) | — |

API:

- `GET /api/settings` — `editable` + `readonly`
- `PATCH /api/settings` — body JSON partial (`workspace`, `media`, `security`, …)

Route cũ **`/security`** redirect tới `/settings?tab=security`.

Giá trị trong `.env` (`DB_DSN`, `STORAGE_SECRET_KEY`, `JWT_SECRET`, …) vẫn là nguồn hạ tầng; UI chỉ hiển thị trạng thái, không ghi đè secret.

Sau khi pull code có migration `000005`, chạy `make migrate-up` rồi **khởi động lại API** (`make api`). Nếu thấy `failed to load settings`, thường là bảng `system_settings` chưa có — chạy migrate như trên.

## Cấu trúc

```txt
backend/     Go API + worker + migrations
frontend/    Next.js dashboard
deployments/ Docker Compose (Postgres, MinIO, Redis)
```

Chi tiết kiến trúc: [mediahub_production_spec.md](mediahub_production_spec.md).
