"""Evaluate the actual upgrade alerts with native promtool, including failures."""

import os
from pathlib import Path
import subprocess
import tempfile
import unittest

import yaml


MANIFEST = Path(__file__).resolve().parents[1] / "manifests/upgrade-alerts.yaml"
NODE = "r0.lan.buetow.org"
INSTANCE = "192.168.2.133:9100"


class AlertTests(unittest.TestCase):
    def test_native_rules(self):
        spec = yaml.safe_load(MANIFEST.read_text())["spec"]
        rules = {r["alert"]: r for g in spec["groups"] for r in g["rules"]}

        def series(metric, values="1+0x200", **labels):
            text = ",".join(f'{key}="{value}"' for key, value in labels.items())
            return {"series": f"{metric}{{{text}}}", "values": values}

        def k3s(metric, values="1+0x200", node=NODE, service="prometheus-prometheus-node-exporter"):
            return series(metric, values, job="node-exporter", namespace="monitoring",
                          service=service, node=node)

        def kernel(metric, values="1+0x200", os_name="freebsd"):
            return series(metric, values, job="node-exporter", os=os_name, instance=INSTANCE)

        def expected(alert, node=NODE):
            labels = {"node": node} if alert.startswith("K3s") else {
                "instance": INSTANCE, "job": "node-exporter", "os": "freebsd"}
            annotation = {k: v.replace("{{ $labels.node }}", node)
                          .replace("{{ $labels.instance }}", INSTANCE)
                          for k, v in rules[alert]["annotations"].items()}
            return {"exp_labels": {**rules[alert]["labels"], **labels},
                    "exp_annotations": annotation}

        nodes = [series("kube_node_info", node=NODE)]
        healthy = nodes + [k3s("f3s_k3s_check_success"),
                           k3s("f3s_k3s_check_timestamp_seconds", "0+60x200")]
        kernel_healthy = [kernel("up"), kernel("node_freebsd_kernel_check_success"),
                          kernel("node_freebsd_kernel_check_timestamp_seconds", "0+60x200")]
        coverage = "K3sVersionCheckCoverage"
        kernel_coverage = "FreebsdKernelCheckCoverage"
        cases = [
            ("clean", healthy + kernel_healthy, {}, "70m"),
            ("patch", healthy + [k3s("f3s_k3s_update_available")],
             {"K3sPatchUpdateAvailable": [expected("K3sPatchUpdateAvailable")]}, "70m"),
            ("minor", healthy + [k3s("f3s_k3s_minor_update_available")],
             {"K3sMinorUpdateAvailable": [expected("K3sMinorUpdateAvailable")]}, "70m"),
            ("missing", nodes, {coverage: [expected(coverage)]}, "70m"),
            ("failed", nodes + [k3s("f3s_k3s_check_success", "0+0x200")],
             {coverage: [expected(coverage)]}, "70m"),
            ("stale", nodes + [k3s("f3s_k3s_check_success"),
                                k3s("f3s_k3s_check_timestamp_seconds", "0+0x200")],
             {coverage: [expected(coverage)]}, "180m"),
            ("future", nodes + [k3s("f3s_k3s_check_success"),
                                 k3s("f3s_k3s_check_timestamp_seconds", "50000+60x200")],
             {coverage: [expected(coverage)]}, "70m"),
            ("wrong-service", nodes + [k3s("f3s_k3s_check_success", service="other"),
                     k3s("f3s_k3s_check_timestamp_seconds", "0+60x200", service="other")],
             {coverage: [expected(coverage)]}, "70m"),
            ("one-peer-missing", healthy + [series("kube_node_info", node="r1.lan.buetow.org")],
             {coverage: [expected(coverage, "r1.lan.buetow.org")]}, "70m"),
            ("kernel-pending", kernel_healthy + [kernel("node_freebsd_kernel_reboot_pending")],
             {"FreebsdKernelRebootPending": [expected("FreebsdKernelRebootPending")]}, "70m"),
            ("kernel-missing", [kernel("up")],
             {kernel_coverage: [expected(kernel_coverage)]}, "70m"),
            ("kernel-failed", [kernel("up"), kernel("node_freebsd_kernel_check_success", "0+0x200")],
             {kernel_coverage: [expected(kernel_coverage)]}, "70m"),
            ("kernel-stale", [kernel("up"), kernel("node_freebsd_kernel_check_success"),
                              kernel("node_freebsd_kernel_check_timestamp_seconds", "0+0x200")],
             {kernel_coverage: [expected(kernel_coverage)]}, "70m"),
            ("exporter-down", [kernel("up", "0+0x200")], {}, "70m"),
            ("other-os", [kernel("up", os_name="openbsd")], {}, "70m"),
            ("pending-grace", nodes + [kernel("up")], {}, "5m"),
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
            subprocess.run([promtool, "check", "rules", str(root / "rules.yaml")], check=True)
            subprocess.run([promtool, "test", "rules", str(root / "tests.yaml")], cwd=root, check=True)


if __name__ == "__main__":
    unittest.main()
