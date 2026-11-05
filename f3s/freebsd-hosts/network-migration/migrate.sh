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
        v4_rows | awk '{print $1}' | grep -qxF "$recovery" || fail 'stage isolated recovery alias first'
    fi
    primary=$(cat "$pending/primary")
    if [ "$primary" != "$lab" ]; then
        ifconfig "$iface" inet "$lab" delete
        ifconfig "$iface" inet "$lab/24"
        ifconfig "$iface" inet "$old/24" alias
    fi
    set_return_route
    sysrc -f /etc/rc.conf "ifconfig_${iface}=inet $lab/24" "ifconfig_${iface}_alias200=inet $old/24" >/dev/null
    sysrc -f /etc/rc.conf -x "ifconfig_${iface}_alias201" >/dev/null
    sysrc -f /etc/rc.conf defaultrouter=10.20.0.1 >/dev/null
    set_default4 10.20.0.1
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

guard_epoch() {
    guard_clock=$(sysctl -n kern.boottime)
    guard_clock=$(printf '%s\n' "$guard_clock" | awk -F '[=,]' '{gsub(/ /,"",$2); print $2}')
    case $guard_clock in ''|*[!0-9]*) fail 'invalid kernel epoch';; esac
    printf '%s\n' "$guard_clock"
}

guard_new_primary() {
    physical_rows=$(ifconfig "$iface" inet)
    [ "$(printf '%s\n' "$physical_rows" | awk '$1=="inet" {print $2; exit}')" = "$lab" ] || fail 'NEW primary required'
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
    for field in ready source hash deadline preboot config-hash helper-hash; do
        guard_private "$checked/$field"
    done
    [ ! -d "$state/pending" ] || fail 'ordinary pending overlaps confirmed guard'
    source_dir=$(guard_source "$(cat "$checked/source")")
    [ "$(guard_hash "$source_dir")" = "$(cat "$checked/hash")" ] || fail 'accepted OLD snapshot changed'
    [ "$(guard_hash "$checked/old")" = "$(cat "$checked/hash")" ] || fail 'guard working snapshot changed'
    [ "$(sha256 -q /usr/local/etc/gonf-network-migration.conf)" = "$(cat "$checked/config-hash")" ] || fail 'guard config changed'
    [ "$(sha256 -q "$helper")" = "$(cat "$checked/helper-hash")" ] || fail 'guard helper changed'
    for field in deadline preboot; do
        value=$(cat "$checked/$field")
        case $value in ''|*[!0-9]*) fail 'invalid guard epoch';; esac
    done
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
    epoch=$(guard_epoch)
    stage=$(mktemp -d "$state/confirmed-boot-preparing.XXXXXX")
    cp -Rp "$source_dir" "$stage/old"
    printf '%s\n' "$1" > "$stage/source"
    printf '%s\n' "$2" > "$stage/hash"
    printf '%s\n' "$epoch" > "$stage/preboot"
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
        [ -e "$guard/boot-epoch" ] || [ -L "$guard/boot-epoch" ]
}

guard_claim_valid() (
    guard_private "$guard/claim-started"
    guard_private "$guard/boot-epoch"
    claim=$(cat "$guard/boot-epoch")
    case $claim in ''|*[!0-9]*) exit 1;; esac
    [ "$claim" != "$(cat "$guard/preboot")" ]
)

guard_boot() (
    [ -e "$guard" ] || [ -L "$guard" ] || return 0
    guard_check
    epoch=$(guard_epoch)
    if [ "$(date +%s)" -ge "$(cat "$guard/deadline")" ] || [ -e "$guard/restoring" ]; then
        guard_restore boot; return
    fi
    if guard_claim_present; then
        if ! guard_claim_valid || [ "$(cat "$guard/boot-epoch")" != "$epoch" ]; then
            guard_restore boot
        fi
        return
    fi
    [ "$epoch" != "$(cat "$guard/preboot")" ] || fail 'guard requires a new kernel'
    touch "$guard/claim-started"; sync
    printf '%s\n' "$epoch" > "$guard/boot-epoch.tmp"
    mv "$guard/boot-epoch.tmp" "$guard/boot-epoch"
    sync
)

guard_release() (
    guard_check
    release_deadline=$(cat "$guard/deadline")
    [ "$(date +%s)" -lt "$release_deadline" ] || fail 'guard expired; restore required'
    [ ! -e "$guard/restoring" ] || fail 'guard restoration has started'
    guard_claim_valid || fail 'guard boot claim is incomplete'
    [ "$(guard_epoch)" = "$(cat "$guard/boot-epoch")" ] || fail 'guard boot identity changed'
    archive=$(mktemp -d "$state/confirmed-boot-released-$(date +%s).XXXXXX")
    guard_new_primary
    # The final runtime query can outlast the lease while this lock is held.
    [ "$(date +%s)" -lt "$release_deadline" ] || fail 'guard expired during release checks'
    mv "$guard" "$archive/guard"
    sync
)

case "${1:-}" in
    stage|promote|confirm|rollback)
        [ ! -e "$guard" ] && [ ! -L "$guard" ] || fail 'confirmed-boot guard is active'
        ;;
esac

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
    boot) rollback boot; guard_boot;;
    watchdog)
        if [ -d "$pending" ] && [ "$(date +%s)" -ge "$(cat "$pending/deadline")" ]; then
            rollback runtime
        fi
        if [ -d "$guard" ]; then
            guard_check
            if [ "$(date +%s)" -ge "$(cat "$guard/deadline")" ] || [ -e "$guard/restoring" ] ||
                { guard_claim_present && { ! guard_claim_valid || [ "$(cat "$guard/boot-epoch")" != "$(guard_epoch)" ]; }; }; then
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
    *) fail 'usage: stage|promote [shared|isolated] [seconds], confirm, rollback, watchdog, boot, status;
confirmed-boot-hash ARCHIVE; arm-confirmed-boot ARCHIVE SHA256 [seconds];
restore-confirmed-boot; release-confirmed-boot';;
esac
