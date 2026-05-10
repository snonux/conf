#!/bin/bash
# NFS mount health monitor — runs every 10 seconds via systemd timer
# (nfs-mount-monitor.timer / nfs-mount-monitor.service)
#
# Checks whether /data/nfs/k3svolumes is mounted and responsive.
# If the mount is stale or missing it attempts a remount, then a
# fresh umount+mount cycle.  On a successful repair it force-deletes
# any pods on this node that are stuck in Unknown/Pending/ContainerCreating,
# allowing the kubelet to reschedule them against the now-healthy volume.
#
# Deploy via Rex: rex -f f3s/r-nodes/Rexfile nfs_mount_monitor

MOUNT_POINT="/data/nfs/k3svolumes"
LOCK_FILE="/var/run/nfs-mount-check.lock"

# Use a lock file to prevent concurrent runs (timer fires every 10 s)
if [ -f "$LOCK_FILE" ]; then
    exit 0
fi
touch "$LOCK_FILE"
trap "rm -f $LOCK_FILE" EXIT

MOUNT_FIXED=0

fix_mount () {
    echo "Attempting to remount NFS mount $MOUNT_POINT"
    if mount -o remount -f "$MOUNT_POINT" 2>/dev/null; then
        echo "Remount command issued for $MOUNT_POINT"
    else
        echo "Failed to remount NFS mount $MOUNT_POINT"
    fi

    echo "Checking if $MOUNT_POINT is a mountpoint"
    if mountpoint "$MOUNT_POINT" >/dev/null 2>&1; then
        echo "$MOUNT_POINT is a valid mountpoint"
    else
        echo "$MOUNT_POINT is not a valid mountpoint, attempting mount"
        if mount "$MOUNT_POINT"; then
            echo "Successfully mounted $MOUNT_POINT"
            MOUNT_FIXED=1
            return
        else
            echo "Failed to mount $MOUNT_POINT"
        fi
    fi

    echo "Attempting to unmount $MOUNT_POINT"
    if umount -f "$MOUNT_POINT" 2>/dev/null; then
        echo "Successfully unmounted $MOUNT_POINT"
    else
        echo "Failed to unmount $MOUNT_POINT (it might not be mounted)"
    fi

    echo "Attempting to mount $MOUNT_POINT"
    if mount "$MOUNT_POINT"; then
        echo "NFS mount $MOUNT_POINT mounted successfully"
        MOUNT_FIXED=1
        return
    else
        echo "Failed to mount NFS mount $MOUNT_POINT"
    fi

    echo "Failed to fix NFS mount $MOUNT_POINT"
    exit 1
}

if ! mountpoint "$MOUNT_POINT" >/dev/null 2>&1; then
    echo "NFS mount $MOUNT_POINT not found"
    fix_mount
fi

if ! timeout 2s stat "$MOUNT_POINT" >/dev/null 2>&1; then
    echo "NFS mount $MOUNT_POINT appears to be unresponsive"
    fix_mount
fi

# After a successful remount, delete pods stuck on this node
if [ "$MOUNT_FIXED" -eq 1 ]; then
    echo "Mount was fixed, checking for stuck pods on this node..."
    NODE=$(hostname)
    export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
    kubectl get pods --all-namespaces --field-selector="spec.nodeName=$NODE" \
      -o json 2>/dev/null | jq -r '
        .items[] |
        select(
          .status.phase == "Unknown" or
          .status.phase == "Pending" or
          (.status.conditions // [] | any(.type == "Ready" and .status == "False")) or
          (.status.containerStatuses // [] | any(.state.waiting.reason == "ContainerCreating"))
        ) | "\(.metadata.namespace) \(.metadata.name)"' | \
      while read ns pod; do
        echo "Deleting stuck pod $ns/$pod"
        kubectl delete pod -n "$ns" "$pod" --grace-period=0 --force 2>&1
      done
fi
