# Storage & CDN

MediaHub lưu blob trên **MinIO/S3** và phát nội dung qua signed URL.

## URL public

| Biến | Dùng cho |
|------|----------|
| `API_PUBLIC_URL` | `/stream/*`, HLS master, API origin |
| `CDN_PUBLIC_URL` | `/assets/*`, `/embed/*` — cache edge |

Khi set `CDN_PUBLIC_URL=https://cdn.example.com`, signed URLs trong response API trỏ qua CDN thay vì API trực tiếp.

## Nginx (gợi ý)

Proxy `/assets` và `/embed` về API origin; **giữ nguyên** query `e` và `s` (chữ ký).

`API_PUBLIC_URL` vẫn dùng cho `/stream` nếu không tách stream CDN riêng.

Ví dụ cấu hình: `deployments/nginx-reverse-proxy.example.conf`

## Stream segment CDN

Set `STREAM_INTERNAL_REDIRECT_PREFIX` để API trả `X-Accel-Redirect` — nginx stream bytes từ MinIO, API không đọc segment.

## TTL signed URL

| Biến | Mặc định | Ảnh hưởng |
|------|----------|-----------|
| `ASSET_DELIVERY_URL_TTL` | 24h | Cache edge `/assets`, `/embed` |
| `STREAM_SEGMENT_URL_TTL` | 1h | Segment HLS signed URL |
| `PRESIGNED_PUT_URL_TTL` | 1h | Direct upload integration |

## Image transform

Signed URL `/assets/{id}/thumbnail?w=320&h=320&fmt=webp` — cache Redis (`IMAGE_TRANSFORM_CACHE_TTL`).

Chi tiết query params: [Convert & delivery](../integration/04-convert-delivery.md)

## Integration API

CMS lấy delivery URLs qua `POST /api/v1/media/delivery-urls` — URLs trong response đã reflect `CDN_PUBLIC_URL` / `API_PUBLIC_URL`.
