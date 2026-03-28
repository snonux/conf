#!/bin/bash
# Shut down the OpenBSD build VM gracefully.

set -e

VMDIR="$(cd "$(dirname "$0")" && pwd)"
PIDFILE="$VMDIR/qemu.pid"
SSH_PORT=2222

if [ ! -f "$PIDFILE" ] || ! kill -0 "$(cat "$PIDFILE")" 2>/dev/null; then
    echo "Build VM is not running."
    rm -f "$PIDFILE"
    exit 0
fi

echo "Shutting down build VM..."
# Graceful shutdown via SSH, fall back to SIGTERM
ssh -q -o ConnectTimeout=5 -p "$SSH_PORT" pbuild@localhost "doas halt -p" 2>/dev/null || true

# Wait for QEMU process to exit
PID=$(cat "$PIDFILE")
for i in $(seq 1 15); do
    if ! kill -0 "$PID" 2>/dev/null; then
        echo "VM stopped."
        rm -f "$PIDFILE"
        exit 0
    fi
    sleep 1
done

# Force kill if still running
echo "Force-killing QEMU (PID $PID)..."
kill "$PID" 2>/dev/null || true
rm -f "$PIDFILE"
