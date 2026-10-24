#!/usr/bin/env python3
"""Exercise all shipped cache scripts as root against isolated fake OS paths.

Run: sudo python3 packages/tests/dserver-key-cache.py
Only fixture path/group constants are rewritten. cat, su, stat, mv and filesystem
permissions are real; no system accounts, services or live caches are changed.
"""
import os
from pathlib import Path
import pwd
import shutil
import stat
import subprocess
import json
import socket
import time
import tempfile
import unittest

REPO = Path(__file__).resolve().parents[2]
SCRIPTS = [
    'frontends/scripts/dserver-update-key-cache.sh.tpl',
    'frontends/scripts/dserver-update-key-cache-freebsd.sh.tpl',
    'frontends/scripts/dserver-update-key-cache-netbsd.sh.tpl',
    'packages/files/dtail-rocky/dserver-update-key-cache.sh',
]


@unittest.skipUnless(os.geteuid() == 0, 'requires root for genuine ownership tests')
class CacheSecurity(unittest.TestCase):
    def setUp(self):
        self.user = pwd.getpwnam('nobody')
        self.temp = tempfile.TemporaryDirectory(prefix='dserver-cache-security-')
        self.base = Path(self.temp.name)
        self.base.chmod(0o755)

    def tearDown(self):
        self.temp.cleanup()

    def fixture(self, variant):
        base = self.base / str(variant)
        base.mkdir(mode=0o755)
        home = base / 'home' / 'nobody'
        (home / '.ssh').mkdir(parents=True, mode=0o755)
        source = home / '.ssh' / 'authorized_keys'
        source.write_text('test-user-key\n')
        source.chmod(0o600)
        os.chown(source, self.user.pw_uid, self.user.pw_gid)
        (base / 'root').mkdir(mode=0o700)
        script = base / 'refresh.sh'
        content = (REPO / SCRIPTS[variant]).read_text()
        content = content.replace('/var/run/dserver', str(base / 'run'))
        content = content.replace('/root', str(base / 'root'))
        content = content.replace('/home', str(base / 'home'))
        content = content.replace('DSERVER_GROUP=_dserver', 'DSERVER_GROUP=nobody')
        content = content.replace('DSERVER_GROUP=dserver', 'DSERVER_GROUP=nobody')
        # id -g nobody is portable even where its primary group is nogroup.
        # chown expects a group name; use the numeric gid for this fixture.
        content = content.replace('chown "root:$DSERVER_GROUP"', 'chown "root:$groupid"')
        script.write_text(content)
        return base, source, script

    def run_script(self, script, success=True):
        result = subprocess.run(['sh', str(script)], capture_output=True, text=True,
                                timeout=10)
        self.assertEqual(result.returncode == 0, success, result.stderr)
        return result

    def as_user(self, args):
        return subprocess.run(['su', '-s', '/bin/sh', 'nobody', '-c', args],
                              capture_output=True, text=True, timeout=5)

    def test_all_variants_attack_and_normal_paths(self):
        for variant, name in enumerate(SCRIPTS):
            with self.subTest(script=name):
                base, source, script = self.fixture(variant)
                self.run_script(script)
                cache = base / 'run' / 'cache'
                target = cache / 'nobody.authorized_keys'
                self.assertEqual(target.read_text(), source.read_text())
                for path, mode in [(cache.parent, 0o750), (cache, 0o750),
                                   (target, 0o640)]:
                    info = path.stat()
                    self.assertEqual((info.st_uid, info.st_gid, stat.S_IMODE(info.st_mode)),
                                     (0, self.user.pw_gid, mode))
                self.assertEqual(self.as_user(f'cat {target}').returncode, 0)
                self.assertNotEqual(self.as_user(f'ln -s /etc/shadow {cache}/root.authorized_keys').returncode, 0)
                sentinel = base / 'root-only-sentinel'
                sentinel.write_text('root-only-must-not-change\n')
                sentinel.chmod(0o600)
                before = sentinel.stat()
                for kind in ['symlink', 'hardlink', 'fifo']:
                    target.unlink()
                    if kind == 'symlink':
                        target.symlink_to(sentinel)
                    elif kind == 'hardlink':
                        os.link(sentinel, target)
                    else:
                        os.mkfifo(target)
                    self.run_script(script)
                    self.assertEqual(target.read_text(), 'test-user-key\n')
                    self.assertFalse(target.is_symlink())
                    self.assertEqual(sentinel.read_text(), 'root-only-must-not-change\n')
                    after = sentinel.stat()
                    self.assertEqual((before.st_uid, before.st_gid, before.st_mode, before.st_mtime_ns),
                                     (after.st_uid, after.st_gid, after.st_mode, after.st_mtime_ns))
                # Invalid sources revoke old keys and never export the sentinel.
                source.unlink()
                source.symlink_to(sentinel)
                self.run_script(script)
                self.assertFalse(target.exists())
                source.unlink()
                source.write_text('wrong-owner\n')
                self.run_script(script)
                self.assertFalse(target.exists())
                os.chown(source, self.user.pw_uid, self.user.pw_gid)
                os.link(source, source.parent / 'borrowed-link')
                self.run_script(script)
                self.assertFalse(target.exists())
                (source.parent / 'borrowed-link').unlink()
                self.run_script(script)
                self.assertTrue(target.exists())
                source.unlink()
                self.run_script(script)
                self.assertFalse(target.exists())
                # Directories (including symlinks to directories) must fail.
                for symlink in [False, True]:
                    if symlink:
                        target.symlink_to(base / 'root', target_is_directory=True)
                    else:
                        target.mkdir()
                    self.run_script(script, success=False)
                    if symlink:
                        target.unlink()
                    else:
                        target.rmdir()
                # Unsafe containers are refused without chmod/chowning targets.
                cache.chmod(0o770)
                self.run_script(script, success=False)
                self.assertEqual(stat.S_IMODE(cache.stat().st_mode), 0o770)
                cache.chmod(0o750)
                cache.rmdir()
                cache.symlink_to(base / 'root')
                self.run_script(script, success=False)
                cache.unlink()
                cache.mkdir(mode=0o750)
                os.chown(cache, 0, self.user.pw_gid)
                os.chown(cache.parent, self.user.pw_uid, self.user.pw_gid)
                self.run_script(script, success=False)
                self.assertEqual(cache.parent.stat().st_uid, self.user.pw_uid)

    def test_publication_keeps_old_file_until_real_rename(self):
        for variant in range(len(SCRIPTS)):
            with self.subTest(script=SCRIPTS[variant]):
                base, source, script = self.fixture(variant)
                self.run_script(script)
                target = base / 'run/cache/nobody.authorized_keys'
                source.write_text('replacement-key\n')
                hookdir = base / 'hooks'
                hookdir.mkdir(mode=0o755)
                hook = hookdir / 'mv'
                hook.write_text(f'#!/bin/sh\ntest -f {target} || exit 41\nexec {shutil.which("mv")} "$@"\n')
                hook.chmod(0o755)
                script.write_text(script.read_text().replace('PATH=/usr/bin:', f'PATH={hookdir}:/usr/bin:'))
                self.run_script(script)
                self.assertEqual(target.read_text(), 'replacement-key\n')
                self.assertFalse((base / 'run/key-cache.lock').exists())
                self.assertEqual(list((base / 'run/cache').glob('.refresh.*')), [])

    @unittest.skipUnless(os.environ.get('DSERVER_TEST_BINARY'),
                         'set DSERVER_TEST_BINARY to a copied installed Linux binary')
    def test_installed_dserver_authenticates_from_group_readable_cache(self):
        base, source, script = self.fixture(3)
        binary = Path(os.environ['DSERVER_TEST_BINARY']).resolve()
        key = base / 'client-key'
        hostkey = base / 'host-key'
        for path in [key, hostkey]:
            subprocess.run(['ssh-keygen', '-q', '-t', 'ed25519', '-N', '',
                            '-f', str(path)], check=True)
        source.write_text(key.with_suffix('.pub').read_text())
        self.run_script(script)
        os.chown(hostkey, 0, self.user.pw_gid)
        hostkey.chmod(0o640)
        logdir = base / 'log'
        logdir.mkdir(mode=0o750)
        os.chown(logdir, self.user.pw_uid, self.user.pw_gid)
        with socket.socket() as listener:
            listener.bind(('127.0.0.1', 0))
            port = listener.getsockname()[1]
        config = json.loads((REPO / 'packages/files/dtail-rocky/dtail.json').read_text())
        config['Server']['HostKeyFile'] = str(hostkey)
        config['Server']['SSHBindAddress'] = '127.0.0.1'
        config['Common'].update(CacheDir=str(base / 'run/cache'),
                                LogDir=str(logdir), SSHPort=port)
        cfg = base / 'config.json'
        cfg.write_text(json.dumps(config))
        cfg.chmod(0o644)
        with (base / 'server-output').open('w+') as output:
            server = subprocess.Popen(['setpriv', '--reuid', str(self.user.pw_uid),
                                       '--regid', str(self.user.pw_gid),
                                       '--clear-groups', str(binary), '-cfg', str(cfg)],
                                      cwd=base, stdout=output, stderr=output)
            try:
                for attempt in range(50):
                    if server.poll() is not None:
                        output.seek(0)
                        self.fail('installed binary failed to start: ' + output.read())
                    try:
                        with socket.create_connection(('127.0.0.1', port), timeout=0.1):
                            break
                    except OSError:
                        time.sleep(0.05)
                else:
                    self.fail('installed binary did not listen')
                command = ['ssh', '-v', '-N', '-p', str(port), '-i', str(key),
                           '-oBatchMode=yes', '-oIdentitiesOnly=yes',
                           '-oStrictHostKeyChecking=no', '-oUserKnownHostsFile=/dev/null',
                           '-oPreferredAuthentications=publickey', 'nobody@127.0.0.1']
                try:
                    result = subprocess.run(command, capture_output=True, timeout=2)
                    evidence = result.stderr
                except subprocess.TimeoutExpired as error:
                    evidence = error.stderr
                self.assertIn(b'Authenticated to 127.0.0.1', evidence)
            finally:
                server.terminate()
                try:
                    server.wait(timeout=3)
                except subprocess.TimeoutExpired:
                    server.kill()
                    server.wait(timeout=3)

    def test_swap_after_source_validation_reads_as_source_owner(self):
        # Deterministically race the actual su call: swap to a root-only file
        # immediately before real su runs. Only PATH is extended for this hook.
        for variant in range(len(SCRIPTS)):
            with self.subTest(script=SCRIPTS[variant]):
                base, source, script = self.fixture(variant)
                sentinel = base / 'secret'
                sentinel.write_text('never-export-this\n')
                sentinel.chmod(0o600)
                hookdir = base / 'hooks'
                hookdir.mkdir(mode=0o755)
                hook = hookdir / 'su'
                hook.write_text(f'#!/bin/sh\nrm -f {source}\nln -s {sentinel} {source}\nexec {shutil.which("su")} "$@"\n')
                hook.chmod(0o755)
                script.write_text(script.read_text().replace('PATH=/usr/bin:', f'PATH={hookdir}:/usr/bin:'))
                self.run_script(script)
                self.assertFalse((base / 'run/cache/nobody.authorized_keys').exists())
                self.assertEqual(sentinel.read_text(), 'never-export-this\n')


if __name__ == '__main__':
    if os.geteuid() != 0:
        raise SystemExit('Run as root: sudo python3 packages/tests/dserver-key-cache.py')
    unittest.main()
