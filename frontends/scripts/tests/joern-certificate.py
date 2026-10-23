#!/usr/bin/env python3
"""Synthetic certificate validation fixtures; never reads production material."""
import hashlib
import os
from pathlib import Path
import ssl
import subprocess
import tempfile

SCRIPT = Path(__file__).resolve().parents[1] / 'check-joern-certificate.sh'
work = Path(tempfile.mkdtemp(prefix='qo3-cert-fixture-'))
os.chmod(work, 0o700)


def openssl(*args):
    subprocess.run(['openssl', *map(str, args)], check=True,
                   stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
                   timeout=10)


openssl('req', '-x509', '-newkey', 'ec', '-pkeyopt', 'ec_paramgen_curve:P-256',
        '-nodes', '-keyout', work / 'ca.key', '-out', work / 'ca.pem',
        '-days', '90', '-subj', '/CN=Fixture CA')


def certificate(label, sans, days=60):
    openssl('req', '-new', '-newkey', 'ec', '-pkeyopt', 'ec_paramgen_curve:P-256',
            '-nodes', '-keyout', work / (label + '.key'), '-out', work / (label + '.csr'),
            '-subj', '/CN=joern.buetow.org')
    ext = work / (label + '.ext')
    ext.write_text('subjectAltName=' + sans + '\nextendedKeyUsage=serverAuth\n')
    openssl('x509', '-req', '-in', work / (label + '.csr'), '-CA', work / 'ca.pem',
            '-CAkey', work / 'ca.key', '-CAcreateserial', '-out', work / (label + '.pem'),
            '-days', str(days), '-extfile', ext)
    pem = (work / (label + '.pem')).read_text()
    return hashlib.sha256(ssl.PEM_cert_to_DER_cert(pem)).hexdigest()


valid = certificate('valid', 'DNS:joern.buetow.org,DNS:www.joern.buetow.org')
standby = certificate('standby', 'DNS:standby.joern.buetow.org')
wrong = certificate('wrong', 'DNS:wrong.example')
missing_www = certificate('missing-www', 'DNS:joern.buetow.org')
short = certificate('short', 'DNS:joern.buetow.org,DNS:www.joern.buetow.org', 2)


def check(label, pin, ok, name='joern.buetow.org', key=None, ca='ca.pem'):
    key = key or label + '.key'
    result = subprocess.run(['sh', str(SCRIPT), name, str(work / (label + '.pem')),
                             str(work / key), pin, str(work / ca)],
                            capture_output=True, text=True, timeout=5)
    assert (result.returncode == 0) == ok, (label, result.stdout, result.stderr)
    # Neither private PEM nor its encoding may appear in diagnostics.
    assert 'PRIVATE KEY' not in result.stdout + result.stderr


check('valid', valid, True)
check('standby', standby, True, 'standby.joern.buetow.org')
check('valid', '0' * 64, False)
check('valid', 'bad', False)
check('wrong', wrong, False)  # Trusted/pinned CN-only spoof cannot pass SAN policy.
check('missing-www', missing_www, False)
check('short', short, False)
check('valid', valid, False, key='standby.key')
openssl('req', '-x509', '-newkey', 'ec', '-pkeyopt', 'ec_paramgen_curve:P-256',
        '-nodes', '-keyout', work / 'other-ca.key', '-out', work / 'other-ca.pem',
        '-days', '90', '-subj', '/CN=Other Fixture CA')
check('valid', valid, False, ca='other-ca.pem')
check('valid', valid, False, name='www.joern.buetow.org')
openssl('pkey', '-in', work / 'valid.key', '-aes-256-cbc', '-passout', 'pass:fixture',
        '-out', work / 'encrypted.key')
check('valid', valid, False, key='encrypted.key')
(work / 'linked.key').symlink_to('valid.key')
check('valid', valid, False, key='linked.key')
(work / 'malformed.key').write_text('invalid synthetic key\n')
check('valid', valid, False, key='malformed.key')
print('PASS: two scoped certificates; pin, SAN, lifetime, key, CA, scope, symlink and encrypted/malformed-key negatives')
print('Retained synthetic fixture:', work)
