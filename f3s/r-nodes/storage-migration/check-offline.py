#!/usr/bin/env python3
"""Exercise actual client dispatch, including its recursive ExecStartPre path."""
import fcntl
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import time

spec = importlib.util.spec_from_file_location('client', Path(__file__).with_name('client.py'))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)

with tempfile.TemporaryDirectory(prefix='storage-client-') as temporary:
    root = Path(temporary)
    module.STATE = root / 'state'
    module.STATE.mkdir(mode=0o700)
    module.PENDING = module.STATE / 'pending'
    module.CONFIG = root / 'stunnel.conf'
    module.BOOT_ID = root / 'boot-id'
    module.BOOT_ID.write_text('original-boot\n')
    module.node = lambda: 'a.example'
    module.os.geteuid = lambda: 0
    module.gate = lambda path, fresh=False: None
    module.maintenance = lambda: None
    original = b'cert = existing.pem\nCAfile = ca.pem\nverify = 2\n' + module.OLD + b'\n'
    calls = []
    failures = {'restart': False}

    def pending(deadline):
        module.PENDING.mkdir(mode=0o700)
        (module.PENDING / 'config').write_bytes(original)
        value = {'node': 'a.example', 'deadline': deadline,
                 'boot_id': 'original-boot', 'uid': os.getuid(),
                 'gid': os.getgid(), 'mode': 0o640, 'active': True}
        (module.PENDING / 'manifest.json').write_text(json.dumps(value))

    def fake(args, *, required=True):
        calls.append(args)
        output = b''
        code = 0
        if args[0] == '/usr/bin/findmnt':
            code = 1
        elif 'show' in args:
            output = str(os.getpid()).encode() if '--property=MainPID' in args else b'inactive\n'
        elif args == ['/usr/bin/systemctl', 'restart', 'stunnel']:
            if failures['restart']:
                failures['restart'] = False
                raise RuntimeError('simulated native restart failure')
            # The real stunnel ExecStartPre must be able to take the lock and
            # dispatch boot without a self-deadlock or recursive restart.
            with (module.STATE / 'lock').open('w') as check:
                fcntl.flock(check, fcntl.LOCK_EX | fcntl.LOCK_NB)
                fcntl.flock(check, fcntl.LOCK_UN)
            saved = sys.argv
            try:
                sys.argv = ['client', 'boot']
                module.main()
                sys.argv = ['client', 'boot-complete']
                module.main()
            finally:
                sys.argv = saved
        return subprocess.CompletedProcess(args, code, output, b'')

    module.run = fake
    saved = sys.argv
    try:
        pending(time.time() + 100)
        module.CONFIG.write_bytes(original)
        sys.argv = ['client', 'stage']
        module.main()
        assert module.CONFIG.read_bytes() == original.replace(module.OLD, module.NEW)
        assert module.PENDING.exists()
        # A new boot restores before stunnel starts, without recursive restart.
        module.BOOT_ID.write_text('new-boot\n')
        count = len(calls)
        sys.argv = ['client', 'boot']
        module.main()
        assert module.CONFIG.read_bytes() == original and module.PENDING.exists()
        assert not any('restart' in call for call in calls[count:])
        sys.argv = ['client', 'boot-complete']
        module.main()
        assert not module.PENDING.exists()
        assert module.CONFIG.stat().st_mode & 0o777 == 0o640
        # An expired runtime transaction restores and restarts outside its lock.
        module.BOOT_ID.write_text('original-boot\n')
        pending(0)
        module.CONFIG.write_bytes(original.replace(module.OLD, module.NEW))
        sys.argv = ['client', 'watchdog']
        failures['restart'] = True
        try:
            module.main()
        except RuntimeError:
            pass
        else:
            raise AssertionError('simulated restart failure did not propagate')
        assert module.PENDING.exists() and (module.PENDING / 'restoring').exists()
        assert module.CONFIG.read_bytes() == original
        # The next native watchdog tick retries runtime restoration before archive.
        module.main()
        assert module.CONFIG.read_bytes() == original and not module.PENDING.exists()
        count = len(calls)
        sys.argv = ['client', 'rollback']
        module.main()
        assert len(calls) == count
        # No fresh acceptance attestation can accept an expired transaction.
        pending(0)
        (module.PENDING / 'staged').touch()
        module.CONFIG.write_bytes(original.replace(module.OLD, module.NEW))
        sys.argv = ['client', 'confirm']
        try:
            module.main()
        except RuntimeError:
            pass
        else:
            raise AssertionError('expired confirmation accepted')
        assert module.PENDING.exists()
    finally:
        sys.argv = saved
print('PASS: actual client stage/ExecStartPre+Post, boot/timed restore, failed-restart journal+retry, unlocked restart, repeat rollback, expired-confirm refusal')
