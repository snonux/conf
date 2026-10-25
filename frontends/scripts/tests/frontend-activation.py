#!/usr/bin/env python3
"""Fault-inject the real activation helper in an isolated temporary filesystem."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

SOURCE = Path(__file__).resolve().parents[1] / "frontend-activate.sh"
STUB = r'''#!/usr/bin/env python3
import hashlib, os, pathlib, sys
root = pathlib.Path(os.environ["ACTIVATION_TEST_ROOT"])
name = pathlib.Path(sys.argv[0]).name
args = sys.argv[1:]
call = ":".join([name] + args)
with (root / "events").open("a") as f:
    f.write(call + "\n")
if (root / "fail").exists() and call.startswith((root / "fail").read_text()):
    sys.exit(1)
if name == "sha256":
    try:
        data = [pathlib.Path(p).read_bytes() for p in args[1:]] if len(args)>1 else [sys.stdin.buffer.read()]
        for item in data:
            print(hashlib.sha256(item).hexdigest())
    except OSError:
        sys.exit(1)
elif name == "pfctl":
    if args == ["-si"]:
        print("Status: " + ("Enabled" if (root / "enabled").exists() else "Disabled"))
    elif args == ["-e"]:
        (root / "enabled").touch()
elif name == "rcctl":
    if args[0] == "get":
        print((root / "flags").read_text())
    elif args[0] == "check":
        sys.exit(0 if (root / "running").exists() else 1)
    elif args[0] == "restart":
        (root / "running").touch()
'''


class Activation(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(prefix="frontend-activation-")
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        for directory in ["etc/login.conf.d", "var/db", "bin"]:
            (self.root / directory).mkdir(parents=True)
        for config in ["pf.conf", "httpd.conf", "inetd.conf", "login.conf.d/inetd"]:
            (self.root / "etc" / config).write_text("configuration\n")
        (self.root / "flags").write_text("")
        (self.root / "enabled").touch()
        (self.root / "running").touch()
        stub = self.root / "bin/stub"
        stub.write_text(STUB)
        stub.chmod(0o755)
        for name in ["sha256", "pfctl", "rcctl", "httpd"]:
            (self.root / "bin" / name).symlink_to("stub")
        self.script = self.root / "activate.sh"
        self.script.write_text(SOURCE.read_text().replace("/etc/", str(self.root / "etc") + "/").replace("/var/db/", str(self.root / "var/db") + "/"))
        self.env = dict(os.environ, ACTIVATION_TEST_ROOT=str(self.root), PATH=str(self.root / "bin") + os.pathsep + os.environ["PATH"])

    def run_helper(self, operation, service, success=True):
        result = subprocess.run(["sh", str(self.script), operation, service], env=self.env, capture_output=True, text=True)
        self.assertEqual(result.returncode == 0, success, result.stderr)

    def inject_failure(self, prefix):
        (self.root / "fail").write_text(prefix)

    def recover(self):
        (self.root / "fail").unlink()

    def test_failed_activations_retry_after_file_converges(self):
        for service, command in [("pf", "pfctl:-f:"), ("httpd", "rcctl:restart:httpd"), ("inetd", "rcctl:restart:inetd")]:
            with self.subTest(service=service):
                self.run_helper("apply", service)
                self.run_helper("check", service)
                (self.root / "etc" / (service + ".conf")).write_text("new configuration\n")
                self.run_helper("check", service, False)
                self.inject_failure(command)
                self.run_helper("apply", service, False)
                self.run_helper("check", service, False)
                self.recover()
                self.run_helper("apply", service)
                self.run_helper("check", service)

    def test_validation_failure_never_loads_or_records_success(self):
        for service, command in [("pf", "pfctl:-nf:"), ("httpd", "httpd:-n:")]:
            with self.subTest(service=service):
                self.inject_failure(command)
                self.run_helper("apply", service, False)
                self.assertFalse((self.root / "var/db" / ("gonf-frontend-" + service + ".sha256")).exists())
                self.assertNotIn("pfctl:-f:", (self.root / "events").read_text())
                self.assertNotIn("rcctl:restart:", (self.root / "events").read_text())
                self.recover()

    def test_pf_disabled_reloads_before_enable_without_flushing(self):
        self.run_helper("apply", "pf")
        (self.root / "enabled").unlink()
        self.run_helper("check", "pf", False)
        (self.root / "events").write_text("")
        self.run_helper("apply", "pf")
        events = (self.root / "events").read_text()
        self.assertLess(events.index("pfctl:-nf:"), events.index("pfctl:-f:"))
        self.assertLess(events.index("pfctl:-f:"), events.index("pfctl:-e"))
        self.assertNotIn("pfctl:-F", events)
        self.run_helper("check", "pf")

    def test_flags_class_and_helper_changes_require_activation(self):
        self.run_helper("apply", "inetd")
        for path in [self.root / "flags", self.root / "etc/login.conf.d/inetd", self.script]:
            with path.open("a") as f:
                f.write("\n# changed\n")
            self.run_helper("check", "inetd", False)
            self.run_helper("apply", "inetd")
            self.run_helper("check", "inetd")

    def test_missing_input_and_failed_health_check_never_record_success(self):
        config = self.root / "etc/httpd.conf"
        config.unlink()
        self.run_helper("apply", "httpd", False)
        config.write_text("configuration\n")
        self.inject_failure("rcctl:check:httpd")
        self.run_helper("apply", "httpd", False)
        self.assertFalse((self.root / "var/db/gonf-frontend-httpd.sha256").exists())
        self.recover()
        self.run_helper("apply", "httpd")
        self.run_helper("check", "httpd")
        (self.root / "running").unlink()
        self.run_helper("check", "httpd", False)


if __name__ == "__main__":
    unittest.main()
