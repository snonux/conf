"""Record actual Pi uploader recipes with dummy credentials, never apply them.

Cron adoption behavior is tested in gonf's generic resource/cron suite. These
checks cover recipe wiring, deletion order and optional-secret policy only.
"""
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[3]
HARNESS = '''package main
import (
    "context"
    "fmt"
    "os"
    "strings"
    "github.com/snonux/conf/gonf/cluster"
    "github.com/snonux/conf/gonf/tasks"
    "github.com/snonux/gonf/api"
    "github.com/snonux/gonf/cli"
    "github.com/snonux/gonf/secret"
)
func main() {
    api.SetSecretProvider(secret.ProviderFunc(func(_ context.Context, ref secret.Ref) ([]byte, error) {
        if !strings.Contains(string(ref), "goprecords") {
            return nil, fmt.Errorf("unexpected fixture secret reference")
        }
        switch os.Getenv("CONF_TEST_SECRET_STATE") {
        case "missing": return nil, &secret.Error{Kind: secret.ErrNotFound, Ref: ref, Msg: "fixture absent"}
        case "failure": return nil, fmt.Errorf("fixture provider unavailable")
        case "empty": return []byte("\\n"), nil
        default: return []byte("fixture-only-never-production"), nil
        }
    }))
    cluster.Register()
    tasks.Register()
    cli.Main()
}
'''


class PlanTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.tmp = tempfile.TemporaryDirectory(prefix="conf-pi-upload-")
        cls.addClassCleanup(cls.tmp.cleanup)
        cls.directory = Path(cls.tmp.name)
        harness = cls.directory / "fixture.go"
        harness.write_text(HARNESS)
        cls.binary = cls.directory / "fixture"
        subprocess.run(["go", "build", "-p", "1", "-o", str(cls.binary), str(harness)],
                       cwd=ROOT / "gonf", check=True, capture_output=True)

    def record(self, mode="present"):
        dest = self.directory / mode
        result = subprocess.run([
            str(self.binary), "plan", "-o", str(dest), "netbsd_goprecords_upload"],
            cwd=ROOT / "gonf", capture_output=True, text=True, timeout=30,
            env={**os.environ, "GONF_CONF_ROOT": str(ROOT), "CONF_TEST_SECRET_STATE": mode})
        return result, dest

    def load(self, mode="present"):
        result, dest = self.record(mode)
        self.assertEqual(result.returncode, 0, result.stderr)
        return [json.loads(line) for line in (dest / "plan.jsonl").read_text().splitlines()]

    def test_cron_adopts_exact_old_command_and_keeps_managed_destination(self):
        crons = [op for op in self.load() if op["op"] == "cron"]
        self.assertTrue(crons)
        for op in crons:
            self.assertEqual(op["name"], "goprecords-upload")
            self.assertEqual(op["cron_user"], "root")
            self.assertEqual(op["schedule"], "17 * * * *")
            host = re.search(r"GOPRECORDS_HOST=([a-zA-Z0-9_-]+)", op["command"])[1]
            self.assertEqual(op["legacy_command"], "env GOPRECORDS_HOST=" + host +
                             " /usr/pkg/bin/goprecords-upload-client.sh >/dev/null 2>&1")
            self.assertIn("/usr/local/bin/goprecords-upload-client.sh", op["command"])
            self.assertIn("logger -t goprecords-upload", op["command"])

    def test_legacy_file_removed_after_cron_without_touching_managed_client(self):
        ops = self.load()
        removed = [op for op in ops if op.get("name") == "retired-goprecords-upload-client"]
        self.assertTrue(removed)
        for op in removed:
            self.assertEqual(op["op"], "command")
            self.assertEqual(op["bin"], "rm")
            self.assertEqual(op["args"], ["-f", "--", "/usr/pkg/bin/goprecords-upload-client.sh"])
            self.assertIn("Cron[root/goprecords-upload]", op["deps"])
        self.assertLess(next(i for i, op in enumerate(ops) if op["op"] == "cron"),
                        next(i for i, op in enumerate(ops)
                             if op.get("name") == "retired-goprecords-upload-client"))
        self.assertTrue(any(op.get("path") == "/usr/local/bin/goprecords-upload-client.sh"
                            and not op.get("absent") for op in ops))

    def test_missing_optional_secret_keeps_existing_schedule_and_old_file(self):
        for mode in ("missing", "empty"):
            with self.subTest(mode=mode):
                ops = self.load(mode)
                self.assertFalse(any(op["op"] == "cron" for op in ops))
                self.assertFalse(any(op.get("absent") for op in ops))
                self.assertFalse(any(op.get("name") == "retired-goprecords-upload-client" for op in ops))

    def test_cleanup_guard_preserves_a_still_referenced_or_missing_script(self):
        op = next(op for op in self.load() if op.get("name") == "retired-goprecords-upload-client")
        guard = op["only_if"]
        with tempfile.TemporaryDirectory(prefix="conf-pi-guard-") as directory:
            root = Path(directory)
            legacy = root / "legacy-client"
            legacy.write_text("fixture")
            crontab = root / "crontab"
            crontab.write_text('#!/bin/sh\n'
                               '[ "${CONF_TEST_CRON_FAILURE:-no}" = no ] || exit 1\n'
                               'printf "%s" "$CONF_TEST_CRON_CONTENT"\n')
            crontab.chmod(0o755)
            args = [arg.replace("/usr/pkg/bin/goprecords-upload-client.sh", str(legacy))
                    for arg in guard["args"]]
            env = {**os.environ, "PATH": str(root) + ":" + os.environ["PATH"]}
            cases = [("unrelated job\n", "no", 0),
                     ("17 * * * * " + str(legacy) + "\n", "no", 1),
                     ("# old path: " + str(legacy) + "\n", "no", 1),
                     ("", "yes", 1)]
            for tab, failure, expected in cases:
                result = subprocess.run([guard["bin"], *args], capture_output=True,
                                        env={**env, "CONF_TEST_CRON_CONTENT": tab,
                                             "CONF_TEST_CRON_FAILURE": failure})
                self.assertEqual(result.returncode, expected)
                self.assertTrue(legacy.exists(), "guard itself must not delete")
            legacy.unlink()
            result = subprocess.run([guard["bin"], *args], capture_output=True,
                                    env={**env, "CONF_TEST_CRON_CONTENT": ""})
            self.assertEqual(result.returncode, 1, "already absent file must skip repeat removal")

    def test_unavailable_secret_provider_fails_recording(self):
        result, _ = self.record("failure")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("fixture provider unavailable", result.stderr)


if __name__ == "__main__":
    unittest.main()
