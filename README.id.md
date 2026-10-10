# Go + React Boilerplate

[English](README.md) · **Bahasa Indonesia**

Fondasi admin yang bisa dipakai ulang: API Go (chi + pgx) · React SPA (Vite, TypeScript, Tailwind, shadcn/ui) · PostgreSQL.
Isinya: autentikasi, user dengan banyak role, hak akses menu × aksi, sidebar dinamis, dan tema (light/dark).

> **Status:** lengkap. Backend, frontend, dan stack Docker sudah jadi, sudah di-review keamanannya, dan sudah ada tes. Mulai dari [FINAL_REPORT.md](FINAL_REPORT.md).

## Dokumen

Dokumen di bawah ditulis dalam bahasa Inggris.

- [ARCHITECTURE.md](ARCHITECTURE.md): stack dan versi, struktur folder, konvensi API, desain auth/RBAC, skema DB, konfigurasi, testing
- [FEATURE_PARITY.md](FEATURE_PARITY.md): perbandingan dengan boilerplate Laravel asalnya, mana yang dipertahankan/diubah, dan bug lama yang sengaja tidak dibawa
- [backend/api/openapi.yaml](backend/api/openapi.yaml): referensi API (juga tersedia di `GET /api/v1/openapi.yaml`)
- [SECURITY.md](SECURITY.md): kontrol keamanan, temuan review, dan risiko yang tersisa
- [FINAL_REPORT.md](FINAL_REPORT.md): ringkasan apa yang dibangun, hasil tes, dan keterbatasan

## Pakai sebagai template project sendiri

1. Clone lalu buang history-nya: `git clone <url-repo-ini> my-app && cd my-app && rm -rf .git && git init -b main`.
2. **Nama app dan logo:** nama bawaan adalah "Admin Panel". Ubah `APP_NAME` (`.env` di root, dipakai Docker), `APP_NAME` di `backend/.env` (dipakai di email), dan `VITE_APP_NAME` di `frontend/.env` (sidebar, halaman login, judul tab browser). Logo ada di `frontend/public/favicon.svg`, ganti dengan SVG milikmu.
3. **Module path Go:** modulenya `github.com/reihnxx/go-react-boilerplate/backend`. Cara menggantinya:
   ```bash
   NEW=github.com/org-kamu/my-app/backend
   cd backend && go mod edit -module "$NEW"
   grep -rl 'github.com/reihnxx/go-react-boilerplate/backend' . | xargs sed -i.bak "s#github.com/reihnxx/go-react-boilerplate/backend#$NEW#g" && find . -name '*.bak' -delete
   go build ./...
   ```
   (`sed -i.bak` jalan di macOS maupun Linux.)
4. Opsional: ganti `name` di `frontend/package.json`, fallback `<title>` di `frontend/index.html`, dan `title` di `backend/api/openapi.yaml`.
5. Warna brand: ubah CSS variable di `frontend/src/index.css` (bawaan `#24695c`).
6. Jalankan `make check` untuk memastikan semuanya masih lolos, lalu ikuti [Mulai cepat](#mulai-cepat).

## Mulai cepat

Pilih salah satu cara menjalankan. Port-nya beda, jangan dicampur.

| Mode | Perintah | Buka | Perubahan kode |
|---|---|---|---|
| **Docker** (full stack) | `make up` | http://localhost:8080 | **tidak langsung terlihat**: rebuild dengan `docker compose up -d --build web` (frontend) atau `api` (backend) |
| **Dev lokal** (tanpa Docker) | `make dev-api` + `make dev-web` | http://localhost:5173 | **langsung terlihat**: hot reload Vite untuk frontend; restart `make dev-api` untuk perubahan backend |
| **Hybrid** | DB di Docker, API + Vite lokal | http://localhost:5173 | sama seperti Dev lokal |

Untuk Hybrid, jalankan database saja dengan `docker compose up -d db`, lalu isi `DATABASE_URL` di `backend/.env` dengan `postgres://app:<POSTGRES_PASSWORD>@127.0.0.1:5433/boilerplate?sslmode=disable` dan lanjutkan dari langkah 3 di [Opsi B](#opsi-b-development-lokal).

## Kebutuhan

- **Opsi A, semua di Docker:** Docker dengan Compose v2.
- **Opsi B, development lokal:** Go 1.26+, Node 22.13+ (disarankan 22.22+ atau 24 LTS), dan PostgreSQL 14+ (misalnya DBngin atau Postgres.app).

## Opsi A: jalankan full stack dengan Docker Compose

```bash
cp .env.example .env                 # isi POSTGRES_PASSWORD (hanya huruf, angka, - dan _)
make up                              # = docker compose up -d --build
ADMIN_PASSWORD='password-kuat-1' make up-admin EMAIL=kamu@example.com
# buka http://localhost:8080 lalu login sebagai "admin"
```

`make up` menjalankan empat service berurutan: `db` → `migrate` (sekali jalan) → `api` → `web`. Yang dibuka ke host hanya port web (8080). Database juga bisa diakses dari komputermu lewat `127.0.0.1:5433` untuk tool DB; port 5433 dipilih supaya tidak bentrok dengan PostgreSQL lokal. `make down` menghentikan stack dan menyimpan datanya. `make reset-db` menghapus volume database.

Tidak ada akun admin bawaan. Akun pertama selalu dibuat lewat `make up-admin` (Docker) atau `make create-admin` (lokal).

## Opsi B: development lokal

### Backend

```bash
# 1. Database (contoh untuk Postgres lokal dengan user "postgres" tanpa password)
createdb -h 127.0.0.1 -U postgres go_boilerplate
createdb -h 127.0.0.1 -U postgres go_boilerplate_test   # hanya untuk integration test

# 2. Konfigurasi
cp backend/.env.example backend/.env      # sesuaikan DATABASE_URL kalau perlu

# 3. Skema + data referensi (role, menu, aksi)
make migrate

# 4. Administrator pertama (password dibaca dari ADMIN_PASSWORD atau ditanyakan)
ADMIN_PASSWORD='password-kuat-1' make create-admin EMAIL=kamu@example.com USERNAME=admin NAME="Super Admin"

# 5. Opsional: 21 user demo dengan role "user" (khusus development)
SEED_PASSWORD='Demo12345' make seed

# 6. Jalankan API di :8080
make dev-api
curl localhost:8080/healthz
```

### Frontend

```bash
cp frontend/.env.example frontend/.env    # nama app, API base, target proxy dev
make install-web                          # npm ci
make dev-web                              # http://localhost:5173 (proxy /api → :8080)
```

Buka http://localhost:5173 dan login dengan admin yang tadi dibuat. SPA dan API harus disajikan dari **origin yang sama**. Saat development, Vite yang mem-proxy `/api`. Di production, pasang reverse proxy di depan keduanya (lihat ARCHITECTURE.md §13).

## Troubleshooting

### Aku edit file (misalnya teks dashboard) tapi setelah refresh tidak berubah

Kemungkinan besar kamu sedang membuka stack Docker di http://localhost:8080. Container itu menyajikan **hasil build** frontend (`npm run build` hanya jalan sekali, di dalam `docker build`, dan hasilnya masuk ke image `web`). Mengedit `src/` di komputermu tidak menyentuh image itu, jadi di-refresh berapa kali pun tidak akan berubah.

Pilih salah satu:

- **Saat development:** jangan buka `:8080`. Jalankan `make dev-api` dan `make dev-web`, lalu buka http://localhost:5173. Vite hot reload tiap kali file disimpan.
- **Supaya perubahan muncul di Docker:** rebuild image: `docker compose up -d --build web` (frontend) atau `docker compose up -d --build api` (backend). Lalu hard refresh (Cmd+Shift+R).

Cek cepat: `lsof -iTCP:5173 -sTCP:LISTEN` tidak menampilkan apa-apa → dev server Vite tidak jalan.

### Masalah umum lain

- **Port 8080 sudah dipakai:** API (`HTTP_ADDR`) dan service `web` Docker sama-sama default ke 8080. Jangan jalankan `make dev-api` saat stack Docker menyala, atau ubah `WEB_PORT` di `.env` root.
- **Login berhasil di Docker tapi gagal di lokal (atau sebaliknya):** `APP_BASE_URL` / `APP_ALLOWED_ORIGINS` harus sama dengan URL di browser (`:5173` untuk dev lokal, `:8080` untuk Docker).
- **Error `POSTGRES_PASSWORD` saat `make up`:** salin `.env.example` ke `.env` lalu isi.
- **Koneksi DB lokal ditolak:** DB Docker ada di `127.0.0.1:5433`, Postgres lokal biasanya di `5432`. Cek `DATABASE_URL` di `backend/.env`.

## Pakai API langsung

Login lewat `POST /api/v1/auth/login` dengan body `{"login": "admin", "password": "…"}`. Respons mengisi cookie HttpOnly `sid` dan mengembalikan `csrf_token`. Kirim token itu sebagai header `X-CSRF-Token` di setiap POST/PUT/PATCH/DELETE.

## Perintah

| Perintah | Fungsi |
|---|---|
| `make dev-api` | jalankan API (`go run ./cmd/api serve`) |
| `make build-api` | build `backend/bin/api` (binary statis) |
| `make migrate` / `migrate-down` / `migrate-status` | migrasi goose (sudah tertanam di binary) |
| `make create-admin EMAIL=…` | buat super-admin |
| `make seed` | user demo (ditolak kalau `APP_ENV=production`) |
| `make test-api` | unit test, tanpa database |
| `make test-api-integration` | unit + integration test ke `TEST_DATABASE_URL` (bawaan `go_boilerplate_test` di 127.0.0.1) |
| `make lint-api` | cek `gofmt` + `go vet` |
| `make check-api` | lint + build + semua tes backend |
| `make dev-web` / `build-web` | dev server Vite / build production ke `frontend/dist` |
| `make lint-web` | ESLint + `tsc` + cek Prettier |
| `make test-web` | Vitest (tanpa backend; API di-mock) |
| `cd frontend && npm run test:coverage` | laporan coverage frontend |
| `make check-web` | lint + tes + build |
| `make check` | semuanya (backend + frontend) |
| `cd frontend && npm run format` | format frontend dengan Prettier |
| `make up` / `down` / `logs` / `ps` | stack Docker Compose |
| `make up-admin EMAIL=…` | buat super-admin di dalam stack Compose (`ADMIN_PASSWORD` dari shell) |
| `make reset-db` | **menghapus** volume database Compose |

Binary-nya punya subcommand yang sama: `bin/api serve | migrate up|down|status | create-admin -email … | seed`.

## Deploy

File Compose bisa langsung dipakai di server, tapi ada yang harus kamu siapkan sendiri:

1. Pasang HTTPS di depan container `web`, pakai reverse proxy atau load balancer. Tambahkan HSTS di situ.
2. Set `APP_ENV=production` dan `COOKIE_SECURE=true`. Di mode production API menolak start kalau cookie tidak aman.
3. Set `APP_BASE_URL` dan `APP_ALLOWED_ORIGINS` ke URL publik.
4. Set `MAIL_DRIVER=smtp` dan setelan `SMTP_*` supaya email reset password terkirim.
5. Backup volume `db-data`.

Daftar lengkapnya ada di ARCHITECTURE.md §13.

### Deploy tanpa Docker

Kamu butuh PostgreSQL 14+, reverse proxy (contoh pakai nginx), dan server Linux. Build bisa di komputer mana pun yang punya Go dan Node.

```bash
# 1. Build
make build-api                                    # → backend/bin/api (binary statis, migrasi sudah tertanam)
VITE_APP_NAME="Nama App" make build-web           # → frontend/dist (nilai VITE_* ikut masuk bundle; jangan taruh secret)

# 2. Salin backend/bin/api ke server (mis. /opt/app/api) dan frontend/dist ke /var/www/app
```

Konfigurasi API lewat environment variable atau file `.env` di working directory-nya (daftar lengkap: `backend/.env.example`). Minimal untuk production:

```bash
APP_ENV=production
HTTP_ADDR=127.0.0.1:8080
DATABASE_URL=postgres://app:PASSWORD@127.0.0.1:5432/boilerplate?sslmode=disable
APP_BASE_URL=https://app.example.com
APP_ALLOWED_ORIGINS=https://app.example.com
COOKIE_SECURE=true
TRUST_PROXY=true            # hanya karena nginx menimpa header forwarding
MAIL_DRIVER=smtp            # plus SMTP_HOST / SMTP_PORT / SMTP_USER / SMTP_PASSWORD / MAIL_FROM
```

```bash
# 3. Pertama kali jalan
/opt/app/api migrate up
ADMIN_PASSWORD='password-kuat-1' /opt/app/api create-admin -email kamu@example.com -username admin -name "Super Admin"
```

Jalankan API lewat systemd (`/etc/systemd/system/app-api.service`):

```ini
[Unit]
Description=Admin Panel API
After=network.target postgresql.service

[Service]
WorkingDirectory=/opt/app
EnvironmentFile=/opt/app/.env
ExecStart=/opt/app/api serve
Restart=on-failure
User=app

[Install]
WantedBy=multi-user.target
```

Lalu `systemctl enable --now app-api`. nginx menyajikan SPA dan mem-proxy `/api/` di **origin yang sama** (HTTPS diterminasi di sini, tambahkan HSTS):

```nginx
server {
    listen 443 ssl;
    server_name app.example.com;
    # ssl_certificate ... ; add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    root /var/www/app;

    location /api/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $remote_addr;   # timpa, jangan append
        proxy_set_header X-Forwarded-Proto $scheme;
    }
    location /assets/ { add_header Cache-Control "public, max-age=31536000, immutable"; try_files $uri =404; }
    location /        { add_header Cache-Control "no-cache"; try_files $uri $uri/ /index.html; }
}
```

Untuk security header (CSP dll.) salin barisnya dari `frontend/deploy/security-headers.conf` ke location `/` dan `/assets/`. Untuk update berikutnya: build ulang, ganti binary dan `dist`, jalankan `api migrate up`, lalu `systemctl restart app-api`.

## Menambah modul

1. **Migrasi:** buat tabel dan insert satu baris `menus` (`slug` unik, `path`, `icon`, group, `sort_order`).
2. **Backend:** buat package `internal/<modul>` berisi `handler.go`, `service.go`, dan `store.go`. Daftarkan route-nya di `internal/app/app.go`. Lindungi tiap route dengan `g.Require("<slug>", rbac.View|Create|Update|Delete)`. Dokumentasikan route di `api/openapi.yaml`; ada tes yang gagal kalau lupa.
3. **Frontend:** buat `src/features/<modul>/` (`api.ts` berisi hook TanStack Query, `schemas.ts` berisi zod, halaman yang memakai `DataTable` + dialog form + `ConfirmDialog`). Tambahkan route lazy di `src/app/routes.tsx` sesuai `path` menu, dibungkus `<RequirePermission menu="<slug>">`. Sembunyikan tombol dengan `usePermission()`. Kalau menu pakai icon baru, daftarkan di `src/lib/icons.ts`.
4. **Hak akses:** beri aksi ke role lewat matriks hak akses. Super-admin otomatis punya semua hak akses.

## Komponen UI

UI memakai [shadcn/ui](https://ui.shadcn.com) (style `radix-nova`, lihat `frontend/components.json`). Komponennya ada di `src/components/ui/` dan jadi bagian repo ini. Cara menambah:

```bash
cd frontend && npx shadcn@4.21.4 add <komponen>
```

Kalau CLI menambahkan lagi `@import "shadcn/tailwind.css"` ke `index.css` atau `shadcn` ke `package.json`, hapus lagi. CSS yang sama sudah ada di `src/styles/shadcn.css`, dan package CLI membawa dependency dev dengan advisory keamanan.

Warna brand dan design token lain adalah CSS variable di `frontend/src/index.css` (`:root` dan `.dark`).

## Lisensi

[MIT](LICENSE)
