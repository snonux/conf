#!/usr/bin/env python3
"""Run the real AP guard against private files and a simulated OpenWrt I/O edge.

File snapshots, hashes, atomic replacement, shell control flow and locks are
real. UCI, service supervision, sockets and boot/uptime are controlled fixtures;
native UCI parsing, procd, SSH and reboot acceptance are separate live checks.
"""

import os
from pathlib import Path
import subprocess
import tempfile
import unittest


DIRECTORY = Path(__file__).resolve().parent
IO = r'''#!/usr/bin/python3
from pathlib import Path
import os
import sys

base = Path(__file__).resolve().parent.parent
name = Path(sys.argv[0]).name
args = sys.argv[1:]
if name == "id":
    print(0)
elif name == "stat":
    print("0:" + oct(Path(args[-1]).stat().st_mode & 0o777)[2:])
elif name == "netstat":
    print((base / "sockets").read_text(), end="")
elif name == "uci":
    directory = base / "configs"
    while args and args[0].startswith("-"):
        flag, value = args[:2]
        if flag == "-c":
            directory = Path(value)
        args = args[2:]
    if args[0] == "changes":
        if (base / "changes").exists():
            print("pending")
    elif args[0] == "get":
        lines = (directory / "network").read_text().splitlines()
        properties = {"ipaddr": lines[0], "proto": "static", "device": "br-lan", "netmask": "255.255.255.0"}
        properties.update(dict(line.split("=", 1) for line in lines[1:] if "=" in line))
        print(properties[args[1].split(".")[-1]])
    elif args[0] == "export":
        data = (directory / args[1]).read_text()
        if "INVALID" in data:
            sys.exit(1)
        print(data)
    else:
        sys.exit(2)
else:
    sys.exit(3)
'''


class Recovery(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="conf-openwrt-guard-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        for name in ["state", "configs", "candidate", "init", "rc", "bin"]:
            (self.root / name).mkdir(mode=0o700)
        self.original = {
            "network": "192.168.1.101/24\noriginal network\n",
            "dhcp": "original dhcp\n",
            "firewall": "original firewall\n",
        }
        self.candidate = {
            "network": "192.168.1.101/24 10.30.0.2/24\ncandidate network\n",
            "dhcp": "candidate dhcp\n",
            "firewall": "original firewall\n",
        }
        for name, contents in self.original.items():
            (self.root / "configs" / name).write_text(contents)
        for name, contents in self.candidate.items():
            (self.root / "candidate" / name).write_text(contents)
        (self.root / "boot-id").write_text("first-boot\n")
        self.clock(100)
        (self.root / "sockets").write_text("")
        io = self.root / "bin/io"
        io.write_text(IO)
        io.chmod(0o700)
        for name in ["uci", "stat", "id", "netstat"]:
            (self.root / "bin" / name).symlink_to(io)
        service = self.root / "init/f3s_ap_recovery"
        service.write_text("#!/bin/sh\ntest \"$1\" = running\n")
        service.chmod(0o700)
        (self.root / "rc/S18f3s_ap_recovery").symlink_to(service)
        for name in ["network", "dhcp", "dnsmasq", "odhcpd", "firewall"]:
            script = self.root / "init" / name
            script.write_text(
                "#!/bin/sh\n"
                f"printf '%s\\n' '{name}' >> '{self.root}/services-called'\n"
                f"test ! -e '{self.root}/fail-{name}'\n"
            )
            script.chmod(0o700)
        source = (DIRECTORY / "ap-network-recovery.sh").read_text()
        replacements = {
            "state=/etc/f3s-ap-recovery": f"state={self.root}/state",
            "configs=/etc/config": f"configs={self.root}/configs",
            "boot_id=/proc/sys/kernel/random/boot_id": f"boot_id={self.root}/boot-id",
            "uptime_file=/proc/uptime": f"uptime_file={self.root}/uptime",
            "/etc/init.d/": f"{self.root}/init/",
            "/etc/rc.d/": f"{self.root}/rc/",
            "PATH=/usr/sbin:/usr/bin:/sbin:/bin": f"PATH={self.root}/bin:/usr/sbin:/usr/bin:/sbin:/bin",
        }
        for before, after in replacements.items():
            self.assertIn(before, source)
            source = source.replace(before, after)
        self.helper = self.root / "guard"
        self.helper.write_text(source)
        self.helper.chmod(0o700)

    def clock(self, seconds):
        (self.root / "uptime").write_text(f"{seconds}.00 0.00\n")

    def run_guard(self, *args, ok=True, connection=""):
        env = {**os.environ, "SSH_CONNECTION": connection}
        result = subprocess.run(["sh", str(self.helper), *map(str, args)], env=env, capture_output=True, timeout=10)
        self.assertEqual(result.returncode == 0, ok, result.stderr.decode())
        return result

    def arm(self, token="trial-token1"):
        self.run_guard("arm", token, "60", self.root / "candidate")

    def activate(self):
        self.arm()
        self.run_guard("activate", "trial-token1")
        self.assert_configs(self.candidate)

    def assert_configs(self, expected):
        for name, contents in expected.items():
            self.assertEqual((self.root / "configs" / name).read_text(), contents)

    def pending(self):
        return (self.root / "state/current/pending").exists()

    def test_expiry_restores_files_and_services(self):
        self.activate()
        self.clock(159)
        self.run_guard("tick")
        self.assertTrue(self.pending())
        self.clock(160)
        self.run_guard("tick")
        self.assertFalse(self.pending())
        self.assert_configs(self.original)
        self.assertEqual((self.root / "state/current/phase").read_text().strip(), "restored-live")
        self.assertEqual(len((self.root / "services-called").read_text().splitlines()), 8)

    def test_boot_restores_before_running_network_services(self):
        self.activate()
        calls = (self.root / "services-called").read_bytes()
        (self.root / "boot-id").write_text("second-boot\n")
        self.clock(1)
        self.run_guard("boot")
        self.assertFalse(self.pending())
        self.assert_configs(self.original)
        self.assertEqual((self.root / "services-called").read_bytes(), calls)

    def test_confirmation_requires_fresh_live_new_address_socket(self):
        self.activate()
        self.run_guard("confirm", "trial-token1", "services-verified", ok=False,
                       connection="10.20.0.130 40000 192.168.1.101 22")
        connection = "10.20.0.130 40001 10.30.0.2 22"
        self.run_guard("confirm", "trial-token1", "services-verified", ok=False, connection=connection)
        (self.root / "sockets").write_text("tcp 0 0 10.30.0.2:22 10.20.0.130:40001 ESTABLISHED\n")
        self.run_guard("confirm", "trial-token1", "services-verified", connection=connection)
        self.assertFalse(self.pending())
        self.clock(10000)
        (self.root / "boot-id").write_text("second-boot\n")
        self.run_guard("boot")
        self.run_guard("tick")
        self.assert_configs(self.candidate)

    def test_old_socket_and_late_confirmation_are_rejected(self):
        (self.root / "sockets").write_text("tcp 0 0 10.30.0.2:22 10.20.0.130:40001 ESTABLISHED\n")
        self.activate()
        self.run_guard("confirm", "trial-token1", "services-verified", ok=False,
                       connection="10.20.0.130 40001 10.30.0.2 22")
        self.clock(160)
        self.run_guard("confirm", "trial-token1", "services-verified", ok=False,
                       connection="10.20.0.130 40002 10.30.0.2 22")
        self.assertTrue(self.pending())

    def test_duplicate_tokens_and_pending_transactions_are_rejected(self):
        self.arm()
        self.run_guard("arm", "trial-token2", "60", self.root / "candidate", ok=False)
        self.run_guard("activate", "wrong-token", ok=False)
        self.run_guard("restore", "trial-token1")
        self.run_guard("arm", "trial-token1", "60", self.root / "candidate", ok=False)
        self.assert_configs(self.original)

    def test_input_change_after_arm_does_not_change_frozen_candidate(self):
        self.arm()
        (self.root / "candidate/network").write_text("later input\n")
        self.run_guard("activate", "trial-token1")
        self.assert_configs(self.candidate)

    def test_reload_failure_keeps_recovery_pending(self):
        self.arm()
        failure = self.root / "fail-network"
        failure.touch()
        self.run_guard("activate", "trial-token1", ok=False)
        self.assertTrue(self.pending())
        self.clock(161)
        self.run_guard("tick", ok=False)
        self.assertTrue(self.pending())
        failure.unlink()
        self.run_guard("tick")
        self.assertFalse(self.pending())
        self.assert_configs(self.original)

    def test_corrupt_beforeimage_is_not_partially_restored(self):
        self.activate()
        (self.root / "state/current/before/network").write_text("corruption\n")
        self.clock(161)
        self.run_guard("tick", ok=False)
        self.assertTrue(self.pending())
        self.assert_configs(self.candidate)

    def test_unknown_clock_or_deadline_restores_instead_of_extending(self):
        self.activate()
        (self.root / "uptime").write_text("unreadable clock\n")
        self.run_guard("tick")
        self.assertFalse(self.pending())
        self.assert_configs(self.original)

    def test_old_address_is_not_enough_if_lan_protocol_changes(self):
        (self.root / "candidate/network").write_text(self.candidate["network"] + "proto=none\n")
        self.run_guard("arm", "trial-token1", "60", self.root / "candidate", ok=False)
        self.assertFalse(self.pending())
        self.assert_configs(self.original)

    def test_old_address_removal_and_unsafe_inputs_are_rejected(self):
        network = self.root / "candidate/network"
        network.write_text("10.30.0.2/24\n")
        self.run_guard("arm", "trial-token1", "60", self.root / "candidate", ok=False)
        self.assertFalse(self.pending())
        self.assert_configs(self.original)
        network.write_text(self.candidate["network"])
        (self.root / "candidate").chmod(0o777)
        self.run_guard("arm", "trial-token2", "60", self.root / "candidate", ok=False)
        self.assertFalse(self.pending())


if __name__ == "__main__":
    os.umask(0o077)
    unittest.main()
