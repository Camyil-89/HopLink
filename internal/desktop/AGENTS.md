# Desktop backend and resources

- `assets.go` is the only Go file at this level. It embeds `frontend/dist` with paths relative to this directory. Linux service binaries are compiled from source at Apply time and are not embedded.
- `app/` owns Wails-bound application state and API methods. Its Go package is named `desktop` so generated Wails bindings remain under `wailsjs/go/desktop/`.
- `remote/` owns reusable SSH command construction and remote command execution.
- `topology/` validates and orders the server graph without depending on Wails or SSH.
- `security/` owns the encrypted settings file envelope, key derivation, and encryption. Preserve compatibility with existing encrypted settings.
- `frontend/` is the React and Tailwind UI. See `frontend/AGENTS.md`.

`cmd/desktop/main.go` wires `app.App` to Wails and uses the assets from this directory. Keep these packages separate; avoid moving domain logic back into `assets.go` or the entry point.
