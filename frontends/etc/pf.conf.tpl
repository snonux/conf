#	$OpenBSD: pf.conf,v 1.55 2017/12/03 20:40:04 sthen Exp $
#
# See pf.conf(5) and /etc/examples/pf.conf

# NAT for WireGuard clients to access internet
# This allows roaming clients (earth, pixel7pro) to route all traffic
# through the VPN and access the internet via the gateway's public IP
match out on vio0 from 192.168.2.0/24 to any nat-to (vio0)

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

# Allow all UDP traffic on WireGuard port
# This is required for WireGuard's encrypted tunnel communication
pass in inet proto udp from any to any port 56709
