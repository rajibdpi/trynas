# TryNAS security review — 13 October 2026

Security fixes preserve the existing application features and accounts. Startup
administrator password logging remains enabled by explicit request.

## Implemented fixes

- Release binaries no longer embed a shared administrator password, hash, or
  recoverable password encoding. Fresh databases require a unique runtime
  ADMIN_PASSWORD. Existing passwords and sessions are not reset by upgrades.
- Go updated to 1.27.2 and golang.org/x/net to 0.60.0. The latest govulncheck
  reports zero vulnerabilities involving reachable symbols. One module-only
  advisory remains in unused deprecated code; no reachable symbols are reported.
- Uploads stream directly into private staging files with remaining-quota limits,
  a 10 GiB request cap, a two-upload concurrency cap and 10,000-file cap. The global
  mutation lock covers final validation and installation, rather than network
  reads. Permissions, destinations and quota are rechecked before committing.
  Multipart field names and field ordering remain compatible with existing clients.
- Unauthorized file requests are rejected before filesystem inspection, closing
  the download file-existence oracle.
- Direct HTTPS is supported through TLS_CERT_FILE and TLS_KEY_FILE. Trusted HTTPS
  proxies can trigger Secure cookies, while untrusted forwarded headers cannot.
  Secure responses include HSTS. Baseline CSP permits existing PDF blob previews;
  individual file previews retain their stricter sandbox policy.
- sprintf-js GHSA-hp3w-g68c-fv3c is addressed by a tracked pnpm patch that bounds
  numeric precision before Number formatting. The security check tests malformed
  and normal formatting before excluding this specific patched advisory. An
  unfiltered registry audit still flags the upstream version; no upstream patched
  release is available. No other advisories are excluded.
- Release CI runs dependency audits and pins GitHub Actions to immutable commits.
  The supplied systemd unit adds privilege and kernel/control-group protections.
  The sample .env requires a unique bootstrap password rather than offering a
  usable example credential.

## Existing protections

Bcrypt password hashing, session expiry and password-reset revocation; HttpOnly
and SameSite cookies; account/IP throttling; explicit trusted proxies; CORS,
Origin/Referer and Fetch Metadata checks; authenticated and administrator-only
routes; folder and recursive permissions; parameterized SQL; path and symlink
checks; ownership-scoped shares and share revocation; sandboxed previews; bounded
archives and ZIP extraction; AES-GCM provider credentials and SFTP known_hosts;
private configuration/database/key files; server timeouts and audit records.

## Deployment and remaining limits

Local HTTP development is retained. An existing .env is not rewritten: configure
HTTPS and secure cookies before remote deployment, following installation.md.
Certificate provisioning, firewalls, host security, mount ACLs and backups require
verification on the deployment host. Rotate previously exposed shared credentials
and redeploy new binaries; source changes cannot revoke passwords on other hosts.

Startup logs still contain the bootstrap password. Restrict journal access. No
MFA or idle-session timeout is added; those would introduce new product behavior.
Sessions remain bearer tokens in the permission-protected database. Rate limits
are process-local. Descriptor-based protection from external filesystem races is
not implemented; external processes must not modify storage paths adversarially.
The baseline CSP preserves existing inline Next.js scripts and PDF previews and
is not a full script allowlist. Storage files are not encrypted by provider-key
AES-GCM. This is a source review with tests and scanners, not a production
penetration test or a guarantee that all attack paths are secure.

## Verification

- Full backend regression suite and go vet passed.
- Race tests passed for app, core and provider modules.
- Production frontend export and TypeScript checks passed; standalone frontend
  security-header configuration was checked separately.
- The frontend precision-patch tests and remaining-advisory audit passed.
- Current govulncheck reports zero reachable vulnerabilities.
- All six local release binaries were rebuilt, their SHA256 checksums verified,
  and build metadata checked for Go 1.27.2, x/net 0.60.0 and absence of credential
  linker flags. No release was published and no running deployment was changed.
- A disposable HTTPS smoke test verified certificate validation, fresh runtime
  bootstrap, login, Secure cookies and HSTS against the rebuilt Linux binary.
