"""Validate actual Gonf-rendered alternate-listener TLS and certificate checks."""

import base64
import copy
import ipaddress
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[3]
SITE = "code.f3s.buetow.org"


def render():
    with tempfile.TemporaryDirectory() as directory:
        subprocess.run(
            [str(ROOT / "gonf.sh"), "plan", "-o", directory,
             "frontends_relayd", "frontends_d_tail_checks"],
            env={**os.environ, "GOMAXPROCS": "2", "GOFLAGS": "-p=1"},
            capture_output=True, check=True, timeout=90)
        rows = [json.loads(line) for line in
                (Path(directory) / "plan.jsonl").read_text().splitlines()]
    configs = {"/etc/relayd.conf": {}, "/etc/gogios.json": {}}
    hosts = []
    for row in rows:
        if row["op"] == "when_begin":
            host = next((c["eq"] for c in row.get("all", [])
                         if c.get("fact") == "hostname_contains" and "eq" in c), None)
            hosts.append(host)
        elif row["op"] == "when_end":
            hosts.pop()
        elif row.get("path") in configs:
            host = next(h for h in reversed(hosts) if h)
            configs[row["path"]][host] = base64.b64decode(row["content_b64"]).decode()
    return configs


def forgejo_protocol(config):
    return re.search(r'http protocol "forgejo-alt" \{\n(.*?)\n\}', config, re.S)[1]


def assert_sni(config):
    pairs = set(re.findall(r"tls keypair (\S+)", forgejo_protocol(config)))
    if pairs != {SITE, "standby." + SITE}:
        raise AssertionError("alternate listener does not bind both SNI certificates")


def assert_monitoring(config, addresses):
    checks = {name: check for name, check in config["Checks"].items()
              if name.startswith("Check TLS Certificate " + SITE + " via ")}
    if len(checks) != len(addresses):
        raise AssertionError("missing frontend certificate coverage")
    for host, address in addresses.items():
        name = f"Check TLS Certificate {SITE} via {host}.buetow.org:2443"
        check = checks.get(name, {})
        if check.get("Args") != ["--sni", "-H", SITE, "-I", address,
                                  "-p", "2443", "-C", "20"]:
            raise AssertionError("wrong primary SNI or unpinned frontend address")
        if check.get("DependsOn") != [f"Check Ping4 {host}.buetow.org"]:
            raise AssertionError("wrong certificate probe dependency")


class TLSConfigTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.configs = render()
        cls.relays = cls.configs["/etc/relayd.conf"]
        cls.monitors = {host: json.loads(value) for host, value in
                        cls.configs["/etc/gogios.json"].items()}
        cls.addresses = {}
        for host, value in cls.relays.items():
            address = re.search(r"listen on ([0-9.]+) port 2443 tls", value)[1]
            if ipaddress.ip_address(address).version != 4:
                raise AssertionError("candidate listener is not IPv4")
            cls.addresses[host] = address

    def test_actual_render_binds_sni_and_covers_each_frontend(self):
        self.assertEqual(len(self.relays), 2)
        self.assertEqual(set(self.relays), set(self.monitors))
        self.assertEqual(len(set(self.addresses.values())), 2)
        for config in self.relays.values():
            assert_sni(config)
            allowed = re.findall(r'pass request header "Host" value "([^"]+)"',
                                 forgejo_protocol(config))
            self.assertEqual(set(allowed), {SITE, SITE + ":2443"})
        for config in self.monitors.values():
            assert_monitoring(config, self.addresses)

    def test_previous_single_keypair_is_rejected(self):
        for config in self.relays.values():
            old = config.replace("    tls keypair standby." + SITE + "\n", "")
            with self.assertRaises(AssertionError):
                assert_sni(old)

    def test_unpinned_wrong_sni_duplicate_endpoint_and_missing_check_rejected(self):
        original = next(iter(self.monitors.values()))
        names = [name for name in original["Checks"] if " via " in name
                 and name.startswith("Check TLS Certificate " + SITE)]
        mutants = []
        unpinned = copy.deepcopy(original)
        args = unpinned["Checks"][names[0]]["Args"]
        i = args.index("-I")
        del args[i:i + 2]
        mutants.append(unpinned)
        wrong_sni = copy.deepcopy(original)
        args = wrong_sni["Checks"][names[0]]["Args"]
        args[args.index("-H") + 1] = "standby." + SITE
        mutants.append(wrong_sni)
        duplicate = copy.deepcopy(original)
        first = duplicate["Checks"][names[0]]["Args"]
        second = duplicate["Checks"][names[1]]["Args"]
        second[second.index("-I") + 1] = first[first.index("-I") + 1]
        mutants.append(duplicate)
        missing = copy.deepcopy(original)
        del missing["Checks"][names[0]]
        mutants.append(missing)
        for mutant in mutants:
            with self.subTest():
                with self.assertRaises(AssertionError):
                    assert_monitoring(mutant, self.addresses)


if __name__ == "__main__":
    unittest.main()
