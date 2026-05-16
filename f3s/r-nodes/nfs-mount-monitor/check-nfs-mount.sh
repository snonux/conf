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
#   6. mount with soft+short-timeo (NFS-specific; avoids infinite kernel D-state)
#
# Step 6 uses explicit soft NFS options instead of `mount $MOUNT_POINT`
# (which would read the fstab `hard` flag).  A `hard` NFS mount that fails
# to reach the server enters uninterruptible kernel sleep (D-state), and
# SIGKILL cannot wake a D-state process on Linux — the script would hang
# indefinitely, leaving the lock file stale.  With `soft,timeo=50,retrans=3`
# the kernel gives up after ~15s and returns ETIMEDOUT, allowing the fail
# counter to increment and eventually trigger the reboot escalation.
#
# A hard 60-second deadline is enforced so the function can never outlast
# its own timer interval (10s) by more than 6x, preventing timer pile-up.
#
# Consecutive-failure escalation:
#   Each fix_mount failure increments a counter persisted to
#   /var/lib/nfs-mount-monitor/fail-count.  A successful repair resets
#   the counter to 0.  When the counter reaches NFS_FAIL_THRESHOLD (default
#   5, configurable via /etc/default/nfs-mount-monitor), the node is cordoned
#   via kubectl so the scheduler stops placing new pods here, a loud message
#   is written to the journal, and 'systemctl reboot' is issued.
#   With the timer firing every 10s, threshold=5 means ~50s of continuously
#   broken NFS before an auto-reboot — safe because r0/r1/r2 form an HA
#   cluster and a Rocky Linux VM reboots in ~30s.
#
# Deploy via Rex: rex -f f3s/r-nodes/Rexfile nfs_mount_monitor

MOUNT_POINT="/data/nfs/k3svolumes"
LOCK_FILE="/var/run/nfs-mount-check.lock"

# State directory for the fail counter; created if absent.
STATE_DIR="/var/lib/nfs-mount-monitor"
FAIL_COUNT_FILE="$STATE_DIR/fail-count"

# Textfile collector output for node_exporter.
# Written on every run so Prometheus always has a current sample.
# The DaemonSet mounts /var/lib/node_exporter/textfile_collector from the host.
TEXTFILE_DIR="/var/lib/node_exporter/textfile_collector"
TEXTFILE_PROM="$TEXTFILE_DIR/nfs_mount_monitor.prom"

# Load tunable configuration (NFS_FAIL_THRESHOLD) from the EnvironmentFile
# deployed alongside this script.  Defaults are defined here so the script
# works even if the file is absent.
NFS_FAIL_THRESHOLD=5
# shellcheck source=/etc/default/nfs-mount-monitor
[ -f /etc/default/nfs-mount-monitor ] && . /etc/default/nfs-mount-monitor

# Use a lock file to prevent concurrent runs (timer fires every 10 s).
# If the lock is older than MAX_LOCK_AGE_SECS it was left by a run that was
# SIGKILL'd before its EXIT trap could clean up (e.g. systemd kills the
# process after its own timeout, bypassing the trap).  Remove the stale lock
# and continue rather than silently skipping all health checks forever.
MAX_LOCK_AGE_SECS=90
if [ -f "$LOCK_FILE" ]; then
    lock_age=$(( $(date +%s) - $(stat -c %Y "$LOCK_FILE") ))
    if (( lock_age < MAX_LOCK_AGE_SECS )); then
        exit 0
    fi
    echo "Stale lock file detected (age=${lock_age}s > ${MAX_LOCK_AGE_SECS}s) — removing and continuing"
    rm -f "$LOCK_FILE"
fi
touch "$LOCK_FILE"
trap "rm -f $LOCK_FILE" EXIT

MOUNT_FIXED=0

# read_fail_count — return the current consecutive-failure counter.
# Returns 0 if the file is absent or contains a non-integer.
read_fail_count() {
    local count=0
    if [ -f "$FAIL_COUNT_FILE" ]; then
        count=$(< "$FAIL_COUNT_FILE")
        # Guard against corrupt file contents
        [[ "$count" =~ ^[0-9]+$ ]] || count=0
    fi
    echo "$count"
}

# write_fail_count — persist COUNT to the state file, creating the
# directory if it does not yet exist.
write_fail_count() {
    local count="$1"
    mkdir -p "$STATE_DIR"
    echo "$count" > "$FAIL_COUNT_FILE"
    # Also export the current count to the node_exporter textfile collector
    # so Prometheus can alert directly without parsing journal logs.
    write_textfile_metric "$count"
}

# write_textfile_metric — write the consecutive-failure gauge to the
# node_exporter textfile_collector directory.  The metric name follows the
# node_exporter convention: lowercase, underscores, no units suffix for counts.
# The host label lets Prometheus distinguish r0/r1/r2 even before
# relabelling resolves the instance IP to a hostname.
# We write atomically (tmp + mv) to avoid node_exporter reading a partial file.
write_textfile_metric() {
    local count="$1"
    local host
    host=$(hostname -s)
    mkdir -p "$TEXTFILE_DIR"
    local tmp_file
    tmp_file="$(mktemp "$TEXTFILE_DIR/nfs_mount_monitor.prom.XXXXXX")"
    # Write metric with HELP/TYPE headers for valid exposition format
    printf '# HELP nfs_mount_monitor_consecutive_failures Consecutive NFS fix_mount failure count\n' > "$tmp_file"
    printf '# TYPE nfs_mount_monitor_consecutive_failures gauge\n' >> "$tmp_file"
    printf 'nfs_mount_monitor_consecutive_failures{host="%s"} %s\n' "$host" "$count" >> "$tmp_file"
    # Make the file world-readable so node_exporter (uid 65534) can scrape it.
    chmod 644 "$tmp_file"
    mv "$tmp_file" "$TEXTFILE_PROM"
}

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
        if mount -t nfs4 -o port=2323,soft,timeo=50,retrans=3 \
               127.0.0.1:/k3svolumes "$MOUNT_POINT" 2>/dev/null; then
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

    # --- Step 6: fresh mount (soft options — see top comment for rationale) ---
    echo "Attempting to mount $MOUNT_POINT"
    if mount -t nfs4 -o port=2323,soft,timeo=50,retrans=3 \
           127.0.0.1:/k3svolumes "$MOUNT_POINT" 2>/dev/null; then
        echo "NFS mount $MOUNT_POINT mounted successfully"
        MOUNT_FIXED=1
        return 0
    fi

    echo "Failed to fix NFS mount $MOUNT_POINT"
    return 1
}

# escalate_reboot — cordon the k3s node so the scheduler stops placing new
# pods here, log loudly to the journal, then trigger a clean reboot.
# Called only after NFS_FAIL_THRESHOLD consecutive fix_mount failures.
escalate_reboot() {
    local node
    node=$(hostname)
    export KUBECONFIG=/etc/rancher/k3s/k3s.yaml

    echo "CRITICAL: NFS mount $MOUNT_POINT has failed $NFS_FAIL_THRESHOLD" \
         "consecutive repair attempts — escalating to reboot"

    # Cordon the node so the scheduler will not place new pods here while
    # the reboot is in progress.  Failure to cordon is non-fatal: we still
    # reboot because a broken NFS node is worse than an uncordoned one.
    if kubectl cordon "$node" 2>&1; then
        echo "Node $node cordoned successfully"
    else
        echo "kubectl cordon failed (will reboot anyway)"
    fi

    # systemd-journald flushes on SIGTERM, which systemctl reboot sends to
    # all services before the node goes down — the message above will survive.
    echo "Initiating systemctl reboot to recover broken NFS mount"
    systemctl reboot
}

# run_fix_mount_with_counter — call fix_mount and update the consecutive-
# failure counter.  On success, counter is reset to 0.  On failure, counter
# is incremented; if it reaches NFS_FAIL_THRESHOLD, escalate_reboot is called.
run_fix_mount_with_counter() {
    if fix_mount; then
        # Repair succeeded — reset the failure streak.
        write_fail_count 0
        echo "NFS repair succeeded; consecutive-failure counter reset to 0"
    else
        # Repair failed — increment the counter and check the threshold.
        local count
        count=$(read_fail_count)
        (( count++ ))
        write_fail_count "$count"
        echo "NFS repair failed; consecutive failures: $count / $NFS_FAIL_THRESHOLD"

        if (( count >= NFS_FAIL_THRESHOLD )); then
            escalate_reboot
        fi
    fi
}

# PROBE_FAILED tracks whether any probe fired run_fix_mount_with_counter.
# If no probe fires, all checks passed cleanly and we can reset the counter.
PROBE_FAILED=0

if ! mountpoint "$MOUNT_POINT" >/dev/null 2>&1; then
    echo "NFS mount $MOUNT_POINT not found"
    run_fix_mount_with_counter
    PROBE_FAILED=1
fi

if ! timeout 2s stat "$MOUNT_POINT" >/dev/null 2>&1; then
    echo "NFS mount $MOUNT_POINT appears to be unresponsive"
    run_fix_mount_with_counter
    PROBE_FAILED=1
fi

# Write-probe: detect the "reads OK, writes hang" failure mode.
# A per-host filename prevents r0/r1/r2 from racing on the same file.
# Timeout of 5s covers one full NFS retransmit window (timeo=10 = 1s,
# retrans=2) plus margin, without making the 10-second cron run too long.
HEALTHCHECK_FILE="$MOUNT_POINT/.healthcheck.$(hostname)"
if ! timeout 5s sh -c "echo \$\$ > '$HEALTHCHECK_FILE' && rm -f '$HEALTHCHECK_FILE'" 2>/dev/null; then
    echo "NFS writes hanging on $MOUNT_POINT"
    run_fix_mount_with_counter
    PROBE_FAILED=1
fi

# If all three probes passed cleanly (no repair attempt needed), reset the
# consecutive-failure counter so a previous partial failure streak does not
# lower the effective reboot threshold.  write_fail_count also refreshes the
# textfile metric so Prometheus always has a current sample.
if [ "$PROBE_FAILED" -eq 0 ]; then
    if [ "$(read_fail_count)" -ne 0 ]; then
        write_fail_count 0
        echo "All probes passed; consecutive-failure counter reset to 0"
    else
        # Counter is already zero; update the textfile metric timestamp
        # so node_exporter sees a fresh scrape on every healthy run.
        write_textfile_metric 0
    fi
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

    # On a healthy remount, also ensure the fail counter is reset.
    write_fail_count 0
    echo "Stuck-pod cleanup done; consecutive-failure counter reset to 0"
fi
