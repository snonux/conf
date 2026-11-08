#!/bin/sh
# Prepare an inactive alias trial without changing live UCI or DHCP behavior.
set -eu
umask 077
set -f
PATH=/usr/sbin:/usr/bin:/sbin:/bin
export PATH

case $# in 1|2) ;; *) exit 1 ;; esac
private_dns=no
if test "$#" -eq 2; then
    test "$2" = --private-dns
    private_dns=yes
fi
candidate=$1
case $candidate in
    /etc/f3s-ap-recovery/candidates/*) ;;
    *) exit 1 ;;
esac
name=${candidate#/etc/f3s-ap-recovery/candidates/}
case $name in ''|*[!a-zA-Z0-9_-]*) exit 1 ;; esac
test "$(id -u)" = 0
status=$(/usr/sbin/f3s-ap-network-recovery status)
case $status in
    'no transaction'|*' pending=no') ;;
    *) exit 1 ;;
esac
test -z "$(uci changes)"
test ! -e "$candidate"
# Refuse to overwrite an existing operator-owned interface or return route.
if uci -q get network.ap_migration >/dev/null; then exit 1; fi
if uci -q get network.ap_migration_admin >/dev/null; then exit 1; fi
test "$(uci get network.lan.ipaddr)" = 192.168.1.101
test "$(uci get network.lan.device)" = br-lan
parent=/etc/f3s-ap-recovery/candidates
test ! -L "$parent"
mkdir -p "$parent"
chmod 0700 "$parent"
mkdir "$candidate" "$candidate/.uci"
for name in network dhcp firewall; do
    cp -p "/etc/config/$name" "$candidate/$name"
done
candidate_uci() { uci -c "$candidate" -t "$candidate/.uci" "$@"; }
candidate_uci batch <<'UCI'
set network.ap_migration=interface
set network.ap_migration.device='@lan'
set network.ap_migration.proto='static'
set network.ap_migration.ipaddr='10.30.0.2/24'
set network.ap_migration_admin=route
set network.ap_migration_admin.interface='ap_migration'
set network.ap_migration_admin.target='10.20.0.130/32'
set network.ap_migration_admin.gateway='10.30.0.1'
commit network
UCI
for name in network dhcp firewall; do
    candidate_uci export "$name" >/dev/null
done
# The old main LAN remains the only configured DHCP interface. Its address,
# default gateway and DNS options are copied unchanged; only alias/route added.
test "$(candidate_uci get network.lan.ipaddr)" = 192.168.1.101
test "$(candidate_uci get network.lan.device)" = br-lan
test "$(candidate_uci get network.ap_migration.device)" = @lan
test "$(candidate_uci get network.ap_migration.ipaddr)" = 10.30.0.2/24
if test "$private_dns" = yes; then
    test "$(candidate_uci get dhcp.@dnsmasq[0].rebind_protection)" = 1
    # Only this private DNS suffix bypasses the existing rebind protection.
    # Refuse a pre-existing identical exception rather than duplicate it.
    if candidate_uci -q get dhcp.@dnsmasq[0].rebind_domain 2>/dev/null |
        grep -Fq /f3s.lan.buetow.org/; then exit 1; fi
    uci export dhcp > "$candidate/dhcp-before-export"
    candidate_uci add_list 'dhcp.@dnsmasq[0].rebind_domain=/f3s.lan.buetow.org/'
    candidate_uci commit dhcp
    candidate_uci export dhcp > "$candidate/dhcp-after-export"
    sed "\|^[[:space:]]*list rebind_domain '/f3s[.]lan[.]buetow[.]org/'$|d" \
        "$candidate/dhcp-after-export" > "$candidate/dhcp-without-new-exception"
    cmp "$candidate/dhcp-before-export" "$candidate/dhcp-without-new-exception"
    test "$(candidate_uci get dhcp.@dnsmasq[0].rebind_protection)" = 1
else
    cmp /etc/config/dhcp "$candidate/dhcp"
fi
cmp /etc/config/firewall "$candidate/firewall"
test -z "$(uci changes)"
(cd "$candidate" && sha256sum network dhcp firewall > manifest)
printf 'Inactive alias/f0 route prepared; private DNS exception=%s; DHCP pools/firewall unchanged.\n' "$private_dns"
