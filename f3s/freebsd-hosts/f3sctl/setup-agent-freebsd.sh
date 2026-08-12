#!/bin/sh
# Set up the restricted f3sctl agent account on a FreeBSD f-host (f0-f3).
#
# f3sctl runs on pi0/pi1 and reaches these hosts over SSH to stop guests, power
# off, and export the zusb pool. Rather than logging in as `paul` (a full-shell
# account), it gets its own unprivileged account that can execute nothing but
# the f3sctl binary:
#
#   * the account has no ~/.ssh -- its authorized_keys lives in a root-owned
#     path outside its home, so the account cannot re-authorise itself;
#   * from="..." pins the key to pi0/pi1 on both LAN and WireGuard addresses;
#   * ForceCommand in sshd_config overrides whatever the key file says, so the
#     restriction is root-owned daemon config rather than a key option;
#   * the agent accepts a single bare word from a fixed allowlist, no
#     arguments, nothing reaching a shell;
#   * doas rules are keyed to exact argv for the four verbs needing root.
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

# pi0 and pi1, on the LAN and over the WireGuard mesh. The key is refused from
# anywhere else -- including earth.
FROM='192.168.1.125,192.168.1.126,192.168.2.203,192.168.2.204'

# --- account -------------------------------------------------------------
# A real shell is required: sshd execs the ForceCommand through the account's
# shell, so nologin would break the agent rather than harden it. The account is
# still unable to obtain a shell, because ForceCommand replaces whatever was
# requested.
if ! pw usershow f3sctl >/dev/null 2>&1; then
    pw useradd f3sctl -d "$HOMEDIR" -s /bin/sh -c "f3sctl power agent" -w no
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
# AuthorizedKeysFile is scoped inside the Match block, so paul and root keep
# their usual ~/.ssh/authorized_keys.
if ! grep -q '^Match User f3sctl' /etc/ssh/sshd_config; then
    cat >> /etc/ssh/sshd_config <<'SSHD'

# f3sctl: the power-control agent driven from pi0/pi1. This account may run
# nothing but the f3sctl binary, enforced here rather than only by the key.
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

# Validate before reloading: a syntax error here would take sshd down and, on a
# host whose whole purpose is being reachable remotely, that is unrecoverable
# without a console.
/usr/sbin/sshd -t
service sshd reload

# --- doas ----------------------------------------------------------------
# Exact-argv allowlist: agent-root refuses to run unless euid is 0 and argv is
# a single allowlisted verb, so this is as narrow as the SSH side.
DOAS=/usr/local/etc/doas.conf

# One rule per verb, each added only if missing. Checking per verb rather than
# for the block as a whole is what makes this re-runnable on a host that was
# set up before a verb existed: the old "does the block exist" test skipped
# the whole append, so carp-quiesce (added 2026-08-11) would silently never
# have been permitted on any already-configured host, and every rack shutdown
# would have fallen back to its slow sequential path.
if ! grep -q '# f3sctl power agent' "$DOAS" 2>/dev/null; then
    printf '\n# f3sctl power agent: only the verbs that genuinely need root.\n' >> "$DOAS"
fi

for verb in poweroff zusb-unload probe carp-quiesce; do
    rule="permit nopass f3sctl as root cmd /usr/local/bin/f3sctl args agent-root $verb"
    if ! grep -qxF "$rule" "$DOAS" 2>/dev/null; then
        printf '%s\n' "$rule" >> "$DOAS"
        echo "appended the $verb rule to $DOAS"
    fi
done

echo "--- result on $(hostname -s) ---"
pw usershow f3sctl
ls -l "$AKDIR/f3sctl" "$BIN"
