# Frontend environment

File: `frontend/.env.local` — copy từ `frontend/.env.example`.

## Biến

| Biến | Mặc định | Mô tả |
|------|----------|--------|
| `NEXT_PUBLIC_API_URL` | `http://localhost:8080` | Base URL API — mọi request từ browser |
| `NEXT_PUBLIC_APP_URL` | *(trống)* | URL public frontend (sitemap, Open Graph) |
| `NEXT_PUBLIC_REQUIRE_SETUP_TOKEN` | `false` | `true` khi test flow setup token như production |

## Lưu ý

- Biến `NEXT_PUBLIC_*` được embed vào bundle client — không chứa secret.
- `NEXT_PUBLIC_API_URL` phải trỏ tới API mà browser truy cập được (cùng origin hoặc CORS qua `APP_URL` backend).

## Production

```env
NEXT_PUBLIC_API_URL=https://api.example.com
NEXT_PUBLIC_APP_URL=https://media.example.com
```

Backend tương ứng:

```env
APP_URL=https://media.example.com
API_PUBLIC_URL=https://api.example.com
```
