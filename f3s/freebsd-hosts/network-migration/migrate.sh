#!/bin/sh
# FreeBSD base sh: do not depend on a package before netif at boot.
set -eu
PATH=/sbin:/bin:/usr/sbin:/usr/bin:/usr/local/sbin:/usr/local/bin
export PATH
umask 077
state=/var/db/gonf-network-migration
helper=/usr/local/sbin/gonf-network-migration
[ "$(id -u)" = 0 ] || exit 1
if [ "${1:-}" != locked ]; then
    exec lockf -k -t 0 "$state/lock" "$helper" locked "$@"
fi
shift
# shellcheck source=/dev/null
. /usr/local/etc/gonf-network-migration.conf
: "${iface:?}" "${old:?}" "${lab:?}" "${recovery:?}" "${ula:?}" "${shared_gateway:?}"
pending=$state/pending
keys="ifconfig_${iface} ifconfig_${iface}_alias200 ifconfig_${iface}_alias201 ifconfig_${iface}_alias202 ifconfig_${iface}_ipv6 defaultrouter ipv6_defaultrouter static_routes route_gonfmigration"

fail() { echo "gonf-network-migration: $*" >&2; exit 1; }

# Only these addresses are owned; CARP, bridge, wg0 and listeners stay intact.
v4_rows() {
    rows=$(ifconfig "$iface" inet) || return
    printf '%s\n' "$rows" | awk -v a="$old" -v b="$lab" -v c="$recovery" '
        $1 == "inet" && ($2 == a || $2 == b || $2 == c) {
            print $2, $4
        }'
}

v6_rows() {
    rows=$(ifconfig "$iface" inet6) || return
    printf '%s\n' "$rows" | awk '
        $1 == "inet6" && $2 !~ /^fe80:/ {
            for (i=3; i<=NF; i++) if ($i == "prefixlen") print $2, $(i+1)
        }'
}

save_key() {
    if sysrc -f /etc/rc.conf -n "$1" > "$state/preparing/$1" 2>/dev/null; then
        touch "$state/preparing/$1.exists"
    fi
}

restore_keys() {
    for key in $keys; do
        if [ -f "$pending/$key.exists" ]; then
            sysrc -f /etc/rc.conf "$key=$(cat "$pending/$key")" >/dev/null
        else
            # sysrc -x returns nonzero when a key is already absent.
            if sysrc -f /etc/rc.conf -n "$key" >/dev/null 2>&1; then
                sysrc -f /etc/rc.conf -x "$key" >/dev/null
            fi
        fi
    done
}

# p24 must install this root-owned hook before an endpoint-changing transaction.
# It must implement snapshot DIR, restore-runtime DIR, restore-boot DIR and confirm DIR, restoring only
# scoped source/persistent/runtime Endpoint deltas, and be idempotent on retry.
endpoint_hook() {
    if [ -f "$pending/endpoints-required" ]; then
        [ -x /usr/local/sbin/gonf-endpoint-rollback ] || fail 'p24 endpoint hook missing; rollback remains armed'
        /usr/local/sbin/gonf-endpoint-rollback "$1" "$pending/endpoints"
    fi
}

arm() {
    [ ! -d "$pending" ] || fail 'transaction already pending; confirm or rollback first'
    [ -f "$state/activation-approved" ] || fail 'activation gate missing; see README'
    case "$seconds" in ''|*[!0-9]*) fail 'invalid deadline';; esac
    [ "$seconds" -ge 120 ] && [ "$seconds" -le 3600 ] || fail 'deadline must be 120..3600 seconds'
    [ "$profile" = shared ] || [ "$profile" = isolated ] || fail 'unknown cabling profile'
    if [ "$profile" = isolated ]; then
        [ -f "$state/igc1-approved" ] || fail 'igc1 recovery/cabling gate missing'
    fi
    # Alias numbering belongs to this helper only. Refuse collisions rather
    # than replacing another recipe's persistent alias.
    for slot in 200 201 202; do
        value=$(sysrc -f /etc/rc.conf -n "ifconfig_${iface}_alias$slot" 2>/dev/null || :)
        case "$value" in ''|"inet $old/24"|"inet $lab/24"|"inet $recovery/24") :;; *) fail "alias$slot belongs to another owner";; esac
    done
    # No DHCP lease/client is silently converted to static configuration.
    current=$(sysrc -n "ifconfig_$iface")
    case "$current" in *DHCP*) fail 'DHCP interface requires reviewed static conversion';; esac
    pgrep -x dhcp6c >/dev/null && fail 'DHCPv6 client requires reviewed scoped stop'
    legacy=$(sysrc -n ipv6_enable 2>/dev/null || :)
    case "$legacy" in YES|yes) fail 'legacy ipv6_enable would re-enable RA at boot';; esac
    cpe=$(sysrc -n ipv6_cpe_wanif 2>/dev/null || :)
    [ "$cpe" != "$iface" ] || fail 'CPE configuration would re-enable RA at boot' 
    rm -rf "$state/preparing"
    mkdir "$state/preparing"
    for key in $keys; do save_key "$key"; done
    cp -p /etc/rc.conf "$state/preparing/rc.conf.evidence"
    ifconfig "$iface" > "$state/preparing/ifconfig.evidence"
    netstat -rn > "$state/preparing/routes.evidence"
    v4_rows > "$state/preparing/v4"
    v6_rows > "$state/preparing/v6"
    awk '{print $1}' "$state/preparing/v4" | grep -qxF "$old" || fail 'retained old host IPv4 is missing'
    rows=$(ifconfig "$iface" inet) || return
    printf '%s\n' "$rows" | awk '$1=="inet" {print $2; exit}' > "$state/preparing/primary"
    primary=$(cat "$state/preparing/primary")
    [ "$primary" = "$old" ] || [ "$primary" = "$lab" ] || fail 'unexpected primary; refusing CARP replacement'
    ifconfig "$iface" inet6 | grep 'nd6 options=.*ACCEPT_RTADV' > "$state/preparing/ra" || :
    route -n get -inet default > "$state/preparing/default4.evidence"
    awk '/gateway:/ {print $2}' "$state/preparing/default4.evidence" > "$state/preparing/default4"
    route -n get -inet 10.99.0.2 > "$state/preparing/return.evidence"
    # A pre-existing specific /32 must be handled explicitly, never overwritten.
    if grep -Eq 'destination: 10\.99\.0\.2$' "$state/preparing/return.evidence"; then
        saved_route=$(sysrc -f /etc/rc.conf -n route_gonfmigration 2>/dev/null || :)
        case "$saved_route" in
            "-host 10.99.0.2 $shared_gateway"|'-host 10.99.0.2 10.98.0.1') :;;
            *) fail 'existing wg2 host route requires ownership review';;
        esac
        awk '/gateway:/ {print $2}' "$state/preparing/return.evidence" > "$state/preparing/return"
    fi
    route -n get -inet6 default > "$state/preparing/default6.evidence" 2>/dev/null || :
    if grep -Eq "interface: $iface$" "$state/preparing/default6.evidence"; then
        awk '/gateway:/ {print $2}' "$state/preparing/default6.evidence" > "$state/preparing/default6"
    fi
    echo "$profile" > "$state/preparing/profile"
    echo "$(($(date +%s) + seconds))" > "$state/preparing/deadline"
    # Endpoint integration is deliberately explicit and refuses partial setup.
    if [ -f "$state/endpoints-required" ]; then
        [ -x /usr/local/sbin/gonf-endpoint-rollback ] || fail 'p24 endpoint hook missing'
        mkdir "$state/preparing/endpoints"
        /usr/local/sbin/gonf-endpoint-rollback snapshot "$state/preparing/endpoints"
        touch "$state/preparing/endpoints-required"
    fi
    mv "$state/preparing" "$pending"
    sync
}

set_return_route() {
    gateway=$shared_gateway
    if [ "$profile" = isolated ]; then gateway=10.98.0.1; fi
    routes=$(sysrc -n static_routes)
    case " $routes " in *' gonfmigration '*) :;; *) routes="$routes gonfmigration";; esac
    sysrc -f /etc/rc.conf "static_routes=$routes" "route_gonfmigration=-host 10.99.0.2 $gateway" >/dev/null
    if route -n get -inet 10.99.0.2 | grep -Eq 'destination: 10\.99\.0\.2$'; then
        route change -host 10.99.0.2 "$gateway"
    else
        route add -host 10.99.0.2 "$gateway"
    fi
}

stage() {
    sysrc -f /etc/rc.conf "ifconfig_${iface}_alias201=inet $lab/24" >/dev/null
    if ! v4_rows | awk '{print $1}' | grep -qxF "$lab"; then
        ifconfig "$iface" inet "$lab/24" alias
    fi
    if [ "$profile" = isolated ]; then
        sysrc -f /etc/rc.conf "ifconfig_${iface}_alias202=inet $recovery/24" >/dev/null
        if ! v4_rows | awk '{print $1}' | grep -qxF "$recovery"; then
            ifconfig "$iface" inet "$recovery/24" alias
        fi
    fi
    set_return_route
    sysrc -f /etc/rc.conf "ifconfig_${iface}_ipv6=inet6 $ula prefixlen 64 -accept_rtadv" >/dev/null
    ifconfig "$iface" inet6 -accept_rtadv
    # Read before deleting; pipelines must not hide ifconfig failures.
    v6_rows > "$pending/v6.remove"
    while read -r address prefix; do
        [ "$address" = "$ula" ] || ifconfig "$iface" inet6 "$address" delete
    done < "$pending/v6.remove"
    if ! v6_rows | awk '{print $1}' | grep -qxF "$ula"; then
        ifconfig "$iface" inet6 "$ula" prefixlen 64 alias
    fi
    if [ -s "$pending/default6" ]; then
        sysrc -f /etc/rc.conf ipv6_defaultrouter=NO >/dev/null
        if route -n get -inet6 default > "$pending/default6.current" 2>/dev/null; then
            grep -Eq "interface: $iface$" "$pending/default6.current" || fail 'IPv6 default changed owner'
            route delete -inet6 default
        fi
    fi
}

promote() {
    # The lab alias must already have passed Stage acceptance. A new arm
    # snapshots that accepted dual-address stage, rather than resetting it.
    v4_rows | awk '{print $1}' | grep -qxF "$lab" || fail 'stage lab address first'
    if [ "$profile" = isolated ]; then
        v4_rows | awk '{print $1}' | grep -qxF "$recovery" || fail 'stage isolated recovery alias first'
    fi
    set_return_route
    primary=$(cat "$pending/primary")
    if [ "$primary" != "$lab" ]; then
        ifconfig "$iface" inet "$lab" delete
        ifconfig "$iface" inet "$lab/24"
        ifconfig "$iface" inet "$old/24" alias
    fi
    sysrc -f /etc/rc.conf "ifconfig_${iface}=inet $lab/24" "ifconfig_${iface}_alias200=inet $old/24" >/dev/null
    sysrc -f /etc/rc.conf -x "ifconfig_${iface}_alias201" >/dev/null
    sysrc -f /etc/rc.conf defaultrouter=10.20.0.1 >/dev/null
    route change default 10.20.0.1
}

restore_runtime() {
    # Restore primary by replacing the primary only. Never clear the interface:
    # CARP alias0 and vm-bhyve bridge membership must survive the transaction.
    primary=$(cat "$pending/primary")
    current=$(ifconfig "$iface" inet | awk '$1=="inet" {print $2; exit}')
    if [ "$current" != "$primary" ]; then
        if v4_rows | awk '{print $1}' | grep -qxF "$primary"; then
            ifconfig "$iface" inet "$primary" delete
        fi
        mask=$(awk -v a="$primary" '$1==a {print $2}' "$pending/v4")
        ifconfig "$iface" inet "$primary" netmask "$mask"
    fi
    for address in "$old" "$lab" "$recovery"; do
        if ! awk '{print $1}' "$pending/v4" | grep -qxF "$address"; then
            if v4_rows | awk '{print $1}' | grep -qxF "$address"; then
                ifconfig "$iface" inet "$address" delete
            fi
        fi
    done
    while read -r address mask; do
        if ! v4_rows | awk '{print $1}' | grep -qxF "$address"; then
            ifconfig "$iface" inet "$address" netmask "$mask" alias
        fi
    done < "$pending/v4"
    route change default "$(cat "$pending/default4")"
    if [ -s "$pending/return" ]; then
        if route -n get -inet 10.99.0.2 | grep -Eq 'destination: 10\.99\.0\.2$'; then
            route change -host 10.99.0.2 "$(cat "$pending/return")"
        else
            route add -host 10.99.0.2 "$(cat "$pending/return")"
        fi
    elif route -n get -inet 10.99.0.2 | grep -Eq 'destination: 10\.99\.0\.2$'; then
        route delete -host 10.99.0.2
    fi
    v6_rows > "$pending/v6.remove"
    while read -r address prefix; do
        if ! awk '{print $1}' "$pending/v6" | grep -qxF "$address"; then
            ifconfig "$iface" inet6 "$address" delete
        fi
    done < "$pending/v6.remove"
    while read -r address prefix; do
        if ! v6_rows | awk '{print $1}' | grep -qxF "$address"; then
            ifconfig "$iface" inet6 "$address" prefixlen "$prefix" alias
        fi
    done < "$pending/v6"
    if [ -s "$pending/ra" ]; then ifconfig "$iface" inet6 accept_rtadv; fi
    if [ -s "$pending/default6" ]; then
        if ! route -n get -inet6 default >/dev/null 2>&1; then
            route add -inet6 default "$(cat "$pending/default6")"
        fi
    fi
}

rollback() {
    [ -d "$pending" ] || return 0
    # Address restoration comes first so old endpoint IPs are reachable.
    restore_keys
    if [ "$1" != boot ]; then restore_runtime; fi
    endpoint_hook "restore-$1"
    archive=$(mktemp -d "$state/rolled-back-$(date +%s).XXXXXX")
    mv "$pending" "$archive/snapshot"
    sync
}

case "${1:-}" in
    stage|promote)
        action=$1; profile=${2:-shared}; seconds=${3:-900}
        arm
        if [ "$action" = stage ]; then stage; else promote; fi
        echo 'Pending: fresh old/new connections and service checks required before confirm.'
        ;;
    confirm)
        [ -d "$pending" ] || fail 'no pending transaction'
        [ "$(date +%s)" -lt "$(cat "$pending/deadline")" ] || fail 'deadline expired; rollback required'
        endpoint_hook confirm
        archive=$(mktemp -d "$state/accepted-$(date +%s).XXXXXX")
        mv "$pending" "$archive/snapshot"
        sync
        ;;
    rollback) rollback runtime;;
    boot) rollback boot;;
    watchdog)
        if [ -d "$pending" ] && [ "$(date +%s)" -ge "$(cat "$pending/deadline")" ]; then
            rollback runtime
        fi
        ;;
    status)
        if [ -d "$pending" ]; then
            echo "pending profile=$(cat "$pending/profile") deadline=$(cat "$pending/deadline")"
        else echo 'no pending transaction'; fi
        ;;
    *) fail 'usage: stage|promote [shared|isolated] [seconds], confirm, rollback, watchdog, boot, status';;
esac
