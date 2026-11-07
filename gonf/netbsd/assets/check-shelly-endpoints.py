#!/usr/bin/env python3
"""Run the actual recipe admission guard against disposable filesystem cases."""
import json
import os
from pathlib import Path
import subprocess
import tempfile

source = Path(__file__).parents[1] / 'shelly_endpoints.go'
guard = source.read_text().split('`')[1]
content = Path(__file__).with_name('shelly-endpoints.json').read_text()
assert json.loads(content) == {'inventory': {'shelly_ip': '10.30.0.28', 'shelly_ac_ip': '10.30.0.29'}}
with tempfile.TemporaryDirectory(prefix='shelly-endpoints-') as directory:
    root = Path(directory)
    config = root / 'f3sctl.json'
    candidate = root / 'f3sctl.json.triggerfish-shelly-candidate'
    gate = root / 'devices-approved'
    candidate.write_text(content)
    commands = root / 'bin'
    commands.mkdir()
    stat = commands / 'stat'
    stat.write_text('''#!/usr/bin/env python3
import os,sys
from pathlib import Path
assert sys.argv[1:3]==['-f','%u:%Lp']
print(os.environ.get('FIXTURE_OWNER','0')+':'+format(Path(sys.argv[3]).stat().st_mode&0o777,'o'))
''')
    stat.chmod(0o755)
    guard = guard.replace('/usr/local/etc/f3sctl.json', str(config))
    guard = guard.replace('/var/db/gonf-shelly-migration/devices-approved', str(gate))
    guard = guard.replace('PATH=/sbin:/bin:/usr/sbin:/usr/bin:/usr/pkg/bin', f'PATH={commands}:/usr/bin:/bin')
    approved = 'shellyplugmg3-0892725e366c 10.30.0.28\nshellyplugmg3-0892725f8a04 10.30.0.29\n'

    def check(success, owner='0'):
        result = subprocess.run(['sh', '-ec', guard], env=dict(os.environ, FIXTURE_OWNER=owner),
                                capture_output=True, text=True, timeout=5)
        assert (result.returncode == 0) == success, (result.stdout, result.stderr)

    check(False)  # No installation without device acceptance.
    gate.write_text(approved)
    gate.chmod(0o600)
    check(True)
    check(False, owner='1000')
    gate.chmod(0o644)
    check(False)
    gate.chmod(0o600)
    gate.write_text(approved.replace('10.30.0.29', '10.30.0.99'))
    check(False)
    gate.write_text(approved)
    config.write_text('{"api_key_file":"preserve-this-custom-setting"}\n')
    before = config.read_bytes()
    check(False)  # Even an approval must not wipe a custom configuration.
    assert config.read_bytes() == before
    config.write_text(content)
    gate.unlink()
    check(True)  # Identical already-active configuration is safely adopted.
    config.unlink()
    config.symlink_to(candidate)
    check(False)
    config.unlink()
    gate.symlink_to(candidate)
    check(False)
    gate.unlink()
    os.mkfifo(config)
    check(False)
print('PASS: actual recipe guard, approved installation, idempotent adoption, custom-config preservation, identity/owner/mode/symlink/special-file refusal')
