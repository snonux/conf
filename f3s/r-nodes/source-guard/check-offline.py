#!/usr/bin/env python3
"""Exercise the actual guard nft files only in disposable network namespaces."""
import ipaddress
import os
import json
from pathlib import Path
import socket
import subprocess
import sys
import time

import yaml


def run(*args):
    result = subprocess.run(args, capture_output=True, text=True)
    if result.returncode:
        raise RuntimeError(f'{args}: {result.stdout}{result.stderr}')
    return result


# u74: each Cluster NodePort and the receiver policy/endpoint port behind it.
# The wg0 expectations below come from these policies, not from the guard, so
# the guard must admit exactly the wg0 (192.168.2.0/24) ipBlocks they admit.
NODEPORT_POLICIES = {30001: ('infra', 'registry-nodeport', 5000),
                     30222: ('services', 'forgejo', 2222),
                     30800: ('services', 'anki-sync-server', 8080),
                     30143: ('services', 'protonbridge-nodeport', 143),
                     30025: ('services', 'protonbridge-nodeport', 25)}


def policy_wg0_sources(root):
    """Map each Cluster NodePort to the wg0 /32 sources its policy admits."""
    mesh = ipaddress.ip_network('192.168.2.0/24')
    allowed = {}
    for nodeport, (namespace, name, port) in NODEPORT_POLICIES.items():
        path = root / 'network-policies/namespaces' / namespace / 'policies.yaml'
        [policy] = [d for d in yaml.safe_load_all(path.read_text())
                    if d and d['metadata']['name'] == name]
        sources = set()
        for rule in policy['spec']['ingress']:
            if rule.get('ports') and port not in [p['port'] for p in rule['ports']]:
                continue
            for peer in rule.get('from', []):
                if 'ipBlock' not in peer:
                    continue  # pod/namespace selectors never match a wg0 host
                cidr = ipaddress.ip_network(peer['ipBlock']['cidr'])
                # No receiver admits IPv6 or a wider mesh block; the guard
                # drops wg0 IPv6 on these ports, so that must stay true.
                assert cidr.version == 4, (name, cidr)
                assert not cidr.overlaps(mesh) or cidr.prefixlen == 32, (name, cidr)
                if cidr.subnet_of(mesh):
                    sources.add(str(cidr.network_address))
        allowed[nodeport] = sources
    # Guard against a vacuous model: relayd ports admit the frontends,
    # Bridge does not; every receiver admits the r-node mesh addresses.
    assert '192.168.2.110' in allowed[30001] and '192.168.2.110' not in allowed[30143], allowed
    assert all('192.168.2.120' in sources for sources in allowed.values()), allowed
    return allowed


def wg0_namespace(sources):
    """Attach a second client namespace as wg0 peer, holding the given sources.

    A separate namespace keeps the mesh /32s (incl. 192.168.2.121, a routed
    target of the pod guard trace in the first client namespace) apart from
    the enp0s5 fixture. The node's own wg0 address here is r2's .122.
    """
    peer = subprocess.Popen(['unshare', '-n', 'sleep', '180'])
    for _ in range(100):
        if os.readlink(f'/proc/{peer.pid}/ns/net') != os.readlink('/proc/self/ns/net'):
            break
        time.sleep(.01)
    via = ['nsenter', '-t', str(peer.pid), '-n']
    run('ip', 'link', 'add', 'wg0', 'type', 'veth', 'peer', 'name', 'client1')
    run('ip', 'link', 'set', 'client1', 'netns', str(peer.pid))
    run('ip', 'link', 'set', 'wg0', 'up')
    run(*via, 'ip', 'link', 'set', 'client1', 'up')
    run('ip', 'addr', 'add', '192.168.2.122/32', 'dev', 'wg0')
    run(*via, 'ip', 'route', 'add', '192.168.2.122/32', 'dev', 'client1')
    for address in sources:
        run(*via, 'ip', 'addr', 'add', address + '/32', 'dev', 'client1')
        run('ip', 'route', 'add', address + '/32', 'dev', 'wg0')
    run('ip', '-6', 'addr', 'add', 'fd42:beef:cafe:2::122/64', 'dev', 'wg0', 'nodad')
    for address in ['fd42:beef:cafe:2::130', 'fd42:beef:cafe:2::110']:
        run(*via, 'ip', '-6', 'addr', 'add', address + '/64', 'dev', 'client1', 'nodad')
    time.sleep(.15)  # let the veth pair come up, as for the enp0s5 fixture
    return peer, via


def check_wg0(probe, drops, allowed, cluster):
    """u74: the Cluster NodePorts over wg0 admit exactly the policy sources.

    f0 (.130), pi0 (.203) and rocky routed through a gateway (.123) stand for
    mesh peers no receiver admits; the frontends .110/.111 reach only the
    relayd ports. Other wg0 ports stay uninspected for every peer.
    """
    # The node's own .122 reaches itself over lo, never wg0: not a source.
    mesh = sorted((set().union(*allowed.values()) - {'192.168.2.122'}) |
                  {'192.168.2.130', '192.168.2.203', '192.168.2.123'})
    peer, via = wg0_namespace(mesh)
    try:
        for port in cluster:
            for source in mesh:
                probe(source, '192.168.2.122', port, source in allowed[port], via)
            for source in ['fd42:beef:cafe:2::130', 'fd42:beef:cafe:2::110']:
                probe(source, 'fd42:beef:cafe:2::122', port, False, via)
        for source in ['192.168.2.130', '192.168.2.203', '192.168.2.110']:
            for port in [22, 6443, 30090, 30091, 32200, 9999]:
                probe(source, '192.168.2.122', port, True, via)
        probe('fd42:beef:cafe:2::130', 'fd42:beef:cafe:2::122', 9999, True, via)
        before = drops('wg0_cluster_nodeport')
        probe('192.168.2.130', '192.168.2.122', 30143, False, via)
        assert drops('wg0_cluster_nodeport') > before, 'wg0_cluster_nodeport drop counter did not count'
    finally:
        peer.terminate()
        peer.wait()


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
    # 192.168.1.120 stays as a fixture destination to prove it is no longer
    # trusted (task 074); the live hosts no longer carry it.
    destinations = ['10.20.0.120', '192.168.1.120', '10.98.0.120']
    # o74: 10.20.0.122 stands for a peer r-node, 10.20.0.123 for rocky and
    # 10.20.0.124 for an unlisted lab host (10.20.0.121 stays a routed
    # remote-alias target for the pod guard trace below).
    sources = ['10.20.0.130', '192.168.1.130', '192.168.1.138', '192.168.1.236',
               '192.168.1.50', '10.99.0.2', '10.99.0.3', '10.99.0.4',
               '10.30.0.2', '10.40.0.11', '10.42.0.2',
               '10.20.0.122', '10.20.0.123', '10.20.0.124']
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
for port in (22, 80, 443, 6443, 30001, 30222, 30800, 30143, 30025, 30090, 30091, 32200, 9999):
    s = socket.socket(socket.AF_INET6)
    s.setsockopt(socket.IPPROTO_IPV6, socket.IPV6_V6ONLY, 0)
    s.bind(('::', port)); s.listen(); sel.register(s, selectors.EVENT_READ)
while True:
    for key, _ in sel.select():
        conn, _ = key.fileobj.accept(); conn.close()
'''])
    time.sleep(.15)
    host = root / 'r-nodes/source-guard/f3s-guard.nft'
    pod = root / 'r-nodes/nodeport-guard/f3s-nodeport-guard.nft'
    run('nft', 'add', 'table', 'inet', 'unrelated_fixture')
    for path in [host, pod, host, pod]:
        run('nft', '-c', '-f', str(path))
        run('nft', '-f', str(path))

    run('nft', 'list', 'table', 'inet', 'unrelated_fixture')
    saved_pod = run('nft', 'list', 'table', 'inet', 'f3s_nodeport_guard').stdout
    run('nft', '-f', str(host))
    assert saved_pod == run('nft', 'list', 'table', 'inet', 'f3s_nodeport_guard').stdout

    def probe(source, target, port, allowed, via=ns):
        code = '''import socket,sys
s=socket.socket(socket.AF_INET6 if ':' in sys.argv[1] else socket.AF_INET)
s.settimeout(.25); s.bind((sys.argv[1],0))
try: s.connect((sys.argv[2],int(sys.argv[3]))); ok=True
except (TimeoutError,OSError): ok=False
assert ok == (sys.argv[4]=='True'), sys.argv[1:]
'''
        run(*via, sys.executable, '-c', code, source, target, str(port), str(allowed))

    def drops(chain):
        listing = run('nft', '-j', 'list', 'chain', 'inet', 'f3s_guard', chain).stdout
        return [e['rule']['expr'][-2]['counter']['packets'] for e in json.loads(listing)['nftables']
                if 'rule' in e and 'counter' in e['rule']['expr'][-2]][0]

    probe('10.20.0.130', destinations[0], 9999, True)
    probe('10.20.0.130', destinations[2], 22, False)
    # Task 074: the retired old /24 (former infra peers, the controller's
    # old .236 and the old local destination) receives no trust any more.
    for source in ['192.168.1.130', '192.168.1.138', '192.168.1.236']:
        for port in [22, 9999]:
            probe(source, destinations[0], port, False)
    probe('10.20.0.130', destinations[1], 9999, False)
    for source in ['192.168.1.50', '10.99.0.4', '10.30.0.2', '10.40.0.11']:
        probe(source, destinations[0], 443, False)
    # n44: the laptop also reaches the Syncthing NodePort 32200, on lab
    # addresses only (not old LAN or recovery aliases); the phone does not.
    for port in [22, 80, 443, 6443, 30001, 32200]:
        probe('10.99.0.2', destinations[0], port, port in [22, 80, 443, 32200])
        probe('10.99.0.3', destinations[0], port, port == 443)
        # j24: the retired recovery alias admits nothing, not even laptop SSH.
        probe('10.99.0.2', destinations[2], port, False)
        probe('10.99.0.3', destinations[2], port, False)
    probe('10.99.0.2', destinations[1], 32200, False)
    for source in ['10.99.0.4', '192.168.1.50', '10.30.0.2']:
        probe(source, destinations[0], 32200, False)
    probe('fd02:ca0b:f1cb:20::130', 'fd02:ca0b:f1cb:20::120', 9999, True)
    probe('fd22:c702:acb7::130', 'fd02:ca0b:f1cb:20::120', 9999, True)
    # o74: the Cluster NodePorts admit only r-nodes, plus rocky to the
    # registry; other lab hosts (f0 .130, .124), wg2 and the trusted IPv6
    # ranges are dropped there but keep every other lab port, including
    # the Local NodePorts 30090/30091 that their own policies filter.
    cluster = [30001, 30222, 30800, 30143, 30025]
    for port in cluster:
        probe('10.20.0.122', destinations[0], port, True)
        probe('10.20.0.123', destinations[0], port, port == 30001)
        for source in ['10.20.0.130', '10.20.0.124', '10.99.0.2', '10.99.0.3']:
            probe(source, destinations[0], port, False)
        for source in ['fd02:ca0b:f1cb:20::130', 'fd22:c702:acb7::130']:
            probe(source, 'fd02:ca0b:f1cb:20::120', port, False)
        # Retired destinations stay closed even to an r-node source.
        probe('10.20.0.122', destinations[1], port, False)
        probe('10.20.0.122', destinations[2], port, False)
    for source in ['10.20.0.130', '10.20.0.123', '10.20.0.124', '10.20.0.122']:
        for port in [22, 443, 6443, 30090, 30091, 9999]:
            probe(source, destinations[0], port, True)
    before = drops('cluster_nodeport')
    probe('10.20.0.130', destinations[0], 30001, False)
    assert drops('cluster_nodeport') > before, 'cluster_nodeport drop counter did not count'
    check_wg0(probe, drops, policy_wg0_sources(root), cluster)
    # Verify the pod guard independently, with the host table removed.
    run('nft', 'delete', 'table', 'inet', 'f3s_guard')
    for target in destinations:
        probe('10.42.0.2', target, 30001, False)
        probe('10.42.0.2', target, 9999, True)
    # Trace the independent guard counters for remote aliases before routing.
    mac = json.loads(run('ip', '-j', 'link', 'show', 'enp0s5').stdout)[0]['address']
    # j24: a remote node's retired recovery alias is no longer a node
    # address, so the node set no longer counts it (no node holds it).
    for target, guarded in [('10.20.0.121', True), ('10.98.0.122', False), ('192.168.2.121', True)]:
        run(*ns, 'ip', 'neigh', 'replace', target, 'lladdr', mac, 'dev', 'client0')
        before = run('nft', 'list', 'chain', 'inet', 'f3s_nodeport_guard', 'from_pod').stdout
        probe('10.42.0.2', target, 30001, False)
        after = run('nft', 'list', 'chain', 'inet', 'f3s_nodeport_guard', 'from_pod').stdout
        assert (before != after) == guarded, target
    print('PASS: native syntax, atomic replacement, retired old /24 and recovery aliases denied, wg2 scope (incl. n44 32200), ULA, o74 Cluster NodePorts node/rocky-only, u74 wg0 Cluster NodePorts per receiver policy, independent local/remote pod guard')
finally:
    if server:
        server.terminate()
        server.wait()
    client.terminate()
    client.wait()
