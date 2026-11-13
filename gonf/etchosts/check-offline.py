#!/usr/bin/env python3
"""Check the generated fleet files against the inventory and the live view.

Runs etchosts/gen into a temporary directory and requires byte equality with
the committed frontends/etc/hosts-fleet.inc and fleet-lan.inc (a stale
go generate fails), then asserts the fleet facts the old view checks covered:
no retired .recovery/10.98 names, .old only where the old address is still
held, the upstream router view, the held AP, the JetKVM leases and ULA only
under explicit .lan6 names. Also checks that gen refuses missing arguments.
"""
from pathlib import Path
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[2]
GONF = ROOT / 'gonf'
HOSTS = ROOT / 'frontends/etc/hosts-fleet.inc'
NSD = ROOT / 'frontends/var/nsd/zones/master/fleet-lan.inc'

with tempfile.TemporaryDirectory(prefix='etchosts-check-') as directory:
    hosts, nsd = Path(directory) / 'hosts', Path(directory) / 'nsd'
    subprocess.run(['go', 'run', './etchosts/gen', '-hosts', str(hosts), '-nsd', str(nsd)], cwd=GONF, check=True)
    assert hosts.read_bytes() == HOSTS.read_bytes(), 'hosts-fleet.inc stale: run go generate ./etchosts'
    assert nsd.read_bytes() == NSD.read_bytes(), 'fleet-lan.inc stale: run go generate ./etchosts'

for args in ([], ['-hosts', 'x'], ['-nsd', 'x'], ['-hosts', 'x', '-nsd', 'y', 'extra']):
    p = subprocess.run(['go', 'run', './etchosts/gen', *args], cwd=GONF, capture_output=True)
    assert p.returncode != 0, args

rows = {}
for line in HOSTS.read_text().splitlines():
    fields = line.split()
    for name in fields[1:]:
        if '.wg0' in name:
            continue  # wg0 names carry one IPv4 and one IPv6 row each
        assert name not in rows, 'duplicate ' + name
        rows[name] = fields[0]
records = [line.split() for line in NSD.read_text().splitlines()]
fqdns = {r[0].rstrip('.'): r[4] for r in records}

assert not any('.recovery.' in n or n.endswith('.recovery.lan') for n in rows)
assert not any(ip.startswith('10.98.') for ip in rows.values())
old = sorted(n.split('.')[0] for n in fqdns if '.old.' in n)
assert old == ['freebsd', 'openwrt', 'triggerfish'], old
assert rows['triggerfish'] == '192.168.1.2' and rows['triggerfish.lab.lan'] == '10.20.0.1'
assert rows['f0'] == '10.20.0.130' and rows['pi3'] == '10.20.0.128' and rows['f3s-storage-ha'] == '10.20.0.138'
assert rows['shelly1'] == '10.30.0.28' and rows['zen'] == '192.168.1.13'
assert rows['openwrt'] == '192.168.1.101' and rows['openwrt.new.lan'] == '10.30.0.2'
assert rows['jetkvm-46793b0d4c8d45d'] == '10.20.0.151' and rows['jetkvm-cee6ab4df2f88754'] == '10.20.0.198'
assert rows['pi0.lan6'] == 'fd02:ca0b:f1cb:20::125'
assert all(':' not in ip for n, ip in rows.items() if '.lan6' not in n), 'ULA outside .lan6'
assert all(n.endswith('.buetow.org') for n in fqdns) and not any(n.startswith('*') for n in fqdns)
assert all(r[1:4] in (['60', 'IN', 'A'], ['60', 'IN', 'AAAA']) for r in records)

print('PASS generated fleet files current; live view: no recovery names, .old only where held, router/AP/JetKVM/ULA rows')
