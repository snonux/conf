"""Evaluate the actual Shelly TLS alerts with native promtool (task s44).

Every case evaluates every alert of the rule file, so a case lists all the
alerts it expects and asserts that the others stay silent. Thresholds are
probed on both sides (59d/61d, just under/over 14d, 1h/2.5h failing,
35h/37h stale, 2h/3.5h absent). Set PROMTOOL to a promtool binary or
wrapper (default: promtool on PATH).
"""

import os
from pathlib import Path
import subprocess
import tempfile
import unittest

import yaml


MANIFEST = Path(__file__).resolve().parents[1] / "manifests/shelly-tls-alerts.yaml"
DAY = 86400
HOUR = 3600
SAMPLES = 2400  # 40h of 1m samples, enough for the 37h stale case


def humanize_duration(seconds):
    """Prometheus's humanizeDuration for whole seconds (|v| >= 1)."""
    sign = "-" if seconds < 0 else ""
    v = abs(int(seconds))
    days, hours, minutes, secs = v // DAY, v // HOUR % 24, v // 60 % 60, v % 60
    if days:
        return f"{sign}{days}d {hours}h {minutes}m {secs}s"
    if hours:
        return f"{sign}{hours}h {minutes}m {secs}s"
    if minutes:
        return f"{sign}{minutes}m {secs}s"
    return f"{sign}{secs}s"


def series(metric, values, **labels):
    labels = {"job": "shelly_tls", **labels}
    text = ",".join(f'{k}="{v}"' for k, v in labels.items())
    return {"series": f"{metric}{{{text}}}", "values": values}


def const(value):
    return f"{int(value)}+0x{SAMPLES}"


FRESH = f"0+60x{SAMPLES}"  # a timestamp metric that always equals time()


def reporter(name, success=1, last_success=FRESH, last_run=FRESH, plugs=("shelly1", "shelly2")):
    out = [series("shelly_tls_probe_last_run_timestamp_seconds", last_run, reporter=name)]
    for plug in plugs:
        address = {"shelly1": "10.30.0.28", "shelly2": "10.30.0.29"}[plug]
        out.append(series("shelly_tls_probe_success", const(success),
                          reporter=name, plug=plug, address=address))
        out.append(series("shelly_tls_probe_last_success_timestamp_seconds", last_success,
                          reporter=name, plug=plug))
    return out


def not_after(name, when, plug=None, cert="leaf"):
    labels = {"reporter": name, "cert": cert}
    if plug:
        labels["plug"] = plug
    return series("shelly_tls_cert_not_after_seconds", const(when), **labels)


class AlertTests(unittest.TestCase):
    def test_native_rules(self):
        spec = yaml.safe_load(MANIFEST.read_text())["spec"]
        rules = {r["alert"]: r for g in spec["groups"] for r in g["rules"]}
        self.assertEqual(set(rules), {"ShellyTLSCertExpiringSoon", "ShellyTLSCertExpiryCritical",
                                      "ShellyTLSProbeFailing", "ShellyTLSProbeStale",
                                      "ShellyTLSProbeAbsent"})

        def render(alert, labels, value=None):
            out = {}
            for key, text in rules[alert]["annotations"].items():
                plug = labels.get("plug")
                text = text.replace("{{ with $labels.plug }} of {{ . }}{{ end }}",
                                    f" of {plug}" if plug else "")
                for name, val in labels.items():
                    text = text.replace("{{ $labels.%s }}" % name, val)
                if value is not None:
                    text = text.replace("{{ $value | humanizeDuration }}", humanize_duration(value))
                # A folded YAML scalar ends in a newline that templating keeps.
                out[key] = text
            return out

        def expected(alert, value=None, **labels):
            return {"exp_labels": {**rules[alert]["labels"], **labels},
                    "exp_annotations": render(alert, labels, value)}

        def expiry(alert, left, plug="shelly1", cert="leaf"):
            labels = {"cert": cert, **({"plug": plug} if plug else {})}
            return expected(alert, left, **labels)

        def failing(rep, plug):
            address = {"shelly1": "10.30.0.28", "shelly2": "10.30.0.29"}[plug]
            return expected("ShellyTLSProbeFailing", job="shelly_tls", reporter=rep,
                            plug=plug, address=address)

        t1 = HOUR  # default evaluation time
        healthy_certs = [not_after(r, t1 + 800 * DAY, p) for r in ("pi0", "pi1")
                         for p in ("shelly1", "shelly2")]
        healthy_certs += [not_after(r, t1 + 3600 * DAY, cert="ca") for r in ("pi0", "pi1")]
        both = reporter("pi0") + reporter("pi1")

        def certs_with(when, plug="shelly1", cert="leaf"):
            """Healthy certificates, but one (on both reporters) changed."""
            out = []
            for s in healthy_certs:
                text = s["series"]
                is_target = f'cert="{cert}"' in text and (plug is None or f'plug="{plug}"' in text)
                out.append(not_after("pi0" if "pi0" in text else "pi1", when, plug, cert)
                           if is_target else s)
            return out

        soon, crit = "ShellyTLSCertExpiringSoon", "ShellyTLSCertExpiryCritical"
        cases = [
            ("healthy", both + healthy_certs, {}, "1h"),
            ("healthy-later", both + healthy_certs, {}, "4h"),
            ("61d-no-alert", both + certs_with(t1 + 61 * DAY), {}, "1h"),
            ("60d-plus-1h-no-alert", both + certs_with(t1 + 60 * DAY + HOUR), {}, "1h"),
            ("60d-minus-1h-warning", both + certs_with(t1 + 60 * DAY - HOUR),
             {soon: [expiry(soon, 60 * DAY - HOUR)]}, "1h"),
            ("59d-warning", both + certs_with(t1 + 59 * DAY), {soon: [expiry(soon, 59 * DAY)]}, "1h"),
            ("15d-still-warning", both + certs_with(t1 + 15 * DAY, "shelly2"),
             {soon: [expiry(soon, 15 * DAY, "shelly2")]}, "1h"),
            ("14d-plus-1h-warning", both + certs_with(t1 + 14 * DAY + HOUR),
             {soon: [expiry(soon, 14 * DAY + HOUR)]}, "1h"),
            ("14d-minus-1h-critical-only", both + certs_with(t1 + 14 * DAY - HOUR),
             {crit: [expiry(crit, 14 * DAY - HOUR)]}, "1h"),
            ("expired-critical", both + certs_with(t1 - DAY),
             {crit: [expiry(crit, -DAY)]}, "1h"),
            ("ca-59d-warning", both + certs_with(t1 + 59 * DAY, None, "ca"),
             {soon: [expiry(soon, 59 * DAY, None, "ca")]}, "1h"),
            ("ca-13d-critical", both + certs_with(t1 + 13 * DAY, None, "ca"),
             {crit: [expiry(crit, 13 * DAY, None, "ca")]}, "1h"),
            # The reporters disagree: the earlier date wins, still one alert.
            ("reporters-disagree-min-wins", both + [
                s for s in healthy_certs if 'plug="shelly1"' not in s["series"]]
             + [not_after("pi0", t1 + 59 * DAY, "shelly1"), not_after("pi1", t1 + 300 * DAY, "shelly1")],
             {soon: [expiry(soon, 59 * DAY)]}, "1h"),
            # Failing: evaluated at 4h30m.
            ("failing-2h30m", reporter("pi0", success=0, last_success=const(2 * HOUR), plugs=("shelly1",))
             + reporter("pi0", plugs=("shelly2",))[1:] + reporter("pi1") + healthy_certs,
             {"ShellyTLSProbeFailing": [failing("pi0", "shelly1")]}, "270m"),
            ("failing-only-1h-no-alert", reporter("pi0", success=0, last_success=const(3.5 * HOUR))
             + reporter("pi1") + healthy_certs, {}, "270m"),
            ("never-verified-fires", reporter("pi1", success=0, last_success=const(0), plugs=("shelly2",))
             + reporter("pi1", plugs=("shelly1",))[1:] + reporter("pi0") + healthy_certs,
             {"ShellyTLSProbeFailing": [failing("pi1", "shelly2")]}, "270m"),
            ("success-with-old-record-no-alert", reporter("pi0", success=1, last_success=const(0))
             + reporter("pi1") + healthy_certs, {}, "270m"),
            # Stale: last run frozen at t=0.
            ("stale-37h", reporter("pi0", last_run=const(0)) + reporter("pi1") + healthy_certs,
             {"ShellyTLSProbeStale": [expected("ShellyTLSProbeStale", 37 * HOUR,
                                               job="shelly_tls", reporter="pi0")]}, "37h"),
            ("stale-35h-no-alert", reporter("pi0", last_run=const(0)) + reporter("pi1") + healthy_certs,
             {}, "35h"),
            # Absent: pi1 never pushed.
            ("absent-3h30m", reporter("pi0") + healthy_certs[:2],
             {"ShellyTLSProbeAbsent": [expected("ShellyTLSProbeAbsent", job="shelly_tls", reporter="pi1")]},
             "210m"),
            ("absent-2h-pending-only", reporter("pi0") + healthy_certs[:2], {}, "2h"),
            ("absent-both", [], {"ShellyTLSProbeAbsent": [
                expected("ShellyTLSProbeAbsent", job="shelly_tls", reporter=r) for r in ("pi0", "pi1")]},
             "210m"),
            # Another job's identically named series never counts.
            ("other-job-ignored", [dict(s, series=s["series"].replace('job="shelly_tls"', 'job="other"'))
                                   for s in both + certs_with(t1 + DAY)] + both + healthy_certs, {}, "1h"),
        ]
        tests = [{"name": name, "interval": "1m", "input_series": inputs,
                  "alert_rule_test": [{"eval_time": when, "alertname": alert,
                                       "exp_alerts": alerts.get(alert, [])}
                                      for alert in rules]}
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
