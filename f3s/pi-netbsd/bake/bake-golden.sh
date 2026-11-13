#!/usr/bin/env bash
# Stage A orchestrator — run on `earth`. Produces netbsd-<piN>-golden.img.gz.
#   Usage: ./bake-golden.sh piN [workdir]
#          ./bake-golden.sh --render piN   # print the rendered setup.sh, nothing else
# Requires: qemu-system-aarch64, qemu-img, edk2 AAVMF, expect, ~/.ssh/id_rsa.pub.
#
# The piN argument renders HOSTNAME, IPADDR, ULA, GW and DNS into
# bake/setup.sh. They come from the steady gonf config in this checkout, so a
# rebuilt Pi boots on the addresses gonf keeps it on (task q74):
#   IPADDR, ULA  netbsd.LANAddress of piN in gonf/cluster/cluster.go
#                (pis_netbsd_network_rc_conf, pis_netbsd_network_npf)
#   GW           labGateway in gonf/netbsd/network.go
#   DNS          Resolver in gonf/labdns/labdns.go (pis_netbsd_resolver_conf)
# A piN without a LANAddress row (pi2/pi3 are Rocky) needs IPADDR and ULA
# in the environment. --render only reads the checkout: no download, no qemu,
# and the password placeholder stays unrendered.
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
GONF="$HERE/../../../gonf"
PUBKEY="${SSHKEY_FILE:-$HOME/.ssh/id_rsa.pub}"
OCTET='(25[0-5]|2[0-4][0-9]|1[0-9]{2}|[1-9]?[0-9])'
IPV4_RE="^($OCTET\\.){3}$OCTET\$"
# fdXX:XXXX:...: a unique-local IPv6 address in the fd00::/8 half of fc00::/7.
ULA_RE='^fd[0-9a-f]{2}(:[0-9a-f]{0,4}){2,7}$'

# gonf_const FILE NAME prints the value of the Go string constant
# `NAME = "value"` in FILE, alone (`const NAME = ...`) or inside a const
# block (empty when absent).
gonf_const() {
    sed -nE "s/^[[:space:]]*(const[[:space:]]+)?$2[[:space:]]*=[[:space:]]*\"([^\"]*)\".*/\2/p" "$1" | head -1
}

# lan_address PIN prints "IPv4 ULA" of the netbsd.LANAddress row inside
# PIN's Host("PIN", ...) call in cluster.go (empty when PIN has none). The
# call ends at the first line holding only ")".
lan_address() {
    awk -v start="Host(\"$1\"," '
        function field(name,   re) {
            re = name ": \"[^\"]*\""
            if (!match($0, re)) return ""
            return substr($0, RSTART + length(name) + 3, RLENGTH - length(name) - 4)
        }
        index($0, start) { inside = 1 }
        inside && /^[[:space:]]*\)[[:space:]]*$/ { exit }
        inside && /netbsd\.LANAddress\{/ { print field("IPv4"), field("ULA"); exit }
    ' "$GONF/cluster/cluster.go"
}

# host_values PIN sets HOSTNAME_FQDN, IPADDR, ULA, GW and DNS, refusing a
# node name other than piN and anything that does not look like an address.
host_values() {
    local inventory
    [[ $1 =~ ^pi[0-9]+$ ]] || { echo "not a Pi node name: '$1'" >&2; exit 1; }
    HOSTNAME_FQDN="$1.lan.buetow.org"
    inventory=$(lan_address "$1")
    if [ -n "$inventory" ]; then
        read -r IPADDR ULA <<<"$inventory"
    else
        IPADDR="${IPADDR:?no netbsd.LANAddress for $1 in cluster.go — set IPADDR and ULA env}"
        ULA="${ULA:?no netbsd.LANAddress for $1 in cluster.go — set IPADDR and ULA env}"
    fi
    GW=$(gonf_const "$GONF/netbsd/network.go" labGateway)
    DNS=$(gonf_const "$GONF/labdns/labdns.go" Resolver)
    local value
    for value in "$IPADDR" "$GW" "$DNS"; do
        [[ $value =~ $IPV4_RE ]] || { echo "not an IPv4 address: '$value'" >&2; exit 1; }
    done
    [[ $ULA =~ $ULA_RE ]] || { echo "not a ULA: '$ULA'" >&2; exit 1; }
}

# render_setup KEY PW writes setup.sh with the host values, the SSH key and
# the password to stdout. It fails when a value line or placeholder of the
# template did not render, so a template change cannot bake a pi0 default.
render_setup() {
    local rendered
    # awk keeps the multiline key intact
    rendered=$(awk -v key="$1" -v pw="$2" -v host="$HOSTNAME_FQDN" -v ip="$IPADDR" \
        -v ula="$ULA" -v gw="$GW" -v dns="$DNS" '
        /^HOSTNAME=/ { print "HOSTNAME=\"" host "\""; next }
        /^IPADDR=/   { print "IPADDR=\"" ip "\""; next }
        /^ULA=/      { print "ULA=\"" ula "\""; next }
        /^GW=/       { print "GW=\"" gw "\""; next }
        /^DNS=/      { print "DNS=\"" dns "\""; next }
        { gsub(/__SSHKEY__/,key); gsub(/__PW__/,pw); print }
    ' "$HERE/setup.sh")
    local line
    for line in "HOSTNAME=\"$HOSTNAME_FQDN\"" "IPADDR=\"$IPADDR\"" "ULA=\"$ULA\"" "GW=\"$GW\"" "DNS=\"$DNS\""; do
        grep -qxF "$line" <<<"$rendered" || { echo "setup.sh did not render $line" >&2; exit 1; }
    done
    ! grep -q __SSHKEY__ <<<"$rendered" || { echo "setup.sh kept __SSHKEY__" >&2; exit 1; }
    printf '%s\n' "$rendered"
}

if [ "${1:-}" = --render ]; then
    PIN="${2:?usage: $0 --render piN}"
    [ -f "$PUBKEY" ] || { echo "missing $PUBKEY" >&2; exit 1; }
    host_values "$PIN"
    render_setup "$(cat "$PUBKEY")" __PW__
    exit 0
fi

PIN="${1:?usage: $0 piN [workdir]}"
WORK="${2:-$HOME/Downloads}"
BASE="$WORK/NetBSD-11.0-evbarm-aarch64-arm64.img.gz"
URL="https://cdn.netbsd.org/pub/NetBSD/NetBSD-11.0/evbarm-aarch64/binary/gzimg/arm64.img.gz"
BASE_SHA512="50c8597cd83b73d12a466e67c1ecc49fb1969941171644b15a7fe377b0f482ccb190b3aa38b043eba3f5e7b4d6e38cf0e7d3c6fd5251813af869b49beda2ee80"
CODE=/usr/share/AAVMF/AAVMF_CODE.fd
VARSRC=/usr/share/AAVMF/AAVMF_VARS.fd

command -v qemu-system-aarch64 >/dev/null || { echo "install qemu-system-aarch64"; exit 1; }
[ -f "$CODE" ] && [ -f "$VARSRC" ] || { echo "install edk2 AAVMF firmware"; exit 1; }
[ -f "$PUBKEY" ] || { echo "missing $PUBKEY"; exit 1; }
mkdir -p "$WORK"

echo "== per-host values from the piN argument and gonf =="
host_values "$PIN"
echo "   $PIN -> $HOSTNAME_FQDN / $IPADDR / $ULA, gateway $GW, DNS $DNS"

echo "== base image =="
[ -f "$BASE" ] || curl -fSL "$URL" -o "$BASE"
printf '%s  %s\n' "$BASE_SHA512" "$BASE" | sha512sum -c -
gzip -t "$BASE"

echo "== fresh work image + varstore =="
IMG="$WORK/netbsd-$PIN-work.img"
VARS="$WORK/AAVMF_VARS_$PIN.fd"
gunzip -kc "$BASE" > "$IMG"
cp "$VARSRC" "$VARS"

echo "== render setup.sh (inject host values + key + generated password) into a served dir =="
SEED="$WORK/seed-$PIN"; mkdir -p "$SEED"
PW=$(openssl rand -base64 9 | tr -d '/+=' | cut -c1-12)
printf '%s\n' "$PW" > "$WORK/$PIN-cred.txt"; chmod 600 "$WORK/$PIN-cred.txt"
render_setup "$(cat "$PUBKEY")" "$PW" > "$SEED/setup.sh"

echo "== serve setup.sh on 127.0.0.1:8000 (guest reaches it at 10.0.2.2) =="
python3 -m http.server 8000 --bind 127.0.0.1 --directory "$SEED" >"$WORK/httpd-$PIN.log" 2>&1 &
HTTPD=$!
trap 'kill "$HTTPD" 2>/dev/null || true' EXIT
until grep -q . "$WORK/httpd-$PIN.log" 2>/dev/null || curl -fsS http://127.0.0.1:8000/setup.sh -o /dev/null 2>/dev/null; do sleep 0.3; done

echo "== boot + configure via qemu/expect (TCG, ~4 min) =="
LOG="$WORK/config-$PIN.log"; : > "$LOG"
NBIMG="$IMG" NBVARS="$VARS" NBLOG="$LOG" NBHTTP="http://10.0.2.2:8000/setup.sh" \
    LC_ALL=C LANG=C expect -f "$HERE/config.exp" || true

echo "== verify markers =="
if ! grep -qa 'SETUP_OK' "$LOG" || ! grep -qa 'halt: halted' "$LOG"; then
    echo "!! bake did not complete cleanly; inspect $LOG"; exit 1
fi
echo "-- on-disk verification block --"
sed -n '/^VBEGIN/,/^VEND/p' <(tr -d '\r' < "$LOG")

echo "== gzip -> golden =="
GOLDEN="$WORK/netbsd-$PIN-golden.img.gz"
gzip -c "$IMG" > "$GOLDEN"
gzip -t "$GOLDEN"
echo
echo "DONE: $GOLDEN"
echo "sha256: $(sha256sum "$GOLDEN" | awk '{print $1}')"
echo "backup password for paul/root: $PW  (also in $WORK/$PIN-cred.txt)"
echo "Next: scp \"$GOLDEN\" paul@$PIN.lan.buetow.org:/home/paul/  (see README Stage B)"
