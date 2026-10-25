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
cases = [
    ('pihole_firewall_stage', '4276353bd7ef3e92aeaa46975b89d235f0ac5756426566f022f9ad9e90b42df1', False),
    ('rnodes_firewall_stage', '794a9c28c4bbfa174eaa7f0edb77e958568e490f701ffc08eede45641ea435e7', True),
]
with tempfile.TemporaryDirectory(prefix='gonf-firewall-render-') as directory:
    for task, expected, wireguard in cases:
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
