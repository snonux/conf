"""Verify native curl sends auth while its process arguments contain no token.

Uses the exact uploader source against a loopback receiver and real readable
uptimed records. Generated credentials are temporary and never printed.
"""
import http.server
import os
from pathlib import Path
import secrets
import subprocess
import tempfile
import threading
import unittest

ROOT = Path(__file__).resolve().parents[2]
CLIENT = ROOT / "frontends/scripts/goprecords-upload-client.sh"


def descendants(pid):
    found = []
    children = Path(f"/proc/{pid}/task/{pid}/children")
    try:
        ids = children.read_text().split()
    except FileNotFoundError:
        return found
    for child in ids:
        found.append(int(child))
        found.extend(descendants(int(child)))
    return found


class HeaderTests(unittest.TestCase):
    def test_header_reaches_receiver_without_argv_or_environment_leak(self):
        if not any(os.access(p, os.R_OK) for p in (
                "/var/spool/uptimed/records", "/var/db/uptimed/records",
                "/usr/local/var/uptimed/records")):
            self.skipTest("requires native readable uptimed records")
        token = secrets.token_hex(32)
        arrived, release = threading.Event(), threading.Event()
        received = []

        class Receiver(http.server.BaseHTTPRequestHandler):
            def do_PUT(self):
                authorized = self.headers.get("Authorization") == "Bearer " + token
                payload = self.rfile.read(int(self.headers.get("Content-Length", "0")))
                received.append((self.path, authorized, len(payload)))
                if len(received) == 1:
                    arrived.set()
                    release.wait(10)
                self.send_response(204 if authorized else 401)
                self.end_headers()

            def log_message(self, *_args):
                pass

        with tempfile.TemporaryDirectory(prefix="dn3-upload-") as directory:
            root = Path(directory)
            root.chmod(0o700)
            credential = root / "token"
            credential.write_text(token + "\n")
            credential.chmod(0o600)
            (root / ".curlrc").write_text("")
            env = {k: v for k, v in os.environ.items()
                   if k.lower() not in ("http_proxy", "https_proxy", "all_proxy")}
            with http.server.ThreadingHTTPServer(("127.0.0.1", 0), Receiver) as server:
                thread = threading.Thread(target=server.serve_forever, daemon=True)
                thread.start()
                env.update(CURL_HOME=str(root), GOPRECORDS_TOKEN_FILE=str(credential),
                           GOPRECORDS_HOST="dn3-fixture",
                           GOPRECORDS_BASE_URL=f"http://127.0.0.1:{server.server_port}")
                process = subprocess.Popen(["sh", str(CLIENT)], env=env,
                                           stdout=subprocess.PIPE, stderr=subprocess.PIPE)
                try:
                    self.assertTrue(arrived.wait(10), "native client did not reach local receiver")
                    curl_args = []
                    for pid in descendants(process.pid):
                        try:
                            args = Path(f"/proc/{pid}/cmdline").read_bytes()
                            environment = Path(f"/proc/{pid}/environ").read_bytes()
                        except FileNotFoundError:
                            continue
                        self.assertFalse(token.encode() in args, "credential leaked in process arguments")
                        self.assertFalse(token.encode() in environment, "credential leaked in process environment")
                        if Path(args.split(b"\0")[0].decode()).name == "curl":
                            curl_args.append(args)
                    self.assertEqual(len(curl_args), 1, "did not observe the actual blocked curl process")
                    release.set()
                    stdout, stderr = process.communicate(timeout=20)
                    self.assertEqual(process.returncode, 0, "native client failed")
                    self.assertFalse(token.encode() in stdout + stderr, "credential leaked in client output")
                    kinds = {path.rsplit("/", 1)[-1] for path, _, _ in received}
                    self.assertTrue({"records", "os.txt", "cpuinfo.txt"} <= kinds)
                    self.assertTrue(all(auth and length > 0 for _, auth, length in received))
                finally:
                    release.set()
                    if process.poll() is None:
                        process.terminate()
                        process.communicate(timeout=5)
                    server.shutdown()
                thread.join(timeout=5)


if __name__ == "__main__":
    unittest.main()
