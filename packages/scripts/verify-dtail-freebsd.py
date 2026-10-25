#!/usr/bin/env python3
"""Verify a native package against its pinned build bundle (Python 3.14/Zstd)."""
import hashlib
import json
from pathlib import Path
import re
import subprocess
import sys
import tarfile

BINS = ('dserver', 'dcat', 'dgrep', 'dmap', 'dtail', 'dtailhealth')
PATHS = {**{name: '/usr/local/bin/' + name for name in BINS},
         'dserver-update-key-cache.sh': '/usr/local/bin/dserver-update-key-cache.sh',
         'dserver.rc': '/usr/local/etc/rc.d/dserver',
         'dtail.json.sample': '/usr/local/etc/dserver/dtail.json.sample',
         'build-info.json': '/usr/local/share/doc/dtail/build-info.json'}
METADATA = {'+MANIFEST', '+COMPACT_MANIFEST'}


def require(condition, message):
    if not condition:
        raise ValueError(message)


def unique_object(pairs):
    value = {}
    for key, item in pairs:
        require(key not in value, 'duplicate JSON key: '+key)
        value[key] = item
    return value


def load_json(data):
    return json.loads(data, object_pairs_hook=unique_object)


def verify(bundle, archive):
    info = load_json((bundle/'build-info.json').read_bytes())
    require(re.fullmatch(r'[0-9a-f]{40}', info['source_commit']), 'invalid source commit')
    require(type(info['freebsd_revision']) is int and info['freebsd_revision'] > 0, 'invalid revision')
    require(info['package_version'] == info['upstream_version']+'_'+str(info['freebsd_revision']), 'version/revision mismatch')
    with tarfile.open(archive, 'r:*') as package:
        entries = package.getmembers()
        names = [entry.name for entry in entries]
        require(len(names) == len(PATHS)+2 and set(names) == set(PATHS.values()) | METADATA, 'unexpected archive members')
        by_path = {entry.name: entry for entry in entries}
        executable_paths = {PATHS[name] for name in (*BINS, 'dserver.rc', 'dserver-update-key-cache.sh')}
        for entry in entries:
            require(entry.isfile() and not entry.linkname, 'nonregular/linked archive member: '+entry.name)
            require(entry.uname == 'root' and entry.gname == 'wheel', 'wrong archive owner: '+entry.name)
            expected_mode = 0o755 if entry.name in executable_paths else 0o644
            require(entry.mode == expected_mode, 'wrong archive mode: '+entry.name)
            # Native pkg2.8.4 packing.c uses portable nobody IDs for payloads,
            # while the root/wheel names govern ownership; metadata IDs are 0.
            expected_id = 0 if entry.name in METADATA else 65534
            require(entry.uid == expected_id and entry.gid == expected_id, 'unexpected native archive IDs: '+entry.name)
        def data(path):
            return package.extractfile(by_path[path]).read()
        manifest = load_json(data('+MANIFEST'))
        compact = load_json(data('+COMPACT_MANIFEST'))
        require(compact == {key: value for key, value in manifest.items() if key != 'files'}, 'compact/full metadata mismatch')
        require(manifest['name'] == 'dtail', 'unexpected package name')
        require(manifest['origin'] == 'local/dtail' and manifest['prefix'] == '/usr/local', 'unexpected origin/prefix')
        require(not manifest.get('deps'), 'package must not add dependencies')
        require(manifest['version'] == info['package_version'], 'package version mismatch')
        require(manifest['abi'] == 'FreeBSD:15:amd64', 'unexpected target ABI')
        require(manifest['annotations']['dtail_source_commit'] == info['source_commit'], 'source annotation mismatch')
        require(not manifest.get('scripts'), 'package must not run installation/service scripts')
        require(set(manifest['files']) == set(PATHS.values()), 'unexpected or missing packaged files')
        for name, path in PATHS.items():
            content = data(path)
            require(content == (bundle/name).read_bytes(), 'package/bundle mismatch: ' + path)
            attrs = manifest['files'][path]
            require(attrs['uname'] == 'root' and attrs['gname'] == 'wheel', 'wrong manifest file ownership: '+path)
            expected_mode = '0755' if path in executable_paths else '0644'
            require(attrs['perm'] == expected_mode, 'wrong manifest file mode: '+path)
            if name != 'build-info.json':
                require(hashlib.sha256(content).hexdigest() == info['payload_sha256'][name], 'payload digest mismatch: '+name)
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
            'archive_headers_and_compact_metadata_match': True,
            'actual_host_config_not_shipped': True,
            'verification_performs_no_install_or_publish': True}


if __name__ == '__main__':
    if len(sys.argv) != 3:
        sys.exit('Usage: verify-dtail-freebsd.py BUNDLE PACKAGE')
    try:
        print(json.dumps(verify(Path(sys.argv[1]), Path(sys.argv[2])), indent=2))
    except (KeyError, IndexError, ValueError, OSError, tarfile.TarError, subprocess.CalledProcessError) as error:
        sys.exit('Package verification failed: ' + str(error))
