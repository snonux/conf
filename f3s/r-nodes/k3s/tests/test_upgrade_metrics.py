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


class KernelPublicationTests(unittest.TestCase):
    """Exercise the nobody-owned directory boundary using synthetic files.

    Hooks deterministically perform the directory owner's allowed replacement
    at the vulnerable command boundary. FreeBSD stat is adapted on Linux;
    file writes, permissions, rename, symlinks and cleanup are real.
    """

    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        for name in ("bin", "metrics", "private-staging", "private-directory"):
            (self.root / name).mkdir(mode=0o700)
        self.metric = self.root / "metrics/kernel_reboot.prom"
        self.private = self.root / "synthetic-private"
        self.private.write_text("synthetic private data\n")
        self.private.chmod(0o600)
        self.env = {**os.environ,
                    "KERNEL_METRICS_TEST_PATH": str(self.root / "bin") + os.pathsep + os.defpath,
                    "KERNEL_METRICS_TEST_OUTPUT": str(self.metric),
                    "KERNEL_METRICS_TEST_PUBLISH_PARENT": str(self.root / "private-staging"),
                    "FIXTURE_ROOT": str(self.root)}
        self.stub("freebsd-version", 'echo 15.1-RELEASE-p2\n')
        if not sys.platform.startswith("freebsd"):
            self.stub("stat", 'test "$1" = -f && test "$2" = %d\n'
                      'exec python3 -c "import os,sys; print(os.stat(sys.argv[1]).st_dev)" "$3"\n')

    def stub(self, name, body):
        tool = self.root / "bin" / name
        tool.write_text("#!/bin/sh\nset -eu\n" + body)
        tool.chmod(0o755)

    def run_producer(self, status=0):
        result = subprocess.run(["sh", str(KERNEL)], env=self.env,
                                capture_output=True, text=True, timeout=5)
        self.assertEqual(result.returncode, status, result.stderr)
        self.assertEqual(list((self.root / "private-staging").iterdir()), [])
        self.assertEqual(self.private.read_text(), "synthetic private data\n")
        self.assertEqual(self.private.stat().st_mode & 0o777, 0o600)
        return result

    def test_private_staging_avoids_public_write_and_chmod_races(self):
        hooks = {
            "mktemp": 'p=$(/usr/bin/mktemp "$@")\n'
                      'case "$p" in "$FIXTURE_ROOT"/metrics/*)\n'
                      'rm -f "$p"; ln -s "$FIXTURE_ROOT/synthetic-private" "$p";; esac\n'
                      'printf "%s\\n" "$p"\n',
            "chmod": 'case "$2" in "$FIXTURE_ROOT"/metrics/*)\n'
                     'rm -f "$2"; ln -s "$FIXTURE_ROOT/synthetic-private" "$2";; esac\n'
                     'exec /bin/chmod "$@"\n',
        }
        for command, body in hooks.items():
            with self.subTest(boundary=command):
                self.stub(command, body)
                self.run_producer()
                self.assertIn("kernel_check_success 1", self.metric.read_text())
                self.assertFalse(self.metric.is_symlink())
                self.assertEqual(self.metric.stat().st_mode & 0o777, 0o644)
                (self.root / "bin" / command).unlink()

    def test_destination_symlink_to_directory_is_replaced(self):
        target = self.root / "private-directory"
        self.metric.symlink_to(target, target_is_directory=True)
        self.run_producer()
        self.assertEqual(list(target.iterdir()), [])
        self.assertFalse(self.metric.is_symlink())
        self.assertIn("kernel_check_success 1", self.metric.read_text())

    def test_failed_rename_preserves_previous_metrics(self):
        self.metric.write_text("previous 1\n")
        self.stub("mv", 'exit 1\n')
        self.run_producer(status=1)
        self.assertEqual(self.metric.read_text(), "previous 1\n")

    def test_failed_kernel_read_publishes_failed_coverage(self):
        self.metric.write_text("node_freebsd_kernel_check_success 1\n")
        self.stub("freebsd-version", 'exit 1\n')
        self.run_producer(status=1)
        self.assertIn("kernel_check_success 0", self.metric.read_text())
        self.assertNotIn("kernel_reboot_pending", self.metric.read_text())

    def test_cross_filesystem_publication_refused_before_move(self):
        self.metric.write_text("previous 1\n")
        self.stub("stat", 'case "$3" in .) echo 2;; *) echo 1;; esac\n')
        self.stub("mv", 'touch "$FIXTURE_ROOT/unexpected-move"\nexit 1\n')
        self.run_producer(status=1)
        self.assertFalse((self.root / "unexpected-move").exists())
        self.assertEqual(self.metric.read_text(), "previous 1\n")

    def test_collector_symlink_redirect_refused(self):
        self.metric.parent.rmdir()
        self.metric.parent.symlink_to(self.root / "private-directory", target_is_directory=True)
        self.run_producer(status=1)
        self.assertEqual(list((self.root / "private-directory").iterdir()), [])

    def test_directory_is_pinned_before_final_rename(self):
        self.stub("stat", 'if [ "$3" = . ]; then\n'
                  '/bin/mv "$FIXTURE_ROOT/metrics" "$FIXTURE_ROOT/moved-metrics"\n'
                  'ln -s "$FIXTURE_ROOT/private-directory" "$FIXTURE_ROOT/metrics"\n'
                  'fi\nprintf "1\\n"\n')
        self.run_producer()
        self.assertEqual(list((self.root / "private-directory").iterdir()), [])
        self.assertIn("kernel_check_success 1",
                      (self.root / "moved-metrics/kernel_reboot.prom").read_text())

    def test_signal_before_publication_preserves_previous_metrics(self):
        self.metric.write_text("previous 1\n")
        self.stub("chmod", 'kill -TERM "$PPID"\nexec /bin/chmod "$@"\n')
        self.run_producer(status=1)
        self.assertEqual(self.metric.read_text(), "previous 1\n")


if __name__ == "__main__":
    unittest.main()
