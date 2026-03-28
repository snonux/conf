#!/bin/bash
# Start the OpenBSD build VM in the background.
# SSH available at localhost:2222 after boot (~15s).

set -e

VMDIR="$(cd "$(dirname "$0")" && pwd)"
DISK="$VMDIR/openbsd-build.qcow2"
PIDFILE="$VMDIR/qemu.pid"
SSH_PORT=2222
RAM=1024
CPUS=2

if [ ! -f "$DISK" ]; then
    echo "Error: $DISK not found. Run setup.sh first."
    exit 1
fi

if [ -f "$PIDFILE" ] && kill -0 "$(cat "$PIDFILE")" 2>/dev/null; then
    echo "Build VM already running (PID $(cat "$PIDFILE"))"
    exit 0
fi

# Use -display none + -serial null for headless background operation.
# -nographic cannot be combined with -daemonize.
echo "Starting OpenBSD build VM..."
qemu-system-x86_64 \
    -machine accel=kvm \
    -cpu host \
    -m "$RAM" \
    -smp "$CPUS" \
    -drive file="$DISK",format=qcow2,if=virtio \
    -netdev user,id=net0,hostfwd=tcp::${SSH_PORT}-:22 \
    -device virtio-net-pci,netdev=net0 \
    -display none \
    -serial null \
    -daemonize \
    -pidfile "$PIDFILE"

echo "VM started (PID $(cat "$PIDFILE")), waiting for SSH..."

# Wait for SSH to become available
for i in $(seq 1 30); do
    if ssh -q -o ConnectTimeout=2 -o StrictHostKeyChecking=no -p "$SSH_PORT" pbuild@localhost true 2>/dev/null; then
        echo "SSH ready at localhost:$SSH_PORT"
        exit 0
    fi
    sleep 2
done

echo "Warning: SSH not responding after 60s. VM may still be booting."
