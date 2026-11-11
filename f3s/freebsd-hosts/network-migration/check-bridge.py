#!/usr/bin/env python3
"""Offline fixture for bridge.sh (task 764): disposable command mocks, never SSH.

The mocks model the FreeBSD state the helper reads and changes: interface
addresses (with CARP rows), the nd6 ifdisabled/auto_linklocal flags, the
IPv4 default route, devd, and rc.conf/system.conf through a file-backed
sysrc. They refuse what FreeBSD would not do (unknown interfaces, an IPv6
address added to an ifdisabled interface, a CARP line with the wrong
password) so ordering mistakes fail the run. This is not a native FreeBSD
acceptance test; see BRIDGE.md for the live procedure.
"""
import json
import os
import re
import shlex
import subprocess
import sys
import tempfile
from pathlib import Path

HERE = Path(__file__).resolve().parent
PASS = 'Fixture1Secret'
MOCK = r'''#!/usr/bin/env python3
import json, os, re, shlex, sys
from pathlib import Path
db = Path(os.environ['FIXTURE_DB']); s = json.loads(db.read_text())
cmd = Path(sys.argv[0]).name; a = sys.argv[1:]; rc = 0
if cmd not in ('id', 'date', 'sync', 'pgrep', 'netstat'):
    s['calls'].append([cmd] + a)
fail = s.get('fail_once')
if fail and fail == ' '.join([cmd] + a):
    s['fail_once'] = None; db.write_text(json.dumps(s)); sys.exit(1)

def v4rows(i):
    for addr, vhid in i['v4']:
        line = '\tinet %s netmask %s broadcast x' % (addr, '0xffffffff' if vhid else '0xffffff00')
        print(line + (' vhid %d' % vhid if vhid else ''))

def v6rows(n, i):
    for addr in i['v6']:
        print('\tinet6 %s prefixlen 64%s' % (addr, ' scopeid 0x1' if addr.startswith('fe80') else ''))

def show(n, i):
    print('%s: flags=UP' % n); v4rows(i); v6rows(n, i)
    if i.get('carp'):
        print('\tcarp: %s vhid %d advbase 1 advskew %d' % (i['carp']['state'], i['carp']['vhid'], i['carp']['skew']))
    for m in i.get('members', []):
        print('\tmember: %s flags=143<LEARNING>' % m)
    print('\tnd6 options=%s' % ('IFDISABLED' if i['ifdisabled'] else 'PERFORMNUD'))

def same24(x, y): return x.rsplit('.', 1)[0] == y.rsplit('.', 1)[0]

def ifconfig(a):
    if a[0] not in s['ifs']: return 1
    n, i = a[0], s['ifs'][a[0]]
    # devd would run carpcontrol.sh on any vhid change (stopping NFS).
    assert not (s['devd'] and ('vhid' in a or (len(a) > 2 and a[2] == s.get('vip')))), 'CARP change while devd runs'
    if len(a) == 1: show(n, i); return 0
    if a[1:] == ['inet']: v4rows(i); return 0
    if a[1:] == ['inet6']: v6rows(n, i); return 0
    if a[1] == 'vhid':
        assert a[3:] == ['state', 'master'] and i['carp'], a
        i['carp']['state'] = 'MASTER'; return 0
    if a[1] == 'inet' and a[2] == 'vhid':
        args = a[2:]; vip = args[args.index('alias') + 1]
        assert args[args.index('pass') + 1] == s['pass'], 'wrong CARP password'
        assert vip.endswith('/32') and not i.get('carp'), a
        skew = int(args[args.index('advskew') + 1]) if 'advskew' in args else 0
        i['v4'].append([vip[:-3], int(args[1])])
        i['carp'] = {'vhid': int(args[1]), 'state': 'BACKUP', 'skew': skew}
        return 0
    if a[1] == 'inet' and a[-1] == '-alias':
        before = len(i['v4']); i['v4'] = [r for r in i['v4'] if r[0] != a[2]]
        assert len(i['v4']) == before - 1, a
        if not any(r[1] for r in i['v4']): i['carp'] = None
        d = s['default4']
        if d and d['if'] == n and not any(same24(r[0], d['gw']) and not r[1] for r in i['v4']):
            s['default4'] = None  # FreeBSD drops routes through a detached subnet
        return 0
    if a[1] == 'inet' and a[-1] == 'alias' and len(a) == 4:
        i['v4'].append([a[2].split('/')[0], None]); return 0
    if a[1] == 'inet6' and a[-1] == '-alias':
        assert a[2] in i['v6'], a
        i['v6'].remove(a[2]); return 0
    if a[1] == 'inet6' and a[-1] == 'alias':
        assert not i['ifdisabled'], 'inet6 address added while ifdisabled'
        i['v6'].append(a[2]); return 0
    if a[1] == 'inet6':
        for flag in a[2:]:
            if flag in ('ifdisabled', '-ifdisabled'): i['ifdisabled'] = flag == 'ifdisabled'
            elif flag in ('auto_linklocal', '-auto_linklocal'): i['auto_ll'] = flag == 'auto_linklocal'
            elif flag == '-accept_rtadv': i['rtadv'] = False
            else: raise AssertionError(a)
        if not i['ifdisabled'] and i['auto_ll'] and not any(x.startswith('fe80') for x in i['v6']):
            i['v6'].insert(0, 'fe80::1%' + n)
        return 0
    raise AssertionError(a)

def route(a):
    d = s['default4']
    if a == ['-n', 'get', '-inet6', 'default']: return 0 if s['route6'] else 1
    if a[:4] == ['-n', 'get', '-inet', 'default']:
        if not d: return 1
        print('   route to: default\ndestination: default\n    gateway: %s\n  interface: %s' % (d['gw'], d['if']))
        return 0
    if a[0] in ('add', 'change') and a[1] == 'default':
        if (a[0] == 'add') == bool(d): return 1
        owner = [n for n, i in s['ifs'].items() if any(same24(r[0], a[2]) and not r[1] for r in i['v4'])]
        if not owner: return 1
        s['default4'] = {'gw': a[2], 'if': owner[0]}; return 0
    raise AssertionError(a)

def sysrc(a):
    assert a[0] == '-f', a
    path = Path(a[1]); a = a[2:]
    lines = path.read_text().splitlines() if path.exists() else []
    def find(k): return [l for l in lines if re.match(re.escape(k) + '=', l)]
    if a[0] == '-n':
        hit = find(a[1])
        if not hit:
            # Real sysrc also answers from /etc/defaults/rc.conf.
            if a[1] in ('cloned_interfaces', 'static_routes', 'ipv6_static_routes'): print(''); return 0
            return 1
        print(shlex.split(hit[-1].split('=', 1)[1])[0] if hit[-1].split('=', 1)[1] else ''); return 0
    if a[0] == '-x':
        if not find(a[1]): return 1
        lines = [l for l in lines if not re.match(re.escape(a[1]) + '=', l)]
    else:
        for item in a:
            k, v = item.split('=', 1); new = '%s="%s"' % (k, v)
            if find(k): lines = [new if re.match(re.escape(k) + '=', l) else l for l in lines]
            else: lines.append(new)
    path.write_text('\n'.join(lines) + '\n'); return 0

if cmd == 'id': print('0')
elif cmd == 'lockf':
    a = [x for x in a if x != '-s']
    os.execv(a[4], a[4:])
elif cmd == 'date': print(s['now'])
elif cmd == 'sync': pass
elif cmd == 'netstat': print('fixture routes')
elif cmd == 'pgrep': rc = 0 if a == ['-x', 'devd'] and s['devd'] else 1
elif cmd == 'service':
    assert a[0] == 'devd' and a[1] in ('start', 'stop'), a
    s['devd'] = a[1] == 'start'
elif cmd == 'pfctl':
    if a == ['-s', 'Running']: rc = 0 if s['pf'] else 1
    elif a == ['-s', 'rules']: print(s['pf_rules'])
    else: raise AssertionError(a)
elif cmd == 'ifconfig': rc = ifconfig(a)
elif cmd == 'route': rc = route(a)
elif cmd == 'sysrc': rc = sysrc(a)
else: raise AssertionError((cmd, a))
db.write_text(json.dumps(s)); sys.exit(rc)
'''

N = 130
LAB, ULA, VIP = f'10.20.0.{N}', f'fd02:ca0b:f1cb:20::{N}', '10.20.0.138'
RC = (f'hostname="f0.lan.buetow.org"\nifconfig_re0="inet {LAB}/24"\n'
      f'ifconfig_re0_ipv6="inet6 {ULA} prefixlen 64 -accept_rtadv"\n'
      'defaultrouter="10.20.0.1"\nvm_enable="YES"\n')
CARP_LINE = f'inet vhid 1 pass {PASS} alias {VIP}/32'
VM = 'switch_list="public"\ntype_public="standard"\nports_public="re0"\n'


def initial(carp=None):
    """carp: None, 'MASTER' or 'BACKUP' (f0-like or f1-like)."""
    re0 = {'v4': [[LAB, None]], 'v6': ['fe80::1%re0', ULA], 'ifdisabled': False, 'auto_ll': True}
    if carp:
        re0['v4'].append([VIP, 1])
        re0['carp'] = {'vhid': 1, 'state': carp, 'skew': 0 if carp == 'MASTER' else 100}
    bridge = {'v4': [], 'v6': [], 'ifdisabled': True, 'auto_ll': False, 'members': ['tap0', 're0']}
    return {'ifs': {'re0': re0, 'vm-public': bridge}, 'default4': {'gw': '10.20.0.1', 'if': 're0'},
            'devd': not carp, 'now': 1000, 'calls': [], 'pass': PASS, 'vip': VIP, 'pf': True,
            'pf_rules': 'block drop in log all\npass in quick on re0 all\npass in quick on vm-public all',
            'route6': False}


class Fixture:
    def __init__(self, tmp, carp=None):
        self.tmp, self.state = tmp, tmp / 'state'
        self.bindir, self.db = tmp / 'bin', tmp / 'db.json'
        self.rc, self.vm = tmp / 'rc.conf', tmp / 'system.conf'
        self.rclocal, self.old = tmp / 'rc.conf.local', tmp / 'old-migration'
        self.devd = tmp / 'devd-carp.conf'
        for d in (self.state, self.bindir, self.old):
            d.mkdir(exist_ok=True)
        for c in ('id', 'lockf', 'date', 'sync', 'netstat', 'pgrep', 'service', 'ifconfig', 'route', 'sysrc', 'pfctl'):
            (self.bindir / c).write_text(MOCK)
            (self.bindir / c).chmod(0o755)
        conf = tmp / 'bridge.conf'
        conf.write_text(f"member='re0'\nbridge='vm-public'\nswitch='public'\nlab='{LAB}'\n"
                        f"ula='{ULA}'\ngateway='10.20.0.1'\n")
        src = (HERE / 'bridge.sh').read_text()
        for old, new in (('PATH=/sbin:/bin:/usr/sbin:/usr/bin:/usr/local/sbin:/usr/local/bin', f'PATH={self.bindir}:/usr/bin:/bin'),
                         ('/var/db/gonf-bridge-migration', str(self.state)),
                         ('/var/db/gonf-network-migration', str(self.old)),
                         ('/usr/local/sbin/gonf-bridge-migration', str(tmp / 'helper')),
                         ('/usr/local/etc/gonf-bridge-migration.conf', str(conf)),
                         ('/zroot/bhyve/.config/system.conf', str(self.vm)),
                         ('/etc/rc.conf.local', str(self.rclocal)),
                         ('/usr/local/etc/devd/carp.conf', str(self.devd)),
                         ('rcconf=/etc/rc.conf', f'rcconf={self.rc}')):
            assert old in src, old
            src = src.replace(old, new)
        self.helper = tmp / 'helper'
        self.helper.write_text(src)
        self.helper.chmod(0o755)
        self.reset(carp)

    def reset(self, carp=None, **changes):
        for p in self.state.iterdir():
            subprocess.run(['rm', '-rf', str(p)], check=True)
        seed = initial(carp)
        seed.update(changes)
        self.db.write_text(json.dumps(seed))
        self.rc.write_text(RC + (f'ifconfig_re0_alias0="{CARP_LINE}"\n' if carp else ''))
        self.vm.write_text(VM)
        # The real asset: the preflight must accept what gonf deploys.
        self.devd.write_text((HERE.parent / 'carp/devd-carp.conf').read_text())
        self.seed = seed
        return seed

    def run(self, *args, ok=True, why=None):
        """Run the helper; a refusal (ok=False) must name its reason (why)."""
        env = dict(os.environ, FIXTURE_DB=str(self.db))
        r = subprocess.run([str(self.helper), *args], env=env, capture_output=True, text=True)
        assert PASS not in r.stdout + r.stderr, 'CARP password printed'
        assert (r.returncode == 0) == ok, (args, r.returncode, r.stdout, r.stderr)
        assert 'Traceback' not in r.stderr, r.stderr
        if not ok:
            assert why and why in r.stderr, (args, why, r.stderr)
        return r

    def s(self):
        return json.loads(self.db.read_text())

    def set(self, **changes):
        s = self.s()
        s.update(changes)
        self.db.write_text(json.dumps(s))

    def gate(self, name='move-approved'):
        (self.state / name).touch()

    def keys(self, path):
        return dict(l.split('=', 1) for l in path.read_text().splitlines() if '=' in l and not l.startswith('unset'))


def norm(s):
    """Interface state for comparisons: address order is irrelevant except the primary."""
    out = {}
    for n, i in s['ifs'].items():
        plain = [r[0] for r in i['v4'] if not r[1]]
        out[n] = (plain[:1], sorted(map(str, i['v4'])), sorted(i['v6']), i['ifdisabled'],
                  i['auto_ll'], (i.get('carp') or {}).get('state'))
    return out, s['default4']


def assert_moved(f, carp=None):
    s = f.s()
    br, re0 = s['ifs']['vm-public'], s['ifs']['re0']
    assert [r[0] for r in br['v4'] if not r[1]] == [LAB], br
    assert ULA in br['v6'] and any(x.startswith('fe80') for x in br['v6']) and not br['ifdisabled'], br
    assert re0['v4'] == [] and re0['v6'] == [] and re0['ifdisabled'], re0
    assert s['default4'] == {'gw': '10.20.0.1', 'if': 'vm-public'}, s['default4']
    k = f.keys(f.rc)
    assert k['ifconfig_vm_public'] == f'"inet {LAB}/24 addm re0 up"', k
    assert k['ifconfig_vm_public_ipv6'] == f'"inet6 {ULA} prefixlen 64 auto_linklocal -accept_rtadv"', k
    assert k['cloned_interfaces'] == '"bridge0"' and k['ifconfig_bridge0_name'] == '"vm-public"', k
    assert k['ifconfig_re0'] == '"up"' and 'ifconfig_re0_ipv6' not in k, k
    v = f.keys(f.vm)
    assert v.get('type_public') == '"manual"' and v.get('bridge_public') == '"vm-public"' and 'ports_public' not in v, v
    if carp:
        assert br['carp']['state'] == carp and [VIP, 1] in br['v4'], br
        assert k['ifconfig_vm_public_alias0'] == f'"{CARP_LINE}"' and 'ifconfig_re0_alias0' not in k, k
    else:
        assert 'ifconfig_vm_public_alias0' not in k, k


def assert_restored(f):
    assert norm(f.s()) == norm(f.seed), (norm(f.s()), norm(f.seed))
    assert f.keys(f.rc) == f.keys_initial_rc, f.keys(f.rc)
    assert f.keys(f.vm) == f.keys_initial_vm, f.keys(f.vm)
    assert not (f.state / 'pending').exists()


def check_plain(f):
    f.keys_initial_rc, f.keys_initial_vm = f.keys(f.rc), f.keys(f.vm)
    f.run('move', ok=False, why='move gate missing')  # gate closed
    assert not (f.state / 'pending').exists() and norm(f.s()) == norm(f.seed)
    for bad in ('50', '4000', 'abc'):
        f.gate(); f.run('move', bad, ok=False, why='deadline')
        assert not (f.state / 'pending').exists()
    f.gate(); f.run('move', '900')
    assert_moved(f)
    assert not (f.state / 'move-approved').exists(), 'gate is single-use'
    f.gate(); f.run('move', ok=False, why='already pending')
    (f.state / 'move-approved').unlink()  # a refused move leaves the gate in place
    f.run('rollback')
    assert_restored(f)
    f.run('rollback')  # idempotent without a journal
    print('PASS: plain host move, refused gate/deadline/second move, runtime rollback')

    f.run('move', ok=False, why='move gate missing')  # gate was consumed and is not recreated
    f.gate(); f.run('move', '600')
    f.set(now=1500); f.run('watchdog')
    assert (f.state / 'pending').exists(), 'watchdog fired early'
    f.set(now=1600); f.run('watchdog')
    assert_restored(f)
    print('PASS: watchdog restores at the deadline, not before')

    f.set(now=1000); f.gate(); f.run('move')
    f.run('confirm')
    assert_moved(f)
    assert not (f.state / 'pending').exists() and list(f.state.glob('accepted-*'))
    f.run('confirm', ok=False, why='no pending transaction')
    f.set(now=99999); f.run('watchdog'); assert_moved(f)
    print('PASS: confirm accepts the move; later watchdog runs leave it alone')


def cached_rc_vars(f, moved_rc):
    """Source the moved rc.conf, then the restored one, in one shell (rc's ALRM reload)."""
    script = ('. "$1"; . "$2"; for v in cloned_interfaces ifconfig_bridge0_name ifconfig_vm_public '
              'ifconfig_vm_public_ipv6 ifconfig_re0 ifconfig_re0_ipv6; do eval "printf \'%s=%s\\n\' $v \\"\\${$v-UNSET}\\""; done')
    out = subprocess.run(['sh', '-c', script, 'sh', str(moved_rc), str(f.rc)], capture_output=True, text=True, check=True).stdout
    return dict(l.split('=', 1) for l in out.splitlines())


def check_boot(f):
    f.reset(); f.gate(); f.run('move')
    moved = f.tmp / 'moved.rc'; moved.write_text(f.rc.read_text())
    f.run('boot')
    v = cached_rc_vars(f, moved)
    assert v['cloned_interfaces'] == 'UNSET' and v['ifconfig_vm_public'] == 'UNSET', v
    assert v['ifconfig_re0'] == f'inet {LAB}/24' and v['ifconfig_re0_ipv6'].startswith('inet6 fd02'), v
    assert f.keys(f.vm) == f.keys_initial_vm and not (f.state / 'pending').exists()
    assert 'gonf-bridge-migration absent keys' in f.rc.read_text()
    f.run('watchdog')
    assert 'absent keys' not in f.rc.read_text() and f.keys(f.rc) == f.keys_initial_rc
    print('PASS: boot restore before netif clears cached moved variables; watchdog drops the unset line')

    f.reset(); f.run('boot-trial', ok=False, why='no pending transaction')  # nothing pending
    f.gate(); f.run('move', '600')
    for bad in ('60', '9000'):
        f.run('boot-trial', bad, ok=False, why='boot-trial window')
    f.run('boot-trial', '1200')
    f.run('boot')
    assert (f.state / 'pending').exists() and not (f.state / 'pending/boot-once').exists()
    assert f.keys(f.rc)['ifconfig_vm_public'].startswith('"inet'), 'trial boot must keep the move'
    f.run('boot')  # a second unconfirmed boot restores
    assert not (f.state / 'pending').exists() and f.keys(f.rc) == f.keys_initial_rc
    f.reset(); f.gate(); f.run('move', '600'); f.run('boot-trial', '600')
    f.set(now=1700); f.run('boot')  # trial window over before the boot
    assert not (f.state / 'pending').exists() and f.keys(f.rc) == f.keys_initial_rc
    f.reset(); f.gate(); f.run('move', '600'); f.run('boot-trial', '600')
    f.run('confirm')
    assert not (f.state / 'pending').exists() and f.keys(f.rc)['cloned_interfaces'] == '"bridge0"'
    print('PASS: boot-trial keeps exactly one boot within its window; confirm ends it')


def check_refusals(f):
    def refused(why, **changes):
        f.reset(**changes)
        f.gate()
        r = f.run('move', ok=False, why=why)
        assert not (f.state / 'pending').exists(), why
        assert norm(f.s()) == norm(f.seed), why
        return r
    f.reset(); f.rc.write_text(f.rc.read_text() + 'ifconfig_re0_alias1="inet 10.20.0.99/32"\n'); f.gate()
    f.run('move', ok=False, why='unowned ifconfig_re0_alias')
    f.reset(); f.rc.write_text(f.rc.read_text() + 'ifconfig_vm_public="up"\n'); f.gate(); f.run('move', ok=False, why='bridge keys already present')
    f.reset(); f.rc.write_text(f.rc.read_text() + 'cloned_interfaces="lagg0 bridge0"\n'); f.gate(); f.run('move', ok=False, why='cloned_interfaces already lists bridge0')
    f.reset(); f.rc.write_text(f.rc.read_text().replace('-accept_rtadv', 'accept_rtadv')); f.gate(); f.run('move', ok=False, why='saved ifconfig_re0_ipv6')
    f.reset(); f.rc.write_text(f.rc.read_text().replace('defaultrouter="10.20.0.1"', 'defaultrouter="10.20.0.254"'))
    f.gate(); f.run('move', ok=False, why='saved defaultrouter')
    f.reset(); f.vm.write_text(VM.replace('standard', 'manual')); f.gate(); f.run('move', ok=False, why='not a standard vm-bhyve switch')
    f.reset(); f.vm.write_text(VM.replace('ports_public="re0"', 'ports_public="em0"')); f.gate(); f.run('move', ok=False, why='does not own re0')
    f.reset(); f.rclocal.write_text('x=1\n'); f.gate(); f.run('move', ok=False, why='rc.conf.local present'); f.rclocal.unlink()
    f.reset(); (f.old / 'pending').mkdir(); f.gate(); f.run('move', ok=False, why='address-migration transaction is active'); (f.old / 'pending').rmdir()
    seed = initial(); seed['ifs']['vm-public']['v4'] = [['10.20.0.77', None]]
    refused('vm-public already has addresses', ifs=seed['ifs'])
    seed = initial(); seed['ifs']['re0']['v6'] = ['fe80::1%re0']
    refused('re0 must carry exactly the lab ULA', ifs=seed['ifs'])
    seed = initial(); seed['ifs']['re0']['v4'].append(['10.20.0.200', None])
    refused('re0 must carry exactly the lab IPv4', ifs=seed['ifs'])
    seed = initial(); seed['ifs']['vm-public']['members'] = ['tap0']
    refused('re0 is not a member of vm-public', ifs=seed['ifs'])
    refused('default route is not the lab gateway', default4={'gw': '10.20.0.254', 'if': 're0'})
    refused('an IPv6 default route needs review', route6=True)
    refused('does not cover vm-public', pf_rules='pass in quick on re0 all')
    f.reset(pf=False, pf_rules=''); f.gate(); f.run('move')  # PF disabled: nothing to check
    f.reset(); f.rc.write_text(f.rc.read_text() + 'static_routes="lan"\n'); f.gate()
    f.run('move', ok=False, why='saved static routes need review')
    f.reset(); f.rc.write_text(f.rc.read_text().replace('defaultrouter="10.20.0.1"', 'defaultrouter="10.20.0.1"\nipv6_defaultrouter="fd02::1"'))
    f.gate(); f.run('move', ok=False, why='ipv6_defaultrouter')
    print('PASS: preflight refusals leave saved and running state untouched')


def check_carp(f):
    f.reset('MASTER')
    f.keys_initial_rc, f.keys_initial_vm = f.keys(f.rc), f.keys(f.vm)
    f.set(devd=True); f.gate(); f.gate('carp-master-approved')
    f.run('move', ok=False, why='stop devd first')  # devd still running
    f.set(devd=False); (f.state / 'carp-master-approved').unlink()
    f.run('move', ok=False, why='carp-master-approved')  # MASTER without its gate
    f.gate('carp-master-approved'); f.run('move')
    assert_moved(f, 'MASTER')
    assert not (f.state / 'carp-master-approved').exists()
    f.run('confirm', ok=False, why='devd is stopped')  # devd must run again first
    f.set(devd=True); f.run('confirm'); assert_moved(f, 'MASTER')
    print('PASS: CARP MASTER needs stopped devd and its gate, keeps MASTER on the bridge, confirm needs devd')

    f.reset('MASTER'); f.gate(); f.gate('carp-master-approved')
    f.devd.write_text(f.devd.read_text().replace('[0-9a-z.-]+', '[0-9a-z.]+'))
    f.run('move', ok=False, why='deploy freebsd_carp_devd_hook first')

    f.reset('MASTER'); f.gate(); f.gate('carp-master-approved'); f.run('move')
    f.set(devd=True)  # the procedure restarts devd after the move
    f.run('rollback')  # the mock refuses any vhid change while devd runs
    assert_restored(f)
    assert f.s()['devd'], 'rollback must start devd again'
    archived = '\n'.join(x.read_text() for x in f.state.glob('*/snapshot/**/*') if x.is_file())
    assert PASS not in archived, 'archived journal keeps the CARP password'
    print('PASS: CARP rollback (devd stopped around it) restores the VIP and MASTER on re0 and restarts devd')

    f.reset('MASTER'); f.gate(); f.gate('carp-master-approved')
    f.set(fail_once='ifconfig vm-public inet6 -ifdisabled auto_linklocal -accept_rtadv')
    f.run('move', ok=False, why='runtime move failed')
    assert not any(r[1] for r in f.s()['ifs']['re0']['v4']), 'VIP should be gone mid-move'
    f.run('rollback')
    assert_restored(f)
    print('PASS: a move failing after the VIP left re0 is restored as MASTER')

    f.reset('BACKUP')
    f.keys_initial_rc, f.keys_initial_vm = f.keys(f.rc), f.keys(f.vm)
    f.gate(); f.run('move')
    assert_moved(f, 'BACKUP')
    f.set(now=5000); f.run('watchdog')
    assert_restored(f)
    print('PASS: CARP BACKUP moves without the master gate; watchdog restores it')


def check_partial(f):
    f.reset()
    f.set(fail_once=f'ifconfig vm-public inet6 {ULA} prefixlen 64 alias')
    f.gate(); f.run('move', ok=False, why='runtime move failed')
    assert (f.state / 'pending').exists(), 'a failed move keeps its journal'
    f.run('confirm', ok=False, why='lacks the lab ULA')  # incomplete move is never accepted
    f.run('rollback')
    assert_restored(f)
    f.reset(); f.gate(); f.run('move')
    f.set(fail_once=f'ifconfig re0 inet {LAB}/24 alias'); f.run('rollback', ok=False, why='runtime restore failed')
    assert (f.state / 'pending').exists(), 'a failed rollback keeps the journal for retry'
    f.run('rollback')
    assert_restored(f)
    print('PASS: a failed runtime step leaves the journal pending; rollback retries until restored')


def main():
    with tempfile.TemporaryDirectory() as d:
        tmp = Path(d)
        f = Fixture(tmp)
        f.keys_initial_rc, f.keys_initial_vm = f.keys(f.rc), f.keys(f.vm)
        check_plain(f)
        check_boot(f)
        check_refusals(f)
        check_partial(f)
        check_carp(f)
    for script in ('bridge.sh', 'rc-bridge-rollback'):
        subprocess.run(['sh', '-n', str(HERE / script)], check=True)
    print('PASS: all bridge-migration fixture checks')


if __name__ == '__main__':
    sys.exit(main())
