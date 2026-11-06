#!/usr/bin/env python3
"""Check the real resolver policy in disposable user/network namespaces."""
import os
from pathlib import Path
import subprocess
import sys
import time


def run(*args):
    p = subprocess.run(args, capture_output=True, text=True, timeout=10)
    assert p.returncode == 0, (args, p.stdout, p.stderr)
    return p.stdout


if '--inside' not in sys.argv:
    os.execvp('unshare', ['unshare', '-Urn', '--', sys.executable,
                         str(Path(__file__).resolve()), '--inside',
                         os.readlink('/proc/self/ns/net'),
                         os.readlink('/proc/self/ns/user')])
assert os.readlink('/proc/self/ns/net') != sys.argv[2]
assert os.readlink('/proc/self/ns/user') != sys.argv[3]
client = subprocess.Popen(['unshare', '-n', 'sleep', '180'])
server = None
try:
    for _ in range(100):
        if os.readlink(f'/proc/{client.pid}/ns/net') != os.readlink('/proc/self/ns/net'):
            break
        time.sleep(.01)
    assert os.readlink(f'/proc/{client.pid}/ns/net') != os.readlink('/proc/self/ns/net')
    ns = ['nsenter', '-t', str(client.pid), '-n']
    run('ip', 'link', 'add', 'eth0', 'type', 'veth', 'peer', 'name', 'client0')
    run('ip', 'link', 'set', 'client0', 'netns', str(client.pid))
    run('ip', 'link', 'set', 'eth0', 'up')
    run(*ns, 'ip', 'link', 'set', 'client0', 'up')
    targets = ['10.20.0.127', '10.20.0.128', '192.168.1.127',
               '10.98.0.127', '10.98.0.128', '10.20.0.120']
    sources = ['10.20.0.130', '192.168.1.236', '10.99.0.2', '10.99.0.3',
               '10.99.0.4', '10.30.0.2', '10.40.0.11']
    for ip in targets:
        run('ip', 'addr', 'add', ip + '/32', 'dev', 'eth0')
        run(*ns, 'ip', 'route', 'add', ip + '/32', 'dev', 'client0')
    for ip in sources:
        run(*ns, 'ip', 'addr', 'add', ip + '/32', 'dev', 'client0')
        run('ip', 'route', 'add', ip + '/32', 'dev', 'eth0')
    run('ip', '-6', 'addr', 'add', 'fd02:ca0b:f1cb:20::127/64', 'dev', 'eth0', 'nodad')
    run(*ns, 'ip', '-6', 'addr', 'add', 'fd02:ca0b:f1cb:20::130/64', 'dev', 'client0', 'nodad')
    server = subprocess.Popen([sys.executable, '-c', '''
import selectors,socket
sel=selectors.DefaultSelector()
for port in (22,53,80,443,9999):
 s=socket.socket(socket.AF_INET6);s.setsockopt(socket.IPPROTO_IPV6,socket.IPV6_V6ONLY,0)
 s.bind(('::',port));s.listen();sel.register(s,selectors.EVENT_READ,'tcp')
for port in (53,9999):
 s=socket.socket(socket.AF_INET6,socket.SOCK_DGRAM);s.setsockopt(socket.IPPROTO_IPV6,socket.IPV6_V6ONLY,0)
 s.bind(('::',port));sel.register(s,selectors.EVENT_READ,'udp')
while True:
 for key,_ in sel.select():
  if key.data=='tcp':
   c,_=key.fileobj.accept();c.sendall(b'policy-fixture');c.close()
  else:
   data,peer=key.fileobj.recvfrom(256);key.fileobj.sendto(data,peer)
'''])
    time.sleep(.15)
    rules = str(Path(__file__).with_name('firewall.nft'))
    run('nft', 'add', 'table', 'inet', 'unrelated_fixture')
    untouched = run('nft', 'list', 'table', 'inet', 'unrelated_fixture')
    for _ in range(2):
        run('nft', '-c', '-f', rules)
        run('nft', '-f', rules)
        assert run('nft', 'list', 'table', 'inet', 'unrelated_fixture') == untouched

    def probe(source, target, port, allowed, transport='tcp'):
        code = '''
import socket,sys
source,target,port,transport,expected=sys.argv[1:]
s=socket.socket(socket.AF_INET6 if ':' in source else socket.AF_INET,
                socket.SOCK_DGRAM if transport=='udp' else socket.SOCK_STREAM)
s.settimeout(.25);s.bind((source,0))
try:
 if transport=='udp':s.sendto(b'policy-fixture',(target,int(port)));data,_=s.recvfrom(256)
 else:s.connect((target,int(port)));data=s.recv(256)
 ok=data==b'policy-fixture'
except (TimeoutError,OSError):ok=False
assert ok==(expected=='True'),sys.argv[1:]
'''
        run(*ns, sys.executable, '-c', code, source, target, str(port), transport, str(allowed))

    for source in ['192.168.1.236', '10.20.0.130', '10.99.0.2', '10.99.0.3']:
        for target in targets[:2]:
            for transport in ('tcp', 'udp'):
                probe(source, target, 53, True, transport)
    probe('192.168.1.236', '192.168.1.127', 53, True, 'udp')
    for source in ['10.99.0.4', '10.30.0.2', '10.40.0.11']:
        for transport in ('tcp', 'udp'):
            probe(source, targets[0], 53, False, transport)
    for source in ['10.99.0.2', '10.99.0.3']:
        probe(source, targets[-1], 53, False)
        probe(source, targets[0], 9999, False, 'udp')
        for target in targets[3:5]:
            probe(source, target, 53, False)
            probe(source, target, 53, False, 'udp')
            probe(source, target, 22, source == '10.99.0.2')
    probe('10.20.0.130', targets[3], 22, False)
    for port in (22,80,443):
        probe('10.99.0.2', targets[0], port, True)
        probe('10.99.0.3', targets[0], port, port == 443)
    for transport in ('tcp', 'udp'):
        probe('fd02:ca0b:f1cb:20::130', 'fd02:ca0b:f1cb:20::127', 53, True, transport)
    print('PASS: native atomic rules, OLD client continuity, NEW/ULA DNS, '
          'scoped two-peer TCP/UDP DNS, recovery SSH and denied Wi-Fi/camera sources')
finally:
    if server:
        server.terminate()
        server.wait()
    client.terminate()
    client.wait()
