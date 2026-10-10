#!/usr/bin/env python3
"""Check a native release artifact outside the checkout without publishing it.

Only the installer's download boundary is replaced by a local release fixture.
Checksum verification, installation, configuration, and the native MCP process
all run normally. This is CI tooling, not an alternative download mechanism.
"""

import argparse
import base64
import hashlib
import json
import os
from pathlib import Path
import queue
import shutil
import struct
import subprocess
import sys
import tempfile
import threading

import install


INSTALL_FIXTURE = r'''
import importlib.util, io, json, pathlib, sys
spec = importlib.util.spec_from_file_location("installer", "install.py")
installer = importlib.util.module_from_spec(spec)
spec.loader.exec_module(installer)
binary, version, destination, digest, mode = sys.argv[1:]
binary = pathlib.Path(binary)
name = installer.asset_for_platform()
assert binary.name == name, (binary.name, name)
download = f"https://github.com/h0rv/d2-mcp/releases/download/{version}/{name}"
metadata = {
    "tag_name": version, "draft": False,
    "assets": [{"name": name, "state": "uploaded", "size": binary.stat().st_size,
                "digest": "sha256:" + digest, "browser_download_url": download}],
}
def fixture(url):
    if url == f"https://api.github.com/repos/h0rv/d2-mcp/releases/tags/{version}":
        return io.BytesIO(json.dumps(metadata).encode())
    if url == download:
        return binary.open("rb")
    raise AssertionError("Unexpected network request: " + url)
installer.open_url = fixture
args = ["--version", version, "--dir", destination]
if mode == "corrupt":
    args += ["--sha256", "0" * 64]
installer.main(args)
'''


def sha256(path):
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def check_mcp(config, directory, version):
    inputs, outputs = directory / "input diagrams", directory / "output diagrams"
    inputs.mkdir()
    outputs.mkdir()
    (inputs / "main.d2").write_text("...@common.d2\na -> b", encoding="utf-8")
    (inputs / "common.d2").write_text("a: ImportedMarker", encoding="utf-8")
    outside = directory / "outside.svg"
    outside.write_bytes(b"keep outside file")
    environment = {**os.environ, "MCP_TRANSPORT": "http", "SSE_MODE": "true"}
    arguments = [config["command"], *config["args"], "--input-dir", str(inputs),
                 "--output-dir", str(outputs), "--allow-overwrite"]
    with (directory / "server.log").open("w+", encoding="utf-8") as logs:
        process = subprocess.Popen(arguments, cwd=directory, env=environment,
                                   stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                   stderr=logs, text=True, encoding="utf-8")
        replies = queue.Queue()

        def read():
            for line in process.stdout:
                replies.put(line)

        threading.Thread(target=read, daemon=True).start()
        request_id = 0

        def send(method, params, notification=False):
            nonlocal request_id
            request = {"jsonrpc": "2.0", "method": method, "params": params}
            if not notification:
                request_id += 1
                request["id"] = request_id
            process.stdin.write(json.dumps(request) + "\n")
            process.stdin.flush()
            if notification:
                return None
            response = json.loads(replies.get(timeout=30))
            assert response.get("id") == request_id, response
            assert "error" not in response, response
            return response["result"]

        def tool(name, arguments, error=None):
            result = send("tools/call", {"name": name, "arguments": arguments})
            assert bool(result.get("isError")) == bool(error), result
            if error:
                assert result["structuredContent"]["error"]["code"] == error, result
            return result

        try:
            initialized = send("initialize", {
                "protocolVersion": "2025-11-25", "capabilities": {},
                "clientInfo": {"name": "native-release-test", "version": "1"},
            })
            assert initialized["serverInfo"]["version"] == version, initialized
            send("notifications/initialized", {}, notification=True)
            assert len(send("tools/list", {})["tools"]) == 3
            tool("compile-d2", {"file_path": "main.d2"})
            tool("compile-d2", {"code": "a: {"}, "invalid_d2")
            tool("compile-d2", {"file_path": "../outside.svg"}, "invalid_arguments")

            name = "diagram #1 日本語.svg"
            result = tool("render-d2", {"file_path": "main.d2", "format": "svg", "output_name": name})
            target = outputs / name
            artifact = result["structuredContent"]["artifact"]
            assert Path(artifact["path"]).resolve() == target.resolve(), artifact
            assert artifact["uri"] == target.as_uri(), artifact
            assert artifact["sha256"] == sha256(target), artifact
            assert "ImportedMarker" in target.read_text(encoding="utf-8")
            links = [entry for entry in result["content"] if entry["type"] == "resource_link"]
            assert len(links) == 1 and links[0]["uri"] == target.as_uri(), links
            args = {"code": "a: ReplacementMarker", "format": "svg", "output_name": name}
            tool("render-d2", args, "write_failed")
            tool("render-d2", {**args, "overwrite": True})
            assert "ReplacementMarker" in target.read_text(encoding="utf-8")
            tool("render-d2", {**args, "output_name": "../outside.svg", "overwrite": True}, "write_failed")
            assert outside.read_bytes() == b"keep outside file"
            assert not list(outputs.glob(".d2-mcp-*"))

            result = tool("render-d2", {"code": "a: |latex x^2 |\na -> b", "format": "png", "layout": "elk"})
            content = result["content"][0]
            assert content["type"] == "image" and content["mimeType"] == "image/png", content
            png = base64.b64decode(content["data"], validate=True)
            assert png.startswith(b"\x89PNG\r\n\x1a\n") and png[12:16] == b"IHDR"
            assert all(20 < dimension <= 8192 for dimension in struct.unpack(">II", png[16:24]))
            result = tool("render-d2", {"code": "a -> b", "format": "ascii", "layout": "tala"})
            assert result["content"][0]["type"] == "text" and result["content"][0]["text"].strip()
            process.stdin.close()
            assert process.wait(timeout=10) == 0
        except BaseException:
            logs.flush()
            logs.seek(0)
            print(logs.read(), file=sys.stderr)
            raise
        finally:
            if process.poll() is None:
                process.terminate()
                try:
                    process.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait(timeout=5)
            process.stdout.close()
            if not process.stdin.closed:
                process.stdin.close()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", type=Path, required=True)
    parser.add_argument("--version", required=True)
    args = parser.parse_args()
    binary = args.binary.resolve()
    assert binary.name == install.asset_for_platform(), "Artifact must match the native release filename"
    digest = sha256(binary)
    with tempfile.TemporaryDirectory(prefix="d2-native-release-") as temporary:
        directory = Path(temporary).resolve() / "client workspace 日本語"
        directory.mkdir()
        fixture = directory / binary.name
        shutil.copyfile(binary, fixture)
        shutil.copyfile(Path(__file__).with_name("install.py"), directory / "install.py")
        destination = directory / "installed bin"
        command = [sys.executable, "-c", INSTALL_FIXTURE, str(fixture), args.version, str(destination), digest]
        failed = subprocess.run([*command, "corrupt"], cwd=directory, capture_output=True,
                                text=True, encoding="utf-8", timeout=60)
        assert failed.returncode == 1 and "SHA-256 verification" in failed.stderr, failed
        assert not list(destination.iterdir()), "Failed verification left an installed file"
        installed = subprocess.run([*command, "metadata"], cwd=directory, capture_output=True,
                                   text=True, encoding="utf-8", check=True, timeout=60)
        config = json.loads(installed.stdout)["mcpServers"]["d2"]
        target = Path(config["command"])
        assert target.parent == destination and target.name == ("d2-mcp.exe" if binary.suffix == ".exe" else "d2-mcp")
        assert sha256(target) == digest
        version = subprocess.run([str(target), "--version"], cwd=directory, capture_output=True,
                                 text=True, encoding="utf-8", check=True, timeout=15)
        assert version.stdout.strip() == f"d2-mcp {args.version} (D2 0.9.0)", version
        check_mcp(config, directory, args.version)
        print(f"Verified native installation and MCP artifacts: {binary.name} {digest}")


if __name__ == "__main__":
    main()
