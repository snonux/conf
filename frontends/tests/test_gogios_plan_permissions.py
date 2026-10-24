"""Check the actual plans from every task that publishes Gogios configuration.

Only local plans are rendered. GONF_TEST_BINARY may name an already-built
candidate to avoid rebuilding it; otherwise the current checkout is compiled.
"""
import base64
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]


class GogiosPlanTests(unittest.TestCase):
    def test_all_writers_preserve_private_mode_and_identical_configuration(self):
        binary = os.environ.get("GONF_TEST_BINARY")
        command = [binary] if binary else ["go", "run", "./cmd/gonf"]
        environment = dict(os.environ, GONF_CONF_ROOT=str(ROOT))
        baseline = None
        for task in ("frontends_low_risk", "frontends_gogios",
                     "frontends_d_tail_checks"):
            with self.subTest(task=task), tempfile.TemporaryDirectory() as directory:
                result = subprocess.run(
                    command + ["plan", "-o", directory, task],
                    cwd=ROOT / "gonf", env=environment, capture_output=True,
                    text=True, timeout=60)
                self.assertEqual(result.returncode, 0, result.stderr)
                operations = [json.loads(line) for line in
                              (Path(directory) / "plan.jsonl").read_text().splitlines()]
                if task == "frontends_gogios":
                    startup_lines = [line for op in operations
                                     if op.get("path") == "/etc/rc.local"
                                     for line in op.get("add_lines", [])]
                    for line in (
                            "rm -f /var/db/f3s/monitoring/f3s_taken_down",
                            "if [ ! -d /var/run/gogios ]; then mkdir /var/run/gogios; fi",
                            "chown _gogios /var/run/gogios"):
                        self.assertEqual(startup_lines.count(line), 2)
                configs = [op for op in operations
                           if op.get("path") == "/etc/gogios.json"]
                self.assertEqual(len(configs), 2)
                contents = []
                for config in configs:
                    self.assertEqual(
                        (config["mode"], config["owner"], config["group"]),
                        ("0640", "root", "_gogios"))
                    contents.append(json.loads(base64.b64decode(config["content_b64"])))
                if baseline is None:
                    baseline = contents
                self.assertEqual(contents, baseline)


if __name__ == "__main__":
    unittest.main()
