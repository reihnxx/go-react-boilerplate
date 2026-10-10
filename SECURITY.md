# Security

This document lists the security controls in the boilerplate, how each one is verified, what the Phase 7 review found and fixed, and the risks that remain. Read it before building on the boilerplate or deploying it.

## Controls

| Area | Control | Verified by |
|---|---|---|
| Passwords | argon2id (19 MiB, t=2, p=1), random salt, constant-time compare. Legacy bcrypt hashes (Laravel import) are accepted and re-hashed. Policy: 8–128 characters, at least one letter and one digit. | `password` unit tests |
| Login | One generic error for wrong password, unknown user, inactive user and deleted user. A dummy hash runs for unknown users to keep timing flat. Inputs over 1 KiB are rejected without hashing. | `TestLoginFailuresAreIndistinguishable`, `TestOversizedPasswordIsRejectedAndCounted` |
| Brute force | Two limits. Per login+IP: 5 failures, then a 60 s lock. Per IP across all usernames: 20 failures, against password spraying. A successful login never resets the per-IP counter. | `TestLoginThrottle`, `TestPasswordSprayingIsLimitedPerIP`, `TestSuccessfulLoginDoesNotResetIPCounter` |
| Client IP | The client IP is taken from the TCP peer. Only with `TRUST_PROXY=true` is `X-Real-IP` used, and nginx always overwrites that header. `X-Forwarded-For` and `True-Client-IP` are always ignored. | `TestForwardingHeadersAreTrustedOnlyFromProxy`, `TestTrustedProxyIP`, Docker smoke test |
| Sessions | Opaque 256-bit token in an HttpOnly, SameSite=Lax cookie, Secure in production. Only `sha256(token)` is stored. Sessions have an idle timeout and an absolute lifetime. The session is rotated on login. Sessions are revoked on logout, password change or reset, deactivation and deletion. | `TestSessionTokenIsHashedAtRest`, `TestLogoutAndSessionExpiry`, `TestLoginRotatesSession`, `TestDeactivationAndPasswordChangeRevokeSessions`, `TestDeletedUserSessionsAreRevoked` |
| CSRF | A per-session token must be sent in `X-CSRF-Token` on every unsafe method. Requests whose `Origin` or `Referer` is not in `APP_ALLOWED_ORIGINS` are rejected, including `Origin: null`. | `TestCSRFProtection` |
| Authorization | The backend checks a (menu, action) permission on every protected route, evaluated for the session's active role. The active role must be assigned to the user and still active; revoking it takes effect immediately. The frontend only hides controls. | `TestProtectedEndpointsRequireAuthentication`, `TestUserRoleCannotReachAdminEndpoints`, `TestActiveRoleSelection` |
| Privilege escalation | Only a super-admin can grant the super-admin role or modify a super-admin. Non-super-admins cannot grant roles or matrix permissions beyond their own, and cannot change their own roles. | `TestPrivilegeEscalationIsBlocked` |
| Lock-out | The last active super-admin cannot be deleted, deactivated or stripped of the role, including under concurrent requests (row lock). Users cannot delete or deactivate themselves. | `TestLastSuperAdminIsProtected`, `TestConcurrentDeletionCannotRemoveAllSuperAdmins` |
| Impersonation | Super-admin only. Super-admins cannot be impersonated. Start and stop are audited and the UI shows a banner. This replaces Laravel's master password. | `TestImpersonation` |
| Password reset | A single-use token valid for 60 minutes, stored as a hash. Only the latest token is valid. The response is the same whether or not the email exists, and the email is sent asynchronously. Requests are throttled per IP. A successful reset revokes all sessions. | `TestPasswordResetFlow`, `TestForgotPasswordThrottleAndHashedToken` |
| Input / SQL | All SQL is parameterized. Sort columns come from a whitelist. LIKE wildcards are escaped. The request body is limited to 1 MiB in the API and 2 MiB in nginx. | `TestInjectionAttemptsAreInert`, `TestParsePageQueryRejectsInvalid`, `TestRequestBodyLimit` |
| Output / XSS | React escapes all output, and nothing uses `dangerouslySetInnerHTML`. Menu paths must start with `/` and icons are names from a fixed list, so `javascript:` links are impossible. The SPA's CSP has no `unsafe-inline` for scripts. | grep review, menu validation tests, nginx headers |
| Errors | Internal errors become a generic 500 with a request ID. Details go only to the server log. | `TestWriteErrorHidesInternalErrors` |
| Headers | API: `nosniff`, `DENY`, `default-src 'none'`, `no-store`, HSTS in production, no CORS. SPA through nginx: CSP, `nosniff`, `DENY`, `Referrer-Policy`, `Permissions-Policy`, COOP. | `TestAPISecurityHeaders`, Docker smoke test |
| Audit | Auth and admin actions are recorded with actor, impersonator, IP and user agent. Metadata never contains passwords, hashes or tokens. | `TestAuditLogContainsNoSecrets` |
| Secrets | Real `.env` files are git-ignored and the examples contain no credentials. No passwords are hard-coded: the first admin comes from `create-admin`. The `log` mail driver redacts message bodies in production. | `TestLogMailerRedactsInProduction`, review |
| Containers | The API runs on distroless as nonroot. nginx runs unprivileged. The database port is bound to 127.0.0.1. | Phase 6/7 Docker runs |
| Dependencies | `govulncheck` reports 0 reachable vulnerabilities on Go 1.26.9. `npm audit` reports 0. | Phase 7 scan |

## Phase 7 review: findings and fixes

| # | Finding | Fix |
|---|---|---|
| S1 | `govulncheck` found 30 advisories in the Go 1.26.0 standard library. | go.mod pins `toolchain go1.26.9`. The Docker builder already used 1.26.9. |
| S2 | chi's `middleware.RealIP` trusts `True-Client-IP` and `X-Forwarded-For`, which clients can spoof (staticcheck SA1019). nginx already overwrote those headers, but the API alone was fragile. | Added a custom `TrustedProxyIP` middleware that reads `X-Real-IP` only. |
| S3 | Rotating usernames from one IP was not limited (password spraying). | Added a per-IP failure counter (`AUTH_IP_MAX_ATTEMPTS`). |
| S4 | A login with a ~1 MiB password forced an argon2 hash, giving cheap CPU amplification. | Login inputs over 1 KiB are rejected before hashing. |
| S5 | Synchronous SMTP made forgot-password slower for registered emails, so timing revealed which addresses exist. | The reset email is sent in the background. |
| S6 | With `MAIL_DRIVER=log` in production, reset links would end up in the logs. | Message bodies are redacted in production. |
| S7 | `api seed` silently skipped users when `SEED_PASSWORD` was invalid. | The password is validated up front, and only duplicate-user errors are ignored. |

Earlier phases had already fixed the defects carried over from Laravel (D1–D22 in FEATURE_PARITY.md §7). They include role self-selection, the master password, the hard-coded Google password, inactive users being able to log in, privilege escalation, last-admin removal, sidebar XSS and unfiltered navigation. Phase 4 browser testing also caught and fixed a stale-sidebar bug after an identity change.

## Residual risks and recommendations

- The login throttle lives in PostgreSQL and is keyed on IP. A distributed attack from many IPs against one account is limited only by the per-login counter (5 failures per minute per IP). Add MFA or a WAF if that matters.
- Self-registration has no CAPTCHA or email verification and is disabled by default. Add both before enabling it publicly.
- There is no MFA, no password-breach check (the Laravel `uncompromised` rule was not ported) and no account-level lockout notification.
- Admin password resets revoke sessions but do not notify the user.
- The audit log is append-only by convention, not enforced. Restrict `DELETE` on `audit_logs` with database grants in production.
- The CSP allows `style-src 'unsafe-inline'`, because Radix positioning needs inline styles. Script injection is still blocked.
- In-flight reset emails can be lost if the API shuts down mid-send. The user can simply request a new link.
- TLS, HSTS, backups and secret management are the deployment's job (see ARCHITECTURE.md §13).

## Reporting

Report vulnerabilities privately to the project maintainers rather than in a public issue.
