# Tổng quan hệ thống

MediaHub gồm **backend** (Go) và **frontend** (Next.js), chạy cùng hạ tầng Postgres, Redis và MinIO (S3-compatible).

## Thành phần

| Thành phần | Vai trò |
|------------|---------|
| `cmd/gateway` | Entry point: kiểm tra môi trường, chạy API + scheduler + worker |
| `cmd/api` | HTTP API riêng (debug) |
| `cmd/scheduler` | Job nền: upload hết hạn, xóa S3, purge thùng rác |
| `cmd/worker` | Hàng đợi convert HLS (Asynq + FFmpeg) |
| Frontend | Dashboard Owner/Member tại port 3000 |
| MinIO | Blob storage (originals, HLS segments, thumbnails) |
| Redis | Cache, queue Asynq, rate limit |
| Postgres | Metadata, users, permissions, settings |

## Yêu cầu

| Thành phần | Phiên bản |
|------------|-----------|
| Go | 1.22+ |
| Node.js + pnpm | Khuyến nghị LTS + pnpm 9+ |
| Docker & Docker Compose | Postgres, MinIO, Redis |
| FFmpeg + FFprobe | Bắt buộc khi chạy worker (convert HLS) |

## Hai cách dùng

1. **Dashboard** — Owner/Member đăng nhập qua browser (`/login`), quản lý file, video, phân quyền.
2. **Integration API** — CMS/website bên ngoài gọi `/api/v1` với header `X-API-Key`.

## Luồng khởi động dev (tóm tắt)

```bash
cp deployments/.env.example deployments/.env
cp backend/.env.example backend/.env
cp frontend/.env.example frontend/.env.local

make dev-full    # infra + migrate + gateway
make fe          # frontend — terminal khác
```

Mở http://localhost:3000/setup để tạo Owner.

**Tiếp theo:** [Cài đặt chi tiết](02-installation.md)
