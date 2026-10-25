#!/usr/bin/env python3
"""Exercise the destination helper with synthetic private state and fake daemons."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

SCRIPTS = Path(__file__).resolve().parents[1]
CONFIG = '''Version = 1.9
<Listener listener0>
    Port = 4242
    IPv4 = true
    IPv6 = false
    SSL = true
    AllowIRC = true
    AllowWeb = true
</Listener>
<User example>
    Pass = synthetic-secret
</User>
'''
STUB = '''#!/usr/bin/env python3
import os, pathlib, signal, subprocess, sys
r = pathlib.Path(os.environ['ZNC_TEST_ROOT'])
c = r / 'configs/znc.conf'
name = pathlib.Path(sys.argv[0]).name
a = sys.argv[1:]
with (r / 'events').open('a') as f: f.write(name + ':' + ':'.join(a) + '\\n')
if name == 'rcctl':
    if a[0] == 'check': sys.exit(0 if (r/'running').exists() else 1)
    if a[0] == 'stop':
        (r/'running').unlink(missing_ok=True)
        c.write_text(c.read_text() + '# saved on stop\\n')
        if (r/'fail-stop').exists():
            (r/'fail-stop').unlink(); sys.exit(1)
        if (r/'bad-stop').exists(): c.write_text(c.read_text().replace('Port = 4242','Port = 4243'))
    if a[0] == 'start':
        if (r/'fail-start').exists():
            (r/'fail-start').unlink(); sys.exit(1)
        (r/'running').touch()
        (r/'bound').write_text('192.168.2.111' if 'Host = 192.168.2.111' in c.read_text() else '*')
if name == 'mv':
    result = subprocess.call(['/bin/mv'] + a)
    if result == 0 and (r/'interrupt-post-rename').exists():
        os.kill(os.getppid(), signal.SIGTERM)
    sys.exit(result)
if name == 'cp':
    count = int((r/'copies').read_text()) + 1 if (r/'copies').exists() else 1
    (r/'copies').write_text(str(count))
    if count == 2 and (r/'interrupt-post-stop').exists():
        os.kill(os.getppid(), signal.SIGTERM)
        sys.exit(1)
    sys.exit(subprocess.call(['/bin/cp'] + a))
if name == 'netstat' and (r/'fail-netstat').exists(): sys.exit(1)
if name == 'netstat' and a[-1] == 'inet' and (r/'running').exists():
    print('tcp 0 0 ' + (r/'bound').read_text() + '.4242 *.* LISTEN')
'''

class ZNC(unittest.TestCase):
    def setUp(self):
        tmp = tempfile.TemporaryDirectory(prefix='znc-wg-')
        self.addCleanup(tmp.cleanup)
        self.root = Path(tmp.name)
        (self.root/'configs').mkdir()
        (self.root/'bin').mkdir()
        self.config = self.root/'configs/znc.conf'
        self.config.write_text(CONFIG)
        (self.root/'running').touch()
        (self.root/'bound').write_text('*')
        stub = self.root/'bin/stub'
        stub.write_text(STUB); stub.chmod(0o755)
        for name in ['rcctl', 'netstat', 'chown', 'cp', 'mv']:
            (self.root/'bin'/name).symlink_to(stub)
        self.helper = self.root/'helper'
        self.helper.write_text((SCRIPTS/'znc-wireguard.sh').read_text()
            .replace('/var/znc/configs', str(self.root/'configs'))
            .replace('/usr/local/libexec/znc-wg-listener.awk', str(SCRIPTS/'znc-wg-listener.awk')))
        self.env = dict(os.environ, ZNC_TEST_ROOT=str(self.root), PATH=str(self.root/'bin')+':'+os.environ['PATH'])

    def run_helper(self, mode='apply', success=True):
        p = subprocess.run(['sh', str(self.helper), mode, '192.168.2.111'], env=self.env, capture_output=True, text=True)
        self.assertEqual(p.returncode == 0, success, p.stderr)
        self.assertNotIn('synthetic-secret', p.stdout+p.stderr)

    def test_preserves_shutdown_save_and_private_data_then_converges(self):
        self.run_helper()
        expected = CONFIG.replace('</Listener>', '\tHost = 192.168.2.111\n</Listener>')+'# saved on stop\n'
        self.assertEqual(self.config.read_text(), expected)
        self.run_helper('check')
        self.run_helper()
        self.assertEqual((self.root/'events').read_text().count('rcctl:stop'), 1)
        self.assertEqual(self.config.stat().st_mode & 0o777, 0o600)

    def test_failed_start_restores_saved_private_config(self):
        (self.root/'fail-start').touch()
        self.run_helper(success=False)
        self.assertEqual(self.config.read_text(), CONFIG+'# saved on stop\n')
        self.assertTrue((self.root/'running').exists())
        self.run_helper()

    def test_initial_stop_save_then_failure_preserves_current_config(self):
        (self.root/'fail-stop').touch()
        self.run_helper(success=False)
        self.assertEqual(self.config.read_text(), CONFIG+'# saved on stop\n')
        self.assertTrue((self.root/'running').exists())
        self.assertEqual((self.root/'events').read_text().count('rcctl:stop'), 1)
        self.assertEqual((self.root/'copies').read_text(), '1')

    def test_signal_after_stop_before_backup_preserves_current_config(self):
        (self.root/'interrupt-post-stop').touch()
        self.run_helper(success=False)
        self.assertEqual(self.config.read_text(), CONFIG+'# saved on stop\n')
        self.assertTrue((self.root/'running').exists())
        self.assertEqual((self.root/'events').read_text().count('rcctl:stop'), 1)
        self.assertEqual((self.root/'copies').read_text(), '2')

    def test_signal_after_atomic_install_restores_post_stop_backup(self):
        (self.root/'interrupt-post-rename').touch()
        self.run_helper(success=False)
        self.assertEqual(self.config.read_text(), CONFIG+'# saved on stop\n')
        self.assertTrue((self.root/'running').exists())
        self.assertEqual((self.root/'copies').read_text(), '3')

    def test_changed_layout_refused_before_stop(self):
        for changed in [CONFIG*2, CONFIG.replace('SSL = true','SSL = false'), CONFIG.replace('IPv6 = false','IPv6 = true')]:
            self.config.write_text(changed)
            self.run_helper(success=False)
            self.assertNotIn('rcctl:stop', (self.root/'events').read_text() if (self.root/'events').exists() else '')

    def test_shutdown_layout_change_rolls_back_without_host_edit(self):
        (self.root/'bad-stop').touch()
        self.run_helper(success=False)
        self.assertNotIn('Host =', self.config.read_text())
        self.assertTrue((self.root/'running').exists())

    def test_listener_probe_failure_restores_config(self):
        (self.root/'fail-netstat').touch()
        self.run_helper(success=False)
        self.assertEqual(self.config.read_text(), CONFIG+'# saved on stop\n')
        self.assertTrue((self.root/'running').exists())

    def test_config_alone_does_not_claim_active_binding(self):
        self.config.write_text(CONFIG.replace('</Listener>', '\tHost = 192.168.2.111\n</Listener>'))
        self.run_helper('check', success=False)
        self.run_helper()
        self.run_helper('check')

if __name__ == '__main__': unittest.main()
