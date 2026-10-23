"""Run the real rollback logic against disposable files and service shims."""

import os
from pathlib import Path
import re
import shlex
import stat
import subprocess
import tempfile
import unittest


SCRIPT = Path(__file__).resolve().parents[1] / "rollback-peer-certificates.sh"


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
            "previous.gogios.json": self.etc / "gogios.json",
            "gogios.buetow.org.fullchain.pem": self.ssl / "gogios.buetow.org.fullchain.pem",
            "gogios.buetow.org.key": self.ssl / "private/gogios.buetow.org.key",
        }
        for name, path in self.files.items():
            (self.work / name).write_text("previous fixture: " + name)
            path.write_text("current fixture: " + name)
        self.modes = {"previous.gogios.json": 0o744,
                      "gogios.buetow.org.fullchain.pem": 0o444, "gogios.buetow.org.key": 0o400}
        for name, mode in self.modes.items():
            (self.work / name).chmod(mode)
        (self.work / "relayd-was-running").touch()
        (self.ssl / "gogios.buetow.org.crt").symlink_to(
            self.ssl / "gogios.buetow.org.fullchain.pem")
        (self.etc / "relayd.conf").write_text("unchanged live configuration fixture")
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
        text = text.replace("work=/root/ro3-peer-certs-20261023",
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
        self.assertEqual((self.etc / "relayd.conf").read_text(),
                         "unchanged live configuration fixture")
        self.assertFalse(list(self.etc.rglob(".ro3-*")))
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
        self.assertTrue((self.ssl / "gogios.buetow.org.crt").is_symlink())

    def test_blowfish_restores_only_three_standby_pairs(self):
        names = ("standby.ecat.buetow.org", "standby.alt.irregular.ninja",
                 "standby.gogios.buetow.org")
        for name in names:
            for suffix, destination in ((".key", self.ssl / "private"),
                                        (".fullchain.pem", self.ssl)):
                (self.work / (name + suffix)).write_text("old standby fixture")
                (destination / (name + suffix)).write_text("new standby fixture")
            (self.ssl / (name + ".crt")).symlink_to(self.ssl / (name + ".fullchain.pem"))
        result = self.run_rollback(TEST_HOST="blowfish")
        self.assertEqual(result.returncode, 0, result.stderr)
        for name in names:
            self.assertEqual((self.ssl / (name + ".fullchain.pem")).read_text(),
                             "old standby fixture")
            self.assertEqual((self.ssl / "private" / (name + ".key")).read_text(),
                             "old standby fixture")
        self.assertEqual(self.files["gogios.buetow.org.key"].read_text(),
                         "current fixture: gogios.buetow.org.key")


    def test_staging_failure_changes_no_live_file(self):
        result = self.run_rollback(TEST_COPY_FAIL="gogios.buetow.org.key")
        self.assertNotEqual(result.returncode, 0)
        for name, path in self.files.items():
            self.assertEqual(path.read_text(), "current fixture: " + name)
        self.assertEqual(self.actions.read_text(), "")

    def test_missing_backup_changes_no_live_file(self):
        (self.work / "gogios.buetow.org.key").unlink()
        result = self.run_rollback()
        self.assertNotEqual(result.returncode, 0)
        for name, path in self.files.items():
            self.assertEqual(path.read_text(), "current fixture: " + name)
        self.assertEqual(self.actions.read_text(), "")

    def test_symlinked_backup_is_rejected(self):
        path = self.work / "gogios.buetow.org.key"
        path.unlink()
        path.symlink_to(self.files["gogios.buetow.org.key"])
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

    def test_existing_rollback_lock_refuses_parallel_restore(self):
        (self.work / "rollback.lock").mkdir()
        result = self.run_rollback()
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(self.actions.read_text(), "")
        for name, path in self.files.items():
            self.assertEqual(path.read_text(), "current fixture: " + name)

    def test_changed_alias_is_rejected_before_publication(self):
        alias = self.ssl / "gogios.buetow.org.crt"
        alias.unlink()
        alias.symlink_to("wrong-chain.pem")
        result = self.run_rollback()
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(self.actions.read_text(), "")
        for name, path in self.files.items():
            self.assertEqual(path.read_text(), "current fixture: " + name)

    def test_linked_live_key_is_rejected_before_publication(self):
        key = self.ssl / "private/gogios.buetow.org.key"
        key.unlink()
        key.symlink_to(self.work / "gogios.buetow.org.key")
        result = self.run_rollback()
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(self.actions.read_text(), "")

    def test_wrong_host_and_unprivileged_invocation_are_rejected(self):
        for env in ({"TEST_HOST": "elsewhere"}, {"TEST_UID": "1001"}):
            with self.subTest(env=env):
                result = self.run_rollback(**env)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(self.actions.read_text(), "")


class GuardTests(unittest.TestCase):
    def test_documented_guard_decisions_and_shell_syntax(self):
        document = (SCRIPT.parents[1] / "docs/peer-certificate-repair.md").read_text()
        blocks = re.findall(r'```(sh|bash)\n(.*?)\n```', document, re.S)
        for language, code in blocks:
            subprocess.run(["bash" if language == "bash" else "sh", "-n"],
                           input=code.encode(), check=True, timeout=5)
        guard = next(code for _, code in blocks if "nohup sh -c" in code)
        command = re.search("nohup sh -c '([^']+)'", guard)[1]
        with tempfile.TemporaryDirectory(prefix="ro3-guard-fixture-") as temporary:
            root = Path(temporary)
            (root / "rollback").write_text("#!/bin/sh\ntouch " +
                                            shlex.quote(str(root / "rolled-back")) + "\n")
            (root / "rollback").chmod(0o700)
            command = command.replace("/root/ro3-peer-certs-20261023", str(root))
            command = command.replace("sleep 900", "sleep 0")
            for started, accepted, expected in ((False, False, False), (True, False, True),
                                                (True, True, False), (False, True, False)):
                with self.subTest(started=started, accepted=accepted):
                    for name, wanted in (("publication-started", started), ("accepted", accepted)):
                        path = root / name
                        if wanted:
                            path.touch()
                        elif path.exists():
                            path.unlink()
                    marker = root / "rolled-back"
                    if marker.exists():
                        marker.unlink()
                    subprocess.run(["sh", "-c", command], check=True, timeout=5)
                    self.assertEqual(marker.exists(), expected)


if __name__ == "__main__":
    unittest.main()
