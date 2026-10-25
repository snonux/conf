"""Real Git/Go build tests; native pkg integration runs separately on FreeBSD."""
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import tarfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
BUILDER = ROOT / 'packages/scripts/build-dtail-freebsd.py'
NATIVE = ROOT / 'packages/scripts/pkg-dtail-freebsd.sh'
VERIFY_PATH = ROOT / 'packages/scripts/verify-dtail-freebsd.py'
spec = importlib.util.spec_from_file_location('verify_dtail', VERIFY_PATH)
VERIFY = importlib.util.module_from_spec(spec)
spec.loader.exec_module(VERIFY)
BINS = ('dserver', 'dcat', 'dgrep', 'dmap', 'dtail', 'dtailhealth')


class PinnedBuild(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        if not shutil.which('go'):
            raise RuntimeError('actual Go toolchain is required; do not treat this as a skipped pass')
        cls.temporary = tempfile.TemporaryDirectory(prefix='mp3-build-test-', dir='/var/tmp')
        cls.base = Path(cls.temporary.name)
        cls.source = cls.base / 'source'
        cls.source.mkdir()
        def git(*args):
            return subprocess.check_output(['git', '-C', str(cls.source), *args], text=True).strip()
        git('init', '-q')
        git('config', 'user.name', 'Package fixture')
        git('config', 'user.email', 'fixture@example.invalid')
        (cls.source / 'go.mod').write_text('module example.org/mp3fixture\n\ngo 1.23\n')
        version = cls.source / 'internal/version';version.mkdir(parents=True)
        (version / 'version.go').write_text('package version\nconst (\n Version string = "4.3.2-ng"\n)\n')
        for name in BINS:
            path = cls.source / 'cmd' / name;path.mkdir(parents=True)
            (path / 'main.go').write_text('package main\nimport "fmt"\nfunc main(){fmt.Println("mp3-fixture-A")}\n')
        git('add', '.');git('commit', '-qm', 'source A')
        cls.commit_a = git('rev-parse', 'HEAD')
        for name in BINS:
            p = cls.source / 'cmd' / name / 'main.go';p.write_text(p.read_text().replace('fixture-A', 'fixture-B'))
        git('add', '.');git('commit', '-qm', 'source B')
        cls.commit_b = git('rev-parse', 'HEAD')
        # Deliberately unbuildable original WIP must never enter either build.
        (cls.source / 'cmd/dserver/main.go').write_text('uncommitted syntax error\n')
        (cls.source / 'untracked.txt').write_text('preserve this fixture WIP\n')
        cls.before = git('status', '--porcelain')
        for label, commit, revision in [('a', cls.commit_a, '1'), ('b', cls.commit_b, '2')]:
            output = cls.base / label
            subprocess.run(['python3', str(BUILDER), '--source', str(cls.source),
                            '--ref', commit, '--revision', revision, '--output', str(output)],
                           capture_output=True, check=True, timeout=120)
        cls.after = git('status', '--porcelain')

    @classmethod
    def tearDownClass(cls):
        cls.temporary.cleanup()

    def invoke(self, ref='missing-ref', revision='1', output=None):
        return subprocess.run(['python3', str(BUILDER), '--source', str(self.source),
                               '--ref', ref, '--revision', revision,
                               '--output', str(output or self.base/'rejected')],
                              capture_output=True, timeout=20)

    def test_selected_commit_compiles_real_code_and_embeds_vcs(self):
        for label, commit, marker, absent in [('a', self.commit_a, b'mp3-fixture-A', b'mp3-fixture-B'),
                                              ('b', self.commit_b, b'mp3-fixture-B', b'mp3-fixture-A')]:
            folder = self.base / label
            info = json.loads((folder/'build-info.json').read_text())
            self.assertEqual(info['source_commit'], commit)
            for name in BINS:
                binary = (folder/name).read_bytes()
                self.assertIn(marker, binary)
                self.assertNotIn(absent, binary)
                metadata = subprocess.check_output(['go', 'version', '-m', str(folder/name)], text=True)
                self.assertIn('vcs.revision='+commit, metadata)
                self.assertIn('vcs.modified=false', metadata)
                self.assertIn('GOOS=freebsd', metadata)
                self.assertIn('CGO_ENABLED=0', metadata)
        self.assertEqual(self.before, self.after)
        self.assertEqual((self.source/'untracked.txt').read_text(), 'preserve this fixture WIP\n')
        self.assertEqual((self.source/'cmd/dserver/main.go').read_text(), 'uncommitted syntax error\n')

    def test_revision_and_package_provenance(self):
        for label, revision in [('a', 1), ('b', 2)]:
            folder=self.base/label
            manifest=json.loads((folder/'+MANIFEST').read_text())
            info=json.loads((folder/'build-info.json').read_text())
            self.assertEqual(manifest['version'], '4.3.2-ng_'+str(revision))
            self.assertEqual(manifest['version'], info['package_version'])
            self.assertEqual(manifest['annotations']['dtail_source_commit'], info['source_commit'])
            for name, digest in info['payload_sha256'].items():
                self.assertEqual(hashlib.sha256((folder/name).read_bytes()).hexdigest(), digest)
            records=[line.split() for line in (folder/'SHA256').read_text().splitlines()]
            self.assertEqual(len(records), 11)
            self.assertEqual(len({name for _, name in records}), 11)
            for digest,name in records:
                self.assertEqual(hashlib.sha256((folder/name).read_bytes()).hexdigest(), digest)

    def test_current_security_assets_and_sample_only_config(self):
        folder=self.base/'a'
        config=json.loads((folder/'dtail.json.sample').read_text())
        self.assertEqual(config['Server']['HostKeyFile'], '/var/db/dserver/ssh_host_key')
        self.assertEqual(config['Server']['Permissions']['Default'],
                         ['readfiles:^/var/log/.*$', 'readfiles:^/var/www/logs/.*$'])
        self.assertFalse((folder/'dtail.json').exists())
        self.assertEqual((folder/'dserver-update-key-cache.sh').read_bytes(),
                         (ROOT/'frontends/scripts/dserver-update-key-cache-freebsd.sh.tpl').read_bytes())
        self.assertEqual((folder/'dserver.rc').read_bytes(),
                         (ROOT/'frontends/etc/rc.d/dserver-freebsd.tpl').read_bytes())

    def test_bad_ref_and_revision_fail_before_output_creation(self):
        for ref,revision in [('missing-ref','1'),(self.commit_a,'0'),(self.commit_a,'-1'),
                             (self.commit_a,'1;touch BAD'),(self.commit_a,'01')]:
            output=self.base/('bad-'+str(len(ref))+revision.replace('/','_'))
            self.assertNotEqual(self.invoke(ref,revision,output).returncode, 0)
            self.assertFalse(output.exists())

    def test_existing_output_is_never_removed(self):
        output=self.base/'keep';output.mkdir()
        sentinel=output/'sentinel';sentinel.write_text('untouched\n')
        self.assertNotEqual(self.invoke(self.commit_a,'1',output).returncode,0)
        self.assertEqual(sentinel.read_text(),'untouched\n')

    def test_build_and_package_make_targets_do_not_publish(self):
        for target in ['dtail-freebsd-build','dtail-freebsd-package','dtail-freebsd']:
            p=subprocess.run(['make','-n','-C',str(ROOT/'packages'),target,
                              'DTAIL_FREEBSD_BUNDLE='+str(self.base/'a')],capture_output=True,text=True,check=True)
            self.assertNotIn('/data/nfs/k3svolumes/pkgrepo',p.stdout)
            self.assertNotIn('doas ',p.stdout)
            if target.endswith('-build'):
                self.assertNotIn('scp ',p.stdout);self.assertNotIn('ssh ',p.stdout)

    def fixture_archive(self, name, mutation=None):
        bundle=self.base/'a'
        info=json.loads((bundle/'build-info.json').read_text())
        manifest={'name':'dtail','origin':'local/dtail','prefix':'/usr/local','version':info['package_version'],'abi':'FreeBSD:15:amd64',
                  'annotations':{'dtail_source_commit':info['source_commit']},
                  'files':{path:{'uname':'root','gname':'wheel',
                     'perm':'0755' if key in BINS or key in ('dserver.rc','dserver-update-key-cache.sh') else '0644'}
                     for key,path in VERIFY.PATHS.items()}}
        payload={path:(bundle/key).read_bytes() for key,path in VERIFY.PATHS.items()}
        duplicate=None
        if mutation=='payload':payload['/usr/local/bin/dserver']=b'changed data'
        elif mutation=='owner':manifest['files']['/usr/local/bin/dserver']['uname']='nobody'
        elif mutation=='deps':manifest['deps']={'unexpected':{'origin':'local/unexpected','version':'1'}}
        elif mutation=='scripts':manifest['scripts']={'post-install':'false'}
        elif mutation=='source':manifest['annotations']['dtail_source_commit']='f'*40
        elif mutation=='version':manifest['version']='4.3.2-ng'
        elif mutation=='live-config':
            path='/usr/local/etc/dserver/dtail.json'
            payload[path]=payload.pop(path+'.sample')
            manifest['files'][path]=manifest['files'].pop(path+'.sample')
        elif mutation=='extra':payload['/unrelated-file']=b'not allowed'
        elif mutation=='duplicate':duplicate='/usr/local/bin/dserver'
        archive=self.base/name
        with tarfile.open(archive,'w') as tar:
            for path,data in {'+MANIFEST':json.dumps(manifest).encode(),
                              '+COMPACT_MANIFEST':json.dumps(manifest).encode(),**payload}.items():
                entry=tarfile.TarInfo(path);entry.size=len(data);tar.addfile(entry,io.BytesIO(data))
            if duplicate:
                entry=tarfile.TarInfo(duplicate);entry.size=len(payload[duplicate]);tar.addfile(entry,io.BytesIO(payload[duplicate]))
        return archive

    def test_package_verifier_reads_real_archive_and_build_metadata(self):
        archive=self.fixture_archive('valid.tar')
        result=VERIFY.verify(self.base/'a',archive)
        self.assertEqual(result['source_commit'],self.commit_a)
        self.assertTrue(result['all_payload_bytes_match'])
        self.assertTrue(result['actual_host_config_not_shipped'])

    def test_package_verifier_rejects_changed_or_extra_content(self):
        for mutation in ['payload','owner','deps','scripts','source','version','live-config','extra','duplicate']:
            with self.subTest(mutation=mutation):
                archive=self.fixture_archive(mutation+'.tar',mutation)
                with self.assertRaises(ValueError):VERIFY.verify(self.base/'a',archive)
        archive=self.fixture_archive('optimized-invalid.tar','owner')
        p=subprocess.run(['python3','-O',str(VERIFY_PATH),str(self.base/'a'),str(archive)],capture_output=True)
        self.assertNotEqual(p.returncode,0)

    def test_native_invalid_invocations_fail_without_tools_or_writes(self):
        for args in [[],['publish'],['assemble','/nonexistent',str(self.base/'bad-output')]]:
            p=subprocess.run(['sh',str(NATIVE),*args],capture_output=True)
            self.assertNotEqual(p.returncode,0)
        self.assertFalse((self.base/'bad-output').exists())


if __name__ == '__main__':
    unittest.main()
