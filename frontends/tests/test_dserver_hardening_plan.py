"""Check the actual existing-installation dserver hardening plan."""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]


class DserverHardeningPlanTests(unittest.TestCase):
    def plan(self, task):
        binary = os.environ.get("GONF_TEST_BINARY")
        command = [binary] if binary else ["go", "run", "./cmd/gonf"]
        with tempfile.TemporaryDirectory() as directory:
            result = subprocess.run(
                command + ["plan", "-o", directory, task],
                cwd=ROOT / "gonf",
                env=dict(os.environ, GONF_CONF_ROOT=str(ROOT)),
                capture_output=True, text=True, timeout=60)
            self.assertEqual(result.returncode, 0, result.stderr)
            return [json.loads(line) for line in
                    (Path(directory) / "plan.jsonl").read_text().splitlines()]

    def assert_scoped(self, operations):
        allowed_paths = {
            "/usr/local/bin/dserver-update-key-cache.sh",
            "/etc/dserver/dtail.json", "/etc/rc.d/dserver",
        }
        allowed_operations = {
            "plan", "when_begin", "when_end", "file", "command", "service",
        }
        for operation in operations:
            self.assertIn(operation["op"], allowed_operations)
            if "path" in operation:
                self.assertIn(operation["path"], allowed_paths)
            if operation["op"] == "service":
                self.assertEqual(operation["name"], "dserver")
            if operation["op"] == "command":
                self.assertEqual(
                    operation["bin"],
                    "/usr/local/bin/dserver-update-key-cache.sh")
                self.assertFalse(operation.get("args"))
        configs = [op for op in operations
                   if op.get("path") == "/etc/dserver/dtail.json"]
        self.assertEqual(len(configs), 2)

    def test_hardening_is_scoped_to_existing_dserver(self):
        self.assert_scoped(self.plan("frontends_d_tail_hardening"))

    def test_normal_maintenance_is_not_a_scoped_hardening_plan(self):
        # The real normal task reconciles a package and account. It must not
        # pass the oracle used to approve the narrower live operation.
        with self.assertRaises(AssertionError):
            self.assert_scoped(self.plan("frontends_d_tail"))


if __name__ == "__main__":
    unittest.main()
