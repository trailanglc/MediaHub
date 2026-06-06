# Production tuning

Tài liệu deploy backend trên VPS / server vật lý. Xem [Cài đặt](../getting-started/02-installation.md) trước.

## Khởi động backend

```bash
cd backend
go build -o mediahub ./cmd/gateway
./mediahub
```

Một binary `cmd/gateway` chạy API + scheduler + worker. Frontend deploy riêng (Next.js / CDN).

Tách worker khi convert nặng làm API chậm:

```bash
# Terminal 1
SKIP_WORKER=1 ./mediahub

# Terminal 2
cd backend && go run ./cmd/worker
```

## Giới hạn tài nguyên (tự động)

Khi **không** set trong `backend/.env`, server tự scale theo CPU/RAM với **trần 90%**:

| Biến (tùy chọn) | Mặc định autoscale |
|-----------------|-------------------|
| `CONVERT_MAX_CONCURRENT` | Theo CPU/RAM, max 90% |
| `CONVERT_QUEUE_MAX_DEPTH` | `CONVERT_MAX × 25` (50–200) |
| `RESOURCE_CPU_RESERVE_PERCENT` | 10 (= max 90% CPU) |
| `RESOURCE_RAM_MIN_IDLE_PERCENT` | max(10%, 1 GiB headroom) |

**Runtime (resource governor):**

- RAM/CPU **≥ 80%** → cảnh báo trên web (owner)
- **≥ 90%** → giảm convert, `system_busy`, từ chối enqueue mới
- Hàng đợi convert đầy → HTTP `503 queue_full`
- Scale **lên** chậm; scale **xuống** nhanh

## Override sau benchmark

```env
CONVERT_MAX_CONCURRENT=12
CONVERT_QUEUE_MAX_DEPTH=150
DB_MAX_CONNS=48
```

Chỉ set tay khi đã quan sát System Health vài ngày.

## systemd (gợi ý)

```ini
[Unit]
Description=MediaHub gateway
After=network.target docker.service

[Service]
Type=simple
User=mediahub
WorkingDirectory=/opt/mediahub/backend
EnvironmentFile=/opt/mediahub/backend/.env
ExecStart=/opt/mediahub/backend/mediahub
Restart=on-failure
RestartSec=5
MemoryMax=40G
MemoryHigh=36G

[Install]
WantedBy=multi-user.target
```

## Health & cảnh báo

Owner xem **System Health** hoặc banner dashboard:

- `resource_limits.system_busy`
- `queue_depth` / `queue_max_depth`
- `warnings` (RAM, CPU, disk host, MinIO)

API: `GET /api/system/health` (owner).

## Infra Docker

Postgres, Redis, MinIO nên cùng VPS hoặc subnet nội bộ. Đảm bảo disk host và bucket MinIO **< 80%**.

## Integration production

Checklist API/CMS: [Production checklist](../integration/07-production-checklist.md)
