#!/usr/bin/env python3
"""Exercise the production helper's process locking without network access.

Only its fixed directory, SSH executable and timeout are rewritten into an
owner-private fixture. Real Perl processes and advisory locks run unchanged.
Native SSH/cron checks separately cover the actual client/server behavior.
"""

import fcntl
import os
from pathlib import Path
import subprocess
import tempfile
import time
import unittest


SOURCE = Path(__file__).resolve().parents[3] / "gonf/triggerfish/assets/router-recovery-tunnel"
SSH_FIXTURE = r'''#!/usr/bin/env python3
import os
from pathlib import Path
import sys
import time

base = Path(__file__).parent
args = sys.argv[1:]
if "-O" in args:
    action = args[args.index("-O") + 1]
    if action == "check":
        try:
            fd = os.open(base / "check-entered", os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
        except FileExistsError:
            pass
        else:
            os.close(fd)
            end = time.monotonic() + 8
            while not (base / "release-check").exists():
                if time.monotonic() >= end:
                    sys.exit(3)
                time.sleep(0.01)
        sys.exit(0 if (base / "control").exists() else 1)
    if action == "exit":
        (base / "control").unlink(missing_ok=True)
        sys.exit(0)
if "-fNT" in args:
    (base / "control").write_text("client running\n")
    (base / "client-started").touch()
    sys.exit(0)
sys.exit(4)
'''


class Locking(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="conf-router-recovery-")
        self.addCleanup(self.temp.cleanup)
        self.directory = Path(self.temp.name)
        ssh = self.directory / "ssh"
        ssh.write_text(SSH_FIXTURE)
        ssh.chmod(0o700)
        source = SOURCE.read_text()
        self.assertEqual(source.count("'/etc/gonf/router-recovery'"), 1)
        self.assertEqual(source.count("'/usr/bin/ssh'"), 1)
        self.assertEqual(source.count("alarm 60;"), 1)
        source = source.replace("'/etc/gonf/router-recovery'", repr(str(self.directory)))
        source = source.replace("'/usr/bin/ssh'", repr(str(ssh)))
        source = source.replace("alarm 60;", "alarm 1;")
        self.helper = self.directory / "run"
        self.helper.write_text(source)

    def run_helper(self, *args):
        return subprocess.run(["perl", str(self.helper), *args], capture_output=True, timeout=4)

    def spawn(self, action):
        process = subprocess.Popen(["perl", str(self.helper), action], stdout=subprocess.PIPE, stderr=subprocess.PIPE)

        def cleanup():
            if process.poll() is None:
                process.kill()
            process.communicate(timeout=3)

        self.addCleanup(cleanup)
        return process

    def wait_for(self, name):
        end = time.monotonic() + 4
        while not (self.directory / name).exists():
            if time.monotonic() >= end:
                self.fail(f"missing fixture barrier: {name}")
            time.sleep(0.01)

    def test_retirement_waits_for_inflight_start(self):
        (self.directory / "enabled").touch()
        start = self.spawn("start")
        self.wait_for("check-entered")
        # A concurrent cron tick skips the held lock without starting a client.
        self.assertEqual(self.run_helper("start").returncode, 0)
        (self.directory / "enabled").unlink()
        stop = self.spawn("stop")
        time.sleep(0.15)
        self.assertIsNone(stop.poll(), "Stop returned before the in-flight Start")
        (self.directory / "release-check").touch()
        self.assertEqual(start.wait(timeout=3), 0)
        self.assertEqual(stop.wait(timeout=3), 0)
        self.assertTrue((self.directory / "client-started").exists())
        self.assertFalse((self.directory / "control").exists(), "retirement left an orphan client")
        # A late cron invocation must recheck the removed marker.
        self.assertEqual(self.run_helper("start").returncode, 0)
        self.assertFalse((self.directory / "control").exists())

    def test_stop_timeout_fails_without_touching_client(self):
        (self.directory / "control").write_text("existing client")
        with (self.directory / "lock").open("a") as lock:
            fcntl.flock(lock, fcntl.LOCK_EX)
            result = self.run_helper("stop")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn(b"lock timeout", result.stderr)
        self.assertEqual((self.directory / "control").read_text(), "existing client")

    def test_invalid_arguments_have_no_side_effect(self):
        for args in [(), ("unknown",), ("start", "extra")]:
            self.assertEqual(self.run_helper(*args).returncode, 2)
        self.assertFalse((self.directory / "lock").exists())
        self.assertFalse((self.directory / "client-started").exists())

    def test_lock_open_failure_does_not_start_client(self):
        (self.directory / "enabled").touch()
        (self.directory / "lock").mkdir()
        self.assertNotEqual(self.run_helper("start").returncode, 0)
        self.assertFalse((self.directory / "client-started").exists())


if __name__ == "__main__":
    os.umask(0o077)
    unittest.main()
