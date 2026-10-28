# HopLink

HopLink deploys a Hysteria2 proxy chain across Linux servers. Traffic travels from the entry node through the configured servers to the exit node.

![HopLink icon](assets/hoplink-icon.png)

The primary Russian guide is [README.md](README.md).

## Requirements

- Python 3.10 or newer.
- A Go version that supports the `go 1.26.0` directive in `go.mod`.
- SSH access with root privileges, or working `sudo`, on every server.
- Linux servers using `x86_64` or `aarch64`.
- The UDP port, `22033` by default, must be open on the chain nodes.

Install the Python dependencies:

```powershell
python -m pip install -r requirements.txt
```

On the first build, Go may download the project's dependencies. The deployment script cross-compiles HopLink from source for the server architectures.

## Configure the servers

Copy the example files to their working names:

```powershell
Copy-Item deploy/servers.txt.example deploy/servers.txt
Copy-Item deploy/users.txt.example deploy/users.txt
```

List servers in `deploy/servers.txt`, ordered from entry to exit. Each non-empty line has four fields:

```text
SSH_USER SSH_PASSWORD SSH_HOST SSH_PORT
```

For example:

```text
root ssh_password 1.1.1.1 22
root ssh_password 2.2.2.2 22
```

Replace the example values with the actual server credentials. This file contains SSH passwords and is excluded from Git.

In `deploy/users.txt`, list Hiddify clients, one per line:

```text
name password
```

Passwords must be unique. If the file is missing, the script creates a `user1` account with a random password.

## Deploy

Run this from the project root:

```powershell
python deploy_chain.py
```

On the first connection, verify each server's SSH fingerprint and type `yes` to trust it. The script checks access to all nodes, builds HopLink from source, and installs and starts the `hoplink.service` systemd unit.

Common options:

```text
--servers PATH       SSH server list
--users PATH         Hiddify user list
--hy2-sni NAME       SNI; defaults to www.cloudflare.com
--hy2-port PORT      UDP port; defaults to 22033
--remote-dir PATH    server directory; defaults to /opt/hoplink
--delete             stop the service and remove the remote directory
```

See `python deploy_chain.py --help` for all options.

The `--delete` command stops and removes the service from all specified servers.

## License

This project is licensed under the GNU Affero General Public License version 3.0. See [LICENSE](LICENSE) for the full text.
