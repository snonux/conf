#!/bin/sh
# gonf-bridge-migration (task 764): move an f-host's lab IPv4, lab ULA and,
# on the storage pair, the CARP VIP from the physical bridge member (re0)
# onto the vm-bhyve bridge (vm-public), under a journaled transaction with
# a timed restore (root cron), an early boot restore and an explicit
# confirm. Installed by gonf freebsd_bridge_migration_prepare; see
# BRIDGE.md next to this file.
#
# Why: with the addresses on the member, a bhyve guest's IPv6 neighbour
# solicitation for its own host arrives on the guest's tap, whose IPv6 is
# disabled, and is dropped; ND only answers for addresses of the receiving
# interface. On the bridge interface the host answers for every member,
# and FreeBSD 15 deprecates addresses on bridge members
# (net.link.bridge.member_ifaddrs).
#
# Persistent layout after "move" (rc.conf; network.subr maps the "-" of
# vm-public to "_" in variable names):
#   cloned_interfaces="... bridge0"  ifconfig_bridge0_name="vm-public"
#   ifconfig_vm_public="inet LAB/24 addm re0 up"
#   ifconfig_vm_public_ipv6="inet6 ULA prefixlen 64 auto_linklocal -accept_rtadv"
#   ifconfig_vm_public_alias0=<the CARP line that was ifconfig_re0_alias0>
#   ifconfig_re0="up"  (no ifconfig_re0_ipv6: netif marks re0 ifdisabled)
# and vm-bhyve's switch becomes type "manual" on that bridge, so "vm init"
# adopts the rc.conf bridge instead of creating (or failing to create) its
# own: type_SWITCH="manual" bridge_SWITCH="vm-public", no ports_SWITCH.
#
# Nothing here restarts netif, a service or a guest, or touches wg0, PF,
# keys or the bridge's members; the running bridge stays in place.
# shellcheck disable=SC2030,SC2031
set -eu
PATH=/sbin:/bin:/usr/sbin:/usr/bin:/usr/local/sbin:/usr/local/bin
export PATH
umask 077
state=/var/db/gonf-bridge-migration
helper=/usr/local/sbin/gonf-bridge-migration
rcconf=/etc/rc.conf
vmconf=/zroot/bhyve/.config/system.conf
oldstate=/var/db/gonf-network-migration
devdrule=/usr/local/etc/devd/carp.conf
[ "$(id -u)" = 0 ] || exit 1
if [ "${1:-}" != locked ]; then
    # The minute watchdog skips silently while an operator verb holds the
    # lock (-s), so cron does not mail root about it.
    quiet=; [ "${1:-}" != watchdog ] || quiet=-s
    exec lockf $quiet -k -t 0 "$state/lock" "$helper" locked "$@"
fi
shift
# shellcheck source=/dev/null
. /usr/local/etc/gonf-bridge-migration.conf
: "${member:?}" "${bridge:?}" "${switch:?}" "${lab:?}" "${ula:?}" "${gateway:?}"
pending=$state/pending
# rc.conf variable spelling of the bridge (network.subr's ltr of ".-/+").
bvar=$(printf '%s' "$bridge" | tr './+-' '____')
clone=bridge0
rckeys="cloned_interfaces ifconfig_${clone}_name ifconfig_${member} ifconfig_${member}_ipv6 ifconfig_${member}_alias0 ifconfig_${bvar} ifconfig_${bvar}_ipv6 ifconfig_${bvar}_alias0"
vmkeys="type_${switch} bridge_${switch} ports_${switch}"
tag='# gonf-bridge-migration absent keys'

fail() { echo "gonf-bridge-migration: $*" >&2; exit 1; }

# Address queries. ifconfig prints "inet A netmask M ... [vhid N]" and
# "inet6 A prefixlen P ..."; CARP rows carry " vhid ".
has_v4() { ifconfig "$1" inet 2>/dev/null | awk -v a="$2" '$1=="inet" && $2==a {f=1} END {exit !f}'; }
has_v6() { ifconfig "$1" inet6 2>/dev/null | awk -v a="$2" '$1=="inet6" && $2==a {f=1} END {exit !f}'; }
plain_v4() { ifconfig "$1" inet 2>/dev/null | awk '$1=="inet" && $0 !~ / vhid / {print $2}'; }
carp_v4() { ifconfig "$1" inet 2>/dev/null | awk '$1=="inet" && / vhid / {print $2}'; }
global_v6() { ifconfig "$1" inet6 2>/dev/null | awk '$1=="inet6" && $2 !~ /^fe80:/ {print $2}'; }
local_v6() { ifconfig "$1" inet6 2>/dev/null | awk '$1=="inet6" && $2 ~ /^fe80:/ {print $2}'; }
carp_state() { ifconfig "$1" 2>/dev/null | awk '$1=="carp:" {print $2; exit}'; }
default_if() { route -n get -inet default 2>/dev/null | awk '$1=="interface:" {print $2}'; }
default_gw() { route -n get -inet default 2>/dev/null | awk '$1=="gateway:" {print $2}'; }
rcval() { sysrc -f "$rcconf" -n "$1" 2>/dev/null || :; }
vmval() { sysrc -f "$vmconf" -n "$1" 2>/dev/null || :; }

# The CARP line is an ifconfig argument list ("inet vhid 1 [advskew N]
# pass P alias VIP/32"). It is read from the journal only and never echoed:
# it carries the vhid password.
carp_line() { cat "$pending/rc/ifconfig_${member}_alias0"; }
carp_vip() { carp_line | awk '{for (i=1; i<NF; i++) if ($i=="alias") {sub("/.*", "", $(i+1)); print $(i+1)}}'; }
carp_vhid() { carp_line | awk '{for (i=1; i<NF; i++) if ($i=="vhid") print $(i+1)}'; }
is_carp() { [ -f "$pending/rc/ifconfig_${member}_alias0.exists" ]; }

set_default4() {
    if route -n get -inet default >/dev/null 2>&1; then
        route change default "$gateway" >/dev/null
    else
        route add default "$gateway" >/dev/null
    fi
}

# Saved configuration as the moved boot will read it.
preflight_saved() {
    [ ! -e /etc/rc.conf.local ] || fail 'rc.conf.local present; review it first'
    [ "$(rcval "ifconfig_$member")" = "inet $lab/24" ] || fail "saved ifconfig_$member is not the lab primary"
    [ "$(rcval "ifconfig_${member}_ipv6")" = "inet6 $ula prefixlen 64 -accept_rtadv" ] ||
        fail "saved ifconfig_${member}_ipv6 is not the lab ULA"
    [ "$(rcval defaultrouter)" = "$gateway" ] || fail 'saved defaultrouter is not the lab gateway'
    # Only the IPv4 default is re-pointed; other saved routes need review.
    case "$(rcval ipv6_defaultrouter)" in ''|NO) :;; *) fail 'saved ipv6_defaultrouter needs review';; esac
    [ -z "$(rcval static_routes)$(rcval ipv6_static_routes)" ] || fail 'saved static routes need review'
    extra=$(grep -E "^ifconfig_${member}_alias[0-9]+=" "$rcconf" | grep -v "^ifconfig_${member}_alias0=" || :)
    [ -z "$extra" ] || fail "unowned ifconfig_${member}_alias lines"
    alias0=$(rcval "ifconfig_${member}_alias0")
    case "$alias0" in ''|'inet vhid '*' alias '*/32) :;; *) fail "ifconfig_${member}_alias0 is not a CARP /32";; esac
    if grep -Eq "^(ifconfig_${bvar}|ifconfig_${clone}_name)" "$rcconf"; then
        fail 'bridge keys already present in rc.conf'
    fi
    case " $(rcval cloned_interfaces) " in *" $clone "*) fail "cloned_interfaces already lists $clone";; esac
    ! ifconfig "$clone" >/dev/null 2>&1 || fail "$clone exists and would collide at boot"
    case "$(vmval "type_$switch")" in standard|'') :;; *) fail "switch $switch is not a standard vm-bhyve switch";; esac
    [ "$(vmval "ports_$switch")" = "$member" ] || fail "switch $switch does not own $member"
    [ -z "$(vmval "bridge_$switch")" ] || fail "switch $switch already names a bridge"
    [ ! -d "$oldstate/pending" ] && [ ! -e "$oldstate/confirmed-boot-guard" ] ||
        fail 'an address-migration transaction is active'
}

# Running state: everything on the member, nothing on the bridge yet.
preflight_runtime() {
    ifconfig "$bridge" 2>/dev/null | grep -q "member: $member " || fail "$member is not a member of $bridge"
    [ "$(plain_v4 "$member")" = "$lab" ] || fail "$member must carry exactly the lab IPv4"
    [ "$(global_v6 "$member")" = "$ula" ] || fail "$member must carry exactly the lab ULA"
    [ -z "$(plain_v4 "$bridge")$(carp_v4 "$bridge")$(global_v6 "$bridge")" ] || fail "$bridge already has addresses"
    [ "$(default_gw)" = "$gateway" ] || fail 'default route is not the lab gateway'
    ! route -n get -inet6 default >/dev/null 2>&1 || fail 'an IPv6 default route needs review'
    # The ruleset must already filter on the bridge, or PF drops everything
    # (CARP adverts included) that arrives there after the move.
    if pfctl -s Running >/dev/null 2>&1; then
        pfctl -s rules 2>/dev/null | grep -q " on $bridge " || fail "loaded PF ruleset does not cover $bridge; deploy freebsd_firewall_config first"
    fi
    vip=$(printf '%s\n' "$alias0" | awk '{for (i=1; i<NF; i++) if ($i=="alias") {sub("/.*", "", $(i+1)); print $(i+1)}}')
    [ "$(carp_v4 "$member")" = "$vip" ] || fail 'running CARP addresses differ from rc.conf'
    if [ -n "$vip" ]; then
        # devd would run carpcontrol.sh on the INIT->BACKUP->MASTER steps of
        # the recreated vhid and stop NFS/stunnel; the operator stops it.
        ! pgrep -x devd >/dev/null || fail 'stop devd first (CARP quiesce); see BRIDGE.md'
        # devd anchors its patterns: an old rule would never match "1@$bridge".
        grep -qF '[0-9a-z.-]+' "$devdrule" 2>/dev/null || fail 'devd CARP rule does not admit the bridge; deploy freebsd_carp_devd_hook first'
        if [ "$(carp_state "$member")" = MASTER ]; then
            [ -f "$state/carp-master-approved" ] || fail 'CARP MASTER needs the carp-master-approved gate'
        fi
    fi
}

save_key() {
    # $1 directory (rc|vm), $2 file, $3 key. Presence is read from the file
    # itself: sysrc -n also answers from /etc/defaults/rc.conf.
    if grep -q "^$3=" "$2"; then
        sysrc -f "$2" -n "$3" > "$state/preparing/$1/$3"
        touch "$state/preparing/$1/$3.exists"
    fi
}

arm() {
    case "$seconds" in ''|*[!0-9]*) fail 'invalid deadline';; esac
    [ "$seconds" -ge 120 ] && [ "$seconds" -le 3600 ] || fail 'deadline must be 120..3600 seconds'
    rm -rf "$state/preparing"
    mkdir -p "$state/preparing/rc" "$state/preparing/vm"
    for key in $rckeys; do save_key rc "$rcconf" "$key"; done
    for key in $vmkeys; do save_key vm "$vmconf" "$key"; done
    ifconfig "$member" > "$state/preparing/member.evidence"
    ifconfig "$bridge" > "$state/preparing/bridge.evidence"
    netstat -rn > "$state/preparing/routes.evidence"
    carp_state "$member" > "$state/preparing/carp-state" || :
    echo "$(($(date +%s) + seconds))" > "$state/preparing/deadline"
    mv "$state/preparing" "$pending"
    sync
    remove_tag
}

# Remove the boot-only unset statement of an earlier boot restore.
remove_tag() {
    grep -qF "$tag" "$rcconf" || return 0
    candidate=$(mktemp "$rcconf.gonf-bridge.XXXXXX")
    cp -p "$rcconf" "$candidate"
    grep -vF "$tag" "$rcconf" > "$candidate" || :
    mv "$candidate" "$rcconf"
}

write_persistent() {
    clones=$(rcval cloned_interfaces)
    sysrc -f "$rcconf" "cloned_interfaces=${clones:+$clones }$clone" \
        "ifconfig_${clone}_name=$bridge" \
        "ifconfig_${bvar}=inet $lab/24 addm $member up" \
        "ifconfig_${bvar}_ipv6=inet6 $ula prefixlen 64 auto_linklocal -accept_rtadv" \
        "ifconfig_${member}=up" >/dev/null
    sysrc -f "$rcconf" -x "ifconfig_${member}_ipv6" >/dev/null
    if is_carp; then
        sysrc -f "$rcconf" "ifconfig_${bvar}_alias0=$(carp_line)" >/dev/null
        sysrc -f "$rcconf" -x "ifconfig_${member}_alias0" >/dev/null
    fi
    sysrc -f "$vmconf" "type_${switch}=manual" "bridge_${switch}=$bridge" >/dev/null
    sysrc -f "$vmconf" -x "ports_${switch}" >/dev/null
}

# Re-add the VIP on $1 with the saved line. $2 is the role the vhid had
# on the other interface a moment ago: a MASTER is told to stay MASTER at
# once, so the peer's 3 s master-down timer never fires and the NFS server
# never changes. A new vhid otherwise starts in INIT and settles as BACKUP.
add_carp() {
    is_carp || return 0
    has_v4 "$1" "$(carp_vip)" && return 0
    # shellcheck disable=SC2046 # the saved rc.conf value is an argument list
    ifconfig "$1" $(carp_line)
    if [ "$2" = MASTER ]; then
        ifconfig "$1" vhid "$(carp_vhid)" state master
    fi
}

# $1 loses the addresses, $2 gains them. Used in both directions; every
# step checks before acting, so a retried rollback is idempotent.
shift_addresses() {
    role=$(carp_state "$1")
    # A failed move may have removed the VIP already: use the role saved at arm.
    [ -n "$role" ] || role=$(cat "$pending/carp-state" 2>/dev/null || :)
    if is_carp && has_v4 "$1" "$(carp_vip)"; then ifconfig "$1" inet "$(carp_vip)" -alias; fi
    if has_v4 "$1" "$lab"; then ifconfig "$1" inet "$lab" -alias; fi
    if has_v6 "$1" "$ula"; then ifconfig "$1" inet6 "$ula" -alias; fi
    if [ "$1" = "$member" ]; then
        ifconfig "$member" inet6 ifdisabled
        for address in $(local_v6 "$member"); do ifconfig "$member" inet6 "$address" -alias; done
        ifconfig "$bridge" inet6 -ifdisabled auto_linklocal -accept_rtadv
    else
        for address in $(local_v6 "$bridge"); do ifconfig "$bridge" inet6 "$address" -alias; done
        ifconfig "$bridge" inet6 ifdisabled -auto_linklocal
        # Clearing ifdisabled recreates re0's link-local (auto_linklocal).
        ifconfig "$member" inet6 -ifdisabled
    fi
    has_v4 "$2" "$lab" || ifconfig "$2" inet "$lab/24" alias
    has_v6 "$2" "$ula" || ifconfig "$2" inet6 "$ula" prefixlen 64 alias
    add_carp "$2" "$role"
    set_default4
}

restore_key() {
    # $1 directory (rc|vm), $2 file, $3 key; echoes absent keys
    if [ -f "$pending/$1/$3.exists" ]; then
        sysrc -f "$2" "$3=$(cat "$pending/$1/$3")" >/dev/null
    else
        # sysrc -n would also answer from defaults; -x needs a line in $2.
        if grep -q "^$3=" "$2"; then sysrc -f "$2" -x "$3" >/dev/null; fi
        echo "$3"
    fi
}

# Restore the saved keys. At boot rc has already sourced the moved
# rc.conf, and deleting a key does not clear its cached variable on the
# ALRM reload: a tagged unset statement does (removed by the next watchdog
# run or arm, once rc has consumed it).
restore_keys() {
    mode=$1
    absent=
    for key in $rckeys; do absent="$absent $(restore_key rc "$rcconf" "$key")"; done
    remove_tag
    # shellcheck disable=SC2086 # collapse the blanks of absent keys
    set -- $absent
    absent=$*
    if [ "$mode" = boot ] && [ -n "$absent" ]; then
        printf 'unset %s %s\n' "$absent" "$tag" >> "$rcconf"
    fi
    # system.conf last: rc.conf must be whole even if this write fails.
    for key in $vmkeys; do restore_key vm "$vmconf" "$key" >/dev/null; done
}

rollback() {
    [ -d "$pending" ] || return 0
    restore_keys "$1"
    if [ "$1" != boot ]; then
        trap '' HUP PIPE
        # As for the move: devd would run carpcontrol.sh on the recreated
        # vhid's INIT->BACKUP step and stop NFS/stunnel on the MASTER.
        if is_carp && pgrep -x devd >/dev/null; then service devd stop >/dev/null 2>&1; fi
        set +e
        (set -e; shift_addresses "$bridge" "$member") >> "$pending/log" 2>&1
        status=$?
        set -e
        # devd stays stopped on a half-restored CARP host: starting it would
        # let carpcontrol.sh act on the half-built vhid. Say so.
        [ "$status" = 0 ] || fail "runtime restore failed ($status); journal kept for retry, see $pending/log; on f0/f1 devd is left stopped"
        if is_carp && ! pgrep -x devd >/dev/null; then service devd start >/dev/null 2>&1 || :; fi
    fi
    archive "rolled-back"
}

# Archive the journal; the CARP line (with the vhid password) stays only in
# rc.conf, never in an archived snapshot.
archive() {
    rm -f "$pending/rc/ifconfig_${member}_alias0"
    archive=$(mktemp -d "$state/$1-$(date +%s).XXXXXX")
    mv "$pending" "$archive/snapshot"
    sync
}

verify_moved() {
    [ "$(plain_v4 "$bridge")" = "$lab" ] || fail "$bridge lacks the lab IPv4"
    [ "$(global_v6 "$bridge")" = "$ula" ] || fail "$bridge lacks the lab ULA"
    [ -z "$(plain_v4 "$member")$(carp_v4 "$member")$(global_v6 "$member")" ] || fail "$member still has addresses"
    [ "$(default_if)" = "$bridge" ] && [ "$(default_gw)" = "$gateway" ] || fail "default route is not via $bridge"
    [ "$(rcval "ifconfig_$bvar")" = "inet $lab/24 addm $member up" ] || fail 'saved bridge key differs'
    [ "$(vmval "type_$switch")" = manual ] || fail 'saved switch is not manual'
    if is_carp; then
        [ "$(carp_v4 "$bridge")" = "$(carp_vip)" ] || fail "$bridge lacks the CARP VIP"
        pgrep -x devd >/dev/null || fail 'devd is stopped; start it before confirming'
    fi
}

move() {
    [ ! -d "$pending" ] || fail 'transaction already pending; confirm or rollback first'
    [ -f "$state/move-approved" ] || fail 'move gate missing; see BRIDGE.md'
    preflight_saved
    preflight_runtime
    arm
    write_persistent
    # Runtime steps are logged, not printed: a stalled SSH stdout must not
    # stop the helper half way (SIGPIPE/HUP are ignored for the same reason).
    trap '' HUP PIPE
    set +e
    (set -e; shift_addresses "$member" "$bridge") >> "$pending/log" 2>&1
    status=$?
    set -e
    rm -f "$state/move-approved" "$state/carp-master-approved"
    [ "$status" = 0 ] || fail "runtime move failed ($status); pending, see $pending/log; rollback or wait for the watchdog"
    echo 'Pending: fresh connections and service checks required before confirm.'
}

boot_trial() {
    [ -d "$pending" ] || fail 'no pending transaction'
    case "$seconds" in ''|*[!0-9]*) fail 'invalid deadline';; esac
    [ "$seconds" -ge 300 ] && [ "$seconds" -le 3600 ] || fail 'boot-trial window must be 300..3600 seconds'
    verify_moved
    echo "$(($(date +%s) + seconds))" > "$pending/deadline"
    touch "$pending/boot-once"
    sync
    echo "Next boot keeps the moved configuration once; deadline=$(cat "$pending/deadline")."
}

# Early boot (rc.d, before netif): an unconfirmed transaction is restored,
# except for the one boot a boot-trial allowed while its deadline lasts.
boot() {
    [ -d "$pending" ] || return 0
    if [ -f "$pending/boot-once" ] && [ "$(date +%s)" -lt "$(cat "$pending/deadline")" ]; then
        rm -f "$pending/boot-once"
        sync
        return 0
    fi
    rollback boot
}

case "${1:-}" in
    move) seconds=${2:-900}; move;;
    boot-trial) seconds=${2:-1200}; boot_trial;;
    confirm)
        [ -d "$pending" ] || fail 'no pending transaction'
        [ "$(date +%s)" -lt "$(cat "$pending/deadline")" ] || fail 'deadline expired; rollback required'
        verify_moved
        rm -f "$pending/boot-once"
        archive accepted
        ;;
    rollback) rollback runtime;;
    boot) boot;;
    watchdog)
        if [ -d "$pending" ] && [ "$(date +%s)" -ge "$(cat "$pending/deadline")" ]; then
            rollback runtime
        fi
        [ -d "$pending" ] || remove_tag
        ;;
    status)
        if [ -d "$pending" ]; then
            trial=; [ ! -f "$pending/boot-once" ] || trial=' boot-trial'
            echo "pending deadline=$(cat "$pending/deadline")$trial"
        else echo 'no pending transaction'; fi
        ;;
    *) fail 'usage: move [seconds] | boot-trial [seconds] | confirm | rollback | watchdog | boot | status';;
esac
