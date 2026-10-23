"""Render the real pinned chart and validate the public scrape configuration."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

import yaml

ROOT = Path(__file__).resolve().parents[3]


class PrometheusTests(unittest.TestCase):
    def test_both_value_sets_mount_the_external_secret(self):
        chart = os.environ["HELM_CHART"]
        app = yaml.safe_load((ROOT / "f3s/argocd-apps/monitoring/prometheus.yaml").read_text())
        authoritative = app["spec"]["sources"][0]["helm"]["valuesObject"]
        with tempfile.TemporaryDirectory(prefix="cn3-chart-") as directory:
            source = Path(directory) / "values.yaml"
            source.write_text(yaml.safe_dump(authoritative))
            for values in (source, ROOT / "f3s/prometheus/persistence-values.yaml"):
                with self.subTest(values=values.name):
                    output = subprocess.check_output(["helm", "template", "prometheus", chart,
                                                      "-n", "monitoring", "-f", str(values)], text=True)
                    prom = next(d for d in yaml.safe_load_all(output) if d and d["kind"] == "Prometheus")
                    spec = prom["spec"]
                    self.assertEqual(spec["secrets"], ["garage-metrics"])
                    self.assertEqual(spec["additionalScrapeConfigs"], {
                        "name": "additional-scrape-configs", "key": "additional-scrape-configs.yaml"})
                    self.assertFalse(spec["enableAdminAPI"])
                    self.assertFalse(spec["enableOTLPReceiver"])

    def test_native_promtool_accepts_both_scrape_files(self):
        promtool = os.environ["PROMTOOL_TEST_BINARY"]
        plain = yaml.safe_load((ROOT / "f3s/prometheus/additional-scrape-configs.yaml").read_text())
        secret = yaml.safe_load((ROOT / "f3s/prometheus/manifests/additional-scrape-configs-secret.yaml").read_text())
        embedded = yaml.safe_load(secret["stringData"]["additional-scrape-configs.yaml"])
        with tempfile.TemporaryDirectory(prefix="cn3-promtool-") as directory:
            config = Path(directory) / "prometheus.yml"
            for jobs in (plain, embedded):
                config.write_text(yaml.safe_dump({"scrape_configs": jobs}))
                result = subprocess.run([promtool, "check", "config", "--syntax-only", str(config)],
                                        capture_output=True, text=True, timeout=15)
                self.assertEqual(result.returncode, 0, "native promtool rejected the scrape configuration")


if __name__ == "__main__":
    unittest.main()
