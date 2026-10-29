# Desktop backend and resources

- `frontend/` is the React and Tailwind UI. See `frontend/AGENTS.md`.
- `app/` is the entire Go backend for the desktop: Wails API, embedded assets, SSH, topology, and encryption.

## Layout under `app/`

| Path | Role |
|------|------|
| `app/` (package `desktop`) | Wails-bound `App`, persisted models, and API methods. Must stay in this directory so bindings remain `wailsjs/go/desktop/`. |
| `app/assets/` | Embedded UI bundle (`ui/dist`, built from `frontend/`) for the Wails asset server. |
| `app/security/` | Encrypted settings envelope, key derivation, AES-GCM. Preserve compatibility with existing `settings.json`. |
| `app/topology/` | Server graph validation and chain ordering (no Wails or SSH). |
| `app/route/` | Adapters from route/map models to `topology`. |
| `app/remote/` | SSH session execution and remote shell command helpers. |

`cmd/desktop/main.go` wires `app.App` to Wails and serves UI assets from `app/assets`.

Do not add Go packages beside `frontend/` at the `internal/desktop/` root; keep backend code under `app/`.
