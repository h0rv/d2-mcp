#!/usr/bin/env python3
"""Build local client packages without modifying client settings or contacting a server."""

import argparse
import json
import os
from pathlib import Path
import shutil
from urllib.parse import urlsplit

ROOT = Path(__file__).resolve().parent
SCHEMA = "https://agent-plugins.org/schemas/1.0.0/"


def endpoint(args, defaults):
    """Explicit local selection wins; otherwise prefer a supplied/verified remote."""
    if args.url is not None and not args.url.strip():
        raise ValueError("--url must be your non-empty MCP endpoint")
    remote = args.url or os.environ.get("D2_MCP_URL") or defaults["remote_url"]
    if args.local_command is None and remote:
        if any(ord(character) <= 32 or ord(character) == 127 for character in remote):
            raise ValueError("MCP URLs cannot contain whitespace or control characters")
        parsed = urlsplit(remote)
        # Accessing .port validates both the integer and its range.
        parsed.port
        if (parsed.scheme != "https" and not (
            parsed.scheme == "http" and parsed.hostname in {"localhost", "127.0.0.1", "::1"}
        )) or not parsed.hostname or parsed.username is not None or parsed.password is not None or parsed.query or parsed.fragment:
            raise ValueError("Use an HTTPS MCP URL (HTTP is allowed only on loopback), without credentials, query strings, or fragments")
        if args.server_arg:
            raise ValueError("--server-arg applies only to a local command")
        return {"type": "streamable-http", "url": remote}
    command = args.local_command or defaults["local_command"]
    resolved = shutil.which(command)
    if resolved is None:
        raise ValueError(f"Local binary not found/executable: {command}; build it first or pass --url")
    return {"type": "stdio", "command": str(Path(resolved).resolve()), "args": args.server_arg}


def write_json(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, indent=2) + "\n", encoding="utf-8")


def build(destination, server):
    destination = destination.resolve()
    source = (ROOT / "package").resolve()
    if destination == source or source in destination.parents:
        raise ValueError("Output must be outside the package source directory")
    # Refuse existing directories so setup cannot erase an installation or user edits.
    destination.mkdir(parents=True, exist_ok=False)
    plugin = destination / "plugins" / "d2"
    shutil.copytree(source, plugin)
    shutil.copyfile(ROOT.parent / "LICENSE", plugin / "LICENSE")
    metadata = json.loads((plugin / "plugin.json").read_text(encoding="utf-8"))
    compatible = {key: value for key, value in metadata.items() if key != "$schema"}
    compatible["commands"] = "./prompts/"
    write_json(plugin / ".claude-plugin" / "plugin.json", compatible)
    # Supported fallback for Codex clients predating root Agent Plugins manifests.
    codex = {key: value for key, value in compatible.items() if key != "commands"}
    codex.update({"skills": "./skills/", "mcpServers": "./.mcp.json"})
    write_json(plugin / ".codex-plugin" / "plugin.json", codex)
    write_json(plugin / "mcp.json", {"$schema": SCHEMA + "mcp.schema.json", "mcpServers": {"d2": server}})
    legacy = dict(server)
    if legacy["type"] == "streamable-http":
        legacy["type"] = "http"
    write_json(plugin / ".mcp.json", {"mcpServers": {"d2": legacy}})
    write_json(destination / ".claude-plugin" / "marketplace.json", {
        "name": "d2-local", "description": "Locally configured D2 diagram tools and skill", "owner": metadata["author"],
        "plugins": [{"name": "d2", "source": "./plugins/d2"}],
    })
    write_json(destination / ".agents" / "plugins" / "marketplace.json", {
        "name": "d2-local", "interface": {"displayName": "D2 diagrams"},
        "plugins": [{"name": "d2", "source": {"source": "local", "path": "./plugins/d2"},
                     "policy": {"installation": "AVAILABLE", "authentication": "ON_INSTALL"},
                     "category": "Productivity"}],
    })
    # These are mergeable snippets, never written over a client's existing settings.
    configs = destination / "configs"
    write_json(configs / "mcp.json", {"mcpServers": {"d2": legacy}})
    write_json(configs / "vscode.json", {"servers": {"d2": legacy}})
    pi = {key: value for key, value in legacy.items() if key != "type"}
    pi["exposure"] = "direct"
    write_json(configs / "pi.json", {"mcpServers": {"d2": pi}})
    # JSON strings/arrays are also valid TOML basic strings/arrays for these values.
    config = '[mcp_servers.d2]\n'
    if "url" in server:
        config += "url = " + json.dumps(server["url"], ensure_ascii=False) + "\n"
    else:
        config += "command = " + json.dumps(server["command"], ensure_ascii=False) + "\nargs = " + json.dumps(server["args"], ensure_ascii=False) + "\n"
    (configs / "codex.toml").write_text(config, encoding="utf-8")
    return plugin


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", required=True, type=Path, help="New destination directory; must not exist")
    choice = parser.add_mutually_exclusive_group()
    choice.add_argument("--url", help="Your Streamable HTTP MCP endpoint; otherwise D2_MCP_URL/defaults apply")
    choice.add_argument("--local-command", help="Explicit local binary; overrides remote environment/default")
    parser.add_argument("--server-arg", action="append", default=[], help="Repeat for local arguments; use --server-arg=--flag")
    args = parser.parse_args()
    try:
        defaults = json.loads((ROOT / "defaults.json").read_text(encoding="utf-8"))
        server = endpoint(args, defaults)
        plugin = build(args.output.resolve(), server)
    except (OSError, ValueError) as error:
        parser.error(str(error))
    print(f"Built {plugin}\nTransport: {server['type']}\nInstall steps: docs/agent-installation.md")


if __name__ == "__main__":
    main()
