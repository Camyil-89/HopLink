# Wails application package

Go methods on `App` must share this package. Keep methods in domain files instead of growing `app.go`:

- `app.go`: app lifecycle, startup, runtime context.
- `models.go`: persisted and Wails-visible models.
- `vault.go`, `store.go`: unlocking, migration, encrypted configuration persistence.
- `map.go`, `topology.go`: route map operations and adapter to `../topology/`.
- `settings.go`, `users.go`, `servers.go`: route settings, user CRUD, server CRUD.
- `ssh.go`, `apply.go`, `remove.go`: SSH checks, synchronization, and removal operations; put reusable remote primitives in `../remote/`.
- `status.go`: on-demand SSH checks of each route's systemd service; keep results ephemeral and do not trigger them during startup or route navigation.
- `binaries.go`: compiles the Linux service from `cmd/hoplink` when Apply runs. The executable lives in `deploy/`, and the source root is one directory above it. Output `HopLink-linux-<arch>` sits beside the desktop executable.

Keep route-specific settings, users, certificates, deployment IDs, and generated connection links scoped to their route. `RemoveRouteService` targets one selected route; `RemoveHoplink` intentionally removes all configured routes on selected servers. Preserve compatibility when changing the encrypted settings schema. New exported Wails methods require a Wails build so `frontend/wailsjs` bindings are refreshed.
