#!/usr/bin/env python3
"""Run the real dispatch/state code with disposable OS command fixtures only."""
from pathlib import Path
import os
import subprocess
import tempfile

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
    env = dict(os.environ, FIXTURE_LOG=str(log), FIXTURE_FAIL=str(root / 'fail'))

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
    run('stage', 'shared')
    assert pending.exists()
    calls = log.read_text()
    assert 'ipv4.addresses 192.0.2.120/24,198.51.100.120/24' in calls
    assert '+ipv4.routes 10.99.0.2/32 192.0.2.2' in calls
    assert '10.98.0.1' not in calls
    assert 'ipv6.method manual' in calls and 'ipv6.gateway  ' in calls
    assert 'systemctl restart k3s' not in calls
    (state / 'confirm-approved').touch()
    run('confirm')
    (state / 'promote-approved').touch()
    (state / 'serial-k3s-approved').write_text(subprocess.check_output(
        ['hostname', '-s'], text=True).strip())
    run('promote', 'shared')
    calls = log.read_text()
    assert 'ipv4.addresses 198.51.100.120/24,192.0.2.120/24' in calls
    assert 'ipv4.gateway 10.20.0.1' in calls
    assert calls.count('systemctl restart k3s') == 1
    run('rollback')
    assert not pending.exists()
    assert log.read_text().count('systemctl restart k3s') == 2
    print('PASS: closed activation, expiry, idempotent rollback, boot ordering, endpoint failures, confirm')
