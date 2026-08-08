#!/bin/sh
# Set up the restricted f3sctl agent account on an OpenBSD gateway
# (blowfish / fishfinger).
#
# On the gateways the agent does exactly one thing: create and remove
# /tmp/f3s_taken_down, the marker Gogios checks (OnlyIfNotExists) so that a
# deliberate cluster shutdown does not page. That needs no privileges, so
# unlike the FreeBSD f-hosts there are no doas rules here at all.
#
# The account still gets the full treatment -- own user, root-owned
# authorized_keys outside its home, source-pinned key, ForceCommand -- because
# these are the internet-facing hosts and the marker file is not the point; the
# shell it must never grant is.
#
# Idempotent: safe to re-run.
#
# Arguments:
#   $1 — the f3sctl public key (one line)

set -e

PUBKEY="$1"
if [ -z "$PUBKEY" ]; then
    echo "usage: $0 '<ssh-ed25519 AAAA... f3sctl@pi>'" >&2
    exit 1
fi

HOMEDIR=/var/db/f3sctl
AKDIR=/etc/ssh/authorized_keys.d
BIN=/usr/local/bin/f3sctl

# pi0 and pi1, on the LAN and over the WireGuard mesh. The gateways reach the
# Pis over WireGuard, so in practice it is the 192.168.2.x addresses that match
# here; the LAN pair is included so the same key works when tested from the
# LAN side.
FROM='192.168.1.125,192.168.1.126,192.168.2.203,192.168.2.204'

# --- account -------------------------------------------------------------
# A real shell is required: sshd execs the ForceCommand through the account's
# shell, so nologin would break the agent rather than harden it.
if ! id f3sctl >/dev/null 2>&1; then
    useradd -d "$HOMEDIR" -s /bin/sh -c "f3sctl power agent" f3sctl
    echo "created user f3sctl"
fi
mkdir -p "$HOMEDIR"
chown f3sctl:f3sctl "$HOMEDIR"
chmod 0750 "$HOMEDIR"

# --- authorized key ------------------------------------------------------
mkdir -p "$AKDIR"
chown root:wheel "$AKDIR"
chmod 0755 "$AKDIR"

printf '%s\n' "from=\"$FROM\",restrict,command=\"$BIN agent\" $PUBKEY" > "$AKDIR/f3sctl"
chown root:wheel "$AKDIR/f3sctl"
chmod 0644 "$AKDIR/f3sctl"

# --- sshd ----------------------------------------------------------------
# AuthorizedKeysFile is scoped inside the Match block, so rex keeps its usual
# ~/.ssh/authorized_keys. Note these hosts run sshd on port 2, not 22.
if ! grep -q '^Match User f3sctl' /etc/ssh/sshd_config; then
    cat >> /etc/ssh/sshd_config <<'SSHD'

# f3sctl: the power-control agent driven from pi0/pi1. Here it only sets and
# clears the Gogios mute marker. This account may run nothing but the f3sctl
# binary, enforced here rather than only by the key.
Match User f3sctl
    AuthorizedKeysFile /etc/ssh/authorized_keys.d/f3sctl
    PasswordAuthentication no
    KbdInteractiveAuthentication no
    ForceCommand /usr/local/bin/f3sctl agent
    PermitTTY no
    AllowTcpForwarding no
    AllowAgentForwarding no
    AllowStreamLocalForwarding no
    PermitTunnel no
    X11Forwarding no
SSHD
    echo "appended the Match User f3sctl block to sshd_config"
fi

# Validate before reloading. These are the internet-facing hosts; taking sshd
# down here means losing the way back in.
/usr/sbin/sshd -f /etc/ssh/sshd_config -t
rcctl reload sshd

echo "--- result on $(hostname -s) ---"
id f3sctl
ls -l "$AKDIR/f3sctl" "$BIN"
