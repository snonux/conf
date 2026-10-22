"""Check real parsing, HTTP/subprocess I/O, publication and kernel comparisons."""

import contextlib
import http.server
import importlib.util
import io
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import threading
import unittest
from unittest.mock import patch


ROOT = Path(__file__).resolve().parents[4]
SOURCE = ROOT / "f3s/r-nodes/k3s/k3s-version-metrics.py"
KERNEL = ROOT / "f3s/freebsd-hosts/node-exporter/kernel-reboot-metrics.sh"
SPEC = importlib.util.spec_from_file_location("version_metrics", SOURCE)
METRICS = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(METRICS)
NODE = "r0.lan.buetow.org"


def channels(patch_version="v1.36.5+k3s1", stable="v1.36.5+k3s1"):
    return {"data": [{"id": "v1.36", "latest": patch_version},
                     {"id": "stable", "latest": stable}]}


class VersionTests(unittest.TestCase):
    def test_release_comparisons(self):
        cases = [
            ("v1.36.5+k3s1", channels(), 0, 0),
            ("v1.36.4+k3s1", channels(), 1, 0),
            ("v1.36.5+k3s1", channels("v1.36.5+k3s2"), 1, 0),
            ("v1.36.5+k3s3", channels(), 0, 0),
            ("v1.36.5+k3s1", channels(stable="v1.37.1+k3s1"), 0, 1),
            ("v1.36.5+k3s1", channels(stable="v1.35.9+k3s1"), 0, 0),
        ]
        for installed, data, patch_update, minor_update in cases:
            with self.subTest(installed=installed, data=data):
                result = "\n".join(METRICS.release_metrics(installed, data, NODE, 100))
                self.assertIn(f'f3s_k3s_update_available{{node="{NODE}"}} {patch_update}', result)
                self.assertIn(f'f3s_k3s_minor_update_available{{node="{NODE}"}} {minor_update}', result)
                self.assertIn("check_success", result)

    def test_invalid_channels_and_versions_refused(self):
        bad = [
            {}, {"data": []}, {"data": [None]},
            channels("v1.37.1+k3s1"), channels("v1.36.6-rc1+k3s1"),
            channels(stable='v1.36.5+k3s1"}\nbogus 1'),
            {"data": channels()["data"] * 2},
        ]
        for data in bad:
            with self.subTest(data=data), self.assertRaises((ValueError, KeyError, TypeError)):
                METRICS.release_metrics("v1.36.5+k3s1", data, NODE, 100)
        for version in ("1.36.5", "v1.36.5+k3s1\n", "v1.36.5-rc1+k3s1"):
            with self.subTest(version=version), self.assertRaises(ValueError):
                METRICS.parse_version(version)

    def test_real_http_and_version_subprocess(self):
        class Handler(http.server.BaseHTTPRequestHandler):
            def do_GET(self):
                self.send_response(200)
                self.end_headers()
                self.wfile.write(json.dumps(channels()).encode())

            def log_message(self, *_args):
                pass

        server = http.server.HTTPServer(("127.0.0.1", 0), Handler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            endpoint = f"http://127.0.0.1:{server.server_port}/channels"
            command = [sys.executable, "-c", "print('k3s version v1.36.4+k3s1 (fixture)')"]
            with patch.object(METRICS, "CHANNELS_URL", endpoint):
                result = "\n".join(METRICS.collect(NODE, 100, command))
                self.assertIn('update_available{node="r0.lan.buetow.org"} 1', result)
                with self.assertRaises(subprocess.CalledProcessError):
                    METRICS.collect(NODE, 100, [sys.executable, "-c", "raise SystemExit(1)"])
        finally:
            server.shutdown()
            server.server_close()
            thread.join()

    def test_failed_check_replaces_old_success_atomically(self):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "result.prom"
            METRICS.publish(output, "old_success 1\n")
            with patch.object(METRICS, "OUTPUT", output), \
                    patch.object(METRICS.socket, "gethostname", return_value=NODE), \
                    patch.object(METRICS, "collect", side_effect=ValueError("private response")), \
                    patch.object(sys, "argv", [str(SOURCE)]), \
                    contextlib.redirect_stderr(io.StringIO()) as errors:
                self.assertEqual(METRICS.main(), 1)
            self.assertNotIn("private response", errors.getvalue())
            text = output.read_text()
            self.assertIn('check_success{node="r0.lan.buetow.org"} 0', text)
            self.assertNotIn("update_available", text)
            self.assertNotIn("old_success", text)
            self.assertEqual(output.stat().st_mode & 0o777, 0o644)
            self.assertEqual(list(Path(directory).iterdir()), [output])

    def test_publication_failure_preserves_previous_file(self):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "result.prom"
            output.write_text("previous\n")
            with patch.object(METRICS.os, "replace", side_effect=OSError("fixture")), \
                    self.assertRaises(OSError):
                METRICS.publish(output, "replacement\n")
            self.assertEqual(output.read_text(), "previous\n")
            self.assertEqual(list(Path(directory).iterdir()), [output])


class KernelTests(unittest.TestCase):
    def test_native_shell_comparisons_and_failures(self):
        with tempfile.TemporaryDirectory() as directory:
            tool = Path(directory) / "freebsd-version"
            tool.write_text('#!/bin/sh\ncase "$1" in\n'
                            '-k) [ "${FAIL_K:-0}" = 0 ] || exit 1; printf "%s\\n" "$INSTALLED";;\n'
                            '-r) [ "${FAIL_R:-0}" = 0 ] || exit 1; printf "%s\\n" "$RUNNING";;\nesac\n')
            tool.chmod(0o755)
            cases = [
                ({"INSTALLED": "15.1-RELEASE-p2", "RUNNING": "15.1-RELEASE-p2"}, 0, 0),
                ({"INSTALLED": "15.1-RELEASE-p2", "RUNNING": "15.1-RELEASE-p1"}, 0, 1),
                ({"INSTALLED": "15.1-RELEASE-p2", "RUNNING": "15.1-RELEASE-p1", "FAIL_K": "1"}, 1, None),
                ({"INSTALLED": "15.1-RELEASE-p2", "RUNNING": "", "FAIL_R": "1"}, 1, None),
                ({"INSTALLED": "garbage", "RUNNING": "garbage"}, 1, None),
                ({"INSTALLED": "15.1-RELEASE-p2\ngarbage", "RUNNING": "15.1-RELEASE-p2"}, 1, None),
                ({"INSTALLED": "", "RUNNING": ""}, 1, None),
            ]
            for values, status, pending in cases:
                with self.subTest(values=values):
                    env = {**os.environ, "KERNEL_METRICS_TEST_PATH": directory + os.pathsep + os.defpath,
                           "FAIL_K": "0", "FAIL_R": "0", **values}
                    result = subprocess.run(["sh", str(KERNEL), "--stdout"],
                                            env=env, capture_output=True, text=True, timeout=5)
                    self.assertEqual(result.returncode, status, result.stderr)
                    self.assertIn("kernel_check_success " + str(int(status == 0)), result.stdout)
                    if pending is None:
                        self.assertNotIn("reboot_pending", result.stdout)
                    else:
                        self.assertIn("kernel_reboot_pending " + str(pending), result.stdout)


if __name__ == "__main__":
    unittest.main()
