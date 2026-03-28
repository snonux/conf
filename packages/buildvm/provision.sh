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
SSH_OPTS="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -p $SSH_PORT"

# First boot the VM
echo "Starting build VM..."
"$VMDIR/start.sh"

# Set up SSH key — use sshpass if available, fall back to manual prompt
echo "Setting up SSH key access..."
if command -v sshpass &>/dev/null; then
    sshpass -p build123 ssh-copy-id $SSH_OPTS pbuild@localhost 2>/dev/null
else
    echo "sshpass not found. Enter the build user password (build123) when prompted:"
    ssh-copy-id $SSH_OPTS pbuild@localhost
fi

SSH="ssh $SSH_OPTS pbuild@localhost"
SCP="scp $SSH_OPTS"

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
