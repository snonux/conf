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
# h24: the JetKVM hold is lifted; canonical names follow the reserved lease.
# 274: the former ISP lease is no longer theirs, so accepted has no .old alias.
for dev, new_ip, old_ip in (('46793b0d4c8d45d', '10.20.0.151', '192.168.1.248'),
                            ('e10a0883debf7d4a', '10.20.0.158', '192.168.1.241'),
                            ('f72978d7f87aa969', '10.20.0.191', '192.168.1.244'),
                            ('cee6ab4df2f88754', '10.20.0.198', '192.168.1.245')):
    assert rows['jetkvm-' + dev] == new_ip and 'jetkvm-' + dev + '.old.lan' not in rows
assert rows['jetkvm-46793b0d4c8d45d.new.lan'] == '10.20.0.151'
assert rows['f0.recovery.lan'] == '10.98.0.130'
# 274: i24 retired the old 192.168.1.x of these endpoints; accepted drops their
# .old names (hosts and DNS). Router, AP and the stopped freebsd VM still hold
# theirs, and the historical staging preview keeps every .old alias.
for host in ('f0', 'f1', 'f2', 'f3', 'f3s-storage-ha', 'r0', 'r1', 'r2', 'rocky',
             'pi0', 'pi1', 'pi2', 'pi3', 'shelly1', 'shelly2'):
    assert host + '.old.lan' not in rows and host + '.old.lan.buetow.org' not in rows, host
pihole_old = sorted(r.split('=')[1].split('.')[0] for r in render('accepted', 'lab')['PiHoleDNSMasq'] if '.old.' in r)
assert pihole_old == ['freebsd', 'openwrt', 'triggerfish'], pihole_old
assert rows['triggerfish.old.lan'] == '192.168.1.2' and rows['openwrt.old.lan'] == '192.168.1.101'
assert rows['freebsd.old.lan'] == '192.168.1.139'
staged = {name: line.split()[0] for line in staging['HostsBlock'] for name in line.split()[1:]}
assert staged['f0.old.lan'] == '192.168.1.130' and staged['jetkvm-46793b0d4c8d45d.old.lan'] == '192.168.1.248'
for profile, view in (('target', 'lab'), ('ap-target', 'wifi')):
    preview = render(profile, view)['HostsBlock']
    assert '192.168.1.130 f0.old.lan f0.old.lan.buetow.org' in preview, profile
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
