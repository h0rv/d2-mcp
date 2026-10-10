#!/usr/bin/env python3
"""Install a pinned d2-mcp GitHub release without a checkout or Go compiler.

Requires Python 3.9+. Downloads only from h0rv/d2-mcp and GitHub's asset CDN,
checks GitHub's SHA-256 asset digest, and prints a client configuration snippet.
This verifies transfer integrity against GitHub metadata, not signed provenance.
Use --sha256 to pin a previously verified digest instead of consulting the API.
"""

import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import stat
import sys
import tempfile
import urllib.error
import urllib.parse
import urllib.request

REPOSITORY = "h0rv/d2-mcp"
MAX_BINARY_SIZE = 256 * 1024 * 1024
MAX_METADATA_SIZE = 4 * 1024 * 1024
PLATFORMS = {
    ("Linux", "x86_64"): "linux-amd64",
    ("Linux", "amd64"): "linux-amd64",
    ("Linux", "aarch64"): "linux-arm64",
    ("Linux", "arm64"): "linux-arm64",
    ("Darwin", "x86_64"): "darwin-amd64",
    ("Darwin", "arm64"): "darwin-arm64",
    ("Windows", "amd64"): "windows-amd64.exe",
    ("Windows", "x86_64"): "windows-amd64.exe",
}


class InstallError(Exception):
    pass


def check_download_url(url):
    parsed = urllib.parse.urlsplit(url)
    if (parsed.scheme != "https" or parsed.username is not None or
            parsed.password is not None or parsed.port not in (None, 443) or
            parsed.fragment):
        raise InstallError("Refusing a non-HTTPS or credential-bearing download URL")
    if parsed.hostname == "api.github.com":
        allowed = parsed.path.startswith(f"/repos/{REPOSITORY}/releases/tags/")
    elif parsed.hostname == "github.com":
        allowed = parsed.path.startswith(f"/{REPOSITORY}/releases/download/")
    else:
        allowed = parsed.hostname in {"release-assets.githubusercontent.com", "objects.githubusercontent.com"}
    if not allowed:
        raise InstallError("Refusing a download outside the project release and GitHub asset hosts")


class GitHubRedirects(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, request, response, code, message, headers, new_url):
        check_download_url(new_url)
        return super().redirect_request(request, response, code, message, headers, new_url)


def open_url(url):
    check_download_url(url)
    request = urllib.request.Request(url, headers={
        "User-Agent": "d2-mcp-release-installer",
        "Accept": "application/vnd.github+json" if url.startswith("https://api.github.com/") else "application/octet-stream",
    })
    return urllib.request.build_opener(GitHubRedirects()).open(request, timeout=30)


def asset_for_platform():
    system, machine = platform.system(), platform.machine().lower()
    # A 32-bit Python on Windows can report x86 while the OS is 64-bit.
    # Do not silently select emulated amd64 binaries on Windows ARM64.
    if system == "Windows":
        machine = os.environ.get("PROCESSOR_ARCHITEW6432", os.environ.get("PROCESSOR_ARCHITECTURE", machine)).lower()
    suffix = PLATFORMS.get((system, machine))
    if suffix is None:
        raise InstallError(f"No release binary for {system}/{machine}; supported: Linux and macOS amd64/arm64, Windows amd64")
    return "d2-mcp-" + suffix


def release_asset(version, name):
    url = f"https://api.github.com/repos/{REPOSITORY}/releases/tags/{version}"
    with open_url(url) as response:
        raw = response.read(MAX_METADATA_SIZE + 1)
    if len(raw) > MAX_METADATA_SIZE:
        raise InstallError("Release metadata exceeds the size limit")
    try:
        metadata = json.loads(raw)
    except (ValueError, UnicodeError) as error:
        raise InstallError("GitHub returned invalid release metadata") from error
    if not isinstance(metadata, dict) or metadata.get("tag_name") != version or metadata.get("draft") is not False:
        raise InstallError("GitHub did not return the requested published release tag")
    assets = metadata.get("assets", [])
    if not isinstance(assets, list):
        raise InstallError("GitHub returned an invalid release asset list")
    matches = [asset for asset in assets if isinstance(asset, dict) and asset.get("name") == name]
    if len(matches) != 1:
        raise InstallError(f"Release {version} does not contain exactly one {name}")
    asset = matches[0]
    digest = asset.get("digest", "")
    size = asset.get("size")
    if not isinstance(digest, str) or not re.fullmatch(r"sha256:[0-9a-fA-F]{64}", digest):
        raise InstallError("Release asset has no GitHub SHA-256 digest; refusing an unverified install")
    if type(size) is not int or not 0 < size <= MAX_BINARY_SIZE or asset.get("state") != "uploaded":
        raise InstallError("Release asset has an invalid state or size")
    download = f"https://github.com/{REPOSITORY}/releases/download/{version}/{name}"
    if asset.get("browser_download_url") != download:
        raise InstallError("Release asset URL does not match the requested project, version and platform")
    return download, digest.removeprefix("sha256:").lower(), size


def default_directory():
    if platform.system() == "Windows":
        return Path(os.environ.get("LOCALAPPDATA", Path.home() / "AppData" / "Local")) / "d2-mcp" / "bin"
    return Path.home() / ".local" / "bin"


def check_target(target, replace):
    try:
        mode = target.lstat().st_mode
    except FileNotFoundError:
        return
    if not stat.S_ISREG(mode):
        raise InstallError(f"Refusing to replace a symlink or non-regular file: {target}")
    if not replace:
        raise InstallError(f"Already exists: {target}; use --replace to explicitly replace it")


def install(version, directory, replace=False, expected_digest=None):
    name = asset_for_platform()
    target = directory.expanduser().resolve() / ("d2-mcp.exe" if name.endswith(".exe") else "d2-mcp")
    check_target(target, replace)
    if expected_digest is None:
        url, digest, size = release_asset(version, name)
    else:
        url = f"https://github.com/{REPOSITORY}/releases/download/{version}/{name}"
        digest, size = expected_digest.lower(), None
    target.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    temporary = None
    try:
        with tempfile.NamedTemporaryFile(prefix=".d2-mcp-", dir=target.parent, delete=False) as output:
            temporary = Path(output.name)
            hasher, count = hashlib.sha256(), 0
            with open_url(url) as response:
                while True:
                    chunk = response.read(1024 * 1024)
                    if not chunk:
                        break
                    count += len(chunk)
                    if count > (size if size is not None else MAX_BINARY_SIZE):
                        raise InstallError("Downloaded binary exceeds the published asset size")
                    hasher.update(chunk)
                    output.write(chunk)
            if size is not None and count != size:
                raise InstallError("Downloaded binary is truncated or has the wrong size")
            if hasher.hexdigest() != digest:
                raise InstallError("Downloaded binary failed SHA-256 verification")
            output.flush()
            os.fsync(output.fileno())
        os.chmod(temporary, 0o755)
        check_target(target, replace)
        if replace:
            os.replace(temporary, target)
        else:
            # Atomic publication without overwriting a concurrently created file.
            os.link(temporary, target)
        return target, digest
    finally:
        if temporary is not None:
            temporary.unlink(missing_ok=True)


def config_snippet(target, client):
    server = {"command": str(target), "args": ["--transport", "stdio"]}
    if client == "codex":
        command = json.dumps(str(target), ensure_ascii=False).replace("\x7f", "\\u007f")
        return f'[mcp_servers.d2]\ncommand = {command}\nargs = ["--transport", "stdio"]\n'
    if client == "pi":
        server["exposure"] = "direct"
    if client == "vscode":
        server["type"] = "stdio"
    return json.dumps({"servers" if client == "vscode" else "mcpServers": {"d2": server}}, indent=2) + "\n"


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--version", required=True, help="Exact published tag, for example 1.2.0; never latest")
    parser.add_argument("--dir", type=Path, default=default_directory(), help="Install directory; default is user-local, no PATH changes")
    parser.add_argument("--replace", action="store_true", help="Explicitly replace an existing regular binary after verification")
    parser.add_argument("--sha256", help="Use this previously verified SHA-256 digest without querying the GitHub API")
    parser.add_argument("--client", choices=("common", "codex", "vscode", "pi"), default="common", help="Print this client configuration to stdout; no settings are changed")
    args = parser.parse_args(argv)
    if not re.fullmatch(r"v?[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z]+(?:[.-][0-9A-Za-z]+)*)?", args.version):
        parser.error("--version must be an exact version tag such as 1.2.0, v1.2.0 or 1.3.0-rc.1")
    if args.sha256 is not None and not re.fullmatch(r"[0-9a-fA-F]{64}", args.sha256):
        parser.error("--sha256 must contain exactly 64 hexadecimal characters")
    try:
        target, digest = install(args.version, args.dir, args.replace, args.sha256)
    except (InstallError, OSError, ValueError) as error:
        parser.exit(1, f"Install failed: {error}\n")
    print(f"Installed d2-mcp {args.version}: {target}\nSHA-256: {digest}\nMerge the stdout snippet into your client settings; no settings or PATH were changed.", file=sys.stderr)
    sys.stdout.write(config_snippet(target, args.client))


if __name__ == "__main__":
    main()
