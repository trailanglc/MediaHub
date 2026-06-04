# Cài đặt và chạy MediaHub

Hướng dẫn môi trường dev / self-host. Xem [README.md](../README.md) để biết tổng quan dự án.

## Yêu cầu

| Thành phần | Phiên bản |
|------------|-----------|
| Go | 1.22+ |
| Node.js + pnpm | Khuyến nghị LTS + pnpm 9+ |
| Docker & Docker Compose | Cho Postgres, MinIO, Redis |
| FFmpeg + FFprobe | Bắt buộc khi chạy `make worker` (chuyển mã HLS) |

## 1. Cấu hình môi trường

```bash
cp deployments/.env.example deployments/.env
cp backend/.env.example backend/.env
cp frontend/.env.example frontend/.env.local
```

Chỉnh `deployments/.env` và `backend/.env` cho khớp credential Postgres / MinIO / Redis.

## 2. Hạ tầng

```bash
make infra-up
```

Service `minio-init` tự tạo bucket (tên theo `STORAGE_BUCKET` trong `deployments/.env`).

Nếu đổi credential hoặc lỗi auth cũ:

```bash
make infra-reset
make migrate-up
```

## 3. Database

```bash
make migrate-up
```

## 4. Chạy ứng dụng

### Một lệnh (khuyến nghị)

```bash
make dev-full
```

Lệnh này: bật Docker (Postgres, MinIO, Redis) → migrate → chạy **một terminal** gồm API, scheduler, worker và frontend. Dừng bằng **Ctrl+C**.

Nếu infra đã chạy sẵn:

```bash
make dev-all
```

Tùy chọn bỏ worker (không cần FFmpeg / không dùng Videos):

```bash
SKIP_WORKER=1 make dev-all
```

### Nhiều terminal (tùy chọn)

| Terminal | Lệnh | Mô tả |
|----------|------|--------|
| 1 | `make api` | HTTP API — http://localhost:8080 |
| 2 | `make scheduler` | Job nền: upload hết hạn, xóa S3, purge thùng rác, … |
| 3 | `make worker` | Chuyển mã HLS (Asynq + FFmpeg) — bắt buộc khi dùng Videos |
| 4 | `make fe` | UI — http://localhost:3000 |

`make dev` chỉ bật infra + migrate; sau đó dùng `make dev-all` hoặc các lệnh riêng ở trên.

Biến môi trường video (xem `backend/.env.example`): `STREAM_SIGNING_SECRET`, `API_PUBLIC_URL` (URL API cho link HLS, mặc định `http://localhost:8080`), `CONVERT_MAX_CONCURRENT`, `CONVERT_JOB_TIMEOUT`, `STREAM_RATE_LIMIT_PER_MIN`.

## 5. Owner lần đầu

Khi chưa có user `owner`:

1. Mở http://localhost:3000 → **`/setup`**
2. Tạo email + mật khẩu (≥ 8 ký tự, có chữ và số)
3. Đăng nhập tại **`/login`**

API:

- `GET /api/setup/status`
- `POST /api/setup/owner`

Production (`APP_ENV=production`): bắt buộc `SETUP_TOKEN` và header `X-Setup-Token` (hoặc field `setup_token`).

## 6. Reset môi trường

**Factory reset** (xóa volume Docker, migrate lại):

```bash
RESET_CONFIRM=1 make reset
```

Sau đó tạo Owner mới tại `/setup`.

**Chỉ xóa dữ liệu ứng dụng** (giữ volume):

```bash
RESET_CONFIRM=1 make reset-data
```

Chỉ reset Docker: `make infra-reset` rồi `make migrate-up`.

## 7. Lệnh Make thường dùng

| Lệnh | Mô tả |
|------|--------|
| `make infra-up` / `infra-down` | Bật/tắt Compose |
| `make migrate-up` / `migrate-down` | Migration |
| `make api` | API server |
| `make scheduler` | Background scheduler |
| `make fe` | Frontend dev |
| `make orphan-cleanup` | Dry-run dọn blob MinIO mồ côi |
| `ORPHAN_CLEANUP_CONFIRM=1 make orphan-cleanup-apply` | Áp dụng dọn orphan |

## 8. Kiểm tra nhanh

- Health: `GET http://localhost:8080/health`
- System health (Owner, đã login): `GET /api/system/health`

Chi tiết vận hành, auth, File Manager, phân quyền: [USAGE.md](USAGE.md).

Kiến trúc production: [mediahub_production_spec.md](../mediahub_production_spec.md).
