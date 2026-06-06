# Webhooks

Owner tạo webhook qua **Dashboard API** (JWT), không qua API key.

| Method | Path |
|--------|------|
| GET | `/api/webhooks` |
| POST | `/api/webhooks` |
| DELETE | `/api/webhooks/{public_id}` |

## Events

| Event | Khi nào |
|-------|---------|
| `media.upload.completed` | Upload complete |
| `video.convert.started` | Bắt đầu convert |
| `video.convert.completed` | Convert thành công |
| `video.convert.failed` | Convert thất bại (hết retry) |
| `media.deleted` | Xóa media |

## Payload mẫu

```json
{
  "event": "media.upload.completed",
  "timestamp": "2026-06-06T10:00:00Z",
  "data": {
    "public_id": "550e8400-e29b-41d4-a716-446655440000",
    "type": "video",
    "name": "clip.mp4",
    "size_bytes": 10485760
  }
}
```

```json
{
  "event": "video.convert.completed",
  "timestamp": "2026-06-06T10:05:00Z",
  "data": {
    "public_id": "550e8400-e29b-41d4-a716-446655440000",
    "hls_status": "ready"
  }
}
```

```json
{
  "event": "video.convert.failed",
  "timestamp": "2026-06-06T10:05:00Z",
  "data": {
    "public_id": "550e8400-e29b-41d4-a716-446655440000",
    "error": "ffmpeg exited with code 1"
  }
}
```

## Verify HMAC

```
signature = HMAC-SHA256(webhook_secret, timestamp + "." + raw_body)
```

Headers gửi kèm:

```http
X-MediaHub-Signature: sha256=<hex>
X-MediaHub-Timestamp: <unix_seconds>
```

Ví dụ Node.js:

```javascript
const crypto = require("crypto");

function verify(secret, rawBody, timestamp, signatureHeader) {
  const expected = crypto
    .createHmac("sha256", secret)
    .update(`${timestamp}.${rawBody}`)
    .digest("hex");
  const received = signatureHeader.replace(/^sha256=/, "");
  return crypto.timingSafeEqual(
    Buffer.from(expected),
    Buffer.from(received),
  );
}
```

## Production

- Endpoint webhook phải **HTTPS**
- Trả `2xx` nhanh; xử lý nặng async phía receiver
- Secret lưu an toàn — rotate khi nghi ngờ lộ
