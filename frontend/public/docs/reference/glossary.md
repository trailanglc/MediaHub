# Thuật ngữ

| Thuật ngữ | Mô tả |
|-----------|--------|
| **Owner** | Tài khoản quản trị cao nhất — tạo members, API keys, settings |
| **Member** | User `manager` hoặc `viewer` — quyền theo folder/file |
| **Integration API** | REST `/api/v1` — auth `X-API-Key` cho CMS |
| **Dashboard API** | REST `/api/*` — auth JWT cookie/Bearer |
| **Scope** | Quyền gắn API key: `media:upload`, `media:read`, … |
| **Root folder** | Giới hạn namespace media của một API key |
| **Signed URL** | URL có query `e` (expiry) + `s` (HMAC) — `/assets`, `/embed`, stream |
| **HLS** | HTTP Live Streaming — video adaptive bitrate qua `.m3u8` + `.ts` |
| **Convert job** | Job Asynq + FFmpeg tạo variant HLS |
| **Governor** | Resource governor — điều tiết convert theo CPU/RAM/Redis |
| **Gateway** | Binary `cmd/gateway` — API + scheduler + worker một process |
| **Scheduler** | Job nền: purge trash, upload expiry, storage delete |
| **Direct upload** | Presigned PUT thẳng MinIO — `upload_mode: direct` |
| **oEmbed** | Chuẩn discover embed HTML từ URL video |
