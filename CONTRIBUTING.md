# Developer guide

## Everyday commands

Run commands from the project root. Set up `.env` from `.env.example` and provide
a unique `ADMIN_PASSWORD` before starting a fresh database.

| Task | Command |
| --- | --- |
| Start frontend and backend | `go run ./scripts/dev` |
| Start backend only | `go run .` |
| Run Go tests | `go test ./...` |
| Check Go code | `go vet ./...` |
| Check frontend types | `cd frontend && pnpm exec tsc --noEmit` |
| Build a release for this machine | `go run ./scripts/build` |
| Build all six platforms | `go run ./scripts/build --all` |
| Rebuild backend with existing UI export | `go run ./scripts/build --skip-frontend` |
| Install a local release | `go run ./scripts/install --source ./dist` |

Development opens the UI on port 3000 and the API on port 8080 by default.
Next.js reloads frontend edits. Restart the command after backend edits. Ctrl+C
stops both servers. Node.js and pnpm are needed for frontend development/builds.

## Where changes belong

| Area | Location |
| --- | --- |
| Executable entry point | `main.go` |
| Configuration and server startup | `config/`, `cmd/` |
| Feature composition and HTTP routes | `app/app.go`, `app/router.go` |
| Shared models, state, path safety, security | `core/` |
| Feature logic | Root feature packages such as `files/`, `share/`, `users/` |
| Database initialization and migrations | `database/` |
| Frontend route entry points | `frontend/app/` |
| Frontend features | `frontend/modules/` |
| Reusable UI components | `frontend/components/ui/` |
| Developer build/dev/install tools | `scripts/build/`, `scripts/dev/`, `scripts/install/` |
| Shared developer process setup | `scripts/internal/tooling/` |
| Published binary installers | `get.sh`, `get.ps1` |
| GitHub release automation | `.github/workflows/release.yml` |

Edit source files; `frontend/dist`, `.next`, `.next-dev`, and the root `dist`
are generated output. Full release builds regenerate the UI before embedding it.

## Backend composition

Each feature has a `Module` with the shared `core.State` and an explicit
`Dependencies` interface for cross-feature operations. `app.New` creates all
modules against the same database, paths, and locks. The application embeds
module aliases so its existing methods are promoted directly from features;
there is no separate layer of forwarding methods to maintain.

For a feature change, edit its package and update relevant regression tests.
For a new feature, define its module/dependencies, wire it in `app.New`, and add
its HTTP routes in `app/router.go`. Keep authentication and permission middleware
around protected routes. Keep cache invalidation with the corresponding writes.
Feature packages depend on shared contracts rather than importing one another.

## Tool organization

The existing developer commands and options remain available. Build target
selection lives in `scripts/build/targets.go`, checksum generation in
`scripts/build/checksums.go`, and build orchestration in `scripts/build/main.go`.
Development orchestration lives in `scripts/dev/main.go`; process supervision
and platform-specific shutdown live in its `process*.go` files. Both commands
use `scripts/internal/tooling.Command` for terminal and environment setup.

## Validation and releases

Run tests relevant to a change, then `go test ./...` and `go vet ./...` for
backend or developer-tool changes. Frontend edits should also pass the type check.
Installer tests run with `go test -count=1 ./scripts/install`; PowerShell tests
require PowerShell and native installation checks require Windows.

See [Installation](installation.md) for platform setup and
[GitHub releases](installation.md#github-releases) for tagging, draft review,
and publishing. Local builds do not publish or restart a running application.
