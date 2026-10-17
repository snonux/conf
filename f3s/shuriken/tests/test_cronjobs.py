"""Exercise the rendered CronJob scripts with fake generation and rsync.

Run: python3 -m unittest discover -s f3s/shuriken/tests -v
Requires Helm and PyYAML. No cluster, images, or network access is used.
"""

import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

import yaml


class CronJobTests(unittest.TestCase):
    NOW = 1800000000  # An exact hour, matching the persisted success marker.

    @classmethod
    def setUpClass(cls):
        chart = Path(__file__).resolve().parents[1] / "helm-chart"
        rendered = subprocess.check_output(["helm", "template", str(chart)], text=True)
        cls.scripts = {}
        for resource in yaml.safe_load_all(rendered):
            if resource and resource["kind"] == "CronJob":
                container = resource["spec"]["jobTemplate"]["spec"]["template"]["spec"]["containers"][0]
                cls.scripts[resource["metadata"]["name"]] = (
                    container["command"], container["args"][0]
                )

    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.data = self.root / "data"
        self.data.mkdir()
        self.bin = self.root / "bin"
        self.bin.mkdir()
        self.log = self.root / "calls"
        self.env = {
            **os.environ,
            "PATH": f"{self.bin}:{os.environ['PATH']}",
            "NOW": str(self.NOW),
            "CALL_LOG": str(self.log),
        }
        self.stub("date", 'printf "%s\\n" "$NOW"')
        self.stub("shuriken-entrypoint", 'echo generate >> "$CALL_LOG"; exit "${GENERATE_EXIT:-0}"')
        self.stub("rsync", '''
            echo "$*" >> "$CALL_LOG"
            if [ -n "${FAIL_RSYNC_HOST:-}" ]; then
                case "$*" in *"$FAIL_RSYNC_HOST"*) exit 1;; esac
            fi
        ''')

    def stub(self, name, body):
        path = self.bin / name
        path.write_text("#!/bin/sh\nset -eu\n" + body + "\n")
        path.chmod(0o755)

    def run_job(self, name, expected=0):
        command, script = self.scripts[name]
        script = script.replace("/data/shuriken.sh", str(self.data))
        result = subprocess.run(command + [script], env=self.env, text=True, capture_output=True)
        self.assertEqual(result.returncode, expected, result.stdout + result.stderr)
        return result.stdout

    def calls(self):
        return self.log.read_text().splitlines() if self.log.exists() else []

    def marker(self, name, value):
        (self.data / name).write_text(str(value) + "\n")

    def status(self, epoch, site="irregular.ninja"):
        directory = self.data / site
        (directory / "dist").mkdir(parents=True, exist_ok=True)
        status = directory / "dist/status.json"
        status.write_text(json.dumps({
            "generated_at": {"unix_epoch": epoch},
            "image_count": 632,
            "total_size_bytes": 3329590718,
        }, indent=2) + "\n")
        return status, directory / ".last-published-status.json"

    def test_generate_waits_full_three_days(self):
        self.marker(".last-generate-run", self.NOW - 259199)
        self.assertIn("skipping", self.run_job("shuriken"))
        self.assertEqual(self.calls(), [])
        self.marker(".last-generate-run", self.NOW - 259200)
        self.run_job("shuriken")
        self.assertEqual(self.calls(), ["generate"])
        self.assertEqual((self.data / ".last-generate-run").read_text(), f"{self.NOW}\n")
        self.assertFalse((self.data / ".last-generate-fail").exists())

    def test_generate_missing_or_invalid_success_marker(self):
        for value in (None, "invalid", "01800000000", self.NOW + 3600):
            with self.subTest(marker=value):
                if value is not None:
                    self.marker(".last-generate-run", value)
                self.run_job("shuriken")
        self.assertEqual(self.calls(), ["generate"] * 4)

    def test_generate_failure_retains_success_marker_and_backs_off(self):
        previous = self.NOW - 259200
        self.marker(".last-generate-run", previous)
        self.env["GENERATE_EXIT"] = "1"
        self.run_job("shuriken", expected=1)
        self.assertEqual((self.data / ".last-generate-run").read_text(), f"{previous}\n")
        self.assertEqual((self.data / ".last-generate-fail").read_text(), f"{self.NOW}\n")
        self.env["NOW"] = str(self.NOW + 14399)
        self.assertIn("backoff", self.run_job("shuriken"))
        self.env["NOW"] = str(self.NOW + 14400)
        self.env["GENERATE_EXIT"] = "0"
        self.run_job("shuriken")
        self.assertEqual(self.calls(), ["generate", "generate"])
        self.assertFalse((self.data / ".last-generate-fail").exists())

    def test_sync_publishes_reshuffle_with_identical_source_statistics(self):
        for site in ("irregular.ninja", "alt.irregular.ninja"):
            status, backup = self.status(self.NOW - 259200, site)
            backup.write_bytes(status.read_bytes())
            self.status(self.NOW, site)
        self.run_job("shuriken-sync")
        self.assertEqual(len(self.calls()), 4)
        for site in ("irregular.ninja", "alt.irregular.ninja"):
            status, backup = self.status(self.NOW, site)
            self.assertEqual(status.read_bytes(), backup.read_bytes())
        self.run_job("shuriken-sync")
        self.assertEqual(len(self.calls()), 4)  # Repeated tick is a no-op.

    def test_sync_failure_on_second_mirror_retries_both(self):
        status, backup = self.status(self.NOW - 259200)
        backup.write_bytes(status.read_bytes())
        old = backup.read_bytes()
        self.status(self.NOW)
        self.env["FAIL_RSYNC_HOST"] = "blowfish.wg0"
        self.run_job("shuriken-sync", expected=1)
        self.assertEqual(backup.read_bytes(), old)
        self.env.pop("FAIL_RSYNC_HOST")
        self.run_job("shuriken-sync")
        self.assertEqual(len(self.calls()), 4)
        self.assertEqual(status.read_bytes(), backup.read_bytes())

    def test_sync_first_publish_and_absent_status(self):
        status, backup = self.status(self.NOW)
        self.run_job("shuriken-sync")
        self.assertEqual(len(self.calls()), 2)
        self.assertEqual(status.read_bytes(), backup.read_bytes())
        status.unlink()  # Generator removes this until success.
        self.run_job("shuriken-sync")
        self.assertEqual(len(self.calls()), 2)


if __name__ == "__main__":
    unittest.main()
