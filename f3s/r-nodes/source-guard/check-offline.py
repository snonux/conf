#!/usr/bin/env python3
"""Exercise actual migration nft files only in disposable network namespaces."""
import os
import json
from pathlib import Path
import socket
import subprocess
import sys
import time


def run(*args):
    result = subprocess.run(args, capture_output=True, text=True)
    if result.returncode:
        raise RuntimeError(f'{args}: {result.stdout}{result.stderr}')
    return result


if '--inside' not in sys.argv:
    os.execvp('unshare', ['unshare', '-Urn', '--', sys.executable,
                         str(Path(__file__).resolve()), '--inside',
                         os.readlink('/proc/self/ns/net'),
                         os.readlink('/proc/self/ns/user')])
assert os.readlink('/proc/self/ns/net') != sys.argv[2]
assert os.readlink('/proc/self/ns/user') != sys.argv[3]
root = Path(__file__).resolve().parents[2]
client = subprocess.Popen(['unshare', '-n', 'sleep', '180'])
server = None
try:
    for _ in range(100):
        if os.readlink(f'/proc/{client.pid}/ns/net') != os.readlink('/proc/self/ns/net'):
            break
        time.sleep(.01)
    ns = ['nsenter', '-t', str(client.pid), '-n']
    run('ip', 'link', 'add', 'enp0s5', 'type', 'veth', 'peer', 'name', 'client0')
    run('ip', 'link', 'set', 'client0', 'netns', str(client.pid))
    run('ip', 'link', 'set', 'enp0s5', 'up')
    run(*ns, 'ip', 'link', 'set', 'client0', 'up')
    destinations = ['10.20.0.120', '192.168.1.120', '10.98.0.120']
    sources = ['10.20.0.130', '192.168.1.130', '192.168.1.236',
               '192.168.1.50', '10.99.0.2', '10.99.0.3', '10.99.0.4',
               '10.30.0.2', '10.40.0.11', '10.42.0.2']
    for address in destinations:
        run('ip', 'addr', 'add', address + '/32', 'dev', 'enp0s5')
    for address in sources:
        run(*ns, 'ip', 'addr', 'add', address + '/32', 'dev', 'client0')
        run('ip', 'route', 'add', address + '/32', 'dev', 'enp0s5')
    for address in destinations + ['10.20.0.121', '10.98.0.122', '192.168.2.121']:
        run(*ns, 'ip', 'route', 'add', address + '/32', 'dev', 'client0')
    for address in ['fd02:ca0b:f1cb:20::120', 'fd22:c702:acb7::120']:
        run('ip', '-6', 'addr', 'add', address + '/64', 'dev', 'enp0s5', 'nodad')
    for address in ['fd02:ca0b:f1cb:20::130', 'fd22:c702:acb7::130']:
        run(*ns, 'ip', '-6', 'addr', 'add', address + '/64', 'dev', 'client0', 'nodad')
    # Listen on real host destinations; connect success proves the SYN passes.
    server = subprocess.Popen([sys.executable, '-c', '''
import selectors, socket
sel = selectors.DefaultSelector()
for port in (22, 80, 443, 6443, 30001, 32200, 9999):
    s = socket.socket(socket.AF_INET6)
    s.setsockopt(socket.IPPROTO_IPV6, socket.IPV6_V6ONLY, 0)
    s.bind(('::', port)); s.listen(); sel.register(s, selectors.EVENT_READ)
while True:
    for key, _ in sel.select():
        conn, _ = key.fileobj.accept(); conn.close()
'''])
    time.sleep(.15)
    host = root / 'r-nodes/source-guard/migration.nft'
    pod = root / 'r-nodes/nodeport-guard/migration.nft'
    run('nft', 'add', 'table', 'inet', 'unrelated_fixture')
    for path in [host, pod, host, pod]:
        run('nft', '-c', '-f', str(path))
        run('nft', '-f', str(path))

    run('nft', 'list', 'table', 'inet', 'unrelated_fixture')
    saved_pod = run('nft', 'list', 'table', 'inet', 'f3s_nodeport_guard').stdout
    run('nft', '-f', str(host))
    assert saved_pod == run('nft', 'list', 'table', 'inet', 'f3s_nodeport_guard').stdout

    def probe(source, target, port, allowed):
        code = '''import socket,sys
s=socket.socket(socket.AF_INET6 if ':' in sys.argv[1] else socket.AF_INET)
s.settimeout(.25); s.bind((sys.argv[1],0))
try: s.connect((sys.argv[2],int(sys.argv[3]))); ok=True
except (TimeoutError,OSError): ok=False
assert ok == (sys.argv[4]=='True'), sys.argv[1:]
'''
        run(*ns, sys.executable, '-c', code, source, target, str(port), str(allowed))

    for source in ['10.20.0.130', '192.168.1.130']:
        for target in destinations[:2]:
            probe(source, target, 9999, True)
        probe(source, destinations[2], 22, False)
    probe('192.168.1.236', destinations[0], 22, True)
    probe('192.168.1.236', destinations[0], 6443, False)
    for source in ['192.168.1.50', '10.99.0.4', '10.30.0.2', '10.40.0.11']:
        probe(source, destinations[0], 443, False)
    # n44: the laptop also reaches the Syncthing NodePort 32200, on lab
    # addresses only (not old LAN or recovery aliases); the phone does not.
    for port in [22, 80, 443, 6443, 30001, 32200]:
        probe('10.99.0.2', destinations[0], port, port in [22, 80, 443, 32200])
        probe('10.99.0.3', destinations[0], port, port == 443)
        probe('10.99.0.2', destinations[2], port, port == 22)
        probe('10.99.0.3', destinations[2], port, False)
    probe('10.99.0.2', destinations[1], 32200, False)
    for source in ['10.99.0.4', '192.168.1.50', '10.30.0.2']:
        probe(source, destinations[0], 32200, False)
    probe('fd02:ca0b:f1cb:20::130', 'fd02:ca0b:f1cb:20::120', 9999, True)
    probe('fd22:c702:acb7::130', 'fd02:ca0b:f1cb:20::120', 9999, True)
    # Verify the pod guard independently, with the host table removed.
    run('nft', 'delete', 'table', 'inet', 'f3s_guard')
    for target in destinations:
        probe('10.42.0.2', target, 30001, False)
        probe('10.42.0.2', target, 9999, True)
    # Trace the independent guard counters for remote aliases before routing.
    mac = json.loads(run('ip', '-j', 'link', 'show', 'enp0s5').stdout)[0]['address']
    for target in ['10.20.0.121', '10.98.0.122', '192.168.2.121']:
        run(*ns, 'ip', 'neigh', 'replace', target, 'lladdr', mac, 'dev', 'client0')
        before = run('nft', 'list', 'chain', 'inet', 'f3s_nodeport_guard', 'from_pod').stdout
        probe('10.42.0.2', target, 30001, False)
        after = run('nft', 'list', 'chain', 'inet', 'f3s_nodeport_guard', 'from_pod').stdout
        assert before != after, target
    print('PASS: native syntax, atomic replacement, mixed addressing, wg2/recovery scope (incl. n44 32200), ULA, independent local/remote pod guard')
finally:
    if server:
        server.terminate()
        server.wait()
    client.terminate()
    client.wait()
