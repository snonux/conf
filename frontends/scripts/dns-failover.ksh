#!/bin/ksh

# DNS health decision client, run every minute from cron on BOTH frontends
# (the KISS HA design, foo.zone "KISS high availability with OpenBSD"). Each
# frontend is an independent NSD master and decides on its own, without a
# quorum: both apply the same rule, so they agree whenever they see the same
# health.
#
#   1. The preferred role alternates weekly (preferred_role): both frontends
#      take turns serving the apex/www records, so each one renews every
#      site's ACME certificate while it is active (acme.sh only requests
#      names that resolve to the host itself).
#   2. If the preferred role fails its public HTTPS health check (IPv4 or
#      IPv6, HEALTH_TRIES consecutive failures), the other role takes over,
#      provided it is healthy itself.
#   3. If both fail, DNS stays unchanged.
#
# When a failed frontend comes back, rule 1 brings both frontends back to the
# same role within a minute, so a split never outlives the outage. A real
# network partition (each side healthy, the peer unreachable) can still make
# both promote themselves; that is accepted, as in the original design.
#
# The publisher owns every mutation; this script only asks it to publish a
# verified role and never edits zone files itself.

set -eu

readonly PUBLISH=/usr/local/bin/dns-publish.ksh
readonly PUBLISHER_CONF=/var/nsd/etc/gonf-publisher/publisher.conf
readonly STATE=/var/nsd/gonf-publisher/state
readonly DOMAIN=buetow.org
readonly LOOKUP_TIMEOUT=10
readonly HEALTH_TRIES=3
readonly HEALTH_RETRY_SLEEP=2

# preferred_role prints the role both frontends serve while both are healthy:
# the Standby host in odd UTC weeks, the Master host in even ones. UTC keeps
# the two hosts (different local time zones) switching in the same minute.
# dns-publish.ksh has the same function for its no-state default; keep both
# in sync.
preferred_role() {
    typeset week
    week=$(date -u +%U)
    week=${week#0}
    if [ $((week % 2)) -eq 1 ]; then
        print -r -- "$STANDBY_NAME"
    else
        print -r -- "$MASTER_NAME"
    fi
}

# committed_role prints the role this host's NSD currently publishes. Without
# a state file (a host that never published) it is the preferred role, the
# same default dns-publish.ksh uses.
committed_role() {
    [ -r "$STATE" ] || { preferred_role; return; }
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

# role_healthy requires both address families: the apex has A and AAAA
# records, so a frontend reachable over only one family is not fit to serve.
role_healthy() {
    health_check_family 4 "$1" && health_check_family 6 "$1"
}

alternate_role() {
    case $1 in
    "$MASTER_NAME") print -r -- "$STANDBY_NAME" ;;
    "$STANDBY_NAME") print -r -- "$MASTER_NAME" ;;
    *) print -u2 "invalid DNS role: $1"; return 1 ;;
    esac
}

main() {
    typeset current desired preferred other
    [ -r "$PUBLISHER_CONF" ] || { print -u2 "missing DNS publisher inputs"; exit 1; }
    # shellcheck disable=SC1090 # controller-installed immutable publisher input
    . "$PUBLISHER_CONF"
    : "${MASTER_NAME:?missing master name}"
    : "${STANDBY_NAME:?missing standby name}"

    current=$(committed_role) || { print -u2 "malformed DNS publisher state"; exit 1; }
    preferred=$(preferred_role)
    other=$(alternate_role "$preferred") || exit 1
    if role_healthy "$preferred"; then
        desired=$preferred
    elif role_healthy "$other"; then
        desired=$other
    else
        print -u2 "both frontend roles failed health checks; leaving DNS unchanged"
        exit 1
    fi
    [ "$desired" = "$current" ] && exit 0
    logger -t dns-failover "DNS role $current -> $desired (preferred $preferred)"
    exec "$PUBLISH" -r "$desired"
}

main
