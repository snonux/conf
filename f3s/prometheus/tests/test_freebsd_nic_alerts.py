"""Evaluate the actual FreeBSD NIC watchdog alert with native promtool (task t74).

Every case lists the alerts it expects at its evaluation time, so the
negative cases (no increase, a reboot reset, bridge/tap errors, an OpenBSD
host, the window after 30m) assert that the alert stays silent. Set
PROMTOOL to a promtool binary or wrapper (default: promtool on PATH).
"""

import os
from pathlib import Path
import subprocess
import tempfile
import unittest

import yaml


MANIFEST = Path(__file__).resolve().parents[1] / "manifests/freebsd-nic-alerts.yaml"
ALERT = "FreebsdNicWatchdogReset"
SAMPLES = 120  # two hours of 1m samples
HOSTS = {"f0": "192.168.2.130:9100", "f1": "192.168.2.131:9100"}


def labels_text(labels):
    return ",".join(f'{k}="{v}"' for k, v in labels.items())


def errs(host, values, device="re0", os_label="freebsd"):
    """node_network_transmit_errs_total of one interface, as node_exporter labels it."""
    labels = {"device": device, "instance": HOSTS[host], "job": "node-exporter", "os": os_label}
    return {"series": f"node_network_transmit_errs_total{{{labels_text(labels)}}}", "values": values}


def uname(host, os_label="freebsd"):
    labels = {"domainname": "lan.buetow.org", "instance": HOSTS[host], "job": "node-exporter",
              "nodename": host, "os": os_label}
    return {"series": f"node_uname_info{{{labels_text(labels)}}}", "values": f"1+0x{SAMPLES}"}


def steady(value=0):
    return f"{value}+0x{SAMPLES}"


def step_at(minute, before=0, after=1):
    """A counter that is `before` up to `minute` and `after` from then on."""
    return f"{before}+0x{minute - 1} {after}+0x{SAMPLES - minute}"


class AlertTests(unittest.TestCase):
    def test_native_rules(self):
        spec = yaml.safe_load(MANIFEST.read_text())["spec"]
        rules = {r["alert"]: r for g in spec["groups"] for r in g["rules"]}
        self.assertEqual(set(rules), {ALERT})

        def expected(host, device="re0"):
            labels = {"device": device, "instance": HOSTS[host], "job": "node-exporter",
                      "nodename": host, "os": "freebsd"}
            annotations = {}
            for key, text in rules[ALERT]["annotations"].items():
                for name, val in labels.items():
                    text = text.replace("{{ $labels.%s }}" % name, val)
                annotations[key] = text
            return {"exp_labels": {**rules[ALERT]["labels"], **labels},
                    "exp_annotations": annotations}

        both_uname = [uname("f0"), uname("f1")]
        quiet = [errs("f0", steady()), errs("f1", steady())]
        cases = [
            ("no-errors", both_uname + quiet, [], "60m"),
            # One watchdog on f1 at minute 30: fires right after it ...
            ("f1-reset-fires", both_uname + [errs("f0", steady()), errs("f1", step_at(30))],
             [expected("f1")], "32m"),
            # ... still within the 30m window ...
            ("f1-reset-still-firing-25m-later", both_uname
             + [errs("f0", steady()), errs("f1", step_at(30))], [expected("f1")], "55m"),
            # ... and resolves once the step left the window.
            ("f1-reset-resolved-after-window", both_uname
             + [errs("f0", steady()), errs("f1", step_at(30))], [], "62m"),
            # A second reset on another host is a second alert.
            ("both-hosts", both_uname + [errs("f0", step_at(20)), errs("f1", step_at(30))],
             [expected("f0"), expected("f1")], "35m"),
            # Counter already 1 from an older reset: no new increase, silent.
            ("old-count-no-alert", both_uname + [errs("f0", steady()), errs("f1", steady(1))],
             [], "60m"),
            # Reboot: the counter drops back to 0, which is not an increase.
            ("reboot-reset-no-alert", both_uname
             + [errs("f0", steady()), errs("f1", step_at(30, before=3, after=0))], [], "40m"),
            # Reboot gap (stale, 4 missed scrapes) and a reset right after
            # boot: 0 -> boot -> 1 is an increase and fires ...
            ("reset-after-short-reboot-fires", both_uname + [errs("f0", steady()), errs(
                "f1", f"0+0x24 stale _x4 1+0x{SAMPLES - 30}")], [expected("f1")], "35m"),
            # ... while 2 -> boot -> 1 is a counter reset plus one: fires too ...
            ("reset-after-reboot-from-2-fires", both_uname + [errs("f0", steady()), errs(
                "f1", f"2+0x24 stale _x4 1+0x{SAMPLES - 30}")], [expected("f1")], "35m"),
            # ... and 1 -> boot -> 1 is the documented blind spot.
            ("reset-after-reboot-same-value-missed", both_uname + [errs("f0", steady()), errs(
                "f1", f"1+0x24 stale _x4 1+0x{SAMPLES - 30}")], [], "35m"),
            # Bridge and tap Oerrs are failed member enqueues, not NIC faults.
            ("bridge-and-tap-ignored", both_uname + quiet
             + [errs("f1", step_at(30, after=500), device="vm-public"),
                errs("f1", step_at(30, after=9), device="tap0")], [], "35m"),
            # An OpenBSD host with an re NIC is not an f-host.
            ("openbsd-ignored", [uname("f0"), uname("f1", os_label="openbsd")]
             + [errs("f0", steady()), errs("f1", step_at(30), os_label="openbsd")], [], "35m"),
            # Without node_uname_info the join has no host name: no alert,
            # which the summary would need anyway (documents the dependency).
            ("no-uname-no-alert", [uname("f0")] + [errs("f0", steady()), errs("f1", step_at(30))],
             [], "35m"),
        ]
        tests = [{"name": name, "interval": "1m", "input_series": inputs,
                  "alert_rule_test": [{"eval_time": when, "alertname": ALERT, "exp_alerts": alerts}]}
                 for name, inputs, alerts, when in cases]
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "rules.yaml").write_text(yaml.safe_dump(spec))
            (root / "tests.yaml").write_text(yaml.safe_dump({
                "rule_files": ["rules.yaml"], "evaluation_interval": "1m", "tests": tests}))
            promtool = os.environ.get("PROMTOOL", "promtool")
            subprocess.run([promtool, "check", "rules", "rules.yaml"], cwd=root, check=True)
            subprocess.run([promtool, "test", "rules", "tests.yaml"], cwd=root, check=True)


if __name__ == "__main__":
    unittest.main()
