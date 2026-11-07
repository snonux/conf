#!/usr/bin/env python3
"""Exercise the real shell dispatcher and files with mocked OS service boundaries."""
import os
import fcntl
from pathlib import Path
import subprocess
import tempfile
import time

ROOT = Path(__file__).resolve().parents[2]
SOURCE = (ROOT / 'gonf/endpointfiles/assets/guard').read_text()
MERGER = (ROOT / 'gonf/fleetfiles/assets/merge-hosts').read_text()


def put(path, value, mode=0o600):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(value)
    path.chmod(mode)


with tempfile.TemporaryDirectory(prefix='conf-endpoint-test-') as directory:
    base = Path(directory)
    fake = base / 'bin'
    state = base / 'state'
    fake.mkdir()
    node = base / 'node'
    platform = base / 'platform'
    put(node, 'f0\n')
    put(platform, 'Linux\n')
    for command, body in {
        'hostname': f'cat {node}',
        'uname': f'cat {platform}',
        'id': 'echo 0',
        'sync': ':',
        'restorecon': ':',
        'cp': f'''test ! -e {base}/fail-copy || exit 1
if [ -e {base}/pause-copy ]; then
    : > {base}/copy-paused
    while [ -e {base}/pause-copy ]; do sleep 0.02; done
fi
exec /bin/cp "$@"''',
        'systemctl': f'''case "$1" in
is-active)
    if [ -f {base}/delayed-active ]; then
        remaining=$(cat {base}/delayed-active)
        if [ "$remaining" -gt 0 ]; then
            echo $((remaining-1)) > {base}/delayed-active
            exit 1
        fi
    fi
    test ! -e {base}/inactive ;;
restart)
    test ! -e {base}/fail-runtime || exit 1
    if [ -f {base}/delay-after-restart ]; then echo 2 > {base}/delayed-active; fi
    cat {base}/local/etc/apcupsd/apcupsd.conf > {base}/runtime
    echo restarted >> {base}/calls ;;
*) exit 2 ;;
esac''',
        'docker': f'''case "$1:$2" in
inspect:*) echo healthy ;;
compose:stop) echo daemon-shutdown-write > {base}/pihole/etc-pihole/pihole.toml ;;
compose:up) test ! -e {base}/fail-runtime ;;
*) exit 2 ;;
esac''',
        'pfctl': f'''case "$1" in
-s) test ! -e {base}/inactive ;;
-nf) test -s "$2" ;;
-f) test ! -e {base}/fail-runtime && cat "$2" > {base}/pf-runtime ;;
*) exit 2 ;;
esac''',
    }.items():
        put(fake / command, '#!/bin/sh\nset -eu\n' + body + '\n', 0o755)
    helper = base / 'guard'
    merge = base / 'merge'
    script = SOURCE.replace('PATH=/usr/local/sbin:', f'PATH={fake}:/usr/local/sbin:')
    replacements = {
        '/var/lib/gonf-endpoint-migration': str(state),
        '/var/db/gonf-endpoint-migration': str(state),
        '/usr/local/sbin/gonf-endpoint-migration': str(helper),
        '/usr/local/sbin/gonf-fleet-hosts': str(merge),
        '/usr/local/etc/gonf-fleet-hosts': str(base / 'fleet'),
        '/usr/local/etc/': str(base / 'local/etc') + '/',
        '/usr/local/sbin/shelly-fans-on': str(base / 'fan-helper'),
        '/var/lib/f3s-pihole-migration': str(base / 'dns-stage'),
        '/home/paul/pihole': str(base / 'pihole'),
        '/etc/hosts': str(base / 'etc/hosts'),
        '/etc/pf.conf': str(base / 'etc/pf.conf'),
        '/var/db/gonf-pf.sha256': str(base / 'pf-digest'),
        '/etc/rc': str(base / 'etc/rc'),
        '/etc/ssh/authorized_keys.d/f3sctl': str(base / 'authorized-key'),
        '/bin/ksh': '/bin/sh',
    }
    for old, new in replacements.items():
        script = script.replace(old, new)
    put(helper, script, 0o755)
    put(merge, MERGER.replace('target=/etc/hosts', 'target=' + str(base / 'etc/hosts')), 0o755)

    def run(*args, ok=True):
        result = subprocess.run([str(helper), *args], text=True, capture_output=True, timeout=8)
        assert (result.returncode == 0) == ok, (args, result.returncode, result.stdout, result.stderr)
        return result

    def expire():
        put(state / 'pending/deadline', '0\n')

    # Exercise the actual kernel lock on all dispatcher branches. An empty
    # lock file is valid: no PID publication/reaping window exists.
    run('status')
    lock_path = state / 'lock'
    inode = lock_path.stat().st_ino
    for name in ('Linux', 'NetBSD', 'FreeBSD', 'OpenBSD'):
        put(platform, name + '\n')
        with lock_path.open('a') as holder:
            fcntl.flock(holder, fcntl.LOCK_EX | fcntl.LOCK_NB)
            run('status', ok=False)
        run('status')
        assert lock_path.stat().st_ino == inode
    put(platform, 'Linux\n')

    hosts = base / 'etc/hosts'
    initial = '127.0.0.1 localhost f0\n192.168.1.130 f0 f0.lan unrelated\n# local comment\n192.0.2.7 extra\n'
    put(hosts, initial, 0o644)
    put(base / 'fleet', '10.20.0.130 f0 f0.lan\n192.168.1.130 f0.old.lan\n')
    for name in ('Linux', 'NetBSD', 'FreeBSD', 'OpenBSD'):
        put(platform, name + '\n')
        put(base / 'pause-copy', '')
        process = subprocess.Popen([str(helper), 'arm', 'hosts', '60'], stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        try:
            limit = time.monotonic() + 3
            while not (base / 'copy-paused').exists():
                assert process.poll() is None and time.monotonic() < limit
                time.sleep(0.01)
            run('status', ok=False)
        finally:
            (base / 'pause-copy').unlink()
        stdout, stderr = process.communicate(timeout=8)
        assert process.returncode == 0, (stdout, stderr)
        (base / 'copy-paused').unlink()
        run('rollback')
    put(platform, 'Linux\n')
    run('apply', 'hosts', ok=False)
    assert hosts.read_text() == initial
    run('arm', 'hosts', '60')
    run('arm', 'hosts', '60', ok=False)
    run('apply', 'hosts')
    assert '192.168.1.130 unrelated' in hosts.read_text()
    assert '127.0.0.1 localhost\n' in hosts.read_text()
    assert '192.0.2.7 extra' in hosts.read_text()
    expire()
    run('confirm', 'hosts', ok=False)
    run('watchdog')
    assert hosts.read_text() == initial and not (state / 'pending').exists()
    assert hosts.stat().st_mode & 0o777 == 0o644
    run('rollback')

    run('arm', 'hosts', '60')
    run('apply', 'hosts')
    put(base / 'fail-copy', '')
    run('rollback', ok=False)
    assert (state / 'pending/restoring').exists()
    (base / 'fail-copy').unlink()
    run('rollback')
    assert hosts.read_text() == initial and not (state / 'pending').exists()

    run('arm', 'hosts', '60')
    run('apply', 'hosts')
    run('boot')
    assert hosts.read_text() == initial and (state / 'pending/restoring').exists()
    run('watchdog')
    assert not (state / 'pending').exists()
    run('arm', 'hosts', '60')
    run('apply', 'hosts')
    run('confirm', 'hosts')
    run('apply', 'hosts')
    assert (state / 'accepted-hosts').is_file()

    ups = base / 'local/etc/apcupsd/apcupsd.conf'
    put(ups, 'OLD endpoint\n', 0o640)
    put(Path(str(ups) + '.triggerfish-candidate'), 'NEW endpoint\n')
    run('arm', 'ups', '900')
    put(base / 'delay-after-restart', '')
    run('apply', 'ups')
    assert (base / 'delayed-active').read_text().strip() == '0'
    (base / 'delay-after-restart').unlink()
    (base / 'delayed-active').unlink()
    assert (base / 'runtime').read_text() == 'NEW endpoint\n'
    put(base / 'fail-runtime', '')
    expire()
    run('watchdog', ok=False)
    assert ups.read_text() == 'OLD endpoint\n' and (state / 'pending/restoring').is_file()
    (base / 'fail-runtime').unlink()
    run('watchdog')
    assert not (state / 'pending').exists()
    assert (base / 'runtime').read_text() == 'OLD endpoint\n'
    assert ups.stat().st_mode & 0o777 == 0o640

    put(base / 'inactive', '')
    run('arm', 'ups', '900', ok=False)
    assert not (state / 'preparing').exists()
    (base / 'inactive').unlink()
    run('arm', 'ups', '900')
    run('boot')
    assert (state / 'pending').exists()
    run('watchdog')
    run('arm', 'controller', '900', ok=False)
    run('arm', 'ups', 'bad', ok=False)

    # Root-owned state does not turn symlinks/special files into config files.
    ups.unlink()
    ups.symlink_to(base / 'runtime')
    run('arm', 'ups', '900', ok=False)
    assert not (state / 'preparing').exists()
    ups.unlink()
    os.mkfifo(ups)
    run('arm', 'ups', '900', ok=False)
    ups.unlink()

    # The existing Firewall owner publishes PF; the guard restores both files
    # and reloads the saved policy if a failed/expired trial needs recovery.
    put(node, 'pi0\n')
    run('arm', 'pf', '900', ok=False)
    put(node, 'f3\n')
    put(platform, 'FreeBSD\n')
    policy = base / 'etc/pf.conf'
    put(policy, 'OLD policy\n', 0o644)
    put(base / 'pf-digest', 'OLD checksum\n')
    run('arm', 'pf', '900')
    put(policy, 'NEW policy\n', 0o644)
    put(base / 'pf-digest', 'NEW checksum\n')
    run('apply', 'pf')
    assert (base / 'pf-runtime').read_text() == 'NEW policy\n'
    expire()
    put(base / 'fail-runtime', '')
    run('watchdog', ok=False)
    assert policy.read_text() == 'OLD policy\n' and (state / 'pending').exists()
    (base / 'fail-runtime').unlink()
    run('watchdog')
    assert not (state / 'pending').exists()
    assert (base / 'pf-runtime').read_text() == 'OLD policy\n'
    assert (base / 'pf-digest').read_text() == 'OLD checksum\n'

    # DNS rollback stops FTL before restoring its TOML and retains failed health.
    put(node, 'pi2\n')
    put(platform, 'Linux\n')
    for name in ('docker-compose.yml', 'etc-pihole/pihole.toml', 'etc-dnsmasq.d/99-f3s-lan-wildcard.conf'):
        put(base / 'pihole' / name, 'ORIGINAL ' + name + '\n')
    put(base / 'dns-stage/docker-compose.yml', 'NEW compose DNS directory enabled\n')
    put(base / 'dns-stage/99-f3s-lan-hosts.conf', 'host-record=test.example,10.20.0.130\n')
    put(base / 'dns-stage/99-f3s-lan-wildcard.conf', 'address=/.example/10.20.0.138\n')
    run('arm', 'dns', '900')
    run('apply', 'dns')
    assert (base / 'pihole/docker-compose.yml').read_text() == 'NEW compose DNS directory enabled\n'
    expire()
    put(base / 'fail-runtime', '')
    run('watchdog', ok=False)
    assert (state / 'pending').exists()
    assert (base / 'pihole/etc-pihole/pihole.toml').read_text() == 'ORIGINAL etc-pihole/pihole.toml\n'
    (base / 'fail-runtime').unlink()
    run('watchdog')
    assert not (state / 'pending').exists()
    assert (base / 'pihole/docker-compose.yml').read_text() == 'ORIGINAL docker-compose.yml\n'
    assert not (base / 'pihole/etc-dnsmasq.d/99-f3s-lan-hosts.conf').exists()

    # Native OpenBSD hook shape is syntax/ordering tested, not a reboot claim.
    put(node, 'blowfish\n')
    put(platform, 'OpenBSD\n')
    rc = base / 'etc/rc'
    put(rc, '#!/bin/sh\nmount -uw /\nrootdisk_nodes\nsysctl_conf\n', 0o755)
    run('install-openbsd-boot')
    installed = rc.read_text()
    run('install-openbsd-boot')
    assert rc.read_text() == installed and installed.index(' boot ||') < installed.index('rootdisk_nodes')
    put(rc, installed.replace(' boot || exit 1', ' status || exit 1'), 0o755)
    run('install-openbsd-boot', ok=False)

print('PASS real dispatcher: expiry/boot, modes, partial failure/retry, inactive/symlink/FIFO refusal, FTL stop race and exact hook guards')
