#!/usr/bin/env bash
set -euo pipefail
umask 077
export PATH=/usr/sbin:/usr/bin:/sbin:/bin
readonly state=/var/lib/gonf-network-migration
readonly pending=$state/pending
readonly helper=/usr/local/sbin/gonf-network-migration
readonly endpoint=/usr/local/sbin/gonf-endpoint-rollback
readonly resolver_dir=/home/paul/pihole
readonly resolver_candidate=/var/lib/f3s-pihole-migration/docker-compose.yml
# NetworkManager global DNS (rocky_resolver_conf); retire requires it.
readonly dns_dropin=/etc/NetworkManager/conf.d/90-gonf-dns.conf
[[ $EUID == 0 ]] || exit 1
# Root-owned recipe output; no operator-supplied shell expressions.
# shellcheck disable=SC1091
source /etc/gonf-network-migration.conf
: "${iface:?}" "${old:?}" "${lab:?}" "${recovery:?}"
: "${shared_gateway:?}"
# Existing r-node configurations predate these explicit role/policy fields.
role=${role:-k3s}
ipv6_policy=${ipv6_policy:-ula}
case "$role:$ipv6_policy" in
    k3s:ula|resolver:ula) : "${ula:?}";;
    standalone:disabled) [[ -z ${ula:-} ]] || exit 1;;
    *) printf 'Unreviewed role or physical IPv6 policy\n' >&2; exit 1;;
esac
mkdir -p "$state"
exec 9>"$state/lock"
flock -x 9

_fail() { printf '%s\n' "$*" >&2; exit 1; }
# Match on captured output, never "cmd | grep -q": with pipefail, grep -q can
# exit at the first match and SIGPIPE the producer, failing a true check.
_v4_has() {
    local rows
    rows=$(ip -4 -o address show dev "$iface")
    [[ $rows == *" $1/24 "* ]]
}
_gate() {
    local file=$state/$1
    [[ -f $file && ! -L $file ]] || _fail "Missing explicit gate: $1"
    [[ $(stat -c '%u:%a' "$file") == 0:600 ]] ||
        _fail "Gate must be root-owned mode 0600: $1"
}
_k3s_maintenance() {
    local gate=$state/k3s-maintenance-approved
    [[ -e $gate || -L $gate ]] || return 1
    _gate k3s-maintenance-approved
    [[ $(<"$gate") == "$(hostname -s)" ]] ||
        _fail 'Maintenance gate names another node'
    [[ $(systemctl show k3s --property=ActiveState --value) == inactive ]] ||
        _fail 'Maintenance requires k3s to be stopped'
}
_endpoint() {
    [[ -f $pending/endpoints-required ]] || return 0
    [[ -x $endpoint ]] || _fail 'Required endpoint rollback helper missing'
    "$endpoint" "$1" "$pending/endpoints"
}
_archive() {
    local outcome=$1
    mv "$pending" "$state/$outcome-$(date +%s)-$$"
}
_refresh_wg_source_cache() {
    local mark current
    # A same-value fwmark update clears WireGuard's cached local UDP source.
    # Before wg-quick at boot there is no interface/cache to refresh.
    [[ -e /sys/class/net/wg0 ]] || return 0
    mark=$(wg show wg0 fwmark)
    [[ $mark == off || $mark =~ ^0x[0-9a-fA-F]{1,8}$ ]] ||
        _fail 'Unrecognized WireGuard fwmark'
    wg set wg0 fwmark "$mark"
    current=$(wg show wg0 fwmark)
    [[ $current == "$mark" ]] || _fail 'WireGuard fwmark changed'
}
_restore_persistent() {
    local profile
    profile=$(<"$pending/profile-path")
    [[ $profile == /etc/NetworkManager/system-connections/* ]] ||
        _fail 'Unsupported saved profile path'
    cp -a "$pending/profile" "$profile"
    restorecon "$profile"
    if [[ -d $pending/resolver ]]; then
        cp -a "$pending/resolver/compose" "$resolver_dir/docker-compose.yml"
        cp -a "$pending/resolver/pihole.toml" "$resolver_dir/etc-pihole/pihole.toml"
        restorecon "$resolver_dir/docker-compose.yml" "$resolver_dir/etc-pihole/pihole.toml"
    fi
    if [[ ! -f $pending/standalone ]]; then
        if [[ -f $pending/flannel-existed ]]; then
            cp -a "$pending/flannel" /etc/rancher/k3s/config.yaml.d/60-network-migration.yaml
        else
            rm -f /etc/rancher/k3s/config.yaml.d/60-network-migration.yaml
        fi
    fi
}
_restore_runtime() {
    local uuid
    uuid=$(<"$pending/uuid")
    nmcli connection reload
    nmcli --wait 60 connection up uuid "$uuid" ifname "$iface"
    # Scope flush/restore to the physical interface, never wg0/flannel/lo.
    ip -4 address flush dev "$iface" scope global
    ip -6 address flush dev "$iface" scope global
    ip -4 address restore < "$pending/address4"
    ip -6 address restore < "$pending/address6"
    ip -4 route flush dev "$iface"
    ip -6 route flush dev "$iface"
    ip -4 route restore < "$pending/route4"
    ip -6 route restore < "$pending/route6"
    sysctl -q -w "net.ipv6.conf.$iface.accept_ra=$(<"$pending/accept-ra")"
    _endpoint restore-runtime
    _refresh_wg_source_cache
    if [[ -d $pending/resolver ]]; then
        # FTL writes TOML during shutdown. Restore once more after it stops so
        # an old container cannot overwrite the saved configuration on exit.
        (cd "$resolver_dir" && docker compose stop --timeout 10)
        cp -a "$pending/resolver/pihole.toml" "$resolver_dir/etc-pihole/pihole.toml"
        restorecon "$resolver_dir/etc-pihole/pihole.toml"
        (cd "$resolver_dir" && docker compose up -d --timeout 10 --wait --wait-timeout 90)
    fi
}
_rollback() {
    [[ -d $pending ]] || return 0
    _restore_persistent
    _endpoint restore-boot
    _restore_runtime
    if [[ -f $pending/k3s-restarted ]]; then
        if ! _k3s_maintenance; then systemctl restart k3s; fi
    fi
    _archive rolled-back
}
# _snapshot REQUIRED: REQUIRED is the address the transaction depends on
# (the retained old IPv4 for stage/promote/retire, the recovery address for
# retire-recovery, which runs after the old address is gone).
_snapshot() {
    local required=$1 uuid profile
    uuid=$(nmcli -g GENERAL.CON-UUID device show "$iface")
    [[ $uuid =~ ^[a-fA-F0-9-]{36}$ ]] || _fail 'No physical NM connection'
    local -a profiles=()
    while IFS= read -r -d '' profile; do
        if grep -Fxq "uuid=$uuid" "$profile"; then profiles+=("$profile"); fi
    done < <(find /etc/NetworkManager/system-connections -maxdepth 1 -type f -print0)
    (( ${#profiles[@]} == 1 )) || _fail 'Requires one persistent NM keyfile'
    profile=${profiles[0]}
    _v4_has "$required" ||
        _fail "Required IPv4 /24 $required is absent"
    [[ ! -d $pending ]] || _fail 'Transaction already pending'
    mkdir "$state/snapshot"
    # A partial snapshot is never treated as a complete recovery journal.
    cp -a "$profile" "$state/snapshot/profile"
    if [[ $role == resolver && -f $resolver_candidate ]]; then
        mkdir "$state/snapshot/resolver"
        cp -a "$resolver_dir/docker-compose.yml" "$state/snapshot/resolver/compose"
        cp -a "$resolver_dir/etc-pihole/pihole.toml" "$state/snapshot/resolver/pihole.toml"
    fi
    printf '%s\n' "$profile" > "$state/snapshot/profile-path"
    printf '%s\n' "$uuid" > "$state/snapshot/uuid"
    ip -4 address save dev "$iface" scope global > "$state/snapshot/address4"
    ip -6 address save dev "$iface" scope global > "$state/snapshot/address6"
    ip -4 route save table main dev "$iface" > "$state/snapshot/route4"
    ip -6 route save table main dev "$iface" > "$state/snapshot/route6"
    sysctl -n "net.ipv6.conf.$iface.accept_ra" > "$state/snapshot/accept-ra"
    if [[ $role != k3s ]]; then
        touch "$state/snapshot/standalone"
    elif [[ -f /etc/rancher/k3s/config.yaml.d/60-network-migration.yaml ]]; then
        cp -a /etc/rancher/k3s/config.yaml.d/60-network-migration.yaml "$state/snapshot/flannel"
        touch "$state/snapshot/flannel-existed"
    fi
    printf '%s\n' "$(( $(date +%s) + 900 ))" > "$state/snapshot/deadline"
    mv "$state/snapshot" "$pending"
    if [[ -f $state/endpoints-required ]]; then
        touch "$pending/endpoints-required"
        _endpoint snapshot
    fi
}
_return_routes() {
    local uuid=$1 return_gateway=$2 current route destination result=''
    current=$(nmcli --escape no -g ipv4.routes connection show uuid "$uuid")
    local -a routes=()
    IFS=',' read -r -a routes <<< "$current"
    for route in "${routes[@]}"; do
        route="${route#"${route%%[![:space:]]*}"}"
        [[ -n $route ]] || continue
        destination=${route%%[[:space:]]*}
        [[ $destination =~ ^[0-9.]+/[0-9]+$ ]] ||
            _fail 'Unrecognized NM route serialization; refusing route replacement'
        # Own this exact /32 only; keep every unrelated route and attribute.
        [[ $destination == 10.99.0.2/32 ]] && continue
        result+="$route,"
    done
    printf '%s10.99.0.2/32 %s' "$result" "$return_gateway"
}
_activate() {
    local action=$1 topology=$2 uuid addresses gateway dns return_routes
    _gate activation-approved
    _gate "$action-approved"
    if [[ $topology == isolated ]]; then
        _gate igc1-approved
    elif [[ $topology != shared ]]; then
        _fail 'Topology must be shared or isolated'
    fi
    systemctl is-enabled --quiet gonf-network-rollback-boot.service
    systemctl is-enabled --quiet gonf-network-rollback-runtime.service
    systemctl is-active --quiet gonf-network-rollback.timer
    [[ $action == stage || $action == promote ]] || _fail 'Invalid action'
    if [[ $action == promote && $role == k3s ]]; then
        # Serial authorization must name precisely this node, never a cluster.
        _gate serial-k3s-approved
        [[ $(<"$state/serial-k3s-approved") == "$(hostname -s)" ]] ||
            _fail 'Serial gate names another node'
        _v4_has "$lab" ||
            _fail 'Stage and independently accept the lab address first'
        if ! _k3s_maintenance; then systemctl is-active --quiet k3s; fi
    fi
    if [[ $action == promote ]]; then
        _v4_has "$lab" ||
            _fail 'Stage and independently accept the lab address first'
    fi
    _snapshot "$old"
    uuid=$(<"$pending/uuid")
    addresses="$old/24,$lab/24"
    gateway=$(nmcli -g IP4.GATEWAY device show "$iface")
    [[ -n $gateway && $gateway != *$'\n'* ]] || _fail 'Requires one IPv4 gateway'
    if [[ $action == promote ]]; then
        addresses="$lab/24,$old/24"
        gateway=10.20.0.1
    fi
    # NM can serialize multiple runtime DNS servers on one "a | b" line.
    # Normalize that form as well as one-server-per-line before writing a list.
    dns=$(nmcli -g IP4.DNS device show "$iface" |
        sed 's/[[:space:]]*|[[:space:]]*/,/g' | paste -sd ,)
    local return_gateway=$shared_gateway
    if [[ $topology == isolated ]]; then
        addresses+=",$recovery/24"
        return_gateway=10.98.0.1
    fi
    return_routes=$(_return_routes "$uuid" "$return_gateway")
    # No network mutation occurs until the complete private journal exists and
    # PID1 watchdog/boot recovery are armed. Keep existing DNS and other routes.
    local -a ipv6_options=(ipv6.method disabled)
    if [[ $ipv6_policy == ula ]]; then
        ipv6_options=(ipv6.method manual ipv6.addresses "$ula/64"
            ipv6.gateway '' ipv6.routes '' ipv6.never-default yes
            ipv6.ignore-auto-routes yes ipv6.ignore-auto-dns yes ipv6.dns '')
    fi
    nmcli connection modify uuid "$uuid" ipv4.method manual \
        ipv4.addresses "$addresses" ipv4.gateway "$gateway" \
        ipv4.dns "$dns" ipv4.ignore-auto-dns yes \
        ipv4.routes "$return_routes" \
        "${ipv6_options[@]}"
    nmcli --wait 60 connection up uuid "$uuid" ifname "$iface"
    sysctl -q -w "net.ipv6.conf.$iface.accept_ra=0"
    # Also remove stale externally-added physical addresses/RA routes; NM
    # profile conversion alone may retain addresses it does not own.
    ip -6 address flush dev "$iface" scope global
    ip -6 route flush default dev "$iface"
    ip -6 route flush dev "$iface" proto ra
    if [[ $ipv6_policy == ula ]]; then
        ip -6 address add "$ula/64" dev "$iface"
    fi
    if [[ $action == promote ]]; then
        _refresh_wg_source_cache
    fi
    if [[ -d $pending/resolver ]]; then
        install -o paul -g paul -m 644 "$resolver_candidate" "$resolver_dir/docker-compose.yml"
        restorecon "$resolver_dir/docker-compose.yml"
        (cd "$resolver_dir" && docker compose up -d --timeout 10 --wait --wait-timeout 90)
    fi
    if [[ $action == promote && $role == k3s ]]; then
        mkdir -p /etc/rancher/k3s/config.yaml.d
        printf 'flannel-iface: %s\n' "$iface" > /etc/rancher/k3s/config.yaml.d/60-network-migration.yaml
        if ! _k3s_maintenance; then
            # Mark before restart so partial starts get reconciled on undo.
            touch "$pending/k3s-restarted"
            systemctl restart k3s
        fi
    fi
    # Single-use approval: subsequent nodes/transactions need fresh evidence.
    rm -f "$state/$action-approved" "$state/serial-k3s-approved"
}

# The runtime gateway/route checks are not enough: the profile is what the
# next activation or boot applies, and without the old address a saved route
# through the old /24 has no connected next hop. Require the promoted saved
# gateway and laptop return route, and refuse any saved route that uses the
# old /24 as destination or next hop.
_retire_saved_routes() {
    local uuid=$1 route destination hop found='' prefix=${old%.*}.
    [[ $(nmcli -g ipv4.gateway connection show uuid "$uuid") == 10.20.0.1 ]] ||
        _fail 'Retire requires the saved 10.20.0.1 gateway'
    local -a routes=()
    IFS=',' read -r -a routes <<< \
        "$(nmcli --escape no -g ipv4.routes connection show uuid "$uuid")"
    for route in "${routes[@]}"; do
        destination='' hop=''
        read -r destination hop _ <<< "$route" || :
        [[ -n $destination ]] || continue
        if [[ $destination == "$prefix"* || $hop == "$prefix"* ]]; then
            _fail "Saved route $destination uses the old /24"
        fi
        if [[ $destination == 10.99.0.2/32 && $hop == 10.98.0.1 ]]; then found=yes; fi
    done
    [[ -n $found ]] || _fail 'Retire requires the saved 10.99.0.2/32 route via 10.98.0.1'
}

# Retire (i24) removes only the retained old IPv4 after the isolated promote
# was accepted: lab primary, gateway 10.20.0.1 and the 10.98.0.1 laptop return
# route must already be in place, so the old address carries nothing the
# transaction still needs. It uses the same private journal, PID1 watchdog and
# boot restore as stage/promote; rollback brings the old address back. The
# old /24 also leaves the profile's per-connection DNS list, which is safe
# only while NetworkManager's global DNS drop-in (rocky_resolver_conf) owns
# resolv.conf, so retire refuses without it. No NM reactivation: the runtime
# address is deleted directly, so lab/recovery/wg0 traffic never bounces.
#
# No igc1-approved gate, unlike stage/promote isolated: that gate authorizes
# moving to the isolated cabling. Retire does not change the topology; it
# requires the isolated topology to be in place already (recovery address,
# runtime and saved 10.98.0.1 return route), and its own retire-approved gate
# is the operator's authorization for this transaction.
_retire() {
    local uuid current kept='' dns='' entry found='' prefix=${old%.*}.
    _gate activation-approved
    _gate retire-approved
    systemctl is-enabled --quiet gonf-network-rollback-boot.service
    systemctl is-enabled --quiet gonf-network-rollback-runtime.service
    systemctl is-active --quiet gonf-network-rollback.timer
    if [[ $role == k3s ]]; then
        # One etcd member at a time, as for promote.
        _gate serial-k3s-approved
        [[ $(<"$state/serial-k3s-approved") == "$(hostname -s)" ]] ||
            _fail 'Serial gate names another node'
    fi
    _v4_has "$lab" ||
        _fail 'Retire requires the accepted lab address'
    _v4_has "$recovery" ||
        _fail 'Retire requires the accepted recovery address'
    [[ $(nmcli -g IP4.GATEWAY device show "$iface") == 10.20.0.1 ]] ||
        _fail 'Retire requires the promoted 10.20.0.1 gateway'
    [[ $(ip -4 route show 10.99.0.2/32 dev "$iface") == *'via 10.98.0.1'* ]] ||
        _fail 'Retire requires the isolated 10.98.0.1 return route'
    [[ -f $dns_dropin ]] ||
        _fail 'Retire requires the global DNS drop-in (rocky_resolver_conf)'
    # Compute and validate the persistent candidate before arming, so a
    # refused retire leaves no journal behind. _snapshot re-reads the same
    # device UUID and fails on any other mismatch (old address absent etc.).
    uuid=$(nmcli -g GENERAL.CON-UUID device show "$iface")
    _retire_saved_routes "$uuid"
    current=$(nmcli --escape no -g ipv4.addresses connection show uuid "$uuid")
    local -a addresses=() servers=()
    IFS=',' read -r -a addresses <<< "$current"
    for entry in "${addresses[@]}"; do
        entry="${entry//[[:space:]]/}"
        if [[ $entry == "$old/24" ]]; then
            found=yes
        elif [[ -n $entry ]]; then
            kept+="$entry,"
        fi
    done
    kept=${kept%,}
    [[ -n $found ]] || _fail 'Persistent profile lacks the old address'
    [[ ,$kept, == *",$lab/24,"* && ,$kept, == *",$recovery/24,"* ]] ||
        _fail 'Persistent profile lacks lab or recovery address'
    IFS=',' read -r -a servers <<< \
        "$(nmcli --escape no -g ipv4.dns connection show uuid "$uuid" | tr ';' ',')"
    for entry in "${servers[@]}"; do
        entry="${entry//[[:space:]]/}"
        if [[ -n $entry && $entry != "$prefix"* ]]; then dns+="$entry,"; fi
    done
    _snapshot "$old"
    [[ $(<"$pending/uuid") == "$uuid" ]] || _fail 'Connection changed while arming'
    nmcli connection modify uuid "$uuid" ipv4.addresses "$kept" ipv4.dns "${dns%,}"
    ip -4 address del "$old/24" dev "$iface"
    _refresh_wg_source_cache
    # Single-use approval, as for stage/promote.
    rm -f "$state/retire-approved" "$state/serial-k3s-approved"
}

# Retire-recovery (j24) removes the dedicated igc1 recovery path after i24:
# the recovery address and the laptop's 10.99.0.2/32 return route via
# 10.98.0.1. Replies to the laptop then follow the 10.20.0.1 default, the
# router's normal wg2 path, so the router can drop 10.98.0.1 afterwards.
# Same journal, PID1 watchdog and boot restore as retire; rollback brings
# the address and route back. Refuses unless the old address is already
# retired (runtime and profile), the lab address, the 10.20.0.1 gateway
# (runtime and saved) and exactly the expected recovery route are in place,
# and no other saved route uses the recovery /24. No NM reactivation: the
# runtime route and address are deleted directly, so lab/wg0 never bounce.
_retire_recovery() {
    local uuid route destination hop kept_routes='' kept='' entry found=''
    _gate activation-approved
    _gate retire-recovery-approved
    systemctl is-enabled --quiet gonf-network-rollback-boot.service
    systemctl is-enabled --quiet gonf-network-rollback-runtime.service
    systemctl is-active --quiet gonf-network-rollback.timer
    if [[ $role == k3s ]]; then
        # One etcd member at a time, as for promote and retire.
        _gate serial-k3s-approved
        [[ $(<"$state/serial-k3s-approved") == "$(hostname -s)" ]] ||
            _fail 'Serial gate names another node'
    fi
    ! _v4_has "$old" || _fail 'Retire the old address (i24) first'
    _v4_has "$lab" || _fail 'Retire-recovery requires the accepted lab address'
    _v4_has "$recovery" || _fail 'Recovery address already absent'
    [[ $(nmcli -g IP4.GATEWAY device show "$iface") == 10.20.0.1 ]] ||
        _fail 'Retire-recovery requires the 10.20.0.1 gateway'
    [[ $(ip -4 route show 10.99.0.2/32 dev "$iface") == *'via 10.98.0.1'* ]] ||
        _fail 'Retire-recovery requires the 10.98.0.1 return route'
    [[ -f $dns_dropin ]] ||
        _fail 'Retire-recovery requires the global DNS drop-in (rocky_resolver_conf)'
    uuid=$(nmcli -g GENERAL.CON-UUID device show "$iface")
    [[ $(nmcli -g ipv4.gateway connection show uuid "$uuid") == 10.20.0.1 ]] ||
        _fail 'Retire-recovery requires the saved 10.20.0.1 gateway'
    local -a routes=() addresses=()
    IFS=',' read -r -a routes <<< \
        "$(nmcli --escape no -g ipv4.routes connection show uuid "$uuid")"
    for route in "${routes[@]}"; do
        route="${route#"${route%%[![:space:]]*}"}"
        destination='' hop=''
        read -r destination hop _ <<< "$route" || :
        [[ -n $destination ]] || continue
        [[ $destination =~ ^[0-9.]+/[0-9]+$ ]] ||
            _fail 'Unrecognized NM route serialization; refusing route replacement'
        if [[ $destination == 10.99.0.2/32 && $hop == 10.98.0.1 ]]; then
            found=yes
            continue
        fi
        if [[ $destination == 10.98.* || $hop == 10.98.* ]]; then
            _fail "Saved route $destination uses the recovery /24"
        fi
        kept_routes+="$route,"
    done
    [[ -n $found ]] || _fail 'Profile lacks the saved 10.99.0.2/32 route via 10.98.0.1'
    found=''
    IFS=',' read -r -a addresses <<< \
        "$(nmcli --escape no -g ipv4.addresses connection show uuid "$uuid")"
    for entry in "${addresses[@]}"; do
        entry="${entry//[[:space:]]/}"
        [[ $entry != "$old/24" ]] || _fail 'Persistent profile still has the old address'
        if [[ $entry == "$recovery/24" ]]; then
            found=yes
        elif [[ -n $entry ]]; then
            kept+="$entry,"
        fi
    done
    [[ -n $found ]] || _fail 'Persistent profile lacks the recovery address'
    [[ ,$kept, == *",$lab/24,"* ]] || _fail 'Persistent profile lacks the lab address'
    _snapshot "$recovery"
    [[ $(<"$pending/uuid") == "$uuid" ]] || _fail 'Connection changed while arming'
    nmcli connection modify uuid "$uuid" ipv4.addresses "${kept%,}" \
        ipv4.routes "${kept_routes%,}"
    ip -4 route del 10.99.0.2/32 dev "$iface"
    ip -4 address del "$recovery/24" dev "$iface"
    _refresh_wg_source_cache
    # Single-use approval, as for every transaction.
    rm -f "$state/retire-recovery-approved" "$state/serial-k3s-approved"
}

case ${1:-} in
    init)
        [[ ! -d $pending ]] || _fail 'Pending transaction'
        systemctl daemon-reload
        systemctl enable gonf-network-rollback-boot.service \
            gonf-network-rollback-runtime.service gonf-network-rollback.timer
        systemctl start gonf-network-rollback.timer
        ;;
    request)
        shift
        [[ ${1:-} == stage || ${1:-} == promote || ${1:-} == retire ||
            ${1:-} == retire-recovery || ${1:-} == rollback ]] ||
            _fail 'request stage|promote shared|isolated, request retire|retire-recovery, or request rollback'
        # PID1 starts the worker after this process releases its flock. A lost
        # SSH session cannot interrupt activation or its independent watchdog.
        systemd-run --no-block --unit="gonf-network-change-$(date +%s)-$$" \
            --property=Type=oneshot --property=TimeoutStartSec=180 \
            "$helper" "${@}"
        ;;
    stage|promote) _activate "$1" "${2:-}" ;;
    retire) _retire ;;
    retire-recovery) _retire_recovery ;;
    rollback) _rollback ;;
    watchdog)
        if [[ -d $pending ]] && (( $(date +%s) >= $(<"$pending/deadline") )); then
            _rollback
        fi
        ;;
    restore-boot)
        if [[ -d $pending ]]; then
            _restore_persistent
            _endpoint restore-boot
            touch "$pending/boot-restored"
        fi
        ;;
    restore-after-boot)
        if [[ -f $pending/boot-restored ]]; then
            _restore_runtime
            _archive boot-rolled-back
        fi
        ;;
    confirm)
        [[ -d $pending ]] || _fail 'No pending transaction'
        (( $(date +%s) < $(<"$pending/deadline") )) || _fail 'Expired; rollback required'
        _gate confirm-approved
        _endpoint confirm
        _archive confirmed
        rm -f "$state/confirm-approved"
        ;;
    status)
        if [[ -d $pending ]]; then
            printf 'Pending rollback deadline: %s\n' "$(<"$pending/deadline")"
        else
            printf 'No pending transaction\n'
        fi
        ;;
    *) _fail 'Use init|request|retire|retire-recovery|watchdog|restore-boot|restore-after-boot|confirm|status' ;;
esac
