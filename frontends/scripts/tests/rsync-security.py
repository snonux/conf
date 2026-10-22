#!/usr/bin/env python3
"""Exercise real recipe plans, native loopback rsync and the rendered publisher.

Run: python3 frontends/scripts/tests/rsync-security.py
Requires Go, Helm, PyYAML and rsync. No production secrets, network or writes.
The daemon fixture substitutes temporary paths, retains its unprivileged uid/gid and disables
chroot; production module authentication and IP access rules remain intact.
"""
import base64
from contextlib import contextmanager
import json
import os
from pathlib import Path
import shutil
import signal
import socket
import stat
import subprocess
import tempfile
import time
import unittest

import yaml

ROOT = Path(__file__).resolve().parents[3]
PASSWORD = "fixture-only-not-a-production-credential"
HARNESS = '''package main
import (
    "context"
    "fmt"
    "os"
    "github.com/snonux/conf/gonf/cluster"
    "github.com/snonux/conf/gonf/tasks"
    "github.com/snonux/gonf/api"
    "github.com/snonux/gonf/cli"
    "github.com/snonux/gonf/secret"
)
func main() {
    api.SetSecretProvider(secret.ProviderFunc(func(_ context.Context, ref secret.Ref) ([]byte, error) {
        if string(ref) != "frontends/etc/rsync/shuriken.password" {
            return nil, fmt.Errorf("unexpected fixture secret reference")
        }
        value := os.Getenv("CONF_TEST_RSYNC_PASSWORD")
        if value == "" { return nil, fmt.Errorf("fixture secret unavailable") }
        return []byte(value), nil
    }))
    cluster.Register()
    tasks.Register()
    cli.Main()
}
'''


class RsyncSecurity(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.workspace = tempfile.TemporaryDirectory(prefix="conf-rsync-tests-")
        cls.root = Path(cls.workspace.name)
        cls.addClassCleanup(cls.workspace.cleanup)
        harness = cls.root / "fixture.go"
        harness.write_text(HARNESS)
        cls.binary = cls.root / "gonf-fixture"
        subprocess.run(["go", "build", "-p", "1", "-o", str(cls.binary), str(harness)],
                       cwd=ROOT / "gonf", check=True, capture_output=True)
        cls.env = {**os.environ, "GONF_CONF_ROOT": str(ROOT),
                   "CONF_TEST_RSYNC_PASSWORD": PASSWORD}
        cls.plan = cls.root / "plan"
        result = cls.record(cls.plan)
        if result.returncode:
            raise AssertionError(result.stderr.decode())
        cls.ops = [json.loads(line) for line in (cls.plan / "plan.jsonl").read_text().splitlines()]
        configs = [base64.b64decode(op["content_b64"]).decode() for op in cls.ops
                   if op.get("op") == "file" and op.get("path") == "/etc/rsyncd.conf"]
        if len(configs) != 2 or configs[0] != configs[1]:
            raise AssertionError("expected identical daemon configs for both frontends")
        cls.config = configs[0]
        if "{{" in cls.config:
            raise AssertionError("daemon template was not rendered")
        resources = list(yaml.safe_load_all(subprocess.check_output(
            ["helm", "template", str(ROOT / "f3s/shuriken/helm-chart")], text=True)))
        job = next(r for r in resources if r and r["kind"] == "CronJob"
                   and r["metadata"]["name"] == "shuriken-sync")
        cls.publisher = job["spec"]["jobTemplate"]["spec"]["template"]["spec"]["containers"][0]["args"][0]
        cls.rsync = shutil.which("rsync")
        if not cls.rsync:
            raise RuntimeError("native rsync is required")

    @classmethod
    def record(cls, destination, password=None, task="frontends_rsync"):
        env = {**cls.env}
        if password is not None:
            env["CONF_TEST_RSYNC_PASSWORD"] = password
        return subprocess.run([str(cls.binary), "plan", "-o", str(destination), task],
                              cwd=ROOT / "gonf", env=env, capture_output=True, timeout=30)

    def test_recipe_keeps_credentials_private_and_removes_root_cron(self):
        credentials = [op for op in self.ops if op.get("path") == "/etc/rsyncd.secrets"]
        self.assertEqual(len(credentials), 2)
        for op in credentials:
            self.assertTrue(op["sensitive"])
            self.assertEqual(op["mode"], "0600")
            self.assertEqual(base64.b64decode(op["content_b64"]), f"shuriken:{PASSWORD}\n".encode())
        crons = [op for op in self.ops if op.get("op") == "cron"]
        self.assertTrue(any(op.get("cron_user") == "root" and op.get("absent") for op in crons))
        present = [op for op in crons if not op.get("absent")]
        self.assertEqual(len(present), 1)
        self.assertEqual(present[0]["cron_user"], "_joernsync")

    def test_bad_secret_fails_before_a_usable_plan_is_written(self):
        for i, password in enumerate(("", "line1\nline2", "\n", " leading", "trailing ", "a\rb")):
            with self.subTest(case=i):
                path = self.root / f"bad-plan-{i}"
                result = self.record(path, password)
                self.assertNotEqual(result.returncode, 0)
                self.assertFalse((path / "plan.jsonl").exists())
                self.assertNotIn(b"line1", result.stdout + result.stderr)

    def test_migration_pause_declares_no_network_or_secret_resources(self):
        path = self.root / "pause-plan"
        result = self.record(path, "", "frontends_rsync_pause_joern")
        self.assertEqual(result.returncode, 0, result.stderr.decode())
        ops = [json.loads(line) for line in (path / "plan.jsonl").read_text().splitlines()]
        crons = [op for op in ops if op.get("op") == "cron"]
        self.assertEqual({op["cron_user"] for op in crons}, {"root", "_joernsync"})
        self.assertTrue(all(op.get("absent") for op in crons))
        self.assertFalse(any(op.get("op") in ("file", "command", "service", "package") for op in ops))

    @contextmanager
    def daemon(self):
        with tempfile.TemporaryDirectory(dir=self.root) as directory:
            root = Path(directory)
            dirs = {}
            config = self.config
            for name in ("joern", "irregular.ninja", "alt.irregular.ninja"):
                dirs[name] = root / name
                dirs[name].mkdir()
                config = config.replace(f"/var/www/htdocs/{name}", str(dirs[name]))
            credentials = root / "secrets"
            credentials.write_text(f"shuriken:{PASSWORD}\n")
            credentials.chmod(0o600)
            password = root / "password"
            password.write_text(PASSWORD + "\n")
            password.chmod(0o600)
            config = config.replace("/etc/rsyncd.secrets", str(credentials))
            config = config.replace("uid = www\n", "").replace("gid = www\n", "")
            config = ("use chroot = no\nlock file = " + str(root / "lock")
                      + "\nlog file = " + str(root / "daemon.log") + "\n" + config)
            config_file = root / "rsyncd.conf"
            config_file.write_text(config)
            with socket.socket() as sock:
                sock.bind(("127.0.0.1", 0))
                port = sock.getsockname()[1]
            proc = subprocess.Popen([self.rsync, "--daemon", "--no-detach", "--address=127.0.0.1",
                                     f"--port={port}", f"--config={config_file}"],
                                    stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                                    start_new_session=True)
            try:
                for _ in range(100):
                    if proc.poll() is not None:
                        raise AssertionError("fixture daemon exited: " + proc.communicate()[1].decode())
                    try:
                        with socket.create_connection(("127.0.0.1", port), timeout=0.1):
                            break
                    except OSError:
                        time.sleep(0.02)
                else:
                    raise AssertionError("fixture daemon did not listen")
                yield root, dirs, password, port
            finally:
                try:
                    os.killpg(proc.pid, signal.SIGTERM)
                    proc.communicate(timeout=2)
                except subprocess.TimeoutExpired:
                    os.killpg(proc.pid, signal.SIGKILL)
                    proc.communicate(timeout=2)
                except ProcessLookupError:
                    proc.communicate(timeout=2)

    def client(self, *args):
        env = {k: v for k, v in os.environ.items() if k != "RSYNC_PASSWORD"}
        return subprocess.run([self.rsync, "--contimeout=2", "--timeout=3", *args],
                              input=b"", capture_output=True, timeout=8, env=env)

    def test_native_authentication_listing_and_source_allowlist(self):
        with self.daemon() as (root, dirs, password, port):
            listing = self.client(f"rsync://127.0.0.1:{port}/")
            self.assertEqual(listing.returncode, 0)
            self.assertIn(b"joernshtdocs", listing.stdout)
            self.assertNotIn(b"irregular-ninja", listing.stdout)
            source = root / "source"
            source.mkdir()
            (source / "index.html").write_text("fixture album")
            for module, target in (("irregular-ninja", "irregular.ninja"),
                                   ("alt-irregular-ninja", "alt.irregular.ninja")):
                url = f"rsync://shuriken@127.0.0.1:{port}/{module}/"
                denied = self.client("-r", str(source) + "/", url)
                self.assertNotEqual(denied.returncode, 0)
                self.assertFalse((dirs[target] / "index.html").exists())
                wrong = root / "wrong"
                wrong.write_text("wrong-fixture-password\n")
                wrong.chmod(0o600)
                for user, secret, bind in (("shuriken", wrong, "127.0.0.1"),
                                           ("other", password, "127.0.0.1"),
                                           ("shuriken", password, "127.0.0.2")):
                    result = self.client("-r", f"--address={bind}", f"--password-file={secret}",
                                         str(source) + "/", url.replace("shuriken@", user + "@"))
                    self.assertNotEqual(result.returncode, 0)
                    self.assertFalse((dirs[target] / "index.html").exists())
                result = self.client("-r", f"--password-file={password}", str(source) + "/", url)
                self.assertEqual(result.returncode, 0, result.stderr.decode())
                self.assertEqual((dirs[target] / "index.html").read_text(), "fixture album")

    def test_rendered_publisher_authenticates_and_keeps_failed_marker(self):
        with self.daemon() as (root, dirs, password, port):
            data = root / "data"
            for site in ("irregular.ninja", "alt.irregular.ninja"):
                dist = data / site / "dist"
                dist.mkdir(parents=True)
                (dist / "status.json").write_text('{"generated_at": 123}\n')
                (dist / "index.html").write_text("published fixture")
            script = self.publisher.replace("/data/shuriken.sh", str(data))
            script = script.replace("/etc/shuriken-rsync/password", str(password))
            script = script.replace('"rsync://shuriken@$srv/$mod/"', f'"rsync://shuriken@127.0.0.1:{port}/$mod/"')
            password.write_text("wrong-password\n")
            failed = subprocess.run(["bash", "-c", script], capture_output=True, timeout=10)
            self.assertNotEqual(failed.returncode, 0)
            self.assertFalse((data / "irregular.ninja/.last-published-status.json").exists())
            password.write_text(PASSWORD + "\n")
            success = subprocess.run(["bash", "-c", script], capture_output=True, timeout=10)
            self.assertEqual(success.returncode, 0, success.stderr.decode())
            for site in ("irregular.ninja", "alt.irregular.ninja"):
                self.assertEqual((dirs[site] / "index.html").read_text(), "published fixture")
                self.assertEqual((data / site / "dist/status.json").read_bytes(),
                                 (data / site / ".last-published-status.json").read_bytes())

    def test_pull_options_with_native_receiver_and_identity_gate(self):
        with self.daemon() as (root, dirs, _, port):
            source = dirs["joern"]
            (source / "index.txt").write_text("safe content")
            (source / "index.txt").chmod(0o4777)
            (source / "safe-link").symlink_to("index.txt")
            (source / "unsafe-link").symlink_to("/etc/passwd")
            os.mkfifo(source / "fifo")
            destination = root / "received"
            destination.mkdir()
            (destination / "stale").write_text("remove me")
            script = (ROOT / "frontends/scripts/rsync.sh.tpl").read_text()
            bins = root / "bin"
            bins.mkdir()
            for name, body in (("hostname", "echo fishfinger"),
                               ("id", 'if [ "$1" = -u ]; then echo "${TEST_UID:-1001}"; else echo "${TEST_USER:-_joernsync}"; fi')):
                path = bins / name
                path.write_text("#!/bin/sh\n" + body + "\n")
                path.chmod(0o755)
            (bins / "rsync").symlink_to(self.rsync)
            script = script.replace("PATH=/usr/local/bin:/usr/bin:/bin", f"PATH={bins}:/usr/bin:/bin")
            script = script.replace("rsync://192.168.2.110/joernshtdocs/", f"rsync://127.0.0.1:{port}/joernshtdocs/")
            script = script.replace("/var/www/htdocs/joern/", str(destination) + "/")
            denied = subprocess.run(["sh", "-c", script], env={**os.environ, "TEST_UID": "0"}, capture_output=True)
            self.assertNotEqual(denied.returncode, 0)
            self.assertTrue((destination / "stale").exists())
            wrong_user = subprocess.run(["sh", "-c", script],
                                        env={**os.environ, "TEST_USER": "unrelated"}, capture_output=True)
            self.assertNotEqual(wrong_user.returncode, 0)
            self.assertTrue((destination / "stale").exists())
            result = subprocess.run(["sh", "-c", script], capture_output=True, timeout=10)
            self.assertEqual(result.returncode, 0, result.stderr.decode())
            self.assertEqual((destination / "index.txt").read_text(), "safe content")
            self.assertTrue((destination / "safe-link").is_symlink())
            self.assertFalse((destination / "unsafe-link").is_symlink())
            self.assertFalse((destination / "fifo").exists())
            self.assertFalse((destination / "stale").exists())
            self.assertFalse((destination / "index.txt").stat().st_mode & (stat.S_ISUID | stat.S_ISGID))


if __name__ == "__main__":
    unittest.main(verbosity=2)
