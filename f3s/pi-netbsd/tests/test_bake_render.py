"""Render the NetBSD bake's setup.sh offline and compare it with gonf.

Runs `bake-golden.sh --render` (no download, no qemu), then steps 1-3 of the
rendered setup.sh against a fixture /etc, and checks that the rc.conf lines
equal what pis_netbsd_network_rc_conf records for the same Pi (task q74).
The negative cases cover a Pi without inventory, a bad address and a
template line that no longer renders.

    python3 -m unittest discover -s f3s/pi-netbsd/tests -p test_bake_render.py
"""
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[3]
BAKE = ROOT / 'f3s/pi-netbsd/bake'
KEY = 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFixtureOnlyNotARealKey000000000000000 test@fixture'


def gonf_rc_conf_lines():
    """Returns {pin: [keyed rc.conf lines]} from gonf's redacted preview."""
    out = subprocess.run([str(ROOT / 'gonf.sh'), 'plan', '-redacted', 'pis_netbsd_network_rc_conf'],
                         capture_output=True, text=True, check=True, timeout=600).stdout
    lines, host = {}, None
    for raw in out.splitlines():
        op = json.loads(raw)
        if op.get('op') == 'when_begin' and op['id'].startswith('when.hostname:'):
            host = op['id'].split(':', 1)[1]
        elif op.get('op') == 'file' and op.get('name') == 'rc-conf-network':
            lines[host] = [k['line'] for k in op['keyed_lines']]
    return lines


class BakeRender(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.tmp = tempfile.TemporaryDirectory(prefix='conf-bake-render-')
        cls.key = Path(cls.tmp.name) / 'id.pub'
        cls.key.write_text(KEY + '\n')
        cls.gonf = gonf_rc_conf_lines()

    @classmethod
    def tearDownClass(cls):
        cls.tmp.cleanup()

    def render(self, pin, script=BAKE / 'bake-golden.sh', **env):
        environment = {k: v for k, v in os.environ.items() if k not in ('IPADDR', 'ULA')}
        environment.update({'SSHKEY_FILE': str(self.key), **env})
        return subprocess.run([str(script), '--render', pin], capture_output=True, text=True,
                              env=environment, timeout=60)

    def apply_network_steps(self, rendered):
        """Runs steps 1-3 of a rendered setup.sh against a fixture /etc."""
        etc = Path(tempfile.mkdtemp(dir=self.tmp.name)) / 'etc'
        etc.mkdir()
        (etc / 'rc.conf').write_text('rc_configured=YES\nifconfig_mue0="inet 192.168.1.125 netmask 0xffffff00"\n'
                                     'defaultroute="192.168.1.1"\ndhcpcd=YES\n')
        (etc / 'rc.conf').chmod(0o600)
        steps = rendered.split('# 4)', 1)[0].replace('/etc/', f'{etc}/').replace('RC=/etc/rc.conf', f'RC={etc}/rc.conf')
        subprocess.run(['sh', '-ec', steps], check=True, timeout=30)
        return etc

    def test_pi0_pi1_match_gonf(self):
        self.assertEqual(sorted(self.gonf), ['pi0', 'pi1'])
        for pin, suffix in (('pi0', '125'), ('pi1', '126')):
            with self.subTest(pin=pin):
                result = self.render(pin)
                self.assertEqual(result.returncode, 0, result.stderr)
                subprocess.run(['sh', '-n'], input=result.stdout, text=True, check=True)
                self.assertIn(KEY, result.stdout)
                self.assertNotIn('__SSHKEY__', result.stdout)
                etc = self.apply_network_steps(result.stdout)
                rc = (etc / 'rc.conf').read_text().splitlines()
                got = [l for l in rc if l.startswith(('ifconfig_mue0=', 'defaultroute='))]
                self.assertEqual(got, self.gonf[pin])
                self.assertIn(f'inet 10.20.0.{suffix} netmask', got[0])
                self.assertIn(f'inet6 fd02:ca0b:f1cb:20::{suffix} prefixlen 64 alias', got[0])
                self.assertIn(f'hostname="{pin}.lan.buetow.org"', rc)
                self.assertIn('dhcpcd=NO', rc)
                self.assertNotIn('dhcpcd=YES', rc)
                self.assertEqual((etc / 'rc.conf').stat().st_mode & 0o777, 0o644)
                self.assertEqual((etc / 'resolv.conf').read_text(), 'nameserver 10.20.0.1\n')
                local = (etc / 'rc.local').read_text()
                subprocess.run(['sh', '-n', str(etc / 'rc.local')], check=True)
                self.assertIn(f'inet6 fd02:ca0b:f1cb:20::{suffix} prefixlen 64 alias', local)
                self.assertIn('route add default 10.20.0.1', local)
                self.assertNotIn('192.168.1.', (etc / 'rc.conf').read_text() + local)

    def test_pi_without_inventory_needs_env(self):
        result = self.render('pi2')
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(result.stdout, '')
        self.assertIn('set IPADDR and ULA', result.stderr)
        result = self.render('pi2', IPADDR='10.20.0.127', ULA='fd02:ca0b:f1cb:20::127')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('IPADDR="10.20.0.127"\nULA="fd02:ca0b:f1cb:20::127"\n', result.stdout)

    def test_bad_values_refused(self):
        for env, error in (({'IPADDR': '10.20.0.1270', 'ULA': 'fd02::127'}, 'not an IPv4 address'),
                           ({'IPADDR': '999.20.0.127', 'ULA': 'fd02::127'}, 'not an IPv4 address'),
                           ({'IPADDR': '10.20.0.127; reboot', 'ULA': 'fd02::127'}, 'not an IPv4 address'),
                           ({'IPADDR': '10.20.0.127', 'ULA': '2001:db8::1'}, 'not a ULA'),
                           ({'IPADDR': '10.20.0.127', 'ULA': 'fdfd'}, 'not a ULA')):
            with self.subTest(env=env):
                result = self.render('pi9', **env)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(result.stdout, '')
                self.assertIn(error, result.stderr)

    def test_bad_node_name_refused(self):
        result = self.render('x"; reboot; "', IPADDR='10.20.0.127', ULA='fd02::127')
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(result.stdout, '')
        self.assertIn('not a Pi node name', result.stderr)

    def test_missing_key_refused(self):
        result = self.render('pi0', SSHKEY_FILE=str(Path(self.tmp.name) / 'absent.pub'))
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(result.stdout, '')
        self.assertIn('missing', result.stderr)

    def test_template_drift_refused(self):
        """A setup.sh whose ULA line no longer renders must not bake pi0's default."""
        tree = Path(tempfile.mkdtemp(dir=self.tmp.name))
        bake = tree / 'f3s/pi-netbsd/bake'
        bake.mkdir(parents=True)
        (tree / 'gonf').symlink_to(ROOT / 'gonf')
        shutil.copy2(BAKE / 'bake-golden.sh', bake)
        template = (BAKE / 'setup.sh').read_text()
        self.assertIn('\nULA=', template)
        (bake / 'setup.sh').write_text(template.replace('\nULA=', '\nLAB_ULA='))
        result = self.render('pi1', script=bake / 'bake-golden.sh')
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('did not render ULA=', result.stderr)


if __name__ == '__main__':
    unittest.main()
