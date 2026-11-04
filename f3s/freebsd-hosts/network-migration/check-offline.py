#!/usr/bin/env python3
"""Exercise the shell transaction with disposable command fixtures, never SSH."""
import json
import os
import shlex
from pathlib import Path
import subprocess
import tempfile

HERE = Path(__file__).resolve().parent
MOCK = r'''#!/usr/bin/env python3
import json, os, re, shlex, sys
from pathlib import Path
p=Path(os.environ['FIXTURE_DB'])
s=json.loads(p.read_text())
a=sys.argv[1:]; cmd=Path(sys.argv[0]).name
s['calls'].append([cmd]+a)
rc=0
if cmd == 'id': print('0')
elif cmd == 'lockf':
    os.execv(a[4], a[4:])
elif cmd == 'date': print(s['now'])
elif cmd == 'pgrep': rc=1
elif cmd == 'sync': pass
elif cmd == 'netstat': print('fixture evidence')
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
    assert a[0] == 're0', a
    if len(a)==1 or len(a)==2:
        print('re0: flags=UP')
        if len(a)==1 or a[1]=='inet':
            for ip in s['v4']: print(' inet',ip,'netmask 0xffffff00 broadcast 192.168.1.255')
            print(' inet 192.168.1.138 netmask 0xffffffff vhid 1')
        if len(a)==1 or a[1]=='inet6':
            print(' inet6 fe80::1%re0 prefixlen 64 scopeid 0x1')
            for ip in s['v6']: print(' inet6',ip,'prefixlen 64')
            print(' nd6 options=23<PERFORMNUD'+(',ACCEPT_RTADV' if s['ra'] else '')+'>')
    elif a[1]=='inet6' and a[2] in ['accept_rtadv','-accept_rtadv']:
        s['ra']=a[2]=='accept_rtadv'
    else:
        family=a[1]; ip=a[2].split('/')[0]; field='v4' if family=='inet' else 'v6'
        if a[-1]=='delete':
            assert ip in s[field], (ip,s[field]); s[field].remove(ip)
        elif a[-1]=='alias':
            assert ip not in s[field], (ip,s[field]); s[field].append(ip)
        else:
            assert family=='inet', a
            s[field][0]=ip
elif cmd == 'route':
    if 'get' in a:
        if '-inet6' in a:
            if s['default6']:
                print('destination: default\n gateway:',s['default6'],'\n interface: re0')
            else: rc=1
        elif a[-1]=='10.99.0.2' and s['return']:
            print('destination: 10.99.0.2\n gateway:',s['return'],'\n interface: re0')
        else: print('destination: default\n gateway:',s['default4'],'\n interface: re0')
    elif '-host' in a:
        if a[0]=='delete': s['return']=None
        else: s['return']=a[-1]
    elif '-inet6' in a:
        s['default6']=None if a[0]=='delete' else a[-1]
    else: s['default4']=a[-1]
else: raise AssertionError((cmd,a))
p.write_text(json.dumps(s))
sys.exit(rc)
'''


def main():
    with tempfile.TemporaryDirectory(prefix="freebsd-migration-") as raw:
        tmp = Path(raw)
        state = tmp / "state"
        state.mkdir()
        bindir = tmp / "bin"
        bindir.mkdir()
        db = tmp / "db.json"
        rcconf = tmp / "rc.conf"
        rcconf.write_text('# fixture only\n')
        conf = tmp / "migration.conf"
        conf.write_text("iface=re0\nold=192.168.1.130\nlab=10.20.0.130\n"
                        "recovery=10.98.0.130\nula=fd02:ca0b:f1cb:20::130\n"
                        "shared_gateway=192.168.1.2\n")
        for command in ('id', 'lockf', 'date', 'pgrep', 'sync', 'netstat',
                        'sysrc', 'ifconfig', 'route'):
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
            assert (result.returncode == 0) == ok, (args, result.stdout, result.stderr)
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
        promoted['now'] = 2000
        db.write_text(json.dumps(promoted))
        run('confirm', ok=False)
        run('watchdog')
        current = json.loads(db.read_text())
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
        print('PASS: activation gates, dual-address stage, explicit confirm, primary promotion,')
        print('timer rollback, repeated rollback, cached parent rc reload/alias enumeration,')
        print('endpoint fail-closed, CARP/bhyve preservation')


if __name__ == '__main__':
    main()
