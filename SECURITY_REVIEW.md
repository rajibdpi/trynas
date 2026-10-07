# TryNAS security review — 8 October 2026

The eight main findings have been addressed in source and local development configuration. Remote deployment still requires HTTPS and secure cookies. Rebuild existing standalone binaries before deployment to include these fixes.

## Resolved findings

| Original finding | Resolution |
| --- | --- |
| Configuration and database readable by other local users | Configuration, database and SQLite sidecars use mode 0600; the local data directory uses 0700. New database directories use 0700. |
| Expired sessions accepted because of string comparisons | Existing session timestamps are normalized to UTC; authentication compares actual timestamps with SQLite julianday. New sessions use UTC. |
| Unlimited password attempts and login body | Login and share unlock bodies are limited to 4 KiB. Attempts are capped at 10 per account/share and 30 per socket IP per five minutes. Forwarded IP headers are not trusted. |
| Cross-origin login accepted | Browser mutations validate Origin/Referer and reject cross-site requests; JSON API requests enforce application/json. |
| Unbounded public ZIP creation and long-held mutation lock | Archives are limited to 1 GiB and 10,000 entries, with two concurrent downloads and a five-minute build context. The mutation lock is released before delivery. |
| No HTTP server timeouts | Header reads are limited to 10 seconds, idle connections to 60 seconds, and reads/writes to 15 minutes. Headers are capped at 1 MiB. |
| Unsafe deployment defaults and fallback credentials | Listener defaults to 127.0.0.1:8080; bootstrap requires an explicit 12–72-byte password and rejects example placeholders. Local configuration now uses loopback. |
| Vulnerable RealIP middleware | Updated chi to 5.3.0 and x/crypto to 0.56.0; removed RealIP middleware. |

## Verification

- Backend regression and race tests passed, including expired sessions, origin/content-type rejection, throttling, timestamp migration, permissions, archive limits and releasing the mutation lock before delivery.
- Updated govulncheck reports zero reachable vulnerable symbols. A dependency-only advisory, GO-2026-5932, remains for unused deprecated OpenPGP code; the scanner reports no fixed module version for it.
- The preceding frontend dependency audit reported zero known vulnerabilities.
- Local configuration, database and existing SQLite sidecars were verified as mode 0600; the data directory is 0700.

## Deployment requirements and limits

Use HTTPS and COOKIE_SECURE=true for remote access. Configure network exposure deliberately. Behind a proxy, rate limits use the proxy socket IP; trusted forwarded-IP support is not enabled. Limits are process-local and reset on restart.

Protect backups with equivalent file permissions. Filesystem races involving external processes changing the storage tree remain outside the application's serialized mutations. This review does not establish the security of a production proxy, firewall or host and is not a full penetration test.

## References

- [OWASP authentication guidance](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html)
- [OWASP CSRF prevention guidance](https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html)
- [Go vulnerability database: GO-2026-5777](https://pkg.go.dev/vuln/GO-2026-5777)
