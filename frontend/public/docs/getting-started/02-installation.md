# Cài đặt

Hướng dẫn môi trường dev / self-host. Xem [Tổng quan](01-overview.md) trước khi bắt đầu.

## 1. Cấu hình môi trường

```bash
cp deployments/.env.example deployments/.env
cp backend/.env.example backend/.env
cp frontend/.env.example frontend/.env.local
```

Chỉnh `deployments/.env` và `backend/.env` cho khớp credential Postgres / MinIO / Redis. Chi tiết từng biến: [Configuration](../configuration/README.md).

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

### Khởi động dev (khuyến nghị)

**Terminal 1 — hạ tầng + backend:**

```bash
make dev-full     # infra + migrate + gateway (API + scheduler + worker)
```

Hoặc nếu infra đã chạy sẵn:

```bash
make gateway
```

`cmd/gateway` kiểm tra Postgres/Redis/MinIO + schema, rồi khởi động API + scheduler + worker trong một process. Dừng bằng **Ctrl+C**.

Tùy chọn bỏ worker (không cần FFmpeg / không dùng Videos):

```bash
SKIP_WORKER=1 make gateway
```

**Terminal 2 — frontend:**

```bash
make fe           # UI — http://localhost:3000
```

### Từng service riêng (tùy chọn)

| Terminal | Lệnh | Mô tả |
|----------|------|--------|
| 1 | `make api` | HTTP API — http://localhost:8080 |
| 2 | `make scheduler` | Job nền: upload hết hạn, xóa S3, purge thùng rác, … |
| 3 | `make worker` | Chuyển mã HLS (Asynq + FFmpeg) — bắt buộc khi dùng Videos |
| 4 | `make fe` | UI — http://localhost:3000 |

`make dev` chỉ bật infra + migrate; sau đó dùng `make gateway` + `make fe` hoặc các lệnh riêng ở trên.

Biến môi trường video: xem [Backend env](../configuration/backend-env.md).

## 5. Reset môi trường

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

## 6. Lệnh Make thường dùng

| Lệnh | Mô tả |
|------|--------|
| `make infra-up` / `infra-down` | Bật/tắt Compose |
| `make migrate-up` / `migrate-down` | Migration |
| `make gateway` | API + scheduler + worker |
| `make api` | API server riêng |
| `make scheduler` | Background scheduler |
| `make worker` | Worker convert HLS |
| `make fe` | Frontend dev |
| `make docs-sync` | Đồng bộ tài liệu sang frontend |
| `make orphan-cleanup` | Dry-run dọn blob MinIO mồ côi |
| `ORPHAN_CLEANUP_CONFIRM=1 make orphan-cleanup-apply` | Áp dụng dọn orphan |

## 7. Kiểm tra nhanh

- Health: `GET http://localhost:8080/health`
- System health (Owner, đã login): `GET /api/system/health`

**Tiếp theo:** [Chạy lần đầu — tạo Owner](03-first-run.md)
