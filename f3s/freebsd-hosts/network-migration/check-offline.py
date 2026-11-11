#!/usr/bin/env python3
"""Exercise the shell transaction with disposable command fixtures, never SSH."""
import hashlib
import json
import os
import shlex
from pathlib import Path
import subprocess
import sys
import tempfile

HERE = Path(__file__).resolve().parent
MOCK = r'''#!/usr/bin/env python3
import hashlib, json, os, re, shlex, subprocess, sys
from pathlib import Path
p=Path(os.environ['FIXTURE_DB'])
s=json.loads(p.read_text())
a=sys.argv[1:]; cmd=Path(sys.argv[0]).name
# Pure observations neither mutate fixture state nor contribute to the
# network/unsafe-command trace. Keep all route queries (including clock faults).
pure = cmd in ('id','pgrep','date','sysctl','stat','sha256','sync')
if not pure: s['calls'].append([cmd]+a)
rc=0
if cmd == 'id': print('0')
elif cmd == 'lockf':
    os.execv(a[4], a[4:])
elif cmd == 'date': print(s['now'])
elif cmd == 'pgrep': rc=1
elif cmd == 'sync': pass
elif cmd == 'sha256':
    print(hashlib.sha256(Path(a[-1]).read_bytes()).hexdigest())
elif cmd == 'stat':
    target=Path(a[-1]); uid=1001 if s.get('untrusted_path')==str(target) else 0
    print(str(uid)+':0:'+oct(target.stat().st_mode & 0o777)[2:])
elif cmd == 'sysctl':
    if a == ['-b','kern.boot_id']:
        data=bytes.fromhex(s.get('boot_id','01'*16)); size=s.get('boot_id_size',16)
        os.write(1,(data+b'\x00'*max(0,size-len(data)))[:size])
        rc=s.get('boot_id_status',0)
    elif a == ['-n','kern.boottime']: print('{ sec = '+str(s.get('boot_epoch',100))+', usec = 0 }')
    else: raise AssertionError(a)
elif cmd == 'cp':
    if 'confirmed-boot-preparing.' in a[-1] and s.pop('fail_guard_copy_once',False):
        Path(a[-1]).mkdir(mode=0o700); rc=1
    else: rc=subprocess.run(['/bin/cp',*a]).returncode
elif cmd == 'netstat': print('fixture evidence')
elif cmd == 'wg':
    assert s.get('wg_present'), a
    if a == ['showconf','wg0']:
        if s.pop('wg_show_fail_once',False): rc=1
        else: print(s['wg_runtime'],end='')
    elif a[:2] == ['setconf','wg0']:
        data=Path(a[2]).read_text()
        assert Path(a[2]).stat().st_mode & 0o777 == 0o600
        if s.pop('wg_set_fail_once',False):
            s['wg_runtime']='partial peer application\n'; rc=1
        else: s['wg_runtime']=data
    else: raise AssertionError(a)
elif cmd == 'sysrc':
    rcfile=Path(os.environ['FIXTURE_RC'])
    lines=rcfile.read_text().splitlines()
    def write_key(key, value=None):
        global lines
        lines=[line for line in lines if not re.match(re.escape(key)+r'=.*',line)]
        if value is not None: lines.append(key+'='+shlex.quote(value))
        rcfile.write_text('\n'.join(lines)+'\n')
    if a[:1] == ['-f']: a=a[2:]
    if a[:1] == ['-n']:
        key=a[1]
        if key in s['keys']: print(s['keys'][key])
        else: rc=1
    elif a[:1] == ['-x']:
        if a[1] in s['keys']:
            del s['keys'][a[1]]; write_key(a[1])
        else: rc=1
    else:
        for item in a:
            k,v=item.split('=',1); s['keys'][k]=v; write_key(k,v)
elif cmd == 'ifconfig':
    if a[0] == 'wg0':
        assert len(a)==1, 'wg0 addresses/flags must not be changed'
        if s.get('wg_present'): print('wg0: flags=UP')
        else: rc=1
        p.write_text(json.dumps(s)); sys.exit(rc)
    def detach_gateway_routes(ip):
        # FreeBSD removes routes through the detached connected subnet,
        # including when ifconfig replaces the primary with another subnet.
        if ip == s.get('carp',{}).get('vip'): return
        prefix=ip.rsplit('.',1)[0]+'.'
        for key in ('default4','return'):
            if s.get(key) and s[key].startswith(prefix): s[key]=None
    if a[:1] == ['-L']: a=a[1:]
    assert a[0] == 're0', a
    if len(a)==1 or len(a)==2:
        print('re0: flags=UP')
        if len(a)==1 or a[1]=='inet':
            for ip in s['v4']:
                if ip == s.get('carp',{}).get('vip'):
                    print(' inet',ip,'netmask 0xffffffff vhid',s['carp']['vhid'])
                else: print(' inet',ip,'netmask 0xffffff00 broadcast 192.168.1.255')
            if len(a)==1 and s.get('carp',{}).get('vip') in s['v4']:
                print(' carp:',s['carp']['state'],'vhid',s['carp']['vhid'])
        if len(a)==1 or a[1]=='inet6':
            print(' inet6 fe80::1%re0 prefixlen 64 scopeid 0x1')
            for ip in s['v6']:
                props=s.get('v6_properties',{}).get(ip,{})
                print(' inet6',ip,'prefixlen 64',
                      'autoconf' if props.get('autoconf') else '',
                      ('pltime '+str(props['pltime'])+' vltime '+str(props['vltime']))
                      if 'pltime' in props else '',
                      'temporary' if props.get('temporary') else '')
            print(' nd6 options=23<PERFORMNUD'+(',ACCEPT_RTADV' if s['ra'] else '')+'>')
    elif a[1]=='inet6' and a[2] in ['accept_rtadv','-accept_rtadv']:
        s['ra']=a[2]=='accept_rtadv'
    else:
        family=a[1]; ip=a[2].split('/')[0]; field='v4' if family=='inet' else 'v6'
        if a[-1]=='delete':
            assert ip in s[field], (ip,s[field]); s[field].remove(ip)
            if family=='inet': detach_gateway_routes(ip)
            if family=='inet6': s.setdefault('v6_properties',{}).pop(ip,None)
        elif a[-1]=='alias':
            assert ip not in s[field], (ip,s[field]); s[field].append(ip)
            if family=='inet6':
                props={'autoconf': 'autoconf' in a}
                for flag in ('pltime','vltime'):
                    if flag in a: props[flag]=int(a[a.index(flag)+1])
                s.setdefault('v6_properties',{})[ip]=props
        else:
            assert family=='inet', a
            # Native FreeBSD removes the first address and appends the new
            # one. A CARP alias can consequently become the first row.
            detach_gateway_routes(s[field].pop(0))
            s[field].append(ip)
elif cmd == 'route':
    if 'get' in a and a[-1]=='default' and s.get('advance_default_query_to'):
        s['now']=s.pop('advance_default_query_to')
    if a[0] in ('add','change') and 'default' in a and '-inet6' not in a and s.pop('fail_default_once',False):
        p.write_text(json.dumps(s)); sys.exit(1)
    if 'get' in a:
        if '-inet6' in a:
            if s['default6']:
                print('destination: default\n gateway:',s['default6'],'\n interface: re0')
                print(' flags: <UP,GATEWAY,DONE'+(',STATIC' if s.get('default6_static') else '')+'>')
            else: rc=1
        elif a[-1]=='10.99.0.2' and s['return']:
            print('destination: 10.99.0.2\n gateway:',s['return'],'\n interface: re0')
        elif s['default4']: print('destination: default\n gateway: '+s['default4']+'\n interface: re0')
        else: rc=1
    elif '-host' in a:
        if a[0]=='delete': s['return']=None
        else: s['return']=a[-1]
    elif '-inet6' in a:
        s['default6']=None if a[0]=='delete' else a[-1]
        if a[0]!='delete': s['default6_static']='-nostatic' not in a
    else:
        if (a[0]=='change' and not s['default4']) or (a[0]=='add' and s['default4']): rc=1
        else: s['default4']=a[-1]
else: raise AssertionError((cmd,a))
if not pure: p.write_text(json.dumps(s))
sys.exit(rc)
'''


def check_guest(bindir, initial, conf, db, rcconf, state, run):
    # Actual guest interface, explicit no-ULA native policy and retained
    # wg0 ownership: the fixture refuses commands for any other interface.
    for command in ('id', 'lockf', 'date', 'pgrep', 'sync', 'netstat',
                    'sysrc', 'ifconfig', 'route', 'sha256', 'stat', 'sysctl', 'cp'):
        (bindir / command).write_text(MOCK.replace('re0', 'vtnet0'))
    guest = json.loads(json.dumps(initial).replace('re0', 'vtnet0')
                       .replace('.130', '.139').replace('::130', '::139'))
    guest['calls'] = []
    conf.write_text('iface=vtnet0\nold=192.168.1.139\nlab=10.20.0.139\n'
                    'recovery=10.98.0.139\nula=\n'
                    'shared_gateway=192.168.1.2\nipv6_policy=unallocated\n')
    db.write_text(json.dumps(guest))
    rcconf.write_text(''.join(f'{key}={shlex.quote(value)}\n'
                             for key, value in guest['keys'].items()))
    run('stage', ok=False)
    assert not (state / 'pending').exists()
    conf.write_text(conf.read_text().replace('unallocated', 'disabled'))
    run('stage', 'isolated', ok=False)
    staged = run('stage')
    assert staged['v4'] == ['192.168.1.139', '10.20.0.139']
    assert staged['v6'] == [] and staged['ra'] is False
    assert staged['keys']['ifconfig_vtnet0_ipv6'] == 'inet6 -accept_rtadv'
    assert staged['return'] == '192.168.1.2'
    run('rollback')
    restored = json.loads(db.read_text())
    for field in ('keys', 'v4', 'v6', 'ra', 'default4', 'default6', 'return'):
        assert restored[field] == guest[field], field
    run('stage')
    run('confirm')
    run('promote')
    promoted = json.loads(db.read_text())
    assert promoted['v4'] == ['10.20.0.139', '192.168.1.139']
    run('boot')
    restored = json.loads(db.read_text())
    assert restored['keys']['ifconfig_vtnet0'] == guest['keys']['ifconfig_vtnet0']
    assert not any('wg0' in ' '.join(call) for call in restored['calls'])
    print("PASS: standalone guest stage/promotion/boot and unchanged wg0 ownership", flush=True)


def check_retire(state, db, rcconf, run, boot_with_cached_rc, accepted):
    """i24: retire removes only alias200 after accepted isolated promotion."""
    fields = ('keys', 'v4', 'v6', 'default4', 'return')

    def reset(**changes):
        seed = json.loads(json.dumps(accepted))
        seed.update(changes)
        db.write_text(json.dumps(seed))
        rcconf.write_text(''.join(f'{k}={shlex.quote(v)}\n' for k, v in seed['keys'].items()))
        return seed

    def same(current, expected):
        # Restoration re-adds alias200 after the others; only the physical
        # primary's position matters to FreeBSD, so compare it plus the set.
        for field in fields:
            have, want = current[field], expected[field]
            if field == 'v4':
                have, want = (have[:1], sorted(have)), (want[:1], sorted(want))
            assert have == want, (field, current[field], expected[field])

    def refused(seed):
        (state / 'retire-approved').touch()
        current = run('retire', ok=False)
        assert not (state / 'pending').exists(), 'refused retire armed a journal'
        same(current, seed)
        (state / 'retire-approved').unlink()

    seed = reset()
    current = run('retire', ok=False)  # Single-use gate is closed by default.
    assert not (state / 'pending').exists()
    same(current, seed)
    refused(reset(**{'return': '192.168.1.2'}))
    refused(reset(default4='192.168.1.1'))
    refused(reset(v4=['10.20.0.130', '192.168.1.130']))  # recovery missing
    refused(reset(v4=['192.168.1.130', '10.20.0.130', '10.98.0.130']))  # old primary
    refused(reset(v4=['10.20.0.130', '10.98.0.130']))  # already retired
    # Saved configuration is what the next boot uses: runtime alone is not
    # enough. Each seed keeps the accepted runtime and breaks one saved key.
    def saved(**keys):
        changed = dict(accepted['keys'])
        for key, value in keys.items():
            if value is None:
                changed.pop(key, None)
            else:
                changed[key] = value
        return reset(keys=changed)
    assert accepted['keys']['defaultrouter'] == '10.20.0.1'
    refused(saved(defaultrouter='192.168.1.1'))
    refused(saved(defaultrouter=None))
    refused(saved(route_gonfmigration='-host 10.99.0.2 192.168.1.2'))
    refused(saved(static_routes='other'))  # return route no longer started
    refused(saved(route_other='-net 172.16.0.0/12 192.168.1.1'))  # old next hop
    refused(saved(route_other='-net 192.168.1.64/26 10.20.0.1'))  # old destination
    carp = json.loads(json.dumps(accepted))
    carp['v4'] = carp['v4'] + ['192.168.1.138']
    carp['carp'] = {'vip': '192.168.1.138', 'vhid': 1, 'state': 'MASTER'}
    refused(reset(v4=carp['v4'], carp=carp['carp']))
    # A lab-subnet CARP VIP is not the old /24 and does not block retirement.
    lab_carp = {'vip': '10.20.0.138', 'vhid': 1, 'state': 'MASTER'}
    # An unrelated saved route through the lab gateway does not block retire.
    keys = dict(accepted['keys'], route_other='-net 172.16.0.0/12 10.20.0.1')
    reset(v4=accepted['v4'] + ['10.20.0.138'], carp=lab_carp, keys=keys)
    (state / 'retire-approved').touch()
    current = run('retire')
    assert current['v4'] == ['10.20.0.130', '10.98.0.130', '10.20.0.138'], current['v4']
    run('rollback')
    seed = reset()
    (state / 'retire-approved').touch()
    retired = run('retire')
    assert not (state / 'retire-approved').exists(), 'retire gate not consumed'
    assert retired['v4'] == ['10.20.0.130', '10.98.0.130'], retired['v4']
    assert 'ifconfig_re0_alias200' not in retired['keys']
    assert 'ifconfig_re0_alias200' not in rcconf.read_text()
    assert retired['keys']['ifconfig_re0_alias202'] == 'inet 10.98.0.130/24'
    assert retired['default4'] == '10.20.0.1' and retired['return'] == '10.98.0.1'
    assert retired['v6'] == seed['v6']
    same(run('rollback'), seed)
    # Expiry through the independent watchdog.
    reset()
    (state / 'retire-approved').touch()
    expired = run('retire')
    expired['now'] += 901
    db.write_text(json.dumps(expired))
    run('confirm', ok=False)
    same(run('watchdog'), seed)
    # Unconfirmed boot restores alias200 before netif reads rc.conf.
    reset()
    (state / 'retire-approved').touch()
    run('retire')
    boot_with_cached_rc(seed)
    assert not (state / 'pending').exists()
    # Confirmation keeps the retired state; a second retire refuses.
    reset()
    (state / 'retire-approved').touch()
    run('retire')
    confirmed = run('confirm')
    assert '192.168.1.130' not in confirmed['v4']
    refused(confirmed)
    print('PASS: retire gate/preconditions/old-CARP refusal, alias200-only removal, '
          'rollback, watchdog expiry, boot restore and confirm', flush=True)
    return confirmed


def check_retire_recovery(state, db, rcconf, run, boot_with_cached_rc, accepted):
    """j24: retire-recovery removes alias202 and the 10.98.0.1 return route
    after i24, so wg2 replies use the 10.20.0.1 default."""
    fields = ('keys', 'v4', 'v6', 'default4', 'return')
    gate = state / 'retire-recovery-approved'

    def reset(**changes):
        seed = json.loads(json.dumps(accepted))
        seed.update(changes)
        db.write_text(json.dumps(seed))
        rcconf.write_text(''.join(f'{k}={shlex.quote(v)}\n' for k, v in seed['keys'].items()))
        return seed

    def same(current, expected):
        for field in fields:
            have, want = current[field], expected[field]
            if field == 'v4':
                have, want = (have[:1], sorted(have)), (want[:1], sorted(want))
            if field == 'keys':  # restore_keys may rewrite whitespace-free values only
                have = {k: v.strip() for k, v in have.items()}
                want = {k: v.strip() for k, v in want.items()}
            assert have == want, (field, current[field], expected[field])

    def saved(**keys):
        changed = dict(accepted['keys'])
        for key, value in keys.items():
            if value is None:
                changed.pop(key, None)
            else:
                changed[key] = value
        return reset(keys=changed)

    def refused(seed):
        gate.touch()
        current = run('retire-recovery', ok=False)
        assert not (state / 'pending').exists(), 'refused retire-recovery armed a journal'
        same(current, seed)
        gate.unlink()

    assert accepted['v4'] == ['10.20.0.130', '10.98.0.130'], accepted['v4']
    seed = reset()
    current = run('retire-recovery', ok=False)  # gate closed by default
    assert not (state / 'pending').exists()
    same(current, seed)
    refused(reset(v4=['10.20.0.130', '10.98.0.130', '192.168.1.130']))  # old not retired
    refused(saved(ifconfig_re0_alias200='inet 192.168.1.130/24'))  # old still saved
    refused(reset(v4=['10.20.0.130']))  # recovery already gone
    refused(saved(ifconfig_re0_alias202=None))
    refused(reset(default4='10.98.0.1'))
    refused(saved(defaultrouter='10.98.0.1'))
    refused(reset(**{'return': '10.20.0.1'}))
    refused(saved(route_gonfmigration='-host 10.99.0.2 10.20.0.1'))
    refused(saved(static_routes='other'))
    refused(saved(route_other='-net 172.16.0.0/12 10.98.0.9'))  # recovery next hop
    refused(saved(route_other='-net 10.98.0.0/24 10.20.0.1'))  # recovery destination
    # An unrelated saved route stays, and static_routes keeps only it.
    routes = accepted['keys']['static_routes'].split()
    assert 'gonfmigration' in routes, routes
    if 'other' not in routes:
        routes.append('other')
    keys = dict(accepted['keys'], route_other='-net 172.16.0.0/12 10.20.0.1',
                static_routes=' '.join(routes))
    reset(keys=keys)
    gate.touch()
    current = run('retire-recovery')
    want = ' '.join(r for r in routes if r != 'gonfmigration')
    assert current['keys']['static_routes'] == want, (current['keys'], want)
    assert current['keys']['route_other'] == '-net 172.16.0.0/12 10.20.0.1'
    run('rollback')
    seed = reset()
    gate.touch()
    done = run('retire-recovery')
    assert not gate.exists(), 'retire-recovery gate not consumed'
    assert done['v4'] == ['10.20.0.130'], done['v4']
    for key in ('ifconfig_re0_alias202', 'route_gonfmigration'):
        assert key not in done['keys'], key
        assert key + '=' not in rcconf.read_text(), key
    left = [r for r in accepted['keys']['static_routes'].split() if r != 'gonfmigration']
    assert done['keys'].get('static_routes', '').split() == left, done['keys']
    assert done['default4'] == '10.20.0.1' and done['return'] is None, done
    assert done['v6'] == seed['v6']
    same(run('rollback'), seed)
    # Expiry through the independent watchdog.
    reset()
    gate.touch()
    expired = run('retire-recovery')
    expired['now'] += 901
    db.write_text(json.dumps(expired))
    run('confirm', ok=False)
    same(run('watchdog'), seed)
    # Unconfirmed boot restores alias202 and the route before netif.
    reset()
    gate.touch()
    run('retire-recovery')
    boot_with_cached_rc(seed)
    assert not (state / 'pending').exists()
    # Confirmation keeps the final state; a second run refuses.
    reset()
    gate.touch()
    run('retire-recovery')
    confirmed = run('confirm')
    assert confirmed['v4'] == ['10.20.0.130'] and confirmed['return'] is None
    refused(confirmed)
    print('PASS: retire-recovery gate/preconditions, alias202 and return-route-only removal, '
          'rollback, watchdog expiry, boot restore and confirm', flush=True)


def main():
    with tempfile.TemporaryDirectory(prefix="freebsd-migration-") as raw:
        tmp = Path(raw)
        state = tmp / "state"
        state.mkdir(mode=0o700)
        bindir = tmp / "bin"
        bindir.mkdir()
        db = tmp / "db.json"
        rcconf = tmp / "rc.conf"
        rcconf.write_text('# fixture only\n')
        rcconf.chmod(0o644)  # Native live rc.conf mode, independent of caller umask.
        conf = tmp / "migration.conf"
        conf.write_text("iface=re0\nold=192.168.1.130\nlab=10.20.0.130\n"
                        "recovery=10.98.0.130\nula=fd02:ca0b:f1cb:20::130\n"
                        "shared_gateway=192.168.1.2\n")
        for command in ('id', 'lockf', 'date', 'pgrep', 'sync', 'netstat',
                        'sysrc', 'ifconfig', 'route', 'sha256', 'stat', 'sysctl', 'cp'):
            file = bindir / command
            file.write_text(MOCK)
            file.chmod(0o755)
        helper = tmp / "migrate"
        source = (HERE / "migrate.sh").read_text()
        source = source.replace('PATH=/sbin:/bin:/usr/sbin:/usr/bin:/usr/local/sbin:/usr/local/bin',
                                f'PATH={bindir}:/usr/bin:/bin')
        source = source.replace('/var/db/gonf-network-migration', str(state))
        source = source.replace('/usr/local/sbin/gonf-network-migration', str(helper))
        source = source.replace('/usr/local/etc/gonf-network-migration.conf', str(conf))
        source = source.replace('/etc/rc.conf', str(rcconf))
        source = source.replace('/usr/local/sbin/gonf-endpoint-rollback', str(tmp / 'endpoint-hook'))
        helper.write_text(source)
        helper.chmod(0o755)
        env = dict(os.environ, FIXTURE_DB=str(db), FIXTURE_RC=str(rcconf))
        initial = dict(keys={'ifconfig_re0': 'inet 192.168.1.130/24',
                             'ifconfig_re0_alias0': 'CARP owned separately',
                             'vm_enable': 'YES', 'static_routes': 'other',
                             'defaultrouter': '192.168.1.1',
                             'ifconfig_re0_ipv6': 'inet6 accept_rtadv'},
                       v4=['192.168.1.130'], v6=['2a01:5a8::130', 'fd22:c702:acb7::130'],
                       ra=True, default4='192.168.1.1', default6='fe80::1%re0',
                       return_=None, now=1000, calls=[])
        initial['return'] = initial.pop('return_')
        db.write_text(json.dumps(initial))
        rcconf.write_text(''.join(f'{key}={shlex.quote(value)}\n'
                                 for key, value in initial['keys'].items()))

        def run(*args, ok=True):
            result = subprocess.run([str(helper), *args], env=env,
                                    capture_output=True, text=True)
            assert (result.returncode == 0) == ok, (args, result.stdout, result.stderr,
                                                   json.loads(db.read_text())['calls'][-5:])
            return json.loads(db.read_text())

        def boot_with_cached_rc(expected):
            # Emulate /etc/rc's parent: source unconfirmed keys, execute the
            # rollback child, then source again as its ALRM handler does.
            # This deliberately keeps removed variables alive across reload
            # unless the generated unset statement clears them in the parent.
            checks = []
            for key in ('ifconfig_re0', 'ifconfig_re0_alias200',
                        'ifconfig_re0_alias201', 'ifconfig_re0_alias202',
                        'ifconfig_re0_alias0', 'route_gonfmigration',
                        'ipv6_defaultrouter', 'vm_enable'):
                if key in expected['keys']:
                    checks.append(f'test "${{{key}}}" = {shlex.quote(expected["keys"][key])}')
                else:
                    checks.append(f'test "${{{key}+present}}" != present')
            script = '. "$1"; "$2" boot; . "$1"; ' + '; '.join(checks)
            script += "; set | awk '/^ifconfig_re0_alias[0-9]+=/ {print}'"
            result = subprocess.run(['sh', '-ec', script, 'cached-rc', str(rcconf), str(helper)],
                                    env=env, capture_output=True, text=True)
            assert result.returncode == 0, (result.stdout, result.stderr, rcconf.read_text())
            aliases = {line.split('=', 1)[0] for line in result.stdout.splitlines()}
            expected_aliases = {key for key in expected['keys'] if key.startswith('ifconfig_re0_alias')}
            assert aliases == expected_aliases, (aliases, expected_aliases)

        def assert_restored():
            current = json.loads(db.read_text())
            for field in ('keys', 'v4', 'v6', 'ra', 'default4', 'default6', 'return'):
                assert current[field] == initial[field], (field, current[field], initial[field])

        if sys.argv[1:] == ["--guest-only"]:
            (state / "activation-approved").touch()
            check_guest(bindir, initial, conf, db, rcconf, state, run)
            return

        run('stage', ok=False)  # Activation is deliberately closed by default.
        (state / 'activation-approved').touch()
        staged = run('stage')
        assert staged['v4'] == ['192.168.1.130', '10.20.0.130']
        assert staged['return'] == '192.168.1.2'
        assert staged['v6'] == ['fd02:ca0b:f1cb:20::130']
        assert staged['keys']['ifconfig_re0_alias0'] == 'CARP owned separately'
        run('stage', ok=False)  # Never replace an armed snapshot.
        run('rollback')
        assert_restored()
        assert json.loads(db.read_text())['default6_static'] is False
        run('rollback')  # Restore retries are harmless after completion.
        run('stage')
        boot_with_cached_rc(initial)  # Absent alias201 must disappear in parent.
        assert not (state / 'pending').exists()
        # Simulate initial netif configuration after the proved cached reload.
        db.write_text(json.dumps(initial))
        run('stage')
        run('confirm')
        accepted = json.loads(db.read_text())
        run('promote')
        promoted = json.loads(db.read_text())
        assert promoted['v4'] == ['10.20.0.130', '192.168.1.130']
        assert promoted['default4'] == '10.20.0.1'
        assert promoted['return'] == '192.168.1.2'
        assert ['route','add','default','10.20.0.1'] in promoted['calls']
        promoted['now'] = 2000
        db.write_text(json.dumps(promoted))
        run('confirm', ok=False)
        run('watchdog')
        current = json.loads(db.read_text())
        assert ['route','add','default','192.168.1.1'] in current['calls']
        for field in ('keys', 'v4', 'v6', 'default4', 'return'):
            assert current[field] == accepted[field], field
        run('promote')
        boot_with_cached_rc(accepted)  # Clear alias200; preserve accepted alias201.
        current = json.loads(db.read_text())
        assert current['keys'] == accepted['keys']
        assert not (state / 'pending').exists()
        # Simulate subsequent netif reading restored persistent state.
        accepted['now'] = 2000
        db.write_text(json.dumps(accepted))
        (state / 'endpoints-required').touch()
        run('promote', ok=False)  # Missing endpoint hook fails before mutation.
        assert not (state / 'pending').exists()
        (state / 'endpoints-required').unlink()
        run('stage', 'isolated', ok=False)  # Human/cabling gate stays closed.
        calls = json.loads(db.read_text())['calls']
        assert not any('netif' in ' '.join(call) or 'bridge' in ' '.join(call) for call in calls)
        # Add recovery directly to an already confirmed lab primary. A second
        # Stage would otherwise persist a duplicate lab alias beside primary.
        run('promote'); run('confirm')
        shared_new = json.loads(db.read_text())
        run('promote', 'isolated', ok=False)
        assert not (state / 'pending').exists()
        missing_gate = json.loads(db.read_text())
        for field in ('keys', 'v4', 'v6', 'default4', 'return'):
            assert missing_gate[field] == shared_new[field], field
        (state / 'igc1-approved').touch()
        recovery_new = run('promote', 'isolated')
        assert recovery_new['v4'] == ['10.20.0.130', '192.168.1.130', '10.98.0.130']
        assert recovery_new['default4'] == shared_new['default4'] == '10.20.0.1'
        assert recovery_new['return'] == '10.98.0.1'
        assert recovery_new['keys']['ifconfig_re0_alias202'] == 'inet 10.98.0.130/24'
        assert 'ifconfig_re0_alias201' not in recovery_new['keys']
        assert recovery_new['v6'] == shared_new['v6']
        recovery_new['now'] += 901
        db.write_text(json.dumps(recovery_new)); run('watchdog')
        restored_new = json.loads(db.read_text())
        for field in ('keys', 'v4', 'v6', 'default4', 'return'):
            assert restored_new[field] == shared_new[field], field
        run('promote', 'isolated')
        boot_with_cached_rc(shared_new)
        assert not (state / 'pending').exists()
        run('promote', 'isolated'); confirmed_recovery = run('confirm')
        assert confirmed_recovery['keys']['ifconfig_re0_alias202'] == 'inet 10.98.0.130/24'
        assert confirmed_recovery['return'] == '10.98.0.1'
        retired = check_retire(state, db, rcconf, run, boot_with_cached_rc, confirmed_recovery)
        check_retire_recovery(state, db, rcconf, run, boot_with_cached_rc, retired)
        (state / 'igc1-approved').unlink()
        if sys.argv[1:] == ['--recovery-only']:
            print('PASS: direct shared-NEW recovery, missing-gate refusal, persistence, expiry and boot rollback', flush=True)
            return
        # Reproduce f1: a real CARP row moves ahead of ordinary addresses
        # after primary replacement. It must survive promotion AND rollback.
        carp_seed=json.loads(json.dumps(initial))
        carp_seed.update(v4=['192.168.1.130','192.168.1.138'],now=3000,calls=[],
                         carp={'vip':'192.168.1.138','vhid':1,'state':'BACKUP'})
        db.write_text(json.dumps(carp_seed))
        rcconf.write_text(''.join(f'{k}={shlex.quote(v)}\n' for k,v in carp_seed['keys'].items()))
        run('stage'); carp_stage=run('confirm')
        carp_new=run('promote','shared','300')
        assert '192.168.1.138' in carp_new['v4'], carp_new['v4']
        assert [ip for ip in carp_new['v4'] if ip!='192.168.1.138']==['10.20.0.130','192.168.1.130']
        carp_old=run('rollback')
        assert '192.168.1.138' in carp_old['v4'], carp_old['v4']
        assert [ip for ip in carp_old['v4'] if ip!='192.168.1.138']==['192.168.1.130','10.20.0.130']
        assert carp_old['carp']==carp_seed['carp'] and carp_old['keys']==carp_stage['keys']
        assert all(not(call[0]=='ifconfig' and '192.168.1.138' in call) for call in carp_old['calls'])
        # A different ordinary alias is not CARP. Refuse rotation before
        # mutation rather than hiding or deleting another owner's address.
        carp_old['v4'].append('192.168.1.199')
        db.write_text(json.dumps(carp_old)); before_foreign=json.loads(db.read_text())
        foreign=run('promote','shared','300',ok=False)
        assert foreign['v4']==before_foreign['v4'] and foreign['keys']==before_foreign['keys']
        assert not (state/'pending').exists()
        print('PASS: CARP address/role survives native primary reordering and rollback; foreign aliases refuse',flush=True)
        # Confirmed-NEW boot protection is an isolated lease over an immutable
        # accepted OLD snapshot. Ordinary pending is never edited or bypassed.
        guard = state / 'confirmed-boot-guard'

        def guard_trial(now):
            seed = json.loads(json.dumps(initial))
            seed.update(now=now, boot_epoch=100, boot_id='01'*16, calls=[])
            db.write_text(json.dumps(seed))
            rcconf.write_text(''.join(f'{k}={shlex.quote(v)}\n' for k,v in seed['keys'].items()))
            run('stage'); accepted_stage = run('confirm')
            existing = set(state.glob('accepted-*'))
            run('promote', 'shared', '300'); confirmed = run('confirm')
            archive, = set(state.glob('accepted-*')) - existing
            snapshot = archive / 'snapshot'
            assert (snapshot / 'rc.conf.evidence').stat().st_mode & 0o777 == 0o600
            assert rcconf.stat().st_mode & 0o777 == 0o644
            result = subprocess.run([str(helper), 'confirmed-boot-hash', archive.name],
                                    env=env, capture_output=True, text=True)
            assert result.returncode == 0, result.stderr
            digest = result.stdout.strip()
            assert len(digest) == 64
            return archive.name, snapshot, digest, accepted_stage, confirmed

        def set_fixture(**values):
            current = json.loads(db.read_text()); current.update(values)
            db.write_text(json.dumps(current))

        def same_network(expected, fields=('keys','v4','v6','default4','return')):
            current = json.loads(db.read_text())
            assert all(current[k] == expected[k] for k in fields), fields
            assert not (state / 'pending').exists()

        name, snapshot, digest, old_stage, new_primary = guard_trial(4000)
        immutable = {f.name:f.read_bytes() for f in snapshot.iterdir()}
        run('arm-confirmed-boot', name, '0'*64, '120', ok=False)
        assert not guard.exists(); same_network(new_primary)
        for field, replacement in [('v4', b'changed\n'), ('primary', b'10.20.0.130\n')]:
            original = (snapshot / field).read_bytes()
            (snapshot / field).write_bytes(replacement)
            run('arm-confirmed-boot', name, digest, '120', ok=False)
            assert not guard.exists(); same_network(new_primary)
            (snapshot / field).write_bytes(original)
        field = snapshot / 'v4'; original = field.read_bytes()
        field.unlink(); field.symlink_to(snapshot / 'primary')
        run('arm-confirmed-boot', name, digest, '120', ok=False)
        field.unlink(); field.write_bytes(original); field.chmod(0o600)
        field.chmod(0o644)
        run('arm-confirmed-boot', name, digest, '120', ok=False)
        field.chmod(0o600)
        set_fixture(untrusted_path=str(snapshot))
        run('arm-confirmed-boot', name, digest, '120', ok=False)
        set_fixture(untrusted_path='')
        # An owned-address filter must not hide a foreign actual primary.
        set_fixture(v4=['192.168.1.138',*new_primary['v4']])
        run('arm-confirmed-boot', name, digest, '120', ok=False)
        assert not guard.exists()
        set_fixture(v4=new_primary['v4'])
        subprocess.run([str(bindir/'sysrc'),'-f',str(rcconf),'-x','ifconfig_re0_alias200'],env=env,check=True,capture_output=True)
        run('arm-confirmed-boot', name, digest, '120', ok=False)
        assert not guard.exists() and '192.168.1.130' in json.loads(db.read_text())['v4']
        subprocess.run([str(bindir/'sysrc'),'-f',str(rcconf),'ifconfig_re0_alias200=inet 192.168.1.130/24'],env=env,check=True,capture_output=True)

        # Interrupted copy creates only a private preparing directory; active
        # lease publication remains atomic and ordinary configuration untouched.
        set_fixture(fail_guard_copy_once=True)
        run('arm-confirmed-boot', name, digest, '120', ok=False)
        assert not guard.exists(); same_network(new_primary)
        run('arm-confirmed-boot', name, digest, '120')
        original_deadline = (guard / 'deadline').read_bytes()
        for command in ('stage','promote','confirm','rollback'):
            run(command, ok=False); same_network(new_primary)
        # Retire is refused by the guard itself, not by a later precondition.
        (state / 'retire-approved').touch()
        result = subprocess.run([str(helper), 'retire'], env=env, capture_output=True, text=True)
        assert result.returncode != 0 and 'confirmed-boot guard is active' in result.stderr, result
        same_network(new_primary); (state / 'retire-approved').unlink()
        run('arm-confirmed-boot', name, digest, '900', ok=False)
        assert (guard / 'deadline').read_bytes() == original_deadline
        run('release-confirmed-boot', ok=False)
        run('boot', ok=False)  # Same kernel cannot fabricate a boot claim.
        set_fixture(now=4121)
        run('watchdog')  # A confirmed NEW primary expires independently.
        same_network(old_stage); assert not guard.exists()
        run('restore-confirmed-boot'); run('restore-confirmed-boot')
        assert immutable == {f.name:f.read_bytes() for f in snapshot.iterdir()}
        run('promote', 'shared', '300')
        run('arm-confirmed-boot', name, digest, '120', ok=False)
        run('rollback'); same_network(old_stage)

        print('PASS: guard admission, atomic publication, exclusivity and independent expiry', flush=True)

        name, snapshot, digest, old_stage, new_primary = guard_trial(5000)
        run('arm-confirmed-boot', name, digest, '600')
        set_fixture(boot_epoch=101, boot_id='02'*16, now=5010)
        run('boot'); same_network(new_primary)
        run('boot'); same_network(new_primary)  # Same-kernel replay is harmless.
        # After a genuine new-kernel claim, release must check both surfaces:
        # the actual first physical IPv4, and the OLD alias persisted for reboot.
        set_fixture(v4=['192.168.1.138',*new_primary['v4']])
        run('release-confirmed-boot', ok=False)
        assert guard.exists()
        set_fixture(v4=new_primary['v4'])
        subprocess.run([str(bindir/'sysrc'),'-f',str(rcconf),'-x','ifconfig_re0_alias200'],env=env,check=True,capture_output=True)
        run('release-confirmed-boot', ok=False)
        assert guard.exists() and '192.168.1.130' in json.loads(db.read_text())['v4']
        subprocess.run([str(bindir/'sysrc'),'-f',str(rcconf),'ifconfig_re0_alias200=inet 192.168.1.130/24'],env=env,check=True,capture_output=True)
        (guard / 'claim-started').rename(guard / 'claim-saved')
        run('release-confirmed-boot', ok=False)  # Epoch alone is not a complete claim.
        (guard / 'claim-saved').rename(guard / 'claim-started')
        run('release-confirmed-boot')
        set_fixture(now=5700)
        run('watchdog'); same_network(new_primary)
        assert not guard.exists()

        name, snapshot, digest, old_stage, new_primary = guard_trial(6000)
        run('arm-confirmed-boot', name, digest, '600')
        set_fixture(boot_epoch=101, boot_id='02'*16, now=6010); run('boot')
        set_fixture(boot_epoch=101, boot_id='03'*16)  # Different boot, identical wall-clock estimate.
        run('release-confirmed-boot', ok=False)
        boot_with_cached_rc(old_stage)  # A second NEW boot consumes no new lease.
        assert not guard.exists()
        assert json.loads(db.read_text())['keys'] == old_stage['keys']

        for index, claim in enumerate(('missing','malformed','nonprivate')):
            name, snapshot, digest, old_stage, new_primary = guard_trial(7000+index*100)
            run('arm-confirmed-boot', name, digest, '600')
            set_fixture(boot_epoch=101, boot_id='02'*16)
            (guard / 'claim-started').touch(mode=0o600)
            if claim != 'missing':
                (guard / 'boot-id').write_text('bad\n' if claim=='malformed' else hashlib.sha256(bytes.fromhex('02'*16)).hexdigest()+'\n')
                (guard / 'boot-id').chmod(0o644 if claim=='nonprivate' else 0o600)
            boot_with_cached_rc(old_stage)
            assert not guard.exists()  # Interrupted claim forces OLD before netif.

        name, snapshot, digest, old_stage, new_primary = guard_trial(8000)
        run('arm-confirmed-boot', name, digest, '120')
        set_fixture(boot_epoch=101, boot_id='02'*16, now=8121)
        run('release-confirmed-boot', ok=False)
        boot_with_cached_rc(old_stage); assert not guard.exists()

        name, snapshot, digest, old_stage, new_primary = guard_trial(9000)
        run('arm-confirmed-boot', name, digest, '600')
        (guard / 'claim-started').touch(mode=0o600)
        set_fixture(boot_epoch=101, boot_id='02'*16)
        run('watchdog'); same_network(old_stage)  # Retry an interrupted boot claim.

        print('PASS: one-boot allowance, release, expired boot and interrupted claims', flush=True)

        name, snapshot, digest, old_stage, new_primary = guard_trial(10000)
        immutable = {f.name:f.read_bytes() for f in snapshot.iterdir()}
        run('arm-confirmed-boot', name, digest, '600')
        set_fixture(fail_default_once=True)
        run('restore-confirmed-boot', ok=False)
        assert guard.exists() and (guard / 'restoring').exists()
        assert not (state / 'pending').exists()
        run('watchdog'); same_network(old_stage)  # Before deadline, restoring retries.
        run('restore-confirmed-boot')
        assert immutable == {f.name:f.read_bytes() for f in snapshot.iterdir()}

        name, snapshot, digest, old_stage, new_primary = guard_trial(11000)
        run('arm-confirmed-boot', name, digest, '600')
        original = (snapshot / 'v4').read_bytes(); (snapshot / 'v4').write_text('changed\n')
        run('restore-confirmed-boot', ok=False); same_network(new_primary)
        (snapshot / 'v4').write_bytes(original)
        original_conf = conf.read_bytes(); conf.write_bytes(original_conf+b'# changed\n')
        run('restore-confirmed-boot', ok=False); same_network(new_primary)
        conf.write_bytes(original_conf)
        run('restore-confirmed-boot'); same_network(old_stage)

        name, snapshot, digest, old_stage, new_primary = guard_trial(12000)
        run('arm-confirmed-boot', name, digest, '600')
        set_fixture(boot_epoch=101, boot_id='02'*16, now=12010); run('boot')
        deadline = int((guard / 'deadline').read_text())
        set_fixture(advance_default_query_to=deadline)
        run('release-confirmed-boot', ok=False)  # Last runtime query crosses deadline.
        assert guard.exists(); same_network(new_primary)
        run('watchdog'); same_network(old_stage)

        print('PASS: partial restore retry, immutable input and deadline-crossing release', flush=True)

        # Kernel identity is independent of the wall-clock boot-time estimate.
        name, snapshot, digest, old_stage, new_primary = guard_trial(13000)
        run('arm-confirmed-boot', name, digest, '600')
        set_fixture(boot_epoch=101, now=13010)  # Clock correction, no actual new boot ID.
        run('boot', ok=False)
        assert guard.exists() and not (guard / 'claim-started').exists()
        assert not (guard / 'boot-id').exists()
        same_network(new_primary)
        run('restore-confirmed-boot'); same_network(old_stage)

        name, snapshot, digest, old_stage, new_primary = guard_trial(14000)
        run('arm-confirmed-boot', name, digest, '600')
        set_fixture(boot_id='02'*16, now=14010)  # New boot ID, unchanged boot_epoch=100.
        run('boot'); same_network(new_primary)
        assert (guard / 'boot-id').read_text().strip() == hashlib.sha256(bytes.fromhex('02'*16)).hexdigest()
        set_fixture(boot_epoch=101)
        run('watchdog'); same_network(new_primary)
        set_fixture(boot_epoch=99)
        run('boot'); same_network(new_primary)
        run('release-confirmed-boot')
        assert not guard.exists()

        name, snapshot, digest, old_stage, new_primary = guard_trial(15000)
        run('arm-confirmed-boot', name, digest, '600')
        set_fixture(boot_id='02'*16, now=15010); run('boot')
        set_fixture(boot_id='03'*16)  # A second boot still reports boot_epoch=100.
        run('release-confirmed-boot', ok=False)
        assert guard.exists()
        run('watchdog'); same_network(old_stage)

        # Preserve producer failure even if sysctl writes a complete 16-byte ID.
        invalid_ids = ({'boot_id_size':0}, {'boot_id_size':15}, {'boot_id_size':17},
                       {'boot_id_size':0, 'boot_id_status':1},
                       {'boot_id_size':16, 'boot_id_status':1})
        for index, invalid in enumerate(invalid_ids):
            name, snapshot, digest, old_stage, new_primary = guard_trial(16000+index*100)
            set_fixture(**invalid)
            run('arm-confirmed-boot', name, digest, '600', ok=False)
            assert not guard.exists() and not list(state.glob('boot-id.*'))
            same_network(new_primary)
            set_fixture(boot_id_size=16, boot_id_status=0)
            run('arm-confirmed-boot', name, digest, '600')
            set_fixture(boot_id='02'*16, **invalid)
            boot_with_cached_rc(old_stage)  # Includes unavailable/unseeded entropy.
            assert not guard.exists() and not list(state.glob('boot-id.*'))

        name, snapshot, digest, old_stage, new_primary = guard_trial(17000)
        run('arm-confirmed-boot', name, digest, '600')
        set_fixture(boot_id='02'*16); run('boot')
        set_fixture(boot_id_status=1, boot_id_size=16)
        run('release-confirmed-boot', ok=False)
        assert guard.exists()
        run('watchdog'); same_network(old_stage)
        assert not list(state.glob('boot-id.*'))

        print('PASS: stable opaque boot identity, clock correction, identical-epoch boots,', flush=True)
        print('strict producer/size admission, early entropy failure and watchdog recovery', flush=True)

        # Native SLAAC ownership and lifetimes must survive detached rollback,
        # with time spent in the trial deducted rather than renewed forever.
        db.write_text(json.dumps(initial))
        rcconf.write_text(''.join(f'{key}={shlex.quote(value)}\n'
                                 for key, value in initial['keys'].items()))
        native=json.loads(db.read_text())
        native['v6_properties']={
            native['v6'][0]: {'autoconf':True, 'pltime':1200, 'vltime':1800},
            native['v6'][1]: {'autoconf':True, 'pltime':20, 'vltime':30}}
        db.write_text(json.dumps(native))
        run('stage')
        trial=json.loads(db.read_text()); trial['now']+=600
        db.write_text(json.dumps(trial))
        restored=run('rollback')
        assert restored['v6']==[native['v6'][0]]
        assert restored['v6_properties'][native['v6'][0]]=={
            'autoconf':True, 'pltime':600, 'vltime':1200}
        # No supported setter for temporary ownership: refuse before mutation.
        native['v6_properties'][native['v6'][0]]['temporary']=True
        db.write_text(json.dumps(native))
        run('stage', ok=False)
        assert not (state / 'pending').exists()
        assert json.loads(db.read_text())['v6']==native['v6']
        check_guest(bindir, initial, conf, db, rcconf, state, run)
        print('PASS: activation gates, dual-address stage, explicit confirm, primary promotion,')
        print('timer rollback, repeated rollback, cached parent rc reload/alias enumeration,')
        print('endpoint fail-closed, CARP/bhyve preservation, confirmed-boot guard')


if __name__ == '__main__':
    main()
