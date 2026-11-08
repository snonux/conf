#!/bin/sh
# One AP-local transaction; procd owns runtime polling and early boot restore.
set -eu
umask 077
set -f
PATH=/usr/sbin:/usr/bin:/sbin:/bin
export PATH

state=/etc/f3s-ap-recovery
configs=/etc/config
boot_id=/proc/sys/kernel/random/boot_id
uptime_file=/proc/uptime
service=/etc/init.d/f3s_ap_recovery
boot_link=/etc/rc.d/S18f3s_ap_recovery
files='network dhcp firewall'
current=$state/current

fail() { printf '%s\n' "$*" >&2; exit 1; }
now() {
    awk 'NR==1 && $1 ~ /^[0-9]+([.][0-9]+)?$/ { print int($1); valid=1; exit } END { if (!valid) exit 1 }' "$uptime_file"
}
private_directory() {
    test -d "$1" && test ! -L "$1" || return 1
    # This AP has no stat command/applet. Read only ls's metadata fields;
    # ignore filenames and subsequent lines, including embedded newlines.
    # shellcheck disable=SC2012
    LC_ALL=C ls -ldn "$1" | awk '
        NR == 1 { valid = ($1 == "drwx------" && $3 == "0") }
        END { exit !valid }'
}
phase() { printf '%s\n' "$1" > "$current/phase.tmp"; mv "$current/phase.tmp" "$current/phase"; }
pending() { test -f "$current/pending"; }
validate_token() {
    case $1 in ''|*[!a-zA-Z0-9_-]*) fail 'invalid token';; esac
    test "${#1}" -ge 8 && test "${#1}" -le 64 || fail 'invalid token length'
}
check_token() {
    pending || fail 'no pending transaction'
    test "$(cat "$current/token")" = "$1" || fail 'transaction token mismatch'
}
verify() { (cd "$1" && sha256sum -c manifest >/dev/null); }
install_configs() {
    verify "$1"
    for name in $files; do
        cp -p "$1/$name" "$configs/.$name.f3s-recovery"
        mv "$configs/.$name.f3s-recovery" "$configs/$name"
    done
    sync
}
reload_services() {
    /etc/init.d/firewall reload
    /etc/init.d/network reload
    /etc/init.d/dnsmasq restart
    /etc/init.d/odhcpd restart
}
restore() {
    phase restoring
    install_configs "$current/before"
    if test "$1" = live; then reload_services; fi
    phase "restored-$1"
    rm "$current/pending"
    sync
}
expired() {
    test "$(cat "$boot_id" 2>/dev/null || true)" = "$(cat "$current/boot-id" 2>/dev/null || true)" || return 0
    clock_seconds=$(now) || return 0
    clock_limit=$(cat "$current/deadline") || return 0
    case $clock_limit in ''|*[!0-9]*) return 0;; esac
    test "$clock_seconds" -ge "$clock_limit"
}

action=${1:-}
case $action in
watch)
    test "$#" -eq 1 || fail 'watch takes no arguments'
    # This supervisor has no lock descriptor; each bounded tick owns its lock.
    while :; do "$0" tick; sleep 2; done
    ;;
arm|activate|confirm|restore|tick|boot|status) ;;
*) fail 'usage: ap-network-recovery arm TOKEN SECONDS CANDIDATE | activate TOKEN | confirm TOKEN services-verified | restore TOKEN | tick | boot | status';;
esac

test "$(id -u)" = 0 || fail 'root required'
test -d "$state" && test ! -L "$state" || fail 'private state directory required'
private_directory "$state" || fail 'unsafe state directory'
exec 9>>"$state/lock"
flock -x 9

case $action in
arm)
    test "$#" -eq 4 || fail 'arm requires token, seconds and candidate directory'
    validate_token "$2"
    case $3 in ''|0*|*[!0-9]*) fail 'invalid timeout';; esac
    test "$3" -ge 60 && test "$3" -le 1800 || fail 'timeout must be 60-1800 seconds'
    pending && fail 'another transaction is pending'
    test ! -e "$state/history/$2" || fail 'token already used'
    if test -d "$current" && test "$(cat "$current/token")" = "$2"; then fail 'token already used'; fi
    test -L "$boot_link" || fail 'early boot hook required'
    "$service" running || fail 'independent watchdog required'
    test -z "$(uci changes)" || fail 'uncommitted UCI changes exist'
    test -d "$4" && test ! -L "$4" || fail 'candidate directory required'
    private_directory "$4" || fail 'unsafe candidate directory'
    boot=$(cat "$boot_id")
    test -n "$boot" || fail 'stable boot identity unavailable'
    started=$(now) || fail 'monotonic clock unavailable'
    staging=$state/arm.$$
    test ! -e "$staging" || fail 'staging path exists'
    mkdir "$staging" "$staging/before" "$staging/candidate" "$staging/uci"
    for name in $files; do
        test -f "$configs/$name" && test ! -L "$configs/$name" || fail 'unsafe live config'
        test -f "$4/$name" && test ! -L "$4/$name" || fail 'unsafe candidate config'
        cp -p "$configs/$name" "$staging/before/$name"
        cp -p "$4/$name" "$staging/candidate/$name"
        uci -c "$staging/candidate" -t "$staging/uci" export "$name" >/dev/null
    done
    test "$(uci -c "$staging/candidate" -t "$staging/uci" get network.lan.proto)" = static || fail 'static LAN required'
    test "$(uci -c "$staging/candidate" -t "$staging/uci" get network.lan.device)" = br-lan || fail 'LAN bridge must be retained'
    old_retained=no
    for address in $(uci -c "$staging/candidate" -t "$staging/uci" get network.lan.ipaddr); do
        case $address in
        192.168.1.101)
            test "$(uci -c "$staging/candidate" -t "$staging/uci" get network.lan.netmask)" = 255.255.255.0 || fail 'old prefix must be retained'
            old_retained=yes
            ;;
        192.168.1.101/24) old_retained=yes;;
        esac
    done
    test "$old_retained" = yes || fail 'candidate must retain old management address'
    for directory in before candidate; do
        # Three fixed filenames; split the controlled list into path arguments.
        # shellcheck disable=SC2086
        (cd "$staging/$directory" && sha256sum $files > manifest)
    done
    printf '%s\n' "$boot" > "$staging/boot-id"
    printf '%s\n' "$(( started + $3 ))" > "$staging/deadline"
    printf '%s\n' "$2" > "$staging/token"
    printf '%s\n' armed > "$staging/phase"
    netstat -nt > "$staging/sockets-before"
    : > "$staging/pending"
    mkdir -p "$state/history"
    if test -e "$current"; then
        previous=$(cat "$current/token")
        validate_token "$previous"
        test ! -e "$state/history/$previous" || fail 'history collision'
        mv "$current" "$state/history/$previous"
    fi
    sync
    mv "$staging" "$current"
    sync
    ;;
activate)
    test "$#" -eq 2 || fail 'activate requires token'
    check_token "$2"
    expired && fail 'transaction expired; watchdog will restore'
    test "$(cat "$current/phase")" = armed || fail 'transaction is not armed'
    # Pending is durable before the first live write. Partial apply stays pending.
    phase activating
    install_configs "$current/candidate"
    reload_services
    phase active
    ;;
confirm)
    test "$#" -eq 3 && test "$3" = services-verified || fail 'service verification required'
    check_token "$2"
    expired && fail 'transaction expired; cannot confirm'
    test "$(cat "$current/phase")" = active || fail 'transaction is not active'
    test -n "${SSH_CONNECTION:-}" || fail 'fresh SSH connection required'
    # SSH supplies a four-field tuple; glob expansion is disabled above.
    # shellcheck disable=SC2086
    set -- $SSH_CONNECTION
    test "$#" -eq 4 && test "$3" = 10.30.0.2 && test "$4" = 22 || fail 'confirmation must use new management address'
    local_socket=$3:$4
    remote_socket=$1:$2
    if awk -v l="$local_socket" -v r="$remote_socket" '$4==l && $5==r && $6=="ESTABLISHED" { found=1 } END { exit !found }' "$current/sockets-before"; then
        fail 'SSH connection predates transaction'
    fi
    netstat -nt | awk -v l="$local_socket" -v r="$remote_socket" '$4==l && $5==r && $6=="ESTABLISHED" { found=1 } END { exit !found }' || fail 'SSH socket not established'
    verify "$current/candidate"
    for name in $files; do cmp "$configs/$name" "$current/candidate/$name" >/dev/null || fail 'live configuration drift'; done
    expired && fail 'transaction expired during verification; cannot confirm'
    phase confirmed
    rm "$current/pending"
    sync
    ;;
restore)
    test "$#" -eq 2 || fail 'restore requires token'
    check_token "$2"
    restore live
    ;;
boot)
    test "$#" -eq 1 || fail 'boot takes no arguments'
    # Never retain unconfirmed persistent configuration across an AP reboot.
    if pending && test "$(cat "$boot_id" 2>/dev/null || true)" != "$(cat "$current/boot-id")"; then restore boot; fi
    ;;
tick)
    test "$#" -eq 1 || fail 'tick takes no arguments'
    if pending && expired; then restore live; fi
    ;;
status)
    test "$#" -eq 1 || fail 'status takes no arguments'
    if test -d "$current"; then
        printf 'token=%s phase=%s deadline=%s pending=' "$(cat "$current/token")" "$(cat "$current/phase")" "$(cat "$current/deadline")"
        if pending; then printf 'yes\n'; else printf 'no\n'; fi
    else printf 'no transaction\n'; fi
    ;;
esac
