#!/usr/bin/env python3
"""Exercise the real NetBSD dispatcher with disposable command/kernel fixtures."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import time

source = Path(__file__).with_name('migrate.sh').read_text()
with tempfile.TemporaryDirectory(prefix='netbsd-network-fixture-') as directory:
    root = Path(directory)
    state = root / 'state'
    state.mkdir()
    commands = root / 'bin'
    commands.mkdir()
    model_file = root / 'model.json'
    script = root / 'helper'
    config = root / 'config'
    config.write_text("host=controller\niface=mue0\nold=192.0.2.125\nlab=198.51.100.125\n"
                      "recovery=203.0.113.125\nula=fd00::125\nshared_gateway=192.0.2.2\n")
    candidate = root / 'candidate'
    candidate.write_text('reviewed new NPF policy')
    npf = root / 'npf.conf'
    (root / 'bootrc').write_text('fixture boot callback')
    (root / 'bootrc').chmod(0o755)
    (root / 'bootflag').write_text('gonf_network_migration=YES\n')
    text = source.replace('/var/db/gonf-network-migration', str(state))
    for old, new in [('/usr/local/sbin/gonf-network-migration', script),
                     ('/usr/local/etc/gonf-network-migration.conf', config),
                     ('/usr/local/etc/gonf-network-migration.npf', candidate),
                     ('/etc/npf.conf', npf),
                     ('/etc/rc.d/gonf_network_migration', root / 'bootrc'),
                     ('/etc/rc.conf.d/gonf_network_migration', root / 'bootflag'),
                     ('/etc/rc.conf', root / 'rc.conf')]:
        text = text.replace(old, str(new))
    text = text.replace('PATH=/sbin:/bin:/usr/sbin:/usr/bin:/usr/pkg/sbin:/usr/pkg/bin:/usr/local/sbin',
                        f'PATH={commands}:/usr/bin:/bin')
    script.write_text(text)
    script.chmod(0o755)
    mock = '''#!/usr/bin/env python3
import json,os,sys,time
from pathlib import Path
r=Path(os.environ['FIXTURE_ROOT']);path=r/'model.json';m=json.loads(path.read_text())
name=Path(sys.argv[0]).name;args=sys.argv[1:]
with (r/'calls').open('a')as f:f.write(json.dumps([name,*args])+'\\n')
def save():
 p=r/'model.next';p.write_text(json.dumps(m));p.replace(path)
if name=='id':print(0)
elif name=='uname':print('NetBSD')
elif name=='hostname':print('controller')
elif name=='date':print(m['clock'])
elif name=='stat':print('0:'+oct(Path(args[-1]).stat().st_mode&0o777)[2:])
elif name=='sync':pass
elif name=='ndp':print('Flags: nud auto_linklocal')
elif name=='pgrep':
 # NetBSD's observed process name is /usr/sbin/cron, not Linux's basename.
 valid=args==['-f','^/usr/sbin/cron($|[[:space:]])']
 sys.exit(0 if m['cron']and valid else 1)
elif name=='crontab':print(str(r/'helper')+' watchdog' if m['cron']else '')
elif name=='ifconfig':
 if args==['mue0']:
  print('mue0: flags=UP,RUNNING');print(' inet6 fe80::1/64 flags 0')
  for a in m['v4']:print(' inet '+a+' flags 0')
  if m['ula']:print(' inet6 fd00::125/64 flags 0')
 elif args[0]=='mue0'and args[1]in('inet','inet6'):
  if m.get('delay'):
   m['delay']=False;save();time.sleep(1)
  family,address,action=args[1:]
  if family=='inet':
   if action=='alias'and address not in m['v4']:m['v4'].append(address)
   elif action=='delete':
    primary=m['v4'][0].split('/')[0]if m['v4']else None
    m['v4']=[a for a in m['v4']if a.split('/')[0]!=address]
    # Native NetBSD removed both routes when the primary address was deleted.
    if address==primary:m['gateway']=None;m['return']='absent'
  else:m['ula']=action=='alias'
  save()
 else:raise AssertionError(args)
elif name=='route':
 a=[x for x in args if x!='-n']
 if a==['get','default']:
  if m['gateway']is None:sys.exit(1)
  print('destination: default\\ngateway: '+m['gateway'])
 elif a==['get','10.99.0.2']:
  if m['return']=='absent'and m['gateway']is None:sys.exit(1)
  print('destination: '+('default'if m['return']=='absent'else '10.99.0.2')+'\\ngateway: '+m['return'])
 elif a[:2]==['change','default']:
  if m['gateway']is None:sys.exit(1)
  m['gateway']=a[2];save()
 elif a[:2]==['add','default']:
  if m['gateway']is not None:sys.exit(1)
  m['gateway']=a[2];save()
 elif len(a)>=3 and a[1:3]==['-host','10.99.0.2']:
  m['return']='absent'if a[0]=='delete'else a[3];save()
 else:raise AssertionError(args)
elif name=='npfctl':
 if args==['show']:print('# filtering: '+('active'if m['npf_active']else 'inactive'))
 elif args[0]=='validate':sys.exit(7 if m.get('reject_validate')else 0)
 elif args==['reload']:
  if m.get('reject_reload'):sys.exit(8)
  m['loaded_npf']=(r/'npf.conf').read_text();save()
 else:raise AssertionError(args)
else:raise AssertionError(name)
'''
    for name in ('id', 'uname', 'hostname', 'date', 'stat', 'sync', 'ndp', 'pgrep',
                 'crontab', 'ifconfig', 'route', 'npfctl'):
        p = commands / name
        p.write_text(mock)
        p.chmod(0o755)
    env = dict(os.environ, FIXTURE_ROOT=str(root))

    def model():
        return json.loads(model_file.read_text())

    def edit(**kwargs):
        m = model()
        m.update(kwargs)
        model_file.write_text(json.dumps(m))

    def reset():
        for p in state.iterdir():
            if p.is_dir():
                shutil.rmtree(p)
            else:
                p.unlink()
        npf.write_text('original NPF policy')
        model_file.write_text(json.dumps({'clock': 10000, 'v4': ['192.0.2.125/24'],
                                         'ula': False, 'gateway': '192.0.2.1',
                                         'return': 'absent', 'npf_active': True,
                                         'loaded_npf': 'original NPF policy', 'cron': True,
                                         'delay': False}))

    def gate(name, value='controller', mode=0o600):
        p = state / name
        p.write_text(value)
        p.chmod(mode)

    def run(*args, success=True):
        p = subprocess.run(['sh', str(script), *args], env=env, capture_output=True,
                           text=True, timeout=5)
        assert (p.returncode == 0) == success, (args, p.stdout, p.stderr)
        return p.stdout

    def request(action, success=True, topology='shared'):
        before = set(state.glob('request.*'))
        start = time.monotonic()
        run('request', action, topology)
        elapsed = time.monotonic() - start
        created = set(state.glob('request.*')) - before
        assert len(created) == 1
        folder = created.pop()
        for _ in range(200):
            if (folder / 'result').exists():
                break
            time.sleep(.025)
        assert (folder / 'result').is_file(), folder
        rc = int((folder / 'result').read_text())
        assert (rc == 0) == success, (folder, rc, (folder / 'log').read_text())
        return elapsed

    def stage():
        gate('activation-approved')
        gate('stage-approved')
        request('stage')

    def confirm():
        gate('confirm-approved')
        run('confirm')

    pending = state / 'pending'
    reset()
    original = model()
    request('stage', success=False)
    assert model() == original and not pending.exists()
    for value, mode in [('foreign', 0o600), ('controller', 0o644)]:
        gate('activation-approved', value, mode)
        gate('stage-approved')
        request('stage', success=False)
        assert model() == original and not pending.exists()
    reset()
    edit(npf_active=False)
    gate('activation-approved'); gate('stage-approved')
    request('stage', success=False)
    assert not pending.exists(), 'inactive must not match active'
    reset()
    edit(cron=False)
    gate('activation-approved'); gate('stage-approved')
    request('stage', success=False)
    assert not pending.exists()

    reset()
    edit(delay=True)
    gate('activation-approved'); gate('stage-approved')
    assert request('stage') < .8, 'request waited on its child while holding flock'
    assert (pending / 'deadline').read_text().strip() == '10900'
    assert model()['v4'] == ['192.0.2.125/24', '198.51.100.125/24']
    assert model()['ula'] and model()['return'] == '192.0.2.2'
    edit(clock=10901)
    run('confirm', success=False)
    run('watchdog')
    m = model();m['clock'] = 10000
    assert m == original and not pending.exists()
    assert npf.read_text() == 'original NPF policy'
    run('watchdog'); request('rollback')  # Repeated recovery is a no-op.

    reset()
    stage(); confirm()
    stage_state = model()
    assert (state / 'accepted').read_text().strip() == 'stage shared'
    gate('promote-approved')
    request('promote')
    assert model()['v4'] == ['198.51.100.125/24', '192.0.2.125/24']
    assert model()['gateway'] == '10.20.0.1'
    assert model()['return'] == '192.0.2.2', 'primary deletion lost controller route'
    edit(clock=10901)
    run('watchdog')
    edit(clock=10000)
    assert model() == stage_state, 'promotion expiry failed after native route deletion'
    gate('promote-approved'); request('promote')
    # Model a crash after publishing NEW but before removing pending. The real
    # boot branch must prefer the complete pending snapshot, not this file.
    (state / 'accepted').write_text('promote shared\n')
    edit(v4=['192.0.2.125/24'], ula=False, gateway='192.0.2.1',
         npf_active=False, loaded_npf='not yet loaded at boot')
    run('boot')
    assert (state / 'accepted').read_text().strip() == 'stage shared'
    assert not pending.exists()
    edit(npf_active=True, loaded_npf=npf.read_text())
    assert model() == stage_state
    gate('promote-approved'); request('promote'); confirm()
    promoted = model()
    edit(v4=['192.0.2.125/24'], ula=False, gateway='192.0.2.1',
         npf_active=False, loaded_npf='not yet loaded at boot')
    run('boot')
    edit(npf_active=True, loaded_npf=npf.read_text())
    assert model() == promoted, 'confirmed boot did not reapply NEW'
    gate('stage-approved'); request('stage', success=False)
    assert not pending.exists() and model() == promoted

    # A rack already promoted on the shared wire must be able to add the
    # dedicated recovery path without downgrading its primary through Stage.
    gate('promote-approved')
    request('promote', success=False, topology='isolated')
    assert not pending.exists() and model() == promoted, 'missing igc1 gate mutated NEW'
    gate('igc1-approved')
    request('promote', topology='isolated')
    isolated = model()
    assert isolated['v4'] == ['198.51.100.125/24', '192.0.2.125/24', '203.0.113.125/24']
    assert isolated['gateway'] == promoted['gateway'] == '10.20.0.1'
    assert isolated['return'] == '10.98.0.1' and isolated['ula']
    edit(clock=10901)
    run('watchdog'); edit(clock=10000)
    assert model() == promoted, 'recovery expiry did not restore promoted shared profile'
    assert (state / 'accepted').read_text().strip() == 'promote shared'
    gate('promote-approved'); request('promote', topology='isolated')
    run('boot')
    assert model() == promoted, 'unconfirmed recovery boot did not restore shared NEW'
    gate('promote-approved'); request('promote', topology='isolated'); confirm()
    edit(v4=['192.0.2.125/24'], ula=False, gateway='192.0.2.1',
         npf_active=False, loaded_npf='not yet loaded at boot')
    run('boot')
    booted = model()
    assert booted['v4'][0] == '198.51.100.125/24'
    assert set(booted['v4']) == set(isolated['v4'])
    assert booted['gateway'] == '10.20.0.1' and booted['return'] == '10.98.0.1'
    assert (state / 'accepted').read_text().strip() == 'promote isolated'

    # i24 retire: an accepted promote isolated profile drops OLD for good,
    # at runtime and in rc.conf's base primary/defaultroute lines.
    rcfile = root / 'rc.conf'
    rc_old = ('hostname="controller"\ndhcpcd=NO\n'
              'ifconfig_mue0="inet 192.0.2.125 netmask 0xffffff00"\ndefaultroute="192.0.2.1"\n')
    rc_new = ('hostname="controller"\ndhcpcd=NO\n'
              'ifconfig_mue0="inet 198.51.100.125 netmask 0xffffff00"\ndefaultroute="10.20.0.1"\n')
    rcfile.write_text(rc_old)
    edit(npf_active=True, loaded_npf=npf.read_text())  # NPF loads after the boot hook.
    booted = model()
    for leftover in ('retire-approved', 'confirm-approved'):
        (state / leftover).unlink(missing_ok=True)

    def unchanged(before):
        assert not pending.exists() and model() == before and rcfile.read_text() == rc_old
        assert (state / 'accepted').read_text().strip() == 'promote isolated'

    request('retire', success=False)  # The retire gate is closed by default.
    unchanged(booted)
    gate('retire-approved'); (state / 'igc1-approved').unlink()
    request('retire', success=False)
    unchanged(booted)
    gate('igc1-approved')
    for bad in ('ifconfig_mue0="inet 192.0.2.99 netmask 0xffffff00"\ndefaultroute="192.0.2.1"\n',
                'ifconfig_mue0="inet 192.0.2.125 netmask 0xffffff00"\n',
                rc_old + 'ifconfig_mue0="inet 192.0.2.125 netmask 0xffffff00"\n',
                rc_old + 'static_routes="x"\nroute_x="-net 172.16.0.0/12 192.0.2.1"\n'):
        rcfile.write_text(bad)
        request('retire', success=False)
        assert not pending.exists() and model() == booted and rcfile.read_text() == bad, bad
    rcfile.write_text(rc_old)
    for field, value in (('gateway', '192.0.2.1'), ('return', '192.0.2.2')):
        edit(**{field: value})
        request('retire', success=False)
        assert not pending.exists() and rcfile.read_text() == rc_old, field
        edit(**{field: booted[field]})
    edit(v4=['192.0.2.125/24', '198.51.100.125/24', '203.0.113.125/24'])
    request('retire', success=False)  # OLD is still the primary.
    edit(v4=booted['v4'])
    unchanged(booted)

    def retired_model(m):
        assert m['v4'] == ['198.51.100.125/24', '203.0.113.125/24'], m
        assert m['gateway'] == '10.20.0.1' and m['return'] == '10.98.0.1' and m['ula'], m

    def restored_model(m):
        assert m['v4'][0] == '198.51.100.125/24' and set(m['v4']) == set(booted['v4']), m
        assert m['gateway'] == '10.20.0.1' and m['return'] == '10.98.0.1' and m['ula'], m
        assert rcfile.read_text() == rc_old
        assert (state / 'accepted').read_text().strip() == 'promote isolated'
        assert not pending.exists()

    request('retire')
    retired_model(model())
    assert rcfile.read_text() == rc_new, rcfile.read_text()
    assert not (state / "retire-approved").exists()
    request('rollback')
    restored_model(model())
    # Expiry through the independent watchdog.
    gate('retire-approved'); request('retire'); edit(clock=10901)
    run('confirm', success=False)
    run('watchdog'); edit(clock=10000)
    restored_model(model())
    # Unconfirmed boot: base networking applied the retired rc.conf first.
    gate('retire-approved'); request('retire')
    edit(v4=['198.51.100.125/24'], ula=False, npf_active=False, **{'return': 'absent'})
    run('boot')
    edit(npf_active=True)
    restored_model(model())
    # Another owner's rc.conf edit keeps the journal instead of overwriting it.
    gate('retire-approved'); request('retire')
    rcfile.write_text(rc_new + 'foreign="edit"\n')
    edit(clock=10901)
    run('watchdog', success=False)
    assert pending.exists() and 'foreign' in rcfile.read_text()
    rcfile.write_text(rc_new)
    run('watchdog'); edit(clock=10000)
    restored_model(model())
    # Confirmation persists retire isolated; boots never re-add OLD.
    gate('retire-approved'); request('retire'); confirm()
    assert (state / 'accepted').read_text().strip() == 'retire isolated'
    assert rcfile.read_text() == rc_new
    edit(v4=['198.51.100.125/24'], ula=False, npf_active=False, **{'return': 'absent'})
    run('boot')
    edit(npf_active=True)
    retired_model(model())
    retired = model()
    request('rollback')  # Nothing pending: a no-op.
    gate('promote-approved'); request('promote', success=False, topology='isolated')
    gate('retire-approved'); request('retire', success=False)
    assert model() == retired and not pending.exists() and rcfile.read_text() == rc_new
    print('PASS: retire gates/preconditions/rc.conf shape, OLD-free runtime and rc.conf, '
          'rollback/expiry/unconfirmed boot/foreign rc.conf edit, confirmed boots stay retired')

    # The new transition is narrow: a claimed prior promotion cannot replace
    # the required staged lab address or accept an unrelated profile state.
    for invalid in ('promote shared', 'promote unrelated'):
        reset()
        (state / 'accepted').write_text(invalid+'\n')
        if invalid == 'promote unrelated':
            edit(v4=['198.51.100.125/24', '192.0.2.125/24'], ula=True,
                 gateway='10.20.0.1', **{'return': '192.0.2.2'})
            # Keep the otherwise valid promoted state; only its accepted
            # profile is invalid, so a missing lab alias cannot mask admission.
        gate('activation-approved'); gate('promote-approved'); gate('igc1-approved')
        before_invalid = model()
        request('promote', success=False, topology='isolated')
        assert not pending.exists() and model() == before_invalid

    reset()
    edit(reject_reload=True)
    gate('activation-approved'); gate('stage-approved')
    request('stage', success=False)
    assert pending.exists(), 'partial failure lost its complete recovery journal'
    edit(reject_reload=False, clock=10901)
    run('watchdog')
    assert not pending.exists() and npf.read_text() == 'original NPF policy'
    reset()
    stage()
    npf.write_text('foreign NPF edit')
    edit(clock=10901)
    run('watchdog', success=False)
    assert pending.exists() and npf.read_text() == 'foreign NPF edit'
    print('PASS: actual request/flock dispatch, gates, immutable900s deadline, '
          'expiry/NPF restore, shared-NEW to isolated recovery, partial failure, '
          'unconfirmed/confirmed boot and foreign-edit refusal')
