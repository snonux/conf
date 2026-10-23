#!/usr/bin/env python3
"""Synthetic fixtures; --native-host runs them in a disposable SSH /tmp directory.

The optional remote archive contains generated fixture keys only. No production
certificate, credential, configuration or key is read or changed.
"""
import argparse
import hashlib
import os
from pathlib import Path
import shlex
import ssl
import subprocess
import tarfile
import tempfile

SCRIPT = Path(__file__).resolve().parents[1] / 'check-peer-certificate.sh'
NAMES = ('standby.ecat.buetow.org', 'standby.alt.irregular.ninja',
         'standby.gogios.buetow.org', 'gogios.buetow.org')


def openssl(*args):
    subprocess.run(['openssl', *map(str, args)], check=True,
                   stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
                   timeout=10)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--native-host', choices=('blowfish', 'fishfinger'))
    args = parser.parse_args()
    with tempfile.TemporaryDirectory(prefix='ro3-synthetic-') as temporary:
        work = Path(temporary)
        os.chmod(work, 0o700)
        (work / 'validator').write_text(SCRIPT.read_text())
        openssl('req', '-x509', '-newkey', 'ec', '-pkeyopt', 'ec_paramgen_curve:P-256',
                '-nodes', '-keyout', work / 'ca.key', '-out', work / 'ca.pem',
                '-days', '90', '-subj', '/CN=Synthetic Fixture CA')

        def certificate(label, sans, days=60, rsa=False):
            algorithm = ['rsa:2048'] if rsa else ['ec', '-pkeyopt', 'ec_paramgen_curve:P-256']
            openssl('req', '-new', '-newkey', *algorithm, '-nodes', '-keyout',
                    work / (label + '.key'), '-out', work / (label + '.csr'),
                    '-subj', '/CN=Synthetic Fixture')
            ext = work / (label + '.ext')
            ext.write_text('subjectAltName=' + sans + '\nextendedKeyUsage=serverAuth\n')
            openssl('x509', '-req', '-in', work / (label + '.csr'), '-CA', work / 'ca.pem',
                    '-CAkey', work / 'ca.key', '-CAcreateserial', '-out', work / (label + '.pem'),
                    '-days', str(days), '-extfile', ext)
            leaf = (work / (label + '.pem')).read_text()
            pin = hashlib.sha256(ssl.PEM_cert_to_DER_cert(leaf)).hexdigest()
            # Exercise fullchain inputs as well as isolated leaves.
            (work / (label + '.pem')).write_text(leaf + (work / 'ca.pem').read_text())
            return pin

        pins = {n: certificate(str(i), 'DNS:' + n +
                              (',DNS:www.gogios.buetow.org' if i == 3 else ''), rsa=i == 3)
                for i, n in enumerate(NAMES)}
        wrong = certificate('wrong', 'DNS:wrong.example')
        missing = certificate('missing', 'DNS:gogios.buetow.org')
        extra = certificate('extra', 'DNS:standby.ecat.buetow.org,DNS:extra.example')
        short = certificate('short', 'DNS:standby.ecat.buetow.org', 2)
        openssl('pkey', '-in', work / '0.key', '-aes-256-cbc', '-passout', 'pass:fixture',
                '-out', work / 'encrypted.key')
        (work / 'linked.key').symlink_to('0.key')
        (work / 'linked.pem').symlink_to('0.pem')
        (work / 'malformed.key').write_text('invalid synthetic key\n')
        openssl('req', '-x509', '-newkey', 'ec', '-pkeyopt', 'ec_paramgen_curve:P-256',
                '-nodes', '-keyout', work / 'other-ca.key', '-out', work / 'other-ca.pem',
                '-days', '90', '-subj', '/CN=Other Synthetic Fixture CA')
        before = {p.name: p.read_bytes() for p in work.iterdir() if p.is_file()}
        ssh = []
        target = str(work)
        if args.native_host:
            ssh = ['ssh', '-T', '-oBatchMode=yes', '-oStrictHostKeyChecking=yes',
                   '-oConnectTimeout=7', 'rex@' + args.native_host + '.buetow.org']
            target = subprocess.check_output(ssh + ['mktemp -d /tmp/ro3-synthetic.XXXXXXXXXX'],
                                             text=True, timeout=15).strip()
            assert target.startswith('/tmp/ro3-synthetic.') and target.count('/') == 2
            archive = work / 'fixture.tar'
            with tarfile.open(archive, 'w') as tar:
                for p in work.iterdir():
                    if p != archive:
                        tar.add(p, arcname=p.name)
            with archive.open('rb') as stream:
                subprocess.run(ssh + ['tar -C ' + shlex.quote(target) + ' -xf -'],
                               stdin=stream, check=True, timeout=15)

        count = 0

        def check(label, pin, ok, name=NAMES[0], key=None, ca='ca.pem'):
            nonlocal count
            command = ['sh', target + '/validator', name, target + '/' + label + '.pem',
                       target + '/' + (key or label + '.key'), pin, target + '/' + ca]
            if ssh:
                command = ssh + [shlex.join(command)]
            result = subprocess.run(command, capture_output=True, text=True, timeout=15)
            assert (result.returncode == 0) == ok, (label, result.stdout, result.stderr)
            assert 'PRIVATE KEY' not in result.stdout + result.stderr
            count += 1

        try:
            for i, n in enumerate(NAMES):
                check(str(i), pins[n], True, n)
            check('0', '0' * 64, False)
            check('0', 'bad', False)
            check('wrong', wrong, False)
            check('missing', missing, False, NAMES[3])
            check('extra', extra, False)
            check('short', short, False)
            check('0', pins[NAMES[0]], False, key='1.key')
            check('0', pins[NAMES[0]], False, ca='other-ca.pem')
            check('0', pins[NAMES[0]], False, name='unapproved.example')
            for key in ('encrypted.key', 'linked.key', 'malformed.key', 'absent.key'):
                check('0', pins[NAMES[0]], False, key=key)
            check('linked', pins[NAMES[0]], False, key='0.key')
            check('absent', pins[NAMES[0]], False, key='0.key')
            for name, contents in before.items():
                assert (work / name).read_bytes() == contents
        finally:
            if ssh:
                subprocess.run(ssh + ['rm -rf ' + shlex.quote(target)], check=True, timeout=15)
        print(f'PASS: {count} synthetic validator cases on {args.native_host or "local OpenSSL"}; fixtures removed')


if __name__ == '__main__':
    main()
