#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.11"
# dependencies = []
# ///
"""Hermetic release installer checks; no network or installed client required."""

import contextlib
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import tomllib
import unittest
from unittest import mock
import urllib.error

SCRIPT = Path(__file__).with_name("install.py")
spec = importlib.util.spec_from_file_location("installer", SCRIPT)
installer = importlib.util.module_from_spec(spec)
spec.loader.exec_module(installer)
PAYLOAD = b"test release bytes\n"
DIGEST = hashlib.sha256(PAYLOAD).hexdigest()
ASSET = "d2-mcp-linux-amd64"
URL = f"https://github.com/h0rv/d2-mcp/releases/download/1.2.0/{ASSET}"


def metadata():
    return {"tag_name": "1.2.0", "draft": False, "assets": [{
        "name": ASSET, "digest": "sha256:" + DIGEST, "size": len(PAYLOAD),
        "state": "uploaded", "browser_download_url": URL,
    }]}


class InstallerTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.directory = Path(self.temporary.name) / "user bin"
        self.platform = mock.patch.object(installer, "asset_for_platform", return_value=ASSET)
        self.platform.start()
        self.addCleanup(self.platform.stop)

    def network(self, release=None, payload=PAYLOAD):
        release = metadata() if release is None else release

        def fetch(url):
            if url == "https://api.github.com/repos/h0rv/d2-mcp/releases/tags/1.2.0":
                return io.BytesIO(json.dumps(release).encode())
            self.assertEqual(url, URL)
            return io.BytesIO(payload)
        return mock.patch.object(installer, "open_url", side_effect=fetch)

    def assert_clean(self):
        if self.directory.exists():
            self.assertEqual(list(self.directory.glob(".d2-mcp-*")), [])

    def test_verified_install_and_exact_config(self):
        with self.network():
            target, digest = installer.install("1.2.0", self.directory)
        self.assertEqual(target.read_bytes(), PAYLOAD)
        self.assertEqual(digest, DIGEST)
        self.assertTrue(target.is_absolute())
        self.assertEqual(json.loads(installer.config_snippet(target, "common")), {
            "mcpServers": {"d2": {"command": str(target), "args": ["--transport", "stdio"]}},
        })
        self.assertEqual(json.loads(installer.config_snippet(target, "vscode"))["servers"]["d2"]["type"], "stdio")
        self.assertEqual(json.loads(installer.config_snippet(target, "pi"))["mcpServers"]["d2"]["exposure"], "direct")
        self.assert_clean()

    def test_explicit_digest_needs_no_metadata(self):
        with mock.patch.object(installer, "open_url", return_value=io.BytesIO(PAYLOAD)) as fetch:
            target, _ = installer.install("1.2.0", self.directory, expected_digest=DIGEST.upper())
        fetch.assert_called_once_with(URL)
        self.assertEqual(target.read_bytes(), PAYLOAD)

    def test_corruption_truncation_oversize_preserve_installed_binary(self):
        self.directory.mkdir()
        target = self.directory / "d2-mcp"
        target.write_bytes(b"existing")
        for payload in (b"x" * len(PAYLOAD), PAYLOAD[:-1], PAYLOAD + b"extra"):
            with self.subTest(payload=payload), self.network(payload=payload):
                with self.assertRaises(installer.InstallError):
                    installer.install("1.2.0", self.directory, replace=True)
            self.assertEqual(target.read_bytes(), b"existing")
            self.assert_clean()

    def test_bad_metadata_fails_before_download_or_directory_creation(self):
        modifications = (
            lambda data: data.update(tag_name="1.3.0"),
            lambda data: data.update(draft=True),
            lambda data: data.update(assets=[]),
            lambda data: data["assets"].append(data["assets"][0]),
            lambda data: data["assets"][0].update(digest=None),
            lambda data: data["assets"][0].update(size=installer.MAX_BINARY_SIZE + 1),
            lambda data: data["assets"][0].update(size=True),
            lambda data: data["assets"][0].update(state="new"),
            lambda data: data["assets"][0].update(browser_download_url="https://example.com/program"),
        )
        for change in modifications:
            data = metadata()
            change(data)
            with self.subTest(data=data), self.network(release=data):
                with self.assertRaises(installer.InstallError):
                    installer.install("1.2.0", self.directory)
            self.assertFalse(self.directory.exists())

    def test_replacement_is_explicit_and_symlinks_refused(self):
        self.directory.mkdir()
        target = self.directory / "d2-mcp"
        target.write_bytes(b"old")
        with self.network() as fetch:
            with self.assertRaises(installer.InstallError):
                installer.install("1.2.0", self.directory)
            fetch.assert_not_called()
        with self.network():
            installer.install("1.2.0", self.directory, replace=True)
        self.assertEqual(target.read_bytes(), PAYLOAD)
        target.unlink()
        target.mkdir()
        with self.assertRaises(installer.InstallError):
            installer.check_target(target, True)
        target.rmdir()
        try:
            target.symlink_to(self.directory / "missing")
        except OSError:
            return  # Windows runners can lack symlink privileges.
        with self.network() as fetch:
            with self.assertRaises(installer.InstallError):
                installer.install("1.2.0", self.directory, replace=True)
            fetch.assert_not_called()

    def test_concurrent_creation_is_not_overwritten(self):
        real_link = os.link

        def concurrent_link(source, destination):
            destination.write_bytes(b"concurrent installation")
            return real_link(source, destination)

        with self.network(), mock.patch.object(installer.os, "link", side_effect=concurrent_link):
            with self.assertRaises(FileExistsError):
                installer.install("1.2.0", self.directory)
        self.assertEqual((self.directory / "d2-mcp").read_bytes(), b"concurrent installation")
        self.assert_clean()

    def test_network_failure_and_limit_cleanup(self):
        with mock.patch.object(installer, "open_url", side_effect=urllib.error.URLError("offline")):
            with self.assertRaises(urllib.error.URLError):
                installer.install("1.2.0", self.directory, expected_digest=DIGEST)
        self.assertFalse((self.directory / "d2-mcp").exists())
        self.assert_clean()

        class InterruptedDownload(io.BytesIO):
            def read(self, amount):
                if self.tell():
                    raise urllib.error.URLError("connection lost")
                return super().read(4)

        with mock.patch.object(installer, "open_url", return_value=InterruptedDownload(PAYLOAD)):
            with self.assertRaises(urllib.error.URLError):
                installer.install("1.2.0", self.directory, expected_digest=DIGEST)
        self.assertFalse((self.directory / "d2-mcp").exists())
        self.assert_clean()
        with mock.patch.object(installer, "MAX_BINARY_SIZE", len(PAYLOAD) - 1), self.network():
            with self.assertRaises(installer.InstallError):
                installer.install("1.2.0", self.directory, expected_digest=DIGEST)
        self.assert_clean()

    def test_redirects_are_restricted(self):
        for url in (URL, "https://release-assets.githubusercontent.com/a?sig=123", "https://objects.githubusercontent.com/a"):
            installer.check_download_url(url)
        for url in ("http://github.com/h0rv/d2-mcp/releases/download/1.2.0/a", "https://evil.com/a",
                    "https://github.com/other/repo/releases/download/1.2.0/a", "https://user@github.com/a",
                    "file:///tmp/program", "https://release-assets.githubusercontent.com:8443/a"):
            with self.subTest(url=url), self.assertRaises(installer.InstallError):
                installer.check_download_url(url)

    def test_platforms_and_unsupported_platform(self):
        self.platform.stop()
        for (system, machine), suffix in installer.PLATFORMS.items():
            with mock.patch.object(installer.platform, "system", return_value=system), mock.patch.object(installer.platform, "machine", return_value=machine), mock.patch.dict(os.environ, {}, clear=True):
                self.assertEqual(installer.asset_for_platform(), "d2-mcp-" + suffix)
        with mock.patch.object(installer.platform, "system", return_value="Windows"), mock.patch.dict(os.environ, {"PROCESSOR_ARCHITECTURE": "AMD64", "PROCESSOR_ARCHITEW6432": "ARM64"}):
            with self.assertRaises(installer.InstallError):
                installer.asset_for_platform()
        with mock.patch.object(installer.platform, "system", return_value="FreeBSD"), mock.patch.object(installer, "open_url") as fetch:
            with self.assertRaises(installer.InstallError):
                installer.install("1.2.0", self.directory)
            fetch.assert_not_called()

    def test_arguments_reject_unpinned_or_arbitrary_urls(self):
        with mock.patch.object(installer, "open_url") as fetch, contextlib.redirect_stderr(io.StringIO()):
            for version in ("latest", "main", "../1.2.0", "https://example.com/program", "1.2.0?x=y"):
                with self.subTest(version=version), self.assertRaises(SystemExit):
                    installer.main(["--version", version])
            with self.assertRaises(SystemExit):
                installer.main(["--version", "1.2.0", "--sha256", "bad"])
            fetch.assert_not_called()

    def test_codex_unicode_and_control_characters(self):
        target = Path('/tmp/bin "\\\U0001f680\x7f\n/d2-mcp')
        snippet = installer.config_snippet(target, "codex")
        self.assertNotIn("\x7f", snippet)
        self.assertNotIn("\\ud83d", snippet)
        self.assertEqual(tomllib.loads(snippet)["mcp_servers"]["d2"], {"command": str(target), "args": ["--transport", "stdio"]})

    def test_uv_standalone_entry_point_ignores_unrelated_project(self):
        copied = Path(self.temporary.name) / "install.py"
        shutil.copyfile(SCRIPT, copied)
        # A downloaded installer must not install/build the project in the
        # user's current directory. Inline metadata supplies its own runtime.
        project = Path(self.temporary.name) / "pyproject.toml"
        project.write_text('[project]\nname = "unrelated"\nversion = "0"\nrequires-python = "<3.11"\ndependencies = ["not-an-installer-dependency"]\n')
        result = subprocess.run(
            ["uv", "run", "--offline", "--no-python-downloads", "--python", sys.executable,
             str(copied), "--help"], cwd=self.temporary.name,
            capture_output=True, text=True, check=True, timeout=30,
        )
        self.assertIn("--version", result.stdout)
        self.assertFalse(project.with_name(".venv").exists())
        self.assertFalse(project.with_name("uv.lock").exists())

    def test_standalone_installer_outside_checkout(self):
        copied = Path(self.temporary.name) / "install.py"
        shutil.copyfile(SCRIPT, copied)
        # Exercise the copied entry point with fixed release bytes at its network
        # boundary. No repository imports, executable URL override, or compiler.
        harness = '''import importlib.util, io, pathlib, sys
spec = importlib.util.spec_from_file_location("installer", "install.py")
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)
module.asset_for_platform = lambda: "d2-mcp-linux-amd64"
module.open_url = lambda url: io.BytesIO(b"test release bytes\\n")
module.main(["--version", "1.2.0", "--sha256", sys.argv[1], "--dir", "installed bin"])
'''
        result = subprocess.run([sys.executable, "-c", harness, DIGEST], cwd=self.temporary.name, capture_output=True, text=True, check=True)
        target = Path(self.temporary.name) / "installed bin" / "d2-mcp"
        self.assertEqual(target.read_bytes(), PAYLOAD)
        self.assertEqual(json.loads(result.stdout)["mcpServers"]["d2"]["command"], str(target.resolve()))
        self.assertIn("Installed d2-mcp 1.2.0", result.stderr)


if __name__ == "__main__":
    unittest.main()
