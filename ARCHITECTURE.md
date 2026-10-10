# Architecture

Status: **Complete (Phase 7 done)** — backend, frontend and Docker Compose implemented, security-reviewed and verified. See [SECURITY.md](SECURITY.md) and [FINAL_REPORT.md](FINAL_REPORT.md).

Companion docs: [FEATURE_PARITY.md](FEATURE_PARITY.md) (what the Laravel boilerplate does and what must be preserved).

---

## 1. Overview

```
Browser ── React SPA (Vite, TS, Tailwind, shadcn/ui)
   │  same-origin HTTP/JSON, cookie session
   ▼
/api/v1 ── Go API (net/http + chi)
   │  handlers → services → stores
   ▼
PostgreSQL (pgx pool, goose migrations)
```

- **Modular monolith.** One Go binary, one SPA, one database. No Redis, queues, or microservices — nothing in the Laravel boilerplate needs them (rate limiting and sessions live in Postgres).
- **Same-origin deployment.** Dev: Vite proxies `/api` to the Go server. Prod: a reverse proxy (nginx container in Compose) serves the SPA and forwards `/api` to the API. Consequence: no CORS, cookies stay `SameSite=Lax`, CSRF surface is small.
- **Backend is the source of truth** for authentication, authorization, and data integrity. The frontend only uses permissions to hide controls.

## 2. Technology and versions

Versions checked against the npm registry / Go module proxy on 2026-10-09 and against the local toolchain (Go 1.26.0, Node 22.17.1).

| Area | Choice | Version | Why / notes |
|---|---|---|---|
| Go | toolchain | **1.26** (`go 1.26.0` minimum, `toolchain go1.26.9` in go.mod) | goose v3.28 / x/crypto v0.57 need 1.26; 1.26.9 fixes 30 stdlib advisories that govulncheck reported for 1.26.0 (`GOTOOLCHAIN=auto` downloads it) |
| Router | `github.com/go-chi/chi/v5` | v5.3.2 | stdlib-compatible handlers + middleware |
| DB driver | `github.com/jackc/pgx/v5` (+ `pgxpool`) | v5.11.0 | native Postgres, no ORM |
| Migrations | `github.com/pressly/goose/v3` (as a library) | v3.28.0 | SQL files embedded in the binary; no extra CLI to install |
| Crypto | `golang.org/x/crypto` (argon2, bcrypt) | v0.57.0 | argon2id for new hashes; bcrypt verify only for imported Laravel hashes |
| UUID | `github.com/google/uuid` | v1.6.0 | UUIDv7 generated in Go (works on any PG ≥ 14) |
| Logging | `log/slog` (stdlib) | — | JSON in prod, text in dev |
| Config / validation | stdlib + small internal helpers | — | avoids validator/config frameworks |
| PostgreSQL | `postgres:18-alpine` in Compose | 18 | schema stays compatible with PG ≥ 14 |
| React | `react`, `react-dom` | 19.3.0 | |
| Build | `vite` + `@vitejs/plugin-react` | 8.3.4 / 6.1.2 | |
| Language | `typescript` | **6.0.3** (pinned) | TS 7.0 is out, but `typescript-eslint` 8.71 supports `<6.1` only |
| Styling | `tailwindcss` + `@tailwindcss/vite` | 4.3.3 | CSS-first tokens |
| Components | shadcn/ui (CLI `shadcn` 4.21.x), `lucide-react` 1.54, `sonner` 2.0.8 | — | components copied into repo, not a runtime dep |
| Routing | `react-router` | **7.18.4** (pinned) | v8.4 needs Node ≥ 22.22; local Node is 22.17.1. Upgrade path documented |
| Server state | `@tanstack/react-query` | 5.104.x | |
| Forms | `react-hook-form` 7.89 + `zod` 4.6 + `@hookform/resolvers` 5.9 | — | resolvers 5.x supports zod 4 |
| Tests (web) | `vitest` 5.0 + `@testing-library/react` 16.3 + `jsdom` **29.1** (pinned) | — | vitest 5 needs Node ≥ 22.12; jsdom 30 needs Node ≥ 22.22 |
| Lint / format | `eslint` 10 + `typescript-eslint` 8.71 + `eslint-plugin-react-hooks` 7.1 (incl. React Compiler rules), `prettier` 3.9 | — | generated `components/ui` excluded from Prettier |
| UI extras | `cn` (shadcn's class merger), `@fontsource-variable/geist`, `tw-animate-css` | — | added by the shadcn `radix-nova` preset; fonts are bundled, no CDN |

Node requirement: **≥ 22.12** (Vite/Vitest/ESLint). Recommended: Node 22.22+ or 24 LTS.

Deliberately not used: ORM, JWT, Redis, TanStack Table, MSW, state libraries (Redux/Zustand), CSS-in-JS.

## 3. Repository layout

```
go-react-boilerplate/
├── backend/
│   ├── cmd/api/                   # main.go (serve | migrate up|down|status), admin.go (create-admin, seed)
│   ├── internal/
│   │   ├── app/                   # wires stores/services/handlers into one http.Handler (+ integration tests)
│   │   ├── config/                # env parsing + validation (fail fast on boot), .env loader
│   │   ├── identity/              # Principal + client info in request context (shared, no deps)
│   │   ├── platform/
│   │   │   ├── database/          # pgxpool setup, InTx helper, goose migration runner
│   │   │   ├── httpx/             # JSON envelope, typed API errors, Decode, pagination/sort whitelist
│   │   │   ├── validate/          # field validators collecting per-field messages
│   │   │   ├── password/          # argon2id hash/verify, legacy bcrypt verify, policy
│   │   │   └── mailer/            # Mailer interface: SMTP + log implementations
│   │   ├── httpserver/            # middleware: request id, access log, recover, security headers,
│   │   │                          #   no-store, body limit, client info
│   │   ├── auth/                  # login/logout/me, sessions, throttle, active role, CSRF + session
│   │   │                          #   middleware, password reset, registration, impersonation
│   │   ├── rbac/                  # actions, permission store, Guard middleware (Require*)
│   │   ├── user/                  # user CRUD, status, role assignment, admin password set, guards
│   │   ├── role/                  # role CRUD + permission matrix
│   │   ├── menu/                  # menu CRUD, menu groups, tree, navigation builder
│   │   ├── profile/               # self-service profile + own password (single file)
│   │   ├── audit/                 # audit log writer (runs inside the caller's transaction)
│   │   └── testsupport/           # integration-test helpers (build tag `integration`)
│   ├── migrations/                # 0000N_name.sql (goose), embedded via embed.FS
│   ├── api/openapi.yaml           # API reference, served at /api/v1/openapi.yaml
│   ├── Dockerfile · go.mod
│   └── .env.example
├── frontend/
│   ├── src/
│   │   ├── main.tsx               # bootstraps router + providers
│   │   ├── index.css              # Tailwind + design tokens (CSS variables, light/dark)
│   │   ├── app/                   # routes.tsx (route tree + lazy pages), providers, query client
│   │   ├── components/ui/         # shadcn/ui components (generated, radix-nova style)
│   │   ├── components/            # DataTable+Pagination, PageHeader, ConfirmDialog, states, FormField,
│   │   │                          #   SearchInput, ThemeToggle, DynamicIcon
│   │   ├── layouts/               # AppLayout (header, breadcrumb, impersonation banner), AppSidebar,
│   │   │                          #   UserMenu, AuthLayout
│   │   ├── features/<module>/     # auth, dashboard, users, roles, menus, profile:
│   │   │                          #   api.ts (queries/mutations) · schemas.ts (zod) · pages · dialogs
│   │   ├── pages/                 # HomeRedirect, Forbidden, NotFound, RouteError
│   │   ├── hooks/                 # useTheme, useListParams (URL list state), useDebouncedValue, useIsMobile
│   │   ├── lib/                   # api client, query keys, icon registry, errors→form mapping, format
│   │   ├── styles/shadcn.css      # vendored shadcn base CSS (avoids the CLI as a dependency)
│   │   ├── types/api.ts           # API DTOs
│   │   └── test/                  # Vitest setup + render/mock helpers
│   ├── Dockerfile · deploy/ · package.json · vite.config.ts · eslint.config.js · components.json
│   └── .env.example
│   (frontend/deploy/nginx.conf + security-headers.conf: SPA + /api reverse proxy)
├── docker-compose.yml             # db + migrate + api + web; settings from ./.env (.env.example)
├── Makefile
├── README.md · ARCHITECTURE.md · FEATURE_PARITY.md
```

Feature packages are used instead of the reference `handler/ service/ repository/` top-level split: a new module lives in one folder, which is what someone extending the boilerplate needs. Each feature package keeps the same internal split:

| File | Responsibility |
|---|---|
| `handler.go` | HTTP: decode, call service, encode; `Routes(r chi.Router, g *rbac.Guard)` registers the module's routes with their permission guards. No SQL. |
| `service.go` | Validation, business rules, transactions, authorization rules beyond route-level checks, audit calls. |
| `store.go` | SQL via pgx on a `database.DBTX`; `WithTx(tx)` returns a store bound to a transaction. Domain structs live here too. |

Small modules may keep everything in one file (`profile/profile.go`).

### Dependency rules

1. `handler → service → store → pgx`. Handlers never touch the store directly.
2. Feature packages may import `platform/*`, `rbac`, `audit`. They must not import another feature's store; cross-feature needs go through that feature's exported service or a narrow interface defined by the consumer.
3. `platform/*` imports nothing from `internal/` feature packages.
4. `internal/app` is the only place that wires concrete types (manual DI, no container). Cycles are broken with consumer-defined interfaces: `user.SessionRevoker` and `profile.SessionRevoker` (implemented by `auth.Service`), `auth.Registrar` (implemented by `user.Service`).
5. Frontend: `features/*` may import `components`, `lib`, `hooks`, `types`; features do not import each other except via `features/auth` hooks (`useAuth`, `usePermission`).

## 4. API conventions

- Base path `/api/v1`. Plural kebab-case nouns. IDs are UUID strings.
- JSON only (`Content-Type: application/json`), body limit 1 MiB, request timeout 30 s (configurable), context propagated to every DB call.
- Field names `snake_case` in JSON.

### Response envelope

```jsonc
// 200/201
{ "data": { ... } }
// list
{ "data": [ ... ], "meta": { "page": 1, "per_page": 20, "total": 134 } }
// error
{ "error": { "code": "VALIDATION_ERROR", "message": "Data tidak valid.",
             "fields": { "email": "Email sudah digunakan." }, "request_id": "…" } }
```

| HTTP | `code` | When |
|---|---|---|
| 400 | `BAD_REQUEST` | malformed JSON, bad query params |
| 401 | `UNAUTHENTICATED` | no/expired/invalid session |
| 401 | `INVALID_CREDENTIALS` | login failed (wrong password, unknown/inactive/deleted user) — distinct code so the SPA does not treat it as an expired session |
| 403 | `FORBIDDEN` | missing permission or CSRF failure (`CSRF_FAILED`) |
| 403 | `ROLE_SELECTION_REQUIRED` | authenticated but no active role yet |
| 404 | `NOT_FOUND` | resource missing, soft-deleted, or malformed ID |
| 405 | `METHOD_NOT_ALLOWED` | |
| 413 | `PAYLOAD_TOO_LARGE` | body > 1 MiB |
| 409 | `CONFLICT` | unique violation, last-admin guard, menu has children |
| 422 | `VALIDATION_ERROR` | field errors in `fields` |
| 429 | `TOO_MANY_REQUESTS` | login throttle; `Retry-After` header set |
| 500 | `INTERNAL_ERROR` | anything else; details only in server log with `request_id` |

Messages are Indonesian by default (matches the source app) and live in one place per package so they can be swapped.

### Lists

`?page=1&per_page=20&q=search&sort=created_at&order=desc` plus module-specific filters (e.g. `is_active`, `role_id`). `per_page` capped at 100. `sort` is whitelisted per endpoint and mapped to a column name — never interpolated from input.

### Endpoint map

| Method & path | Permission (menu slug : action) |
|---|---|
| `GET /auth/options` | public — `{app_name, registration_enabled}` for the login screen |
| `POST /auth/login` | public (throttled) |
| `POST /auth/logout` | session |
| `GET /auth/me` | session (no active role needed) |
| `PUT /auth/active-role` | session; role must belong to the user |
| `POST /auth/forgot-password`, `POST /auth/reset-password` | public (throttled) |
| `POST /auth/register` | public, only if `AUTH_REGISTRATION_ENABLED=true` |
| `POST /auth/impersonation/stop` | session that is impersonating |
| `GET /navigation` | active role |
| `GET /users` · `GET /users/{id}` | `users:view` |
| `POST /users` | `users:create` |
| `PATCH /users/{id}` · `PATCH /users/{id}/status` · `PUT /users/{id}/roles` · `PUT /users/{id}/password` | `users:update` |
| `DELETE /users/{id}` | `users:delete` |
| `POST /users/{id}/impersonate` | super-admin only |
| `GET /roles` · `GET /roles/{id}` · `GET /roles/{id}/permissions` | `roles:view` |
| `POST /roles` | `roles:create` |
| `PATCH /roles/{id}` · `PUT /roles/{id}/permissions` | `roles:update` |
| `DELETE /roles/{id}` | `roles:delete` |
| `GET /menus` · `GET /menus/{id}` · `GET /menu-groups` · `GET /actions` | `menus:view` |
| `POST /menus` | `menus:create` |
| `PATCH /menus/{id}` | `menus:update` |
| `DELETE /menus/{id}` | `menus:delete` |
| `GET /profile` · `PUT /profile` · `PUT /profile/password` | session + active role (own account only, never takes a user ID; not tied to the `profile` menu permission, which only controls the sidebar entry) |
| `GET /healthz` · `GET /readyz` | public, outside `/api/v1` |

Slugs are renamed from the Laravel originals to English (`pengguna→users`, `otoritas→roles`, `manajemen_menu→menus`, `beranda→dashboard`); display names stay Indonesian.

## 5. Authentication

- **Server-side sessions.** On login the server generates 32 random bytes (crypto/rand), sends them base64url-encoded in cookie `sid` (`HttpOnly`, `SameSite=Lax`, `Path=/`, `Secure` when `APP_ENV=production`). Only `sha256(token)` is stored in `sessions.token_hash`, so a DB leak does not yield usable sessions.
- **Expiry.** Idle timeout `SESSION_IDLE_TIMEOUT` (default 120m, matches Laravel) slid on activity (`last_seen_at` updated at most once per minute) and absolute lifetime `SESSION_MAX_LIFETIME` (default 7d). Expired rows are rejected on read and purged by a background ticker (hourly) in the API process.
- **Login** accepts one `login` field (email if it contains `@` and parses, else username — same as Laravel), password verified with argon2id (constant-time). Unknown user, wrong password, inactive user, deleted user → same 401 `INVALID_CREDENTIALS` "Username/email atau kata sandi salah." A dummy hash verify runs for unknown users to keep timing flat.
- **Legacy hashes.** If `password_hash` starts with `$2y$`/`$2a$`/`$2b$` it is verified with bcrypt and transparently re-hashed to argon2id — lets projects import users from the Laravel DB.
- **Throttle.** `login_attempts(key, attempts, window_started_at, locked_until)`. Two counters per failed login: `login|<login>|<ip>` (5 → locked for `AUTH_LOCKOUT`, default 60s, Laravel parity) and `login-ip||<ip>` across all usernames (`AUTH_IP_MAX_ATTEMPTS`, default 20, against password spraying). 429 with `Retry-After`. Success clears only the per-login counter; the per-IP counter is never cleared by a success, so an attacker's own valid account cannot reset it. Login inputs over 1 KiB are rejected without hashing (no argon2 CPU abuse) and still counted. Forgot-password requests are throttled per IP; the reset email is sent asynchronously so response time does not reveal whether the address exists.
- **Active role.** Stored on the session row. Login response returns `{user, roles, active_role, requires_role_selection}`; a user with exactly one active role gets it auto-selected (Laravel behavior). `PUT /auth/active-role` verifies the role is assigned to the user, active, and not deleted (fixes D1).
- **CSRF.** Each session has a random `csrf_token` (stored as-is: it is useless without the `sid` cookie, and the server must be able to return it). `GET /auth/me` and login return it; the SPA keeps it in memory and sends `X-CSRF-Token` on POST/PUT/PATCH/DELETE. Middleware also rejects unsafe requests whose `Origin` (or `Referer`) is not in `APP_ALLOWED_ORIGINS`. Login/forgot/reset (no session yet) rely on the Origin check + SameSite.
- **Invalidation.** Logout deletes the session row. Password change/reset deletes all of the user's other sessions. Deactivation, soft delete, or removal of the user's roles deletes/clears affected sessions.
- **Impersonation.** The admin's session is replaced by a new session for the target with `impersonator_user_id` + `impersonator_role_id` set. Stop deletes it and issues a fresh session for the admin with the remembered active role (the old token cannot be restored because only its hash is stored). Rules: actor's active role is super-admin; target is active, not self, not a super-admin. Start/stop are audited, the UI shows a persistent banner. Replaces Laravel's master password (D2).
- **Password policy** (single policy everywhere, fixes D21): 8–72 chars, at least one letter and one digit. Admins set passwords explicitly; no shared default reset password (D12).
- **Nothing auth-related is stored in localStorage/sessionStorage.** localStorage is used only for theme and sidebar-collapse state.

## 6. Authorization (RBAC)

Model carried over from Laravel: **permission = (role, menu, action)**, evaluated against the session's **active role only**.

- `actions`: fixed reference rows keyed by code — `view`, `create`, `update`, `delete`, `download`.
- `role_permissions(role_id, menu_id, action)` composite PK.
- **Super-admin** = role with `is_super_admin = true`. It implicitly has every permission (no rows needed, so newly added menus are accessible immediately — difference from Laravel, where the seeder had to attach every row). Exactly one super-admin role is seeded and marked `is_system` (cannot be deleted, slug/flags immutable).
- **Route guard:** `perm.Require("users", rbac.Create)` middleware. Resolver loads the active role's permission set once per request (one indexed query) and caches it in the request context.
- **Service-level rules** (cannot be expressed as a route check):
  - Only a super-admin actor may grant or revoke the super-admin role, or modify/delete/deactivate/reset-password a user who holds it (fixes D6).
  - The last active, non-deleted user holding the super-admin role cannot be deleted, deactivated, or lose the role. Enforced in a transaction that locks the super-admin role row (`SELECT … FOR UPDATE`) to serialize concurrent changes (fixes D10).
  - Users cannot delete or deactivate themselves.
  - Inactive menus deny access the same as missing permission.
- `GET /auth/me` returns `permissions: { "<menu_slug>": ["view","create",…] }` and `is_super_admin` for UI gating.

## 7. Menus and navigation

- `menu_groups(id, name, sort_order)` → sidebar section titles.
- `menus(id, group_id, parent_id, name, slug, path, icon, sort_order, is_active, deleted_at)`.
  - `slug` unique among non-deleted rows, `^[a-z][a-z0-9_]*$`, **immutable after create** (it is the permission key referenced from code — fixes D13/D14).
  - Max depth 2: a parent must itself have `parent_id IS NULL` (checked in service).
  - Main menu requires `group_id` + `icon`; child requires `parent_id` + `path` (Laravel rule kept). A main menu may also have a `path` (leaf).
  - Delete is refused with 409 while active children exist; deleting a menu soft-deletes it and removes its `role_permissions` rows.
- `icon` stores a **Lucide icon name** (e.g. `users`, `shield`). The frontend keeps a curated registry (`lib/icons.ts`) so the bundle doesn't include all Lucide icons; unknown names fall back to a default icon. The menu form's icon picker lists the registry.
- `GET /navigation` returns groups → menus → children, only active, non-deleted menus the active role can `view` (children filtered too, fixes D8), ordered by `sort_order`, empty groups dropped.
- Adding a module: (1) migration inserting its menu row, (2) backend routes guarded with that slug, (3) React route whose path equals the menu `path`, guarded with the same slug. Documented as a checklist in README.

## 8. Database schema

All tables have `created_at timestamptz NOT NULL DEFAULT now()`; mutable ones have `updated_at`. Soft delete (`deleted_at`) only where Laravel had it and a restore/audit trail makes sense: `users`, `roles`, `menus`.

```sql
users (
  id uuid PK, name text NOT NULL, username text NOT NULL, email text NOT NULL,
  password_hash text NOT NULL, is_active boolean NOT NULL DEFAULT true,
  last_login_at timestamptz, created_at, updated_at, deleted_at
)
  UNIQUE (lower(username)) WHERE deleted_at IS NULL
  UNIQUE (lower(email))    WHERE deleted_at IS NULL

roles (
  id uuid PK, name text NOT NULL, slug text NOT NULL,
  is_super_admin boolean NOT NULL DEFAULT false, is_system boolean NOT NULL DEFAULT false,
  is_active boolean NOT NULL DEFAULT true, created_at, updated_at, deleted_at
)
  UNIQUE (lower(name)) WHERE deleted_at IS NULL ; UNIQUE (slug) WHERE deleted_at IS NULL
  UNIQUE (is_super_admin) WHERE is_super_admin   -- one super-admin role

user_roles (user_id uuid FK→users ON DELETE CASCADE, role_id uuid FK→roles ON DELETE CASCADE,
            created_at, PRIMARY KEY (user_id, role_id))
  INDEX (role_id)

menu_groups (id uuid PK, name text NOT NULL, sort_order int NOT NULL DEFAULT 0, created_at, updated_at)

menus (
  id uuid PK, group_id uuid FK→menu_groups NULL, parent_id uuid FK→menus NULL,
  name text NOT NULL, slug text NOT NULL, path text NULL, icon text NULL,
  sort_order int NOT NULL DEFAULT 0, is_active boolean NOT NULL DEFAULT true,
  created_at, updated_at, deleted_at,
  CHECK (parent_id IS NOT NULL OR group_id IS NOT NULL)
)
  UNIQUE (slug) WHERE deleted_at IS NULL ; INDEX (parent_id) ; INDEX (group_id, sort_order)

actions (code text PK, label text NOT NULL, sort_order int NOT NULL)

role_permissions (role_id uuid FK→roles ON DELETE CASCADE, menu_id uuid FK→menus ON DELETE CASCADE,
                  action text FK→actions, created_at, PRIMARY KEY (role_id, menu_id, action))

sessions (
  token_hash bytea PK, user_id uuid FK→users ON DELETE CASCADE NOT NULL,
  active_role_id uuid FK→roles ON DELETE SET NULL,
  impersonator_user_id uuid FK→users ON DELETE CASCADE NULL,
  impersonator_role_id uuid FK→roles ON DELETE SET NULL,   -- restored on stop
  csrf_token text NOT NULL, ip inet, user_agent text,
  created_at, last_seen_at timestamptz NOT NULL, expires_at timestamptz NOT NULL
)
  INDEX (user_id) ; INDEX (expires_at)

login_attempts (key text PK, attempts int NOT NULL, window_started_at timestamptz NOT NULL,
                locked_until timestamptz NULL)

password_reset_tokens (token_hash bytea PK, user_id uuid FK→users ON DELETE CASCADE,
                       expires_at timestamptz NOT NULL, used_at timestamptz NULL, created_at)

user_profiles (user_id uuid PK FK→users ON DELETE CASCADE, phone text, birth_place text,
               birth_date date, address text, created_at, updated_at)

audit_logs (id bigint GENERATED ALWAYS AS IDENTITY PK, actor_user_id uuid NULL,
            impersonator_user_id uuid NULL, action text NOT NULL, entity_type text NOT NULL,
            entity_id text NULL, metadata jsonb NOT NULL DEFAULT '{}', ip inet, user_agent text,
            created_at)
  INDEX (entity_type, entity_id) ; INDEX (actor_user_id) ; INDEX (created_at)
```

Notes:
- Booleans replace Laravel's `tinyint`; UUIDv7 for every entity (generated in Go, time-ordered, index friendly).
- `user_profiles` keeps only the biodata fields the Laravel UI actually used; name and email live in `users` (Laravel synced them across both tables — here there is one copy).
- Laravel tables not carried over: `cache*`, `jobs*`, `failed_jobs` (no queue/cache needed), `audits` (replaced by `audit_logs`).

## 9. Migrations and seeding

- goose SQL migrations in `backend/migrations`, embedded with `embed.FS`, run with `api migrate up|down|status`. Each file has `-- +goose Up` / `-- +goose Down`. Never edit an applied migration; add a new one.
- **Reference data lives in migrations** (needed in every environment): actions, the two system roles (`super-admin`, `user`), menu groups, the core menus, and the default `user` role permissions (dashboard + profile view).
- **`api seed`** (dev only, refuses when `APP_ENV=production`): idempotent sample users.
- **`api create-admin`** (any env): creates the first super-admin from `--email --username --name` and reads the password from `ADMIN_PASSWORD` env or an interactive prompt. No credentials are hard-coded anywhere (fixes D3/D4).
- Migrations run explicitly (Makefile/Compose service), not silently at API startup; `MIGRATE_ON_START=true` exists for single-node convenience.

## 10. Configuration

All via environment (`backend/.env.example` documents each); `.env` is git-ignored. Loaded once into a typed `config.Config`; invalid/missing required values abort startup with a clear message.

| Variable | Default | Purpose |
|---|---|---|
| `APP_ENV` | `development` | `development` / `test` / `production` |
| `APP_NAME` | `Boilerplate` | mail subject, logs |
| `HTTP_ADDR` | `:8080` | listen address |
| `APP_BASE_URL` | `http://localhost:5173` | links in emails |
| `APP_ALLOWED_ORIGINS` | `http://localhost:5173` | Origin check for unsafe methods |
| `DATABASE_URL` | — (required) | pgx DSN |
| `DB_MAX_CONNS` | `10` | pool size |
| `SESSION_IDLE_TIMEOUT` | `120m` | sliding expiry |
| `SESSION_MAX_LIFETIME` | `168h` | absolute expiry |
| `COOKIE_SECURE` | `true` in production | override for local HTTPS testing |
| `AUTH_MAX_ATTEMPTS` / `AUTH_LOCKOUT` | `5` / `60s` | login throttle per (login, IP) |
| `AUTH_IP_MAX_ATTEMPTS` | `20` | failed logins per IP across all usernames (must be ≥ `AUTH_MAX_ATTEMPTS`) |
| `AUTH_REGISTRATION_ENABLED` | `false` | public sign-up |
| `AUTH_DEFAULT_ROLE` | `user` | role slug for self-registered users |
| `REQUEST_TIMEOUT` | `30s` | per-request context deadline |
| `LOG_LEVEL` / `LOG_FORMAT` | `info` / `text` (dev), `json` (prod) | slog |
| `MAIL_DRIVER` | `log` | `log` or `smtp`; in production the `log` driver redacts message bodies (reset links) |
| `SMTP_HOST` `SMTP_PORT` `SMTP_USER` `SMTP_PASSWORD` `MAIL_FROM` | — | when `MAIL_DRIVER=smtp` |
| `MIGRATE_ON_START` | `false` | |
| `TRUST_PROXY` | `false` | trust `X-Forwarded-For` / `X-Real-IP` (set `true` behind nginx) |

Frontend `.env.example`: `VITE_APP_NAME`, `VITE_API_BASE` (default `/api/v1`). No secrets in the frontend.

## 11. Cross-cutting backend concerns

- **Middleware order:** RequestID → TrustedProxyIP (only if `TRUST_PROXY=true`; uses **only** `X-Real-IP`, ignores `X-Forwarded-For`/`True-Client-IP` — chi's `RealIP` is deprecated as spoofable) → structured access log → Recoverer → security headers → body limit → timeout → session loader → CSRF → route-level `RequirePermission`.
- **Security headers (API):** `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: strict-origin-when-cross-origin`, `Content-Security-Policy: default-src 'none'; frame-ancestors 'none'`, `Cache-Control: no-store` on `/api`, HSTS in production. The nginx config sets the SPA's CSP.
- **Errors:** handlers return `error`; one `httpx.WriteError` maps typed errors (`ErrNotFound`, `ValidationError`, `ConflictError`, `ForbiddenError`) to the envelope. pgx unique violations (`23505`) map to 409/422 with the field name. Unknown errors → 500, logged with request ID and stack.
- **Transactions:** `database.InTx(ctx, pool, func(tx pgx.Tx) error)` for multi-step operations (user create + roles, permission matrix replace, menu delete + permissions, last-admin checks).
- **SQL:** parameterized only; dynamic `ORDER BY` uses a whitelist map.
- **Logging:** `slog` with `request_id`, `user_id`, `method`, `path`, `status`, `duration`. Never log passwords, tokens, cookies, or full request bodies.
- **Graceful shutdown:** SIGINT/SIGTERM → `http.Server.Shutdown` with 15 s timeout → close pool.
- **Health:** `/healthz` (process up), `/readyz` (DB ping ≤ 2 s).
- **API docs:** `backend/api/openapi.yaml`, served at `/api/v1/openapi.yaml`, kept in sync per endpoint.

## 12. Frontend architecture

- **Routing:** `createBrowserRouter` (route tree in `app/routes.tsx`). Pages are `React.lazy` chunks rendered inside `<Suspense>` in the layouts; vendor code is split into `vendor-react`, `vendor-ui`, `vendor-data` chunks (≈ 235 kB gzip on first load). Public: `/login`, `/forgot-password`, `/reset-password`, `/register` (only if enabled). Authenticated: `/choose-role`, then `AppLayout` children: `/dashboard`, `/users`, `/roles`, `/roles/:id/permissions`, `/menus`, `/profile`. Plus `/403` and `*` (404).
- **Guards:** `RequireAuth` (uses `useMe()` query; 401 → `/login?next=…`), `RequireActiveRole` (→ `/choose-role`), `RequirePermission slug action` (→ 403 page). These are UX only.
- **API client:** thin `fetch` wrapper (`credentials: 'same-origin'`, JSON, CSRF header from memory, typed `ApiError` with `code/fields`). Global handling: `UNAUTHENTICATED` → identity cleared, guard redirects to login; `ROLE_SELECTION_REQUIRED` → "me" refetched, guard redirects to choose-role. `INVALID_CREDENTIALS` is handled by the login form only.
- **Server state:** TanStack Query, query keys centralized (`lib/query-keys.ts`). Mutations invalidate affected keys. Login, role switch and impersonation call `resetSession()`, which sets the new identity and **resets** (not removes) every other query so mounted components (sidebar) refetch for the new identity; removing would leave them showing the previous user's data — this was a real bug found in browser testing. After an explicit logout the guard redirects to a plain `/login` (no `next`), so the next person does not land on the previous user's page.
- **List state:** page, search, sort and filters live in the URL (`useListParams`), so lists are linkable and survive reloads; search is debounced.
- **Forms:** RHF + zod schema per form (`features/*/schemas.ts`). Server `fields` errors mapped back with `setError`. Client validation mirrors server rules for UX; the server remains authoritative.
- **CRUD pattern:** each module = list page (`DataTable` with search, sort, pagination, empty/error/loading states) + form dialog/sheet + `ConfirmDialog` for destructive actions + `sonner` toasts. A new module copies this pattern.
- **Theme:** shadcn CSS variables in `src/index.css`; light/dark/system via `class="dark"` on `<html>`; choice persisted in localStorage; system mode follows `prefers-color-scheme`. Default primary colour `#24695c` (Laravel/viho primary). Changing brand = editing tokens in one file.
- **Sidebar:** shadcn `Sidebar` built from `/navigation`; groups → items → collapsible sub-items; active state from the router location (`isActivePath`); collapsible to an icon rail (state in localStorage instead of the generated cookie), off-canvas sheet below 768 px; skeleton while loading, message when empty. Menu icons are Lucide names resolved through `lib/icons.ts` (unknown → neutral fallback).
- **Landing page:** `/` goes to `/dashboard` when allowed, otherwise to the first menu the role can open, otherwise an empty state.
- **Permission-aware UI:** buttons/menus/switches appear only when `usePermission()` allows; the user form disables the super-admin role for non-super-admins; the matrix disables grants the editor does not hold (mirrors the API's anti-escalation rule). Menus with submenus only offer "view" in the matrix (same as Laravel).
- **Text:** Indonesian UI copy, collected per feature so it can be swapped; no i18n library yet.

## 13. Development and deployment workflow

- **Local development (verified from a clean copy of the repo):** any PostgreSQL ≥ 14 (DBngin/Postgres.app/Compose) → `cp backend/.env.example backend/.env` → `make migrate` → `ADMIN_PASSWORD=… make create-admin EMAIL=…` → `make dev-api` (Go on :8080). Frontend: `cp frontend/.env.example frontend/.env` → `make install-web` → `make dev-web` (Vite on :5173 with `/api` proxy). Hot reload is left to the developer (no `air` dependency).
- **Compose full stack (verified):** `docker compose up -d --build` (or `make up`) with settings from the root `.env`:

  | Service | Image | Notes |
  |---|---|---|
  | `db` | `postgres:18-alpine` | named volume `db-data` mounted at `/var/lib/postgresql` (PG 18 layout); `pg_isready` healthcheck; host port `127.0.0.1:5433` only |
  | `migrate` | `boilerplate-api:local` | one-shot `migrate up`; idempotent ("no pending migrations" on restart) |
  | `api` | `boilerplate-api:local` (≈ 23 MB) | multi-stage `golang:1.26-alpine` → `distroless/static-debian12:nonroot`; healthcheck `api healthcheck` (probes `/readyz`, no shell needed); not published, only reachable through `web`; `TRUST_PROXY=true` |
  | `web` | `boilerplate-web:local` (≈ 92 MB) | `node:24-alpine` build → `nginxinc/nginx-unprivileged:1.30-alpine` (non-root, port 8080); serves `dist`, SPA fallback, proxies `/api/` to `api:8080` |

  Start order is enforced with `depends_on` conditions: db healthy → migrate completed → api healthy → web.
- **nginx (`frontend/deploy/`):** CSP `default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; …; frame-ancestors 'none'` (the theme bootstrap is the external `public/theme-init.js` so no inline script is needed; inline *styles* are required by Radix positioning), `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`, `Permissions-Policy`, `Cross-Origin-Opener-Policy`; hashed `/assets/` cached for a year, `index.html` `no-cache`; gzip. Forwarding headers are **overwritten** (`X-Real-IP`/`X-Forwarded-For` = peer address, `True-Client-IP` cleared) so clients cannot spoof their IP to dodge the login throttle — verified: six failed logins with rotating spoofed headers were still locked on the 5th.
- **Production checklist:**
  1. Put a TLS-terminating proxy or load balancer in front of `web`, and add HSTS there.
  2. If that proxy forwards client IPs, configure nginx `real_ip` with the proxy's address.
  3. Set `APP_ENV=production` and `COOKIE_SECURE=true`. With `production`, the API refuses to start with insecure cookies.
  4. Set `APP_BASE_URL` and `APP_ALLOWED_ORIGINS` to the public URL.
  5. Use a strong, URL-safe `POSTGRES_PASSWORD`.
  6. Set `MAIL_DRIVER=smtp` and the SMTP settings so password reset can send email.
  7. Back up the `db-data` volume.
  8. Create the first admin with `make up-admin EMAIL=…`.
  9. Keep `AUTH_REGISTRATION_ENABLED=false` unless public sign-up is wanted.
- **Current machine:** PostgreSQL 18.4 via DBngin on 127.0.0.1:5432 (databases `go_boilerplate`, `go_boilerplate_test`) for local development; Docker Desktop 29.8 for the Compose stack (DB on 5433, web on 8080).

## 14. Testing strategy

| Level | Tooling | Scope |
|---|---|---|
| Go unit | `go test` | password hashing/verify/policy, validators, login identifier parsing, permission resolver, pagination/sort whitelist, error mapping |
| Go integration | `go test -tags=integration` with `TEST_DATABASE_URL` (Compose db); each test runs migrations into a fresh schema | stores, services (last-admin guard, role-grant guard, matrix replace, menu delete rules), full HTTP flows through the real router: login success/failure/inactive/throttle, session expiry, logout, CSRF, active-role validation, 401/403 on every protected route, privilege-escalation attempts, navigation filtering |
| Frontend unit/component | Vitest + Testing Library + jsdom; `fetch` stubbed per test (`test/utils.tsx` → `mockApi`), full route tree rendered with `renderApp` | API client (CSRF only on unsafe methods, error envelope, 401/role handlers, network errors), helpers (`safeNext`, `isActivePath`, icons), guards and redirects (`next`, open-redirect rejection), login flow, role choice, session expiry, access-denied/404, impersonation banner, users page (permission gating, URL search/sort, form + server field errors, super-admin role disabled, delete confirmation), permission matrix (dirty state, payload, read-only, anti-escalation), menu form rules |
| Build checks | `gofmt -l`, `go vet`, `go build`, `tsc --noEmit`, `eslint`, `vite build` | CI-equivalent `make check` |
| Smoke | `docker compose up` + curl health/login | where Docker is available |

Integration tests skip (with a clear message) when `TEST_DATABASE_URL` is unset, so `go test ./...` always runs offline. Each integration test creates its own schema (`search_path`), runs all migrations, and drops the schema afterwards, so tests can run in parallel against one database without touching dev data.

**Final status (Phase 7):** backend 72 tests (38 run without a database, the rest are PostgreSQL integration tests) pass with `-race`; statement coverage 83.8% across `internal/...`. Frontend 54 tests pass; statement coverage 86.3% (excluding generated shadcn components). `staticcheck`, `go vet`, `gofmt`, ESLint (incl. React Compiler rules), `tsc`, Prettier: clean. `govulncheck`: 0 reachable vulnerabilities; `npm audit`: 0.

**Frontend status (Phase 4):** 35 tests pass. A mutation check (making `usePermission` always return true) makes 3 of them fail, so the authorization tests are not vacuous. Verified manually in Chrome against the real API: login (validation, wrong password, success), dashboard, dynamic sidebar, users list/search/create (client + server errors), impersonation start/stop, role choice, permission matrix save/revert, menu create/delete with live sidebar update, profile, logout, access-denied page, light/dark theme, 390 px layout with mobile sidebar sheet.

**Backend status (Phase 3):** 50 tests (unit + integration) pass with `-race`, including `TestOpenAPICoversAllRoutes`, which fails when a route is missing from `api/openapi.yaml`, and `TestMigrationsRollBackAndReapply`.

## 15. Decision log

| # | Decision | Alternatives considered | Reason |
|---|---|---|---|
| A1 | Modular monolith, feature packages | layer-first folders; microservices | one folder per module eases extension; scale doesn't justify services |
| A2 | Server sessions in Postgres, opaque cookie | JWT in cookie/localStorage | instant revocation, matches Laravel, no token in JS |
| A3 | Keep menu × action permission model + active role | permission-string model (`users.create`) | it is the defining feature of the source boilerplate |
| A4 | Implicit super-admin | seed every permission row | new menus usable immediately, impossible to lock out super-admin |
| A5 | goose embedded | golang-migrate CLI, atlas | single binary, plain SQL, simple |
| A6 | Raw SQL + pgx, no sqlc | GORM, sqlc | explicit, no codegen step; sqlc can be added later |
| A7 | argon2id (+ legacy bcrypt verify) | bcrypt only | modern default; still imports Laravel hashes |
| A8 | Rate limiting in Postgres | in-memory, Redis | correct across multiple instances, no new service |
| A9 | Same-origin via proxy | CORS with credentials | simpler and safer cookie/CSRF story |
| A10 | Pin TypeScript 6.0.3, React Router 7.18.4 | TS 7, RR 8 | lint tooling / local Node compatibility; revisit when unblocked |
| A11 | Lucide icon names in DB | keep IcoFont classes | IcoFont tied to old template; Lucide ships with shadcn |
| A12 | Not ported: master password, shared reset password, Google OAuth, email verification, layout variants, file-upload helper, log viewer | — | security issues or unused in source (see FEATURE_PARITY §7) |

## 16. Known open points

- Google OAuth: left as an optional future module (OIDC with state + PKCE).
- Audit log has no UI in the first iteration (same as Laravel).
- Menu-group CRUD: read-only API at first (Laravel had no UI either).
- Super-admins can deactivate/delete core menus (e.g. `menus`); non-super roles then lose access to that screen. Not prevented yet — consider an `is_system` flag on menus if this becomes a problem.
- Login throttle keys on `login|IP`; a distributed attack across many IPs against one account is only slowed by the per-IP limit.
