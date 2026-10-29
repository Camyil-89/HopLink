# Wails application package

Go methods on `App` must live in this directory (package `desktop`). Domain libraries live in sibling folders under `app/`; see `../AGENTS.md`.

Keep methods in domain files instead of growing `app.go`:

- `app.go`: app lifecycle, startup, runtime context.
- `models.go`: persisted and Wails-visible models.
- `vault.go`, `store.go`: unlocking, migration, encrypted configuration persistence (`../security/`).
- `map.go`, `topology.go`: route map operations; chain rules in `../route/` and `../topology/`.
- `settings.go`, `users.go`, `servers.go`: route settings, user CRUD, server CRUD.
- `ssh.go`, `apply.go`, `remove.go`: SSH checks, synchronization, and removal; reusable remote primitives in `../remote/`.
- `status.go`: on-demand SSH checks of each route's systemd service; keep results ephemeral and do not trigger them during startup or route navigation.
- `binaries.go`, `releases.go`, `selfupdate.go`, `process_*.go`: Linux service binary build/download and desktop self-update.

Keep route-specific settings, users, certificates, deployment IDs, and generated connection links scoped to their route. `RemoveRouteService` targets one selected route; `RemoveHoplink` intentionally removes all configured routes on selected servers. Preserve compatibility when changing the encrypted settings schema. New exported Wails methods require a Wails build so `frontend/wailsjs` bindings are refreshed.
