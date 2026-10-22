#!/usr/bin/env python3
"""Record real Foostats recipes with dummy keys and test the purge on fixtures.

Usage: python3 foostats-privacy.py /path/to/reviewed/foostats
No production provider, command, purge, configuration or connection is used.
"""
import base64
import ipaddress
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[3]
SOURCE = Path(sys.argv.pop(1)).resolve()
PUBLIC = "/var/www/htdocs/buetow.org/self/foostats"
HARNESS = r'''package main
import (
 "context"
 "fmt"
 "strings"
 "github.com/snonux/conf/gonf/cluster"
 "github.com/snonux/conf/gonf/tasks"
 "github.com/snonux/gonf/api"
 "github.com/snonux/gonf/cli"
 "github.com/snonux/gonf/secret"
)
func main() {
 api.SetSecretProvider(secret.ProviderFunc(func(_ context.Context, ref secret.Ref) ([]byte,error) {
  switch string(ref) {
  case "frontends/etc/rsync/shuriken.password": return []byte("fixture-rsync-password"),nil
  case "frontends/etc/foostats/ip-hmac.key": return []byte(strings.Repeat("a1",32)),nil
  default: return nil,fmt.Errorf("unexpected fixture secret")
  }
 }))
 cluster.Register(); tasks.Register(); cli.Main()
}
'''


class PrivacyPlan(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.workspace = tempfile.TemporaryDirectory(prefix="conf-foostats-tests-")
        cls.addClassCleanup(cls.workspace.cleanup)
        cls.root = Path(cls.workspace.name)
        harness = cls.root / "fixture.go"
        harness.write_text(HARNESS)
        cls.binary = cls.root / "gonf-fixture"
        subprocess.run(["go", "build", "-p", "1", "-o", str(cls.binary), str(harness)],
                       cwd=ROOT / "gonf", check=True, capture_output=True)
        cls.env = {**os.environ, "GONF_CONF_ROOT": str(ROOT), "GONF_FOOSTATS_ROOT": str(SOURCE)}
        result = cls.record(cls.root / "plan")
        if result.returncode:
            raise AssertionError(result.stderr.decode())
        cls.ops = [json.loads(line) for line in (cls.root / "plan/plan.jsonl").read_text().splitlines()]

    @classmethod
    def record(cls, destination, source=None):
        env = {**cls.env}
        if source is not None:
            env["GONF_FOOSTATS_ROOT"] = str(source)
        return subprocess.run([str(cls.binary), "plan", "-o", str(destination),
                               "frontends_foostats_purge_public"], cwd=ROOT / "gonf",
                              env=env, capture_output=True, timeout=30)

    def test_private_paths_key_modes_and_peer_only_exchange(self):
        keys = [op for op in self.ops if op.get("path") == "/etc/foostats/ip-hmac.key"]
        self.assertEqual(len(keys), 2)
        for op in keys:
            self.assertTrue(op["sensitive"])
            self.assertEqual(op["mode"], "0600")
            self.assertEqual(op["owner"], "root")
        dirs = [op for op in self.ops if op.get("path") == "/var/db/foostats"]
        self.assertEqual(len(dirs), 2)
        self.assertTrue(all(op["mode"] == "0750" for op in dirs))
        self.assertTrue(all(op["owner"] == "root" and op["group"] == "www" for op in dirs))
        self.assertFalse(any(op.get("path", "").startswith(PUBLIC + "/") for op in self.ops))
        daily = [op for op in self.ops if op.get("path") == "/etc/daily.local"]
        for op in daily:
            self.assertIn("--partner-address 192.168.2.", json.dumps(op))
        configs = [base64.b64decode(op["content_b64"]).decode() for op in self.ops
                   if op.get("path") == "/etc/rsyncd.conf"]
        for config in configs:
            block = re.search(r'\[foostats-private\](.*?)(?=\n\[|\Z)', config, re.S)[1]
            self.assertIn("read only = yes", block)
            self.assertIn("list = no", block)
            self.assertIn("path = /var/db/foostats", block)
            sources = re.search(r'^hosts allow = (.*)$', block, re.M)[1].split()
            self.assertTrue(sources)
            for source in sources:
                address = ipaddress.ip_address(source)
                self.assertTrue(address.is_loopback or str(address) in {
                    "192.168.2.110", "192.168.2.111", "fd42:beef:cafe:2::110", "fd42:beef:cafe:2::111"})
            self.assertNotIn("192.168.2.120", block)

    def test_retired_script_cannot_be_selected_as_fallback(self):
        destination = self.root / "stale-plan"
        result = self.record(destination, ROOT / "frontends/scripts")
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse((destination / "plan.jsonl").exists())
        self.assertIn(b"private-statistics support", result.stderr)

    def test_real_purge_command_only_unlinks_legacy_counter_fixtures(self):
        op = next(op for op in self.ops if op.get("id") == "Command[foostats-purge-public-archives]")
        args = op["args"]
        script = args[-1]
        self.assertEqual(script.count(PUBLIC), 1)
        public = self.root / "public"
        public.mkdir()
        outside = self.root / "outside.json.gz"
        outside.write_text("preserve outside")
        raw = public / "web_20261020.peer.example.json.gz"
        raw.write_text("private fixture")
        partial = public / "gemini_20261020.peer.example.json.gz.tmp"
        partial.write_text("partial private fixture")
        link = public / "web_20261021.peer.example.json.gz"
        link.symlink_to(outside)
        aggregate = public / "20261020.gmi"
        aggregate.write_text("preserve aggregate")
        unrelated = public / "other.json.gz"
        unrelated.write_text("preserve unrelated")
        fixture_args = args[:-1] + [script.replace(PUBLIC, str(public))]
        result = subprocess.run(["perl", *fixture_args], capture_output=True, timeout=5)
        self.assertEqual(result.returncode, 0, result.stderr.decode())
        self.assertFalse(raw.exists())
        self.assertFalse(partial.exists())
        self.assertFalse(link.is_symlink())
        self.assertTrue(outside.exists())
        self.assertTrue(aggregate.exists())
        self.assertTrue(unrelated.exists())
        subprocess.run(["perl", *fixture_args], check=True, capture_output=True, timeout=5)
        alias = self.root / "public-alias"
        alias.symlink_to(public, target_is_directory=True)
        refused = subprocess.run(["perl", *args[:-1], script.replace(PUBLIC, str(alias))],
                                  capture_output=True, timeout=5)
        self.assertNotEqual(refused.returncode, 0)
        self.assertTrue(aggregate.exists())


if __name__ == "__main__":
    unittest.main(verbosity=2)
