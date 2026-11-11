#!/bin/sh
# NetBSD base tools only; boot recovery runs before NPF and NETWORKING.
set -eu
PATH=/sbin:/bin:/usr/sbin:/usr/bin:/usr/pkg/sbin:/usr/pkg/bin:/usr/local/sbin
export PATH
umask 077
state=/var/db/gonf-network-migration
helper=/usr/local/sbin/gonf-network-migration
candidate=/usr/local/etc/gonf-network-migration.npf
# Base network settings (ifconfig_mue0, defaultroute); only retire edits them.
rcconf=/etc/rc.conf
[ "$(id -u)" = 0 ] && [ "$(uname -s)" = NetBSD ] || exit 1
# shellcheck source=/dev/null
. /usr/local/etc/gonf-network-migration.conf
: "${host:?}" "${iface:?}" "${old:?}" "${lab:?}" "${recovery:?}" "${ula:?}" "${shared_gateway:?}"
[ "$(hostname -s)" = "$host" ] || exit 1
mkdir -p "$state"
if [ "${1:-}" != locked ]; then
    # Use the verified native command form rather than shell-opened FD mode.
    # This form holds the lock in its monitor; -o closes the
    # descriptor in our payload, including any subsequently detached worker.
    exec flock -xo "$state/lock" "$helper" locked "$@"
fi
shift
pending=$state/pending
accepted=$state/accepted
booting=no

fail() { echo "gonf-netbsd-network: $*" >&2; exit 1; }
gate() {
    [ -f "$state/$1" ] && [ ! -L "$state/$1" ] || fail "missing gate: $1"
    [ "$(stat -f '%u:%Lp' "$state/$1")" = 0:600 ] || fail "unsafe gate: $1"
    [ "$(cat "$state/$1")" = "$host" ] || fail "foreign gate: $1"
}
v4() {
    rows=$(ifconfig "$iface") || return
    printf '%s\n' "$rows" | awk '$1=="inet" {split($2,a,"/"); print a[1],a[2]}'
}
has4() { v4 | awk '{print $1}' | grep -qxF "$1"; }
has6() { ifconfig "$iface" | awk '$1=="inet6" {split($2,a,"/");print a[1]}' | grep -qxF "$ula"; }
default_gateway() {
    rows=$(route -n get default) || return
    printf '%s\n' "$rows" | awk '$1=="gateway:" {print $2}'
}
set_default() {
    desired=$1
    if current=$(default_gateway); then
        [ "$current" = "$desired" ] || route -n change default "$desired"
    else
        route -n add default "$desired"
    fi
}
return_gateway() {
    # Removing the primary IPv4 can remove routes tied to that address.
    # With no default either, a missing host route is still simply absent.
    if ! rows=$(route -n get 10.99.0.2); then
        echo absent
        return
    fi
    printf '%s\n' "$rows" | awk '
        $1=="destination:" {dest=$2} $1=="gateway:" {gw=$2}
        END {if(dest=="10.99.0.2") print gw; else print "absent"}'
}
filter_active() { npfctl show | grep -q 'filtering:[[:space:]]*active$'; }
reload_filter() {
    # At boot the ordinary NPF service will load after this rc hook.
    if [ "$booting" = no ]; then
        npfctl reload
        filter_active || fail 'NPF lost its active state'
    fi
}
set_return() {
    desired=$1
    current=$(return_gateway)
    if [ "$desired" = absent ]; then
        [ "$current" = absent ] || route -n delete -host 10.99.0.2
    elif [ "$current" = absent ]; then
        route -n add -host 10.99.0.2 "$desired"
    elif [ "$current" != "$desired" ]; then
        route -n change -host 10.99.0.2 "$desired"
    fi
}
# i24 retired the OLD address for good; the accepted retire modes run without it.
old_retired() { [ -f "$accepted" ] && grep -q '^retire ' "$accepted"; }
check_addresses() {
    old_retired || has4 "$old" || fail 'OLD IPv4 missing'
    foreign=$(v4 | awk -v a="$old" -v b="$lab" -v c="$recovery" '
        ($1!=a && $1!=b && $1!=c) || $2!=24 {print}')
    [ -z "$foreign" ] || fail 'unreviewed physical IPv4/prefix'
    foreign=$(ifconfig "$iface" | awk -v a="$ula" '
        $1=="inet6" {split($2,v,"/"); if(v[1]!~/^fe80:/ && (v[1]!=a || v[2]!=64)) print}')
    [ -z "$foreign" ] || fail 'unreviewed physical IPv6'
    if ndp -i "$iface" | grep -q accept_rtadv; then fail 'unexpected router advertisements'; fi
    current=$(return_gateway)
    case "$current" in absent|"$shared_gateway"|10.98.0.1) ;; *) fail 'foreign recovery route';; esac
}
select_primary() {
    target=$1
    has4 "$target" || ifconfig "$iface" inet "$target/24" alias
    while :; do
        first=$(v4 | awk 'NR==1 {print $1}')
        [ "$first" != "$target" ] || break
        case "$first" in "$old"|"$lab"|"$recovery") ifconfig "$iface" inet "$first" delete;;
            *) fail 'foreign primary';; esac
    done
}
runtime() {
    mode=$1 topology=$2
    # Every mode but retire keeps the OLD address (the i24 retirement drops it).
    [ "$mode" = retire ] || has4 "$old" || ifconfig "$iface" inet "$old/24" alias
    has4 "$lab" || ifconfig "$iface" inet "$lab/24" alias
    has6 || ifconfig "$iface" inet6 "$ula/64" alias
    # isolated keeps the igc1 recovery address; shared and lab (j24, the
    # final topology after the recovery retirement) run without it.
    if [ "$topology" = isolated ]; then
        has4 "$recovery" || ifconfig "$iface" inet "$recovery/24" alias
    else
        if has4 "$recovery"; then ifconfig "$iface" inet "$recovery" delete; fi
    fi
    if [ "$mode" = promote ]; then
        select_primary "$lab"
        has4 "$old" || ifconfig "$iface" inet "$old/24" alias
        set_default 10.20.0.1
    elif [ "$mode" = retire ]; then
        select_primary "$lab"
        if has4 "$old"; then ifconfig "$iface" inet "$old" delete; fi
        set_default 10.20.0.1
    fi
    # NetBSD deletes routes associated with an IPv4 address when it is removed.
    # Rebuild routes only after primary/alias changes have finished.
    # lab: no laptop host route; replies follow the 10.20.0.1 default.
    case "$topology" in
        isolated) set_return 10.98.0.1;;
        lab) set_return absent;;
        *) set_return "$shared_gateway";;
    esac
    reload_filter
}
snapshot() {
    check_addresses
    filter_active || fail 'NPF is not active'
    [ -f /etc/npf.conf ] && [ ! -L /etc/npf.conf ] || fail 'unsafe NPF config'
    [ ! -e "$pending" ] && [ ! -e "$state/snapshot" ] || fail 'unfinished transaction'
    mkdir "$state/snapshot"
    v4 > "$state/snapshot/v4"
    default_gateway > "$state/snapshot/default"
    return_gateway > "$state/snapshot/return"
    if has6; then touch "$state/snapshot/ula"; fi
    cp -p /etc/npf.conf "$state/snapshot/npf.conf"
    cp "$candidate" "$state/snapshot/applied.npf"
    if [ -f "$accepted" ]; then cp -p "$accepted" "$state/snapshot/accepted"; fi
    printf '%s\n' "$(($(date +%s) + 900))" > "$state/snapshot/deadline"
    mv "$state/snapshot" "$pending"
    sync
}
archive() { mv "$pending" "$state/$1-$(date +%s)-$$"; }
restore() {
    [ -d "$pending" ] || return 0
    cmp -s /etc/npf.conf "$pending/npf.conf" ||
        cmp -s /etc/npf.conf "$pending/applied.npf" || fail 'NPF config changed outside this transaction'
    if [ -f "$pending/rc.conf" ]; then
        # Retire rewrote the base network lines; refuse another owner's edit.
        cmp -s "$rcconf" "$pending/rc.conf" ||
            cmp -s "$rcconf" "$pending/rc.conf.applied" || fail 'rc.conf changed outside this transaction'
    fi
    cp -p "$pending/npf.conf" /etc/npf.conf.gonf-next
    mv /etc/npf.conf.gonf-next /etc/npf.conf
    if [ -f "$pending/rc.conf" ]; then
        cp -p "$pending/rc.conf" "$rcconf.gonf-next"
        mv "$rcconf.gonf-next" "$rcconf"
    fi
    if [ -f "$pending/accepted" ]; then
        cp -p "$pending/accepted" "$state/accepted.next"
        mv "$state/accepted.next" "$accepted"
    else rm -f "$accepted"; fi
    primary=$(awk 'NR==1 {print $1}' "$pending/v4")
    select_primary "$primary"
    while read -r address prefix; do
        has4 "$address" || ifconfig "$iface" inet "$address/$prefix" alias
    done < "$pending/v4"
    for address in "$old" "$lab" "$recovery"; do
        if ! awk '{print $1}' "$pending/v4" | grep -qxF "$address" && has4 "$address"; then
            ifconfig "$iface" inet "$address" delete
        fi
    done
    if [ -f "$pending/ula" ]; then
        has6 || ifconfig "$iface" inet6 "$ula/64" alias
    elif has6; then ifconfig "$iface" inet6 "$ula" delete; fi
    set_default "$(cat "$pending/default")"
    set_return "$(cat "$pending/return")"
    reload_filter
    archive rolled-back
    rm -f "$state/stage-approved" "$state/promote-approved" "$state/retire-approved" \
        "$state/retire-recovery-approved" "$state/confirm-approved"
    sync
}
# Recovery needs the independent cron watchdog and the boot hook.
prerequisites() {
    crontab -l | grep -Fq "$helper watchdog" || fail 'independent cron missing'
    pgrep -f '^/usr/sbin/cron($|[[:space:]])' >/dev/null || fail 'cron is not running'
    [ -x /etc/rc.d/gonf_network_migration ] || fail 'boot hook missing'
    grep -qx 'gonf_network_migration=YES' /etc/rc.conf.d/gonf_network_migration || fail 'boot hook disabled'
    npfctl validate "$candidate"
}

# The retire candidate replaces exactly one ifconfig_<iface>= line (the OLD
# primary) and one defaultroute= line with the lab primary and 10.20.0.1.
# Any other rc.conf mention of the old /24 refuses: its owner retires it.
rc_retire_candidate() {
    awk -v key="ifconfig_$iface=" -v old="inet $old netmask 0xffffff00" -v lab="$lab" '
        index($0, key) == 1 {
            n++
            if ($0 != key "\"" old "\"") exit 2
            print key "\"inet " lab " netmask 0xffffff00\""; next
        }
        index($0, "defaultroute=") == 1 { d++; print "defaultroute=\"10.20.0.1\""; next }
        { print }
        END { if (n != 1 || d != 1) exit 3 }' "$rcconf"
}

# Retire (i24) drops the retained OLD IPv4 for good. The accepted
# `promote isolated` profile becomes `retire isolated`, and rc.conf's OLD
# primary/default (the base configuration the boot hook overrides) becomes
# the lab primary and 10.20.0.1, so the host boots on the lab address even
# without the hook; the hook still adds recovery, ULA and the return route.
# The ordinary 900 s snapshot plus a private rc.conf copy restore the prior
# accepted mode, OLD address, routes and rc.conf on expiry, unconfirmed
# boot or rollback. NPF is unchanged ($host4 still lists OLD; harmless).
retire_apply() {
    gate activation-approved
    gate retire-approved
    gate igc1-approved
    if [ ! -f "$accepted" ] || ! grep -qx 'promote isolated' "$accepted"; then
        fail 'retire requires the accepted promote isolated profile'
    fi
    [ "$(v4 | awk 'NR==1 {print $1}')" = "$lab" ] || fail 'retire requires the lab primary'
    has4 "$recovery" || fail 'retire requires the recovery address'
    [ "$(default_gateway)" = 10.20.0.1 ] || fail 'retire requires the 10.20.0.1 default'
    [ "$(return_gateway)" = 10.98.0.1 ] || fail 'retire requires the 10.98.0.1 return route'
    prerequisites
    # Not "rows": v4/return_gateway (via snapshot) reuse that global.
    rc_rows=$(rc_retire_candidate) || fail 'rc.conf lacks the expected OLD primary/defaultroute lines'
    if printf '%s\n' "$rc_rows" | grep -qF "${old%.*}."; then
        fail 'rc.conf still references the old /24 elsewhere'
    fi
    snapshot
    cp -p "$rcconf" "$pending/rc.conf"
    printf '%s\n' "$rc_rows" > "$pending/rc.conf.applied"
    printf 'retire isolated\n' > "$pending/next"
    sync
    cp -p "$rcconf" "$rcconf.gonf-next"
    cat "$pending/rc.conf.applied" > "$rcconf.gonf-next"
    mv "$rcconf.gonf-next" "$rcconf"
    runtime retire isolated
    rm -f "$state/retire-approved"
}

# Retire-recovery (j24) removes the dedicated igc1 recovery path after i24:
# the accepted `retire isolated` profile becomes `retire lab`, which drops
# the recovery address and the laptop's 10.99.0.2 host route via 10.98.0.1
# (replies follow the 10.20.0.1 default), and the NPF candidate without the
# recovery and OLD rules replaces /etc/npf.conf. rc.conf already holds the lab
# primary and 10.20.0.1 since retire and carries no recovery line (the boot
# hook adds it per the accepted mode), so it stays untouched. The ordinary
# 900 s snapshot restores the previous accepted mode, NPF, address and route
# on expiry, unconfirmed boot or rollback.
recovery_retire_apply() {
    gate activation-approved
    gate retire-recovery-approved
    if [ ! -f "$accepted" ] || ! grep -qx 'retire isolated' "$accepted"; then
        fail 'retire-recovery requires the accepted retire isolated profile'
    fi
    ! has4 "$old" || fail 'retire-recovery requires the retired OLD address'
    [ "$(v4 | awk 'NR==1 {print $1}')" = "$lab" ] || fail 'retire-recovery requires the lab primary'
    has4 "$recovery" || fail 'retire-recovery requires the recovery address'
    [ "$(default_gateway)" = 10.20.0.1 ] || fail 'retire-recovery requires the 10.20.0.1 default'
    [ "$(return_gateway)" = 10.98.0.1 ] || fail 'retire-recovery requires the 10.98.0.1 return route'
    grep -qx "ifconfig_$iface=\"inet $lab netmask 0xffffff00\"" "$rcconf" ||
        fail 'rc.conf lacks the lab primary'
    grep -qx 'defaultroute="10.20.0.1"' "$rcconf" || fail 'rc.conf lacks defaultroute 10.20.0.1'
    if grep -qF "${recovery%.*}." "$rcconf"; then fail 'rc.conf references the recovery /24'; fi
    if grep -qF "$recovery" "$candidate"; then fail 'NPF candidate still admits the recovery address'; fi
    prerequisites
    snapshot
    printf 'retire lab\n' > "$pending/next"
    sync
    cp "$pending/applied.npf" /etc/npf.conf.gonf-next
    chmod 644 /etc/npf.conf.gonf-next
    mv /etc/npf.conf.gonf-next /etc/npf.conf
    runtime retire lab
    rm -f "$state/retire-recovery-approved"
}

apply() {
    action=$1 topology=$2
    case "$action:$topology" in stage:shared|stage:isolated|promote:shared|promote:isolated) ;; *) fail 'invalid action';; esac
    gate activation-approved
    gate "$action-approved"
    [ "$topology" != isolated ] || gate igc1-approved
    if [ "$action" = promote ]; then
        [ -f "$accepted" ] || fail 'accept Stage first'
        if ! grep -qx "stage $topology" "$accepted"; then
            # Shared-wire promotion is already accepted. The separately gated
            # igc1 transition adds recovery without downgrading NEW via Stage.
            if [ "$topology" != isolated ] || ! grep -qx 'promote shared' "$accepted"; then
                fail 'accept Stage first'
            fi
        fi
        has4 "$lab" || fail 'accepted lab address missing'
    elif [ -f "$accepted" ] && grep -q '^promote ' "$accepted"; then
        fail 'already promoted; use status rather than downgrading Stage'
    fi
    prerequisites
    snapshot
    printf '%s %s\n' "$action" "$topology" > "$pending/next"
    cp "$pending/applied.npf" /etc/npf.conf.gonf-next
    chmod 644 /etc/npf.conf.gonf-next
    mv /etc/npf.conf.gonf-next /etc/npf.conf
    runtime "$action" "$topology"
    rm -f "$state/$action-approved"
}

case ${1:-} in
    request)
        action=${2:-} topology=${3:-shared}
        case "$action" in stage|promote|retire|retire-recovery|rollback) ;; *) fail 'invalid request';; esac
        # Retire is defined for the isolated topology only, retire-recovery
        # for the final lab topology only.
        [ "$action" != retire ] || topology=isolated
        [ "$action" != retire-recovery ] || topology=lab
        [ ! -f "$state/worker.pid" ] || fail 'inspect existing worker before a new request'
        request=$(mktemp -d "$state/request.XXXXXX")
        printf '%s\n' "$action $topology" > "$request/action"
        # A fresh helper acquires a fresh flock; no inherited lock or SSH fd.
        nohup timeout 180 "$helper" worker "$action" "$topology" "$request" \
            </dev/null >"$request/log" 2>&1 &
        printf '%s\n' "Dispatched $request"
        ;;
    worker)
        action=${2:-} topology=${3:-} request=${4:-}
        case "$request" in "$state"/request.*) [ -d "$request" ] && [ ! -L "$request" ] || exit 1;; *) exit 1;; esac
        printf '%s\n' "$$" > "$state/worker.pid"
        result=0
        trap 'result=$?; rm -f "$state/worker.pid"; printf "%s\n" "$result" > "$request/result"' 0
        trap 'exit 143' TERM
        case "$action" in stage|promote) apply "$action" "$topology";; retire) retire_apply;;
            retire-recovery) recovery_retire_apply;;
            rollback) restore;; *) fail 'invalid worker';; esac
        ;;
    watchdog)
        if [ -d "$pending" ] && [ "$(date +%s)" -ge "$(cat "$pending/deadline")" ]; then restore; fi
        ;;
    boot)
        booting=yes
        rm -f "$state/worker.pid"
        if [ -d "$pending" ]; then
            restore
        elif [ -f "$accepted" ]; then
            read -r mode topology extra < "$accepted"
            [ -z "${extra:-}" ] || fail 'invalid accepted profile'
            case "$mode:$topology" in stage:shared|stage:isolated|promote:shared|promote:isolated|retire:isolated|retire:lab) ;;
                *) fail 'invalid accepted profile';; esac
            runtime "$mode" "$topology"
        fi
        ;;
    confirm)
        [ -d "$pending" ] || fail 'no pending transaction'
        [ "$(date +%s)" -lt "$(cat "$pending/deadline")" ] || fail 'expired transaction'
        gate confirm-approved
        cp "$pending/next" "$state/accepted.next"
        mv "$state/accepted.next" "$accepted"
        sync
        archive confirmed
        rm -f "$state/confirm-approved"
        sync
        ;;
    status)
        if [ -d "$pending" ]; then cat "$pending/deadline"; else echo 'No pending transaction'; fi
        if [ -f "$accepted" ]; then cat "$accepted"; fi
        ;;
    *) fail 'use request|watchdog|boot|confirm|status';;
esac
