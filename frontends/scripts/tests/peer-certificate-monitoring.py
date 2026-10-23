#!/usr/bin/env python3
"""Render actual Go monitoring policy with a synthetic exported overlay.

No recipe plan, secret provider, production write or *_test.go file is used.
Optionally export only the ten additional check definitions per frontend.
"""
import argparse
import copy
import json
import os
from pathlib import Path
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[3]
HOSTS = {'blowfish': ('23.88.35.144', '2a01:4f8:c17:20f1::42'),
         'fishfinger': ('46.23.94.99', '2a03:6000:6f67:624::99')}
NAMES = ('standby.ecat.buetow.org', 'standby.alt.irregular.ninja',
         'standby.gogios.buetow.org', 'gogios.buetow.org', 'www.gogios.buetow.org')


def validate(config, host):
    checks = {k: v for k, v in config['Checks'].items() if k.startswith('Check Peer HTTPS ')}
    assert len(checks) == 10
    for name in NAMES:
        for family, address in zip((4, 6), HOSTS[host]):
            check = checks[f'Check Peer HTTPS IPv{family} {name} on {host}']
            assert check['Plugin'] == '/usr/local/libexec/nagios/check_curl'
            assert check['Args'] == ['-S', '-D', '--sni', '-H', name, '-I', address,
                                     f'-{family}', '-C', '20,7', '--continue-after-certificate',
                                     '-e', 'HTTP/1.1 200', '-t', '10']
            assert check['Local'] is True and not check.get('DependsOn')
            assert check['RunInterval'] == 300 and check['RandomSpread'] == 10
            assert not check.get('OnlyIfNotExists')
    return checks


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path)
    parser.add_argument('--baseline', default='613310e')
    args = parser.parse_args()
    env = {**os.environ, 'GOFLAGS': '-p=1', 'GOMAXPROCS': '2'}
    with tempfile.TemporaryDirectory(prefix='ro3-monitoring-') as temporary:
        work = Path(temporary)
        shim = work / 'fixture.go'
        shim.write_text('package frontends\nfunc PeerCertificateFixture(name string) string {\n'
                        'return renderGogios(MustServer(name))\n}\n')
        main = work / 'main.go'
        main.write_text('package main\nimport("fmt";"os";'
                        '"github.com/snonux/conf/gonf/frontends")\nfunc main(){'
                        'fmt.Print(frontends.PeerCertificateFixture(os.Args[1]))}\n')
        overlay = {str(ROOT / 'gonf/frontends/ro3_fixture.go'): str(shim)}
        overlay_file = work / 'overlay.json'

        def render(host, baseline=False):
            mapping = dict(overlay)
            if baseline:
                source = subprocess.check_output(['git', 'show', args.baseline +
                                                  ':gonf/frontends/monitoring.go'], cwd=ROOT)
                old = work / 'baseline.go'
                old.write_bytes(source)
                mapping[str(ROOT / 'gonf/frontends/monitoring.go')] = str(old)
            overlay_file.write_text(json.dumps({'Replace': mapping}))
            return json.loads(subprocess.check_output(['go', 'run', '-overlay', str(overlay_file),
                                                      str(main), host], cwd=ROOT / 'gonf', env=env))

        for host in HOSTS:
            current = render(host)
            checks = validate(current, host)
            stripped = copy.deepcopy(current)
            for name in checks:
                del stripped['Checks'][name]
            assert stripped == render(host, baseline=True), host + ': unexpected existing configuration delta'
            for field, value in [('Local', False), ('DependsOn', ['non-local ping'])]:
                bad = copy.deepcopy(current)
                next(v for k, v in bad['Checks'].items() if k.startswith('Check Peer HTTPS '))[field] = value
                try:
                    validate(bad, host)
                except AssertionError:
                    pass
                else:
                    raise AssertionError('failed negative control: ' + field)
            for token in ('-D', '-I', '-C', '--sni'):
                bad = copy.deepcopy(current)
                next(v for k, v in bad['Checks'].items() if k.startswith('Check Peer HTTPS '))['Args'].remove(token)
                try:
                    validate(bad, host)
                except AssertionError:
                    pass
                else:
                    raise AssertionError('failed negative control: ' + token)
            if args.output:
                args.output.mkdir(parents=True, exist_ok=True)
                (args.output / (host + '-checks.json')).write_text(json.dumps(checks, indent=2) + '\n')
    print('PASS: 20 own-address strict Local SAN/family checks; unchanged existing config; dependency/Local/trust/address/expiry/SNI negatives')


if __name__ == '__main__':
    main()
