#!/bin/bash
# Stage A orchestrator — run on `earth`. Produces netbsd-<piN>-golden.img.gz.
#   Usage: ./bake-golden.sh piN [workdir]
# Requires: qemu-system-aarch64, qemu-img, edk2 AAVMF, expect, ~/.ssh/id_rsa.pub.
# EDIT bake/setup.sh (HOSTNAME/IPADDR) for the target Pi BEFORE running this.
set -euo pipefail

PIN="${1:?usage: $0 piN [workdir]}"
HERE="$(cd "$(dirname "$0")" && pwd)"
WORK="${2:-$HOME/Downloads}"
BASE="$WORK/NetBSD-10.1-evbarm-aarch64-arm64.img.gz"
URL="https://cdn.netbsd.org/pub/NetBSD/NetBSD-10.1/evbarm-aarch64/binary/gzimg/arm64.img.gz"
CODE=/usr/share/AAVMF/AAVMF_CODE.fd
VARSRC=/usr/share/AAVMF/AAVMF_VARS.fd
PUBKEY="${SSHKEY_FILE:-$HOME/.ssh/id_rsa.pub}"

command -v qemu-system-aarch64 >/dev/null || { echo "install qemu-system-aarch64"; exit 1; }
[ -f "$CODE" ] && [ -f "$VARSRC" ] || { echo "install edk2 AAVMF firmware"; exit 1; }
[ -f "$PUBKEY" ] || { echo "missing $PUBKEY"; exit 1; }
mkdir -p "$WORK"

echo "== base image =="
[ -f "$BASE" ] || curl -fSL "$URL" -o "$BASE"
gzip -t "$BASE"

echo "== fresh work image + varstore =="
IMG="$WORK/netbsd-$PIN-work.img"
VARS="$WORK/AAVMF_VARS_$PIN.fd"
gunzip -kc "$BASE" > "$IMG"
cp "$VARSRC" "$VARS"

echo "== per-host values from the piN argument =="
case "$PIN" in
    pi0) IPADDR=192.168.1.125 ;;
    pi1) IPADDR=192.168.1.126 ;;
    pi2) IPADDR=192.168.1.127 ;;
    pi3) IPADDR=192.168.1.128 ;;
    *)   IPADDR="${IPADDR:?unknown $PIN — set IPADDR env}" ;;
esac
HOSTNAME_FQDN="$PIN.lan.buetow.org"
echo "   $PIN -> $HOSTNAME_FQDN / $IPADDR"

echo "== render setup.sh (inject host/ip + key + generated password) into a served dir =="
SEED="$WORK/seed-$PIN"; mkdir -p "$SEED"
PW=$(openssl rand -base64 9 | tr -d '/+=' | cut -c1-12)
printf '%s\n' "$PW" > "$WORK/$PIN-cred.txt"; chmod 600 "$WORK/$PIN-cred.txt"
KEY=$(cat "$PUBKEY")
# substitute host/ip lines + placeholders; awk keeps the multiline key intact
awk -v key="$KEY" -v pw="$PW" -v host="$HOSTNAME_FQDN" -v ip="$IPADDR" '
    /^HOSTNAME=/ { print "HOSTNAME=\"" host "\""; next }
    /^IPADDR=/   { print "IPADDR=\"" ip "\""; next }
    { gsub(/__SSHKEY__/,key); gsub(/__PW__/,pw); print }
' "$HERE/setup.sh" > "$SEED/setup.sh"

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
echo "Next: scp \"$GOLDEN\" paul@$PIN.lan.buetow.org:/home/paul/netbsd-pi0-golden.img.gz  (see README Stage B)"
