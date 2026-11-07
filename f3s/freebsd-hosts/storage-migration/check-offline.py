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
    module.active = lambda service: True
    module.carp_allowed = lambda: current['allowed']

    def fake(args, *, required=True):
        calls.append(args)
        output = b''
        if args[0] == '/sbin/sysctl':
            current['allowed'] = args[-1].endswith('=1')
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
        return subprocess.CompletedProcess(args, 0, output, b'')

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
    finally:
        sys.argv = original_argv
print('PASS: actual deadline/boot dispatch, owned-file restore, repeat rollback, persistent fence, missing-pair and expired-confirm refusal')
