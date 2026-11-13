#!/usr/bin/env python3
"""Run the real fleet hosts merger (assets/merge-hosts) against temporary files.

Checks idempotent apply, preservation of local rows and unrelated aliases,
and the refusals (unterminated marker block, empty or symlinked candidate).
It never touches the real /etc/hosts. Moved here from gonf/etchosts in task
f74 when that package's migration views were deleted.
"""
from pathlib import Path
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[2]

with tempfile.TemporaryDirectory(prefix='conf-hosts-test-') as directory:
    base = Path(directory)
    hosts, candidate, helper = (base / n for n in ('hosts', 'candidate', 'merge'))
    helper.write_text((ROOT / 'gonf/fleetfiles/assets/merge-hosts').read_text().replace('target=/etc/hosts', 'target=' + str(hosts)))
    helper.chmod(0o755)
    original = '# local\n127.0.0.1 localhost f0\n192.168.1.130 f0 unrelated\n2001:db8::1 public.example\n'
    hosts.write_text(original)
    hosts.chmod(0o644)
    candidate.write_text('10.20.0.130 f0\n192.168.1.130 f0.old.lan\n')
    def run(action, ok=True):
        p = subprocess.run([str(helper), action, str(candidate)], capture_output=True)
        assert (p.returncode == 0) == ok, (action, p.returncode, p.stderr)
    run('check', False)
    run('apply')
    run('check')
    saved = hosts.read_bytes()
    assert b'192.168.1.130 unrelated' in saved and b'2001:db8::1 public.example' in saved
    assert b'127.0.0.1 localhost\n' in saved and hosts.stat().st_mode & 0o777 == 0o644
    run('apply')
    assert hosts.read_bytes() == saved
    hosts.write_text(original + '# BEGIN GONF fleet\nmissing end\n')
    run('apply', False)
    assert hosts.read_text().endswith('missing end\n')
    hosts.write_text(original)
    candidate.write_text('')
    run('apply', False)
    assert hosts.read_text() == original
    candidate.unlink()
    candidate.symlink_to(hosts)
    run('apply', False)
    assert hosts.read_text() == original

print('PASS idempotent/conflict-preserving hosts merge and its refusals')
