#!/usr/bin/env python3
"""Run the real dispatch/state code with disposable OS command fixtures only."""
from pathlib import Path
import os
import json
import subprocess
import tempfile
import time

source = Path(__file__).with_name('migrate.sh').read_text()
with tempfile.TemporaryDirectory(prefix='rocky-migration-fixture-') as directory:
    root = Path(directory)
    state = root / 'state'
    state.mkdir()
    fixtures = root / 'bin'
    fixtures.mkdir()
    log = root / 'calls'
    conf = root / 'config'
    conf.write_text("iface=enp0s5\nold=192.0.2.120\nlab=198.51.100.120\n"
                    "recovery=203.0.113.120\nula=fd00::120\nshared_gateway=192.0.2.2\n")
    script = root / 'migrate'
    script.write_text(source.replace('/var/lib/gonf-network-migration', str(state))
                      .replace('/etc/gonf-network-migration.conf', str(conf))
                      .replace('export PATH=/usr/sbin:/usr/bin:/sbin:/bin',
                               f'export PATH={fixtures}:/usr/bin:/bin')
                      .replace('[[ $EUID == 0 ]] || exit 1', ': # fixture identity')
                      .replace('/etc/NetworkManager/system-connections', str(root) + '/profiles')
                      .replace('/etc/rancher/k3s/config.yaml.d', str(root) + '/dropins')
                      .replace('/usr/local/sbin/gonf-network-migration', str(script))
                      .replace('/usr/local/sbin/gonf-endpoint-rollback', str(fixtures / 'endpoint')))
    # All network/service commands are replaced, with no host network access.
    fake = '''#!/usr/bin/env bash
set -eu
printf '%s %s\\n' "${0##*/}" "$*" >> "$FIXTURE_LOG"
case ${0##*/} in
 stat) printf '0:600\\n';;
 sysctl) [[ $* != '-n '* ]] || printf '1\\n';;
 nmcli)
   case "$*" in
    '-g GENERAL.CON-UUID device show enp0s5') printf '00000000-0000-0000-0000-000000000000\\n';;
    '-g IP4.GATEWAY device show enp0s5') printf '192.0.2.1\\n';;
    '-g IP4.DNS device show enp0s5') printf '192.0.2.53\\n';;
   esac;;
 ip)
   if [[ $* == '-4 -o address show dev enp0s5' ]]; then
     printf '2: enp0s5 inet 192.0.2.120/24 scope global\\n'
     printf '2: enp0s5 inet 198.51.100.120/24 scope global\\n'
   fi;;
 endpoint) [[ ! -f "$FIXTURE_FAIL" ]];;
esac
'''
    for name in ('nmcli', 'ip', 'sysctl', 'systemctl', 'systemd-run',
                 'restorecon', 'stat', 'endpoint'):
        command = fixtures / name
        command.write_text(fake)
        command.chmod(0o755)
    env = dict(os.environ, FIXTURE_LOG=str(log), FIXTURE_FAIL=str(root / 'fail'),
               FIXTURE_ROOT=str(root))

    def run(*args, success=True):
        result = subprocess.run(['bash', str(script), *args], env=env,
                                text=True, capture_output=True)
        assert (result.returncode == 0) == success, (args, result.stderr)
        return result.stdout

    pending = state / 'pending'

    def journal(expired=False, endpoint=False):
        pending.mkdir()
        (pending / 'deadline').write_text('1' if expired else '9999999999')
        (pending / 'uuid').write_text('00000000-0000-0000-0000-000000000000')
        (root / 'profiles').mkdir(exist_ok=True)
        (root / 'dropins').mkdir(exist_ok=True)
        (pending / 'profile-path').write_text(str(root / 'profiles' / 'test.nmconnection'))
        (pending / 'profile').write_text('original physical profile')
        (pending / 'accept-ra').write_text('1')
        for name in ('address4', 'address6', 'route4', 'route6'):
            (pending / name).write_text('private runtime snapshot')
        if endpoint:
            (pending / 'endpoints-required').touch()

    run('stage', 'shared', success=False)
    assert not log.exists(), 'Unapproved activation ran an OS command'
    journal(expired=True)
    run('confirm', success=False)
    assert pending.exists(), 'Expired confirmation cleared recovery'
    run('watchdog')
    # Model systemd-run's startup semantics using a real subprocess: the
    # worker gets fresh descriptors and must acquire the request's real flock.
    # A blocking Type=oneshot client waits for completion and deadlocks here.
    (fixtures / 'systemd-run').write_text('''#!/usr/bin/env python3
import os, subprocess, sys
from pathlib import Path
root = Path(os.environ['FIXTURE_ROOT'])
args = sys.argv[1:]
worker = next(i for i, arg in enumerate(args) if not arg.startswith('--'))
code = """import subprocess, sys
from pathlib import Path
result = subprocess.run(['bash', *sys.argv[2:]])
Path(sys.argv[1]).write_text(str(result.returncode))
"""
with (root / 'worker-log').open('w') as output:
    child = subprocess.Popen([sys.executable, '-c', code,
        str(root / 'worker-result'), *args[worker:]], close_fds=True,
        start_new_session=True, stdout=output, stderr=output)
if '--no-block' not in args:
    try:
        sys.exit(child.wait(timeout=1))
    except subprocess.TimeoutExpired:
        child.kill()
        child.wait()
        sys.exit(124)
''')
    # Negatively prove the fixture catches the exact previous deadlock.
    fixed = script.read_text()
    script.write_text(fixed.replace('systemd-run --no-block', 'systemd-run'))
    blocked = subprocess.run(['bash', str(script), 'request', 'rollback'],
                             env=env, capture_output=True, timeout=3)
    assert blocked.returncode == 124, 'Old request failed without reproducing the startup timeout'
    assert not (root / 'worker-result').exists()
    script.write_text(fixed)
    run('request', 'rollback')
    for _ in range(100):
        if (root / 'worker-result').exists():
            break
        time.sleep(0.02)
    assert (root / 'worker-result').read_text() == '0', 'Queued worker did not acquire flock'

    def request(*args):
        (root / 'worker-result').unlink(missing_ok=True)
        run('request', *args)
        for _ in range(100):
            if (root / 'worker-result').exists():
                break
            time.sleep(0.02)
        assert (root / 'worker-result').read_text() == '0', (args, (root / 'worker-log').read_text())

    # Stateful NM route fixture: reads current routes and models list replace
    # versus append. Preserve a route with metric and attributes byte-for-byte.
    routes_file = root / 'routes.json'
    unrelated = '203.0.113.0/24 192.0.2.9 77 table=200 onlink=true'
    routes_file.write_text(json.dumps([unrelated]))
    (fixtures / 'nmcli').write_text('''#!/usr/bin/env python3
import ipaddress, json, os, sys
from pathlib import Path
root = Path(os.environ['FIXTURE_ROOT'])
args = sys.argv[1:]
with Path(os.environ['FIXTURE_LOG']).open('a') as log:
    log.write('nmcli ' + ' '.join(args) + '\\n')
routes_file = root / 'routes.json'
routes = json.loads(routes_file.read_text())
if args[:2] == ['--escape', 'no']:
    print(', '.join(routes))
elif args[:2] == ['-g', 'GENERAL.CON-UUID']:
    print('00000000-0000-0000-0000-000000000000')
elif args[:2] == ['-g', 'IP4.GATEWAY']:
    print('192.0.2.1')
elif args[:2] == ['-g', 'IP4.DNS']:
    # Actual NM runtime output may combine a display-delimited line and rows.
    print('192.0.2.53 | 192.0.2.54\\n192.0.2.55')
elif args[:2] == ['connection', 'modify']:
    if 'ipv4.dns' in args:
        for server in args[args.index('ipv4.dns') + 1].split(','):
            ipaddress.IPv4Address(server)
    for key in ('ipv4.routes', '+ipv4.routes'):
        if key in args:
            value = args[args.index(key) + 1]
            new = [r.strip() for r in value.split(',') if r.strip()]
            routes = routes + new if key.startswith('+') else new
            routes_file.write_text(json.dumps(routes))
''')
    assert not pending.exists()
    assert (root / 'profiles' / 'test.nmconnection').read_text() == 'original physical profile'
    calls = log.read_text()
    assert 'ip -6 address flush dev enp0s5 scope global' in calls
    assert 'wg0' not in calls
    run('rollback')  # idempotent
    journal(endpoint=True)
    (root / 'fail').touch()
    run('restore-boot', success=False)
    assert pending.exists() and not (pending / 'boot-restored').exists()
    (root / 'fail').unlink()
    run('restore-boot')
    assert (pending / 'boot-restored').exists()
    run('restore-after-boot')
    assert not pending.exists()
    journal(endpoint=True)
    (state / 'confirm-approved').touch()
    (root / 'fail').touch()
    run('confirm', success=False)
    assert pending.exists(), 'Endpoint failure cleared recovery'
    (root / 'fail').unlink()
    run('confirm')
    assert not pending.exists() and not (state / 'confirm-approved').exists()
    run('watchdog')
    # Exercise real snapshot and activation logic with fixture OS output.
    profile = root / 'profiles' / 'test.nmconnection'
    profile.write_text('uuid=00000000-0000-0000-0000-000000000000\n')
    for name in ('activation-approved', 'stage-approved'):
        (state / name).touch()
    request('stage', 'shared')
    assert pending.exists()
    calls = log.read_text()
    assert 'ipv4.addresses 192.0.2.120/24,198.51.100.120/24' in calls
    assert 'ipv4.dns 192.0.2.53,192.0.2.54,192.0.2.55' in calls
    assert json.loads(routes_file.read_text()) == [unrelated, '10.99.0.2/32 192.0.2.2']
    assert '10.98.0.1' not in calls
    assert 'ipv6.method manual' in calls and 'ipv6.gateway  ' in calls
    assert 'systemctl restart k3s' not in calls
    (state / 'confirm-approved').touch()
    run('confirm')
    (state / 'promote-approved').touch()
    (state / 'serial-k3s-approved').write_text(subprocess.check_output(
        ['hostname', '-s'], text=True).strip())
    request('promote', 'shared')
    calls = log.read_text()
    assert 'ipv4.addresses 198.51.100.120/24,192.0.2.120/24' in calls
    assert 'ipv4.gateway 10.20.0.1' in calls
    assert calls.count('systemctl restart k3s') == 1
    request('rollback')
    assert not pending.exists()
    assert log.read_text().count('systemctl restart k3s') == 2
    # Shared acceptance followed by the later explicitly gated isolated stage.
    # Include duplicate stale owned /32s with distinct metrics to prove exact
    # destination ownership rather than one hardcoded gateway/value deletion.
    shared_routes = [unrelated, '10.99.0.2/32 192.0.2.2',
                     '10.99.0.2/32 192.0.2.2 99']
    (state / 'stage-approved').touch()
    (state / 'igc1-approved').touch()
    routes_file.write_text(json.dumps(shared_routes))
    # Old append behavior must fail the same post-transition assertion.
    bad = fixed.replace('ipv4.routes "$return_routes"',
                        '+ipv4.routes "10.99.0.2/32 $return_gateway"')
    script.write_text(bad)
    run('stage', 'isolated')
    def assert_isolated_routes():
        assert json.loads(routes_file.read_text()) == [unrelated, '10.99.0.2/32 10.98.0.1']

    try:
        assert_isolated_routes()
    except AssertionError:
        pass  # The exact fixed-code assertion rejects the original append.
    else:
        raise AssertionError('Route regression check did not reject old append behavior')
    run('rollback')
    script.write_text(fixed)
    routes_file.write_text(json.dumps(shared_routes))
    (state / 'stage-approved').touch()
    run('stage', 'isolated')
    assert_isolated_routes()
    run('rollback')
    # Standalone policy explicitly has no ULA, no k3s/drop-in writes and
    # preserves the same timer/boot/endpoint transaction contracts.
    conf.write_text(conf.read_text().replace('ula=fd00::120', 'ula=') +
                    'role=standalone\nipv6_policy=unallocated\n')
    log.write_text('')
    run('stage', 'shared', success=False)
    assert not log.read_text(), 'Unknown IPv6 policy ran OS commands'
    conf.write_text(conf.read_text().replace('unallocated', 'disabled'))
    dropin = root / 'dropins' / '60-network-migration.yaml'
    dropin.write_text('unrelated preexisting content')
    (state / 'stage-approved').touch()
    request('stage', 'shared')
    assert (pending / 'standalone').exists()
    assert not (pending / 'flannel').exists()
    assert 'ipv6.method disabled' in log.read_text()
    assert 'ip -6 address add' not in log.read_text()
    (state / 'confirm-approved').touch()
    run('confirm')
    (state / 'promote-approved').touch()
    request('promote', 'shared')  # No serial-k3s-approved gate exists.
    run('restore-boot')
    run('restore-after-boot')
    assert dropin.read_text() == 'unrelated preexisting content'
    assert 'k3s' not in log.read_text()
    assert 'wg0' not in log.read_text()
    print('PASS: request flock dispatch (negative old deadlock), owned /32 replacement '
          '(negative old append), stage/promote, expiry/boot/endpoint/confirm recovery')
