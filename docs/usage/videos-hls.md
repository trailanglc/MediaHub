# Videos & HLS

Route dashboard: **`/videos`**, **`/videos/[id]`**

Cần `make worker` (FFmpeg) và Redis. Upload video qua File Manager (type `video`).

## Convert

1. Mở Videos → chọn video → **Convert**
2. API: `POST /api/videos/{id}/convert` — body tuỳ chọn `{ "variants": ["1080p","720p","480p"] }`
3. Mặc định không upscale; bitrate scale theo nguồn

Retry khi failed: `POST /api/videos/{id}/convert/retry`

Hủy job đang chờ/chạy: `POST /api/videos/{id}/convert/cancel` (worker dừng cooperative tại checkpoint FFmpeg, không kill process ngay).

Owner có thể **pause/resume** queue tại **`/system/queue`** (`POST /api/system/queue/pause|resume`). Khi paused, convert mới trả `503 queue_paused`.

Job `running` quá `CONVERT_JOB_TIMEOUT` được scheduler đánh `failed` (stale recovery).

## Stream HLS

- **Token phiên**: `GET /api/videos/{id}/hls` → master URL có `?token=&exp=`; API set cookie `mh_stream`
- **Playlist** `.m3u8`: rate limit chỉ playlist; cache ~60s
- **Segment** `.ts`: bytes trực tiếp, `Range` 206, cookie phiên
- Player: `credentials: include` + `crossOrigin="use-credentials"`

**Domain allowlist:** host khớp chính xác. Allowlist rỗng = không embed ngoài qua allowlist. Subdomain: `.example.com`.

Stream policy: `GET/PATCH /api/videos/{id}/stream-policy`

## API keys cho embed ngoài

Owner → **`/api-keys`** — scope `stream`, header `X-API-Key` only. Chi tiết: [Integration API](../integration/README.md).

## Resource governor

Khi `RESOURCE_GOVERNOR_ENABLED=true`, worker/API điều tiết convert theo CPU/RAM/Redis:

- Giảm job song song và FFmpeg threads khi bận
- `503 system_busy` khi áp lực cao
- System Health hiển thị `resource_limits`

Biến: `RESOURCE_SAMPLE_INTERVAL`, `RESOURCE_CPU_RESERVE_PERCENT`, … — xem [Backend env](../configuration/backend-env.md).

## System monitoring (Owner)

- **`/system/storage`**, **`/system/queue`**
- API: `/api/system/storage`, `/api/system/queue`, `/api/system/stream-analytics`
- Health component: `GET /api/system/health/{database|redis|storage|worker|ffmpeg}`
- Prometheus: `GET /metrics` (firewall/nginx)

Production proxy: `TRUSTED_PROXIES` — `deployments/nginx-reverse-proxy.example.conf`
