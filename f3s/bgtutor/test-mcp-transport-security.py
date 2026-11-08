#!/usr/bin/env python3
"""Read-only checks of the deployed MCP origin and content-type protections."""
import json
import os
import sys
import urllib.error
import urllib.parse
import urllib.request


def main():
    base = sys.argv[1] if len(sys.argv) > 1 else "https://bgtutor-mcp.f3s.buetow.org"
    token = os.environ.get("BGTUTOR_TOKEN")
    if not token:
        raise RuntimeError("BGTUTOR_TOKEN is required")
    origin = urllib.parse.urlsplit(base)
    same_origin = origin.scheme + "://" + origin.netloc
    cases = [
        (same_origin, "application/json", 200),
        ("https://untrusted-origin.invalid", "application/json", 403),
        (same_origin, "text/plain", 415),
    ]
    payload = json.dumps({"jsonrpc": "2.0", "id": 1, "method": "tools/list"}).encode()
    for request_origin, content_type, expected in cases:
        request = urllib.request.Request(
            base.rstrip("/") + "/mcp",
            data=payload,
            headers={
                "Authorization": "Bearer " + token,
                "Origin": request_origin,
                "Content-Type": content_type,
                "Accept": "application/json, text/event-stream",
            },
        )
        try:
            response = urllib.request.urlopen(request, timeout=20)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            if response.status != expected:
                raise RuntimeError(f"MCP transport check expected {expected}, got {response.status}")
    print("PASS same-origin tools/list, cross-origin rejection and content-type enforcement")


if __name__ == "__main__":
    try:
        main()
    except (RuntimeError, urllib.error.URLError, ValueError) as error:
        # Never print authenticated requests, URLs or response bodies.
        print(f"FAIL {type(error).__name__}: MCP transport verification failed", file=sys.stderr)
        sys.exit(1)
