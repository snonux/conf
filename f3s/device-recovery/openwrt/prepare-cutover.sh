#!/bin/sh
# Prepare the final dumb-AP settings in an isolated UCI directory only.
set -eu
umask 077
set -f
PATH=/usr/sbin:/usr/bin:/sbin:/bin
export PATH

test "$#" -eq 1
candidate=$1
case $candidate in /etc/f3s-ap-recovery/candidates/*) ;; *) exit 1 ;; esac
name=${candidate#/etc/f3s-ap-recovery/candidates/}
case $name in ''|*[!a-zA-Z0-9_-]*) exit 1 ;; esac
case $(/usr/sbin/f3s-ap-network-recovery status) in *'phase=confirmed '*'pending=no') ;; *) exit 1 ;; esac
test -z "$(uci changes)"
test "$(uci get network.lan.proto)" = static
test "$(uci get network.lan.device)" = br-lan
test "$(uci get network.ap_migration.ipaddr)" = 10.30.0.2/24
test ! -e "$candidate"
mkdir "$candidate" "$candidate/.uci"
for name in network dhcp firewall; do cp -p "/etc/config/$name" "$candidate/$name"; done
cu() { uci -c "$candidate" -t "$candidate/.uci" "$@"; }
cu delete network.lan.ipaddr
cu -q delete network.lan.netmask || true
cu add_list network.lan.ipaddr=10.30.0.2/24
cu add_list network.lan.ipaddr=192.168.1.101/24
cu set network.lan.gateway=10.30.0.1
cu -q delete network.lan.dns || true
cu add_list network.lan.dns=10.30.0.1
cu -q delete network.lan.ip6assign || true
cu set network.lan.delegate=0
cu delete network.ap_migration
cu delete network.ap_migration_admin
cu set dhcp.lan.ignore=1
cu set dhcp.lan.dhcpv4=disabled
cu set dhcp.lan.ra=disabled
cu set dhcp.lan.dhcpv6=disabled
cu set dhcp.lan.ndp=disabled
# The old gateway advertisement is no longer useful with DHCP disabled.
test "$(cu get dhcp.lan.dhcp_option)" = 3,192.168.1.1
cu delete dhcp.lan.dhcp_option
cu commit network
cu commit dhcp
test "$(cu get network.lan.ipaddr)" = '10.30.0.2/24 192.168.1.101/24'
test "$(cu get dhcp.lan.ignore)" = 1
test "$(cu get dhcp.@dnsmasq[0].rebind_protection)" = 1
test "$(cu get dhcp.@dnsmasq[0].rebind_domain)" = /f3s.lan.buetow.org/
for name in network dhcp firewall; do cu export "$name" >/dev/null; done
cmp /etc/config/firewall "$candidate/firewall"
test -z "$(uci changes)"
(cd "$candidate" && sha256sum network dhcp firewall > manifest)
printf '%s\n' 'Inactive cutover prepared: new/old management, gateway/DNS30.1, AP DHCP/RA/DHCPv6 off.'
