# MediaHub Integration Guide

MediaHub có thể dùng làm **media backend** cho website/CMS khác qua Integration API (`/api/v1`).

## 1. Tạo API Key

1. Đăng nhập owner → **Quản trị → API Keys**
2. Chọn scopes:
   - `media:upload` — upload file
   - `media:read` — đọc metadata, delivery URLs, HLS
   - `media:convert` — chuyển mã video sang HLS
   - `media:delete` — xóa media
   - `stream` — phát HLS qua `/stream` (embed player)
3. (Tuỳ chọn) Gán **root folder** — mọi upload/read bị giới hạn trong cây thư mục đó
4. (Tuỳ chọn) **Allowed IPs** — chỉ chấp nhận request từ các IP server (để trống = không giới hạn)

Gửi key trong header:

```http
X-API-Key: mh_...
```

## 2. Upload

### Multipart (mặc định)

Phù hợp file lớn, upload qua API:

```bash
# 1. Init
curl -X POST "$API/api/v1/media/upload/init" \
  -H "X-API-Key: $KEY" \
  -H "Content-Type: application/json" \
  -d '{"file_name":"clip.mp4","size":10485760,"mime_type":"video/mp4"}'

# 2. PUT từng chunk
curl -X PUT "$API/api/v1/media/upload/$SESSION/chunks/0" \
  -H "X-API-Key: $KEY" \
  --data-binary @chunk0.bin

# 3. Complete
curl -X POST "$API/api/v1/media/upload/$SESSION/complete" \
  -H "X-API-Key: $KEY"
```

### Direct (presigned PUT)

Client upload thẳng lên MinIO/S3 — giảm tải API:

```bash
curl -X POST "$API/api/v1/media/upload/init" \
  -H "X-API-Key: $KEY" \
  -H "Content-Type: application/json" \
  -d '{"file_name":"photo.jpg","size":204800,"mime_type":"image/jpeg","upload_mode":"direct"}'
```

Response gồm `put_url` và `put_headers`. Browser/server PUT file lên URL đó, sau đó:

```bash
curl -X POST "$API/api/v1/media/upload/$SESSION/complete" \
  -H "X-API-Key: $KEY"
```

TTL presigned URL: env `PRESIGNED_PUT_URL_TTL` (mặc định 1h).

## 3. Convert video → HLS

```bash
curl -X POST "$API/api/v1/media/$VIDEO_ID/convert" \
  -H "X-API-Key: $KEY" \
  -H "Content-Type: application/json" \
  -d '{"variants":["1080p","720p","480p"]}'
```

Khi convert thất bại (`hls_status: failed`):

```bash
curl -X POST "$API/api/v1/media/$VIDEO_ID/convert/retry" \
  -H "X-API-Key: $KEY"
```

## 4. Delivery URLs

Lấy URL signed cho thumbnail, image, HLS, embed:

```bash
curl -X POST "$API/api/v1/media/delivery-urls" \
  -H "X-API-Key: $KEY" \
  -H "Content-Type: application/json" \
  -d '{"ids":["uuid-1","uuid-2"]}'
```

- Ảnh: `/assets/{id}/thumbnail|image`
- Video HLS: `/stream/...` hoặc `GET /api/v1/media/{id}/hls`
- Embed player: `/embed/{video_id}?e=&s=`

`file_url` chỉ có khi video `allow_download=true`.

### Image transform

Thêm query params vào signed `/assets/{id}/thumbnail` hoặc `/assets/{id}/image`:

```
/assets/{id}/thumbnail?e=...&s=...&w=320&h=320&fmt=webp
```

| Param | Mô tả |
|-------|--------|
| `w` | Chiều rộng tối đa (px, max 4096) |
| `h` | Chiều cao tối đa (px, max 4096) |
| `fmt` | `jpeg`, `png`, `webp` (webp cần build CGO + libwebp) |

Kết quả được cache Redis (`IMAGE_TRANSFORM_CACHE_TTL`).

## 5. oEmbed

CMS/editor có thể discover embed HTML:

```bash
curl "http://localhost:8080/oembed?url=http://localhost:8080/embed/VIDEO_UUID"
```

URL được chấp nhận (host phải khớp `CDN_PUBLIC_URL`, `API_PUBLIC_URL`, hoặc `APP_URL`):

- `{base}/embed/{video_id}`
- `{base}/videos/{video_id}` (frontend)

Response theo [oEmbed 1.0](https://oembed.com/): `html`, `thumbnail_url`, `width`, `height`.

## 6. CDN domain

Set `CDN_PUBLIC_URL=https://cdn.example.com` để signed URLs trỏ qua CDN/nginx (cache `/assets`, `/embed`).

Nginx proxy `/assets` và `/embed` về API origin; giữ nguyên query `e` và `s`.

`API_PUBLIC_URL` vẫn dùng cho `/stream` nếu không tách stream CDN riêng.

## 7. Webhooks

Owner tạo webhook tại `POST /api/webhooks` (JWT, không phải API key).

Events:

| Event | Khi nào |
|-------|---------|
| `media.upload.completed` | Upload complete |
| `video.convert.started` | Bắt đầu convert |
| `video.convert.completed` | Convert thành công |
| `video.convert.failed` | Convert thất bại (hết retry) |
| `media.deleted` | Xóa media |

Verify HMAC:

```
signature = HMAC-SHA256(secret, timestamp + "." + raw_body)
Header: X-MediaHub-Signature: sha256=<hex>
Header: X-MediaHub-Timestamp: <unix>
```

## 8. Quota

Per API key (không áp dụng `/stream` segments):

| Env | Mặc định |
|-----|----------|
| `API_KEY_UPLOAD_INIT_PER_MIN` | 120 |
| `API_KEY_CONVERT_PER_HOUR` | 60 |

Vượt quota → HTTP 429.

## 9. OpenAPI

Spec: [`docs/api/openapi.yaml`](./api/openapi.yaml)

Import vào Postman/Insomnia hoặc codegen client.

## 10. Production checklist

- `API_PUBLIC_URL` trỏ domain public (signed URLs)
- `STREAM_INTERNAL_REDIRECT_PREFIX` + nginx cho segment CDN
- `ASSET_DELIVERY_URL_TTL` cho cache edge
- HTTPS bắt buộc cho webhook endpoints
