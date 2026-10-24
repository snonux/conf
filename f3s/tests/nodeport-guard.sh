#!/usr/bin/env bash
set -euo pipefail

# All network mutations and servers live in new unprivileged namespaces.
if [[ ${1:-} != --inside ]]; then
    exec unshare -Urn -- "$0" --inside
fi
[[ $(readlink /proc/self/ns/user) != "$(readlink /proc/1/ns/user)" ]]
[[ $(readlink /proc/self/ns/net) != "$(readlink /proc/1/ns/net)" ]]
repo=$(cd -- "$(dirname -- "$0")/../.." && pwd)
declare -r repo
declare -r rules=$repo/f3s/r-nodes/nodeport-guard/candidate.nft
scratch=$(mktemp -d)
declare -r scratch
declare -a children=()
declare pod='' peer='' external='' backend_a='' backend_b=''
_cleanup() {
    local status=$?
    if (( status != 0 )); then
        for log in "$scratch"/*.log; do
            [[ ! -f $log ]] || cat "$log" >&2
        done
        nft list table ip simulated_kube_proxy >&2 || true
        ip route >&2
    fi
    kill "${children[@]}" 2>/dev/null || true
    rm -rf "$scratch"
}
trap _cleanup EXIT

for role in pod peer external backend_a backend_b; do
    unshare -n -- sleep 600 &
    pid=$!
    children+=("$pid")
    printf -v "$role" '%s' "$pid"
    for ((attempt=0; attempt<100; attempt++)); do
        if [[ $(readlink "/proc/$pid/ns/net") != \
            "$(readlink /proc/self/ns/net)" ]]; then
            break
        fi
        sleep 0.01
    done
    nsenter -t "$pid" -n ip link set lo up
done
ip link set lo up
ip link add cni0 type veth peer name pod0
ip link set pod0 netns "$pod"
ip addr add 10.42.0.1/24 dev cni0
ip link set cni0 up
nsenter -t "$pod" -n ip addr add 10.42.0.99/24 dev pod0
nsenter -t "$pod" -n ip addr add 10.42.1.99/32 dev pod0
nsenter -t "$pod" -n ip addr add 10.42.2.99/32 dev pod0
nsenter -t "$pod" -n ip link set pod0 up
nsenter -t "$pod" -n ip route add default via 10.42.0.1
ip route add 10.42.1.99/32 dev cni0
ip route add 10.42.2.99/32 dev cni0

ip link add lan0 type veth peer name peer0
ip link set peer0 netns "$peer"
ip addr add 192.168.1.120/24 dev lan0
ip link set lan0 up
nsenter -t "$peer" -n ip addr add 192.168.1.121/24 dev peer0
nsenter -t "$peer" -n ip addr add 192.168.1.122/32 dev peer0
nsenter -t "$peer" -n ip link set peer0 up
for address in 192.168.2.121 192.168.2.122; do
    nsenter -t "$peer" -n ip addr add "$address/32" dev lo
    ip route add "$address/32" via 192.168.1.121
done
for subnet in 10.42.1.0/24 10.42.2.0/24; do
    ip route add "$subnet" via 192.168.1.121
done
nsenter -t "$peer" -n ip route add 10.42.0.0/24 via 192.168.1.120
for address in 10.42.1.99 10.42.2.99; do
    nsenter -t "$peer" -n ip route add "$address/32" via 192.168.1.120
done

ip link add wg0 type veth peer name external0
ip link set external0 netns "$external"
ip addr add 192.168.2.120/24 dev wg0
ip link set wg0 up
for address in 192.168.2.110 192.168.2.111 192.168.2.200 192.168.2.201; do
    nsenter -t "$external" -n ip addr add "$address/24" dev external0
done
nsenter -t "$external" -n ip addr add 192.168.1.6/32 dev external0
nsenter -t "$external" -n ip link set external0 up
nsenter -t "$external" -n ip route add default via 192.168.2.120
for address in 192.168.2.121 192.168.2.122; do
    nsenter -t "$external" -n ip route add "$address/32" via 192.168.2.120
done
ip route add 192.168.1.6/32 dev wg0
nsenter -t "$peer" -n ip route add 192.168.1.6/32 via 192.168.1.120
nsenter -t "$peer" -n ip route add 192.168.2.0/24 via 192.168.1.120
sysctl -qw net.ipv4.ip_forward=1
sysctl -qw net.ipv4.conf.all.route_localnet=1
nsenter -t "$peer" -n sysctl -qw net.ipv4.ip_forward=1

for n in 1 2; do
    if (( n == 1 )); then backend=$backend_a; else backend=$backend_b; fi
    nsenter -t "$peer" -n ip link add "back$n" type veth peer name receiver0
    nsenter -t "$peer" -n ip link set receiver0 netns "$backend"
    nsenter -t "$peer" -n ip addr add "10.42.$n.1/24" dev "back$n"
    nsenter -t "$peer" -n ip link set "back$n" up
    nsenter -t "$backend" -n ip addr add "10.42.$n.2/24" dev receiver0
    nsenter -t "$backend" -n ip link set receiver0 up
    nsenter -t "$backend" -n ip route add default via "10.42.$n.1"
    nsenter -t "$backend" -n nft -f - <<'NFT'
table inet simulated_receiver_policy {
    chain input {
        type filter hook input priority filter; policy accept;
        ip saddr { 10.42.1.1, 10.42.2.1 } return
        ip saddr 10.42.0.0/16 tcp dport 5000 counter drop
    }
}
NFT
done

cat > "$scratch/server.py" <<'PY'
import http.server
import json
import sys

class Handler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        body = json.dumps({'peer': self.client_address[0]}).encode()
        self.send_response(200)
        self.send_header('Content-Length', str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *_):
        pass

class Server(http.server.ThreadingHTTPServer):
    request_queue_size = 128
    daemon_threads = True

Server(('0.0.0.0', int(sys.argv[1])), Handler).serve_forever()
PY
for backend in "$backend_a" "$backend_b"; do
    nsenter -t "$backend" -n python3 "$scratch/server.py" 5000 \
        > "$scratch/server-$backend.log" 2>&1 &
    children+=("$!")
done
nsenter -t "$peer" -n python3 "$scratch/server.py" 6443 \
    > "$scratch/other-port.log" 2>&1 &
children+=("$!")
nsenter -t "$external" -n python3 "$scratch/server.py" 30001 \
    > "$scratch/outgoing.log" 2>&1 &
children+=("$!")

_proxy() {
    local destination=$1
    cat > "$scratch/proxy.nft" <<NFT
add table ip simulated_kube_proxy
delete table ip simulated_kube_proxy
table ip simulated_kube_proxy {
    chain prerouting {
        type nat hook prerouting priority dstnat; policy accept;
        fib daddr type local tcp dport {30001,30222,30800,30143,30025} meta mark set 0x4000 dnat to $destination:5000
        ip daddr 10.43.0.9 tcp dport 5000 dnat to $destination:5000
    }
    chain output {
        type nat hook output priority dstnat; policy accept;
        fib daddr type local tcp dport {30001,30222,30800,30143,30025} meta mark set 0x4000 dnat to $destination:5000
    }
    chain postrouting {
        type nat hook postrouting priority srcnat; policy accept;
        meta mark 0x4000 masquerade
        ip saddr 10.42.0.0/16 ip daddr != 10.42.0.0/16 masquerade
    }
}
NFT
    nft -f "$scratch/proxy.nft"
    nsenter -t "$peer" -n nft -f "$scratch/proxy.nft"
}
_proxy 10.42.1.2
# The source node must route this virtual Service toward the destination node.
ip route add 10.43.0.9/32 via 192.168.1.121

cat > "$scratch/probe.py" <<'PY'
import concurrent.futures
import http.client
import json
import sys
import time

phase = sys.argv[1]
ports = [30001, 30222, 30800, 30143, 30025]
nodes = ['192.168.1.120', '192.168.1.121', '192.168.1.122',
         '192.168.2.120', '192.168.2.121', '192.168.2.122']
pods = ['10.42.0.99', '10.42.1.99', '10.42.2.99']
trusted = ['192.168.2.110', '192.168.2.111', '192.168.2.200',
           '192.168.2.201', '192.168.1.6']

def request(case):
    source, destination, port, expected = case
    connection = http.client.HTTPConnection(destination, port, timeout=0.5,
                                            source_address=(source, 0) if source else None)
    peer = None
    try:
        connection.request('GET', '/')
        response = connection.getresponse()
        assert response.status == 200
        peer = json.loads(response.read())['peer']
    except (TimeoutError, ConnectionRefusedError, ConnectionResetError):
        pass
    finally:
        connection.close()
    assert (peer is not None) == expected, (phase, source, destination, port, expected, peer)
    return peer

if phase == 'ready':
    for attempt in range(100):
        try:
            request((None, '127.0.0.1', 30001, True))
            break
        except (AssertionError, OSError):
            time.sleep(0.02)
    else:
        raise RuntimeError('fixture listener never became ready')
elif phase == 'trusted':
    cases = [(s, d, p, True) for s in trusted for d in nodes for p in ports]
elif phase == 'receiver-only':
    cases = [(pods[0], '192.168.1.121', p, True) for p in ports]
elif phase == 'host-local':
    cases = [(None, '127.0.0.1', p, True) for p in ports]
elif phase == 'other-paths':
    cases = [(s, '192.168.1.121', 6443, True) for s in pods]
    cases += [(s, '192.168.2.110', 30001, True) for s in pods]
    cases += [(s, d, 5000, False) for s in pods for d in ['10.43.0.9', sys.argv[2]]]
else:
    expected = phase == 'unfiltered'
    assert phase in ('unfiltered', 'guarded')
    cases = [(s, d, p, expected) for s in pods for d in nodes for p in ports]
    if not expected:
        cases += [(s, d, p, False) for s in pods
                  for d in ['10.42.0.1', '10.42.1.1', '10.42.2.1'] for p in ports]
if phase != 'ready':
    with concurrent.futures.ThreadPoolExecutor(max_workers=6) as executor:
        peers = list(executor.map(request, cases))
    if phase == 'receiver-only':
        assert set(peers) == {'10.42.1.1'}, peers
    print(json.dumps({'phase': phase, 'cases': len(cases), 'passed': True}))
PY
python3 "$scratch/probe.py" ready
nsenter -t "$pod" -n python3 "$scratch/probe.py" unfiltered
nsenter -t "$external" -n python3 "$scratch/probe.py" trusted
nsenter -t "$pod" -n python3 "$scratch/probe.py" other-paths 10.42.1.2
# Installing a destination-only filter is an intentionally failing fix:
# source-node masquerading has already removed the pod's source address.
nsenter -t "$peer" -n nft -c -f "$rules"
nsenter -t "$peer" -n nft -f "$rules"
nsenter -t "$pod" -n python3 "$scratch/probe.py" receiver-only
nft -c -f "$rules"
nft -f "$rules"
nft -c -f "$rules"
nft -f "$rules"
nsenter -t "$pod" -n python3 "$scratch/probe.py" guarded
nsenter -t "$external" -n python3 "$scratch/probe.py" trusted
python3 "$scratch/probe.py" host-local
nsenter -t "$pod" -n python3 "$scratch/probe.py" other-paths 10.42.1.2
for pid in "$$" "$peer"; do
    nsenter -t "$pid" -n nft -j list table inet f3s_nodeport_guard \
        | python3 -c 'import json,sys; x=json.load(sys.stdin); n=sum(e["counter"]["packets"] for v in x["nftables"] for e in v.get("rule",{}).get("expr",[]) if "counter" in e); assert n>0; print("PASS: native guard drop counter increased")'
done
# Endpoint movement changes only simulated kube-proxy, never the guard.
_proxy 10.42.2.2
nsenter -t "$pod" -n python3 "$scratch/probe.py" guarded
nsenter -t "$external" -n python3 "$scratch/probe.py" trusted
python3 "$scratch/probe.py" host-local
nsenter -t "$pod" -n python3 "$scratch/probe.py" other-paths 10.42.2.2
# A broken transaction must not remove the current guard or the proxy table.
cp "$rules" "$scratch/broken.nft"
printf '\nadd rule inet missing_table missing_chain drop\n' >> "$scratch/broken.nft"
if nft -f "$scratch/broken.nft" 2>/dev/null; then
    echo 'FAIL: invalid transaction was accepted' >&2
    exit 1
fi
nsenter -t "$pod" -n python3 "$scratch/probe.py" guarded
nft list table ip simulated_kube_proxy >/dev/null
nft delete table inet f3s_nodeport_guard
nsenter -t "$peer" -n nft delete table inet f3s_nodeport_guard
nsenter -t "$pod" -n python3 "$scratch/probe.py" unfiltered
echo 'PASS: endpoint movement, atomic failure and owned-table removal preserve other rules'
