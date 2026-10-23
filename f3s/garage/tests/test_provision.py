"""Test the credential boundary with real kubectl and a private HTTP API fixture.

No real vault or cluster is contacted. The fake credential provider supplies
only generated disposable data; API requests use native kubectl transport.
"""
import base64
import http.server
import json
import os
from pathlib import Path
import secrets
import shlex
import shutil
import subprocess
import tempfile
import threading
import unittest

SCRIPT = Path(__file__).resolve().parents[1] / "scripts/provision-metrics-secret.py"


class ProvisionTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="cn3-secret-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.root.chmod(0o700)
        self.token = secrets.token_hex(32).encode()
        self.stored = None
        self.created = []
        self.reject = False
        self.raw_token = self.token
        self.server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), self.handler())
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()
        self.addCleanup(self.close_server)
        config = {"apiVersion": "v1", "kind": "Config", "current-context": "fixture",
                  "clusters": [{"name": "fixture", "cluster": {
                      "server": f"http://127.0.0.1:{self.server.server_port}"}}],
                  "contexts": [{"name": "fixture", "context": {"cluster": "fixture"}}]}
        (self.root / "kubeconfig").write_text(json.dumps(config))
        self.bin = self.root / "bin"
        self.bin.mkdir()
        binary = shutil.which("kubectl")
        self.assertTrue(binary, "native kubectl required")
        wrapper = self.bin / "kubectl"
        wrapper.write_text("#!/bin/sh\ncase \"$1\" in create) set -- \"$@\" --validate=false;; esac\nexec " +
                           shlex.quote(binary) + " --cache-dir=" +
                           shlex.quote(str(self.root / "cache")) + " \"$@\"\n")
        wrapper.chmod(0o700)
        provider = self.bin / "foostore"
        provider.write_text("#!/bin/sh\ncat " + shlex.quote(str(self.root / "credential")) + "\n")
        provider.chmod(0o700)

    def close_server(self):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(timeout=5)

    def handler(self):
        fixture = self

        class API(http.server.BaseHTTPRequestHandler):
            def reply(self, status, obj):
                body = json.dumps(obj).encode()
                self.send_response(status)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)

            def do_GET(self):
                path = self.path.split("?", 1)[0]
                if path == "/api":
                    self.reply(200, {"kind": "APIVersions", "apiVersion": "v1", "versions": ["v1"],
                                     "serverAddressByClientCIDRs": []})
                elif path == "/apis":
                    self.reply(200, {"kind": "APIGroupList", "apiVersion": "v1", "groups": []})
                elif path == "/api/v1":
                    self.reply(200, {"kind": "APIResourceList", "apiVersion": "v1", "groupVersion": "v1",
                                     "resources": [{"name": "secrets", "singularName": "secret",
                                                    "namespaced": True, "kind": "Secret",
                                                    "verbs": ["get", "create"]}]})
                elif path.endswith("/namespaces/monitoring/secrets/garage-metrics") and fixture.stored:
                    self.reply(200, fixture.stored)
                else:
                    self.reply(404, {"kind": "Status", "apiVersion": "v1", "status": "Failure",
                                     "reason": "NotFound", "code": 404})

            def do_POST(self):
                obj = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
                if fixture.reject:
                    self.reply(403, {"kind": "Status", "apiVersion": "v1", "status": "Failure",
                                     "reason": "Forbidden", "code": 403,
                                     "message": fixture.token.decode()})
                    return
                fixture.created.append(obj)
                fixture.stored = obj
                self.reply(201, obj)

            def log_message(self, *_args):
                pass

        return API

    def run_script(self, apply=False):
        p = self.root / "credential"
        p.write_bytes(self.raw_token)
        p.chmod(0o600)
        env = {**os.environ, "PATH": str(self.bin) + os.pathsep + os.environ["PATH"],
               "KUBECONFIG": str(self.root / "kubeconfig")}
        result = subprocess.run(["python3", str(SCRIPT), *(["--apply"] if apply else [])],
                                env=env, capture_output=True, timeout=15)
        self.assertFalse(self.token in result.stdout + result.stderr,
                         "credential escaped through operation output")
        return result

    def test_check_is_read_only_and_apply_creates_once(self):
        self.assertNotEqual(self.run_script().returncode, 0)
        self.assertEqual(self.created, [])
        self.assertEqual(self.run_script(True).returncode, 0)
        self.assertEqual(len(self.created), 1)
        obj = self.created[0]
        self.assertEqual((obj["kind"], obj["metadata"]["name"], obj["metadata"]["namespace"]),
                         ("Secret", "garage-metrics", "monitoring"))
        self.assertTrue(base64.b64decode(obj["data"]["token"]) == self.token)
        self.assertEqual(self.run_script(True).returncode, 0)
        self.assertEqual(len(self.created), 1)

    def test_existing_different_credential_is_not_rotated(self):
        self.assertEqual(self.run_script(True).returncode, 0)
        self.stored["data"]["token"] = base64.b64encode(b"different-fixture-only").decode()
        self.assertNotEqual(self.run_script(True).returncode, 0)
        self.assertEqual(len(self.created), 1)

    def test_api_error_cannot_print_credential(self):
        self.reject = True
        self.assertNotEqual(self.run_script(True).returncode, 0)
        self.assertEqual(self.created, [])

    def test_provider_failure_does_not_consume_or_print_partial_secret(self):
        provider = self.bin / "foostore"
        credential = shlex.quote(str(self.root / "credential"))
        provider.write_text("#!/bin/sh\ncat " + credential + "\ncat " + credential + " >&2\nexit 6\n")
        self.assertNotEqual(self.run_script(True).returncode, 0)
        self.assertEqual(self.created, [])

    def test_existing_wrong_type_is_not_reused(self):
        self.assertEqual(self.run_script(True).returncode, 0)
        self.stored["type"] = "kubernetes.io/service-account-token"
        self.assertNotEqual(self.run_script(True).returncode, 0)
        self.assertEqual(len(self.created), 1)

    def test_empty_or_header_injection_token_is_rejected_before_api(self):
        for value in (b"", b"bad token", b"bad\nheader"):
            self.raw_token = value
            self.assertNotEqual(self.run_script(True).returncode, 0)
            self.assertEqual(self.created, [])


if __name__ == "__main__":
    unittest.main()
