# Hướng dẫn sử dụng MediaHub

Tài liệu vận hành và tính năng chính. Cài đặt: [INSTALL.md](INSTALL.md).

## Dev stack — dừng an toàn

Ưu tiên **`make dev-all`** (một terminal): **Ctrl+C** dừng API, scheduler, worker và Next.js; script giải phóng cổng và process sót (`go run`, `next dev`, FFmpeg nếu còn).

Nếu chạy riêng `make api` / `make scheduler` / `make worker` / `make fe`, cần **Ctrl+C từng terminal**. Sau khi tắt worker giữa lúc convert, kiểm tra `ps aux | grep ffmpeg` nếu nghi process sót.

## Đăng nhập và phiên

- Cookie HttpOnly `access_token` + `refresh_token`
- `POST /api/auth/login`, `/refresh`, `/logout`, `GET /api/auth/me`
- Production: `JWT_SECRET`, mã hóa mật khẩu RSA (`GET /api/auth/crypto/public-key`)

Biến môi trường: xem `backend/.env.example`.

## Redis (cache, queue, rate limit)

Docker Redis (`make infra-up`) dùng **maxmemory 512MB** và **allkeys-lru** — key cache có TTL tự hết; queue Asynq và session vẫn lưu trên Redis.

| Lớp cache | TTL | Invalidate khi |
|-----------|-----|----------------|
| User auth (`cache:auth:user:*`) | 60s | Đổi role/status member, disable/restore/purge |
| Settings (`cache:settings:v1`) | 30s | Owner `PATCH /api/settings` |
| Stream metadata (`cache:stream:video:*`) | 45s | Đổi stream policy, convert HLS, xóa HLS |

**Stream HLS (phiên bản mới):**

- **Token phiên** (HMAC `videoID|exp`): lần vào qua `GET /api/videos/{id}/hls` → URL master có `?token=&exp=`; sau đó API set cookie `mh_stream` (path `/stream/{videoID}`).
- **Playlist** `.m3u8`: URL tương đối trong playlist (không ký từng segment), cache memory ~60s; rate limit **chỉ playlist**.
- **Segment** `.ts`: API trả thẳng bytes (**1 request**, hỗ trợ `Range` 206), `Cache-Control: immutable`; xác thực bằng cookie phiên (hoặc query token lần đầu).
- Player/dashboard: `credentials: include` + `crossOrigin="use-credentials"` (xem `hls-player.tsx`).
- Redis lỗi (`APP_ENV=production|staging`): từ chối rate limit stream (503) và login fail-closed.

**Domain allowlist:** so khớp **host** chính xác (không dùng substring). Allowlist rỗng = không cho embed ngoài qua allowlist (vẫn xem được trong app qua `APP_URL` hoặc cookie đã đăng nhập). Subdomain: thêm mục `.example.com`.

Tune client (tùy chọn): `REDIS_POOL_SIZE`, `REDIS_MIN_IDLE_CONNS`, `REDIS_READ_TIMEOUT`, `REDIS_WRITE_TIMEOUT` trong `backend/.env`.

## Điều tiết tài nguyên (Resource Governor)

Khi `RESOURCE_GOVERNOR_ENABLED=true` (mặc định), worker đo % nhàn rỗi CPU/RAM/Redis và publish snapshot `system:resource:v1` lên Redis. API/scheduler đọc snapshot để:

- Giảm **số job convert song song** và **`-threads` FFmpeg** khi hệ thống bận (`CONVERT_MAX_CONCURRENT` là trần, `CONVERT_MIN_CONCURRENT` là sàn).
- Hoãn job convert mới nếu RAM idle dưới `RESOURCE_RAM_MIN_IDLE_PERCENT` (retry Asynq).
- Từ chối enqueue convert mới (`503 system_busy`) khi áp lực ≥ ~85%.
- Rút ngắn TTL cache Redis và có thể bỏ ghi cache stream khi Redis gần đầy.
- Giảm batch xóa S3 của scheduler khi tải cao.
- Giảm **xóa S3 ngay trên API** (immediate delete, tối đa 8 → tối thiểu 2 slot) khi tải cao.
- Khi Redis dùng ≥ `RESOURCE_REDIS_MAX_USED_PERCENT` (mặc định 85%): rút TTL cache và có thể bỏ ghi cache mới.

Biến chính: `RESOURCE_SAMPLE_INTERVAL`, `RESOURCE_CPU_RESERVE_PERCENT`, `RESOURCE_RAM_MIN_IDLE_PERCENT`, `RESOURCE_REDIS_MAX_USED_PERCENT`. Worker luôn publish; API chỉ đọc (prod tách service). System Health trả thêm `resource_limits`.

## Members và phân quyền

Owner: **`/members`**, **`/permissions`**.

- Role: `manager`, `viewer` (owner toàn quyền)
- Quyền resource: `read`, `upload`, `update`, `delete`, `manage`, …
- Chia sẻ folder con không bắt buộc share cả cây cha; member chỉ thấy nhánh được cấp
- Folder Root seed: `00000000-0000-4000-8000-000000000001`
- Member: `GET /api/permissions/mine`, Dashboard riêng

## File Manager (`/files`)

Cần migration `000006` trở lên. Chạy `make scheduler` để xóa S3 / purge thùng rác tự động.

Tính năng chính: upload multipart S3, folder tree, thùng rác, tìm kiếm, bulk rename, preview, share quyền.

API chính: `GET/POST /api/objects`, `/api/folders`, `/api/upload/*`, trash / restore / purge.

## Videos (`/videos`)

Cần `make worker` (FFmpeg) và Redis. Upload video qua File Manager (type `video`), sau đó mở **Videos** để chuyển mã HLS thủ công.

1. **Convert** — `POST /api/videos/{id}/convert` (quyền `convert`). Body tùy chọn: `{ "variants": ["1080p","720p","480p"] }`. Mặc định: mọi bản **không upscale** so với chiều cao nguồn; bitrate encode **scale theo file gốc** (video YouTube 1080p ~2 Mbps vẫn có thể ra 1080p/720p/480p, không bị khóa chỉ 480p).
2. **Preview** — player HLS trong dashboard (không phát file gốc từ `originals/`).
3. **Stream URL** — `GET /api/videos/{id}/hls` trả URL master có token phiên; mọi segment/variant qua `GET /stream/{id}/*` trên `API_PUBLIC_URL` (cookie sau lần tải master).
4. **Stream policy** — domain allowlist, TTL token (`PATCH /api/videos/{id}/stream-policy`).
5. **API keys** (Owner) — `/api-keys` cho embed site ngoài (scope `stream`, **chỉ** header `X-API-Key`, không dùng query `api_key`).

Owner: **`/system/storage`**, **`/system/queue`** (UI), API **`/api/system/storage`**, **`/api/system/queue`**, **`/api/system/stream-analytics`**, **`/api/system/security`**.

Health tách component: `GET /api/system/health/{database|redis|storage|worker|ffmpeg}`. Prometheus: `GET /metrics` (hạn chế bằng firewall/nginx).

Production: `TRUSTED_PROXIES` trỏ IP reverse proxy — xem [deployments/nginx-reverse-proxy.example.conf](../deployments/nginx-reverse-proxy.example.conf).

## Settings (Owner)

**`/settings`** — workspace, media, streaming, storage, security, maintenance (`system_settings`).

## Dashboard

- **Owner**: tổng quan + System Health
- **Member**: thư mục được chia sẻ + liên kết File Manager

## Kiểm tra bảo mật

```bash
export TEST_OWNER_PASSWORD='...'
cd backend && go test ./internal/handler/... -run TestSecurity_ -v
```

```bash
export API_URL=http://localhost:8080 OWNER_EMAIL=... OWNER_PASSWORD=...
./scripts/security-smoke.sh
```

## E2E

```bash
cd frontend
pnpm exec playwright install chromium
E2E_EMAIL=owner@example.com E2E_PASSWORD=secret pnpm test:e2e
```
