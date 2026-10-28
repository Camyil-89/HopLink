#!/usr/bin/env python3
"""Deploy a Hysteria2 relay chain over SSH.

servers.txt order is entry -> ... -> exit. Each row is:
    ssh_user ssh_password host ssh_port

Optional users.txt rows are:
    client_name client_password
"""

from __future__ import annotations

import argparse
import base64
import datetime as dt
import hashlib
import ipaddress
import json
import os
import re
import secrets
import shlex
import shutil
import subprocess
import sys
import time
import uuid
from dataclasses import dataclass
from pathlib import Path

from cryptography import x509
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import ec
from cryptography.x509.oid import NameOID

try:
    import paramiko
except ImportError:
    print("Paramiko не установлен. Выполни: python -m pip install -r requirements.txt", file=sys.stderr)
    raise SystemExit(2)


ROOT = Path(__file__).resolve().parent
DEPLOY_DIR = ROOT / "deploy"
SERVICE = "hoplink.service"
REMOTE_DIR_DEFAULT = "/opt/hoplink"
HY2_PORT_DEFAULT = 22033
FINAL_USERS_DEFAULT = DEPLOY_DIR / "final_users.txt"
LINKS_CACHE_DEFAULT = DEPLOY_DIR / ".user_links.json"
TLS_CERT_DEFAULT = DEPLOY_DIR / "hy2-server.crt"
TLS_KEY_DEFAULT = DEPLOY_DIR / "hy2-server.key"


@dataclass
class Server:
    user: str
    password: str
    host: str
    ssh_port: int
    arch: str = ""
    client: paramiko.SSHClient | None = None


@dataclass
class ClientUser:
    name: str
    password: str


def parse_servers(path: Path) -> list[Server]:
    if not path.is_file():
        raise RuntimeError(f"Не найден {path}. Нужен формат: <ssh_user> <ssh_pass> <ip> <ssh_port>")
    try:
        path.chmod(0o600)
    except OSError:
        pass
    servers: list[Server] = []
    for line_no, raw in enumerate(path.read_text(encoding="utf-8-sig").splitlines(), 1):
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        fields = line.split()
        if len(fields) != 4:
            raise RuntimeError(f"{path.name}:{line_no}: ожидается ровно 4 поля: <user> <pass> <ip> <port>")
        user, password, host, port_text = fields
        try:
            port = int(port_text)
        except ValueError as exc:
            raise RuntimeError(f"{path.name}:{line_no}: SSH port должен быть числом") from exc
        if not (1 <= port <= 65535):
            raise RuntimeError(f"{path.name}:{line_no}: SSH port вне диапазона")
        servers.append(Server(user, password, host, port))
    if len(servers) < 2:
        raise RuntimeError("Для цепочки нужно минимум два сервера; строки servers.txt идут от входного к выходному")
    return servers


def load_or_create_users(path: Path) -> list[ClientUser]:
    if not path.exists():
        user = ClientUser("user1", secrets.token_hex(32))
        path.write_text(f"{user.name} {user.password}\n", encoding="utf-8")
        try:
            path.chmod(0o600)
        except OSError:
            pass
        print(f"Создан {path.name} с одним случайным клиентом user1. Можно добавить пользователей отдельными строками <имя> <пароль>.")
    users: list[ClientUser] = []
    names: set[str] = set()
    passwords: set[str] = set()
    for line_no, raw in enumerate(path.read_text(encoding="utf-8-sig").splitlines(), 1):
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        fields = line.split()
        if len(fields) != 2:
            raise RuntimeError(f"{path.name}:{line_no}: ожидается формат <имя> <пароль>, без пробелов в полях")
        name, password = fields
        if not re.fullmatch(r"[A-Za-z0-9_.-]{1,48}", name):
            raise RuntimeError(f"{path.name}:{line_no}: имя может содержать латиницу, цифры, '.', '_' и '-'")
        if not password:
            raise RuntimeError(f"{path.name}:{line_no}: пароль не должен быть пустым")
        if name in names or password in passwords:
            raise RuntimeError(f"{path.name}:{line_no}: имена и пароли клиентов должны быть уникальными")
        names.add(name)
        passwords.add(password)
        users.append(ClientUser(name, password))
    if not users:
        raise RuntimeError(f"{path.name} не содержит пользователей")
    try:
        path.chmod(0o600)
    except OSError:
        pass
    return users


def load_or_create_server_certificate(cert_path: Path, key_path: Path, sni: str):
    cert_exists, key_exists = cert_path.exists(), key_path.exists()
    if cert_exists != key_exists:
        raise RuntimeError(f"Найдена только часть TLS-пары: {cert_path.name} и {key_path.name}; восстанови оба файла")
    cert_path.parent.mkdir(parents=True, exist_ok=True)
    if not cert_exists:
        private_key = ec.generate_private_key(ec.SECP256R1())
        subject = x509.Name([x509.NameAttribute(NameOID.COMMON_NAME, sni)])
        try:
            san_name = x509.IPAddress(ipaddress.ip_address(sni))
        except ValueError:
            san_name = x509.DNSName(sni)
        now = dt.datetime.now(dt.timezone.utc)
        cert = (
            x509.CertificateBuilder()
            .subject_name(subject)
            .issuer_name(subject)
            .public_key(private_key.public_key())
            .serial_number(x509.random_serial_number())
            .not_valid_before(now - dt.timedelta(minutes=1))
            .not_valid_after(now + dt.timedelta(days=365 * 5))
            .add_extension(x509.SubjectAlternativeName([san_name]), critical=False)
            .add_extension(x509.BasicConstraints(ca=False, path_length=None), critical=True)
            .add_extension(x509.KeyUsage(digital_signature=True, content_commitment=False, key_encipherment=False,
                                         data_encipherment=False, key_agreement=False, key_cert_sign=False,
                                         crl_sign=False, encipher_only=None, decipher_only=None), critical=True)
            .add_extension(x509.ExtendedKeyUsage([x509.oid.ExtendedKeyUsageOID.SERVER_AUTH]), critical=False)
            .sign(private_key, hashes.SHA256())
        )
        cert_bytes = cert.public_bytes(serialization.Encoding.PEM)
        key_bytes = private_key.private_bytes(
            serialization.Encoding.PEM, serialization.PrivateFormat.PKCS8, serialization.NoEncryption()
        )
        cert_tmp, key_tmp = cert_path.with_suffix(cert_path.suffix + ".tmp"), key_path.with_suffix(key_path.suffix + ".tmp")
        cert_tmp.write_bytes(cert_bytes)
        key_tmp.write_bytes(key_bytes)
        try:
            cert_tmp.chmod(0o644)
            key_tmp.chmod(0o600)
        except OSError:
            pass
        cert_tmp.replace(cert_path)
        key_tmp.replace(key_path)
        print(f"Создан постоянный TLS-сертификат: {cert_path}")
    cert = x509.load_pem_x509_certificate(cert_path.read_bytes())
    private_key = serialization.load_pem_private_key(key_path.read_bytes(), password=None)
    if cert.public_key().public_numbers() != private_key.public_key().public_numbers():
        raise RuntimeError(f"Сертификат и приватный ключ не совпадают: {cert_path}, {key_path}")
    try:
        key_path.chmod(0o600)
    except OSError:
        pass
    return cert_path, key_path


def user_link_identity(user: ClientUser, entry_host: str, hy2_port: int, sni: str) -> str:
    """Stable identity for a client's share link, excluding relay-chain internals."""
    fields = (user.name, user.password, entry_host.strip("[]"), str(hy2_port), sni)
    return hashlib.sha256("\0".join(fields).encode("utf-8")).hexdigest()


def load_link_cache(path: Path) -> dict[str, str]:
    if not path.exists():
        return {}
    try:
        raw = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise RuntimeError(f"Не удалось прочитать кэш ссылок {path}: {exc}; файл оставлен без изменений") from exc
    if not isinstance(raw, dict) or any(not isinstance(k, str) or not isinstance(v, str) for k, v in raw.items()):
        raise RuntimeError(f"Кэш ссылок {path} имеет неверный формат; файл оставлен без изменений")
    return raw


def save_link_cache(path: Path, cache: dict[str, str]):
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp = path.with_suffix(path.suffix + ".tmp")
    tmp.write_text(json.dumps(cache, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    try:
        tmp.chmod(0o600)
    except OSError:
        pass
    tmp.replace(path)


class ConfirmHostKey(paramiko.MissingHostKeyPolicy):
    def __init__(self, known_hosts: Path):
        self.known_hosts = known_hosts

    def missing_host_key(self, client, hostname, key):
        fp = base64.b64encode(hashlib.sha256(key.asbytes()).digest()).decode("ascii").rstrip("=")
        print(f"\nНовый SSH host key для {hostname}: SHA256:{fp}")
        if input("Доверять этому серверу? Введи yes: ").strip().lower() != "yes":
            raise paramiko.SSHException("SSH host key не подтверждён пользователем")
        client.get_host_keys().add(hostname, key.get_name(), key)
        self.known_hosts.parent.mkdir(parents=True, exist_ok=True)
        client.save_host_keys(str(self.known_hosts))


class Remote:
    def __init__(self, server: Server, known_hosts: Path, timeout: int):
        self.server = server
        self.client = paramiko.SSHClient()
        self.client.load_system_host_keys()
        if known_hosts.exists():
            self.client.load_host_keys(str(known_hosts))
        self.client.set_missing_host_key_policy(ConfirmHostKey(known_hosts))
        self.client.connect(
            hostname=server.host,
            port=server.ssh_port,
            username=server.user,
            password=server.password,
            timeout=timeout,
            banner_timeout=timeout,
            auth_timeout=timeout,
            look_for_keys=False,
            allow_agent=False,
        )
        server.client = self.client

    def close(self):
        self.client.close()

    def run(self, command: str, *, sudo: bool = False, check: bool = True, timeout: int = 60) -> tuple[int, str, str]:
        if sudo and self.server.user != "root":
            command = "sudo -S -p '' bash -lc " + shlex.quote(command)
        stdin, stdout, stderr = self.client.exec_command(command, timeout=timeout, get_pty=(sudo and self.server.user != "root"))
        if sudo and self.server.user != "root":
            stdin.write(self.server.password + "\n")
            stdin.flush()
        try:
            stdin.channel.shutdown_write()
        except Exception:
            pass
        out = stdout.read().decode("utf-8", errors="replace")
        err = stderr.read().decode("utf-8", errors="replace")
        status = stdout.channel.recv_exit_status()
        if check and status != 0:
            details = redact_remote_output((out + "\n" + err).strip())
            raise RuntimeError(
                f"{self.server.host}: удалённая команда завершилась с кодом {status}\n"
                f"Команда: {command}\n{details or '(сервер не вернул stdout/stderr)'}"
            )
        return status, out, err

    def upload(self, local: Path, remote_path: str):
        with self.client.open_sftp() as sftp:
            sftp.put(str(local), remote_path)

    def upload_secret(self, secret: str, remote_path: str):
        with self.client.open_sftp() as sftp:
            with sftp.file(remote_path, "wb") as remote_file:
                remote_file.write(secret.encode("ascii"))
                remote_file.chmod(0o600)


def build_binary(goos: str, arch: str) -> Path:
    suffix = ".exe" if goos == "windows" else ""
    path = ROOT / f"HopLink-{goos}-{arch}{suffix}"
    go = shutil.which("go")
    if not go:
        raise RuntimeError(f"Не найден Go: для сборки {path.name} из исходников установи Go.")
    env = os.environ.copy()
    env.update({"GOOS": goos, "GOARCH": arch, "CGO_ENABLED": "0"})
    print(f"Собираю HopLink для {goos}/{arch} из исходников...")
    result = subprocess.run([go, "build", "-o", str(path), "./cmd/hoplink"], cwd=ROOT, env=env, text=True, capture_output=True)
    if result.returncode:
        raise RuntimeError(f"go build завершился с кодом {result.returncode}\n{result.stdout}{result.stderr}")
    return path


def linux_binary(arch: str) -> Path:
    return build_binary("linux", arch)


def preflight(servers: list[Server], known_hosts: Path, timeout: int, *, require_install_tools: bool) -> list[Remote]:
    """Authenticate to every listed host and check deploy prerequisites before mutation."""
    remotes: list[Remote] = []
    try:
        print("Проверяю SSH-доступ ко всем серверам до начала изменений...")
        for index, server in enumerate(servers, 1):
            try:
                remote = Remote(server, known_hosts, timeout)
            except Exception as exc:
                raise RuntimeError(
                    f"SSH-подключение не удалось: сервер {server.host}:{server.ssh_port}, "
                    f"пользователь {server.user}, строка {index} в servers.txt: {exc}"
                ) from exc
            remotes.append(remote)
            try:
                _, identity, _ = remote.run("id -un && uname -m")
            except Exception as exc:
                raise RuntimeError(
                    f"SSH подключён, но проверка сервера {server.host}:{server.ssh_port} "
                    f"(пользователь {server.user}, строка {index}) не прошла: {exc}"
                ) from exc
            lines = identity.splitlines()
            if len(lines) < 2:
                raise RuntimeError(f"{server.host}: не удалось определить пользователя и архитектуру")
            remote_user, machine = lines[0].strip(), lines[1].strip()
            if remote_user != "root":
                _, sudo_status, _ = remote.run("sudo -n true", check=False)
                # Password-based sudo is also supported during installation; verify it
                # now while no remote files or services have been modified.
                if sudo_status != 0:
                    remote.run("true", sudo=True, timeout=timeout)
            if require_install_tools:
                if machine not in {"x86_64", "aarch64", "arm64"}:
                    raise RuntimeError(f"{server.host}: архитектура {machine!r} не поддерживается (нужна x86_64 или aarch64)")
                remote.run("command -v systemctl >/dev/null && command -v base64 >/dev/null")
            server.arch = "amd64" if machine == "x86_64" else "arm64" if machine in {"aarch64", "arm64"} else ""
            print(f"  [{index}/{len(servers)}] {server.host}:{server.ssh_port} — SSH OK, пользователь {remote_user}, архитектура {machine}")
        print("Проверка SSH завершена успешно; можно начинать операцию.")
        return remotes
    except Exception:
        for remote in remotes:
            remote.close()
        raise


def shell_quote(value: str) -> str:
    return shlex.quote(value)


def redact_remote_output(value: str) -> str:
    """Hide credentials embedded in Hysteria share links before printing logs."""
    value = re.sub(r"hysteria2://[^@\s]+@", "hysteria2://[REDACTED]@", value)
    return value[-16000:]


def configure_firewall(remote: Remote, remote_dir: str):
    script = f"""
set -eu
MARKER={shell_quote(remote_dir + '/.firewall-managed')}
if command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -q '^Status: active'; then
  RULES=$(ufw status | grep -F '22033/udp' || true)
  if printf '%s' "$RULES" | grep -q 'ALLOW'; then
    echo 'ufw: UDP 22033 уже разрешён'
  elif [ -n "$RULES" ]; then
    echo 'ufw: обнаружено существующее правило для UDP 22033; не меняю его автоматически'
  else
    ufw allow 22033/udp comment 'go-rel-hys'
    printf 'ufw\\n' > "$MARKER"
    echo 'ufw: добавлено разрешение UDP 22033'
  fi
elif command -v firewall-cmd >/dev/null 2>&1 && firewall-cmd --state >/dev/null 2>&1; then
  if firewall-cmd --query-port=22033/udp >/dev/null 2>&1; then
    echo 'firewalld: UDP 22033 уже разрешён'
  else
    firewall-cmd --permanent --add-port=22033/udp
    firewall-cmd --reload
    printf 'firewalld\\n' > "$MARKER"
    echo 'firewalld: добавлено разрешение UDP 22033'
  fi
else
  echo 'Локальный firewall не менялся; проверь UDP 22033 в firewall хостинга/сервера'
fi
"""
    _, out, err = remote.run(script, sudo=True)
    messages = [line.strip() for line in (out + "\n" + err).splitlines() if line.strip()]
    if messages and not any("firewall" in line.lower() and "проверь" in line.lower() for line in messages):
        print(f"  Firewall: {messages[0]}")


def firewall_cleanup(remote: Remote, remote_dir: str):
    qdir = shell_quote(remote_dir)
    script = f"""
set -eu
MARKER={qdir}/.firewall-managed
if [ -f "$MARKER" ]; then
  KIND=$(cat "$MARKER")
  if [ "$KIND" = ufw ] && command -v ufw >/dev/null 2>&1; then
    ufw --force delete allow 22033/udp comment 'go-rel-hys' || true
  elif [ "$KIND" = firewalld ] && command -v firewall-cmd >/dev/null 2>&1; then
    firewall-cmd --permanent --remove-port=22033/udp || true
    firewall-cmd --reload || true
  fi
fi
systemctl disable --now {SERVICE} 2>/dev/null || true
rm -f /etc/systemd/system/{SERVICE}
systemctl daemon-reload
rm -rf -- {qdir}
"""
    remote.run(script, sudo=True)


def next_address(host: str, port: int) -> str:
    if ":" in host and not host.startswith("["):
        return f"[{host}]:{port}"
    return f"{host}:{port}"


def service_unit(binary_path: str, role: str, forward: str | None, public_host: str | None, users_path: str | None, cert_path: str | None, key_path: str | None, remote_dir: str, hy2_port: int) -> str:
    args = [binary_path, "-mode", role, "-hy2-listen", f":{hy2_port}", "-key-file", f"{remote_dir}/shared.key", "-log-level", "info"]
    if forward:
        args += ["-f", forward]
    if public_host:
        args += ["-hy2-public-host", public_host]
    if users_path:
        args += ["-hy2-users-file", users_path]
    if cert_path and key_path:
        args += ["-hy2-cert", cert_path, "-hy2-cert-key", key_path]
    command = " ".join(shlex.quote(arg) for arg in args)
    return f"""[Unit]
Description=Go Hysteria2 relay chain node
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
WorkingDirectory={remote_dir}
ExecStart={command}
Restart=on-failure
RestartSec=3
LimitNOFILE=65535
UMask=0077

[Install]
WantedBy=multi-user.target
"""


def provision_one(remote: Remote, server: Server, index: int, servers: list[Server], binaries: dict[str, Path], chain_key: str, users_file: Path, cert_file: Path, cert_key_file: Path, use_users_file: bool, remote_dir: str, hy2_port: int):
    arch = server.arch
    binary = binaries[arch]
    stage = f"/tmp/hop-link-{uuid.uuid4().hex}"
    remote.run(f"mkdir -m 700 {shell_quote(stage)}")
    remote.upload(binary, stage + "/HopLink")
    remote.upload_secret(chain_key + "\n", stage + "/shared.key")
    first = index == 0
    if first and use_users_file:
        remote.upload(users_file, stage + "/users.txt")
    if first:
        remote.upload(cert_file, stage + "/hy2-server.crt")
        remote.upload(cert_key_file, stage + "/hy2-server.key")
    role = "hy2-forward" if index == len(servers) - 1 else "hy2-enter"
    forward = None if role == "hy2-forward" else next_address(servers[index + 1].host, hy2_port)
    public_host = server.host if first else None
    users_path = f"{remote_dir}/users.txt" if first and use_users_file else None
    cert_path = f"{remote_dir}/hy2-server.crt" if first else None
    key_path = f"{remote_dir}/hy2-server.key" if first else None
    unit = service_unit(f"{remote_dir}/HopLink", role, forward, public_host, users_path, cert_path, key_path, remote_dir, hy2_port)
    unit_path = ROOT / f".{server.host.replace(':', '_')}.service.tmp"
    unit_path.write_bytes(unit.replace("\r\n", "\n").encode("utf-8"))
    remote.upload(unit_path, stage + "/" + SERVICE)
    try:
        unit_path.unlink()
    except OSError:
        pass

    qstage, qdir = shell_quote(stage), shell_quote(remote_dir)
    install = f"""
set -eu
systemctl stop {SERVICE} 2>/dev/null || true
install -d -m 700 {qdir}
install -m 755 {qstage}/HopLink {qdir}/HopLink
install -m 600 {qstage}/shared.key {qdir}/shared.key
"""
    if first and use_users_file:
        install += f"install -m 600 {qstage}/users.txt {qdir}/users.txt\n"
    elif first:
        install += f"rm -f {qdir}/users.txt\n"
    if first:
        install += f"install -m 644 {qstage}/hy2-server.crt {qdir}/hy2-server.crt\ninstall -m 600 {qstage}/hy2-server.key {qdir}/hy2-server.key\n"
    install += f"install -m 644 {qstage}/{SERVICE} /etc/systemd/system/{SERVICE}\nrm -rf -- {qstage}\nsystemctl daemon-reload\nsystemctl enable {SERVICE}\n"
    remote.run(install, sudo=True)
    configure_firewall(remote, remote_dir)
    print(f"  Подготовлен {server.host} ({role})")


def deploy(args, servers: list[Server], chain_key: str, users_file: Path, cert_file: Path, cert_key_file: Path, users: list[ClientUser]):
    known_hosts = Path.home() / ".ssh" / "known_hosts"
    remotes = preflight(servers, known_hosts, args.ssh_timeout, require_install_tools=True)
    try:
        binaries = {arch: linux_binary(arch) for arch in sorted({s.arch for s in servers})}

        # Stop upstreams and entries before replacing binaries/configuration.
        for remote in remotes:
            remote.run(f"systemctl stop {SERVICE} 2>/dev/null || true", sudo=True, check=False)
        for index, (remote, server) in enumerate(zip(remotes, servers)):
            provision_one(remote, server, index, servers, binaries, chain_key, users_file, cert_file, cert_key_file, True, args.remote_dir, args.hy2_port)

        # Start the exit first, then walk back toward the entry.
        started_at = int(time.time())
        for index in range(len(remotes) - 1, -1, -1):
            remote, server = remotes[index], servers[index]
            print(f"Запускаю {SERVICE} на {server.host}...")
            status, out, err = remote.run(f"systemctl restart {SERVICE}", sudo=True, check=False, timeout=30)
            if status == 0:
                status, out, err = remote.run(f"sleep 2; systemctl is-active --quiet {SERVICE}", sudo=True, check=False, timeout=10)
            if status != 0:
                diag_cmd = f"systemctl status {SERVICE} --no-pager -l; journalctl -u {SERVICE} -n 80 --no-pager"
                _, diag_out, diag_err = remote.run(diag_cmd, sudo=True, check=False, timeout=20)
                details = redact_remote_output("\n".join(part for part in (out, err, diag_out, diag_err) if part.strip()))
                raise RuntimeError(
                    f"{server.host}: {SERVICE} не запустился (код {status}).\n"
                    f"Вывод запуска и диагностика systemd:\n{details or '(сервер не вернул диагностический вывод)'}"
                )
            print(f"  Запущен {server.host}")

        first = remotes[0]
        _, logs, _ = first.run(f"journalctl -u {SERVICE} --since '@{started_at}' -o cat --no-pager", sudo=True, check=False)
        generated: dict[str, str] = {}
        for line in logs.splitlines():
            match = re.search(r"^([A-Za-z0-9_.-]+):\s+(hysteria2://\S+)\s*$", line.strip())
            if match:
                generated[match.group(1)] = match.group(2)

        cache = load_link_cache(args.links_cache)
        entry_host = servers[0].host
        result: list[tuple[ClientUser, str]] = []
        missing: list[str] = []
        for user in users:
            identity = user_link_identity(user, entry_host, args.hy2_port, args.hy2_sni)
            link = generated.get(user.name, "") or cache.get(identity, "")
            if not link:
                missing.append(user.name)
                continue
            cache[identity] = link
            result.append((user, link))
        if missing:
            raise RuntimeError(
                "Не удалось получить актуальные ссылки для пользователей: " + ", ".join(missing)
                + ". Старый final_users.txt и кэш оставлены без изменений. Проверьте журнал входного сервиса."
            )

        args.final_users.parent.mkdir(parents=True, exist_ok=True)
        tmp_final = args.final_users.with_suffix(args.final_users.suffix + ".tmp")
        tmp_final.write_text("".join(f"{user.name} {link}\n" for user, link in result), encoding="utf-8")
        try:
            tmp_final.chmod(0o600)
        except OSError:
            pass
        tmp_final.replace(args.final_users)
        save_link_cache(args.links_cache, cache)
        print("\nСсылки подключения Hiddify:")
        for user, link in result:
            print(f"{user.name}: {link}")
        print(f"\nСохранено: {args.final_users}")
        print("Создан новый ключ цепочки и передан узлам; локально он не сохранялся.")
        print("Проверьте, что UDP-порт открыт в панели хостинга.")
    finally:
        for remote in remotes:
            remote.close()


def delete(args, servers: list[Server]):
    known_hosts = Path.home() / ".ssh" / "known_hosts"
    remotes = preflight(servers, known_hosts, args.ssh_timeout, require_install_tools=False)
    try:
        for remote, server in zip(remotes, servers):
            firewall_cleanup(remote, args.remote_dir)
            print(f"Удалён сервис и каталог {args.remote_dir} на {server.host}")
    finally:
        for remote in remotes:
            remote.close()


def main() -> int:
    parser = argparse.ArgumentParser(description="Развернуть или удалить цепочку Hysteria2 на Linux-серверах по SSH")
    parser.add_argument("--servers", type=Path, default=DEPLOY_DIR / "servers.txt", help="файл SSH-серверов; строки идут от входного к выходному")
    parser.add_argument("--users", type=Path, default=DEPLOY_DIR / "users.txt", help="список Hiddify-клиентов в формате <имя> <пароль>")
    parser.add_argument("--final-users", type=Path, default=FINAL_USERS_DEFAULT, help="куда сохранить итоговые Hiddify-ссылки: <имя> <ссылка>")
    parser.add_argument("--links-cache", type=Path, default=LINKS_CACHE_DEFAULT, help="защищённый кэш выданных ссылок, чтобы повторное развёртывание сохраняло их")
    parser.add_argument("--hy2-sni", default="www.cloudflare.com", help="SNI для Hysteria2 (изменение создаёт новые ссылки)")
    parser.add_argument("--remote-dir", default=REMOTE_DIR_DEFAULT, help="каталог установки на каждом узле")
    parser.add_argument("--hy2-port", type=int, default=HY2_PORT_DEFAULT, help="UDP-порт Hysteria2 на всех узлах")
    parser.add_argument("--ssh-timeout", type=int, default=20)
    parser.add_argument("--delete", action="store_true", help="остановить и удалить systemd-сервис и каталог на всех узлах")
    args = parser.parse_args()
    if not args.remote_dir.startswith("/opt/") or args.remote_dir in {"/opt", "/opt/"}:
        parser.error("--remote-dir должен быть отдельным каталогом внутри /opt")
    if not (1 <= args.hy2_port <= 65535):
        parser.error("--hy2-port вне диапазона")
    try:
        servers = parse_servers(args.servers)
        if args.delete:
            delete(args, servers)
            return 0
        key = secrets.token_hex(32)
        cert_file, cert_key_file = load_or_create_server_certificate(TLS_CERT_DEFAULT, TLS_KEY_DEFAULT, args.hy2_sni)
        users = load_or_create_users(args.users)
        users.sort(key=lambda user: user.name)
        if any(user.password == key for user in users):
            raise RuntimeError("Пароли клиентов должны отличаться от общего ключа цепочки")
        print(f"Развёртывание Hysteria2: {len(servers)} узла(ов), порядок — входной → выходной.")
        print("При первом подключении проверь SSH fingerprint и введи yes.")
        deploy(args, servers, key, args.users, cert_file, cert_key_file, users)
        return 0
    except (OSError, RuntimeError, paramiko.SSHException) as exc:
        print(f"Ошибка развёртывания: {exc}", file=sys.stderr)
        return 1
    finally:
        if sys.stdin.isatty():
            try:
                input("\nНажми Enter, чтобы закрыть окно...")
            except EOFError:
                pass


if __name__ == "__main__":
    raise SystemExit(main())
