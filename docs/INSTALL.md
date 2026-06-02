# Cài đặt và chạy MediaHub

Hướng dẫn môi trường dev / self-host. Xem [README.md](../README.md) để biết tổng quan dự án.

## Yêu cầu

| Thành phần | Phiên bản |
|------------|-----------|
| Go | 1.22+ |
| Node.js + pnpm | Khuyến nghị LTS + pnpm 9+ |
| Docker & Docker Compose | Cho Postgres, MinIO, Redis |
| FFmpeg | Chỉ khi bật worker convert video (tùy chọn, chưa bắt buộc) |

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

Cần **hai process** backend trong dev (API + scheduler):

| Terminal | Lệnh | Mô tả |
|----------|------|--------|
| 1 | `make api` | HTTP API — http://localhost:8080 |
| 2 | `make scheduler` | Job nền: upload hết hạn, xóa S3, purge thùng rác, … |
| 3 | `cd frontend && pnpm install && pnpm dev` | UI — http://localhost:3000 |

Hoặc sau `make dev` (infra + migrate), tự mở thêm `make api`, `make scheduler`, `make fe`.

`make worker` — queue video convert (asynq), **chưa cần** nếu chưa dùng Video Manager.

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
