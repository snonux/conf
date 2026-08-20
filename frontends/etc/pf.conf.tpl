#	$OpenBSD: pf.conf,v 1.55 2017/12/03 20:40:04 sthen Exp $
#
# See pf.conf(5) and /etc/examples/pf.conf

# NAT for WireGuard clients to access internet (IPv4)
# This allows roaming clients (earth, pixel7pro) to route all traffic
# through the VPN and access the internet via the gateway's public IP
match out on vio0 from 192.168.2.0/24 to any nat-to (vio0)

# NAT66 for WireGuard clients to access internet (IPv6)
# This allows roaming clients to route IPv6 traffic through the VPN
# Uses NPTv6 (Network Prefix Translation) to translate ULA to public IPv6
match out on vio0 inet6 from fd42:beef:cafe:2::/64 to any nat-to (vio0)

set skip on lo

block return	# block stateless traffic
pass		# establish keep-state

# By default, do not permit remote connections to X11
block return in on ! lo0 proto tcp to port 6000:6010

# Port build user does not need network
block return out log proto {tcp udp} user _pbuild

# Allow inbound traffic on WireGuard interface
# This permits traffic from VPN clients to access services on this host
pass in on wg0

# Allow all UDP traffic on WireGuard port (IPv4 and IPv6)
# This is required for WireGuard's encrypted tunnel communication
pass in inet proto udp from any to any port 56709
pass in inet6 proto udp from any to any port 56709

# ---------------------------------------------------------------------------
# Per-service traffic accounting.
#
# These rules exist to COUNT, not to filter: the bare `pass` above already
# permits all of this, so the policy is unchanged and every rule here is a
# `pass` too. What they buy is attribution -- `pfctl -sl` reports
# evaluations/packets/bytes per label, which a cron job scrapes into the
# node_exporter textfile collector (see rc.conf.local + scripts/pf-labels).
#
# Why this is needed at all: Traefik's Prometheus metrics only cover HTTP
# that relayd forwards into the k3s ingress. Gemini, git+ssh, the pi0/pi1
# static sites and the NodePort relays (jellyfin, anki, garage, registry)
# all bypass Traefik entirely, so it reports zero for them however busy
# they are. pf sits underneath everything and sees the lot.
#
# Placement and form both matter:
#   - They are LAST in the file on purpose. pf is last-match-wins (absent
#     `quick`), so these override the earlier bare `pass` and become the
#     state-creating rule for their ports -- which is what makes the byte
#     counters accumulate. Labelling a `match` rule would not work: `match`
#     does not create state, so its byte counters stay at zero.
#   - No address family is specified, so each rule covers IPv4 and IPv6.
#     That is deliberate: an v4-only fault is otherwise invisible, and the
#     two families have already been shown to behave differently here.
#
# Adding a service? Append a labelled rule; the exporter picks it up with no
# further change, as the scrape config already targets both gateways.
pass in on vio0 proto tcp to port 443   label "svc_https"
pass in on vio0 proto tcp to port 80    label "svc_http"
pass in on vio0 proto tcp to port 1965  label "svc_gemini"
pass in on vio0 proto tcp to port 2443  label "svc_forgejo_web_alt"
pass in on vio0 proto tcp to port 2022  label "svc_forgejo_ssh"
pass in on vio0 proto tcp to port 2222  label "svc_dserver"
pass in on vio0 proto tcp to port 2     label "svc_ssh_admin"
pass in on vio0 proto udp to port 56709 label "svc_wireguard"
