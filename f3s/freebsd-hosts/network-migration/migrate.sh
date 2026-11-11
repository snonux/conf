#!/bin/sh
# FreeBSD base sh: do not depend on a package before netif at boot.
# Guard restoration binds pending only inside its subshell; callers retain it.
# shellcheck disable=SC2030,SC2031
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
: "${iface:?}" "${old:?}" "${lab:?}" "${recovery:?}" "${shared_gateway:?}"
ipv6_policy=${ipv6_policy:-ula}
case "$ipv6_policy" in
    ula) : "${ula:?}";;
    disabled) [ -z "${ula:-}" ] || exit 1;;
    *) echo "unreviewed native IPv6 policy" >&2; exit 1;;
esac
pending=$state/pending
guard=$state/confirmed-boot-guard
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

# CARP rows may precede the physical addresses after an address is deleted.
# Do not mistake a vhid's VIP for the host's primary or hide a foreign primary.
physical_primary() {
    rows=$(ifconfig "$iface" inet) || return
    printf '%s\n' "$rows" | awk '
        $1 == "inet" && $0 !~ /[[:space:]]vhid[[:space:]]/ { print $2; exit }'
}

check_primary_rotation() {
    rows=$(ifconfig "$iface" inet) || return
    foreign=$(printf '%s\n' "$rows" | awk -v a="$old" -v b="$lab" -v c="$recovery" '
        $1 == "inet" && $0 !~ /[[:space:]]vhid[[:space:]]/ &&
        $2 != a && $2 != b && $2 != c { print $2 }')
    [ -z "$foreign" ] || fail 'unmanaged physical IPv4 alias requires review before primary rotation'
}

# Keep the desired address present while rotating only preceding owned
# addresses. An implicit SIOCSIFADDR replacement can delete a CARP VIP;
# explicit alias operations never replace another owner's first address.
select_primary() {
    rotation_target=$1
    if ! v4_rows | awk '{print $1}' | grep -qxF "$rotation_target"; then
        mask=$(awk -v a="$rotation_target" '$1==a {print $2}' "$pending/v4")
        [ -n "$mask" ] || fail 'saved physical primary mask missing'
        ifconfig "$iface" inet "$rotation_target" netmask "$mask" alias
    fi
    while :; do
        rotation_current=$(physical_primary) || return
        [ "$rotation_current" != "$rotation_target" ] || break
        case $rotation_current in
            "$old"|"$lab"|"$recovery") ifconfig "$iface" inet "$rotation_current" delete;;
            *) fail 'unmanaged physical primary prevents safe rotation';;
        esac
    done
}

restore_v4_aliases() {
    while read -r address mask; do
        if ! v4_rows | awk '{print $1}' | grep -qxF "$address"; then
            ifconfig "$iface" inet "$address" netmask "$mask" alias
        fi
    done < "$pending/v4"
}

# FreeBSD remembers a peer's local source address independently of the route.
# Reapply the unchanged live configuration in place after a default change:
# setconf recreates peer transport state without touching wg0 addresses/routes.
# The original complete input remains in pending if a partial setconf fails.
refresh_wg_runtime() {
    [ -f "$pending/wg0.runtime.conf" ] || return 0
    [ ! -L "$pending/wg0.runtime.conf" ] || fail 'unsafe WireGuard runtime snapshot'
    ifconfig wg0 >/dev/null 2>&1 || fail 'wg0 missing; runtime restoration remains pending'
    wg setconf wg0 "$pending/wg0.runtime.conf"
}

v6_rows() {
    rows=$(ifconfig "$iface" inet6) || return
    printf '%s\n' "$rows" | awk '
        $1 == "inet6" && $2 !~ /^fe80:/ {
            for (i=3; i<=NF; i++) if ($i == "prefixlen") print $2, $(i+1)
        }'
}

# Keep native ownership and finite lifetimes: recreating an autoconfigured
# address as a plain alias would silently turn it into an immortal static IP.
save_v6_metadata() {
    rows=$(ifconfig -L "$iface" inet6) || return
    printf '%s\n' "$rows" | awk '
        $1 == "inet6" && $2 !~ /^fe80:/ {
            prefix=""; owner="-autoconf"; preferred="infty"; valid="infty"
            for (i=3; i<=NF; i++) {
                if ($i == "prefixlen") prefix=$(i+1)
                if ($i == "autoconf") owner="autoconf"
                if ($i == "temporary") exit 1
                if ($i == "pltime") preferred=$(i+1)
                if ($i == "vltime") valid=$(i+1)
            }
            if (prefix !~ /^[0-9]+$/) exit 1
            if (owner == "autoconf" && (preferred == "infty" || valid == "infty")) exit 1
            if (preferred != "infty" && preferred !~ /^[0-9]+$/) exit 1
            if (valid != "infty" && valid !~ /^[0-9]+$/) exit 1
            print $2, prefix, owner, preferred, valid
        }'
}

restore_v6_metadata() {
    elapsed=$(($(date +%s) - $(cat "$pending/v6-captured-at")))
    [ "$elapsed" -ge 0 ] || elapsed=0
    while read -r address prefix owner preferred valid; do
        if [ "$valid" != infty ]; then
            valid=$((valid - elapsed))
            [ "$valid" -gt 0 ] || continue
        fi
        if [ "$preferred" != infty ]; then
            preferred=$((preferred - elapsed))
            [ "$preferred" -ge 0 ] || preferred=0
        fi
        if ! v6_rows | awk '{print $1}' | grep -qxF "$address"; then
            set -- "$iface" inet6 "$address" prefixlen "$prefix" "$owner"
            if [ "$preferred" != infty ]; then set -- "$@" pltime "$preferred"; fi
            if [ "$valid" != infty ]; then set -- "$@" vltime "$valid"; fi
            ifconfig "$@" alias
        fi
    done < "$pending/v6-metadata"
}

save_key() {
    if sysrc -f /etc/rc.conf -n "$1" > "$state/preparing/$1" 2>/dev/null; then
        touch "$state/preparing/$1.exists"
    fi
}

# rc has already sourced the unconfirmed keys into its parent shell. Merely
# deleting an assignment does not clear that cached variable on ALRM reload.
# A tagged unset statement acts in the sourcing parent's environment; empty
# alias assignments are unsafe because network.subr stops at an empty alias.
rewrite_absent_keys() {
    absent_keys=$1
    candidate=$(mktemp /etc/rc.conf.gonf-migration.XXXXXX)
    cp -p /etc/rc.conf "$candidate"
    awk '!/^unset .* # gonf-network-migration absent keys$/' /etc/rc.conf > "$candidate"
    if [ -n "$absent_keys" ]; then
        printf 'unset%s # gonf-network-migration absent keys\n' "$absent_keys" >> "$candidate"
    fi
    mv "$candidate" /etc/rc.conf
}

restore_keys() {
    absent_keys=
    for key in $keys; do
        if [ -f "$pending/$key.exists" ]; then
            sysrc -f /etc/rc.conf "$key=$(cat "$pending/$key")" >/dev/null
        else
            absent_keys="$absent_keys $key"
            # sysrc -x returns nonzero when a key is already absent.
            if sysrc -f /etc/rc.conf -n "$key" >/dev/null 2>&1; then
                sysrc -f /etc/rc.conf -x "$key" >/dev/null
            fi
        fi
    done
    rewrite_absent_keys "$absent_keys"
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
    chmod 600 "$state/preparing/rc.conf.evidence"
    ifconfig "$iface" > "$state/preparing/ifconfig.evidence"
    netstat -rn > "$state/preparing/routes.evidence"
    v4_rows > "$state/preparing/v4"
    v6_rows > "$state/preparing/v6"
    date +%s > "$state/preparing/v6-captured-at"
    save_v6_metadata > "$state/preparing/v6-metadata" || fail 'unsupported native IPv6 metadata; no address changed'
    # retire-recovery (j24) runs after i24 retired the old address; every
    # other action still needs it.
    if [ "$action" != retire-recovery ]; then
        awk '{print $1}' "$state/preparing/v4" | grep -qxF "$old" || fail 'retained old host IPv4 is missing'
    fi
    physical_primary > "$state/preparing/primary"
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
    if { [ "$action" = promote ] || [ "$action" = retire ] || [ "$action" = retire-recovery ]; } &&
        [ "$iface" = re0 ] && ifconfig wg0 >/dev/null 2>&1; then
        wg showconf wg0 > "$state/preparing/wg0.runtime.conf"
        chmod 600 "$state/preparing/wg0.runtime.conf"
    fi
    mv "$state/preparing" "$pending"
    sync
    # A previous rollback's absent keys are now represented by absent
    # assignments. Remove its reload-only cleanup before sysrc adds new ones.
    rewrite_absent_keys ""
}

set_return_route() {
    gateway=$shared_gateway
    if [ "$profile" = isolated ]; then gateway=10.98.0.1; fi
    routes=$(sysrc -n static_routes 2>/dev/null || :)
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
    ipv6_config='inet6 -accept_rtadv'
    if [ "$ipv6_policy" = ula ]; then
        ipv6_config="inet6 $ula prefixlen 64 -accept_rtadv"
    fi
    sysrc -f /etc/rc.conf "ifconfig_${iface}_ipv6=$ipv6_config" >/dev/null
    ifconfig "$iface" inet6 -accept_rtadv
    # Read before deleting; pipelines must not hide ifconfig failures.
    v6_rows > "$pending/v6.remove"
    while read -r address prefix; do
        [ "$address" = "${ula:-}" ] || ifconfig "$iface" inet6 "$address" delete
    done < "$pending/v6.remove"
    if [ "$ipv6_policy" = ula ] && ! v6_rows | awk '{print $1}' | grep -qxF "$ula"; then
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

# Replacing a physical primary can remove its connected gateway routes.
# Select add/change from actual native state, including during rollback.
set_default4() {
    if route -n get -inet default >/dev/null 2>&1; then
        route change default "$1"
    else
        route add default "$1"
    fi
}

promote() {
    # The lab alias must already have passed Stage acceptance. A new arm
    # snapshots that accepted dual-address stage, rather than resetting it.
    v4_rows | awk '{print $1}' | grep -qxF "$lab" || fail 'stage lab address first'
    if [ "$profile" = isolated ]; then
        # arm already captured the current-cabling profile and checked igc1.
        # Add recovery here so an accepted lab primary needs no second Stage
        # that would persist a duplicate lab alias alongside its primary.
        sysrc -f /etc/rc.conf "ifconfig_${iface}_alias202=inet $recovery/24" >/dev/null
        if ! v4_rows | awk '{print $1}' | grep -qxF "$recovery"; then
            ifconfig "$iface" inet "$recovery/24" alias
        fi
    fi
    select_primary "$lab"
    restore_v4_aliases
    set_return_route
    sysrc -f /etc/rc.conf "ifconfig_${iface}=inet $lab/24" "ifconfig_${iface}_alias200=inet $old/24" >/dev/null
    if sysrc -f /etc/rc.conf -n "ifconfig_${iface}_alias201" >/dev/null 2>&1; then
        sysrc -f /etc/rc.conf -x "ifconfig_${iface}_alias201" >/dev/null
    fi
    sysrc -f /etc/rc.conf defaultrouter=10.20.0.1 >/dev/null
    set_default4 10.20.0.1
    refresh_wg_runtime
}

# Retire (i24) removes only the retained old IPv4 (alias200) after the
# isolated promote was accepted: lab primary, recovery alias202, default
# 10.20.0.1 and the 10.98.0.1 laptop return route must already be in place.
# The caller has armed the ordinary journal, so restore_keys/restore_runtime
# and the boot hook bring alias200 back when it is not confirmed in time.
# A CARP VIP in the old /24 is refused: it belongs to the carp/relayd/stunnel
# owners and is retired with them first, never implicitly here.
# The runtime checks above are not enough: netif and routing read rc.conf at
# the next boot. Without alias200 a saved default or static route through the
# old /24 would come up without its connected subnet and strand the host, so
# the saved default and laptop return route must already be the promoted ones
# and no saved route may use the old /24 (destination or gateway). sysrc
# without -f reads the merged configuration that boot actually uses.
retire_saved_routes() {
    [ "$(sysrc -n defaultrouter 2>/dev/null || :)" = 10.20.0.1 ] ||
        fail 'retire requires the saved defaultrouter 10.20.0.1'
    [ "$(sysrc -n route_gonfmigration 2>/dev/null || :)" = '-host 10.99.0.2 10.98.0.1' ] ||
        fail 'retire requires the saved isolated 10.98.0.1 return route'
    routes=$(sysrc -n static_routes 2>/dev/null || :)
    case " $routes " in *' gonfmigration '*) :;; *) fail 'static_routes lacks gonfmigration';; esac
    for name in $routes; do
        for word in $(sysrc -n "route_$name" 2>/dev/null || :); do
            case "$word" in "$old_net"*) fail "saved route_$name uses the old /24";; esac
        done
    done
}

retire_preflight() {
    [ -f "$state/retire-approved" ] || fail 'retire gate missing; see README'
    [ "$(physical_primary)" = "$lab" ] || fail 'retire requires the accepted lab primary'
    for address in "$old" "$recovery"; do
        v4_rows | awk '{print $1}' | grep -qxF "$address" || fail "retire requires $address present"
    done
    [ "$(sysrc -f /etc/rc.conf -n "ifconfig_$iface" 2>/dev/null || :)" = "inet $lab/24" ] ||
        fail 'persistent primary is not the lab address'
    [ "$(sysrc -f /etc/rc.conf -n "ifconfig_${iface}_alias200" 2>/dev/null || :)" = "inet $old/24" ] ||
        fail 'alias200 is not the retained old address'
    [ "$(sysrc -f /etc/rc.conf -n "ifconfig_${iface}_alias202" 2>/dev/null || :)" = "inet $recovery/24" ] ||
        fail 'alias202 is not the recovery address'
    [ "$(route -n get -inet default | awk '/gateway:/ {print $2}')" = 10.20.0.1 ] ||
        fail 'retire requires the promoted 10.20.0.1 default'
    [ "$(route -n get -inet 10.99.0.2 | awk '/gateway:/ {print $2}')" = 10.98.0.1 ] ||
        fail 'retire requires the isolated 10.98.0.1 return route'
    old_net=${old%.*}.
    retire_saved_routes
    if ifconfig "$iface" inet | awk -v p="$old_net" '$1 == "inet" && index($2, p) == 1 && / vhid /' | grep -q .; then
        fail 'a CARP VIP in the old /24 is still configured; retire it with its owners first'
    fi
}

retire() {
    if sysrc -f /etc/rc.conf -n "ifconfig_${iface}_alias200" >/dev/null 2>&1; then
        sysrc -f /etc/rc.conf -x "ifconfig_${iface}_alias200" >/dev/null
    fi
    if v4_rows | awk '{print $1}' | grep -qxF "$old"; then
        ifconfig "$iface" inet "$old" delete
    fi
    # A peer may still cache the deleted address as its UDP source.
    refresh_wg_runtime
    # Single-use approval, as the README requires for every transaction.
    rm -f "$state/retire-approved"
}

# Retire-recovery (j24) removes the dedicated igc1 recovery path after i24:
# alias202 (the recovery address) and the laptop's 10.99.0.2 return route via
# 10.98.0.1 (route_gonfmigration in static_routes). Replies to the laptop then
# follow the 10.20.0.1 default, the router's normal wg2 path, so the router
# can drop 10.98.0.1 afterwards. The caller has armed the ordinary journal:
# restore_keys/restore_runtime and the boot hook bring both back when it is
# not confirmed in time. Every precondition is checked before arming, saved
# (merged rc.conf, what boot uses) as well as runtime: the old address must
# already be gone, the lab primary and 10.20.0.1 default in place, and no
# other saved route may use the recovery /24.
retire_recovery_preflight() {
    [ -f "$state/retire-recovery-approved" ] || fail 'retire-recovery gate missing; see README'
    [ "$(physical_primary)" = "$lab" ] || fail 'retire-recovery requires the accepted lab primary'
    ! v4_rows | awk '{print $1}' | grep -qxF "$old" || fail 'retire the old address (i24) first'
    v4_rows | awk '{print $1}' | grep -qxF "$recovery" || fail 'recovery address already absent'
    [ "$(sysrc -f /etc/rc.conf -n "ifconfig_$iface" 2>/dev/null || :)" = "inet $lab/24" ] ||
        fail 'persistent primary is not the lab address'
    ! sysrc -f /etc/rc.conf -n "ifconfig_${iface}_alias200" >/dev/null 2>&1 ||
        fail 'alias200 (old address) is still saved'
    [ "$(sysrc -f /etc/rc.conf -n "ifconfig_${iface}_alias202" 2>/dev/null || :)" = "inet $recovery/24" ] ||
        fail 'alias202 is not the recovery address'
    [ "$(route -n get -inet default | awk '/gateway:/ {print $2}')" = 10.20.0.1 ] ||
        fail 'retire-recovery requires the 10.20.0.1 default'
    [ "$(sysrc -n defaultrouter 2>/dev/null || :)" = 10.20.0.1 ] ||
        fail 'retire-recovery requires the saved defaultrouter 10.20.0.1'
    [ "$(route -n get -inet 10.99.0.2 | awk '/gateway:/ {print $2}')" = 10.98.0.1 ] ||
        fail 'retire-recovery requires the 10.98.0.1 return route'
    [ "$(sysrc -n route_gonfmigration 2>/dev/null || :)" = '-host 10.99.0.2 10.98.0.1' ] ||
        fail 'retire-recovery requires the saved 10.98.0.1 return route'
    routes=$(sysrc -n static_routes 2>/dev/null || :)
    case " $routes " in *' gonfmigration '*) :;; *) fail 'static_routes lacks gonfmigration';; esac
    for name in $routes; do
        [ "$name" != gonfmigration ] || continue
        for word in $(sysrc -n "route_$name" 2>/dev/null || :); do
            case "$word" in 10.98.*) fail "saved route_$name uses the recovery /24";; esac
        done
    done
}

retire_recovery() {
    routes=
    for name in $(sysrc -f /etc/rc.conf -n static_routes 2>/dev/null || :); do
        [ "$name" = gonfmigration ] || routes="$routes $name"
    done
    if [ -n "$routes" ]; then
        sysrc -f /etc/rc.conf "static_routes=${routes# }" >/dev/null
    elif sysrc -f /etc/rc.conf -n static_routes >/dev/null 2>&1; then
        sysrc -f /etc/rc.conf -x static_routes >/dev/null
    fi
    for key in route_gonfmigration "ifconfig_${iface}_alias202"; do
        if sysrc -f /etc/rc.conf -n "$key" >/dev/null 2>&1; then
            sysrc -f /etc/rc.conf -x "$key" >/dev/null
        fi
    done
    if route -n get -inet 10.99.0.2 | grep -Eq 'destination: 10\.99\.0\.2$'; then
        route delete -host 10.99.0.2
    fi
    if v4_rows | awk '{print $1}' | grep -qxF "$recovery"; then
        ifconfig "$iface" inet "$recovery" delete
    fi
    # A peer may still cache the deleted address as its UDP source.
    refresh_wg_runtime
    # Single-use approval, as the README requires for every transaction.
    rm -f "$state/retire-recovery-approved"
}

restore_runtime() {
    # Rotate only owned addresses back to the saved physical primary. Never
    # clear the interface or replace a CARP alias; bridge membership survives.
    primary=$(cat "$pending/primary")
    select_primary "$primary"
    for address in "$old" "$lab" "$recovery"; do
        if ! awk '{print $1}' "$pending/v4" | grep -qxF "$address"; then
            if v4_rows | awk '{print $1}' | grep -qxF "$address"; then
                ifconfig "$iface" inet "$address" delete
            fi
        fi
    done
    restore_v4_aliases
    set_default4 "$(cat "$pending/default4")"
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
    restore_v6_metadata
    if [ -s "$pending/ra" ]; then ifconfig "$iface" inet6 accept_rtadv; fi
    if [ -s "$pending/default6" ]; then
        if ! route -n get -inet6 default >/dev/null 2>&1; then
            # route(8) defaults to STATIC, whereas a router-advertisement
            # default belongs to the kernel's expiring router list.
            if grep -Eq 'flags: .*STATIC' "$pending/default6.evidence"; then
                route add -inet6 default "$(cat "$pending/default6")"
            else
                route add -inet6 -nostatic default "$(cat "$pending/default6")"
            fi
        fi
    fi
    refresh_wg_runtime
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

# The confirmed-boot guard owns a separate OLD snapshot. Ordinary pending
# transactions keep their unconditional early-boot rollback semantics.
guard_private() {
    [ ! -L "$1" ] && [ -e "$1" ] || fail 'guard path missing or symlinked'
    guard_meta=$(stat -f '%u:%g:%Lp' "$1")
    case $guard_meta in 0:0:*) :;; *) fail 'guard path is not root-owned';; esac
    case $guard_meta in
        0:0:700) [ -d "$1" ] || fail 'guard directory type changed';;
        0:0:600) [ -f "$1" ] || fail 'guard file type changed';;
        *) fail 'guard requires directories 0700 and files 0600';;
    esac
}

guard_hash() (
    guard_private "$1"
    [ -d "$1" ] || fail 'guard snapshot is not a directory'
    hash_file=$(mktemp "$state/guard-hash.XXXXXX")
    trap 'rm -f "$hash_file"' 0
    LC_ALL=C; export LC_ALL
    for item in "$1"/* "$1"/.[!.]* "$1"/..?*; do
        [ -e "$item" ] || [ -L "$item" ] || continue
        guard_private "$item"
        [ -f "$item" ] || fail 'guard snapshot must contain regular files only'
        name=${item##*/}
        case $name in v6.remove) continue;; *[!a-zA-Z0-9._-]*) fail 'invalid snapshot filename';; esac
        digest=$(sha256 -q "$item") || exit 1
        printf '%s %s\n' "$name" "$digest" >> "$hash_file"
    done
    sha256 -q "$hash_file"
)

guard_source() (
    case $1 in accepted-*) :;; *) fail 'guard requires an accepted OLD snapshot';; esac
    case $1 in *[!a-zA-Z0-9._-]*) fail 'invalid accepted snapshot name';; esac
    guard_private "$state"; guard_private "$state/$1"
    source_dir=$state/$1/snapshot
    guard_private "$source_dir"
    for field in primary default4 profile v4; do guard_private "$source_dir/$field"; done
    [ "$(cat "$source_dir/primary")" = "$old" ] || fail 'snapshot primary is not OLD'
    [ "$(cat "$source_dir/default4")" = 192.168.1.1 ] || fail 'snapshot default is not OLD'
    [ "$(cat "$source_dir/profile")" = shared ] || fail 'guard requires shared profile'
    awk '{print $1}' "$source_dir/v4" | grep -qxF "$lab" || fail 'snapshot lacks accepted Stage alias'
    [ ! -e "$source_dir/endpoints-required" ] || fail 'guard is address-only'
    printf '%s\n' "$source_dir"
)

guard_id_valid() {
    case $1 in ''|*[!0-9a-f]*) return 1;; esac
    [ "${#1}" -eq 64 ]
}

guard_boot_id() (
    guard_private "$state"
    id_file=$(mktemp "$state/boot-id.XXXXXX") || exit 1
    trap 'rm -f "$id_file"' 0
    trap 'exit 1' 1 2 3 15
    # kern.boot_id is opaque, not text. Check the producer status before
    # hashing; a failed sysctl can still have written partial or full output.
    sysctl -b kern.boot_id > "$id_file" || exit 1
    id_size=$(wc -c < "$id_file") || exit 1
    [ "$id_size" -eq 16 ] || fail 'invalid opaque boot identity size'
    sha256 -q "$id_file"
)

guard_new_primary() {
    [ "$(physical_primary)" = "$lab" ] || fail 'NEW physical primary required'
    v4_rows | awk '{print $1}' | grep -qxF "$old" || fail 'retained OLD alias missing'
    [ "$(sysrc -n "ifconfig_$iface")" = "inet $lab/24" ] || fail 'NEW persistent primary required'
    [ "$(sysrc -n "ifconfig_${iface}_alias200")" = "inet $old/24" ] || fail 'persistent OLD alias required'
    [ "$(sysrc -n defaultrouter)" = 10.20.0.1 ] || fail 'NEW persistent default required'
    route -n get -inet default | grep -q 'gateway: 10.20.0.1$' || fail 'NEW runtime default required'
}

guard_check() (
    checked=${1:-$guard}
    guard_private "$checked"
    [ -f "$checked/ready" ] || fail 'guard arming did not finish'
    for field in ready source hash deadline preboot-id config-hash helper-hash; do
        guard_private "$checked/$field"
    done
    [ ! -d "$state/pending" ] || fail 'ordinary pending overlaps confirmed guard'
    source_dir=$(guard_source "$(cat "$checked/source")")
    [ "$(guard_hash "$source_dir")" = "$(cat "$checked/hash")" ] || fail 'accepted OLD snapshot changed'
    [ "$(guard_hash "$checked/old")" = "$(cat "$checked/hash")" ] || fail 'guard working snapshot changed'
    [ "$(sha256 -q /usr/local/etc/gonf-network-migration.conf)" = "$(cat "$checked/config-hash")" ] || fail 'guard config changed'
    [ "$(sha256 -q "$helper")" = "$(cat "$checked/helper-hash")" ] || fail 'guard helper changed'
    value=$(cat "$checked/deadline")
    case $value in ''|*[!0-9]*) fail 'invalid guard deadline';; esac
    guard_id_valid "$(cat "$checked/preboot-id")" || fail 'invalid guard boot identity'
)

guard_arm() (
    [ -f "$state/activation-approved" ] || fail 'activation gate missing'
    [ ! -e "$guard" ] && [ ! -L "$guard" ] || fail 'confirmed guard already exists'
    [ ! -d "$pending" ] || fail 'ordinary transaction is pending'
    [ ! -d /var/db/gonf-endpoint/pending ] || fail 'endpoint transaction is pending'
    case $3 in ''|*[!0-9]*) fail 'invalid guard lifetime';; esac
    [ "$3" -ge 120 ] && [ "$3" -le 900 ] || fail 'guard lifetime must be 120..900 seconds'
    [ "${#2}" -eq 64 ] || fail 'invalid snapshot hash'
    case $2 in *[!0-9a-f]*) fail 'invalid snapshot hash';; esac
    source_dir=$(guard_source "$1")
    [ "$(guard_hash "$source_dir")" = "$2" ] || fail 'OLD snapshot does not match approved hash'
    guard_new_primary
    boot_id=$(guard_boot_id) || fail 'kernel boot identity unavailable'
    stage=$(mktemp -d "$state/confirmed-boot-preparing.XXXXXX")
    cp -Rp "$source_dir" "$stage/old"
    printf '%s\n' "$1" > "$stage/source"
    printf '%s\n' "$2" > "$stage/hash"
    printf '%s\n' "$boot_id" > "$stage/preboot-id"
    printf '%s\n' "$(($(date +%s) + $3))" > "$stage/deadline"
    sha256 -q /usr/local/etc/gonf-network-migration.conf > "$stage/config-hash"
    sha256 -q "$helper" > "$stage/helper-hash"
    touch "$stage/ready"
    guard_check "$stage"
    [ ! -e "$guard" ] && [ ! -L "$guard" ] || fail 'guard appeared while arming'
    arm_deadline=$(cat "$stage/deadline")
    [ "$(date +%s)" -lt "$arm_deadline" ] || fail 'guard expired while arming'
    mv "$stage" "$guard"
    sync
    printf 'Confirmed-boot guard armed; deadline=%s\n' "$(cat "$guard/deadline")"
)

guard_restore() (
    [ -e "$guard" ] || [ -L "$guard" ] || return 0
    guard_check
    touch "$guard/restoring"; sync
    pending=$guard/old
    restore_keys
    if [ "$1" != boot ]; then restore_runtime; fi
    archive=$(mktemp -d "$state/confirmed-boot-restored-$(date +%s).XXXXXX")
    mv "$guard" "$archive/guard"
    sync
)

guard_claim_present() {
    [ -e "$guard/claim-started" ] || [ -L "$guard/claim-started" ] ||
        [ -e "$guard/boot-id" ] || [ -L "$guard/boot-id" ]
}

guard_claim_valid() (
    guard_private "$guard/claim-started"
    guard_private "$guard/boot-id"
    claim=$(cat "$guard/boot-id")
    guard_id_valid "$claim" || exit 1
    [ "$claim" != "$(cat "$guard/preboot-id")" ]
)

guard_boot() (
    [ -e "$guard" ] || [ -L "$guard" ] || return 0
    guard_check
    if [ "$(date +%s)" -ge "$(cat "$guard/deadline")" ] || [ -e "$guard/restoring" ]; then
        guard_restore boot; return
    fi
    if ! boot_id=$(guard_boot_id); then
        guard_restore boot; return
    fi
    if guard_claim_present; then
        if ! guard_claim_valid || [ "$(cat "$guard/boot-id")" != "$boot_id" ]; then
            guard_restore boot
        fi
        return
    fi
    [ "$boot_id" != "$(cat "$guard/preboot-id")" ] || fail 'guard requires a new kernel'
    touch "$guard/claim-started"; sync
    printf '%s\n' "$boot_id" > "$guard/boot-id.tmp"
    mv "$guard/boot-id.tmp" "$guard/boot-id"
    sync
)

guard_release() (
    guard_check
    release_deadline=$(cat "$guard/deadline")
    [ "$(date +%s)" -lt "$release_deadline" ] || fail 'guard expired; restore required'
    [ ! -e "$guard/restoring" ] || fail 'guard restoration has started'
    guard_claim_valid || fail 'guard boot claim is incomplete'
    boot_id=$(guard_boot_id) || fail 'kernel boot identity unavailable; restore required'
    [ "$boot_id" = "$(cat "$guard/boot-id")" ] || fail 'guard boot identity changed'
    archive=$(mktemp -d "$state/confirmed-boot-released-$(date +%s).XXXXXX")
    guard_new_primary
    # The final runtime query can outlast the lease while this lock is held.
    [ "$(date +%s)" -lt "$release_deadline" ] || fail 'guard expired during release checks'
    mv "$guard" "$archive/guard"
    sync
)

# A confirmed-boot guard owns an immutable OLD snapshot; no transaction,
# including retire, may arm or change the network beside it.
case "${1:-}" in
    stage|promote|retire|retire-recovery|confirm|rollback)
        [ ! -e "$guard" ] && [ ! -L "$guard" ] || fail 'confirmed-boot guard is active'
        ;;
esac

case "${1:-}" in
    stage|promote)
        action=$1; profile=${2:-shared}; seconds=${3:-900}
        if [ "$action" = promote ]; then check_primary_rotation; fi
        arm
        if [ "$action" = stage ]; then stage; else promote; fi
        echo 'Pending: fresh old/new connections and service checks required before confirm.'
        ;;
    retire)
        # Always the isolated topology: arm() then also requires igc1-approved.
        action=retire; profile=isolated; seconds=${2:-900}
        retire_preflight
        arm
        retire
        echo 'Pending: fresh new/recovery connections and service checks required before confirm.'
        ;;
    retire-recovery)
        # Always the isolated topology: arm() then also requires igc1-approved.
        action=retire-recovery; profile=isolated; seconds=${2:-900}
        retire_recovery_preflight
        arm
        retire_recovery
        echo 'Pending: fresh lab-address connections and service checks required before confirm.'
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
    boot) rollback boot; guard_boot;;
    watchdog)
        if [ -d "$pending" ] && [ "$(date +%s)" -ge "$(cat "$pending/deadline")" ]; then
            rollback runtime
        fi
        if [ -d "$guard" ]; then
            guard_check
            if [ "$(date +%s)" -ge "$(cat "$guard/deadline")" ] || [ -e "$guard/restoring" ] ||
                ! boot_id=$(guard_boot_id) ||
                { guard_claim_present && { ! guard_claim_valid || [ "$(cat "$guard/boot-id")" != "$boot_id" ]; }; }; then
                guard_restore runtime
            fi
        fi
        ;;
    confirmed-boot-hash) guard_hash "$(guard_source "${2:-}")";;
    arm-confirmed-boot) guard_arm "${2:-}" "${3:-}" "${4:-600}";;
    restore-confirmed-boot) guard_restore runtime;;
    release-confirmed-boot) guard_release;;
    status)
        if [ -d "$guard" ]; then
            guard_check
            echo "confirmed-boot guard deadline=$(cat "$guard/deadline")"
            exit 0
        fi
        if [ -d "$pending" ]; then
            echo "pending profile=$(cat "$pending/profile") deadline=$(cat "$pending/deadline")"
        else echo 'no pending transaction'; fi
        ;;
    *) fail 'usage: stage|promote [shared|isolated] [seconds], retire [seconds], retire-recovery [seconds], confirm, rollback, watchdog, boot, status;
confirmed-boot-hash ARCHIVE; arm-confirmed-boot ARCHIVE SHA256 [seconds];
restore-confirmed-boot; release-confirmed-boot';;
esac
