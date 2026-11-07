#!/usr/local/bin/python3.12
"""Guard the existing CARP pair's storage transaction; never elect a new owner.

Coordinator attestations are exact-node root-owned mode0600 files. The helper
cannot infer the other member's state: it requires fresh pair-held and
pair-matching attestations before staging or enabling advertisements. Rollback
always leaves CARP disabled until BOTH restored members have been audited.
"""
import fcntl
import json
import os
from pathlib import Path
import shutil
import stat
import subprocess
import sys
import time

STATE = Path('/var/db/gonf-storage-migration')
PENDING = STATE / 'pending'
PF_PENDING = STATE / 'pf-pending'
PF_CONFIG = Path('/etc/pf.conf')
PF_DIGEST = Path('/var/db/gonf-pf.sha256')
RC = Path('/etc/rc.conf')
ALIASES = ('ifconfig_re0_alias0', 'ifconfig_re0_alias1')
FILES = (Path('/usr/local/etc/stunnel/stunnel.conf'),
         Path('/usr/local/etc/relayd.conf'))
OLD = '192.168.1.138'
NEW = '10.20.0.138'
SERVICES = ('devd', 'stunnel', 'nfsd', 'mountd', 'nfsuserd', 'relayd')
SUFFIX = '.triggerfish-candidate'


def run(args, *, required=True):
    result = subprocess.run(args, capture_output=True, timeout=60)
    if required and result.returncode:
        # Never include argv or native output: sysrc/ifconfig may carry a key.
        raise RuntimeError('native operation failed: ' + Path(args[0]).name)
    return result


def write(path, value):
    temporary = path.with_name(path.name + '.tmp')
    with temporary.open('w') as stream:
        json.dump(value, stream)
        stream.flush()
        os.fsync(stream.fileno())
    temporary.chmod(0o600)
    temporary.replace(path)


def node():
    value = run(['/bin/hostname', '-s']).stdout.decode().strip()
    if value not in ('f0', 'f1'):
        raise RuntimeError('not an existing CARP member')
    return value


def gate(name, *, fresh=False):
    path = STATE / name
    info = path.lstat()
    if not stat.S_ISREG(info.st_mode) or info.st_uid != 0 or stat.S_IMODE(info.st_mode) != 0o600:
        raise RuntimeError('invalid coordinator gate: ' + name)
    if path.read_text().strip() != node():
        raise RuntimeError('coordinator gate names another host')
    if fresh and not 0 <= time.time() - info.st_mtime <= 120:
        raise RuntimeError('stale pair attestation')


def aliases():
    values = {}
    for key in ALIASES:
        result = run(['/usr/sbin/sysrc', '-f', str(RC), '-n', key], required=False)
        if result.returncode == 0:
            values[key] = result.stdout.decode().strip()
        elif result.returncode != 1:
            raise RuntimeError('cannot inspect CARP alias')
    return values


def carp_allowed():
    return run(['/sbin/sysctl', '-n', 'net.inet.carp.allow']).stdout.strip() == b'1'


def active(service):
    return run(['/usr/sbin/service', service, 'onestatus'], required=False).returncode == 0


def stop(service):
    if active(service):
        run(['/usr/sbin/service', service, 'onestop'], required=False)
        if active(service):
            raise RuntimeError('service remains active after stop')


def drain_hooks():
    deadline = time.monotonic() + 60
    while True:
        rows = run(['/bin/ps', '-axo', 'args=']).stdout.decode().splitlines()
        hooks = [row for row in rows if '/usr/local/bin/carpcontrol.sh' in row.split()]
        if not hooks:
            return
        if time.monotonic() >= deadline:
            raise RuntimeError('existing CARP hook did not finish')
        time.sleep(0.1)


def hold():
    # Stop transition hooks before disabling advertisements. This does not stop
    # cron or edit its independent unattended schedules.
    stop('devd')
    run(['/sbin/sysctl', 'net.inet.carp.allow=0'])
    if carp_allowed():
        raise RuntimeError('CARP fence did not take effect')
    (STATE / 'held').touch(mode=0o600)
    drain_hooks()
    for service in ('stunnel', 'nfsd', 'mountd', 'nfsuserd', 'relayd'):
        stop(service)


def arm(seconds):
    gate('approved', fresh=True)
    if PENDING.exists() or PF_PENDING.exists() or (STATE / 'held').exists():
        raise RuntimeError('unfinished storage transaction')
    if not 60 <= seconds <= 1800:
        raise RuntimeError('deadline must be 60..1800 seconds')
    current = aliases()
    line = current.get(ALIASES[0], '')
    if ALIASES[1] in current or OLD + '/32' not in line or 'vhid 1 ' not in line:
        raise RuntimeError('unexpected original CARP aliases')
    if NEW in run(['/sbin/ifconfig', 're0']).stdout.decode():
        raise RuntimeError('new VIP already present')
    if not carp_allowed() or not active('devd'):
        raise RuntimeError('original CARP hooks are not active')
    temporary = STATE / 'preparing'
    temporary.mkdir(mode=0o700)
    manifest = {'node': node(), 'deadline': int(time.time()) + seconds,
                'aliases': current, 'services': {name: active(name) for name in SERVICES},
                'carp_allowed': carp_allowed(), 'files': []}
    for index, path in enumerate(FILES):
        info = path.lstat()
        if not stat.S_ISREG(info.st_mode) or info.st_uid != 0:
            raise RuntimeError('unexpected managed configuration metadata')
        shutil.copyfile(path, temporary / str(index))
        (temporary / str(index)).chmod(0o600)
        manifest['files'].append({'path': str(path), 'mode': stat.S_IMODE(info.st_mode),
                                  'uid': info.st_uid, 'gid': info.st_gid})
    write(temporary / 'manifest.json', manifest)
    temporary.rename(PENDING)


def manifest():
    value = json.loads((PENDING / 'manifest.json').read_text())
    if value['node'] != node():
        raise RuntimeError('saved transaction names another node')
    return value


def stage():
    value = manifest()
    gate('pair-held', fresh=True)
    if carp_allowed() or active('devd') or not (STATE / 'held').exists():
        raise RuntimeError('member is not fenced')
    if time.time() >= value['deadline'] - 30:
        raise RuntimeError('insufficient transaction time')
    if aliases() != value['aliases']:
        raise RuntimeError('CARP alias changed after backup')
    for index, path in enumerate(FILES):
        if path.read_bytes() != (PENDING / str(index)).read_bytes():
            raise RuntimeError('managed configuration changed after backup')
    relay = Path(str(FILES[1]) + SUFFIX)
    run(['/usr/local/sbin/relayd', '-n', '-f', str(relay)])
    # Native stunnel admission is a separate isolated-listener validation; no
    # daemon is started as a purported syntax-only check here.
    gate('stunnel-validated', fresh=True)
    line = value['aliases'][ALIASES[0]]
    run(['/usr/sbin/sysrc', '-f', str(RC), ALIASES[0] + '=' + line.replace(OLD, NEW)])
    run(['/usr/sbin/sysrc', '-f', str(RC), ALIASES[1] + '=' + line])
    for path in FILES:
        shutil.copyfile(str(path) + SUFFIX, path)
    # Existing OLD address stays under vhid1; adding NEW to that same group
    # changes the authenticated set only while both members are fenced.
    args = line.split()
    run(['/sbin/ifconfig', 're0'] + [NEW + '/32' if part == OLD + '/32' else part for part in args])
    (PENDING / 'staged').touch(mode=0o600)


def rollback():
    value = manifest()
    hold()
    for index, metadata in enumerate(value['files']):
        path = Path(metadata['path'])
        shutil.copyfile(PENDING / str(index), path)
        os.chown(path, metadata['uid'], metadata['gid'])
        path.chmod(metadata['mode'])
    for key in ALIASES:
        if key in value['aliases']:
            run(['/usr/sbin/sysrc', '-f', str(RC), key + '=' + value['aliases'][key]])
        elif key in aliases():
            run(['/usr/sbin/sysrc', '-f', str(RC), '-x', key])
    if NEW in run(['/sbin/ifconfig', 're0']).stdout.decode():
        run(['/sbin/ifconfig', 're0', 'inet', NEW, '-alias'])
    archive = STATE / ('restored-' + str(time.time_ns()))
    PENDING.rename(archive)
    write(STATE / 'held', {'restored': archive.name})
    # Do not enable CARP or restart services. The other member may still use
    # the new set; only a fresh pair audit can authorize release-old.


def enable():
    value = manifest()
    gate('pair-matching', fresh=True)
    if not (PENDING / 'staged').exists() or time.time() >= value['deadline'] - 30:
        raise RuntimeError('no current staged transaction')
    runtime = run(['/sbin/ifconfig', 're0']).stdout.decode()
    if not all(address in runtime for address in (OLD, NEW)):
        raise RuntimeError('missing paired VIP address')
    run(['/sbin/sysctl', 'net.inet.carp.allow=1'])
    run(['/usr/sbin/service', 'devd', 'onestart'])
    run(['/usr/sbin/service', 'relayd', 'onestart'])
    (STATE / 'held').unlink()


def reconcile():
    # devd was stopped during the transition and does not replay missed events.
    # Apply its existing hook to the current elected state, never force MASTER.
    state = run(['/usr/local/bin/carp', 'state']).stdout.decode().strip()
    if state not in ('MASTER', 'BACKUP'):
        raise RuntimeError('CARP election is not settled')
    run(['/usr/local/bin/carpcontrol.sh', '1@re0', state])


def release_old():
    gate('old-pair-matching', fresh=True)
    if PENDING.exists() or not (STATE / 'held').exists():
        raise RuntimeError('not a restored fenced transaction')
    archive_name = json.loads((STATE / 'held').read_text())['restored']
    if not archive_name.startswith('restored-') or '/' in archive_name:
        raise RuntimeError('invalid restored transaction')
    value = json.loads((STATE / archive_name / 'manifest.json').read_text())
    if aliases() != value['aliases'] or NEW in run(['/sbin/ifconfig', 're0']).stdout.decode():
        raise RuntimeError('restored address set changed')
    run(['/sbin/sysctl', 'net.inet.carp.allow=1'])
    run(['/usr/sbin/service', 'devd', 'onestart'])
    if value['services']['relayd']:
        run(['/usr/sbin/service', 'relayd', 'onestart'])
    (STATE / 'held').unlink()


def pf_arm(seconds):
    gate('approved', fresh=True)
    if PENDING.exists() or PF_PENDING.exists() or (STATE / 'held').exists():
        raise RuntimeError('overlapping PF or storage transaction')
    if not 60 <= seconds <= 900:
        raise RuntimeError('PF deadline must be 60..900 seconds')
    if run(['/sbin/pfctl', '-s', 'Running'], required=False).returncode != 0:
        raise RuntimeError('PF is not already enabled')
    metadata = []
    for path in (PF_CONFIG, PF_DIGEST):
        info = path.lstat()
        if not stat.S_ISREG(info.st_mode) or info.st_uid != 0:
            raise RuntimeError('unexpected PF file metadata')
        metadata.append({'path': str(path), 'uid': info.st_uid, 'gid': info.st_gid,
                         'mode': stat.S_IMODE(info.st_mode)})
    temporary = STATE / 'pf-preparing'
    temporary.mkdir(mode=0o700)
    for index, path in enumerate((PF_CONFIG, PF_DIGEST)):
        shutil.copyfile(path, temporary / str(index))
        (temporary / str(index)).chmod(0o600)
    write(temporary / 'manifest.json', {'node': node(), 'deadline': int(time.time()) + seconds,
                                      'files': metadata})
    temporary.rename(PF_PENDING)


def pf_restore(*, boot=False):
    value = json.loads((PF_PENDING / 'manifest.json').read_text())
    if value['node'] != node():
        raise RuntimeError('PF transaction names another member')
    enabled = run(['/sbin/pfctl', '-s', 'Running'], required=False).returncode == 0
    if not boot and not enabled:
        raise RuntimeError('runtime PF unexpectedly disabled')
    if enabled:
        run(['/sbin/pfctl', '-nf', str(PF_PENDING / '0')])
    for index, metadata in enumerate(value['files']):
        path = Path(metadata['path'])
        shutil.copyfile(PF_PENDING / str(index), path)
        os.chown(path, metadata['uid'], metadata['gid'])
        path.chmod(metadata['mode'])
    if enabled:
        run(['/sbin/pfctl', '-f', str(PF_CONFIG)])
    # A failed load retains the journal for the next native watchdog tick.
    PF_PENDING.rename(STATE / ('pf-restored-' + str(time.time_ns())))


def main():
    if os.geteuid() != 0:
        raise RuntimeError('root required')
    os.umask(0o077)
    STATE.mkdir(mode=0o700, exist_ok=True)
    with (STATE / 'lock').open('w') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        action = sys.argv[1]
        if action == 'pf-arm':
            pf_arm(int(sys.argv[2]))
        elif action == 'pf-confirm':
            gate('pf-accepted', fresh=True)
            value = json.loads((PF_PENDING / 'manifest.json').read_text())
            if value['node'] != node() or time.time() >= value['deadline']:
                raise RuntimeError('PF transaction expired or names another member')
            PF_PENDING.rename(STATE / ('pf-accepted-' + str(time.time_ns())))
        elif action == 'pf-rollback':
            if PF_PENDING.exists():
                pf_restore()
        elif action == 'arm':
            arm(int(sys.argv[2]))
        elif action == 'hold':
            manifest()
            hold()
        elif action == 'stage':
            stage()
        elif action == 'enable':
            enable()
        elif action == 'reconcile':
            gate('pair-matching', fresh=True)
            reconcile()
        elif action == 'release-old':
            release_old()
        elif action == 'rollback':
            if PENDING.exists():
                rollback()
        elif action in ('watchdog', 'boot'):
            if PF_PENDING.exists():
                value = json.loads((PF_PENDING / 'manifest.json').read_text())
                if action == 'boot' or time.time() >= value['deadline']:
                    pf_restore(boot=action == 'boot')
            if PENDING.exists() and (action == 'boot' or time.time() >= manifest()['deadline']):
                rollback()
            if action == 'boot' and (STATE / 'held').exists():
                run(['/sbin/sysctl', 'net.inet.carp.allow=0'])
        elif action == 'boot-hold':
            # Normal rc.d startup can start enabled services after the early
            # pre-netif restore. Keep CARP fenced throughout, then stop those
            # services once their ordinary boot hooks have completed.
            if (STATE / 'held').exists():
                hold()
        elif action == 'confirm':
            gate('accepted', fresh=True)
            value = manifest()
            if time.time() >= value['deadline'] or not (PENDING / 'staged').exists() or (STATE / 'held').exists() or not carp_allowed():
                raise RuntimeError('cannot confirm a fenced member')
            PENDING.rename(STATE / ('accepted-' + str(time.time_ns())))
        elif action == 'status':
            value = manifest() if PENDING.exists() else {}
            print(json.dumps({'node': node(), 'pending': PENDING.exists(),
                              'deadline': value.get('deadline'),
                              'held': (STATE / 'held').exists(),
                              'carp_allowed': carp_allowed(), 'pf_pending': PF_PENDING.exists()}))
        else:
            raise RuntimeError('unsupported storage action')


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        # Exception values from the standard library may contain saved data.
        print('storage migration refused: ' + type(error).__name__, file=sys.stderr)
        sys.exit(1)
