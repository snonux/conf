# Frontend inbound default-deny. Rendered per host by frontends_pf.
set skip on lo

table <bruteforce> persist

# Keep roaming clients' existing IPv4 NAT and stateful IPv6 NAT.
match out on vio0 inet from 192.168.2.0/24 to any nat-to (vio0)
match out on vio0 inet6 from fd42:beef:cafe:2::/64 to any nat-to (vio0)

block return
# Outbound connections and mesh transit create reply states as before.
pass out

# These quick blocks must precede every public allow rule.
block drop in quick on vio0 from 185.177.72.38 label "abusive_crawler"
block drop in quick on vio0 proto tcp from <bruteforce> to (vio0) port 2 label "ssh_bruteforce"
block return out log quick proto { tcp udp } user _pbuild
block return in quick on vio0 proto tcp to port 6000:6010

# dhcpleased on both live hosts uses UDP68. Initial broadcast replies may
# arrive before an outbound state exists; this does not expose a daemon port.
pass in on vio0 inet proto udp from any port 67 to any port 68 label "dhcp_client"

# IPv4 PMTU/errors and diagnostics; IPv6 errors, PMTU, RA and ND are needed
# for public addressing and on-link reachability. Permit no redirects.
pass in on vio0 inet proto icmp icmp-type { echoreq unreach timex paramprob } label "icmp_in"
pass in on vio0 inet6 proto icmp6 icmp6-type { unreach toobig timex paramprob echoreq } label "icmp6_in"
pass in on vio0 inet6 proto icmp6 icmp6-type { routersol routeradv neighbrsol neighbradv } label "ipv6_nd"

# Public services. Family-neutral rules provide IPv4/IPv6 parity. Labels
# create the states scraped by pf-labels-exporter.sh, including reply bytes.
# Limits count completed TCP connections, per source per rule. Keep HTTPS
# separate from the SSH overload table so a busy NAT cannot ban SSH users.
pass in on vio0 proto tcp to (vio0) port 2 keep state (max-src-conn 100, max-src-conn-rate 30/60, overload <bruteforce> flush global) label "svc_ssh_admin"
pass in on vio0 proto tcp to (vio0) port 25 label "svc_smtp"
pass in on vio0 proto { tcp udp } to (vio0) port 53 label "svc_dns"
pass in on vio0 proto tcp to (vio0) port 80 label "svc_http"
pass in on vio0 proto tcp to (vio0) port 443 keep state (max-src-conn 200, max-src-conn-rate 100/10) label "svc_https"
pass in on vio0 proto tcp to (vio0) port 1965 label "svc_gemini"
pass in on vio0 proto tcp to (vio0) port 2443 label "svc_forgejo_web_alt"
pass in on vio0 proto tcp to (vio0) port 2022 label "svc_forgejo_ssh"
pass in on vio0 proto udp to (vio0) port 56709 label "svc_wireguard"
# No public 8080 (httpd), 873 (rsync), 2222 (dserver), or 4242 (ZNC).
# ZNC is bound to WireGuard by frontends_wg_znc.

# Backend leg, over the WireGuard mesh.
#
# The rules above measure the front door only (`in on vio0`), where
# everything TLS arrives on :443 and is therefore indistinguishable --
# Immich, foo.zone and the Pi static sites all land in svc_https
# together. The services only separate once relayd picks a backend, so
# label that hop too: destination address and port together identify the
# service unambiguously.
#
# Note both address and port are needed. Traefik on the r-nodes and
# bozohttpd on the Pis both listen on :80, so port alone cannot tell the
# k3s cluster apart from the Raspberry Pi static sites.
#
# Counters here are per state and cover both directions, so bytes_out is
# what the backend sent back, i.e. what was ultimately served to the client.
pass out on wg0 proto tcp to { 192.168.2.203 192.168.2.204 } port 80    label "dst_pi_static"
pass out on wg0 proto tcp to { 192.168.2.120 192.168.2.121 192.168.2.122 } port 80    label "dst_traefik"
pass out on wg0 proto tcp to { 192.168.2.120 192.168.2.121 192.168.2.122 } port 30800 label "dst_anki"
pass out on wg0 proto tcp to { 192.168.2.120 192.168.2.121 192.168.2.122 } port 30001 label "dst_registry"
pass out on wg0 proto tcp to { 192.168.2.120 192.168.2.121 192.168.2.122 } port 30222 label "dst_forgejo_ssh"
pass out on wg0 proto tcp to { 192.168.2.130 192.168.2.131 192.168.2.132 } port 3900  label "dst_garage"

# Authenticated mesh access, including rsync and administrative monitoring.
pass in on wg0 label "wg_mesh_in"
