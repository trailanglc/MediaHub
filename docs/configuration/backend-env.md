# Backend environment

File: `backend/.env` — copy từ `backend/.env.example`.

## Ứng dụng

| Biến | Mặc định | Mô tả |
|------|----------|--------|
| `APP_ENV` | `development` | `production` bật chế độ bảo mật nghiêm (Redis fail-closed, …) |
| `APP_URL` | `http://localhost:3000` | URL frontend — CORS, oEmbed allowlist |
| `API_PUBLIC_URL` | `http://localhost:8080` | Base URL signed link HLS, stream |
| `CDN_PUBLIC_URL` | *(trống)* | CDN cho `/assets`, `/embed` — xem [Storage & CDN](storage-cdn.md) |
| `API_ADDR` | `:8080` | Port lắng nghe API |

## Database & Redis

| Biến | Mô tả |
|------|--------|
| `DB_DSN` | Postgres connection string |
| `DB_MAX_CONNS` / `DB_MIN_CONNS` | Pool — autoscale khi bỏ trống |
| `REDIS_ADDR` | Redis host:port |
| `REDIS_POOL_SIZE` | Tuỳ chọn tune pool |

## Storage (S3/MinIO)

| Biến | Mô tả |
|------|--------|
| `STORAGE_DRIVER` | `s3` |
| `STORAGE_ENDPOINT` | `http://localhost:9000` (MinIO dev) |
| `STORAGE_BUCKET` | Tên bucket |
| `STORAGE_ACCESS_KEY` / `STORAGE_SECRET_KEY` | Credential MinIO |
| `STORAGE_USE_PATH_STYLE` | `true` cho MinIO local |

## Auth & setup

| Biến | Bắt buộc khi |
|------|--------------|
| `JWT_SECRET` | Mọi môi trường — `openssl rand -hex 32` |
| `SETUP_TOKEN` | Production — bảo vệ `POST /api/setup/owner` |

## Video / HLS / stream

| Biến | Mặc định | Mô tả |
|------|----------|--------|
| `FFMPEG_PATH` / `FFPROBE_PATH` | `/usr/bin/...` | Worker convert |
| `STREAM_SIGNING_SECRET` | — | Ký token stream, signed assets |
| `CONVERT_MAX_CONCURRENT` | autoscale | Trần job convert song song |
| `CONVERT_MIN_CONCURRENT` | 1 | Sàn khi governor scale xuống |
| `CONVERT_QUEUE_MAX_DEPTH` | MAX×25 | Hàng đợi Asynq |
| `CONVERT_JOB_TIMEOUT` | `2h` | Timeout mỗi job |
| `STREAM_RATE_LIMIT_PER_MIN` | 120 | Rate limit playlist `.m3u8` |
| `STREAM_SEGMENT_URL_TTL` | `1h` | TTL signed segment URL |
| `ASSET_DELIVERY_URL_TTL` | `24h` | TTL signed `/assets`, `/embed` |
| `STREAM_INTERNAL_REDIRECT_PREFIX` | *(trống)* | nginx X-Accel-Redirect cho segment |
| `PRESIGNED_PUT_URL_TTL` | `1h` | Direct upload integration API |
| `IMAGE_TRANSFORM_CACHE_TTL` | `24h` | Cache transform ảnh Redis |

## Resource governor

| Biến | Mặc định | Mô tả |
|------|----------|--------|
| `RESOURCE_GOVERNOR_ENABLED` | `true` | Điều tiết CPU/RAM/Redis runtime |
| `RESOURCE_SAMPLE_INTERVAL` | `10s` | Chu kỳ đo |
| `RESOURCE_CPU_RESERVE_PERCENT` | 10 | Giữ ≥10% CPU idle (max 90% dùng) |
| `RESOURCE_RAM_MIN_IDLE_PERCENT` | autoscale | Headroom RAM |
| `RESOURCE_REDIS_MAX_USED_PERCENT` | 85 | Rút TTL cache khi Redis đầy |

## Upload & Integration API quota

| Biến | Mặc định |
|------|----------|
| `UPLOAD_MAX_PENDING_PER_USER` | 10 |
| `API_KEY_UPLOAD_INIT_PER_MIN` | 120 |
| `API_KEY_CONVERT_PER_HOUR` | 60 |

## Proxy

| Biến | Mô tả |
|------|--------|
| `TRUSTED_PROXIES` | CIDR/IP reverse proxy cho ClientIP |

Xem thêm: [Production tuning](production-tuning.md)
