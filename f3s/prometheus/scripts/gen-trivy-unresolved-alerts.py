#!/usr/bin/env python3
"""Refresh TRIVY-UNRESOLVED-ALERTS.md from live Prometheus (+ Alertmanager count)."""

from __future__ import annotations

import argparse
import json
import subprocess
import datetime
from collections import defaultdict
from pathlib import Path


def kubectl_exec(pod: str, ns: str, container: str, url: str) -> bytes:
    return subprocess.check_output(
        [
            "kubectl",
            "exec",
            "-n",
            ns,
            pod,
            "-c",
            container,
            "--",
            "wget",
            "-qO-",
            url,
        ],
        stderr=subprocess.DEVNULL,
    )


def main() -> None:
    script_dir = Path(__file__).resolve().parent
    default_out = script_dir.parent / "TRIVY-UNRESOLVED-ALERTS.md"

    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument(
        "-o",
        "--output",
        type=Path,
        default=default_out,
        help=f"markdown output path (default: {default_out})",
    )
    args = ap.parse_args()

    prom_pod = "prometheus-prometheus-kube-prometheus-prometheus-0"
    prom_ns = "monitoring"
    prom_c = "prometheus"

    raw = kubectl_exec(
        prom_pod, prom_ns, prom_c, "http://127.0.0.1:9090/api/v1/alerts"
    )
    data = json.loads(raw)

    rows: list[dict] = []
    for a in data.get("data", {}).get("alerts", []):
        lab = a.get("labels") or {}
        name = lab.get("alertname", "")
        if not name.startswith("Trivy"):
            continue
        st = a.get("state")
        if st not in ("firing", "pending"):
            continue
        rows.append(
            {
                "state": st,
                "alertname": name,
                "namespace": lab.get("namespace", "—"),
                "resource": lab.get("resource_name", "—"),
                "container": lab.get("container_name", "—"),
                "image": lab.get("image_repository", "—"),
                "activeAt": a.get("activeAt", "—"),
            }
        )

    agg: dict = defaultdict(
        lambda: {"critical": set(), "high": set(), "image": "", "ns": "", "res": "", "cont": ""}
    )
    for r in rows:
        key = (r["namespace"], r["resource"], r["container"])
        sev = "critical" if "Critical" in r["alertname"] else "high"
        agg[key][sev].add(r["state"])
        agg[key]["image"] = r["image"]
        agg[key]["ns"] = r["namespace"]
        agg[key]["res"] = r["resource"]
        agg[key]["cont"] = r["container"]

    def fmt_states(states: set) -> str:
        if not states:
            return "—"
        return ", ".join(sorted(states))

    workloads = []
    for _k, v in agg.items():
        workloads.append(
            {
                "ns": v["ns"],
                "resource": v["res"],
                "container": v["cont"],
                "image": v["image"],
                "critical": fmt_states(v["critical"]),
                "high": fmt_states(v["high"]),
            }
        )
    workloads.sort(key=lambda x: (x["ns"], x["resource"], x["container"]))

    n_crit = sum(1 for r in rows if "Critical" in r["alertname"])
    n_high = sum(1 for r in rows if "High" in r["alertname"])
    n_series = len(rows)
    n_workloads = len(workloads)

    am_count = None
    try:
        am_raw = kubectl_exec(
            "alertmanager-prometheus-kube-prometheus-alertmanager-0",
            prom_ns,
            "alertmanager",
            "http://127.0.0.1:9093/api/v2/alerts?active=true&silenced=false&inhibited=false",
        )
        am_alerts = json.loads(am_raw)
        am_count = len(
            [
                x
                for x in am_alerts
                if (x.get("labels") or {}).get("alertname", "").startswith("Trivy")
            ]
        )
    except (subprocess.CalledProcessError, json.JSONDecodeError):
        pass

    date = datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%d %H:%M UTC")

    def esc(s: str) -> str:
        return str(s).replace("|", "\\|")

    lines = [
        "# Unresolved Trivy alerts (Prometheus)",
        "",
        f"Generated: **{date}** from Prometheus `GET /api/v1/alerts` "
        f"(pod `{prom_pod}`, namespace `{prom_ns}`).",
        "",
        "## Definitions",
        "",
        "- **Trivy alerts:** `TrivyContainerCriticalVulnerabilities` and "
        "`TrivyContainerHighVulnerabilities` (label `component=trivy`).",
        "- **Unresolved:** alert state is `firing` or `pending` in Prometheus "
        "(still active, not cleared).",
    ]
    if am_count is not None:
        lines.append(
            "- **Alertmanager (active, not silenced, not inhibited):** "
            f"{am_count} Trivy alert(s)."
        )
    lines.extend(
        [
            "",
            "## Summary",
            "",
            "| Metric | Value |",
            "|--------|------:|",
            f"| Active Trivy alert time series (`firing` + `pending`) | {n_series} |",
            f"| Of those, Critical rule instances | {n_crit} |",
            f"| Of those, High rule instances | {n_high} |",
            f"| Distinct workloads (namespace / resource / container) | {n_workloads} |",
            "",
            "One workload can appear once below but still account for two time series "
            "if both Critical and High are active.",
            "",
            "## By workload",
            "",
            "| Namespace | Resource | Container | Image | Critical | High |",
            "|-----------|----------|-----------|-------|----------|------|",
        ]
    )
    for w in workloads:
        lines.append(
            f"| {esc(w['ns'])} | {esc(w['resource'])} | {esc(w['container'])} | "
            f"`{esc(w['image'])}` | {esc(w['critical'])} | {esc(w['high'])} |"
        )

    lines.extend(
        [
            "",
            "## Raw alert series (optional detail)",
            "",
            "| State | Alert | Namespace | Resource | Container | Image | Active since |",
            "|-------|-------|-----------|----------|-----------|-------|--------------|",
        ]
    )
    for r in sorted(
        rows,
        key=lambda x: (x["namespace"], x["resource"], x["container"], x["alertname"]),
    ):
        lines.append(
            f"| {esc(r['state'])} | `{esc(r['alertname'])}` | {esc(r['namespace'])} | "
            f"{esc(r['resource'])} | {esc(r['container'])} | `{esc(r['image'])}` | "
            f"{esc(r['activeAt'])} |"
        )

    lines.extend(
        [
            "",
            "---",
            "",
            "## CVE detail",
            "",
            "Prometheus alerts do not list CVE IDs. Inspect Trivy reports, for example:",
            "",
            "```bash",
            "kubectl get vulnerabilityreports -A",
            "kubectl describe vulnerabilityreport -n <namespace> <name>",
            "```",
            "",
            "---",
            "",
            "## Regenerate",
            "",
            "From the conf repo root (with `kubectl` pointing at the cluster):",
            "",
            "```bash",
            "python3 f3s/prometheus/scripts/gen-trivy-unresolved-alerts.py",
            "```",
            "",
            "Optional: `-o /path/to/out.md`",
            "",
        ]
    )

    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text("\n".join(lines) + "\n", encoding="utf-8")
    print(f"Wrote {args.output}")


if __name__ == "__main__":
    main()
