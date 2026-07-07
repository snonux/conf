#!/bin/sh
# NetBSD headless image customization. Fetched over qemu user-net and run as
# root INSIDE the NetBSD-in-qemu VM by config.exp during a bake.
#
# `bake-golden.sh` substitutes __SSHKEY__ and __PW__ before serving this file.
# EDIT the two per-host values below for the target Pi (see README "Doing pi1").
set -eu

HOSTNAME="pi0.lan.buetow.org"      # <-- EDIT for the target Pi (e.g. pi1.lan.buetow.org)
IPADDR="192.168.1.125"             # <-- EDIT for the target Pi (e.g. 192.168.1.126)
GW="192.168.1.1"
DNS="192.168.1.1"

RC=/etc/rc.conf

# 1) rc.conf: static networking + hostname + sshd; strip conflicting keys first.
tmp=$(mktemp)
grep -vE '^(hostname|ifconfig_mue0|defaultroute|dhcpcd|sshd)=' "$RC" > "$tmp"
mv "$tmp" "$RC"
cat >> "$RC" <<EOF
hostname="$HOSTNAME"
sshd=YES
dhcpcd=NO
ifconfig_mue0="inet $IPADDR netmask 0xffffff00"
defaultroute="$GW"
EOF

# 2) resolv.conf
echo "nameserver $DNS" > /etc/resolv.conf

# 3) rc.local fallback: if the LAN78xx iface is not named mue0, put the static IP
#    on the first real ethernet interface so the box stays reachable.
cat > /etc/rc.local <<EOF
#!/bin/sh
if ! ifconfig mue0 >/dev/null 2>&1; then
  iface=\$(ifconfig -l | tr ' ' '\n' | grep -E '^(mue|ure|axe|cdce|vioif)[0-9]' | head -1)
  if [ -n "\$iface" ]; then
    ifconfig "\$iface" inet $IPADDR netmask 0xffffff00 up
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
