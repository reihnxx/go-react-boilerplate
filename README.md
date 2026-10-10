# Go + React Boilerplate

**English** · [Bahasa Indonesia](README.id.md)

Reusable admin foundation: Go API (chi + pgx) · React SPA (Vite, TypeScript, Tailwind, shadcn/ui) · PostgreSQL.
It is a re-implementation of the internal Laravel "viho" boilerplate: authentication, multi-role users, menu × action permissions, dynamic sidebar, and theming.

> **Status:** complete. The backend, frontend and Docker stack are implemented, security-reviewed and tested. Start with [FINAL_REPORT.md](FINAL_REPORT.md).

## Documents

- [ARCHITECTURE.md](ARCHITECTURE.md): stack and versions, layout, API conventions, auth/RBAC design, schema, config, testing
- [FEATURE_PARITY.md](FEATURE_PARITY.md): what the Laravel boilerplate does, what is kept or changed, and known defects not carried over
- [backend/api/openapi.yaml](backend/api/openapi.yaml): API reference (also served at `GET /api/v1/openapi.yaml`)
- [SECURITY.md](SECURITY.md): security controls, review findings and remaining risks
- [FINAL_REPORT.md](FINAL_REPORT.md): summary of what was built, differences from Laravel, test results, limitations

## Using this as a template for your own project

1. Clone it and remove the history: `git clone <this-repo> my-app && cd my-app && rm -rf .git && git init -b main`.
2. **App name and logo:** the default name is "Admin Panel". Change `APP_NAME` (root `.env`, used by Docker), `APP_NAME` in `backend/.env` (used in emails) and `VITE_APP_NAME` in `frontend/.env` (sidebar, login page, browser title). The logo is `frontend/public/favicon.svg`; replace it with your own SVG.
3. **Go module path:** the module is `github.com/reihnxx/go-react-boilerplate/backend`. To rename it:
   ```bash
   NEW=github.com/your-org/my-app/backend
   cd backend && go mod edit -module "$NEW"
   grep -rl 'github.com/reihnxx/go-react-boilerplate/backend' . | xargs sed -i.bak "s#github.com/reihnxx/go-react-boilerplate/backend#$NEW#g" && find . -name '*.bak' -delete
   go build ./...
   ```
   (`sed -i.bak` works on both macOS and Linux.)
4. Optionally rename `name` in `frontend/package.json`, the `<title>` fallback in `frontend/index.html`, and the title in `backend/api/openapi.yaml`.
5. Brand colour: edit the CSS variables in `frontend/src/index.css` (default `#24695c`).
6. Run `make check` to confirm everything still passes, then follow [Quick start](#quick-start).

## Quick start

Pick one way to run the project. They use different ports, so do not mix them up.

| Mode | Command | Open | Code changes |
|---|---|---|---|
| **Docker** (full stack) | `make up` | http://localhost:8080 | **not live**: rebuild with `docker compose up -d --build web` (frontend) or `api` (backend) |
| **Local dev** (no Docker) | `make dev-api` + `make dev-web` | http://localhost:5173 | **live**: Vite hot reload for the frontend; restart `make dev-api` for backend changes |
| **Hybrid** | DB in Docker, API + Vite local | http://localhost:5173 | same as Local dev |

For Hybrid, start only the database with `docker compose up -d db`, then set `DATABASE_URL` in `backend/.env` to `postgres://app:<POSTGRES_PASSWORD>@127.0.0.1:5433/boilerplate?sslmode=disable` and follow [Option B](#option-b-local-development) from step 3.

## Requirements

- **Option A, everything in Docker:** Docker with Compose v2.
- **Option B, local development:** Go 1.26+, Node 22.13+ (22.22+ or 24 LTS recommended), and PostgreSQL 14+ (for example DBngin or Postgres.app).

## Option A: run the full stack with Docker Compose

```bash
cp .env.example .env                 # set POSTGRES_PASSWORD (letters, digits, - and _ only)
make up                              # = docker compose up -d --build
ADMIN_PASSWORD='choose-a-strong-one1' make up-admin EMAIL=you@example.com
# then open http://localhost:8080 and sign in as "admin"
```

`make up` starts four services in order: `db` → `migrate` (one-shot) → `api` → `web`. Only the web port (8080) is published. The database is also reachable from your machine on `127.0.0.1:5433` for DB tools; port 5433 avoids clashing with a local PostgreSQL. `make down` stops the stack and keeps the data. `make reset-db` deletes the database volume.

## Option B: local development

### Backend

```bash
# 1. Databases (example for a local Postgres with user "postgres" and no password)
createdb -h 127.0.0.1 -U postgres go_boilerplate
createdb -h 127.0.0.1 -U postgres go_boilerplate_test   # only for integration tests

# 2. Configuration
cp backend/.env.example backend/.env      # adjust DATABASE_URL if needed

# 3. Schema + reference data (roles, menus, actions)
make migrate

# 4. First administrator (password is read from ADMIN_PASSWORD or prompted)
ADMIN_PASSWORD='choose-a-strong-one1' make create-admin EMAIL=you@example.com USERNAME=admin NAME="Super Admin"

# 5. Optional: 21 demo users with the "user" role (development only)
SEED_PASSWORD='Demo12345' make seed

# 6. Run the API on :8080
make dev-api
curl localhost:8080/healthz
```

### Frontend

```bash
cp frontend/.env.example frontend/.env    # app name, API base, dev proxy target
make install-web                          # npm ci
make dev-web                              # http://localhost:5173 (proxies /api → :8080)
```

Open http://localhost:5173 and sign in with the admin you created. The SPA and the API must be served from the **same origin**. In development Vite proxies `/api`. In production, put a reverse proxy in front of both (see ARCHITECTURE.md §13).

## Troubleshooting

### I edited a file (for example the dashboard text) and a refresh does not show it

You are almost certainly looking at the Docker stack on http://localhost:8080. That container serves a **pre-built copy** of the frontend (`npm run build` runs once, inside `docker build`, and the result is baked into the `web` image). Editing `src/` on your machine does not touch it, so no amount of refreshing will help.

Use one of these:

- **While developing:** stop looking at `:8080`. Run `make dev-api` and `make dev-web`, then open http://localhost:5173. Vite hot-reloads on save.
- **To see the change in Docker:** rebuild the image: `docker compose up -d --build web` (frontend) or `docker compose up -d --build api` (backend). Then hard-refresh (Cmd+Shift+R). `index.html` is served with `no-cache` and hashed assets get new names, so a normal refresh is usually enough after the rebuild.

Quick check: `lsof -iTCP:5173 -sTCP:LISTEN` shows nothing → the Vite dev server is not running.

### Other common problems

- **Port 8080 already in use:** the API (`HTTP_ADDR`) and the Docker `web` service both default to 8080. Do not run `make dev-api` while the Docker stack is up, or change `WEB_PORT` in the root `.env`.
- **Login works in Docker but fails locally (or the other way around):** `APP_BASE_URL` / `APP_ALLOWED_ORIGINS` must match the URL in the browser (`:5173` for local dev, `:8080` for Docker).
- **`POSTGRES_PASSWORD` error on `make up`:** copy `.env.example` to `.env` and set it.
- **Local database connection refused:** Docker's DB is on `127.0.0.1:5433`, a local Postgres is usually on `5432`. Check `DATABASE_URL` in `backend/.env`.

## Using the API directly

Log in with `POST /api/v1/auth/login` `{"login": "admin", "password": "…"}`. The response sets an HttpOnly `sid` cookie and returns a `csrf_token`. Send that token as `X-CSRF-Token` on every POST/PUT/PATCH/DELETE.

## Commands

| Command | What it does |
|---|---|
| `make dev-api` | run the API (`go run ./cmd/api serve`) |
| `make build-api` | build `backend/bin/api` (static binary) |
| `make migrate` / `migrate-down` / `migrate-status` | goose migrations (embedded in the binary) |
| `make create-admin EMAIL=…` | create a super-admin |
| `make seed` | demo users (refuses when `APP_ENV=production`) |
| `make test-api` | unit tests, no database needed |
| `make test-api-integration` | unit + integration tests against `TEST_DATABASE_URL` (default `go_boilerplate_test` on 127.0.0.1) |
| `make lint-api` | `gofmt` check + `go vet` |
| `make check-api` | lint + build + all backend tests |
| `make dev-web` / `build-web` | Vite dev server / production build to `frontend/dist` |
| `make lint-web` | ESLint + `tsc` + Prettier check |
| `make test-web` | Vitest (no backend needed; the API is mocked) |
| `cd frontend && npm run test:coverage` | frontend coverage report |
| `make check-web` | lint + tests + build |
| `make check` | everything (backend + frontend) |
| `cd frontend && npm run format` | format the frontend with Prettier |
| `make up` / `down` / `logs` / `ps` | Docker Compose stack |
| `make up-admin EMAIL=…` | create a super-admin inside the Compose stack (`ADMIN_PASSWORD` from your shell) |
| `make reset-db` | **deletes** the Compose database volume |

The binary takes the same subcommands: `bin/api serve | migrate up|down|status | create-admin -email … | seed`.

## Deploying

The Compose file runs as-is on a server, but you need to do the following yourself:

1. Put HTTPS in front of the `web` container, using a reverse proxy or load balancer. Add HSTS there.
2. Set `APP_ENV=production` and `COOKIE_SECURE=true`. In production mode the API refuses to start with insecure cookies.
3. Set `APP_BASE_URL` and `APP_ALLOWED_ORIGINS` to the public URL.
4. Set `MAIL_DRIVER=smtp` and the `SMTP_*` settings so password-reset emails are delivered.
5. Back up the `db-data` volume.

See ARCHITECTURE.md §13 for the full checklist.

### Deploying without Docker

You need PostgreSQL 14+, a reverse proxy (nginx is shown), and a Linux server. Build on any machine with Go and Node.

```bash
# 1. Build
make build-api                                    # → backend/bin/api (static binary, migrations embedded)
VITE_APP_NAME="My App" make build-web             # → frontend/dist (VITE_* values are baked in; no secrets)

# 2. Copy backend/bin/api to the server (e.g. /opt/app/api) and frontend/dist to /var/www/app
```

Configure the API with environment variables or a `.env` file in its working directory (full list: `backend/.env.example`). Minimum for production:

```bash
APP_ENV=production
HTTP_ADDR=127.0.0.1:8080
DATABASE_URL=postgres://app:PASSWORD@127.0.0.1:5432/boilerplate?sslmode=disable
APP_BASE_URL=https://app.example.com
APP_ALLOWED_ORIGINS=https://app.example.com
COOKIE_SECURE=true
TRUST_PROXY=true            # only because nginx overwrites the forwarding headers
MAIL_DRIVER=smtp            # plus SMTP_HOST / SMTP_PORT / SMTP_USER / SMTP_PASSWORD / MAIL_FROM
```

```bash
# 3. First run
/opt/app/api migrate up
ADMIN_PASSWORD='choose-a-strong-one1' /opt/app/api create-admin -email you@example.com -username admin -name "Super Admin"
```

Run the API under systemd (`/etc/systemd/system/app-api.service`):

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

Then `systemctl enable --now app-api`. nginx serves the SPA and proxies `/api/` on the **same origin** (HTTPS terminated here, add HSTS):

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
        proxy_set_header X-Forwarded-For $remote_addr;   # overwrite, never append
        proxy_set_header X-Forwarded-Proto $scheme;
    }
    location /assets/ { add_header Cache-Control "public, max-age=31536000, immutable"; try_files $uri =404; }
    location /        { add_header Cache-Control "no-cache"; try_files $uri $uri/ /index.html; }
}
```

For the security headers (CSP etc.) copy the lines from `frontend/deploy/security-headers.conf` into the `/` and `/assets/` locations. To update later: rebuild, replace the binary and `dist`, run `api migrate up`, `systemctl restart app-api`.

## Adding a module

1. **Migration:** create the tables and insert a `menus` row (unique `slug`, `path`, `icon`, group, `sort_order`).
2. **Backend:** create a new package `internal/<module>` with `handler.go`, `service.go` and `store.go`. Register its routes in `internal/app/app.go`. Guard each route with `g.Require("<slug>", rbac.View|Create|Update|Delete)`. Document the routes in `api/openapi.yaml`; a test fails if you forget.
3. **Frontend:** create `src/features/<module>/` (`api.ts` with TanStack Query hooks, `schemas.ts` with zod, a page using `DataTable` + a form dialog + `ConfirmDialog`). Add a lazy route in `src/app/routes.tsx` at the menu's `path`, wrapped in `<RequirePermission menu="<slug>">`. Gate buttons with `usePermission()`. If the menu uses a new icon, add it to `src/lib/icons.ts`.
4. **Permissions:** grant the actions to roles in the permission matrix. Super-admin has every permission automatically.

## UI components

The UI uses [shadcn/ui](https://ui.shadcn.com) (style `radix-nova`, see `frontend/components.json`). Components live in `src/components/ui/` and are part of this repo. To add one:

```bash
cd frontend && npx shadcn@4.21.4 add <component>
```

If the CLI re-adds `@import "shadcn/tailwind.css"` to `index.css` or `shadcn` to `package.json`, remove them again. The same CSS is already vendored in `src/styles/shadcn.css`, and the CLI package pulls in dev-only dependencies with known advisories.

Brand colours and other design tokens are CSS variables in `frontend/src/index.css` (`:root` and `.dark`).

## License

[MIT](LICENSE)
