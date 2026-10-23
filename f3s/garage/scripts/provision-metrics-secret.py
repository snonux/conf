#!/usr/bin/env python3
"""Check/provision the external Prometheus credential from its exact vault entry.

Default is read-only. --apply creates only an absent Secret; it never rotates or
overwrites an existing credential. Secret bytes stay in process memory/stdin.
"""
import argparse
import base64
import hmac
import json
import subprocess
import sys

REFERENCE = "Infra/garage-metrics"
NAMESPACE = "monitoring"
NAME = "garage-metrics"


def run(args, payload=None):
    try:
        result = subprocess.run(args, input=payload, capture_output=True, timeout=40)
    except (OSError, subprocess.TimeoutExpired) as exc:
        raise RuntimeError("credential operation unavailable or timed out") from exc
    if result.returncode:
        # Child errors may contain API response data. Do not repeat them.
        raise RuntimeError(f"credential operation failed (exit {result.returncode})")
    return result.stdout


def token_from_vault():
    token = run(["foostore", "read", "--field", "Password", "--exact", "--raw",
                 "--non-interactive", "--", REFERENCE]).rstrip(b"\r\n")
    if not token or any(c < 33 or c > 126 for c in token):
        raise RuntimeError("metrics token must be nonempty printable ASCII without whitespace")
    return token


def provision(apply=False):
    token = token_from_vault()
    output = run(["kubectl", "-n", NAMESPACE, "get", "secret", NAME,
                  "--ignore-not-found", "-o", "json"])
    if output.strip():
        try:
            obj = json.loads(output)
            if (obj.get("kind") != "Secret" or obj.get("type") != "Opaque" or
                    obj.get("metadata", {}).get("name") != NAME or
                    obj.get("metadata", {}).get("namespace") != NAMESPACE):
                raise ValueError("unexpected Secret identity or type")
            existing = base64.b64decode(obj["data"]["token"], validate=True)
        except (ValueError, KeyError, TypeError) as exc:
            raise RuntimeError("existing Secret has no usable token; refusing replacement") from exc
        if not hmac.compare_digest(existing, token):
            raise RuntimeError("existing Secret differs from the vault; refusing rotation")
        print("Metrics Secret matches the vault; unchanged.")
        return
    if not apply:
        raise RuntimeError("metrics Secret is absent; creation requires --apply and rollout approval")
    obj = {"apiVersion": "v1", "kind": "Secret", "type": "Opaque",
           "metadata": {"name": NAME, "namespace": NAMESPACE},
           "data": {"token": base64.b64encode(token).decode("ascii")}}
    run(["kubectl", "create", "-f", "-"], json.dumps(obj).encode())
    print("Metrics Secret created from the vault.")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--apply", action="store_true")
    args = parser.parse_args()
    try:
        provision(args.apply)
    except RuntimeError as exc:
        print(str(exc), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
