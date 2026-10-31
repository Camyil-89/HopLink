# Linux HopLink service (`internal/hoplink`)

`cmd/hoplink/` only starts the binary; the service logic lives here. Package `hoplink` at this root wires CLI flags to the subpackages below.

## Layout

| Path | Role |
|------|------|
| `app.go` (`hoplink`) | `-mode`, flags, signal handling, orchestration |
| `log/` | Shared log level and `[TRACE]`…`[ERROR]` formatting |
| `relay/` | Opaque UDP relay between chain hops (QUIC admission, sessions, idle) |
| `ssh/` | SSH tunnel transport for relay traffic (`hoplink-udp-relay` channel) |
| `ws/` | WSS relay transport using the route TLS certificate (`/hoplink-udp-relay`) |
| `wire/` | Shared UDP frame encoding for SSH and WebSocket transports |
| `obfs/` | UDP obfuscation (Salamander, Gecko) on packet connections |
| `hysteria/` | Hysteria2 exit server (TLS, users, Hiddify links) |

Data path:

- **Exit**: client → … → `relay` (optional `obfs` on legs) → `hysteria` (VPN termination).
- **Between relays**: raw UDP (`relay`) or `relay` → `ssh` → next `relay`.

After changes, run `go test -short ./internal/hoplink/...` from the repository root. Full chain tests: `go test -tags=integration ./internal/hoplink/relay/...`.
