#!/usr/bin/env python3
"""Build a pinned DTail commit into a checksummed FreeBSD packaging bundle."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile

BINARIES = ('dserver', 'dcat', 'dgrep', 'dmap', 'dtail', 'dtailhealth')
ASSETS = {
    'dtail.json.sample': 'etc/dserver/dtail-freebsd.json.tpl',
    'dserver-update-key-cache.sh': 'scripts/dserver-update-key-cache-freebsd.sh.tpl',
    'dserver.rc': 'etc/rc.d/dserver-freebsd.tpl',
}


def run(*args, cwd=None, env=None):
    return subprocess.check_output(args, cwd=cwd, env=env, text=True).strip()


def build(source, ref, revision, output, frontends):
    if not re.fullmatch(r'[1-9][0-9]*', revision):
        raise ValueError('FreeBSD revision must be a positive integer')
    source = source.resolve()
    commit = run('git', '-C', str(source), 'rev-parse', '--verify', ref + '^{commit}')
    text = run('git', '-C', str(source), 'show', commit + ':internal/version/version.go')
    match = re.search(r'^\s*Version\s+string\s*=\s*"([0-9][A-Za-z0-9.+-]*)"\s*$', text, re.M)
    if not match:
        raise ValueError('cannot read a safe upstream version from the selected commit')
    version = match.group(1) + '_' + revision
    # Refuse accidental adoption/deletion of another run or user directory.
    output.parent.mkdir(parents=True, exist_ok=True)
    output.mkdir(mode=0o700)
    env = dict(os.environ, CGO_ENABLED='0', GOOS='freebsd', GOARCH='amd64',
               GOWORK='off', GOAMD64='v1', GOMAXPROCS=os.environ.get('GOMAXPROCS', '2'))
    with tempfile.TemporaryDirectory(prefix='.dtail-source-', dir=output.parent) as temp:
        checkout = Path(temp) / 'source'
        # A detached clean clone excludes original WIP and lets Go embed VCS data.
        subprocess.run(['git', 'clone', '--quiet', '--shared', '--no-checkout',
                        str(source), str(checkout)], check=True)
        subprocess.run(['git', '-C', str(checkout), 'checkout', '--quiet', '--detach', commit], check=True)
        for binary in BINARIES:
            print('Building ' + binary + ' from ' + commit[:12], file=sys.stderr)
            subprocess.run(['go', 'build', '-p=1', '-mod=readonly', '-trimpath',
                            '-buildvcs=true', '-tags=nozstd', '-o', str(output / binary),
                            './cmd/' + binary], cwd=checkout, env=env, check=True)
        toolchain = run('go', 'version', cwd=checkout, env=env)
        if run('git', '-C', str(checkout), 'status', '--porcelain'):
            raise ValueError('source changed during the build')
    for name, path in ASSETS.items():
        shutil.copyfile(frontends / path, output / name)
    payload = {name: hashlib.sha256((output / name).read_bytes()).hexdigest()
               for name in (*BINARIES, *ASSETS)}
    provenance = {
        'package': 'dtail', 'package_version': version,
        'source_commit': commit, 'upstream_version': match.group(1),
        'freebsd_revision': int(revision),
        'go_version': toolchain, 'goos': 'freebsd', 'goarch': 'amd64',
        'cgo_enabled': False, 'build_tags': ['nozstd'], 'buildvcs': True,
        'payload_sha256': payload,
    }
    (output / 'build-info.json').write_text(json.dumps(provenance, indent=2) + '\n')
    manifest = {
        'name': 'dtail', 'version': version, 'origin': 'local/dtail',
        'comment': 'Distributed log tail and grep tool',
        'desc': 'DTail log clients and server; configure dserver with host-specific Gonf settings.',
        'maintainer': 'paul@buetow.org', 'www': 'https://github.com/snonux/dtail',
        'prefix': '/usr/local',
        'annotations': {'dtail_source_commit': commit,
                        'dtail_go_version': provenance['go_version']},
        'message': 'The configuration example is dtail.json.sample. Gonf owns the live per-host dtail.json; configure it before starting dserver.',
    }
    (output / '+MANIFEST').write_text(json.dumps(manifest, indent=2) + '\n')
    # Native assembly checks every input before running pkg create.
    names = (*BINARIES, *ASSETS, 'build-info.json', '+MANIFEST')
    (output / 'SHA256').write_text(''.join(
        hashlib.sha256((output / name).read_bytes()).hexdigest() + ' ' + name + '\n'
        for name in names))
    print(str(output))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--source', type=Path, required=True)
    parser.add_argument('--ref', default='HEAD')
    parser.add_argument('--revision', required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    os.umask(0o077)
    frontends = Path(__file__).resolve().parents[2] / 'frontends'
    try:
        build(args.source, args.ref, args.revision, args.output.absolute(), frontends)
    except (ValueError, OSError, subprocess.CalledProcessError) as error:
        parser.exit(1, 'DTail build failed: ' + str(error) + '\n')


if __name__ == '__main__':
    main()
