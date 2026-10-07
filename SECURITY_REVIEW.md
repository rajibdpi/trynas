# TryNAS security review — 8 October 2026

Scope: source review, configuration/file permission inspection, focused tests on isolated temporary databases, backend test suite, frontend dependency audit and Go govulncheck. This is not a full penetration test. No production credentials or file contents were exposed in the review.

## Findings

| Priority | Finding | Evidence and impact | Recommended change |
| --- | --- | --- | --- |
| High | Configuration and database readable by other local users | `.env` and `data/trynas.db` have mode 0644. The configuration contains credentials; the database contains password hashes and live session/share tokens. Access depends on parent directory permissions. | Restrict both files to 0600 and their directories to the service account; cover SQLite WAL/SHM files and backups too. |
| High | Session expiry compares timestamp strings | `backend/main.go:225` compares `expires_at > CURRENT_TIMESTAMP`. A focused test confirmed a session expired one hour ago, stored with +0600, still authenticates. | Normalize existing session timestamps to UTC, store UTC for new sessions and compare actual instants. |
| High | Password attempts are not throttled | Login and protected-share unlock use bcrypt but have no attempt limits or backoff. Login also decodes an unbounded body. | Add bounded request bodies and per-account/share plus connection-based throttling with trusted proxy handling. |
| Medium | Cross-origin login accepted | An isolated request with an untrusted Origin and `Content-Type: text/plain` successfully logged in and issued a session. CORS controls reading responses, not whether requests execute. SameSite=Lax reduces some authenticated CSRF scenarios but does not substitute for origin/token checks. | Validate Origin/Referer for browser mutations, enforce expected content types and add CSRF protection appropriate to the cookie-based API. |
| Medium | Public ZIP requests can consume disk and block writes | `downloadSharedItems` creates an uncapped temporary ZIP and holds the global mutation mutex through serving the response. A slow recipient or repeated public-folder requests can block mutations and consume temporary disk. | Bound archive size/concurrency and request duration; release the global lock before sending the completed archive. |
| Medium | HTTP server lacks timeouts | `http.ListenAndServe` uses a server with no ReadHeaderTimeout, IdleTimeout or other configured resource limits. | Configure an explicit http.Server and suitable proxy/body limits; account for long NAS transfers. |
| Medium | Unsafe deployment defaults | Current configuration uses `COOKIE_SECURE=false` and `ADDR=:8080`, with plain HTTP available on all interfaces. The bootstrap fallback password is predictable if administrator settings are omitted. This review did not establish public internet exposure. | Use HTTPS and secure cookies, restrict listener/network exposure, and require explicit strong bootstrap credentials. |
| Medium | Vulnerable RealIP middleware | govulncheck reports reachable RealIP IP-spoofing findings GO-2026-5774, GO-2026-5775 and GO-2026-5777 in chi v5.2.3; the scanner lists v5.3.0 as fixed. This app installs middleware.RealIP. | Upgrade chi and accept forwarding headers only from trusted proxies before relying on client IP for security controls. |

## Protections present

- Bcrypt hashes for account and share passwords; cryptographically random 24-byte tokens.
- HttpOnly and SameSite=Lax cookies; server-side sessions revoked on account password resets.
- Backend admin checks for monitoring, activity, users, permissions, Settings and disk health.
- Folder grants applied to listing, searching, file mutations, shared listings and ZIP downloads.
- Relative-path validation, reserved-path restrictions and symlink rejection; public shares remain scoped to their saved items.
- Parameterized SQL and escaped React/Go HTML templates.
- Sandboxed file previews with nosniff, upload request limits, quota checks and serialized mutations.
- Share expiry and access checks also apply after password unlock. Migration preserves older share expiry values.

These measures do not eliminate filesystem races with external local processes changing the storage tree.

## Verification and limits

- Backend `go test ./...` passed.
- Focused tests reproduced the expired-session and cross-origin-login behavior in temporary databases.
- Frontend `pnpm audit --json`: zero known vulnerabilities reported by the registry at review time.
- Go scanner: three reachable chi RealIP findings, plus dependency-level reports in chi and x/crypto. Dependency-only reports, such as SSH/OpenPGP issues, are not proof the vulnerable code is invoked by this app.
- No browser exploit, proxy/firewall assessment, exhaustive race testing or remediation was performed. Existing deployment may have protections outside this repository.

## References

- [OWASP authentication guidance](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html)
- [OWASP CSRF prevention guidance](https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html)
- [Go vulnerability database: GO-2026-5777](https://pkg.go.dev/vuln/GO-2026-5777)
