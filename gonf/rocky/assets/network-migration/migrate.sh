#!/usr/bin/env bash
set -euo pipefail
umask 077
export PATH=/usr/sbin:/usr/bin:/sbin:/bin
readonly state=/var/lib/gonf-network-migration
readonly pending=$state/pending
readonly helper=/usr/local/sbin/gonf-network-migration
readonly endpoint=/usr/local/sbin/gonf-endpoint-rollback
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
    k3s:ula) : "${ula:?}";;
    standalone:disabled) [[ -z ${ula:-} ]] || exit 1;;
    *) printf 'Unreviewed role or physical IPv6 policy\n' >&2; exit 1;;
esac
mkdir -p "$state"
exec 9>"$state/lock"
flock -x 9

_fail() { printf '%s\n' "$*" >&2; exit 1; }
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
_snapshot() {
    local uuid profile
    uuid=$(nmcli -g GENERAL.CON-UUID device show "$iface")
    [[ $uuid =~ ^[a-fA-F0-9-]{36}$ ]] || _fail 'No physical NM connection'
    local -a profiles=()
    while IFS= read -r -d '' profile; do
        if grep -Fxq "uuid=$uuid" "$profile"; then profiles+=("$profile"); fi
    done < <(find /etc/NetworkManager/system-connections -maxdepth 1 -type f -print0)
    (( ${#profiles[@]} == 1 )) || _fail 'Requires one persistent NM keyfile'
    profile=${profiles[0]}
    ip -4 -o address show dev "$iface" | grep -Fq " $old/24 " ||
        _fail 'Old IPv4 /24 is absent'
    [[ ! -d $pending ]] || _fail 'Transaction already pending'
    mkdir "$state/snapshot"
    # A partial snapshot is never treated as a complete recovery journal.
    cp -a "$profile" "$state/snapshot/profile"
    printf '%s\n' "$profile" > "$state/snapshot/profile-path"
    printf '%s\n' "$uuid" > "$state/snapshot/uuid"
    ip -4 address save dev "$iface" scope global > "$state/snapshot/address4"
    ip -6 address save dev "$iface" scope global > "$state/snapshot/address6"
    ip -4 route save table main dev "$iface" > "$state/snapshot/route4"
    ip -6 route save table main dev "$iface" > "$state/snapshot/route6"
    sysctl -n "net.ipv6.conf.$iface.accept_ra" > "$state/snapshot/accept-ra"
    if [[ $role == standalone ]]; then
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
        ip -4 -o address show dev "$iface" | grep -Fq " $lab/24 " ||
            _fail 'Stage and independently accept the lab address first'
        if ! _k3s_maintenance; then systemctl is-active --quiet k3s; fi
    fi
    if [[ $action == promote ]]; then
        ip -4 -o address show dev "$iface" | grep -Fq " $lab/24 " ||
            _fail 'Stage and independently accept the lab address first'
    fi
    _snapshot
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
        [[ ${1:-} == stage || ${1:-} == promote || ${1:-} == rollback ]] ||
            _fail 'request stage|promote shared|isolated, or request rollback'
        # PID1 starts the worker after this process releases its flock. A lost
        # SSH session cannot interrupt activation or its independent watchdog.
        systemd-run --no-block --unit="gonf-network-change-$(date +%s)-$$" \
            --property=Type=oneshot --property=TimeoutStartSec=180 \
            "$helper" "${@}"
        ;;
    stage|promote) _activate "$1" "${2:-}" ;;
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
    *) _fail 'Use init|request|watchdog|restore-boot|restore-after-boot|confirm|status' ;;
esac
