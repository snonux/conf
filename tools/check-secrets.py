#!/usr/bin/env python3
"""Run pinned Gitleaks and print location metadata, never finding contents."""
import argparse
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile


POLICY_ROOT = Path(__file__).resolve().parents[1]
# Selecting fields here prevents matches, secrets, commit messages and authors
# from entering even the temporary report. Scanner console output is discarded.
REPORT_TEMPLATE = '''[
{{- range $i, $f := . }}{{if $i}},{{end}}
{{ dict "rule" $f.RuleID "file" $f.File "line" $f.StartLine "commit" $f.Commit "fingerprint" $f.Fingerprint | toJson }}
{{- end }}
]
'''


def scan(repo, history=False):
    ignored = repo / ".gitleaksignore"
    if ignored.exists() or ignored.is_symlink():
        # Gitleaks also reads this automatically despite --gitleaks-ignore-path.
        print("Secret scan blocked: .gitleaksignore is unsupported; review narrow policy exceptions instead.",
              file=sys.stderr)
        return 2
    expected = (POLICY_ROOT / ".gitleaks-version").read_text().strip()
    binary = os.environ.get("GITLEAKS_BIN") or shutil.which("gitleaks")
    if not binary:
        print(f"Secret scan unavailable: install Gitleaks {expected}.", file=sys.stderr)
        return 2
    try:
        version = subprocess.run([binary, "version"], capture_output=True,
                                 text=True, timeout=10, check=False)
    except (OSError, subprocess.TimeoutExpired):
        print("Secret scan unavailable: scanner could not start.", file=sys.stderr)
        return 2
    if version.returncode or version.stdout.strip().lstrip("v") != expected:
        print(f"Secret scan unavailable: Gitleaks {expected} is required.", file=sys.stderr)
        return 2
    with tempfile.TemporaryDirectory(prefix="conf-secret-scan-") as directory:
        root = Path(directory)
        root.chmod(0o700)
        template = root / "metadata.tmpl"
        template.write_text(REPORT_TEMPLATE)
        report = root / "metadata.json"
        # Read the policy beside this script in the working tree, including its
        # exact public placeholders. This local hook is editable by its user.
        ignore = root / "empty-ignore"
        ignore.write_text("")
        command = [binary, "git", str(repo), "--no-banner", "--no-color",
                   "--redact=100", "--ignore-gitleaks-allow", "--timeout=120",
                   "--baseline-path=",
                   "--config", str(POLICY_ROOT / ".gitleaks.toml"),
                   "--gitleaks-ignore-path", str(ignore),
                   "--report-format=template", "--report-template", str(template),
                   "--report-path", str(report)]
        command += ["--log-opts=HEAD"] if history else ["--pre-commit", "--staged"]
        env = dict(os.environ, GOMAXPROCS="2", GOMEMLIMIT="512MiB")
        for name in ("GITLEAKS_CONFIG", "GITLEAKS_CONFIG_TOML"):
            env.pop(name, None)
        try:
            result = subprocess.run(command, env=env, stdout=subprocess.DEVNULL,
                                    stderr=subprocess.DEVNULL, timeout=130, check=False)
        except (OSError, subprocess.TimeoutExpired):
            print("Secret scan failed or timed out; commit blocked.", file=sys.stderr)
            return 2
        try:
            findings = json.loads(report.read_text())
        except (OSError, ValueError):
            print("Secret scan failed to produce a valid metadata report; commit blocked.", file=sys.stderr)
            return 2
        if result.returncode not in (0, 1) or not isinstance(findings, list):
            print("Secret scan failed; commit blocked.", file=sys.stderr)
            return 2
        if not all(isinstance(item, dict) and
                   set(item) == {"file", "line", "rule", "commit", "fingerprint"} and
                   isinstance(item["line"], int) and
                   all(isinstance(item[key], str) for key in ("file", "rule", "commit", "fingerprint"))
                   for item in findings):
            print("Secret scan returned invalid location metadata; commit blocked.", file=sys.stderr)
            return 2
        if result.returncode == 0 and findings or result.returncode == 1 and not findings:
            print("Secret scan returned inconsistent results; commit blocked.", file=sys.stderr)
            return 2
        if findings:
            print(f"Secret scan blocked: {len(findings)} potential secret(s).", file=sys.stderr)
            for item in findings:
                # JSON quoting prevents filenames from injecting terminal lines.
                safe = {key: item.get(key) for key in ("file", "line", "rule", "commit", "fingerprint")}
                print(json.dumps(safe, ensure_ascii=True), file=sys.stderr)
            return 1
    print("Secret scan passed (history)." if history else "Secret scan passed (staged index).")
    return 0


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    mode = parser.add_mutually_exclusive_group(required=True)
    mode.add_argument("--staged", action="store_true")
    mode.add_argument("--history", action="store_true")
    args = parser.parse_args()
    repo = subprocess.run(["git", "rev-parse", "--show-toplevel"],
                          capture_output=True, text=True, check=False)
    if repo.returncode:
        print("Secret scan requires a Git worktree.", file=sys.stderr)
        return 2
    return scan(Path(repo.stdout.rstrip("\n")), history=args.history)


if __name__ == "__main__":
    sys.exit(main())
