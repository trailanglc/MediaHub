# Bản đồ cấu hình

MediaHub dùng **ba file env** chính. Luôn đồng bộ credential giữa `deployments/.env` và `backend/.env`.

| File | Phạm vi |
|------|---------|
| [deployments/.env](deployments-env.md) | Postgres, MinIO, Redis (Docker Compose) |
| [backend/.env](backend-env.md) | API, JWT, storage driver, stream, governor, quota |
| [frontend/.env.local](frontend-env.md) | URL API, SEO, setup token UI |

## Biến quan trọng nhất

| Biến | File | Ảnh hưởng |
|------|------|-----------|
| `DB_DSN` | backend | Kết nối Postgres |
| `STORAGE_*` | backend | MinIO/S3 — upload, HLS, thumbnails |
| `REDIS_ADDR` | backend | Cache, queue convert, rate limit |
| `JWT_SECRET` | backend | Phiên đăng nhập dashboard |
| `API_PUBLIC_URL` | backend | Signed URL HLS, stream |
| `CDN_PUBLIC_URL` | backend | Signed URL `/assets`, `/embed` qua CDN |
| `NEXT_PUBLIC_API_URL` | frontend | Frontend gọi API |

## Production

- [Storage & CDN](storage-cdn.md) — nginx, signed URL, cache edge
- [Production tuning](production-tuning.md) — autoscale 90%, systemd, tách worker

## Dev nhanh

Sau khi copy `.env.example`, chỉ cần đảm bảo password Postgres/MinIO trong `deployments/.env` khớp `backend/.env` (`DB_DSN`, `STORAGE_ACCESS_KEY`, `STORAGE_SECRET_KEY`).
