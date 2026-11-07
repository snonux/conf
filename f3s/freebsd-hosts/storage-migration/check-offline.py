#!/usr/bin/env python3
"""Exercise real recovery dispatch against private files and fake native calls."""
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile

spec = importlib.util.spec_from_file_location('storage', Path(__file__).with_name('migrate.py'))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)

with tempfile.TemporaryDirectory(prefix='storage-recovery-') as temporary:
    root = Path(temporary)
    module.STATE = root / 'state'
    module.STATE.mkdir(mode=0o700)
    module.PENDING = module.STATE / 'pending'
    module.PF_PENDING = module.STATE / 'pf-pending'
    module.PF_CONFIG = root / 'pf.conf'
    module.PF_DIGEST = root / 'pf.sha256'
    module.PENDING.mkdir(mode=0o700)
    module.node = lambda: 'a.example'
    module.os.geteuid = lambda: 0
    calls = []
    config = root / 'service.conf'
    config.write_text('new transport\n')
    untouched = root / 'other.conf'
    untouched.write_text('unrelated scheduled tasks\n')
    (module.PENDING / '0').write_text('old transport\n')
    values = {module.ALIASES[0]: 'new alias', module.ALIASES[1]: 'old compatibility alias'}
    original = {module.ALIASES[0]: 'original alias'}
    current = {'allowed': True, 'new': True}
    value = {'node': 'a.example', 'deadline': 0, 'aliases': original,
             'services': {'relayd': True}, 'carp_allowed': True,
             'files': [{'path': str(config), 'uid': os.getuid(),
                        'gid': os.getgid(), 'mode': 0o640}]}
    (module.PENDING / 'manifest.json').write_text(json.dumps(value))
    module.aliases = lambda: dict(values)
    current['services'] = {service: True for service in module.SERVICES}
    current['stop_rc'] = 0
    current['keep_active'] = False
    current['pf_fail'] = False
    current['hooks'] = False
    module.active = lambda service: current['services'].get(service, False)
    module.carp_allowed = lambda: current['allowed']

    def fake(args, *, required=True):
        calls.append(args)
        output = b''
        code = 0
        if args[0] == '/sbin/sysctl':
            current['allowed'] = args[-1].endswith('=1')
        elif args[0] == '/bin/ps' and current['hooks']:
            output = b'/bin/sh /usr/local/bin/carpcontrol.sh 1@em0 MASTER\n'
        elif args[0] == '/usr/sbin/service' and args[-1] == 'onestop':
            if not current['keep_active']:
                current['services'][args[1]] = False
            code = current['stop_rc']
        elif args[0] == '/sbin/pfctl' and '-f' in args and current['pf_fail']:
            if required:
                raise RuntimeError('simulated PF load failure')
            code = 1
        elif args[0] == '/sbin/ifconfig':
            if args[-1] == '-alias':
                current['new'] = False
            else:
                output = (module.NEW if current['new'] else module.OLD).encode()
        elif args[0] == '/usr/sbin/sysrc':
            if '-x' in args:
                values.pop(args[-1])
            else:
                key, content = args[-1].split('=', 1)
                values[key] = content
        return subprocess.CompletedProcess(args, code, output, b'')

    module.run = fake
    original_argv = sys.argv
    try:
        # Real watchdog dispatch must restore both runtime and persistent state,
        # stop hooks before fencing, and never restart any service/advertisement.
        sys.argv = ['migration', 'watchdog']
        module.main()
        assert not module.PENDING.exists()
        assert not current['allowed'] and not current['new']
        assert values == original and config.read_text() == 'old transport\n'
        assert config.stat().st_mode & 0o777 == 0o640
        assert untouched.read_text() == 'unrelated scheduled tasks\n'
        assert calls[0] == ['/usr/sbin/service', 'devd', 'onestop']
        assert calls[1] == ['/sbin/sysctl', 'net.inet.carp.allow=0']
        assert all('onestart' not in call and 'net.inet.carp.allow=1' not in call for call in calls)
        assert json.loads((module.STATE / 'held').read_text())['restored'].startswith('restored-')
        # Repeated rollback is a no-op. A later boot must retain the fence.
        count = len(calls)
        sys.argv = ['migration', 'rollback']
        module.main()
        assert len(calls) == count
        sys.argv = ['migration', 'boot']
        module.main()
        assert calls[-1] == ['/sbin/sysctl', 'net.inet.carp.allow=0']
        # Missing pair attestation cannot re-enable advertisements.
        sys.argv = ['migration', 'release-old']
        count = len(calls)
        try:
            module.main()
        except FileNotFoundError:
            pass
        else:
            raise AssertionError('release accepted a missing gate')
        assert len(calls) == count
        # A fresh external acceptance gate cannot override an expired immutable
        # deadline before the next watchdog tick.
        module.PENDING.mkdir()
        (module.PENDING / 'manifest.json').write_text(json.dumps(value))
        (module.PENDING / 'staged').touch()
        (module.STATE / 'held').unlink()
        current['allowed'] = True
        module.gate = lambda name, fresh=False: None
        sys.argv = ['migration', 'confirm']
        try:
            module.main()
        except RuntimeError:
            pass
        else:
            raise AssertionError('confirmation accepted an expired deadline')
        assert module.PENDING.exists() and len(calls) == count
        # A nonzero rc.d exit is accepted only after actual inactive status.
        current['services']['stunnel'] = True
        current['stop_rc'] = 1
        module.stop('stunnel')
        current['services']['stunnel'] = True
        current['keep_active'] = True
        try:
            module.stop('stunnel')
        except RuntimeError:
            pass
        else:
            raise AssertionError('still-active service stop accepted')
        current['keep_active'] = False
        current['stop_rc'] = 0
        # An existing transition hook must finish naturally before hold succeeds.
        clock = module.time.monotonic
        ticks = iter((0, 61))
        current['hooks'] = True
        module.time.monotonic = lambda: next(ticks)
        try:
            module.drain_hooks()
        except RuntimeError:
            pass
        else:
            raise AssertionError('hold accepted an unfinished CARP hook')
        finally:
            module.time.monotonic = clock
            current['hooks'] = False
        # PF and storage journals must never overlap, in either direction.
        try:
            module.pf_arm(60)
        except RuntimeError:
            pass
        else:
            raise AssertionError('PF arm overlapped storage')
        import shutil
        shutil.rmtree(module.PENDING)
        module.PF_PENDING.mkdir()
        try:
            module.arm(60)
        except RuntimeError:
            pass
        else:
            raise AssertionError('storage arm overlapped PF')
        pf_value = {'node': 'a.example', 'deadline': 0, 'files': []}
        for index, path in enumerate((module.PF_CONFIG, module.PF_DIGEST)):
            path.write_text('new PF state\n')
            (module.PF_PENDING / str(index)).write_text('old PF state\n')
            pf_value['files'].append({'path': str(path), 'uid': os.getuid(), 'gid': os.getgid(), 'mode': 0o640})
        (module.PF_PENDING / 'manifest.json').write_text(json.dumps(pf_value))
        sys.argv = ['migration', 'pf-confirm']
        try:
            module.main()
        except RuntimeError:
            pass
        else:
            raise AssertionError('expired PF confirmation accepted')
        assert module.PF_PENDING.exists()
        current['pf_fail'] = True
        sys.argv = ['migration', 'watchdog']
        try:
            module.main()
        except RuntimeError:
            pass
        else:
            raise AssertionError('PF load failure ignored')
        assert module.PF_PENDING.exists() and module.PF_CONFIG.read_text() == 'old PF state\n'
        current['pf_fail'] = False
        module.main()
        assert not module.PF_PENDING.exists()
        assert module.PF_DIGEST.read_text() == 'old PF state\n'
        assert ['/sbin/pfctl', '-f', str(module.PF_CONFIG)] in calls
        # An unconfirmed PF journal restores on boot even before its deadline.
        module.PF_PENDING.mkdir()
        pf_value['deadline'] = 10**12
        (module.PF_PENDING / 'manifest.json').write_text(json.dumps(pf_value))
        for index, path in enumerate((module.PF_CONFIG, module.PF_DIGEST)):
            path.write_text('unconfirmed PF\n')
            (module.PF_PENDING / str(index)).write_text('old PF state\n')
        sys.argv = ['migration', 'boot']
        module.main()
        assert not module.PF_PENDING.exists() and module.PF_CONFIG.read_text() == 'old PF state\n'
    finally:
        sys.argv = original_argv
print('PASS: actual storage/PF deadline+boot dispatch, failed-load retry, overlap refusal, stop-status verification, persistent fence, missing-pair/expired-confirm refusal')
