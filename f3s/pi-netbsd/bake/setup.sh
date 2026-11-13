#!/bin/sh
# NetBSD headless image customization. Fetched over qemu user-net and run as
# root INSIDE the NetBSD-in-qemu VM by config.exp during a bake.
#
# `bake-golden.sh` substitutes __SSHKEY__ and __PW__ before serving this file.
# These defaults document pi0; bake-golden.sh substitutes the selected piN's
# values from the steady gonf config (lab address and ULA from cluster.go,
# gateway and DNS triggerfish), so the rebuilt Pi comes up where gonf keeps it.
# The rc.conf lines below are written exactly as pis_netbsd_network_rc_conf
# renders them.
set -eu

HOSTNAME="pi0.lan.buetow.org"
IPADDR="10.20.0.125"
ULA="fd02:ca0b:f1cb:20::125"
GW="10.20.0.1"
DNS="10.20.0.1"

RC=/etc/rc.conf

# 1) rc.conf: static networking + hostname + sshd; strip conflicting keys first.
#    rc.d/network splits ifconfig_mue0 at ';': the /24 lab IPv4 first, then
#    the ULA as an inet6 alias (no router advertisements). mktemp leaves 0600;
#    rc.conf is the stock 0644 (as gonf's RootOwned keeps it).
tmp=$(mktemp)
grep -vE '^(hostname|ifconfig_mue0|defaultroute|dhcpcd|sshd)=' "$RC" > "$tmp"
mv "$tmp" "$RC"
chmod 0644 "$RC"
cat >> "$RC" <<EOF
hostname="$HOSTNAME"
sshd=YES
dhcpcd=NO
ifconfig_mue0="inet $IPADDR netmask 0xffffff00;inet6 $ULA prefixlen 64 alias"
defaultroute="$GW"
EOF

# 2) resolv.conf
echo "nameserver $DNS" > /etc/resolv.conf

# 3) rc.local fallback: if the LAN78xx iface is not named mue0, put the static
#    addresses on the first real ethernet interface so the box stays reachable.
cat > /etc/rc.local <<EOF
#!/bin/sh
if ! ifconfig mue0 >/dev/null 2>&1; then
  iface=\$(ifconfig -l | tr ' ' '\n' | grep -E '^(mue|ure|axe|cdce|vioif)[0-9]' | head -1)
  if [ -n "\$iface" ]; then
    ifconfig "\$iface" inet $IPADDR netmask 0xffffff00 up
    ifconfig "\$iface" inet6 $ULA prefixlen 64 alias
    route add default $GW
  fi
fi
EOF
chmod 0755 /etc/rc.local

# 4) user paul (+wheel) + authorized key
if ! id paul >/dev/null 2>&1; then useradd -m -G wheel -s /bin/sh paul; fi
mkdir -p /home/paul/.ssh
cat > /home/paul/.ssh/authorized_keys <<'EOF'
__SSHKEY__
EOF
chmod 0700 /home/paul/.ssh
chmod 0600 /home/paul/.ssh/authorized_keys
chown -R paul:users /home/paul/.ssh

# 5) passwords (native NetBSD argon2id hash), same for paul + root (backup login)
H=$(pwhash '__PW__')
usermod -p "$H" paul
usermod -p "$H" root

# 6) sshd: ensure key + password auth (appended lines win over commented defaults)
cat >> /etc/ssh/sshd_config <<'EOF'
PubkeyAuthentication yes
PasswordAuthentication yes
EOF

rm -f /tmp/setup.sh
# Flush all writes to the image before qemu is stopped (soft-dep FFS).
sync; sync
echo SETUP_OK
