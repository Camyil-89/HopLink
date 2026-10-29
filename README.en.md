# HopLink

HopLink deploys a Hysteria2 proxy chain across Linux servers. The Go/Wails desktop app manages servers and users, lets you build the route on an interactive map, and synchronizes it over SSH.

![HopLink icon](assets/hoplink-icon.png)

The primary Russian guide is [README.md](README.md).

## Features

- Drag servers from the library onto the map and connect them into a linear chain.
- Assign entry and exit nodes. Applying a changed map updates the chain and removes HopLink from nodes removed from that map.
- CRUD for Hiddify users, Hysteria2 settings, and local storage for the chain key and TLS pair.
- Remove every route's service from selected servers.
- Encrypted `settings.json` is stored beside the launched application executable.

## Development

Requires Go supporting the `go 1.26.0` directive, Node.js/NPM, Wails CLI v2.15, and the WebView2 Runtime on Windows.

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0
Set-Location cmd/desktop
wails dev
```

Build the desktop app from the repository root:

```powershell
node scripts/build-desktop.mjs
```

The executable is written to `deploy/HopLink.exe`.

When applying a route, the app runs `go build` for the required Linux architecture from the repository root (one directory above `deploy/`). Go must be on `PATH`, and the source tree, including `cmd/hoplink/`, must accompany `deploy/`. Each Apply rebuilds `deploy/HopLink-linux-amd64` or `deploy/HopLink-linux-arm64`; Linux binaries are not embedded in the desktop EXE.

Servers need Linux `x86_64` or `aarch64`, SSH access with root privileges or working `sudo`, and an open UDP port (22033 by default). The app asks you to review and trust new SSH host keys.

Settings, users, SSH credentials, keys, and route maps are stored in encrypted `settings.json` beside `HopLink.exe`. Treat this file as a secret and keep it out of Git.

## License

This project is licensed under the GNU Affero General Public License version 3.0. See [LICENSE](LICENSE) for the full text.
