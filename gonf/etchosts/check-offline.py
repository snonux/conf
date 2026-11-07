#!/usr/bin/env python3
"""Check real renderer output and the native hosts merger, including failures."""
import json
from pathlib import Path
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[2]


def render(profile, view='upstream'):
    return json.loads(subprocess.check_output(
        ['go', 'run', './cmd/hosts-migration', '-profile', profile, '-router-view', view], cwd=ROOT / 'gonf'))


staging = render('staging')
accepted = render('accepted')
target = render('target')
rows = {name: line.split()[0] for line in accepted['HostsBlock'] for name in line.split()[1:]}
assert rows['f0'] == '10.20.0.130' and rows['pi3'] == '10.20.0.128'
assert rows['shelly1'] == '10.30.0.28' and rows['freebsd'] == '10.20.0.139'
assert rows['openwrt'] == '192.168.1.101'
assert rows['jetkvm-46793b0d4c8d45d'] == '192.168.1.248'
assert rows['jetkvm-46793b0d4c8d45d.new.lan'] == '10.20.0.151'
assert rows['f0.old.lan'] == '192.168.1.130' and rows['f0.recovery.lan'] == '10.98.0.130'
assert rows['triggerfish'] == '192.168.1.2'
assert '10.20.0.1 triggerfish ' in '\n'.join(render('accepted', 'lab')['HostsBlock'])
assert staging['WireGuardBlock'] == accepted['WireGuardBlock'] == target['WireGuardBlock']
assert all(':' not in rows[name] for name in ('f0', 'pi0', 'r0', 'f0.lan.buetow.org'))
assert rows['pi0.lan6'] == 'fd02:ca0b:f1cb:20::125'
for args in (['-profile', 'unknown'], ['-profile', 'staging', '-router-view', 'lab'], ['-format', 'bad']):
    p = subprocess.run(['go', 'run', './cmd/hosts-migration', *args], cwd=ROOT / 'gonf', capture_output=True)
    assert p.returncode and not p.stdout

for fmt, asset in [('authoritative-hosts', 'frontends/var/nsd/zones/master/accepted-lan.inc'),
                   ('hosts', 'frontends/etc/hosts-fleet.inc')]:
    actual = subprocess.check_output(['go', 'run', './cmd/hosts-migration', '-profile', 'accepted',
                                     '-format', fmt], cwd=ROOT / 'gonf')
    assert actual == (ROOT / asset).read_bytes(), 'generated source stale'
    if fmt == 'authoritative-hosts':
        assert b'*.f3s.lan.' not in actual and b' SOA ' not in actual

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

print('PASS accepted/current DHCP views, unchanged mesh, explicit-only ULA, generated records and real idempotent/conflict-preserving hosts merge')
