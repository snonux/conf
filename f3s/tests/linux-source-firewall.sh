#!/usr/bin/env bash
set -euo pipefail

# Destructive networking commands below run only inside a new user/network
# namespace. Never run this with a real host's network namespace.
if [[ ${1:-} != --inside ]]; then
    exec unshare -Urn -- "$0" --inside
fi
[[ $(readlink /proc/self/ns/user) != "$(readlink /proc/1/ns/user)" ]]
[[ $(readlink /proc/self/ns/net) != "$(readlink /proc/1/ns/net)" ]]
repo=$(cd -- "$(dirname -- "$0")/../.." && pwd)
declare -r repo
declare -r assets=$repo/gonf/hostfirewall/assets
declare -a children=()
scratch=$(mktemp -d)
declare -r scratch
_cleanup() {
    kill "${children[@]}" 2>/dev/null || true
    rm -rf "$scratch"
}
trap _cleanup EXIT

unshare -n -- sleep 120 &
client=$!
children+=("$client")
unshare -n -- sleep 120 &
pod=$!
children+=("$pod")
for namespace_pid in "$client" "$pod"; do
    for ((attempt=0; attempt<100; attempt++)); do
        if [[ $(readlink "/proc/$namespace_pid/ns/net") != \
            "$(readlink /proc/self/ns/net)" ]]; then
            break
        fi
        sleep 0.01
    done
    nsenter -t "$namespace_pid" -n ip link set lo up
done
ip link set lo up
ip link add enp0s5 type veth peer name client0
ip link set client0 netns "$client"
ip addr add 192.168.1.120/24 dev enp0s5
ip -6 addr add fd22:c702:acb7::120/64 dev enp0s5 nodad
ip -6 addr add 2001:db8:1::120/64 dev enp0s5 nodad
ip link set enp0s5 up
ip route add 203.0.113.0/24 dev enp0s5
nsenter -t "$client" -n ip addr add 192.168.1.156/24 dev client0
nsenter -t "$client" -n ip addr add 203.0.113.2/24 dev client0
nsenter -t "$client" -n ip -6 addr add fd22:c702:acb7::2/64 dev client0 nodad
nsenter -t "$client" -n ip -6 addr add 2001:db8:1::2/64 dev client0 nodad
nsenter -t "$client" -n ip link set client0 up

ip link add cni0 type veth peer name pod0
ip link set pod0 netns "$pod"
ip addr add 10.42.0.1/24 dev cni0
ip link set cni0 up
nsenter -t "$pod" -n ip addr add 10.42.0.2/24 dev pod0
nsenter -t "$pod" -n ip link set pod0 up
nsenter -t "$pod" -n ip route add default via 10.42.0.1
sysctl -qw net.ipv4.ip_forward=1

python3 -m http.server 6443 --bind :: --directory "$scratch" \
    > "$scratch/host.log" 2>&1 &
children+=("$!")
nsenter -t "$pod" -n python3 -m http.server 9090 --bind 0.0.0.0 \
    --directory "$scratch" > "$scratch/pod.log" 2>&1 &
children+=("$!")
nsenter -t "$client" -n python3 -m http.server 8080 --bind 0.0.0.0 \
    --directory "$scratch" > "$scratch/client.log" 2>&1 &
children+=("$!")
cat > "$scratch/udp.py" <<'PY'
import selectors
import socket
selector = selectors.DefaultSelector()
for port in (53, 56709):
    sock = socket.socket(socket.AF_INET6, socket.SOCK_DGRAM)
    sock.setsockopt(socket.IPPROTO_IPV6, socket.IPV6_V6ONLY, 0)
    sock.bind(('::', port))
    selector.register(sock, selectors.EVENT_READ)
while True:
    for key, _ in selector.select():
        data, peer = key.fileobj.recvfrom(1024)
        key.fileobj.sendto(data, peer)
PY
python3 "$scratch/udp.py" > "$scratch/udp.log" 2>&1 &
children+=("$!")
nft -f - <<'NFT'
table ip simulated_kube_proxy {
    chain prerouting {
        type nat hook prerouting priority dstnat; policy accept;
        tcp dport 30090 dnat to 10.42.0.2:9090
    }
}
NFT

_udp_probe() {
    nsenter -t "$client" -n python3 - "$@" <<'PY'
import socket
import sys
expected, source, target, port = sys.argv[1:]
sock = socket.socket(socket.AF_INET6 if ':' in source else socket.AF_INET,
                     socket.SOCK_DGRAM)
sock.settimeout(1)
sock.bind((source, 0))
sock.sendto(b'f3s-check', (target, int(port)))
try:
    outcome = 'allow' if sock.recv(64) == b'f3s-check' else 'invalid'
except TimeoutError:
    outcome = 'drop'
assert outcome == expected, (source, target, port, expected, outcome)
print(f'PASS: UDP {source} -> {target}:{port} = {outcome}')
PY
}

_probe() {
    local expected=$1 source=$2 target=$3
    local outcome=drop
    if nsenter -t "$client" -n curl -g -fs --noproxy '*' \
        --interface "$source" --connect-timeout 1 --max-time 2 \
        "$target" >/dev/null 2>&1; then
        outcome=allow
    fi
    [[ $outcome == "$expected" ]] || {
        printf 'FAIL: %s -> %s expected %s, got %s\n' \
            "$source" "$target" "$expected" "$outcome" >&2
        exit 1
    }
    printf 'PASS: %s -> %s = %s\n' "$source" "$target" "$outcome"
}

# Wait for startup, then establish the unfiltered control path.
for ((attempt=0; attempt<100; attempt++)); do
    if curl -fs --noproxy '*' http://127.0.0.1:6443 >/dev/null 2>&1; then
        break
    fi
    sleep 0.02
done
_probe allow 203.0.113.2 http://192.168.1.120:30090
_probe allow 2001:db8:1::2 'http://[2001:db8:1::120]:6443'

# Render just the two simple Go template substitutions for the real asset.
python3 - "$assets/candidate.nft.tmpl" "$scratch/rules.nft" <<'PY'
import pathlib
import sys
rules = pathlib.Path(sys.argv[1]).read_text()
rules = rules.replace('{{.Interface}}', 'enp0s5')
rules = rules.replace('{{if .WireGuardPort}}', '').replace('{{end}}', '')
rules = rules.replace('{{.WireGuardPort}}', '56709')
pathlib.Path(sys.argv[2]).write_text(rules)
PY
nft -c -f "$scratch/rules.nft"
nft -f "$scratch/rules.nft"
# A repeat replacement is atomic and leaves the unrelated simulated table.
nft -c -f "$scratch/rules.nft"
nft -f "$scratch/rules.nft"
nft list table ip simulated_kube_proxy >/dev/null
cp "$scratch/rules.nft" "$scratch/broken.nft"
echo 'add rule inet missing_table missing_chain drop' >> "$scratch/broken.nft"
if nft -f "$scratch/broken.nft" 2>/dev/null; then
    echo 'FAIL: deliberately invalid transaction succeeded' >&2
    exit 1
fi
_probe allow 192.168.1.156 http://192.168.1.120:6443
_probe allow fd22:c702:acb7::2 'http://[fd22:c702:acb7::120]:6443'
_probe drop 203.0.113.2 http://192.168.1.120:6443
_probe drop 2001:db8:1::2 'http://[2001:db8:1::120]:6443'
_probe allow 192.168.1.156 http://192.168.1.120:30090
_probe drop 203.0.113.2 http://192.168.1.120:30090
_udp_probe allow 192.168.1.156 192.168.1.120 53
_udp_probe allow fd22:c702:acb7::2 fd22:c702:acb7::120 53
_udp_probe drop 203.0.113.2 192.168.1.120 53
_udp_probe drop 2001:db8:1::2 2001:db8:1::120 53
_udp_probe allow 203.0.113.2 192.168.1.120 56709
curl -fs --noproxy '*' --interface 192.168.1.120 --max-time 2 \
    http://203.0.113.2:8080 >/dev/null
echo 'PASS: host-initiated traffic receives replies through the source guard'
nsenter -t "$pod" -n curl -fs --noproxy '*' --max-time 2 \
    http://10.42.0.1:6443 >/dev/null
echo 'PASS: pod-to-host path and unrelated kube-proxy table preserved'
nft list table inet f3s_guard > "$scratch/snapshot.nft"

# The Pi profile has no WireGuard exception and uses eth0.
ip link set enp0s5 name eth0
python3 - "$assets/candidate.nft.tmpl" "$scratch/pi.nft" <<'PY'
import pathlib
import re
import sys
rules = pathlib.Path(sys.argv[1]).read_text().replace('{{.Interface}}', 'eth0')
rules = re.sub(r'{{if .WireGuardPort}}.*?{{end}}', '', rules, flags=re.S)
pathlib.Path(sys.argv[2]).write_text(rules)
PY
nft -c -f "$scratch/pi.nft"
nft -f "$scratch/pi.nft"
_udp_probe allow 192.168.1.156 192.168.1.120 53
_udp_probe allow fd22:c702:acb7::2 fd22:c702:acb7::120 53
_udp_probe drop 2001:db8:1::2 2001:db8:1::120 53
_udp_probe drop 203.0.113.2 192.168.1.120 56709

# Match the helper's own-table rollback batch and restore the prior profile.
ip link set eth0 name enp0s5
{
    printf 'add table inet f3s_guard\ndelete table inet f3s_guard\n'
    cat "$scratch/snapshot.nft"
} > "$scratch/rollback.nft"
nft -c -f "$scratch/rollback.nft"
nft -f "$scratch/rollback.nft"
_udp_probe allow 203.0.113.2 192.168.1.120 56709
nft list table ip simulated_kube_proxy >/dev/null
echo 'PASS: rollback restores only the prior owned table'
