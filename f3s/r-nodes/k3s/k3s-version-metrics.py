#!/usr/bin/python3
"""Report installed k3s versus release channels; never install or restart it."""

import argparse
import json
import os
from pathlib import Path
import re
import socket
import subprocess
import sys
import tempfile
import time
import urllib.request


CHANNELS_URL = "https://update.k3s.io/v1-release/channels"
OUTPUT = Path("/var/lib/node_exporter/textfile_collector/k3s_version.prom")
VERSION = re.compile(r"v([0-9]{1,3})\.([0-9]{1,3})\.([0-9]{1,4})\+k3s([0-9]{1,4})")
HELP = {
    "f3s_k3s_update_available": "A newer release exists in the installed minor channel.",
    "f3s_k3s_minor_update_available": "The stable channel is on a newer minor.",
    "f3s_k3s_version_info": "Installed and channel release identifiers.",
    "f3s_k3s_check_success": "Whether the most recent release check succeeded.",
    "f3s_k3s_check_timestamp_seconds": "Unix time of the most recent release check.",
}


def parse_version(value):
    match = VERSION.fullmatch(value)
    if match is None:
        raise ValueError("invalid release version")
    return tuple(int(part) for part in match.groups())


def release_metrics(installed, channels, node, now):
    current = parse_version(installed)
    minor = "v{}.{}".format(*current[:2])
    selected = {}
    for channel in channels["data"]:
        if channel["id"] in (minor, "stable"):
            if channel["id"] in selected:
                raise ValueError("duplicate channel")
            selected[channel["id"]] = channel["latest"]
    patch = parse_version(selected[minor])
    stable = parse_version(selected["stable"])
    if patch[:2] != current[:2]:
        raise ValueError("minor channel crossed a minor boundary")
    labels = 'node="{}"'.format(node)
    return [
        'f3s_k3s_update_available{{{}}} {}'.format(labels, int(patch > current)),
        'f3s_k3s_minor_update_available{{{}}} {}'.format(
            labels, int(stable[:2] > current[:2])),
        'f3s_k3s_version_info{{{},installed="{}",minor_target="{}",stable_target="{}"}} 1'.format(
            labels, installed, selected[minor], selected["stable"]),
        'f3s_k3s_check_success{{{}}} 1'.format(labels),
        'f3s_k3s_check_timestamp_seconds{{{}}} {}'.format(labels, now),
    ]


def collect(node, now, version_command=("/usr/local/bin/k3s", "--version")):
    result = subprocess.run(version_command,
                            capture_output=True, text=True, check=True, timeout=5)
    lines = result.stdout.splitlines()
    match = re.fullmatch(r"k3s version (\S+)(?: \([^\r\n]+\))?", lines[0])
    if match is None:
        raise ValueError("unrecognized installed version")
    # Ignore ambient proxy configuration: only this fixed HTTPS origin is read.
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    with opener.open(CHANNELS_URL, timeout=10) as response:
        data = response.read(1024 * 1024 + 1)
    if len(data) > 1024 * 1024:
        raise ValueError("oversized channel response")
    return release_metrics(match[1], json.loads(data), node, now)


def publish(output, text):
    temporary = None
    try:
        with tempfile.NamedTemporaryFile(mode="w", dir=output.parent,
                                         prefix=".k3s-version-", delete=False) as stream:
            temporary = Path(stream.name)
            stream.write(text)
            stream.flush()
            os.fchmod(stream.fileno(), 0o644)
        os.replace(temporary, output)
    finally:
        if temporary is not None:
            temporary.unlink(missing_ok=True)


def render(metrics):
    lines = []
    for metric in metrics:
        name = metric.partition("{")[0]
        lines.extend(["# HELP {} {}".format(name, HELP[name]),
                      "# TYPE {} gauge".format(name), metric])
    return "\n".join(lines) + "\n"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--stdout", action="store_true", help="read-only probe; write no file")
    args = parser.parse_args()
    node = socket.gethostname()
    if re.fullmatch(r"[a-z0-9][a-z0-9.-]{0,252}", node) is None:
        print("invalid node name", file=sys.stderr)
        return 1
    now = int(time.time())
    status = 0
    try:
        metrics = collect(node, now)
    except (OSError, ValueError, KeyError, TypeError, IndexError,
            subprocess.SubprocessError):
        metrics = ['f3s_k3s_check_success{{node="{}"}} 0'.format(node),
                   'f3s_k3s_check_timestamp_seconds{{node="{}"}} {}'.format(node, now)]
        print("k3s release-channel check failed", file=sys.stderr)
        status = 1
    text = render(metrics)
    if args.stdout:
        print(text, end="")
    else:
        publish(OUTPUT, text)
    return status


if __name__ == "__main__":
    sys.exit(main())
