#!/usr/bin/python3
"""Guard existing r-node/earth stunnel endpoints; preserve TLS identity/mounts."""
import fcntl
import json
import os
from pathlib import Path
import stat
import subprocess
import sys
import time

STATE = Path('/var/lib/gonf-storage-client')
PENDING = STATE / 'pending'
CONFIG = Path('/etc/stunnel/stunnel.conf')
OLD = b'connect = 192.168.1.138:2323'
NEW = b'connect = 10.20.0.138:2323'
BOOT_ID = Path('/proc/sys/kernel/random/boot_id')


def run(args, *, required=True):
    result = subprocess.run(args, capture_output=True, timeout=60)
    if required and result.returncode:
        raise RuntimeError('native operation failed: ' + Path(args[0]).name)
    return result


def node():
    value = run(['/bin/hostname', '-s']).stdout.decode().strip()
    if value not in ('r0', 'r1', 'r2', 'earth'):
        raise RuntimeError('not an existing storage client')
    return value


def gate(path, *, fresh=False):
    info = path.lstat()
    if not stat.S_ISREG(info.st_mode) or info.st_uid != 0 or stat.S_IMODE(info.st_mode) != 0o600:
        raise RuntimeError('invalid node gate')
    if path.read_text().strip() != node():
        raise RuntimeError('gate names another node')
    if fresh and not 0 <= time.time() - info.st_mtime <= 120:
        raise RuntimeError('stale coordinator gate')


def write(path, content, mode=0o600):
    temporary = path.with_name(path.name + '.tmp')
    with temporary.open('wb') as stream:
        stream.write(content)
        stream.flush()
        os.fsync(stream.fileno())
    temporary.chmod(mode)
    temporary.replace(path)


def manifest():
    value = json.loads((PENDING / 'manifest.json').read_text())
    if value['node'] != node():
        raise RuntimeError('saved transaction names another node')
    return value


def maintenance():
    if node() == 'earth':
        for mount in ('/data/nfs/earthdata', '/data/nfs/k3svolumes'):
            if run(['/usr/bin/findmnt', '-rn', mount], required=False).returncode == 0:
                raise RuntimeError('release the laptop NFS mounts cleanly')
        return
    gate(Path('/var/lib/gonf-network-migration/k3s-maintenance-approved'))
    state = run(['/usr/bin/systemctl', 'show', 'k3s', '--property=ActiveState', '--value']).stdout.strip()
    if state != b'inactive':
        raise RuntimeError('k3s is not held inactive')
    for path in Path('/proc').glob('[0-9]*/cgroup'):
        try:
            content = path.read_text()
        except FileNotFoundError:
            continue
        if 'kubepods' in content:
            raise RuntimeError('application process still present')


def arm(seconds):
    gate(STATE / 'approved', fresh=True)
    maintenance()
    if PENDING.exists() or not 60 <= seconds <= 1800:
        raise RuntimeError('unfinished transaction or invalid deadline')
    info = CONFIG.lstat()
    if not stat.S_ISREG(info.st_mode) or info.st_uid != 0:
        raise RuntimeError('unexpected client config metadata')
    original = CONFIG.read_bytes()
    if original.count(OLD) != 1 or NEW in original:
        raise RuntimeError('unexpected original client endpoint')
    if run(['/usr/bin/systemctl', 'is-active', '--quiet', 'stunnel'], required=False).returncode != 0:
        raise RuntimeError('original transport is not active')
    temporary = STATE / 'preparing'
    temporary.mkdir(mode=0o700)
    write(temporary / 'config', original)
    value = {'node': node(), 'boot_id': BOOT_ID.read_text().strip(),
             'deadline': int(time.time()) + seconds, 'mode': stat.S_IMODE(info.st_mode),
             'uid': info.st_uid, 'gid': info.st_gid,
             'active': run(['/usr/bin/systemctl', 'is-active', '--quiet', 'stunnel'], required=False).returncode == 0}
    write(temporary / 'manifest.json', json.dumps(value).encode())
    temporary.rename(PENDING)


def stage():
    value = manifest()
    maintenance()
    gate(STATE / 'server-accepted', fresh=True)
    if time.time() >= value['deadline'] - 30:
        raise RuntimeError('insufficient transaction time')
    if node() != 'earth':
        for service in ('nfs-mount-monitor.timer', 'nfs-mount-monitor.service'):
            state = run(['/usr/bin/systemctl', 'show', service, '--property=ActiveState', '--value']).stdout.strip()
            if state != b'inactive':
                raise RuntimeError('NFS monitor is not paused')
    if run(['/usr/bin/findmnt', '-rn', '/data/nfs/k3svolumes'], required=False).returncode == 0:
        raise RuntimeError('release the NFS mount cleanly before changing transport')
    original = (PENDING / 'config').read_bytes()
    if CONFIG.read_bytes() != original:
        raise RuntimeError('client config changed after backup')
    write(CONFIG, original.replace(OLD, NEW), value['mode'])
    os.chown(CONFIG, value['uid'], value['gid'])
    run(['/usr/sbin/restorecon', str(CONFIG)])
    (PENDING / 'staged').touch(mode=0o600)


def rollback(*, boot=False):
    value = manifest()
    write(CONFIG, (PENDING / 'config').read_bytes(), value['mode'])
    os.chown(CONFIG, value['uid'], value['gid'])
    run(['/usr/sbin/restorecon', str(CONFIG)])
    # ExecStartPre must never recursively restart its own unit. Boot restore is
    # configuration-only and runs before the normal stunnel ExecStart.
    (PENDING / 'restoring').touch(mode=0o600)
    return not boot and value['active']


def complete_restore():
    if not PENDING.exists() or not (PENDING / 'restoring').exists():
        return
    value = manifest()
    if CONFIG.read_bytes() != (PENDING / 'config').read_bytes():
        raise RuntimeError('restored configuration drifted before runtime acceptance')
    if value['active']:
        pid = int(run(['/usr/bin/systemctl', 'show', 'stunnel', '--property=MainPID', '--value']).stdout.strip())
        if pid <= 1:
            raise RuntimeError('restored transport has no daemon PID')
        os.kill(pid, 0)
    PENDING.rename(STATE / ('restored-' + str(time.time_ns())))


def main():
    if os.geteuid() != 0:
        raise RuntimeError('root required')
    os.umask(0o077)
    STATE.mkdir(mode=0o700, exist_ok=True)
    restart = False
    with (STATE / 'lock').open('w') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        action = sys.argv[1]
        if action == 'arm':
            arm(int(sys.argv[2]))
        elif action == 'stage':
            stage()
            restart = True
        elif action in ('rollback', 'watchdog', 'boot'):
            if PENDING.exists():
                value = manifest()
                rebooted = BOOT_ID.read_text().strip() != value['boot_id']
                if action == 'rollback' or time.time() >= value['deadline'] or (action == 'boot' and rebooted) or (PENDING / 'restoring').exists():
                    restart = rollback(boot=action == 'boot')
        elif action == 'boot-complete':
            complete_restore()
        elif action == 'confirm':
            gate(STATE / 'accepted', fresh=True)
            value = manifest()
            if time.time() >= value['deadline'] or not (PENDING / 'staged').exists() or CONFIG.read_bytes().count(NEW) != 1:
                raise RuntimeError('no staged client transport')
            PENDING.rename(STATE / ('accepted-' + str(time.time_ns())))
        elif action == 'status':
            value = manifest() if PENDING.exists() else {}
            print(json.dumps({'node': node(), 'pending': PENDING.exists(),
                              'deadline': value.get('deadline')}))
        else:
            raise RuntimeError('unsupported client action')
    # stunnel's ExecStartPre calls this helper too. Release the lock and archive
    # a rollback before restarting, otherwise the native start job deadlocks.
    if restart:
        run(['/usr/bin/systemctl', 'restart', 'stunnel'])
        with (STATE / 'lock').open('w') as lock:
            fcntl.flock(lock, fcntl.LOCK_EX)
            complete_restore()


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        print('storage client refused: ' + type(error).__name__, file=sys.stderr)
        sys.exit(1)
