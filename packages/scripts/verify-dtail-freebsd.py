#!/usr/bin/env python3
"""Verify an assembled package against its pinned local build bundle."""
import hashlib
import json
import re
from pathlib import Path
import subprocess
import sys

BINS = ('dserver', 'dcat', 'dgrep', 'dmap', 'dtail', 'dtailhealth')
PATHS = {**{name: '/usr/local/bin/' + name for name in BINS},
         'dserver-update-key-cache.sh': '/usr/local/bin/dserver-update-key-cache.sh',
         'dserver.rc': '/usr/local/etc/rc.d/dserver',
         'dtail.json.sample': '/usr/local/etc/dserver/dtail.json.sample',
         'build-info.json': '/usr/local/share/doc/dtail/build-info.json'}


def member(archive, name):
    return subprocess.check_output(['bsdtar', '-xOf', str(archive), name], stderr=subprocess.PIPE)


def require(condition, message):
    if not condition:
        raise ValueError(message)


def verify(bundle, archive):
    info = json.loads((bundle/'build-info.json').read_text())
    require(re.fullmatch(r'[0-9a-f]{40}', info['source_commit']), 'invalid source commit')
    require(type(info['freebsd_revision']) is int and info['freebsd_revision'] > 0, 'invalid revision')
    require(info['package_version'] == info['upstream_version']+'_'+str(info['freebsd_revision']), 'version/revision mismatch')
    manifest = json.loads(member(archive, '+MANIFEST'))
    require(manifest['name'] == 'dtail', 'unexpected package name')
    require(manifest['origin'] == 'local/dtail' and manifest['prefix'] == '/usr/local', 'unexpected origin/prefix')
    require(not manifest.get('deps'), 'package must not add dependencies')
    require(manifest['version'] == info['package_version'], 'package version mismatch')
    require(manifest['abi'] == 'FreeBSD:15:amd64', 'unexpected target ABI')
    require(manifest['annotations']['dtail_source_commit'] == info['source_commit'], 'source annotation mismatch')
    require(not manifest.get('scripts'), 'package must not run installation/service scripts')
    require(set(manifest['files']) == set(PATHS.values()), 'unexpected or missing packaged files')
    names = subprocess.check_output(['bsdtar', '-tf', str(archive)], text=True).splitlines()
    require(len(names) == len(PATHS)+2 and set(names) == set(PATHS.values()) | {'+MANIFEST', '+COMPACT_MANIFEST'}, 'unexpected archive members')
    for name, path in PATHS.items():
        data = member(archive, path)
        require(data == (bundle/name).read_bytes(), 'package/bundle mismatch: ' + path)
        attrs = manifest['files'][path]
        require(attrs['uname'] == 'root' and attrs['gname'] == 'wheel', 'wrong file ownership: '+path)
        executable = name in BINS or name in ('dserver.rc', 'dserver-update-key-cache.sh')
        require(attrs['perm'] == ('0755' if executable else '0644'), 'wrong file mode: '+path)
        if name != 'build-info.json':
            require(hashlib.sha256(data).hexdigest() == info['payload_sha256'][name], 'payload digest mismatch: '+name)
        if name in BINS:
            metadata = subprocess.check_output(['go', 'version', '-m', str(bundle/name)], text=True)
            require(metadata.splitlines()[0].endswith(': '+info['go_version'].split()[2]), 'compiler provenance mismatch')
            for flag in ['vcs.revision='+info['source_commit'], 'vcs.modified=false',
                         'GOOS=freebsd', 'GOARCH=amd64', 'GOAMD64=v1', 'CGO_ENABLED=0', '-tags=nozstd']:
                require(flag in metadata, 'binary build metadata mismatch: '+name+' '+flag)
    return {'package': manifest['name'], 'version': manifest['version'],
            'source_commit': info['source_commit'], 'abi': manifest['abi'],
            'artifact_sha256': hashlib.sha256(archive.read_bytes()).hexdigest(),
            'all_payload_bytes_match': True, 'all_file_owner_modes_match': True,
            'actual_host_config_not_shipped': True, 'verification_performs_no_install_or_publish': True}


if __name__ == '__main__':
    if len(sys.argv) != 3:
        sys.exit('Usage: verify-dtail-freebsd.py BUNDLE PACKAGE')
    try:
        print(json.dumps(verify(Path(sys.argv[1]), Path(sys.argv[2])), indent=2))
    except (KeyError, IndexError, ValueError, OSError, subprocess.CalledProcessError) as error:
        sys.exit('Package verification failed: ' + str(error))
