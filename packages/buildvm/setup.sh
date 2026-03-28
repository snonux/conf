#!/bin/bash
# Create and configure an OpenBSD QEMU/KVM VM for native package builds.
#
# Fully automated via expect driving the serial console installer.
# The expect script is in install-expect.exp (separate file avoids
# bash/expect quoting issues with password prompts).
#
# Prerequisites: qemu-system-x86_64, expect, KVM (/dev/kvm)

set -e

VMDIR="$(cd "$(dirname "$0")" && pwd)"
DISK="$VMDIR/openbsd-build.qcow2"
OBSD_VERSION="7.8"
ISO="$VMDIR/install${OBSD_VERSION//./}.iso"
SSH_PORT=2222
RAM=1024
CPUS=2

if [ -f "$DISK" ]; then
    echo "Disk $DISK already exists. Delete it first to reinstall."
    exit 1
fi

# Download install ISO if not cached
if [ ! -f "$ISO" ]; then
    echo "Downloading OpenBSD $OBSD_VERSION install ISO..."
    curl -L -o "$ISO" "https://cdn.openbsd.org/pub/OpenBSD/$OBSD_VERSION/amd64/install${OBSD_VERSION//./}.iso"
fi

echo "Creating ${DISK}..."
qemu-img create -f qcow2 "$DISK" 4G

echo ""
echo "Starting automated OpenBSD install (takes ~5 minutes)..."
echo ""

expect "$VMDIR/install-expect.exp" "$DISK" "$ISO" "$RAM" "$CPUS" "$SSH_PORT"

echo ""
echo "OpenBSD install complete. Now run: $VMDIR/provision.sh"
