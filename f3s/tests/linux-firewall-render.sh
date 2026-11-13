#!/usr/bin/env bash
# Exercise the actual serialized-plan destination renderer without host writes.
set -euo pipefail
repo=$(cd -- "$(dirname -- "$0")/../.." && pwd)
python3 - "$repo" <<'PY'
import hashlib
import json
import pathlib
import subprocess
import sys
import tempfile

repo = pathlib.Path(sys.argv[1])
# Both groups select a template-free Rules file, so the rendering must equal
# that file's bytes. Hashing the asset (instead of a pinned digest, which went
# stale when staging switched to the migration files) keeps the oracle exact.
cases = [
    ('pihole_firewall_stage', 'gonf/pihole/assets/firewall/f3s-guard.nft', False),
    ('rnodes_firewall_stage', 'f3s/r-nodes/source-guard/f3s-guard.nft', True),
]
with tempfile.TemporaryDirectory(prefix='gonf-firewall-render-') as directory:
    for task, asset, wireguard in cases:
        expected = hashlib.sha256((repo / asset).read_bytes()).hexdigest()
        recorded = subprocess.run([str(repo / 'gonf.sh'), 'plan', '-stdout', task],
                                  check=True, capture_output=True, text=True)
        ops = [json.loads(line) for line in recorded.stdout.splitlines()]
        candidate = next(op for op in ops if op.get('path') == '/etc/f3s-firewall/candidate.nft')
        # Keep the real template bytes, serialized profile and renderer flags.
        # Change only destination/privilege metadata and omit native nft
        # validation, which the separate namespace packet suite exercises.
        destination = pathlib.Path(directory) / task
        candidate['path'] = str(destination)
        candidate['id'] = 'candidate'
        for key in ('owner', 'group', 'elevate', 'validation_bin', 'validation_args'):
            candidate.pop(key, None)
        plan = '\n'.join(json.dumps(op) for op in [ops[0], candidate]) + '\n'
        subprocess.run([str(repo / 'gonf.sh'), 'apply', '-'], input=plan,
                       check=True, capture_output=True, text=True)
        content = destination.read_bytes()
        actual = hashlib.sha256(content).hexdigest()
        assert actual == expected, (task, actual, expected)
        assert b'udp dport 0 return' not in content
        assert (b'udp dport 56709 return' in content) == wireguard
        print(task + ': actual serialized Gonf rendering PASS ' + actual)
PY
