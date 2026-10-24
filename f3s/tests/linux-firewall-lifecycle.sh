#!/usr/bin/env bash
set -euo pipefail

# Run the ACTUAL activation helper against native nft, with isolated fixed
# filesystem paths and fake systemd/SSH metadata. No host paths are changed.
if [[ ${1:-} != --inside ]]; then
    exec unshare -Urnm -- "$0" --inside
fi
[[ $(readlink /proc/self/ns/user) != "$(readlink /proc/1/ns/user)" ]]
[[ $(readlink /proc/self/ns/net) != "$(readlink /proc/1/ns/net)" ]]
[[ $(readlink /proc/self/ns/mnt) != "$(readlink /proc/1/ns/mnt)" ]]
repo=$(cd -- "$(dirname -- "$0")/../.." && pwd)
scratch=$(mktemp -d)
trap 'rm -rf "$scratch"' EXIT
mkdir "$scratch/bin" "$scratch/systemd"
export F3S_TEST_SYSTEMD=$scratch/systemd
native_nft=$(command -v nft)
declare -r native_nft
export F3S_TEST_NFT=$native_nft
export PATH=$scratch/bin:$PATH
cp /etc/protocols "$scratch/protocols"
cp /etc/services "$scratch/services"

# Mount propagation must be private before mounting over system directories.
mount --make-rprivate /
for directory in /etc /var/lib /run /usr/local/sbin; do
    mount -t tmpfs tmpfs "$directory"
done
cp "$scratch/protocols" /etc/protocols
cp "$scratch/services" /etc/services
mkdir -p /etc/f3s-firewall
install -m 755 "$repo/gonf/hostfirewall/assets/f3s-firewall" \
    /usr/local/sbin/f3s-firewall
declare -r helper=/usr/local/sbin/f3s-firewall
declare -r pending=/var/lib/f3s-firewall/pending
declare -r first='192.168.1.156 10001 192.168.1.120 22'
declare -r second='192.168.1.156 10002 192.168.1.120 22'

cat > "$scratch/bin/systemctl" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
case "$1" in
    is-enabled) test -f "$F3S_TEST_SYSTEMD/enabled" ;;
    is-active) test -f "$F3S_TEST_SYSTEMD/timer" ;;
    enable) touch "$F3S_TEST_SYSTEMD/enabled" ;;
    disable) rm -f "$F3S_TEST_SYSTEMD/enabled" ;;
    stop) rm -f "$F3S_TEST_SYSTEMD/timer" ;;
    reset-failed) : ;;
    *) exit 99 ;;
esac
SH
cat > "$scratch/bin/systemd-run" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
if [[ -f $F3S_TEST_SYSTEMD/fail-timer ]]; then
    exit 42
fi
[[ "$*" == *'--on-active=5m'* ]]
[[ "$*" == *'/usr/local/sbin/f3s-firewall rollback'* ]]
touch "$F3S_TEST_SYSTEMD/timer"
SH
cat > "$scratch/bin/ss" <<'SH'
#!/usr/bin/env bash
printf 'session one\nsession two\n'
SH
cat > "$scratch/bin/restorecon" <<'SH'
#!/usr/bin/env bash
exit 0
SH
cat > "$scratch/bin/nft" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
if [[ "$*" == '-f /var/lib/f3s-firewall/pending/candidate.nft' ]]; then
    test -f "$F3S_TEST_SYSTEMD/timer"
    if [[ -f $F3S_TEST_SYSTEMD/fail-load ]]; then
        rm "$F3S_TEST_SYSTEMD/fail-load"
        exit 42
    fi
fi
exec "$F3S_TEST_NFT" "$@"
SH
chmod +x "$scratch/bin/"*

python3 - "$repo/gonf/hostfirewall/assets/candidate.nft.tmpl" <<'PY'
from pathlib import Path
import sys
rules = Path(sys.argv[1]).read_text().replace('{{.Interface}}', 'enp0s5')
rules = rules.replace('{{if .WireGuardPort}}', '').replace('{{end}}', '')
rules = rules.replace('{{.WireGuardPort}}', '56709')
Path('/etc/f3s-firewall/candidate.nft').write_text(rules)
PY
nft -f - <<'NFT'
table inet unrelated {
    chain sentinel { counter; }
}
table inet f3s_guard {
    chain previous { counter; }
}
NFT
nft list table inet f3s_guard > "$scratch/prior.nft"
nft list table inet unrelated > "$scratch/unrelated.nft"
# co3 is already deployed. Exercise its actual candidate in the same kernel
# namespace, and require every source-firewall operation to preserve it.
nft -f "$repo/f3s/r-nodes/nodeport-guard/candidate.nft"
nft list table inet f3s_nodeport_guard > "$scratch/nodeport-before.nft"
_helper() {
    local helper_status=0
    "$helper" "$@" || helper_status=$?
    if ! nft list table inet f3s_nodeport_guard > "$scratch/nodeport-after.nft" ||
        ! cmp "$scratch/nodeport-before.nft" "$scratch/nodeport-after.nft"; then
        echo 'FAIL: source-firewall operation changed the NodePort guard' >&2
        exit 1
    fi
    return "$helper_status"
}
_previous_restored() {
    nft list table inet f3s_guard > "$scratch/current.nft"
    cmp "$scratch/prior.nft" "$scratch/current.nft"
    [[ ! -e $pending && ! -e $F3S_TEST_SYSTEMD/timer ]]
    [[ ! -e /run/f3s-firewall-applied.sha256 ]]
}

touch "$F3S_TEST_SYSTEMD/fail-load"
if _helper apply "$first"; then
    echo 'FAIL: injected load failure succeeded' >&2
    exit 1
fi
[[ ! -e $F3S_TEST_SYSTEMD/fail-load ]]
_previous_restored
[[ ! -e /etc/f3s-firewall/confirmed.nft ]]
echo 'PASS: failed load rolls back prior table and leaves no success marker'

_helper apply "$first"
[[ -f $F3S_TEST_SYSTEMD/timer && -f $pending/table.nft ]]
# Dispatch the command the independent timer was told to execute.
_helper rollback
_previous_restored
echo 'PASS: timer rollback restores the prior table'

_helper apply "$first"
if _helper confirm "$first"; then
    echo 'FAIL: confirmation from the apply session succeeded' >&2
    exit 1
fi
[[ -f $F3S_TEST_SYSTEMD/timer && -d $pending ]]
[[ ! -e /etc/f3s-firewall/confirmed.nft ]]
_helper confirm "$second"
[[ -f $F3S_TEST_SYSTEMD/enabled && ! -e $pending ]]
[[ ! -e $F3S_TEST_SYSTEMD/timer ]]
cmp /etc/f3s-firewall/candidate.nft /etc/f3s-firewall/confirmed.nft
nft list table inet f3s_guard > "$scratch/confirmed-table.nft"
cp /etc/f3s-firewall/confirmed.nft "$scratch/confirmed.nft"
echo 'PASS: same-session confirmation refused; new-session confirmation persists'

sed -i 's/counter drop/counter drop comment "next-policy"/' \
    /etc/f3s-firewall/candidate.nft
_helper apply "$first"
# Simulate a reboot's fresh ruleset while its unconfirmed disk state remains.
nft delete table inet f3s_guard
_helper boot
nft list table inet f3s_guard > "$scratch/current.nft"
cmp "$scratch/confirmed-table.nft" "$scratch/current.nft"
cmp "$scratch/confirmed.nft" /etc/f3s-firewall/confirmed.nft
rm "$F3S_TEST_SYSTEMD/timer"
_helper rollback
[[ -f $F3S_TEST_SYSTEMD/enabled ]]
echo 'PASS: boot during pending apply loads only the previous confirmed policy'

touch "$F3S_TEST_SYSTEMD/fail-timer"
if _helper apply "$first"; then
    echo 'FAIL: activation without an armed timer succeeded' >&2
    exit 1
fi
[[ ! -e $pending ]]
rm "$F3S_TEST_SYSTEMD/fail-timer"
nft list table inet f3s_guard > "$scratch/current.nft"
cmp "$scratch/confirmed-table.nft" "$scratch/current.nft"
nft list table inet unrelated > "$scratch/current.nft"
cmp "$scratch/unrelated.nft" "$scratch/current.nft"
echo 'PASS: timer-arm failure makes no change; unrelated table remains intact'

_helper revert "$second"
[[ ! -f $F3S_TEST_SYSTEMD/enabled ]]
[[ ! -f /etc/f3s-firewall/confirmed.nft ]]
nft list table inet f3s_guard > "$scratch/current.nft"
cmp "$scratch/prior.nft" "$scratch/current.nft"
echo 'PASS: explicit post-confirm revert restores prior rules and enablement'
echo 'PASS: deployed NodePort guard survives every source-firewall operation'
