# Deployments environment

File: `deployments/.env` — copy từ `deployments/.env.example`. Dùng bởi Docker Compose.

## Biến

| Biến | Mô tả |
|------|--------|
| `POSTGRES_USER` | User Postgres container |
| `POSTGRES_PASSWORD` | Password — phải khớp `DB_DSN` trong `backend/.env` |
| `POSTGRES_DB` | Tên database |
| `MINIO_ROOT_USER` | MinIO admin user |
| `MINIO_ROOT_PASSWORD` | MinIO admin password — khớp `STORAGE_ACCESS_KEY` / `STORAGE_SECRET_KEY` |
| `STORAGE_BUCKET` | Bucket tạo bởi `minio-init` |

## Lệnh liên quan

```bash
make infra-up      # Bật Postgres, MinIO, Redis
make infra-down    # Tắt containers
make infra-reset   # Xóa volume + bật lại (mất dữ liệu)
```

Sau `infra-reset`: chạy `make migrate-up`.

## Đồng bộ với backend

Ví dụ dev local:

```env
# deployments/.env
POSTGRES_USER=mediahub
POSTGRES_PASSWORD=your_password
POSTGRES_DB=mediahub
MINIO_ROOT_USER=mediahub
MINIO_ROOT_PASSWORD=your_password
STORAGE_BUCKET=mediahub
```

```env
# backend/.env
DB_DSN=postgres://mediahub:your_password@localhost:15432/mediahub?sslmode=disable
REDIS_ADDR=localhost:16379
STORAGE_ACCESS_KEY=mediahub
STORAGE_SECRET_KEY=your_password
STORAGE_BUCKET=mediahub
```

> Host ports mặc định của Compose MediaHub: Postgres **15432**, Redis **16379** (tránh chiếm 5432/6379). Trong container vẫn là 5432/6379.