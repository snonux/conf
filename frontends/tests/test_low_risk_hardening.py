"""Exercise the real mute helper and Unix access boundaries in a private sandbox.

Run with sudo so children can drop to isolated numeric identities. Nothing
outside the temporary directory is changed; no local accounts are created.
"""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
WRITER, READER, LOG_READER, OUTSIDER = 65001, 65002, 65003, 65534


class PermissionTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        if os.geteuid() != 0:
            raise unittest.SkipTest("run with sudo for real credential-drop tests")

    def setUp(self):
        self.directory = tempfile.TemporaryDirectory(prefix="dn3-permissions-")
        self.addCleanup(self.directory.cleanup)
        self.base = Path(self.directory.name)
        self.base.chmod(0o755)
        self.parent = self.base / "protected"
        self.parent.mkdir(mode=0o755)
        self.control = self.parent / "monitoring"
        self.control.mkdir(mode=0o750)
        os.chown(self.control, WRITER, READER)
        self.marker = self.control / "f3s_taken_down"
        self.helper = self.base / "mute"
        source = (ROOT / "frontends/scripts/f3s-gogios-mute").read_text()
        self.helper.write_text(source.replace(
            "/var/db/f3s/monitoring/f3s_taken_down", str(self.marker)))
        self.helper.chmod(0o555)
        # This file makes a wildcard verb expand to a valid verb unless the
        # actual helper disables globbing before parsing untrusted input.
        (self.base / "gogios-mute").touch()

    @staticmethod
    def drop(identity):
        def credentials():
            os.setgroups([])
            os.setgid(identity)
            os.setuid(identity)
        return credentials

    def run_as(self, identity, args, **kwargs):
        return subprocess.run(args, capture_output=True, text=True,
                              preexec_fn=self.drop(identity), timeout=5,
                              cwd=self.base, **kwargs)

    def helper_run(self, verb):
        return self.run_as(WRITER, ["sh", str(self.helper)],
                           env={"PATH": "/bin:/usr/bin", "SSH_ORIGINAL_COMMAND": verb})

    def test_allowed_verbs_and_reboot_clear(self):
        self.assertEqual(self.helper_run("gogios-status").stdout.strip(), "unmuted")
        self.assertEqual(self.helper_run("gogios-mute").returncode, 0)
        self.assertEqual(self.marker.stat().st_uid, WRITER)
        self.assertEqual(self.marker.stat().st_mode & 0o777, 0o640)
        self.assertEqual(self.helper_run("gogios-status").stdout.strip(), "muted")
        self.assertEqual(self.helper_run("gogios-unmute").returncode, 0)
        self.assertFalse(self.marker.exists())
        self.assertEqual(self.helper_run("gogios-mute").returncode, 0)
        subprocess.run(["rm", "-f", str(self.marker)], check=True)
        self.assertEqual(self.helper_run("gogios-status").stdout.strip(), "unmuted")

    def test_extra_words_metacharacters_and_globs_are_rejected(self):
        for verb in ("", "gogios-mute extra", "gogios-mute; true", "gogios-*", "*",
                     "$(gogios-mute)", "gogios-status\ngogios-mute"):
            with self.subTest(verb=verb):
                self.assertNotEqual(self.helper_run(verb).returncode, 0)
                self.assertFalse(self.marker.exists())

    def test_unprivileged_user_cannot_silence_monitoring(self):
        for identity in (READER, OUTSIDER):
            with self.subTest(identity=identity):
                result = self.run_as(identity, ["touch", str(self.marker)])
                self.assertNotEqual(result.returncode, 0)
                self.assertFalse(self.marker.exists())
        self.assertEqual(self.helper_run("gogios-mute").returncode, 0)
        result = self.run_as(READER, ["test", "-e", str(self.marker)])
        self.assertEqual(result.returncode, 0)
        result = self.run_as(OUTSIDER, ["rm", "-f", str(self.marker)])
        self.assertNotEqual(result.returncode, 0)
        self.assertTrue(self.marker.exists())

    def test_reader_can_read_config_but_cannot_modify_it(self):
        config = self.base / "gogios.json"
        config.write_text('{"Checks": {}}\n')
        os.chown(config, 0, READER)
        config.chmod(0o640)
        self.assertEqual(self.run_as(READER, ["cat", str(config)]).returncode, 0)
        self.assertNotEqual(self.run_as(READER, ["test", "-w", str(config)]).returncode, 0)
        self.assertNotEqual(self.run_as(OUTSIDER, ["cat", str(config)]).returncode, 0)

    def test_dserver_keeps_read_only_access_to_logs(self):
        log = self.base / "access.log"
        log.write_text("local fixture\n")
        os.chown(log, 0, LOG_READER)
        log.chmod(0o640)
        self.assertEqual(self.run_as(LOG_READER, ["cat", str(log)]).returncode, 0)
        self.assertNotEqual(self.run_as(LOG_READER, ["test", "-w", str(log)]).returncode, 0)
        self.assertNotEqual(self.run_as(OUTSIDER, ["cat", str(log)]).returncode, 0)


if __name__ == "__main__":
    unittest.main()
