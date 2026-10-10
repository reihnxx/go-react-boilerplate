# Feature Parity — Laravel Boilerplate → Go + React + PostgreSQL

Status: **Complete** — parity implemented, security-reviewed and tested (see §10, SECURITY.md, FINAL_REPORT.md).
Reference implementation: internal Laravel "viho" boilerplate @ commit `8c431dd` ("impersonate, ignore anomali").
Method: every claim below was verified by reading the source. Items that could not be verified are listed in §8.
Secrets: `.env` was not read. Seed/default passwords found in source are referred to but deliberately not reproduced here.

---

## 1. Existing stack (as found)

| Layer | Found |
|---|---|
| Backend | Laravel 11 (PHP ^8.2), Blade server-rendered pages + jQuery AJAX endpoints |
| Auth | Session guard `web`, DB session driver, Breeze scaffolding (heavily modified), Socialite (Google) |
| DB | `DB_CONNECTION=sqlite` in `.env.example`; UUID (v7) PKs for `users`, `menus`; bigint PKs elsewhere |
| UI template | "viho" Bootstrap admin theme (`public/assets`), jQuery 3.5, DataTables (server-side via yajra), Select2, SweetAlert2, Toastr, LoadingOverlay, Feather + IcoFont icons |
| Vite/Tailwind/Alpine | Present from Breeze but **unused** by the real pages (layout loads theme assets directly) |
| Extra packages | `owen-it/laravel-auditing` (used on `User`, `Menu`), `opcodesio/log-viewer`, `mpdf`, `phpspreadsheet` (installed, **no usage found** in `app/`) |
| UI language | Indonesian (labels, validation messages, toasts) |

---

## 2. Feature-parity table

| Feature | Existing behavior (verified) | Relevant implementation | Required Go/React equivalent |
|---|---|---|---|
| **Login** | Single field accepts username **or** email (detected with `FILTER_VALIDATE_EMAIL`) + password. Rate limit: 5 failed attempts per `lower(username)|IP`, then lockout message with seconds remaining. Generic failure message. Session regenerated on success, redirect to `dashboard`. Does **not** check `is_active`. Soft-deleted users cannot log in (Eloquent default scope). | `app/Http/Requests/Auth/LoginRequest.php`, `app/Http/Controllers/Auth/AuthenticatedSessionController.php`, `resources/views/auth/login.blade.php` | `POST /api/v1/auth/login` with `{login, password}`; argon2id/bcrypt verify; DB-backed rate limiter keyed by login+IP (5 attempts / window); reject inactive + deleted users with the same generic error; issue new opaque session ID (rotate). |
| **Master password** | If `MASTER_PASSWORD` (bcrypt hash in env) matches, logs in **as any user**; only a warning log is written. | `LoginRequest::authenticate`, `config/auth.php` (`master_password`) | **Do not port.** Replaced by audited impersonation (below). Documented as an intentional difference. |
| **Logout** | Logs out, invalidates session, regenerates CSRF token, clears `sessionStorage.menu_data` client-side. | `AuthenticatedSessionController::destroy`, `layouts/app.blade.php` | `POST /api/v1/auth/logout` deletes session row + clears cookie; frontend clears TanStack Query cache. |
| **Session** | `database` driver, lifetime 120 min (sliding), `http_only=true`, `same_site=lax`, `secure` from env; HTTPS forced when `APP_ENV` ∈ {production, ministry, egov}. | `config/session.php`, `app/Providers/AppServiceProvider.php` | `sessions` table (hashed opaque token, user_id, active_role_id, ip, UA, expires_at, last_seen_at); idle + absolute expiry via env; cookie HttpOnly, SameSite=Lax, Secure in prod. |
| **Active role selection (multi-role)** | After login the RBAC middleware requires `session.role_id`; if missing → `choose-role`. 1 role → auto-select. >1 role → select page. Header shows "Ganti Otoritas" when multi-role. All permission checks use the **active role only**, not the union of roles. | `app/Http/Controllers/Auth/ChooseRoleController.php`, `app/Http/Middleware/RbacCheck.php`, `resources/views/auth/roles.blade.php` | Keep the concept: `active_role_id` on the session. `GET /api/v1/auth/me` returns user, roles, active role. `POST /api/v1/auth/active-role` **must validate the role belongs to the user** (see D1). React: role picker page + switch in profile menu. |
| **Registration** | Public `/register`: name, username (unique), email (unique, lowercase), password (min 8, confirmed). New user gets role id `2` ("User"), redirected to login. | `RegisteredUserController.php`, `Requests/Auth/RegisterRequest.php` | `POST /api/v1/auth/register` behind env flag `AUTH_REGISTRATION_ENABLED` (default `false`); default role resolved by slug from config, not by numeric ID. |
| **Forgot / reset password** | Email link (custom Indonesian mail, 60 min), Laravel password broker, new password min 8 + confirmed. | `PasswordResetLinkController.php`, `NewPasswordController.php`, `Notifications/ResetPasswordNotification.php`, `auth/forgot-password`, `auth/reset-password` views | `password_reset_tokens` table (hashed token, expiry); SMTP mailer interface with a log/console mailer for dev; response does not reveal whether email exists; all user sessions revoked after reset. |
| **Google sign-in** | Socialite stateless callback; matches by email (incl. soft-deleted → refused); unknown email → auto-creates user with role 2 and a **hardcoded password**. `register/google` routes point to methods that do not exist. | `Auth/GoogleController.php`, `routes/auth.php`, `config/services.php` | **Deferred / optional** (not part of core). If added later: OIDC with state+PKCE, no password for OAuth-created users. |
| **Email verification / confirm password** | Routes exist, but `User` does not implement `MustVerifyEmail` and the `verify-email` / `confirm-password` views are missing → effectively unused. | `routes/auth.php`, `Auth/*Verification*`, `Auth/ConfirmablePasswordController.php` | Not ported (not functional in source). |
| **Users (Pengguna)** | Server-side DataTable: index, email, name, active switch, actions; default order by `created_at desc`; global search + sort. Create (email, username, name, password min 5 + confirm; uniqueness rule commented out). Update (name, username only — email shown but ignored). Toggle `is_active`. Assign roles (checkbox list, `sync`). Admin "reset password" to a shared default password. Admin "change password" endpoint (strong policy: min 8, mixed case, number, symbol, `uncompromised`) — no UI found calling it. Soft delete. Impersonate button. Buttons shown per `permissions.update/delete`. | `Panel/UserController.php`, `Requests/Panel/User/*`, `routes/panel/user.php`, `contents/user/list.blade.php`, `public/js/page/pengguna/list.js` | `GET/POST /api/v1/users`, `GET/PATCH/DELETE /api/v1/users/{id}`, `PATCH /users/{id}/status`, `PUT /users/{id}/roles`, `POST /users/{id}/password`. Unique email+username (case-insensitive), consistent password policy, soft delete preserved. Guards: no self-delete/self-deactivate, never remove/deactivate the last active super-admin, only super-admin can grant super-admin. React table with search/sort/pagination, forms, confirm dialogs. |
| **Roles (Otoritas)** | List roles where `is_active=1`. Create/update name (unique); `slug_name = Str::snake(name)` and is **re-generated on rename**. Soft delete. "Hak Akses" button opens permission matrix by slug. | `Panel/OtoritasController.php`, `routes/panel/otoritas.php`, `contents/otoritas/list.blade.php`, `public/js/page/otoritas/list.js` | `GET/POST /api/v1/roles`, `PATCH/DELETE /api/v1/roles/{id}`. Slug immutable after create (or unique + system-protected); system roles (super-admin) cannot be deleted. |
| **Permissions (matrix)** | Permission = (role, menu, action). Actions are fixed rows: 1 view, 2 create, 3 update, 4 delete, 5 download. Matrix page: leaf menus show all actions; parent menus with children show only the first action (view). Submit deletes all rows for role and re-inserts in a transaction. | `OtoritasController::formPermission/submitPermission/getMenu`, `contents/otoritas/permissions.blade.php`, `Models/MenuRole.php`, `Models/Action.php` | `GET /api/v1/roles/{id}/permissions` (tree + granted actions), `PUT /api/v1/roles/{id}/permissions` (full replace in a tx). Keep menu × action model. React matrix with tree + per-action checkboxes. |
| **Authorization check** | Route middleware `rbac:{menu_slug},{action_id}` (default action 1). Looks up `menu_role` for active role + menu slug + action → 403 "Access Forbidden". Shares `permissions` map (view/create/update/delete/download → bool) and `activeSlug` to the view; JS hides buttons using that map. Same check via helper `rbacCheck()` in FormRequests/Blade. | `Http/Middleware/RbacCheck.php`, `Helper/rbac.php`, `bootstrap/app.php` | Chi middleware `RequirePermission("users", ActionCreate)` on every protected route. Backend is the only enforcer. `GET /auth/me` returns a permission map `{menu_slug: [actions]}` for UI hints only. Permissions cached per request, not across requests. |
| **Menu management** | Server-side DataTable of all menus (main/child, link, icon, group). Create: `main` requires icon + menu_group; `child` requires parent + link. `slug_name = Str::snake(name)` at creation, never updated. `menu_order = count(menus)+1`; children copy parent's order. `is_active` set to 1, no UI to change. Update name/link/icon/group/parent. Soft delete (children and permissions untouched). Icon picker over a hardcoded list of ~2,100 IcoFont classes. Only 2 levels (main → child) supported by the UI. | `Panel/MenusController.php`, `routes/panel/menu.php`, `contents/menu/list.blade.php`, `public/js/page/menu/list.js`, `Helper/icon.php`, `Models/Menu.php` | `GET/POST /api/v1/menus`, `PATCH/DELETE /api/v1/menus/{id}`, `GET /api/v1/menu-groups`. Unique immutable `slug`, explicit `sort_order` editable, `is_active` editable, max depth 2 enforced, delete blocked when children exist (or cascade explicitly). Icons: Lucide icon names (curated list) instead of IcoFont. |
| **Menu groups** | Table `menu_groups` (Beranda, Manajemen, Referensi, Data) used as sidebar section titles; no CRUD UI. | `Models/MenuGroup.php`, `MenuGroupSeeder.php` | Keep as seeded table + read endpoint; CRUD optional (small add). |
| **Sidebar** | `GET /menus` returns groups → menus where active role has action 1 (view), ordered by `menu_order`, with **all children unfiltered**. Cached in `sessionStorage` keyed by base URL + role name. Groups without rendered items are hidden. Active item from server-provided `activeSlug` + URL path match. Collapsible (toggle), auto-collapsed < 992 px, accordion submenus, mobile slide-in. | `UtilityController::loadMenu`, `public/js/menu.js`, `layouts/app.blade.php` | `GET /api/v1/navigation` returns only active menus the active role can **view**, children filtered too, groups without items dropped. React sidebar: groups, nested items, active route via React Router, collapsible (state persisted in localStorage), mobile sheet. Data via TanStack Query (invalidated on role switch). |
| **Theme** | viho template: primary `#24695c`, secondary `#ba895d`; 6 colour presets; dark mode (`dark-only` body class); layout variants (compact/modern/horizontal sidebar). Live customizer panel injected by `customizer.js` persists `color/primary/secondary/dark/page-wrapper` in `localStorage`. Header moon toggle writes a key nothing reads (not persisted). Fonts: Montserrat/Roboto/Rubik. | `public/assets/js/script.js`, `public/assets/js/theme-customizer/customizer.js`, `public/assets/js/config.js`, `layouts/components/app-styles.blade.php` | Tailwind + shadcn CSS variables; light/dark/system toggle persisted in `localStorage` (non-sensitive). Default primary = `#24695c`. Colour presets as optional token sets. Layout variants **not ported** (single sidebar layout). |
| **Layout / shell** | Header: logo, sidebar toggle, impersonation banner, fullscreen, bell (static), dark toggle, "Ganti Otoritas", logout. Sidebar header shows user name + active role. Page title + breadcrumb (Beranda › title). Page loader. Static footer. | `resources/views/layouts/app.blade.php`, `layouts/guest.blade.php` | App layout with header, sidebar, breadcrumb, user menu (profile, switch role, theme, logout), impersonation banner. Bell/fullscreen dropped unless needed. |
| **Dashboard (Beranda)** | "Selamat Datang, {name}!" card, guarded by `rbac:beranda,1`. (`contents/home/home.blade.php` is unused template demo.) | `routes/web.php`, `resources/views/dashboard.blade.php` | Simple welcome page guarded by `beranda:view`. |
| **Profil (self-service)** | Biodata form: nama_lengkap, tempat_lahir, tanggal_lahir, email, no_hp (regex `^(\+62|62|08)[0-9]{9,13}$`), alamat (≤500). Upserts `profil` and syncs `users.name/email`. Change own password (current check, min 6, confirm). Guarded by menu `profil` (store/change-password require *create* action). Table has more columns (gender, region, postcode, photo) with no UI. | `Panel/ProfilController.php`, `routes/panel/profil.php`, `contents/profil/list.blade.php`, `public/js/page/profil/list.js`, migration `create_profil_table` | `GET/PUT /api/v1/profile`, `PUT /api/v1/profile/password` — available to every authenticated user for **their own** account (always derived from session, never from request ID). Keep biodata fields actually used; unify password policy. |
| **Impersonation** | Super Admin (checked by session role **name** string) can log in as any user via GET `/impersonate/start/{id}`; stop restores admin and hard-codes `role_id=1`. Banner shows "Stop Impersonate". No RBAC middleware, no audit. | `ImpersonateController.php`, `routes/panel/impersonate.php`, `layouts/app.blade.php`, `AppServiceProvider` | `POST /api/v1/users/{id}/impersonate`, `POST /api/v1/auth/impersonate/stop`. Requires dedicated permission, cannot impersonate super-admins or self, session stores `impersonator_id`, all actions audited, banner in UI. |
| **Audit trail** | `owen-it/laravel-auditing` records create/update/delete for `User` and `Menu` into `audits`. No UI. | `Models/User.php`, `Models/Menu.php`, migration `create_audits_table` | `audit_logs` table written by services for user/role/menu/permission/auth events. Read UI optional. |
| **File upload helper** | `UploadFile::store` validates MIME (content vs client), extension whitelist, random filename. **Not used** anywhere in app. | `app/Library/UploadFile.php` | Not ported in core; noted as candidate shared utility. |
| **Health** | `/up` (Laravel default). | `bootstrap/app.php` | `GET /healthz` (liveness), `GET /readyz` (DB ping). |
| **Responses / errors** | Inconsistent JSON shapes: `{status,msg}`, `{status,message}`, `{success,message}`, `{error}`; 422 for validation; raw exception messages returned to client on 400/500. | All panel controllers | Single envelope: `{data, meta}` on success; `{error:{code,message,fields}}` on failure; never leak internal errors. |
| **Tables** | yajra DataTables server-side: `draw/start/length/search/order`, Indonesian labels, row index column. | Controllers `data()` methods, `public/js/main.js` (`App.options.dt`) | `?page&per_page&q&sort&order` → `{data, meta:{page,per_page,total}}`; reusable React `DataTable` (TanStack Table optional) with debounce search. |
| **UI feedback** | Toastr for success/error, SweetAlert2 for confirm dialogs (Indonesian "Ya/Batal"), LoadingOverlay on modals, inline `.error-*` field messages. | `public/js/main.js` | shadcn `sonner` toasts, `AlertDialog` confirms, form field errors via RHF + server field errors mapped. |
| **Tests** | Only stock Breeze tests (login posts `email`, factory lacks `username`) → stale; `RefreshDatabase` would run against the `.env` database because the in-memory sqlite override is commented out. **Not executed** during analysis (risk of wiping a dev DB). | `tests/`, `phpunit.xml` | Go unit + integration tests (real Postgres), Vitest + Testing Library on frontend. |
| **Deployment** | No Dockerfile / compose / CI in repo. README is the Laravel default. | — | Docker Compose (postgres + api + web), Makefile, README from clean checkout. |

---

## 3. Actual data model

```
users(id uuid pk, name, username uniq, email uniq, email_verified_at, password, remember_token,
      is_active tinyint default 1, timestamps, deleted_at)
roles(id bigint pk, name, slug_name, is_active tinyint default 1, timestamps, deleted_at)
user_role(user_id uuid, role_id int, timestamps)                 -- no PK, no FK, no unique
actions(id bigint pk, name)                                      -- view, create, update, delete, download
menu_groups(id bigint pk, name, timestamps, deleted_at)
menus(id uuid pk, menu_group_id int null, parent_id uuid null, name, slug_name, menu_order,
      link, icon, is_active tinyint default 0, timestamps, deleted_at)   -- slug not unique
menu_role(menu_id uuid, role_id int, action_id int, timestamps)  -- no PK, no FK, no unique
profil(id pk, user_id uuid, nama_lengkap, email uniq, no_hp, tempat_lahir, tanggal_lahir,
       jenis_kelamin enum, alamat, provinsi, kabupaten, kecamatan, kode_pos, foto_profil, timestamps)
audits(...)  sessions(...)  password_reset_tokens(...)  cache, cache_locks, jobs, job_batches, failed_jobs
```

Relationships: user ⟷ role (M:N via `user_role`); role ⟷ menu ⟷ action (ternary via `menu_role`); menu → menu_group; menu → parent menu (self, 2 levels in UI); user → profil (1:1, not enforced).

## 4. Seed data (behavior to reproduce)

- Actions: view, create, update, delete, download (IDs 1–5, the IDs are hard-coded in routes).
- Menu groups: Beranda, Manajemen, Referensi, Data.
- Menus: Beranda (`beranda`, /dashboard), Pengguna (`pengguna`, /users), Manajemen Menu (`manajemen_menu`), Otoritas (`otoritas`), Profil (`profil`).
- Roles: Super Admin (all menus × all actions), User (Beranda + Profil, all actions).
- Users: `admin` (roles Super Admin + User) and `user` (role User), both with weak hard-coded passwords in the seeder.
  → Go version: initial admin created from env (`SEED_ADMIN_EMAIL`, `SEED_ADMIN_PASSWORD`) or a CLI command; no hard-coded passwords.

## 5. Business rules to preserve

1. Login by username **or** email, one field.
2. Brute-force throttle: 5 attempts per login+IP, lockout message with remaining time.
3. Users may hold multiple roles; exactly one role is **active per session**; permissions are evaluated against the active role only; user can switch role without re-login.
4. Permission unit = (menu slug, action). Actions: view/create/update/delete/download.
5. Viewing a page requires `view`; create/update/delete endpoints require the matching action.
6. Sidebar shows only menus the active role can view, grouped by menu group, ordered by menu order.
7. A main menu needs icon + group; a child menu needs parent + link.
8. Role names unique; users/menus/roles are soft-deleted.
9. Permission matrix save is all-or-nothing (transaction, full replace).
10. Profile edit also syncs user display name and email.
11. Session idle lifetime 120 min; session ID rotated on login; invalidated on logout.
12. Indonesian UI copy by default.

## 6. Security constraints for the new implementation

- Backend enforces every permission; frontend hints only.
- Active role must be one of the user's (non-deleted, active) roles.
- Inactive or deleted users cannot log in; deactivating/deleting a user revokes their sessions.
- No master password, no shared default passwords, no hard-coded credentials.
- Never return raw exception messages; log them server-side with request ID.
- Escape all menu/user-provided strings in UI (React default; no `dangerouslySetInnerHTML`).
- CSRF: SameSite=Lax cookie + required custom header / double-submit token for unsafe methods.
- Last active super-admin cannot be deleted, deactivated, or stripped of the role; users cannot delete/deactivate themselves.
- Only a super-admin may grant the super-admin role or impersonate; super-admins cannot be impersonated.

## 7. Defects found in the Laravel source (do NOT replicate)

| # | Severity | Finding | Location |
|---|---|---|---|
| D1 | Critical | `POST choose-role` accepts any `role_id` (only `required`), never checks it belongs to the user → any logged-in user can activate Super Admin. | `ChooseRoleController::store`, `SelectRoleRequest` |
| D2 | Critical | Master password logs in as any account. | `LoginRequest::authenticate` |
| D3 | Critical | Google-created accounts receive a hard-coded password literal (also used by seeder) → anyone knowing it can log in to those accounts with username/password. | `GoogleController::handleGoogleCallback` |
| D4 | High | `.env.example` contains a commented line that looks like a real credential value; `config/app.php` has a hard-coded fallback for the default reset password. Should be rotated and removed. | `.env.example`, `config/app.php` |
| D5 | High | `is_active` is never checked at login → deactivated users can still sign in. | `LoginRequest` |
| D6 | High | Role assignment does not validate role IDs nor restrict granting Super Admin → any user with `pengguna:update` can escalate. | `UserController::updateRole` |
| D7 | High | Impersonation: no RBAC middleware, GET for state change, check by role **name** string, uses non-existent `User` attributes, `stop()` hard-codes `role_id=1`, can impersonate other admins, no audit. | `ImpersonateController` |
| D8 | Medium | Sidebar children not permission-filtered; inactive menus not filtered (backend routes still guarded, so this is information disclosure). | `UtilityController::loadMenu` |
| D9 | Medium | Sidebar builds HTML from menu name/icon via string templates → stored XSS for anyone able to edit menus. | `public/js/menu.js` |
| D10 | Medium | No protection against deleting/deactivating yourself or the last admin. | `UserController::switchStatus/delete` |
| D11 | Medium | User create/update has no uniqueness validation (commented out); DB errors and raw exception messages are returned to the client. | `UserCreateRequest`, `UserUpdateRequest`, all controllers |
| D12 | Medium | Admin "reset password" sets a shared, known default password. | `UserController::resetPassword` |
| D13 | Low | Role slug generated two ways (`Str::slug` in seeder → `super-admin`, `Str::snake` in controller → `super_admin`) and regenerated on rename. | `RoleSeeder`, `OtoritasController` |
| D14 | Low | Menu slug not unique, derived from name; route middleware hard-codes slugs. | `MenusController::store`, `routes/panel/*` |
| D15 | Low | Pivot tables have no PK/FK/unique → duplicates possible; soft-deleting roles/menus leaves dangling rows; deleting a parent menu orphans children. | migrations |
| D16 | Low | "Ganti Otoritas" links via GET to a POST-only route (405). | `layouts/app.blade.php` |
| D17 | Low | Otoritas "Tambah" button checks `pengguna` create instead of `otoritas`. | `contents/otoritas/list.blade.php` |
| D18 | Low | Header dark-mode toggle not persisted (writes unused key). | `public/assets/js/script.js` |
| D19 | Low | Dead routes/views: Google register methods missing; verify-email & confirm-password views missing; Breeze `ProfileController` routes not registered. | `routes/auth.php`, `ProfileController` |
| D20 | Low | User edit shows email but ignores it; validation error rendering called with wrong arguments. | `UserController::update`, `public/js/page/pengguna/list.js` |
| D21 | Low | Password policies inconsistent (min 5 / 6 / 8 / strong) across create, profile, reset, admin change. | various Requests/controllers |
| D22 | Info | Tests stale and unsafe to run against a dev DB. | `tests/`, `phpunit.xml` |

## 8. Uncertain / not verified

- Runtime behavior was **not** exercised (app not started, tests not run) — analysis is static.
- Whether production uses MySQL, SQL Server, or SQLite: `.env.example` says sqlite, form requests reference `sqlsrv` schema, impersonation clears `lembaga_*` session keys from another project. Treated as leftovers.
- `download` action (id 5) has no route using it in this repo; kept because the permission matrix exposes it.
- Menu nesting deeper than 2 levels: model supports recursion (`childs` eager-loads recursively) but UI only creates/renders 2 levels → Go version enforces 2.
- `log-viewer`, `mpdf`, `phpspreadsheet` installed but unused in app code — not ported.

## 9. Decisions carried into Phase 2

Finalized in [ARCHITECTURE.md](ARCHITECTURE.md) (§4 endpoint map, §6 RBAC, §8 schema, §15 decision log). Notable renames: menu slugs `beranda→dashboard`, `pengguna→users`, `otoritas→roles`, `manajemen_menu→menus`, `profil→profile`; action IDs 1–5 become codes `view/create/update/delete/download`; super-admin gets every permission implicitly.

- Keep the **menu × action** permission model (it is the core of this boilerplate), with proper FKs and composite PKs.
- Keep **active role per session**.
- Server-side sessions with opaque token in HttpOnly cookie (no JWT, nothing in localStorage).
- Drop: master password, shared reset password, Google OAuth (optional later), email verification, layout variants, IcoFont.
- Add: last-admin guard, role-grant guard, session revocation on deactivate/password change, unified error envelope, audit log via services.

## 10. Implementation status

Legend: ✅ implemented + automated test · 🟡 implemented, verified manually only · ⏳ pending · ➖ intentionally not ported

| Feature | Backend (Go) | Frontend (React) |
|---|---|---|
| Login (username/email), generic failure, throttle 5/login+IP, extra per-IP limit | ✅ | ✅ form, errors, `next` redirect |
| Logout, session expiry (idle + absolute), session rotation, hashed tokens | ✅ | ✅ |
| CSRF (token + Origin check) | ✅ | ✅ header on unsafe requests |
| Active role selection / switch, revoked role takes effect immediately | ✅ | ✅ choose-role page + switch from user menu |
| Self-registration (env flag, `GET /auth/options`) | ✅ | ✅ |
| Forgot / reset password (single-use, 60 min, hashed, throttled, revokes sessions) | ✅ | ✅ |
| Users: list/search/filter/sort/paginate, create, update, status, roles, set password, soft delete | ✅ | ✅ |
| Guards: last super-admin (also under concurrency), self delete/deactivate, super-admin-only grants, no escalation via roles/matrix | ✅ | ✅ UI mirrors them |
| Roles: CRUD, immutable slug, system roles protected | ✅ | ✅ list/create/system-role protection; 🟡 edit/delete dialogs |
| Permission matrix (read, full replace, super-admin read-only) | ✅ | ✅ |
| Menus: CRUD, main/child rules, 2-level max, immutable unique slug, delete guard | ✅ | ✅ create + rules; 🟡 edit/delete |
| Navigation filtered by active role (children + inactive filtered) | ✅ | ✅ sidebar, landing redirect; 🟡 mobile sheet |
| Profile: view/update biodata, change own password (revokes other sessions) | ✅ | ✅ |
| Impersonation (super-admin only, not into super-admins, audited, stop restores role) | ✅ | ✅ banner; 🟡 start/stop clicks (verified in Chrome) |
| Theme light/dark/system (persisted) | — | 🟡 |
| Layout: header, breadcrumb, user menu, 403/404/error pages, loading/empty/error states | — | ✅ 403/404/empty/error; 🟡 visual layout |
| Audit log writes; no secrets in audit metadata | ✅ | ➖ no UI (same as Laravel) |
| SMTP mailer (fake SMTP server test, header-injection guard), log driver redaction in production | ✅ | — |
| Health/readiness, OpenAPI kept in sync with routes | ✅ | — |
| Docker Compose stack (db, migrate, api, web/nginx) | 🟡 verified manually (Phases 6–7) | 🟡 |
| Google OAuth, master password, shared reset password, email verification, layout variants | ➖ | ➖ |
