"""Run the real rollback logic against disposable files and service shims."""

import os
from pathlib import Path
import shlex
import stat
import subprocess
import tempfile
import unittest


SCRIPT = Path(__file__).resolve().parents[1] / "rollback-code-certificate.sh"


class RollbackTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.root = Path(self.directory.name)
        self.etc = self.root / "etc"
        self.ssl = self.etc / "ssl"
        self.work = self.root / "backup"
        self.bin = self.root / "bin"
        for path in (self.ssl / "private", self.work, self.bin):
            path.mkdir(parents=True)
        self.files = {
            "previous.relayd.conf": self.etc / "relayd.conf",
            "previous.gogios.json": self.etc / "gogios.json",
            "previous.fullchain.pem": self.ssl / "code.f3s.buetow.org.fullchain.pem",
            "previous.key": self.ssl / "private/code.f3s.buetow.org.key",
        }
        for name, path in self.files.items():
            (self.work / name).write_text("previous fixture: " + name)
            path.write_text("current fixture: " + name)
        self.modes = {"previous.relayd.conf": 0o600,
                      "previous.gogios.json": 0o744,
                      "previous.fullchain.pem": 0o444, "previous.key": 0o400}
        for name, mode in self.modes.items():
            (self.work / name).chmod(mode)
        (self.work / "relayd-was-running").touch()
        (self.ssl / "code.f3s.buetow.org.crt").symlink_to(
            self.ssl / "code.f3s.buetow.org.fullchain.pem")
        self.actions = self.root / "actions"
        self.actions.write_text("")
        self.state = self.root / "service-state"
        self.state.write_text("running")
        self.env = {**os.environ, "TEST_ACTIONS": str(self.actions),
                    "TEST_STATE": str(self.state)}

        # Only fixed paths and platform/privilege adapters change in this copy.
        # The production script has no environment-based root/path bypass.
        text = SCRIPT.read_text()
        text = text.replace("PATH=/bin:/usr/bin:/sbin:/usr/sbin:/usr/local/bin:/usr/local/sbin",
                            "PATH=" + shlex.quote(str(self.bin)) + ":$PATH")
        text = text.replace("work=/root/ho3-code-cert-20261023",
                            "work=" + shlex.quote(str(self.work)))
        text = text.replace("ssl_dir=/etc/ssl", "ssl_dir=" + shlex.quote(str(self.ssl)))
        text = text.replace("config_dir=/etc", "config_dir=" + shlex.quote(str(self.etc)))
        self.script = self.root / "rollback.sh"
        self.script.write_text(text)
        self.shim("id", '#!/bin/sh\nprintf "%s\\n" "${TEST_UID:-0}"\n')
        self.shim("hostname", '#!/bin/sh\nprintf "%s\\n" "${TEST_HOST:-fishfinger}"\n')
        self.shim("relayd", '#!/bin/sh\nprintf "validate\\n" >>"$TEST_ACTIONS"\nexit "${TEST_PARSE_FAIL:-0}"\n')
        self.shim("rcctl", '''#!/bin/sh
printf '%s\n' "$*" >>"$TEST_ACTIONS"
case "$1" in
check)
    [ "${TEST_CHECK_ERROR:-0}" = 0 ] || exit "$TEST_CHECK_ERROR"
    [ "$(cat "$TEST_STATE")" = running ] ;;
reload)
    if [ "${TEST_RELOAD_FAIL:-0}" != 0 ]; then
        [ "${TEST_RELOAD_STOPS:-0}" = 0 ] || printf stopped >"$TEST_STATE"
        exit 1
    fi ;;
start)
    [ "${TEST_START_FAIL:-0}" = 0 ] || exit 1
    printf running >"$TEST_STATE" ;;
*) exit 1 ;;
esac
''')
        self.shim("cp", '''#!/usr/bin/env python3
import os, sys
from pathlib import Path
args = sys.argv[1:]
if os.environ.get('TEST_COPY_FAIL') == Path(args[-2]).name:
    raise SystemExit(1)
os.execv('/bin/cp', ['cp', *args])
''')

    def shim(self, name, content):
        path = self.bin / name
        path.write_text(content)
        path.chmod(0o755)

    def tearDown(self):
        self.directory.cleanup()

    def run_rollback(self, **overrides):
        result = subprocess.run(["sh", str(self.script)],
                                env={**self.env, **overrides}, capture_output=True,
                                text=True, timeout=10)
        self.assertNotIn("fixture:", result.stdout + result.stderr)
        self.assertFalse(list(self.etc.rglob(".ho3-*")))
        for name in self.files:
            if (self.work / name).is_file():
                self.assertEqual((self.work / name).read_text(), "previous fixture: " + name)
        return result

    def assert_restored(self, certificate=True):
        for name, path in self.files.items():
            expected = "previous" if certificate or name.endswith((".conf", ".json")) else "current"
            self.assertEqual(path.read_text(), expected + " fixture: " + name)

    def test_fishfinger_restores_pair_and_configuration_before_reload(self):
        result = self.run_rollback()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assert_restored()
        self.assertEqual(self.actions.read_text().splitlines(),
                         ["validate", "check relayd", "reload relayd", "check relayd"])
        for name, mode in self.modes.items():
            self.assertEqual(stat.S_IMODE(self.files[name].stat().st_mode), mode)
        self.assertTrue((self.ssl / "code.f3s.buetow.org.crt").is_symlink())

    def test_blowfish_restores_configuration_without_copying_certificate(self):
        for name in ("previous.key", "previous.fullchain.pem"):
            (self.work / name).unlink()
        result = self.run_rollback(TEST_HOST="blowfish")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assert_restored(certificate=False)

    def test_staging_failure_changes_no_live_file(self):
        result = self.run_rollback(TEST_COPY_FAIL="previous.key")
        self.assertNotEqual(result.returncode, 0)
        for name, path in self.files.items():
            self.assertEqual(path.read_text(), "current fixture: " + name)
        self.assertEqual(self.actions.read_text(), "")

    def test_missing_backup_changes_no_live_file(self):
        (self.work / "previous.key").unlink()
        result = self.run_rollback()
        self.assertNotEqual(result.returncode, 0)
        for name, path in self.files.items():
            self.assertEqual(path.read_text(), "current fixture: " + name)
        self.assertEqual(self.actions.read_text(), "")

    def test_symlinked_backup_is_rejected(self):
        path = self.work / "previous.key"
        path.unlink()
        path.symlink_to(self.files["previous.key"])
        # A link is deliberately excluded from the backup-content assertion.
        result = subprocess.run(["sh", str(self.script)], env=self.env,
                                capture_output=True, text=True, timeout=10)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(self.actions.read_text(), "")

    def test_invalid_restored_configuration_is_not_reloaded(self):
        result = self.run_rollback(TEST_PARSE_FAIL="1")
        self.assertNotEqual(result.returncode, 0)
        self.assert_restored()
        self.assertEqual(self.actions.read_text().splitlines(), ["validate"])

    def test_stopped_daemon_without_baseline_is_not_started(self):
        self.state.write_text("stopped")
        (self.work / "relayd-was-running").unlink()
        result = self.run_rollback()
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn("start relayd", self.actions.read_text())
        self.assertNotIn("reload relayd", self.actions.read_text())

    def test_failed_restart_is_recovered_from_running_baseline(self):
        self.state.write_text("stopped")
        result = self.run_rollback()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.state.read_text(), "running")
        self.assert_restored()
        self.assertIn("start relayd", self.actions.read_text())

    def test_failed_recovery_start_is_reported(self):
        self.state.write_text("stopped")
        result = self.run_rollback(TEST_START_FAIL="1")
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn("configuration restored", result.stdout)

    def test_unknown_status_is_not_treated_as_a_stopped_daemon(self):
        result = self.run_rollback(TEST_CHECK_ERROR="124")
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn("start relayd", self.actions.read_text())
        self.assertNotIn("reload relayd", self.actions.read_text())

    def test_reload_that_loses_daemon_is_recovered(self):
        result = self.run_rollback(TEST_RELOAD_FAIL="1", TEST_RELOAD_STOPS="1")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.state.read_text(), "running")
        self.assertIn("start relayd", self.actions.read_text())

    def test_reload_failure_is_reported(self):
        result = self.run_rollback(TEST_RELOAD_FAIL="1")
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn("configuration restored", result.stdout)

    def test_wrong_host_and_unprivileged_invocation_are_rejected(self):
        for env in ({"TEST_HOST": "elsewhere"}, {"TEST_UID": "1001"}):
            with self.subTest(env=env):
                result = self.run_rollback(**env)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(self.actions.read_text(), "")


if __name__ == "__main__":
    unittest.main()
