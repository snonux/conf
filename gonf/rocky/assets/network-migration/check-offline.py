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
                      .replace('/usr/local/sbin/gonf-endpoint-rollback', str(fixtures / 'endpoint'))
                      .replace('/sys/class/net/wg0', str(root / 'wg0')))
    script.write_text(script.read_text()
                      .replace('/home/paul/pihole', str(root / 'resolver'))
                      .replace('/var/lib/f3s-pihole-migration', str(root / 'resolver-candidate'))
                      .replace('install -o paul -g paul -m 644', 'install -m 644'))
    # All network/service commands are replaced, with no host network access.
    fake = '''#!/usr/bin/env bash
set -eu
printf '%s %s\\n' "${0##*/}" "$*" >> "$FIXTURE_LOG"
case ${0##*/} in
 stat)
   if [[ $* == *k3s-maintenance-approved ]]; then
     printf '%s\\n' "${FIXTURE_MAINTENANCE_GATE_STAT:-0:600}"
   else printf '0:600\\n'; fi;;
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

    # Resolver hosts use their physical eth0 with ULA, while retaining the
    # existing non-k3s journal/boot path. Exercise the actual dispatch code.
    standalone_conf = conf.read_text()
    original_ip_fixture = (fixtures / 'ip').read_text()
    conf.write_text(standalone_conf.replace('iface=enp0s5', 'iface=eth0')
                    .replace('ula=\n', 'ula=fd00::120\n')
                    .replace('role=standalone', 'role=resolver'))
    log.write_text('')
    run('stage', 'shared', success=False)  # Disabled IPv6 is not resolver policy.
    assert not log.read_text()
    conf.write_text(conf.read_text().replace('ipv6_policy=disabled', 'ipv6_policy=ula'))
    (fixtures / 'ip').write_text(original_ip_fixture.replace('enp0s5', 'eth0'))
    assert not (state / 'serial-k3s-approved').exists()
    (state / 'stage-approved').touch()
    request('stage', 'shared')
    assert (pending / 'standalone').exists() and not (pending / 'flannel').exists()
    assert 'ipv6.method manual ipv6.addresses fd00::120/64' in log.read_text()
    assert 'ip -6 address add fd00::120/64 dev eth0' in log.read_text()
    (pending / 'deadline').write_text('1')  # Disposable clock fixture only.
    run('watchdog')
    assert not pending.exists() and dropin.read_text() == 'unrelated preexisting content'
    (state / 'stage-approved').touch()
    request('stage', 'shared')
    (state / 'confirm-approved').touch()
    run('confirm')
    (state / 'promote-approved').touch()
    request('promote', 'shared')
    assert 'ipv4.addresses 198.51.100.120/24,192.0.2.120/24' in log.read_text()
    run('restore-boot')
    run('restore-after-boot')
    assert not pending.exists() and dropin.read_text() == 'unrelated preexisting content'
    assert 'k3s' not in log.read_text() and 'wg0' not in log.read_text()

    # A resolver service change shares the existing network journal and
    # watchdog. FTL's shutdown write must not overwrite restored TOML.
    resolver = root / 'resolver'
    (resolver / 'etc-pihole').mkdir(parents=True)
    candidate_dir = root / 'resolver-candidate'
    candidate_dir.mkdir()
    compose = resolver / 'docker-compose.yml'
    toml = resolver / 'etc-pihole' / 'pihole.toml'
    compose.write_text('original compose')
    toml.write_text('original private TOML')
    (candidate_dir / 'docker-compose.yml').write_text('desired compose')
    (fixtures / 'docker').write_text('''#!/usr/bin/env python3
import os, sys
from pathlib import Path
root = Path(os.environ['FIXTURE_ROOT'])
with Path(os.environ['FIXTURE_LOG']).open('a') as f:
    f.write('docker ' + ' '.join(sys.argv[1:]) + '\\n')
toml = root / 'resolver/etc-pihole/pihole.toml'
if sys.argv[1:3] == ['compose', 'stop']:
    assert sys.argv[3:] == ['--timeout', '10']
    toml.write_text('FTL writes at shutdown')
else:
    assert sys.argv[1:] == ['compose', 'up', '-d', '--timeout', '10', '--wait', '--wait-timeout', '90']
    if (root / 'resolver/docker-compose.yml').read_text() == 'desired compose':
        toml.write_text('desired TOML')
    if (root / 'docker-failure').exists() or (root / 'docker-unhealthy').exists():
        sys.exit(1)
''')
    (fixtures / 'docker').chmod(0o755)
    (state / 'stage-approved').touch()
    request('stage', 'shared')
    assert compose.read_text() == 'desired compose'
    assert toml.read_text() == 'desired TOML'
    assert (pending / 'resolver/compose').read_text() == 'original compose'
    (pending / 'deadline').write_text('1')
    run('watchdog')
    assert compose.read_text() == 'original compose'
    assert toml.read_text() == 'original private TOML'
    (state / 'stage-approved').touch()
    request('stage', 'shared')
    run('restore-boot')
    assert compose.read_text() == 'original compose'
    assert toml.read_text() == 'original private TOML'
    run('restore-after-boot')
    assert not pending.exists() and toml.read_text() == 'original private TOML'
    # Failed desired recreation retains a complete recovery journal.
    (root / 'docker-failure').touch()
    (state / 'stage-approved').touch()
    run('stage', 'shared', success=False)
    assert pending.exists()
    (root / 'docker-failure').unlink()
    (root / 'docker-unhealthy').touch()
    run('rollback', success=False)
    assert pending.exists(), 'Unhealthy restored service cleared recovery journal'
    (root / 'docker-unhealthy').unlink()
    run('rollback')
    assert not pending.exists() and compose.read_text() == 'original compose'
    assert toml.read_text() == 'original private TOML'
    print('PASS: resolver service timed/boot/failure restore preserves compose '
          'and private TOML after FTL shutdown, with the same watchdog')
    conf.write_text(standalone_conf)
    (fixtures / 'ip').write_text(original_ip_fixture)
    print('PASS: resolver eth0/ULA stage, expiry, promotion and boot restore '
          'never operate k3s, Flannel or absent WireGuard')
    print('PASS: request flock dispatch (negative old deadlock), owned /32 replacement '
          '(negative old append), stage/promote, expiry/boot/endpoint/confirm recovery')

    # Synthetic sysfs only: never inspect or create a live controller wg0.
    wg_present = root / 'wg0'
    mark_file = root / 'fwmark'
    failure_file = root / 'wg-failure'
    mark_file.write_text('off')
    (fixtures / 'wg').write_text('''#!/usr/bin/env python3
import os, sys
from pathlib import Path
root = Path(os.environ['FIXTURE_ROOT'])
args = sys.argv[1:]
with Path(os.environ['FIXTURE_LOG']).open('a') as log:
    log.write('wg ' + ' '.join(args) + '\\n')
mark = root / 'fwmark'
failure = root / 'wg-failure'
mode = failure.read_text() if failure.exists() else ''
if args == ['show', 'wg0', 'fwmark']:
    if mode == 'query':
        sys.exit(7)
    print(mark.read_text())
elif args[:3] == ['set', 'wg0', 'fwmark'] and len(args) == 4:
    assert args[3] == mark.read_text(), 'Fixture refuses any changed mark'
    if mode == 'set':
        sys.exit(8)
    if mode == 'verify':
        mark.write_text('0x42')  # Simulate an unexpected concurrent/tool change.
else:
    raise AssertionError('Unreviewed WireGuard mutation or query')
''')
    (fixtures / 'wg').chmod(0o755)

    def wg_calls():
        return [line for line in log.read_text().splitlines()
                if line.startswith('wg ')]

    def recovery_journal(expired=False, endpoint=False):
        journal(expired=expired, endpoint=endpoint)
        (pending / 'standalone').touch()

    # No interface at early boot: physical recovery still completes without
    # running a WireGuard command, even if the tools executable exists.
    log.write_text('')
    recovery_journal(expired=True)
    run('watchdog')
    assert not pending.exists() and not wg_calls()

    wg_present.mkdir()
    # Stage activation retains its old-primary contract and does not refresh.
    profile.write_text('uuid=00000000-0000-0000-0000-000000000000\n')
    (state / 'stage-approved').touch()
    log.write_text('')
    run('stage', 'shared')
    assert pending.exists() and not wg_calls()
    run('rollback')
    assert wg_calls() == ['wg show wg0 fwmark', 'wg set wg0 fwmark off',
                          'wg show wg0 fwmark']

    # Real request/dispatch promotion and complete OLD runtime restore both
    # reapply exactly the original metadata, including a nonzero uint32 mark.
    for mark in ('off', '0x1234', '0xffffffff'):
        mark_file.write_text(mark)
        profile.write_text('uuid=00000000-0000-0000-0000-000000000000\n')
        (state / 'promote-approved').touch()
        log.write_text('')
        request('promote', 'shared')
        assert pending.exists() and mark_file.read_text() == mark
        assert wg_calls() == ['wg show wg0 fwmark', 'wg set wg0 fwmark ' + mark,
                              'wg show wg0 fwmark']
        calls = log.read_text().splitlines()
        assert calls.index('ip -6 route flush dev enp0s5 proto ra') < calls.index(
            'wg show wg0 fwmark')
        log.write_text('')
        request('rollback')
        assert not pending.exists() and mark_file.read_text() == mark
        assert wg_calls() == ['wg show wg0 fwmark', 'wg set wg0 fwmark ' + mark,
                              'wg show wg0 fwmark']
        calls = log.read_text().splitlines()
        assert calls.index('ip -6 route restore') < calls.index('wg show wg0 fwmark')
        before_repeat = log.read_text()
        run('rollback')
        assert log.read_text() == before_repeat, 'Repeated rollback changed WG metadata'

    # Both native expiry and boot-runtime recovery share the same helper path.
    mark_file.write_text('off')
    log.write_text('')
    recovery_journal(expired=True)
    run('watchdog')
    assert not pending.exists() and 'wg set wg0 fwmark off' in wg_calls()
    log.write_text('')
    recovery_journal(endpoint=True)
    run('restore-boot')
    assert (pending / 'boot-restored').exists() and not wg_calls()
    run('restore-after-boot')
    assert not pending.exists()
    calls = log.read_text().splitlines()
    assert next(i for i, line in enumerate(calls) if line.startswith('endpoint restore-runtime ')) < calls.index('wg show wg0 fwmark')
    before_repeat = log.read_text()
    run('restore-after-boot')
    run('rollback')
    assert log.read_text() == before_repeat

    # Invalid metadata and either command failure must retain recovery, never
    # archive an incomplete restore or resume a marked k3s service too early.
    for mark in ('', '0', '0x100000000', '0xG', 'off extra'):
        recovery_journal()
        mark_file.write_text(mark)
        log.write_text('')
        run('rollback', success=False)
        assert pending.exists() and not any(line.startswith('wg set ') for line in wg_calls())
        mark_file.write_text('off')
        run('rollback')
        assert not pending.exists()
    for failure in ('query', 'set', 'verify'):
        recovery_journal()
        (pending / 'k3s-restarted').touch()
        mark_file.write_text('off')
        failure_file.write_text(failure)
        log.write_text('')
        run('rollback', success=False)
        assert pending.exists() and 'systemctl restart k3s' not in log.read_text()
        failure_file.unlink()
        mark_file.write_text('off')
        log.write_text('')
        run('rollback')
        assert not pending.exists()
        calls = log.read_text().splitlines()
        assert calls.index('wg set wg0 fwmark off') < calls.index('systemctl restart k3s')

    # A failed refresh during promotion also keeps the complete journal and
    # its unconsumed approval. Recovery can retry after the metadata tool works.
    profile.write_text('uuid=00000000-0000-0000-0000-000000000000\n')
    (state / 'promote-approved').touch()
    failure_file.write_text('set')
    log.write_text('')
    run('promote', 'shared', success=False)
    assert pending.exists() and (state / 'promote-approved').exists()
    failure_file.unlink()
    run('rollback')
    assert not pending.exists()
    assert not any('wg-quick' in line or 'wireguard' in line
                   for line in log.read_text().splitlines() if line.startswith('systemctl '))
    print('PASS: same-value fwmark source-cache refresh: absent/off/nonzero, '
          'stage unchanged, promote/expiry/boot restore, idempotent rollback, '
          'invalid/query/set/verify failures retain journal; no other WG mutation/restart')

    # Full k3s maintenance is explicit and names this node. Network rollback
    # must still finish while the service intentionally remains stopped.
    conf.write_text("iface=enp0s5\nold=192.0.2.120\nlab=198.51.100.120\n"
                    "recovery=203.0.113.120\nula=fd00::120\nshared_gateway=192.0.2.2\nrole=k3s\n")
    service_state = root / 'k3s-state'
    service_state.write_text('inactive')
    (fixtures / 'systemctl').write_text('''#!/usr/bin/env python3
import os, sys
from pathlib import Path
root = Path(os.environ['FIXTURE_ROOT'])
args = sys.argv[1:]
with Path(os.environ['FIXTURE_LOG']).open('a') as log:
    log.write('systemctl ' + ' '.join(args) + '\\n')
state = (root / 'k3s-state').read_text()
if args == ['show', 'k3s', '--property=ActiveState', '--value']:
    print(state)
elif args == ['is-active', '--quiet', 'k3s']:
    sys.exit(0 if state == 'active' else 3)
elif args == ['restart', 'k3s']:
    (root / 'k3s-state').write_text('active')
''')
    node = subprocess.check_output(['hostname', '-s'], text=True).strip()
    maintenance = state / 'k3s-maintenance-approved'
    def arm():
        profile.write_text('uuid=00000000-0000-0000-0000-000000000000\n')
        (state / 'promote-approved').touch()
        (state / 'serial-k3s-approved').write_text(node)
        log.write_text('')

    arm()
    run('promote', 'shared', success=False)
    assert not pending.exists() and 'nmcli connection modify' not in log.read_text()
    maintenance.write_text('another.example')
    arm()
    run('promote', 'shared', success=False)
    assert not pending.exists() and 'nmcli connection modify' not in log.read_text()
    maintenance.unlink()
    maintenance.symlink_to(state / 'activation-approved')
    arm()
    run('promote', 'shared', success=False)
    assert not pending.exists()
    maintenance.unlink()
    maintenance.write_text(node)
    for metadata in ('0:644', '1000:600'):
        env['FIXTURE_MAINTENANCE_GATE_STAT'] = metadata
        arm()
        run('promote', 'shared', success=False)
        assert not pending.exists()
    env.pop('FIXTURE_MAINTENANCE_GATE_STAT')
    service_state.write_text('active')
    arm()
    run('promote', 'shared', success=False)
    assert not pending.exists(), 'Running service accepted as maintenance'
    service_state.write_text('inactive')
    original_dropin = dropin.read_text()
    arm()
    run('promote', 'shared')
    assert pending.exists() and not (pending / 'k3s-restarted').exists()
    assert 'ipv4.gateway 10.20.0.1' in log.read_text()
    assert dropin.read_text() == 'flannel-iface: enp0s5\n'
    assert 'systemctl restart k3s' not in log.read_text()
    run('rollback')
    assert not pending.exists() and dropin.read_text() == original_dropin
    assert service_state.read_text() == 'inactive'
    assert 'systemctl restart k3s' not in log.read_text()
    # A maintenance gate added after a prior live transaction also suppresses
    # its recorded restart during both ordinary and boot-runtime recovery.
    for mode in ('rollback', 'watchdog', 'restore-after-boot'):
        journal(expired=True)
        (pending / 'k3s-restarted').touch()
        log.write_text('')
        if mode == 'restore-after-boot':run('restore-boot')
        run(mode)
        assert not pending.exists() and service_state.read_text() == 'inactive'
        assert 'systemctl restart k3s' not in log.read_text()
    maintenance.unlink()
    service_state.write_text('active')
    arm()
    run('promote', 'shared')
    assert (pending / 'k3s-restarted').exists()
    run('rollback')
    assert not pending.exists() and log.read_text().count('systemctl restart k3s') == 2
    print('PASS: real maintenance dispatch/rollback/expiry/boot preserve stopped k3s; '
          'missing/foreign/symlink/running gates refuse; normal restart retained')
