# Upload

Integration API hỗ trợ hai chế độ: **multipart** (mặc định) và **direct** (presigned PUT lên MinIO).

Yêu cầu scope: `media:upload`

## Giới hạn

```bash
curl -H "X-API-Key: $KEY" "$API/api/v1/media/upload/limits"
```

Response: `{ "max_upload_bytes": ... }` — theo settings workspace.

## Multipart (mặc định)

Phù hợp file lớn, upload qua API:

```bash
# 1. Init
curl -X POST "$API/api/v1/media/upload/init" \
  -H "X-API-Key: $KEY" \
  -H "Content-Type: application/json" \
  -d '{"file_name":"clip.mp4","size":10485760,"mime_type":"video/mp4"}'

# 2. PUT từng chunk (index 0 .. total_chunks-1)
curl -X PUT "$API/api/v1/media/upload/$SESSION/chunks/0" \
  -H "X-API-Key: $KEY" \
  --data-binary @chunk0.bin

# 3. Complete
curl -X POST "$API/api/v1/media/upload/$SESSION/complete" \
  -H "X-API-Key: $KEY"
```

### Body init

| Field | Bắt buộc | Mô tả |
|-------|----------|--------|
| `file_name` | ✓ | Tên file |
| `size` | ✓ | Kích thước bytes |
| `mime_type` | | MIME type |
| `parent_id` | | UUID folder (mặc định = root folder của key) |
| `chunk_size` | | Kích thước chunk (tuỳ chọn) |
| `upload_mode` | | `multipart` (default) hoặc `direct` |

### Hủy session

```bash
curl -X DELETE "$API/api/v1/media/upload/$SESSION" -H "X-API-Key: $KEY"
```

## Direct (presigned PUT)

Client upload thẳng lên MinIO/S3 — giảm tải API:

```bash
curl -X POST "$API/api/v1/media/upload/init" \
  -H "X-API-Key: $KEY" \
  -H "Content-Type: application/json" \
  -d '{"file_name":"photo.jpg","size":204800,"mime_type":"image/jpeg","upload_mode":"direct"}'
```

Response gồm `put_url`, `put_headers`, `session_public_id`. PUT file lên `put_url`, sau đó:

```bash
curl -X POST "$API/api/v1/media/upload/$SESSION/complete" \
  -H "X-API-Key: $KEY"
```

TTL presigned URL: env `PRESIGNED_PUT_URL_TTL` (mặc định 1h).

## Sau khi complete

Response trả `MediaObject` với `public_id` — dùng cho convert, delivery URLs.

Webhook `media.upload.completed` (nếu đã cấu hình): [Webhooks](05-webhooks.md)

Tiếp theo: [Convert & delivery](04-convert-delivery.md)
