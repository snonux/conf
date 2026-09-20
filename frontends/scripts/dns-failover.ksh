#!/bin/ksh

# DNS health decision client. The publisher owns every mutation; this script
# only asks it to promote a verified alternate role. It intentionally has no
# wall-clock rotation and never edits current_* caches or zone files.

set -eu

readonly PUBLISHER=blowfish
readonly PUBLISH=/usr/local/bin/dns-publish.ksh
readonly STATE=/var/nsd/gonf-publisher/state
readonly DOMAIN=buetow.org
readonly LOOKUP_TIMEOUT=10
readonly HEALTH_TRIES=3
readonly HEALTH_RETRY_SLEEP=2

committed_role() {
    [ -r "$STATE" ] || { print fishfinger; return; }
    awk -F= '$1 == "role" { if (++n > 1 || $2 == "") exit 1; role = $2 } END { if (n != 1) exit 1; print role }' "$STATE"
}

health_check_once() {
    typeset proto=$1 role=$2 fqdn=$2.$DOMAIN
    timeout "$LOOKUP_TIMEOUT" ftp -"$proto" -o - "https://$fqdn/index.txt" | grep -q "Welcome to $fqdn"
}

health_check_family() {
    typeset proto=$1 role=$2 fqdn=$2.$DOMAIN
    typeset -i attempt=1
    while [ "$attempt" -le "$HEALTH_TRIES" ]; do
        health_check_once "$proto" "$role" && return 0
        print "https://$fqdn/index.txt IPv$proto health check failed (try $attempt/$HEALTH_TRIES)"
        [ "$attempt" -ge "$HEALTH_TRIES" ] || sleep "$HEALTH_RETRY_SLEEP"
        attempt=$((attempt + 1))
    done
    return 1
}

alternate_role() {
    case $1 in
    fishfinger) print blowfish ;;
    blowfish) print fishfinger ;;
    *) print -u2 "invalid committed DNS role"; return 1 ;;
    esac
}

main() {
    typeset current desired
    [ "$(hostname -s)" = "$PUBLISHER" ] || exit 0
    current=$(committed_role) || { print -u2 "malformed DNS publisher state"; exit 1; }
    desired=$current
    if ! health_check_family 4 "$current" || ! health_check_family 6 "$current"; then
        desired=$(alternate_role "$current") || exit 1
        if ! health_check_family 4 "$desired" || ! health_check_family 6 "$desired"; then
            print -u2 "both frontend roles failed health checks; leaving DNS unchanged"
            exit 1
        fi
    fi
    [ "$desired" = "$current" ] && exit 0
    exec "$PUBLISH" -r "$desired"
}

main
