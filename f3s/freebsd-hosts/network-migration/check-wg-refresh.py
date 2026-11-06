#!/usr/bin/env python3
"""Run real helper dispatch with complete/partial runtime WireGuard fixtures."""
import importlib.util
import json
import os
from pathlib import Path
import shlex
import subprocess
import tempfile

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("migration_fixture", HERE / "check-offline.py")
fixture = importlib.util.module_from_spec(spec)
spec.loader.exec_module(fixture)


def main():
    with tempfile.TemporaryDirectory(prefix="freebsd-wg-refresh-") as raw:
        tmp = Path(raw)
        state, bindir = tmp / "state", tmp / "bin"
        state.mkdir(mode=0o700)
        bindir.mkdir()
        db, rcconf, conf, helper = (tmp / n for n in ("db.json", "rc.conf", "migration.conf", "migrate"))
        conf.write_text("iface=re0\nold=192.168.1.130\nlab=10.20.0.130\nrecovery=10.98.0.130\n"
                        "ula=fd02:ca0b:f1cb:20::130\nshared_gateway=192.168.1.2\n")
        for cmd in ("id", "lockf", "date", "pgrep", "sync", "netstat", "sysrc", "ifconfig",
                    "route", "sha256", "stat", "sysctl", "cp", "wg"):
            path = bindir / cmd
            path.write_text(fixture.MOCK)
            path.chmod(0o755)
        source = (HERE / "migrate.sh").read_text()
        for old, new in (("PATH=/sbin:/bin:/usr/sbin:/usr/bin:/usr/local/sbin:/usr/local/bin", f"PATH={bindir}:/usr/bin:/bin"),
                         ("/var/db/gonf-network-migration", str(state)),
                         ("/usr/local/sbin/gonf-network-migration", str(helper)),
                         ("/usr/local/etc/gonf-network-migration.conf", str(conf)), ("/etc/rc.conf", str(rcconf))):
            source = source.replace(old, new)
        helper.write_text(source)
        helper.chmod(0o755)
        runtime = "[Interface]\nPrivateKey = fixture-only\nListenPort = 56709\n\n[Peer]\nPublicKey = fixture-peer\nAllowedIPs = 192.0.2.1/32, 2001:db8::1/128\nEndpoint = 198.51.100.1:56709\n"
        initial = dict(keys={"ifconfig_re0": "inet 192.168.1.130/24", "defaultrouter": "192.168.1.1",
                             "ifconfig_re0_alias0": "CARP owned separately", "ifconfig_re0_ipv6": "inet6 accept_rtadv"},
                       v4=["192.168.1.130", "192.168.1.138"], v6=[], ra=True, default4="192.168.1.1",
                       default6=None, now=1000, calls=[], carp={"vip": "192.168.1.138", "vhid": 1, "state": "BACKUP"},
                       wg_present=True, wg_runtime=runtime, wg_addresses=["192.0.2.130", "2001:db8::130"])
        initial["return"] = None
        db.write_text(json.dumps(initial))
        rcconf.write_text("".join(f"{k}={shlex.quote(v)}\n" for k, v in initial["keys"].items()))
        env = dict(os.environ, FIXTURE_DB=str(db), FIXTURE_RC=str(rcconf))

        def run(*args, ok=True):
            result = subprocess.run([str(helper), *args], env=env, capture_output=True, text=True)
            assert (result.returncode == 0) == ok, (args, result.stderr)
            return json.loads(db.read_text())

        def update(**fields):
            value = json.loads(db.read_text())
            value.update(fields)
            db.write_text(json.dumps(value))

        (state / "activation-approved").touch()
        run("stage"); stage = run("confirm")
        update(wg_show_fail_once=True)
        failed = run("promote", "shared", "300", ok=False)
        assert not (state / "pending").exists()
        assert failed["v4"] == stage["v4"] and failed["keys"] == stage["keys"]
        update(wg_set_fail_once=True)
        failed = run("promote", "shared", "300", ok=False)
        snapshot = state / "pending" / "wg0.runtime.conf"
        assert snapshot.read_text() == runtime and snapshot.stat().st_mode & 0o777 == 0o600
        assert failed["wg_runtime"] != runtime  # Model a real partial peer replacement.
        update(now=1400)
        restored = run("watchdog")
        assert not (state / "pending").exists() and restored["wg_runtime"] == runtime
        assert restored["keys"] == stage["keys"] and restored["carp"] == initial["carp"]
        assert restored["wg_addresses"] == initial["wg_addresses"]
        promoted = run("promote", "shared", "300")
        assert promoted["wg_runtime"] == runtime and promoted["default4"] == "10.20.0.1"
        assert "192.168.1.138" in promoted["v4"] and promoted["carp"] == initial["carp"]
        update(wg_set_fail_once=True)
        run("rollback", ok=False)
        assert (state / "pending").exists() and snapshot.read_text() == runtime
        restored = run("rollback")
        assert restored["wg_runtime"] == runtime and restored["default4"] == "192.168.1.1"
        assert restored["wg_addresses"] == initial["wg_addresses"] and "192.168.1.138" in restored["v4"]
        print("PASS: unchanged live WG configuration, private complete snapshot, partial failure retry, watchdog and CARP preservation")


if __name__ == "__main__":
    main()
