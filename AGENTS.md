# HopLink project map

- `cmd/hoplink/` starts the Linux HopLink service. Its implementation is in `internal/hoplink/`.
- `cmd/desktop/` starts the Windows Wails desktop application. `cmd/desktop/wails.json` controls its frontend and output paths.
- `internal/desktop/` contains desktop resources and packages; see its own `AGENTS.md` for the detailed map.
- `scripts/build-desktop.mjs` builds the desktop application into `deploy/HopLink.exe`. Keep that output name; do not create `latest` or `fixed` variants.
- `deploy/` contains distributable files and runtime data. A user's encrypted `settings.json` belongs beside the executable. Never commit, print, or replace real settings, passwords, private keys, or certificates.
- `.github/workflows/` contains CI.

The desktop owns its UI, configuration, SSH, and route orchestration. At Apply time it compiles the Linux service from `cmd/hoplink` one directory above `deploy/`; Go must be on PATH and the source tree must accompany the EXE. Each route has its own settings, users, certificates, and service identity. Route-specific operations must not remove another route's service; the global removal action intentionally clears every route on selected servers.

After Go changes, run `go test ./...`. After desktop changes, run `node scripts/build-desktop.mjs` from the repository root and verify `deploy/HopLink.exe`. On Windows, set `GOCACHE` to a writable directory inside this repository if the default cache is inaccessible.
