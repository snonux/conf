"""Offline render checks and real local nginx negative-route checks.

Requires PyYAML, Helm, Podman and the pinned nginx image already pulled.
HELM_CHART must point to the kube-prometheus-stack 92.10.1 chart archive.
Never contacts Kubernetes or the production Prometheus API.
"""

import http.server
import ipaddress
import os
from pathlib import Path
import socket
import subprocess
import tempfile
import threading
import time
import unittest
import urllib.error
import urllib.request

import yaml


ROOT = Path(__file__).resolve().parents[2]
PROM = ROOT / "prometheus"
PROXY = list(yaml.safe_load_all(
    (PROM / "manifests/prometheus-read-proxy.yaml").read_text()))
POLICIES = list(yaml.safe_load_all(
    (PROM / "manifests/prometheus-networkpolicy.yaml").read_text()))


def allows(policy, labels, address, port, namespace="monitoring"):
    """Evaluate these policies for ordinary pods/clients (not node traffic)."""
    for rule in policy["spec"]["ingress"]:
        if port not in [p["port"] for p in rule["ports"]]:
            continue
        for peer in rule["from"]:
            if "ipBlock" in peer:
                if ipaddress.ip_address(address) in ipaddress.ip_network(
                        peer["ipBlock"]["cidr"]):
                    return True
            elif namespace == "monitoring" and labels is not None and all(
                    labels.get(k) == v for k, v in
                    peer["podSelector"]["matchLabels"].items()):
                return True
    return False


class ManifestTests(unittest.TestCase):
    def test_render_authoritative_and_example_values(self):
        chart = os.environ["HELM_CHART"]
        app = yaml.safe_load((ROOT / "argocd-apps/monitoring/prometheus.yaml")
                             .read_text())
        values = app["spec"]["sources"][0]["helm"]["valuesObject"]
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "values.yaml"
            path.write_text(yaml.safe_dump(values))
            for source in (path, PROM / "persistence-values.yaml"):
                rendered = subprocess.check_output([
                    "helm", "template", "prometheus", chart,
                    "--namespace", "monitoring", "--values", str(source)
                ], text=True)
                prom = next(d for d in yaml.safe_load_all(rendered)
                            if d and d["kind"] == "Prometheus")
                spec = prom["spec"]
                self.assertFalse(spec["enableAdminAPI"])
                self.assertFalse(spec["enableOTLPReceiver"])
                self.assertTrue(spec["enableRemoteWriteReceiver"])
                self.assertFalse(spec.get("additionalArgs"))
                self.assertNotIn("otlp-write-receiver", spec["enableFeatures"])
                self.assertEqual(spec["tsdb"]["outOfOrderTimeWindow"], "744h")
                self.assertEqual(spec["storage"]["volumeClaimTemplate"]
                                 ["spec"]["resources"]["requests"]
                                 ["storage"], "10Gi")

    def test_raw_receiver_source_isolation(self):
        raw = POLICIES[0]
        proxy = {"app.kubernetes.io/name": "prometheus-read-proxy"}
        own = raw["spec"]["podSelector"]["matchLabels"]
        self.assertTrue(allows(raw, proxy, "10.42.0.10", 9090))
        self.assertTrue(allows(raw, own, "10.42.0.11", 9090))
        for labels, address in (({}, "10.42.0.12"),
                                (None, "192.168.1.156"),
                                (None, "192.168.2.110")):
            self.assertFalse(allows(raw, labels, address, 9090))
        self.assertFalse(allows(raw, proxy, "10.42.0.10", 8080))
        self.assertFalse(allows(raw, proxy, "10.42.0.10", 9090, "services"))
        self.assertEqual(raw["spec"]["policyTypes"], ["Ingress"])

    def test_proxy_sources_and_node_local_endpoints(self):
        for address in ("192.168.2.110", "192.168.2.111", "192.168.2.200"):
            self.assertTrue(allows(POLICIES[1], None, address, 8080))
        for address in ("192.168.1.156", "192.168.2.201", "10.42.0.12"):
            self.assertFalse(allows(POLICIES[1], {}, address, 8080))
        service = yaml.safe_load(
            (PROM / "manifests/prometheus-nodeport.yaml").read_text())
        self.assertEqual(service["spec"]["externalTrafficPolicy"], "Local")
        self.assertEqual(PROXY[1]["kind"], "DaemonSet")
        template = PROXY[1]["spec"]["template"]
        self.assertEqual(service["spec"]["selector"],
                         template["metadata"]["labels"])
        container = template["spec"]["containers"][0]
        self.assertEqual(service["spec"]["ports"][0]["targetPort"],
                         container["ports"][0]["name"])


class ProxyTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.hits = []
        cls.fail_upstream = False

        class Backend(http.server.BaseHTTPRequestHandler):
            def do_GET(self):
                cls.hits.append(self.path)
                if cls.fail_upstream:
                    # Trigger nginx's request-context upstream error logging.
                    self.connection.shutdown(socket.SHUT_RDWR)
                    self.connection.close()
                    return
                self.send_response(200)
                self.end_headers()
                self.wfile.write(b'{"status":"success","data":{}}')

            def do_HEAD(self):
                cls.hits.append(self.path)
                self.send_response(200)
                self.end_headers()

            def log_message(self, *_args):
                pass

        cls.backend = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Backend)
        threading.Thread(target=cls.backend.serve_forever, daemon=True).start()
        with socket.socket() as sock:
            sock.bind(("127.0.0.1", 0))
            cls.port = sock.getsockname()[1]
        cls.directory = tempfile.TemporaryDirectory()
        os.chmod(cls.directory.name, 0o755)
        config = PROXY[0]["data"]["nginx.conf"].replace(
            "prometheus-kube-prometheus-prometheus.monitoring.svc.cluster.local:9090",
            f"127.0.0.1:{cls.backend.server_port}").replace(
                "listen 8080;", f"listen 127.0.0.1:{cls.port};")
        path = Path(cls.directory.name) / "nginx.conf"
        path.write_text(config)
        image = PROXY[1]["spec"]["template"]["spec"]["containers"][0]["image"]
        cls.command = [
            "podman", "run", "--rm", "--pull=never", "--network=host",
            "--user=101:101", "--read-only", "--cap-drop=ALL",
            "--security-opt=no-new-privileges", "--tmpfs=/tmp:rw,size=16m",
            "--volume", f"{path}:/etc/nginx/nginx.conf:ro,Z",
            "--entrypoint=nginx", image, "-g", "daemon off;"
        ]
        cls.logs = tempfile.TemporaryFile()
        cls.process = subprocess.Popen(
            cls.command, stdout=cls.logs, stderr=cls.logs)
        cls.addClassCleanup(cls.cleanup)
        for _ in range(100):
            if cls.process.poll() is not None:
                raise RuntimeError(os.pread(cls.logs.fileno(), 65536, 0).decode())
            try:
                if cls.request("GET", "/-/ready") == 200:
                    return
            except urllib.error.URLError:
                time.sleep(0.1)
        raise RuntimeError("local nginx did not start")

    @classmethod
    def cleanup(cls):
        cls.process.terminate()
        cls.process.communicate(timeout=15)
        cls.logs.close()
        cls.backend.shutdown()
        cls.backend.server_close()
        cls.directory.cleanup()

    @classmethod
    def request(cls, method, path):
        request = urllib.request.Request(
            f"http://127.0.0.1:{cls.port}{path}", method=method)
        try:
            with urllib.request.urlopen(request, timeout=3) as response:
                return response.status
        except urllib.error.HTTPError as error:
            with error:
                return error.code

    def test_reads_and_query_parameters(self):
        for path in ("/api/v1/alerts", "/api/v1/query?query=up",
                     "/api/v1/query_range?query=up&start=1&end=2&step=1",
                     "/api/v1/label/job/values", "/api/v1/rules", "/-/ready"):
            with self.subTest(path=path):
                self.assertEqual(self.request("GET", path), 200)
                self.assertIn(path, self.hits)
        self.assertEqual(self.request("HEAD", "/api/v1/alerts"), 200)

    def test_upstream_failure_does_not_log_private_query(self):
        sentinel = "synthetic-private-label-6n3"
        path = "/api/v1/query?query=up%7Bprivate%3D%22" + sentinel + "%22%7D"
        type(self).fail_upstream = True
        try:
            self.assertEqual(self.request("GET", path), 502)
        finally:
            type(self).fail_upstream = False
        self.assertIn(path, self.hits)
        logs = os.pread(self.logs.fileno(), 65536, 0)
        self.assertNotIn(sentinel.encode(), logs)
        self.assertNotIn(path.encode(), logs)

    def test_startup_diagnostics_are_retained(self):
        path = Path(self.directory.name)
        invalid = path / "invalid-nginx.conf"
        invalid.write_text((path / "nginx.conf").read_text()
                           + "\ninvalid_startup_directive;\n")
        # Fail after parsing the production log configuration, not before it.
        command = self.command[:-2] + ["-t"]
        command[command.index("--volume") + 1] = (
            f"{invalid}:/etc/nginx/nginx.conf:ro,Z")
        result = subprocess.run(command, capture_output=True, timeout=15)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn(b'unknown directive "invalid_startup_directive"',
                      result.stdout + result.stderr)

    def test_mutation_and_unknown_routes_never_reach_upstream(self):
        for method in ("GET", "HEAD", "POST", "PUT", "DELETE", "OPTIONS"):
            for path in ("/api/v1/admin/tsdb/delete_series?match[]=up",
                         "/api/v1/admin/tsdb/clean_tombstones",
                         "/api/v1/write", "/api/v1/otlp/v1/metrics",
                         "/-/reload", "/-/quit", "/api/v1/status/config",
                         "/api/v1/status/flags", "/api/v1/query/../write",
                         "/api/v1/%77rite", "/unknown"):
                with self.subTest(method=method, path=path):
                    before = len(self.hits)
                    self.assertEqual(self.request(method, path),
                                     404 if method in ("GET", "HEAD") else 405)
                    self.assertEqual(len(self.hits), before)
        self.assertEqual(self.request("POST", "/api/v1/query"), 405)


if __name__ == "__main__":
    unittest.main()
