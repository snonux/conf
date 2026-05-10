#!/bin/bash
# NFS mount health monitor — runs every 10 seconds via systemd timer
# (nfs-mount-monitor.timer / nfs-mount-monitor.service)
#
# Checks whether /data/nfs/k3svolumes is mounted and responsive.
# Three probes are run in order:
#   1. mountpoint — detects completely missing mounts
#   2. stat        — detects read hangs / stale cache misses
#   3. write-probe — detects the "reads OK, writes hang" failure mode
#      (stunnel-wrapped NFSv4 can enter a state where stat returns from
#      cache but ALL writes block indefinitely; only the write probe
#      catches this — mount timeo=10 deciseconds = 1s, so 5s gives one
#      full retransmit window plus margin)
#
# If any probe fails, fix_mount is called to attempt a remount, then a
# fresh umount+mount cycle.  On a successful repair it force-deletes
# any pods on this node that are stuck in Unknown/Pending/ContainerCreating,
# allowing the kubelet to reschedule them against the now-healthy volume.
#
# fix_mount recovery sequence:
#   1. mount -o remount -f  (cheapest — no disruption if mount is stale)
#   2. kill D-state processes pinning the mount (so umount can succeed)
#   3. umount -f            (force unmount)
#   4. umount -l            (lazy detach VFS node if -f failed)
#   5. systemctl restart stunnel + 2s sleep (refresh the TLS transport)
#   6. mount                (fresh mount via stunnel)
#
# A hard 60-second deadline is enforced so the function can never outlast
# its own timer interval (10s) by more than 6x, preventing timer pile-up.
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

# kill_pinning_processes — send SIGKILL to any process whose wchan starts
# with "nfs_" AND whose open file descriptors or cwd point into MOUNT_POINT.
# This unblocks D-state processes so that umount can detach the filesystem.
# Kubelet/containerd will restart the affected pods automatically.
kill_pinning_processes() {
    echo "Scanning for processes pinning $MOUNT_POINT..."
    local killed=0
    for pid_dir in /proc/[0-9]*; do
        local pid
        pid=$(basename "$pid_dir")

        # Skip non-existent pids that vanished while we iterate
        [ -d "$pid_dir" ] || continue

        # Check whether this process is stuck in an NFS kernel wait state
        local wchan
        wchan=$(cat "$pid_dir/wchan" 2>/dev/null) || continue
        [[ "$wchan" == nfs_* ]] || continue

        # Verify the process is actually using our mount point (cwd or fds)
        local cwd_link
        cwd_link=$(readlink "$pid_dir/cwd" 2>/dev/null) || true
        if [[ "$cwd_link" == "$MOUNT_POINT"* ]]; then
            echo "Killing pid $pid (wchan=$wchan, cwd=$cwd_link)"
            kill -9 "$pid" 2>/dev/null && (( killed++ )) || true
            continue
        fi

        # Also check open file descriptors
        local fd
        for fd in "$pid_dir/fd"/*; do
            local fd_target
            fd_target=$(readlink "$fd" 2>/dev/null) || continue
            if [[ "$fd_target" == "$MOUNT_POINT"* ]]; then
                echo "Killing pid $pid (wchan=$wchan, fd=$fd_target)"
                kill -9 "$pid" 2>/dev/null && (( killed++ )) || true
                break
            fi
        done
    done
    echo "Killed $killed process(es) pinning $MOUNT_POINT"
}

fix_mount () {
    # Hard deadline: fix_mount must complete within 60 seconds so the
    # 10-second timer cannot accumulate an unbounded backlog of instances.
    local deadline=$(( SECONDS + 60 ))

    check_deadline() {
        if (( SECONDS >= deadline )); then
            echo "fix_mount: 60-second deadline exceeded — giving up"
            return 1
        fi
        return 0
    }

    echo "Attempting to remount NFS mount $MOUNT_POINT"

    # --- Step 1: cheap remount (no disruption if the mount is merely stale) ---
    if mount -o remount -f "$MOUNT_POINT" 2>/dev/null; then
        echo "Remount succeeded for $MOUNT_POINT"
    else
        echo "Remount failed for $MOUNT_POINT — proceeding to full cycle"
    fi

    check_deadline || return 1

    # If the path is already a healthy mountpoint after remount, we are done.
    if mountpoint "$MOUNT_POINT" >/dev/null 2>&1; then
        echo "$MOUNT_POINT is still a valid mountpoint after remount; trying fresh mount"
    else
        echo "$MOUNT_POINT is not a valid mountpoint — attempting direct mount"
        if mount "$MOUNT_POINT" 2>/dev/null; then
            echo "Successfully mounted $MOUNT_POINT"
            MOUNT_FIXED=1
            return 0
        fi
        echo "Direct mount failed — proceeding to umount+remount cycle"
    fi

    check_deadline || return 1

    # --- Step 2: kill D-state processes so umount can detach cleanly ---
    kill_pinning_processes

    check_deadline || return 1

    # --- Step 3: force unmount ---
    echo "Attempting forced umount of $MOUNT_POINT"
    if umount -f "$MOUNT_POINT" 2>/dev/null; then
        echo "Force umount succeeded for $MOUNT_POINT"
    else
        echo "Force umount failed for $MOUNT_POINT — trying lazy umount"
        # --- Step 4: lazy umount detaches the VFS node even when processes
        # are still stuck, allowing a fresh mount to bind to a clean path ---
        if umount -l "$MOUNT_POINT" 2>/dev/null; then
            echo "Lazy umount succeeded for $MOUNT_POINT"
        else
            echo "Lazy umount also failed for $MOUNT_POINT — will still attempt mount"
        fi
    fi

    check_deadline || return 1

    # --- Step 5: restart stunnel to refresh the TLS transport ---
    # The most common root cause of mount hangs is a stale stunnel client
    # session (e.g. after a cluster-wide reboot or CARP failover). Restarting
    # stunnel tears down the old TCP connection and forces a fresh TLS
    # handshake before the mount call below.
    echo "Restarting stunnel to refresh TLS transport"
    if systemctl restart stunnel 2>/dev/null; then
        echo "stunnel restarted successfully"
    else
        echo "stunnel restart failed — mount may fail too"
    fi
    # Give stunnel two seconds to establish the new connection before mounting.
    sleep 2

    check_deadline || return 1

    # --- Step 6: fresh mount ---
    echo "Attempting to mount $MOUNT_POINT"
    if mount "$MOUNT_POINT" 2>/dev/null; then
        echo "NFS mount $MOUNT_POINT mounted successfully"
        MOUNT_FIXED=1
        return 0
    fi

    echo "Failed to fix NFS mount $MOUNT_POINT"
    return 1
}

if ! mountpoint "$MOUNT_POINT" >/dev/null 2>&1; then
    echo "NFS mount $MOUNT_POINT not found"
    fix_mount
fi

if ! timeout 2s stat "$MOUNT_POINT" >/dev/null 2>&1; then
    echo "NFS mount $MOUNT_POINT appears to be unresponsive"
    fix_mount
fi

# Write-probe: detect the "reads OK, writes hang" failure mode.
# A per-host filename prevents r0/r1/r2 from racing on the same file.
# Timeout of 5s covers one full NFS retransmit window (timeo=10 = 1s,
# retrans=2) plus margin, without making the 10-second cron run too long.
HEALTHCHECK_FILE="$MOUNT_POINT/.healthcheck.$(hostname)"
if ! timeout 5s sh -c "echo \$\$ > '$HEALTHCHECK_FILE' && rm -f '$HEALTHCHECK_FILE'" 2>/dev/null; then
    echo "NFS writes hanging on $MOUNT_POINT"
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
