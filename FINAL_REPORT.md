# Final Report — Go + React + PostgreSQL Boilerplate

This boilerplate re-implements the Laravel "viho" boilerplate (`laravel-viho` @ `8c431dd`) as a Go API, a React SPA and a PostgreSQL schema. Phases 1–7 are complete. Details: [ARCHITECTURE.md](ARCHITECTURE.md), [FEATURE_PARITY.md](FEATURE_PARITY.md), [SECURITY.md](SECURITY.md), [README.md](README.md).

## 1. Architecture and key decisions

The system is a modular monolith. The React SPA calls the Go REST API (`/api/v1`) from the same origin over cookie sessions, and the API stores everything in PostgreSQL. Vite proxies `/api` in development; nginx does it in Docker and production.

| Decision | Why |
|---|---|
| Feature packages (`auth`, `user`, `role`, `menu`, `profile`), each with handler → service → store on pgx, no ORM | A new module lives in one folder. The SQL is explicit and parameterized. |
| Server-side sessions: opaque cookie, only the hash stored, CSRF token, Origin check | Sessions can be revoked instantly. No token is ever exposed to JavaScript or stored in the browser. |
| Kept Laravel's menu × action permission model and the active role per session | This is the core of the original boilerplate. The backend enforces it on every route. |
| Super-admin has every permission implicitly; system roles are protected | New menus are usable immediately, and the system cannot be locked out. |
| goose migrations embedded in the binary; reference data in migrations; first admin via `create-admin` | One binary does everything, and no credentials are hard-coded. |
| argon2id, with legacy bcrypt hashes verified and re-hashed | Modern hashing that can still import Laravel users. |
| Login throttles kept in PostgreSQL (per login+IP and per IP) | Works across multiple API instances without Redis. |
| Not ported: master password, shared reset password, Google OAuth, email verification, layout variants | These were insecure or unused in the source. See FEATURE_PARITY §7. |

## 2. Directory structure

```
go-react-boilerplate/
├── backend/
│   ├── cmd/api/                 serve | migrate | create-admin | seed | healthcheck
│   ├── internal/
│   │   ├── app/                 wiring + integration tests
│   │   ├── auth/ rbac/ user/ role/ menu/ profile/ audit/
│   │   ├── config/ identity/ httpserver/ testsupport/
│   │   └── platform/            database, httpx, validate, password, mailer
│   ├── migrations/              00001–00005 (goose, embedded)
│   ├── api/openapi.yaml         API reference (served, test-enforced)
│   ├── Dockerfile · .env.example · go.mod
├── frontend/
│   ├── src/                     app/ components/ features/ layouts/ pages/ hooks/ lib/ styles/ types/ test/
│   ├── deploy/                  nginx.conf + security-headers.conf
│   ├── public/                  favicon.svg, theme-init.js
│   ├── Dockerfile · .env.example · package.json · vite.config.ts · eslint.config.js
├── docker-compose.yml · .env.example · Makefile
└── README.md · ARCHITECTURE.md · FEATURE_PARITY.md · SECURITY.md · FINAL_REPORT.md
```

The codebase is about 7.7k lines of Go and 5.6k lines of TypeScript, excluding generated shadcn components.

## 3. Features implemented

- **Auth:**
  - Login with username or email.
  - Logout; idle and absolute session expiry.
  - Brute-force limits per login and per IP.
  - CSRF protection.
  - Active-role selection and switching.
  - Forgot and reset password.
  - Optional self-registration.
  - Impersonation with an audit trail.
- **Users:** search, filters, sort and pagination. Create, edit, activate or deactivate, assign roles, set a password, soft delete. Last-admin and anti-escalation guards.
- **Roles and permissions:** role CRUD with immutable slugs and protected system roles. A permission matrix (menu × view/create/update/delete/download) that is read-only for super-admins and cannot grant beyond the editor's own permissions.
- **Menus:** CRUD with main/child rules, at most two levels, a unique immutable slug, an icon picker and a delete guard. The sidebar is filtered by the active role.
- **Profile:** biodata and changing one's own password. A password change signs out the user's other sessions.
- **UI:**
  - shadcn/ui layout with a collapsible sidebar and a mobile sheet.
  - Breadcrumbs, a user menu and an impersonation banner.
  - Light, dark and system themes. Brand color `#24695c`, defined as tokens.
  - Pages for 403, 404, route errors, loading, empty and error states.
  - Toasts and confirmation dialogs.
  - Permission-aware controls.
  - List state kept in the URL.
- **Platform:**
  - Health and readiness endpoints.
  - Structured JSON logs with request IDs.
  - Unified error envelope.
  - OpenAPI document.
  - Audit log.
  - Graceful shutdown.
  - Docker Compose stack.

## 4. Differences from the Laravel boilerplate

- **Removed:** master password, shared default reset password, Google OAuth, email verification, the viho layout variants and live customizer, the IcoFont icon set (replaced by curated Lucide names), log-viewer, PDF and Excel libraries, and the upload helper (it was unused).
- **Changed:**
  - Menu and role slugs are in English: `dashboard`, `users`, `roles`, `menus`, `profile`. Labels stay Indonesian.
  - Action IDs became action codes.
  - The super-admin role has every permission implicitly.
  - One password policy applies everywhere.
  - Profile data is stored once, not duplicated into the users table.
  - The profile page is available to every user, not gated by a menu permission.
- **Added:**
  - Last-admin and anti-escalation guards.
  - Session revocation.
  - Hashed session and reset tokens.
  - Per-IP brute-force limit.
  - Audit trail for impersonation.
  - Optional self-registration flag.
  - Docker deployment.
- **Fixed:** 22 defects found in the Laravel source (D1–D22), including three critical ones: anyone could select any role, the master password, and a hard-coded password for Google-created accounts.

## 5. Database

| Migration | Tables / content |
|---|---|
| 00001 | `users` (UUIDv7, soft delete, case-insensitive unique username and email), `roles`, `user_roles`, `user_profiles`, `set_updated_at()` trigger |
| 00002 | `menu_groups`, `menus` (two levels enforced in the service, partial unique slug), `actions`, `role_permissions` |
| 00003 | `sessions` (token hash, active role, impersonator), `login_attempts`, `password_reset_tokens` |
| 00004 | `audit_logs` |
| 00005 | Reference data: 5 actions, the `super-admin` and `user` roles, 4 menu groups, 5 core menus, and default grants for `user` |

All tables have primary keys, foreign keys with explicit `ON DELETE` rules, CHECK constraints and indexes. Every migration has a Down step; a test rolls back all of them and applies them again.

## 6. Commands

```bash
# Docker (full stack)
cp .env.example .env && make up
ADMIN_PASSWORD='…' make up-admin EMAIL=you@example.com     # http://localhost:8080

# Local development
cp backend/.env.example backend/.env && make migrate
ADMIN_PASSWORD='…' make create-admin EMAIL=you@example.com
make dev-api          # :8080
make install-web && make dev-web   # :5173

# Quality
make check            # gofmt, vet, build, Go unit + integration tests, ESLint, tsc, Prettier, Vitest, vite build
cd frontend && npm run test:coverage
```

## 7. Test results and checks

| Check | Result |
|---|---|
| Go tests (`-race`, PostgreSQL 18.4) | **72 passed**: 38 unit tests that need no database, plus 34 integration tests through the real HTTP stack. Statement coverage across `internal/...` is **83.8%**. |
| Frontend tests (Vitest) | **54 passed**. Statement coverage is **86.3%**, excluding generated shadcn components. A mutation check (forcing the permission check to always pass) makes 3 tests fail, so the authorization tests can catch a broken check. |
| gofmt, go vet, staticcheck | Clean, including the integration build tag. |
| ESLint (React Hooks and React Compiler rules), `tsc`, Prettier | Clean. |
| Production builds | Go static binary. Vite build with route-level code splitting and vendor chunks, about 235 kB gzip on first load. |
| govulncheck | 0 reachable vulnerabilities on Go 1.26.9. One informational module advisory for `x/crypto/openpgp`, which is not used. |
| npm audit | 0 vulnerabilities. |
| Migrations and seed | Fresh migrate, full rollback and re-apply, idempotent re-run, idempotent seed. Invalid seed and admin input is rejected. |
| Docker Compose | Images built (API about 23 MB, web about 92 MB). All services healthy, admin created, login through nginx in Chrome, CSP does not break the UI, data persists across restarts, migrations are idempotent, spoofed IP headers do not bypass the throttle, nginx rejects oversized bodies with 413. |
| Manual browser checks (Chrome) | Login, dashboard, sidebar, users CRUD, impersonation, role choice, permission matrix, menus, profile, logout, 403, both themes, a 390 px layout. |

**Not run or partly covered:**

- No real SMTP server was used. Delivery is tested against a fake SMTP server.
- No load or performance testing.
- No automated end-to-end browser suite; the browser flows were checked manually.
- These UI pieces have no automated test and were only checked by hand: role edit and delete dialogs, menu edit and delete dialogs, theme switching, the mobile sidebar sheet and the impersonation start/stop clicks.
- The Docker checks are manual; there is no CI pipeline.

## 8. Remaining limitations

- No MFA, no breached-password check, no CAPTCHA on registration or forgot-password, and no notification when an admin resets a password.
- Throttling is keyed on IP, so attacks from many IPs are only slowed down. The audit log has no UI, and its append-only property is not enforced by the database.
- Menu groups are read-only through the API. Super-admins can deactivate core menus.
- UI copy is Indonesian and hard-coded per feature; there is no i18n library.
- There is no CI workflow, and the folder is not yet a git repository.
- Two dependencies are pinned below their latest major version because of the local Node 22.17 and lint tooling: TypeScript 6.0 and React Router 7, plus jsdom 29.

## 9. Before deploying

1. Put HTTPS in front of the `web` container, using a reverse proxy or load balancer. Add HSTS there. If that proxy forwards client IPs, configure nginx `real_ip`.
2. In `.env`, set:
   - `APP_ENV=production` and `COOKIE_SECURE=true`
   - `APP_BASE_URL` and `APP_ALLOWED_ORIGINS` to the public URL
   - a strong, URL-safe `POSTGRES_PASSWORD`
   - `MAIL_DRIVER=smtp` and the `SMTP_*` settings
3. Run `make up`, then `ADMIN_PASSWORD=… make up-admin EMAIL=…`.
4. Back up the `db-data` volume. Restrict `DELETE` on `audit_logs` with database grants.
5. Keep `AUTH_REGISTRATION_ENABLED=false` unless public sign-up is wanted, together with a CAPTCHA and email verification.
6. Initialize git and add a CI job that runs `make check` with a PostgreSQL service.
