# Convert & delivery

Scope: `media:read`, `media:convert`, `stream` (cho phát/embed).

## Convert video → HLS

Scope `media:convert`:

```bash
curl -X POST "$API/api/v1/media/$VIDEO_ID/convert" \
  -H "X-API-Key: $KEY" \
  -H "Content-Type: application/json" \
  -d '{"variants":["1080p","720p","480p"]}'
```

Response `202` với object `job`. Poll `GET /api/v1/media/{id}` — field `hls_status`: `pending`, `processing`, `ready`, `failed`.

Khi `failed`:

```bash
curl -X POST "$API/api/v1/media/$VIDEO_ID/convert/retry" \
  -H "X-API-Key: $KEY"
```

## Delivery URLs

Scope `media:read` — lấy URL signed batch:

```bash
curl -X POST "$API/api/v1/media/delivery-urls" \
  -H "X-API-Key: $KEY" \
  -H "Content-Type: application/json" \
  -d '{"ids":["uuid-1","uuid-2"]}'
```

Response map theo `public_id`:

| Field | Mô tả |
|-------|--------|
| `thumbnail_url` | Ảnh thumbnail |
| `image_url` | Ảnh full |
| `file_url` | File gốc — chỉ khi `allow_download=true` |
| `hls_master_url` | Master playlist HLS |
| `embed_url` / `embed_html` | Player embed |
| `cache_until` | Unix timestamp hết hạn cache edge |

Hoặc lấy metadata + URLs một object:

```bash
curl -H "X-API-Key: $KEY" "$API/api/v1/media/$ID"
```

HLS chi tiết:

```bash
curl -H "X-API-Key: $KEY" "$API/api/v1/media/$ID/hls"
```

## Đường dẫn public

- Ảnh: `/assets/{id}/thumbnail` hoặc `/assets/{id}/image`
- Video HLS: `/stream/...` hoặc URL từ `/api/v1/media/{id}/hls`
- Embed: `/embed/{video_id}?e=&s=`

## Image transform

Thêm query vào signed URL `/assets/{id}/thumbnail` hoặc `/assets/{id}/image`:

```
/assets/{id}/thumbnail?e=...&s=...&w=320&h=320&fmt=webp
```

| Param | Mô tả |
|-------|--------|
| `w` | Chiều rộng tối đa (px, max 4096) |
| `h` | Chiều cao tối đa (px, max 4096) |
| `fmt` | `jpeg`, `png`, `webp` (webp cần CGO + libwebp) |

Cache Redis: `IMAGE_TRANSFORM_CACHE_TTL`.

## oEmbed

CMS/editor discover embed HTML (không cần API key):

```bash
curl "$API/oembed?url=$API/embed/VIDEO_UUID"
```

URL được chấp nhận (host khớp `CDN_PUBLIC_URL`, `API_PUBLIC_URL`, hoặc `APP_URL`):

- `{base}/embed/{video_id}`
- `{base}/videos/{video_id}` (frontend)

Response [oEmbed 1.0](https://oembed.com/): `html`, `thumbnail_url`, `width`, `height`.

## CDN

Set `CDN_PUBLIC_URL` — xem [Storage & CDN](../configuration/storage-cdn.md)
