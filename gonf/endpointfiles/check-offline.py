#!/usr/bin/env python3
"""Exercise the real endpoint apply helper with temporary paths.

Runs assets/apply and the real fleetfiles merger against files in a temporary
directory, with id/uname/restorecon mocked. Covers the hosts merge, the
controller ownership rules (legacy, candidate, adopted copy, custom refusal),
the retired boot/watchdog no-ops and the refusals (non-root, unknown action,
symlinked destination, missing candidate).
"""
from pathlib import Path
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[2]
SOURCE = (ROOT / 'gonf/endpointfiles/assets/apply').read_text()
MERGER = (ROOT / 'gonf/fleetfiles/assets/merge-hosts').read_text()


def put(path, value, mode=0o644):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(value)
    path.chmod(mode)


with tempfile.TemporaryDirectory(prefix='conf-endpoint-test-') as directory:
    base = Path(directory)
    fake = base / 'bin'
    state = base / 'state'
    uid = base / 'uid'
    platform = base / 'platform'
    put(uid, '0\n')
    put(platform, 'Linux\n')
    for command, body in {
        'id': f'cat {uid}',
        'uname': f'cat {platform}',
        'restorecon': ':',
    }.items():
        put(fake / command, '#!/bin/sh\nset -eu\n' + body + '\n', 0o755)
    helper = base / 'apply'
    merge = base / 'merge'
    script = SOURCE.replace('PATH=/usr/local/sbin:', f'PATH={fake}:/usr/local/sbin:')
    for old, new in {
        '/var/lib/gonf-endpoint-migration': str(state),
        '/var/db/gonf-endpoint-migration': str(state),
        '/usr/local/sbin/gonf-fleet-hosts': str(merge),
        '/usr/local/etc/gonf-fleet-hosts': str(base / 'fleet'),
        '/usr/local/etc/': str(base / 'local/etc') + '/',
    }.items():
        script = script.replace(old, new)
    assert '/usr/local/etc/' not in script and '/var/' not in script
    put(helper, script, 0o755)
    put(merge, MERGER.replace('target=/etc/hosts', 'target=' + str(base / 'etc/hosts')), 0o755)

    def run(*args, ok=True):
        result = subprocess.run([str(helper), *args], text=True, capture_output=True, timeout=8)
        assert (result.returncode == 0) == ok, (args, result.returncode, result.stdout, result.stderr)
        assert 'secret' not in result.stdout + result.stderr
        return result

    # Retired hooks: no-ops for every platform, even without root, and they
    # create no state (a stale OpenBSD /etc/rc "boot || exit 1" must pass).
    put(uid, '1000\n')
    for name in ('Linux', 'NetBSD', 'FreeBSD', 'OpenBSD'):
        put(platform, name + '\n')
        run('boot')
        run('watchdog')
    assert not state.exists()
    run('apply', 'hosts', ok=False)
    assert 'root required' in run('adopt', 'controller', ok=False).stderr
    put(uid, '0\n')
    put(platform, 'Linux\n')
    for args in ((), ('apply',), ('apply', 'dns'), ('arm', 'hosts', '60'), ('confirm', 'hosts'),
                 ('rollback',), ('status',), ('adopt', 'hosts')):
        assert 'unknown action' in run(*args, ok=False).stderr

    # Hosts: the merger replaces conflicting fleet names, keeps local rows,
    # and needs no accepted-hosts marker any more.
    hosts = base / 'etc/hosts'
    initial = '127.0.0.1 localhost f0\n192.168.1.130 f0 f0.lan unrelated\n# local comment\n192.0.2.7 extra\n'
    put(hosts, initial)
    put(base / 'fleet', '10.20.0.130 f0 f0.lan\n')
    run('apply', 'hosts')
    text = hosts.read_text()
    assert '# BEGIN GONF fleet\n10.20.0.130 f0 f0.lan\n# END GONF fleet' in text, text
    assert '192.168.1.130 unrelated' in text and '127.0.0.1 localhost\n' in text
    assert '192.0.2.7 extra' in text and '# local comment' in text
    assert hosts.stat().st_mode & 0o777 == 0o644
    subprocess.run([str(merge), 'check', str(base / 'fleet')], check=True)
    (base / 'fleet').unlink()
    run('apply', 'hosts', ok=False)
    assert hosts.read_text() == text

    # Controller ownership.
    etc = base / 'local/etc'
    live, candidate, legacy = etc / 'f3sctl.json', etc / 'f3sctl.json.triggerfish-candidate', etc / 'f3sctl.json.triggerfish-legacy'
    owned = state / 'controller-owned'
    put(legacy, '{"v":"legacy"}\n')
    put(candidate, '{"v":1}\n')
    run('apply', 'controller', ok=False)  # no live file
    put(live, '{"v":"legacy"}\n', 0o640)
    run('apply', 'controller')
    assert live.read_text() == '{"v":1}\n' and live.stat().st_mode & 0o777 == 0o640

    # An unedited live file survives a candidate update through adopt.
    # apply records what it published, so the next owner run is settled.
    assert owned.read_text() == '{"v":1}\n' and state.stat().st_mode & 0o777 == 0o700
    owned.unlink()
    run('adopt', 'controller')
    assert owned.read_text() == '{"v":1}\n'
    put(candidate, '{"v":2}\n')
    run('apply', 'controller')
    assert live.read_text() == '{"v":2}\n' and owned.read_text() == '{"v":2}\n'
    # Without adopt the same update is refused (old candidate is unowned).
    owned.unlink()
    put(candidate, '{"v":3}\n')
    assert 'custom controller settings' in run('apply', 'controller', ok=False).stderr
    assert live.read_text() == '{"v":2}\n'

    # A custom (hand-edited) file is never adopted and stays refused.
    put(candidate, '{"v":2}\n')
    run('adopt', 'controller')
    put(live, '{"v":"custom","secret":"x"}\n')
    run('adopt', 'controller')
    assert owned.read_text() == '{"v":2}\n'
    put(candidate, '{"v":4}\n')
    run('apply', 'controller', ok=False)
    assert live.read_text() == '{"v":"custom","secret":"x"}\n'

    # Symlinks are refused: live destination, adopted copy and state dir.
    live.unlink()
    target = base / 'elsewhere.json'
    put(target, '{"v":4}\n')
    live.symlink_to(target)
    run('apply', 'controller', ok=False)
    assert target.read_text() == '{"v":4}\n'
    live.unlink()
    put(live, '{"v":"custom"}\n')
    owned.unlink()
    owned.symlink_to(live)
    run('apply', 'controller', ok=False)
    owned.unlink()
    moved = base / 'state-real'
    state.rename(moved)
    state.symlink_to(moved)
    put(live, '{"v":4}\n')
    run('adopt', 'controller', ok=False)
    assert not list(base.glob('local/etc/*.gonf.*')), 'temporary files left behind'

# OpenBSD /etc/rc retirement: the real script from retire.go against fixtures.
GO = (ROOT / 'gonf/endpointfiles/retire.go').read_text()
block = GO.split('const openBSDRcBlock = "', 1)[1].split('"', 1)[0].replace('\\n', '\n').rstrip('\n')
body = GO.split('script := strings.Join([]string{', 1)[1].split('}, "\\n")', 1)[0]
lines = [part.split('`')[1] for part in body.splitlines() if part.strip().startswith('`')]
assert len(lines) == 8, lines
with tempfile.TemporaryDirectory(prefix='conf-endpoint-rc-') as directory:
    base = Path(directory)
    rc = base / 'rc'
    script = '\n'.join(lines).replace('/etc/rc', str(rc)).replace('/bin/ksh', 'sh')

    def retire(content, ok):
        put(rc, content)
        result = subprocess.run(['sh', '-ec', script, 'retire-rc', block], capture_output=True, text=True)
        assert (result.returncode == 0) == ok, (content, result)
        assert not list(base.glob('rc.gonf-endpoints.*')), 'temporary file left behind'
        return rc.read_text()

    head, tail = 'mount -uw /\n', 'rootdisk_nodes\nsysctl_conf\n'
    assert retire(head + block + '\n' + tail, True) == head + tail
    assert rc.stat().st_mode & 0o777 == 0o644
    changed = head + block.replace('exit 1', 'exit 2') + '\n' + tail
    assert retire(changed, False) == changed
    twice = head + block + '\n' + block + '\n' + tail
    assert retire(twice, False) == twice
    stray = head + block + '\n/usr/local/sbin/gonf-endpoint-migration status\n' + tail
    assert retire(stray, False) == stray
    broken = head + block + '\nif then\n' + tail
    assert retire(broken, False) == broken

print('endpointfiles offline checks passed')
