#!/bin/bash
# Provision the OpenBSD build VM after a fresh install.
# Run once from the host after setup.sh completes.
#
# Installs Go, gmake, git, sets up SSH key access,
# doas for the build user, and signify keys for package signing.
# Uses sshpass for initial password-based SSH (before key is installed).

set -e

VMDIR="$(cd "$(dirname "$0")" && pwd)"
SSH_PORT=2222
# ssh takes the port as -p, scp as -P: keep the port out of the shared options.
SSH_OPTS="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null"

# First boot the VM
echo "Starting build VM..."
"$VMDIR/start.sh"

# Set up SSH key access with a password-only login. ssh-copy-id first offers
# every agent key, and the fresh OpenBSD sshd (PerSourcePenalties) then
# penalises this source and closes further connections ("Not allowed at this
# time"), which is how the 8.0 reinstall failed. The first boot (host key
# generation, kernel relink) also outlasts start.sh's 60 s wait, so retry the
# login every 20 s for up to 5 minutes; the pause also lets a penalty expire.
echo "Setting up SSH key access..."
PASSWORD_OPTS="-o PubkeyAuthentication=no -o PreferredAuthentications=password,keyboard-interactive -o ConnectTimeout=10"
install_keys() {
    { ssh-add -L 2>/dev/null; cat "$HOME"/.ssh/id_*.pub 2>/dev/null; } | sort -u |
        "$@" ssh $SSH_OPTS $PASSWORD_OPTS -p $SSH_PORT pbuild@localhost \
            'umask 077; mkdir -p .ssh; cat >> .ssh/authorized_keys'
}
if command -v sshpass &>/dev/null; then
    for attempt in $(seq 1 15); do
        install_keys sshpass -p build123 && break
        [ "$attempt" -eq 15 ] && { echo "SSH key setup failed"; exit 1; }
        sleep 20
    done
else
    echo "sshpass not found. Enter the build user password (build123) when prompted:"
    install_keys env
fi

SSH="ssh $SSH_OPTS -p $SSH_PORT pbuild@localhost"
SCP="scp $SSH_OPTS -P $SSH_PORT"

# Configure doas for passwordless access (may already be set by setup.sh)
echo "Configuring doas..."
$SSH "echo 'permit nopass pbuild' | doas tee /etc/doas.conf > /dev/null"

# Install build tools
echo "Installing Go, git, gmake..."
$SSH "doas pkg_add go git gmake"

# Copy signify keys for package signing (if available locally)
if [ -f "$VMDIR/custom-pkg.sec" ] && [ -f "$VMDIR/custom-pkg.pub" ]; then
    echo "Installing signify keys..."
    $SCP "$VMDIR/custom-pkg.sec" "$VMDIR/custom-pkg.pub" pbuild@localhost:/tmp/
    $SSH "doas cp /tmp/custom-pkg.sec /tmp/custom-pkg.pub /etc/signify/ && \
          doas chmod 600 /etc/signify/custom-pkg.sec && \
          doas chmod 644 /etc/signify/custom-pkg.pub && \
          rm /tmp/custom-pkg.sec /tmp/custom-pkg.pub"
    echo "Signify keys installed."
else
    echo ""
    echo "WARNING: Signify keys not found at $VMDIR/custom-pkg.{sec,pub}"
    echo "Copy them from fishfinger before building signed packages:"
    echo "  scp rex@fishfinger.buetow.org:/etc/signify/custom-pkg.sec $VMDIR/"
    echo "  scp rex@fishfinger.buetow.org:/etc/signify/custom-pkg.pub $VMDIR/"
    echo "Then re-run: $0"
fi

echo ""
echo "Verifying..."
$SSH "go version && uname -a"
echo ""
echo "Build VM provisioned. Ready for: make dtail-openbsd"
