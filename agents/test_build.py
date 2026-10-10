"""Hermetic packaging tests; optional real server handshake with D2_MCP_TEST_BINARY."""

import argparse
import json
import os
from pathlib import Path
import queue
import shutil
import socket
import subprocess
import sys
import tempfile
import threading
import time
import tomllib
import unittest
from unittest.mock import patch
import urllib.request

import build


def arguments():
    return argparse.Namespace(url=None, local_command=None, server_arg=[])


class PackagingTests(unittest.TestCase):
    def test_endpoint_precedence_and_local_escape(self):
        defaults = {"remote_url": "https://deployed.test/mcp", "local_command": sys.executable}
        with patch.dict(os.environ, {"D2_MCP_URL": "https://operator.test/mcp"}):
            self.assertEqual(build.endpoint(arguments(), defaults)["url"], "https://operator.test/mcp")
            args = arguments()
            args.url = "https://explicit.test/mcp"
            self.assertEqual(build.endpoint(args, defaults)["url"], args.url)
            args.local_command, args.url = sys.executable, None
            self.assertEqual(build.endpoint(args, defaults)["type"], "stdio")
        with patch.dict(os.environ, {}, clear=True):
            self.assertEqual(build.endpoint(arguments(), defaults)["url"], defaults["remote_url"])
            defaults["remote_url"] = None
            self.assertTrue(Path(build.endpoint(arguments(), defaults)["command"]).is_absolute())

    def test_bad_remote_urls(self):
        for url in ["", " ", "http://remote.test/mcp", "file:///tmp/mcp", "https://u:p@host/mcp", "https://host/mcp#secret", "https://host/mcp?token=secret", "https://host:bad/mcp", "https://host:65536/mcp", "https://host/mcp\n", "https://host/\x01mcp", "https://"]:
            with self.subTest(url=url), self.assertRaises(ValueError):
                args = arguments()
                args.url = url
                build.endpoint(args, {"remote_url": None, "local_command": sys.executable})

    def test_loopback_and_local_arguments(self):
        args = arguments()
        args.url = "http://127.0.0.1:8080/mcp"
        self.assertEqual(build.endpoint(args, {})["url"], args.url)
        args.server_arg = ["--input-dir", "/diagrams with spaces"]
        with self.assertRaises(ValueError):
            build.endpoint(args, {})
        args.url, args.local_command = None, sys.executable
        self.assertEqual(build.endpoint(args, {"remote_url": None})["args"], ["--transport", "stdio", *args.server_arg])
        for flags in [["--transport", "http"], ["--transport=http"], ["-transport", "http"], ["-transport=http"], ["--sse"], ["--sse=true"], ["-sse"], ["-sse=true"], ["-t=http"], ["--t=http"], ["-t", "http"]]:
            args.server_arg = flags
            with self.assertRaisesRegex(ValueError, "require stdio"):
                build.endpoint(args, {"remote_url": None})

    def test_static_github_package_is_self_contained(self):
        root = build.ROOT.parent
        plugin = build.ROOT / "package"
        for relative, path_key in [(".claude-plugin/marketplace.json", None), (".agents/plugins/marketplace.json", "path")]:
            catalog = json.loads((root / relative).read_text())
            self.assertEqual(catalog["name"], "d2-mcp")
            entry = catalog["plugins"][0]
            source = entry["source"][path_key] if path_key else entry["source"]
            self.assertEqual((root / source).resolve(), plugin)
            self.assertEqual(entry["name"], json.loads((plugin / "plugin.json").read_text())["name"])
        for relative in [".claude-plugin/plugin.json", ".codex-plugin/plugin.json"]:
            manifest = json.loads((plugin / relative).read_text())
            self.assertNotIn("hooks", manifest)
            for key in ["commands", "skills", "mcpServers"]:
                if key in manifest:
                    self.assertTrue((plugin / manifest[key]).exists())
        expected = {"type": "stdio", "command": "d2-mcp", "args": ["--transport", "stdio"]}
        for relative in [".mcp.json", "mcp.json"]:
            self.assertEqual(json.loads((plugin / relative).read_text())["mcpServers"], {"d2": expected})
        pi = json.loads((root / "package.json").read_text())
        self.assertTrue(pi["private"])
        self.assertNotIn("scripts", pi)
        self.assertNotIn("dependencies", pi)
        for kind in ["skills", "prompts"]:
            self.assertEqual([(root / path).resolve() for path in pi["pi"][kind]], [plugin / kind])
        with tempfile.TemporaryDirectory() as temp:
            generated = build.build(Path(temp) / "bundle", expected)
            for relative in [".claude-plugin/plugin.json", ".codex-plugin/plugin.json", "mcp.json", ".mcp.json"]:
                self.assertEqual((generated / relative).read_bytes(), (plugin / relative).read_bytes())

    def test_hostname_is_consistent_across_clients(self):
        for url in ["https://bad\\host/mcp", "https://bad%00host/mcp", "https://ho|st/mcp", "https://exa^mple/mcp", "https://exa<mple/mcp", "https://[::1%25eth0]/mcp", "https://faß.de/mcp", "https://.host/mcp", "https://host../mcp", "https://999.999.999.999/mcp", "https://127.1/mcp", "https://example.123/mcp", "https://0xfffffffff/mcp", "https://xn--a.test/mcp"]:
            with self.subTest(url=url), self.assertRaises(ValueError):
                args = arguments()
                args.url = url
                build.endpoint(args, {})
        for url in ["https://xn--fa-hia.de/日本語", "https://example.test/mcp", "http://localhost:8080/mcp", "http://127.0.0.1:8080/mcp", "http://[::1]:8080/mcp"]:
            args = arguments()
            args.url = url
            self.assertEqual(build.endpoint(args, {})["url"], url)

    def test_missing_binary_is_actionable(self):
        args = arguments()
        args.local_command = "/does-not-exist/d2-mcp"
        with self.assertRaisesRegex(ValueError, "install a release binary"):
            build.endpoint(args, {"remote_url": None})

    @unittest.skipUnless(os.environ.get("D2_MCP_TEST_CODEX"), "Set D2_MCP_TEST_CODEX for native Codex config parsing")
    def test_native_codex_config_without_persisting_settings(self):
        """Use supported, read-only overrides; never change HOME or CODEX_HOME."""
        servers = [
            {"type": "stdio", "command": "/binary path/📊/d2-mcp", "args": ["--transport", "stdio", "--input-dir", "/some path/日本語"]},
            {"type": "streamable-http", "url": "https://example.test/日本語/mcp"},
        ]
        for server in servers:
            with self.subTest(server=server), tempfile.TemporaryDirectory() as temp:
                destination = Path(temp) / "bundle"
                build.build(destination, server)
                config = tomllib.loads((destination / "configs/codex.toml").read_text())["mcp_servers"]["d2"]
                command = [os.environ["D2_MCP_TEST_CODEX"]]
                for key, value in config.items():
                    command.extend(["-c", f"mcp_servers.d2_package_probe.{key}={build.toml_value(value)}"])
                command.extend(["mcp", "get", "d2_package_probe", "--json"])
                result = subprocess.run(command, cwd=temp, capture_output=True, text=True, timeout=15, check=True)
                transport = json.loads(result.stdout)["transport"]
                self.assertEqual({key: transport[key] for key in config}, config)
                self.assertEqual(transport["type"], "stdio" if server["type"] == "stdio" else "streamable_http")

    def test_output_cannot_recurse_into_package_source(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            source = root / "package"
            source.mkdir()
            alias = root / "alias"
            alias.symlink_to(source, target_is_directory=True)
            with patch.object(build, "ROOT", root):
                for destination in [source, source / "generated", alias / "generated"]:
                    with self.subTest(destination=destination), self.assertRaisesRegex(ValueError, "outside the package source"):
                        build.build(destination, {"type": "stdio", "command": "/bin/test", "args": []})
            self.assertEqual(list(source.iterdir()), [])

    def test_generated_packages_and_config_agree(self):
        servers = [
            {"type": "stdio", "command": "/binary path/📊/d2-mcp", "args": ["--input-dir", "/some path/日本語/📊"]},
            {"type": "stdio", "command": "/binary\x7f/d2-mcp", "args": ["--input-dir", "/some\x7fpath"]},
            {"type": "streamable-http", "url": "http://127.0.0.1:8080/mcp"},
        ]
        for server in servers:
            with self.subTest(server=server), tempfile.TemporaryDirectory() as temp:
                destination = Path(temp) / "bundle"
                plugin = build.build(destination, server)
                portable = json.loads((plugin / "mcp.json").read_text())["mcpServers"]["d2"]
                self.assertEqual(portable, server)
                expected = {key: value for key, value in server.items() if key != "type"}
                self.assertEqual(tomllib.loads((destination / "configs/codex.toml").read_text())["mcp_servers"]["d2"], expected)
                for filename, wrapper in [("mcp.json", "mcpServers"), ("vscode.json", "servers"), ("pi.json", "mcpServers")]:
                    actual = json.loads((destination / "configs" / filename).read_text())[wrapper]["d2"]
                    self.assertEqual({key: value for key, value in actual.items() if key not in {"type", "exposure"}}, expected)
                for kind in ["claude", "codex"]:
                    manifest = json.loads((plugin / f".{kind}-plugin/plugin.json").read_text())
                    for key in ["skills", "commands", "mcpServers"]:
                        if key in manifest:
                            self.assertTrue((plugin / manifest[key]).exists())
                codex_market = json.loads((destination / ".agents/plugins/marketplace.json").read_text())
                self.assertEqual((destination / codex_market["plugins"][0]["source"]["path"]).resolve(), plugin)
                claude_market = json.loads((destination / ".claude-plugin/marketplace.json").read_text())
                self.assertEqual((destination / claude_market["plugins"][0]["source"]).resolve(), plugin)
                pi = json.loads((plugin / "package.json").read_text())["pi"]
                for path in pi["skills"] + pi["prompts"]:
                    self.assertTrue((plugin / path).is_dir())
                for prompt in (plugin / "prompts").glob("*.md"):
                    self.assertIn("$ARGUMENTS", prompt.read_text())
                self.assertIn("name: d2", (plugin / "skills/d2/SKILL.md").read_text())
                with self.assertRaises(FileExistsError):
                    build.build(destination, server)

    @unittest.skipUnless(os.environ.get("D2_MCP_TEST_BINARY"), "Set D2_MCP_TEST_BINARY for a real stdio handshake")
    def test_packaged_stdio_source_to_artifact(self):
        with tempfile.TemporaryDirectory() as temp:
            args = arguments()
            args.local_command = os.environ["D2_MCP_TEST_BINARY"]
            plugin = build.build(Path(temp) / "bundle", build.endpoint(args, {"remote_url": None}))
            config = json.loads((plugin / ".mcp.json").read_text())["mcpServers"]["d2"]
            self.assert_stdio_source_to_artifact(config, temp, os.environ.copy())

    @unittest.skipUnless(os.environ.get("D2_MCP_TEST_BINARY"), "Set D2_MCP_TEST_BINARY for a static package handshake")
    def test_static_package_outside_checkout_source_to_artifact(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            plugin = root / "plugin"
            shutil.copytree(build.ROOT / "package", plugin)
            (root / "bin").mkdir()
            shutil.copyfile(os.environ["D2_MCP_TEST_BINARY"], root / "bin/d2-mcp")
            (root / "bin/d2-mcp").chmod(0o755)
            config = json.loads((plugin / "mcp.json").read_text())["mcpServers"]["d2"]
            environment = {**os.environ, "PATH": str(root / "bin") + os.pathsep + os.environ["PATH"], "MCP_TRANSPORT": "http", "SSE_MODE": "true"}
            self.assert_stdio_source_to_artifact(config, root, environment)

    def assert_stdio_source_to_artifact(self, config, cwd, environment):
        process = subprocess.Popen([config["command"], *config["args"]], stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, text=True, cwd=cwd, env=environment)
        responses = queue.Queue()
        def read():
            for line in process.stdout:
                responses.put(line)
        reader = threading.Thread(target=read, daemon=True)
        reader.start()
        def send(method, params, request_id=None):
            message = {"jsonrpc": "2.0", "method": method, "params": params}
            if request_id is not None:
                message["id"] = request_id
            process.stdin.write(json.dumps(message) + "\n")
            process.stdin.flush()
            if request_id is not None:
                result = json.loads(responses.get(timeout=15))
                self.assertEqual(result["id"], request_id)
                self.assertNotIn("error", result)
                return result["result"]
        try:
            send("initialize", {"protocolVersion": "2025-11-25", "capabilities": {}, "clientInfo": {"name": "package-test", "version": "1"}}, 1)
            send("notifications/initialized", {})
            listed = send("tools/list", {}, 2)
            self.assertEqual({tool["name"] for tool in listed["tools"]}, {"compile-d2", "render-d2", "fetch_d2_cheat_sheet"})
            source = "direction: right\nclient -> api -> database\ndatabase.shape: cylinder"
            for request_id, tool in [(3, "compile-d2"), (4, "render-d2")]:
                inputs = {"code": source, "layout": "tala"}
                if tool == "render-d2":
                    inputs["format"] = "svg"
                result = send("tools/call", {"name": tool, "arguments": inputs}, request_id)
                self.assertFalse(result.get("isError"), result)
                if tool == "render-d2":
                    self.assertTrue(any("<svg" in item.get("resource", {}).get("text", "") for item in result["content"]))
        finally:
            process.stdin.close()
            try:
                process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait()
            reader.join(timeout=5)
            process.stdout.close()

    @unittest.skipUnless(os.environ.get("D2_MCP_TEST_BINARY"), "Set D2_MCP_TEST_BINARY for a real HTTP handshake")
    def test_packaged_http_source_to_artifact(self):
        with tempfile.TemporaryDirectory() as temp:
            with socket.socket() as listener:
                listener.bind(("127.0.0.1", 0))
                port = listener.getsockname()[1]
            server = subprocess.Popen([os.environ["D2_MCP_TEST_BINARY"], "--transport", "http", "--host", "127.0.0.1", "--port", str(port)], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            try:
                deadline = time.monotonic() + 10
                while True:
                    if server.poll() is not None:
                        self.fail("HTTP server exited before accepting connections")
                    try:
                        with socket.create_connection(("127.0.0.1", port), timeout=.1):
                            break
                    except OSError:
                        if time.monotonic() >= deadline:
                            self.fail("HTTP server did not become ready")
                        time.sleep(.02)
                args = arguments()
                args.url = f"http://127.0.0.1:{port}/mcp"
                plugin = build.build(Path(temp) / "bundle", build.endpoint(args, {"remote_url": None}))
                config = json.loads((plugin / "mcp.json").read_text())["mcpServers"]["d2"]
                self.assertEqual(config["type"], "streamable-http")
                protocol = "2025-11-25"
                def send(method, params, request_id=None):
                    body = {"jsonrpc": "2.0", "method": method, "params": params}
                    if request_id is not None:
                        body["id"] = request_id
                    request = urllib.request.Request(config["url"], data=json.dumps(body).encode(), headers={"Content-Type": "application/json", "Accept": "application/json, text/event-stream", "MCP-Protocol-Version": protocol})
                    with urllib.request.urlopen(request, timeout=15) as response:
                        self.assertIsNone(response.headers.get("Mcp-Session-Id"))
                        if request_id is not None:
                            result = json.load(response)
                            self.assertEqual(result["id"], request_id)
                            self.assertNotIn("error", result)
                            return result["result"]
                initialized = send("initialize", {"protocolVersion": protocol, "capabilities": {}, "clientInfo": {"name": "package-test", "version": "1"}}, 1)
                protocol = initialized["protocolVersion"]
                send("notifications/initialized", {})
                listed = send("tools/list", {}, 2)
                self.assertEqual({tool["name"] for tool in listed["tools"]}, {"compile-d2", "render-d2", "fetch_d2_cheat_sheet"})
                source = "direction: right\nclient -> api -> database\ndatabase.shape: cylinder"
                for request_id, tool in [(3, "compile-d2"), (4, "render-d2")]:
                    inputs = {"code": source, "layout": "elk"}
                    if tool == "render-d2":
                        inputs["format"] = "svg"
                    result = send("tools/call", {"name": tool, "arguments": inputs}, request_id)
                    self.assertFalse(result.get("isError"), result)
                    if tool == "render-d2":
                        self.assertTrue(any("<svg" in item.get("resource", {}).get("text", "") for item in result["content"]))
            finally:
                server.terminate()
                try:
                    server.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    server.kill()
                    server.wait()


if __name__ == "__main__":
    unittest.main()
