"""Exercise rendered recipe settings against the matching real Garage binary.

GARAGE_TEST_BINARY points to a local Garage 2.4.1 binary. All state and keys
are disposable, and every fixture listener binds only to loopback.
"""
from contextlib import contextmanager
import hmac
import json
import os
from pathlib import Path
import secrets
import socket
import subprocess
import tempfile
import time
import tomllib
import unittest
import urllib.error
import urllib.request

import yaml

ROOT = Path(__file__).resolve().parents[3]


def free_port():
    with socket.socket() as sock:
        sock.bind(("127.0.0.2", 0))
        return sock.getsockname()[1]


class MetricsTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.workspace = tempfile.TemporaryDirectory(prefix="cn3-fixture-")
        cls.addClassCleanup(cls.workspace.cleanup)
        cls.driver = Path(cls.workspace.name) / "record"
        subprocess.run(["go", "build", "-p", "1", "-o", str(cls.driver),
                        str(Path(__file__).with_name("fixture.go"))],
                       cwd=ROOT / "gonf", check=True, capture_output=True)
        cls.garage = os.environ["GARAGE_TEST_BINARY"]
        cls.opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))

    def record(self, render=False, **overrides):
        values = {"Metrics": "fixture-only-never-production", "Admin": "127.0.0.2:3903",
                  "RPC": "127.0.0.2:3901", "Render": render, **overrides}
        return subprocess.run([str(self.driver)], input=json.dumps(values), text=True,
                              capture_output=True, cwd=ROOT / "gonf", timeout=30,
                              env={**os.environ, "GONF_CONF_ROOT": str(ROOT)})

    def test_actual_plan_marks_credentials_sensitive(self):
        result = self.record()
        self.assertEqual(result.returncode, 0, "fixture plan failed")
        ops = [json.loads(line) for line in result.stdout.splitlines()]
        files = [op for op in ops if op["op"] == "file"]
        self.assertEqual(len(files), 1)
        self.assertTrue(files[0]["sensitive"])
        self.assertEqual((files[0]["owner"], files[0]["group"], files[0]["mode"]),
                         ("root", "garage", "0640"))
        service = next(op for op in ops if op["op"] == "service")
        self.assertIn(files[0]["id"], service["deps"])

    def test_missing_failed_empty_or_unsafe_credentials_refuse_plan(self):
        for overrides in ({"Mode": "missing"}, {"Mode": "failure"}, {"Metrics": ""},
                          {"Metrics": "bad token"}, {"Metrics": "bad\nheader"}):
            with self.subTest(mode=next(iter(overrides))):
                result = self.record(**overrides)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(result.stdout, "")

    def test_wildcard_public_or_malformed_admin_listener_refused(self):
        for address in ("0.0.0.0:3903", "[::]:3903", "8.8.8.8:3903", "localhost:3903",
                        "192.168.50.1:0", "127.0.0.1:65536", ""):
            with self.subTest(address=address):
                result = self.record(Admin=address)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(result.stdout, "")

    @contextmanager
    def running_garage(self, omit_metrics=False):
        with tempfile.TemporaryDirectory(prefix="cn3-garage-") as directory:
            root = Path(directory)
            root.chmod(0o700)
            metrics = secrets.token_hex(32)
            admin = secrets.token_hex(32)
            http, rpc, s3 = free_port(), free_port(), free_port()
            result = self.record(True, Metrics=metrics, Admin=f"127.0.0.2:{http}",
                                 RPC=f"127.0.0.2:{rpc}")
            self.assertEqual(result.returncode, 0, "native fixture rendering failed")
            original = tomllib.loads(result.stdout)
            self.assertEqual(original["replication_factor"], 3)
            self.assertEqual(original["rpc_bind_addr"], "0.0.0.0:3901")
            self.assertEqual(original["s3_api"]["api_bind_addr"], "0.0.0.0:3900")
            self.assertTrue(hmac.compare_digest(original["admin"]["metrics_token"], metrics))
            # Only isolate storage/listeners and use a one-node test layout.
            config = result.stdout.replace("replication_factor = 3", "replication_factor = 1")
            config = config.replace("/var/db/garage/meta", str(root / "meta"))
            config = config.replace("/var/db/garage/data", str(root / "data"))
            config = config.replace("0.0.0.0:3901", f"127.0.0.2:{rpc}")
            config = config.replace("0.0.0.0:3900", f"127.0.0.2:{s3}")
            config += '\nadmin_token = ' + json.dumps(admin) + '\n'
            if omit_metrics:
                config = '\n'.join(x for x in config.splitlines() if not x.startswith("metrics_token ="))
            path = root / "garage.toml"
            path.write_text(config)
            path.chmod(0o600)
            process = subprocess.Popen([self.garage, "-c", str(path), "server"],
                                       stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            try:
                for _ in range(100):
                    self.assertIsNone(process.poll(), "native Garage exited before readiness")
                    try:
                        self.http(http, "/health")
                        break
                    except OSError:
                        time.sleep(.05)
                else:
                    self.fail("native Garage HTTP listener did not start")
                node = self.command(path, "node", "id").split("@", 1)[0].strip()
                self.command(path, "layout", "assign", "-z", "fixture", "-c", "1G", node)
                self.command(path, "layout", "apply", "--version", "1")
                for _ in range(100):
                    if self.http(http, "/health")[0] == 200:
                        break
                    time.sleep(.05)
                else:
                    self.fail("native one-node fixture failed its health check")
                yield http, metrics, admin
            finally:
                process.terminate()
                try:
                    process.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait()

    def command(self, path, *args):
        p = subprocess.run([self.garage, "-c", str(path), *args], capture_output=True,
                           text=True, timeout=10)
        self.assertEqual(p.returncode, 0, "native Garage fixture command failed")
        return p.stdout

    def http(self, port, path, token=None, ip="127.0.0.2"):
        headers = {} if token is None else {"Authorization": "Bearer " + token}
        request = urllib.request.Request(f"http://{ip}:{port}{path}", headers=headers)
        try:
            response = self.opener.open(request, timeout=2)
        except urllib.error.HTTPError as exc:
            response = exc
        with response:
            return response.status, response.read()

    def test_native_metrics_auth_health_and_admin_scope(self):
        with self.running_garage() as (port, token, admin):
            self.assertEqual(self.http(port, "/metrics")[0], 403)
            self.assertEqual(self.http(port, "/metrics", "wrong-fixture-token")[0], 403)
            status, body = self.http(port, "/metrics", token)
            self.assertEqual(status, 200)
            self.assertTrue(b"garage_" in body)
            self.assertEqual(self.http(port, "/health")[0], 200)
            self.assertEqual(self.http(port, "/v2/GetClusterStatus", token)[0], 403)
            self.assertEqual(self.http(port, "/v2/GetClusterStatus", admin)[0], 200)
            with self.assertRaises(OSError):
                self.http(port, "/metrics", token, ip="127.0.0.1")

    def test_actual_prometheus_scrapes_with_file_credential(self):
        binary = os.environ["PROMETHEUS_TEST_BINARY"]
        plain = yaml.safe_load((ROOT / "f3s/prometheus/additional-scrape-configs.yaml").read_text())
        secret = yaml.safe_load((ROOT / "f3s/prometheus/manifests/additional-scrape-configs-secret.yaml").read_text())
        embedded = yaml.safe_load(secret["stringData"]["additional-scrape-configs.yaml"])
        job = next(j for j in plain if j["job_name"] == "garage")
        self.assertEqual(job, next(j for j in embedded if j["job_name"] == "garage"))
        self.assertEqual(job["authorization"], {"type": "Bearer", "credentials_file":
                         "/etc/prometheus/secrets/garage-metrics/token"})
        with self.running_garage() as (port, token, _), tempfile.TemporaryDirectory(prefix="cn3-prom-") as directory:
            root = Path(directory)
            root.chmod(0o700)
            credential = root / "token"
            credential.write_text(token)
            credential.chmod(0o600)
            job["authorization"]["credentials_file"] = str(credential)
            job["static_configs"] = [{"targets": [f"127.0.0.2:{port}"]}]
            cfg = root / "prometheus.yml"
            cfg.write_text(yaml.safe_dump({"global": {"scrape_interval": "1s"}, "scrape_configs": [job]}))
            listen = free_port()
            process = subprocess.Popen([binary, "--config.file=" + str(cfg),
                                        "--storage.tsdb.path=" + str(root / "tsdb"),
                                        f"--web.listen-address=127.0.0.2:{listen}"],
                                       stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            try:
                for _ in range(150):
                    self.assertIsNone(process.poll(), "native Prometheus fixture exited")
                    try:
                        status, body = self.http(listen, "/api/v1/targets")
                        targets = json.loads(body)["data"]["activeTargets"] if status == 200 else []
                        if len(targets) == 1 and targets[0]["health"] == "up":
                            break
                    except (OSError, KeyError, ValueError):
                        pass
                    time.sleep(.1)
                else:
                    self.fail("native Prometheus never scraped authenticated Garage metrics")
                self.assertEqual(targets[0]["labels"]["job"], "garage")
                self.assertEqual(self.http(port, "/metrics")[0], 403)
            finally:
                process.terminate()
                try:
                    process.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait()

    def test_native_metrics_remain_closed_when_token_missing(self):
        with self.running_garage(omit_metrics=True) as (port, token, _):
            self.assertEqual(self.http(port, "/metrics")[0], 403)
            self.assertEqual(self.http(port, "/metrics", token)[0], 403)
            self.assertEqual(self.http(port, "/health")[0], 200)


if __name__ == "__main__":
    unittest.main()
