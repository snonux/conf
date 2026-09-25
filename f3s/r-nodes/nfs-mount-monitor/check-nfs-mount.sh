#!/bin/bash
# NFS mount health monitor — runs every 10 seconds via systemd timer
# (nfs-mount-monitor.timer / nfs-mount-monitor.service)
#
# Checks whether /data/nfs/k3svolumes is mounted and responsive.
# Four probes run in order; the first failure triggers ONE repair per run:
#   1. /proc/self/mounts — detects a missing mount (no stat, so a stale mount
#      still counts as present)
#   2. stat        — detects read hangs / stale cache misses
#   3. stack check — more than one mount on the path (left by older versions
#      of this script, which mounted on top of a stale mount)
#   4. write-probe — detects the "reads OK, writes hang" failure mode
#      (stunnel-wrapped NFSv4 can enter a state where stat returns from
#      cache but ALL writes block; only the write probe catches this, well
#      inside the hard mount's 60s retransmit timeout)
#
# On a successful repair (mount back AND writable) it force-deletes pods on
# this node stuck in Unknown/Pending/ContainerCreating, and marks a restart of
# every NFS-backed workload on the node as pending: on the next run with all
# probes passing it rollout-restarts them (Recreate: stop before start), at
# most once per 2 minutes, retrying only the workloads that failed.
#
# fix_mount recovery sequence:
#   1. kill D-state processes pinning the mount (so umount can succeed)
#   2. unmount EVERY mount on the path (umount -f, then umount -l), and refuse
#      to continue if one is left: mounting on top of a leftover is how r0/r2
#      ended up with a soft mount stacked over a hard one (2026-09-25)
#   3. systemctl restart stunnel + 2s sleep (refresh the TLS transport)
#   4. mount with the fstab options (hard), bounded by timeout -k 5 30
#
# Step 4 used to mount soft,timeo=50 on the belief that a hard mount against
# an unreachable server wedges this script in D-state. The Linux NFS client
# waits in TASK_KILLABLE, so timeout's SIGKILL does end such a mount; and the
# soft repair mount silently switched every repaired node to soft semantics,
# where writes can fail with EIO under load. A mount that times out counts as
# a failed repair and feeds the reboot escalation below as before.
#
# fix_mount checks a 60-second deadline between its steps, and every step
# that can block on a dead server is itself bounded (umount and mount run
# under timeout -k); nfs-mount-monitor.service adds TimeoutStartSec as the
# backstop, so a wedged run is killed and the next timer tick starts fresh
# instead of the monitor stopping for good.
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
# Deploy via Gonf: ./gonf.sh cluster rocky-k3s rnodes_nfs_mount_monitor

MOUNT_POINT="/data/nfs/k3svolumes"
LOCK_FILE="/var/run/nfs-mount-check.lock"

# During a coordinated shutdown, the storage side may already be tearing
# down NFS/stunnel; probing or repairing the mount here would just fight
# that teardown (e.g. re-mounting NFS after the shutdown sequence has
# unmounted it, or burning the fail-count towards an unwanted reboot
# escalation). /dev/shm/shutdown_in_progress is created as an in-memory
# flag by the shutdown sequence, so it is naturally cleared on reboot.
SHUTDOWN_FLAG="/dev/shm/shutdown_in_progress"
if [ -f "$SHUTDOWN_FLAG" ]; then
    echo "Shutdown in progress ($SHUTDOWN_FLAG present) — skipping NFS mount check"
    exit 0
fi

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
# Minimum seconds between stale-NFS pod reaper passes (reap_stale_nfs_pods).
# The timer fires every 10s, but the reaper issues several kubectl calls, so we
# throttle it to limit API churn while still healing within ~one interval.
REAP_INTERVAL=30
# shellcheck source=/etc/default/nfs-mount-monitor
[ -f /etc/default/nfs-mount-monitor ] && . /etc/default/nfs-mount-monitor

# Use a lock file to prevent concurrent runs (timer fires every 10 s).
# If the lock is older than MAX_LOCK_AGE_SECS it was left by a run that was
# SIGKILL'd before its EXIT trap could clean up (e.g. systemd kills the
# process after its own timeout, bypassing the trap).  Remove the stale lock
# and continue rather than silently skipping all health checks forever.
MAX_LOCK_AGE_SECS=180  # above the unit's TimeoutStartSec=150 (worst-case run)
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
# Pending full NFS-workload restart after a repair (see
# restart_nfs_workloads_on_node), and its throttle.
RESTART_PENDING=/run/nfs-mount-monitor/restart-pending
RESTART_STAMP=/run/nfs-mount-monitor/last-workload-restart
RESTART_TRY_STAMP=/run/nfs-mount-monitor/last-workload-restart-attempt
# Storms are already prevented by restarting only on clean runs after a
# repair that proved writes work; the interval only spaces out full restarts
# after back-to-back repairs, and failed attempts retry after a short backoff.
RESTART_MIN_INTERVAL=120
RESTART_RETRY_INTERVAL=60

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
        # NFS waits show up as nfs_*, but also as the sunrpc/page-cache waits
        # underneath (rpc_wait_bit_killable, folio_wait_*); matching only
        # nfs_* missed most of them.
        [[ "$wchan" == nfs_* || "$wchan" == rpc_* || "$wchan" == folio_wait* ]] || continue

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

# reap_stale_nfs_pods — recreate pods on THIS node that are broken by a stale
# NFS bind-mount, even when the node-level mount itself is healthy.
#
# Why this exists separately from fix_mount:
#   fix_mount (and its post-repair stuck-pod cleanup) only run when one of the
#   node-level probes fails — i.e. when /data/nfs/k3svolumes is itself
#   missing/hung/unwritable.  But after a CARP failover / f-host reboot /
#   stunnel reconnect, the node can re-establish a perfectly healthy mount
#   while individual pods that existed during the brief transition keep a
#   STALE bind-mount to the old, detached NFS superblock.  Those pods are
#   invisible to fix_mount (node mount is fine, so MOUNT_FIXED is never set and
#   all node-level probes pass).  Observed real-world fallout of exactly this:
#     * git-server / prometheus: CreateContainerConfigError
#       "failed to prepare subPath for volumeMount ..." — kubelet cannot even
#       (re)create the container, and retries forever on the same node.
#     * immich-server: container is Running and Ready (its liveness probe is an
#       HTTP /ping that never touches the volume) yet every WRITE to the NFS
#       volume returns ESTALE (errno 116) — uploads fail with HTTP 500.
#   Recreating the pod fixes both, because a fresh pod sandbox / container
#   re-binds the now-healthy host mount.
#
# Two detection strategies, both conservative:
#   Case 1 (container cannot be created): visible in pod status.  We match the
#     waiting reason + a subPath/stale message, and only act on pods older than
#     a grace period so a normally slow first start is never reaped.
#   Case 2 (Running+Ready but writes fail): NOT visible in pod status, so it is
#     OPT-IN.  A pod is probed only if it carries the label
#     nfs.stale-restart/enabled=true ; we exec a tiny write probe into it (path
#     from annotation nfs.stale-restart/path, default /data) and reap only if
#     the probe fails twice in a row (guards against a transient exec hiccup).
reap_stale_nfs_pods() {
    command -v kubectl >/dev/null 2>&1 || return 0
    command -v jq >/dev/null 2>&1 || return 0
    export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
    local node
    node=$(hostname)

    # --- Case 1: container-create failures caused by a stale NFS subPath -----
    # Only pods older than CREATE_GRACE_SECS are eligible, so a pod that is
    # briefly in ContainerCreating during a normal start is never reaped.
    local create_grace=120
    local now
    now=$(date +%s)
    timeout 15 kubectl get pods -A --field-selector "spec.nodeName=$node" -o json 2>/dev/null \
      | jq -r --argjson now "$now" --argjson grace "$create_grace" '
          .items[]
          | select((.metadata.creationTimestamp | fromdateiso8601) < ($now - $grace))
          | . as $p
          | ((.status.containerStatuses // []) + (.status.initContainerStatuses // []))
          | map(select(
                ((.state.waiting.reason // "")
                   | test("CreateContainerConfigError|CreateContainerError|RunContainerError"))
                and ((.state.waiting.message // "")
                   | test("subPath|stale file handle"; "i"))
            ))
          | select(length > 0)
          | "\($p.metadata.namespace) \($p.metadata.name)"
        ' 2>/dev/null | sort -u | while read -r ns pod; do
        [ -n "$ns" ] || continue
        echo "reap_stale_nfs_pods: $ns/$pod stuck creating container (stale NFS subPath) — deleting"
        timeout 20 kubectl delete pod -n "$ns" "$pod" --grace-period=0 --force 2>&1
    done

    # --- Case 2: Running+Ready opt-in pods with a stale bind-mount -----------
    timeout 15 kubectl get pods -A \
        --field-selector "spec.nodeName=$node,status.phase=Running" \
        -l nfs.stale-restart/enabled=true -o json 2>/dev/null \
      | jq -r '
          .items[]
          | select((.status.conditions // []) | any(.type=="Ready" and .status=="True"))
          | "\(.metadata.namespace)\t\(.metadata.name)\t\(.metadata.annotations["nfs.stale-restart/path"] // "/data")"
        ' 2>/dev/null | while IFS=$'\t' read -r ns pod path; do
        [ -n "$ns" ] || continue
        # Probe file is per-node so concurrent r0/r1/r2 checks never collide.
        local probe="$path/.nfs-pod-probe.$node"
        # A stale handle makes the write fail immediately (ESTALE); a hung mount
        # makes it block, in which case the outer `timeout` kills kubectl exec
        # and we treat it as a failure — both should trigger a reap.
        if timeout 20 kubectl exec -n "$ns" "$pod" -- \
               sh -c "echo x > '$probe' && rm -f '$probe'" >/dev/null 2>&1; then
            continue
        fi
        sleep 2
        if timeout 20 kubectl exec -n "$ns" "$pod" -- \
               sh -c "echo x > '$probe' && rm -f '$probe'" >/dev/null 2>&1; then
            continue
        fi
        echo "reap_stale_nfs_pods: $ns/$pod write-probe to $path failed twice (stale NFS bind-mount) — deleting"
        timeout 20 kubectl delete pod -n "$ns" "$pod" --grace-period=0 --force 2>&1
    done
}

# nfs_mount_count — how many mounts are stacked on MOUNT_POINT, read from
# /proc/self/mounts. Deliberately not mountpoint(1) or findmnt: both stat the
# path, and a stat on a stale or hung NFS mount fails (ESTALE) or blocks, so
# "is it mounted?" was answered "no" for a mount that was still there. That is
# how a repair stacked a fresh mount on top of the old one (seen on r0/r2 on
# 2026-09-25).
nfs_mount_count() {
    awk -v mp="$MOUNT_POINT" '$2 == mp' /proc/self/mounts | wc -l
}

# detach_all_mounts — unmount every mount stacked on MOUNT_POINT: force first,
# lazy as a fallback. Returns non-zero if anything is still mounted, so the
# caller never mounts on top of a leftover.
detach_all_mounts() {
    local tries=0
    while (( $(nfs_mount_count) > 0 && tries < 3 )); do
        (( tries++ ))
        # Each try can take up to 2x15s when umount blocks; stop at fix_mount's
        # deadline rather than piling tries on a mount that will not let go.
        check_deadline || return 1
        # -i skips the umount.nfs4 helper, and timeout bounds the syscall's
        # path lookup, which can block on a dead NFS root.
        if timeout -k 5 10 umount -i -f "$MOUNT_POINT" 2>/dev/null; then
            echo "Force umount succeeded for $MOUNT_POINT"
        elif timeout -k 5 10 umount -i -l "$MOUNT_POINT" 2>/dev/null; then
            echo "Lazy umount succeeded for $MOUNT_POINT"
        else
            echo "umount -f and umount -l both failed for $MOUNT_POINT"
        fi
    done
    (( $(nfs_mount_count) == 0 ))
}

# restart_nfs_workloads_on_node NODE — rollout-restart every workload that
# has a pod on NODE whose volumes are NFS-backed (a hostPath under the mount
# point, or a PVC bound to a PV whose hostPath is under it). Such pods hold
# handles to the superblock a repair detached and stay broken even when they
# look Ready (Forgejo on r1, 2026-09-25: SQLite "bad file descriptor").
#
# A rollout restart rather than a pod delete: every stateful app here uses the
# Recreate strategy, so the old pod stops before the new one starts and a
# database (Postgres, SQLite) never has two writers on the same NFS
# directory; a plain delete would let the ReplicaSet start the replacement
# while the old pod still terminates. Pods without a Deployment/StatefulSet
# owner are deleted instead.
#
# Returns non-zero if anything could not be listed or restarted, so the
# caller keeps the restart-pending marker and retries next run.
restart_nfs_workloads_on_node() {
    local node="$1" d=/run/nfs-mount-monitor claims rc=0 kind ns name
    local -  # restores shell options on return
    set -o pipefail
    # 0700: the API dumps can contain literal env values of pod specs.
    mkdir -p "$d" && chmod 700 "$d"
    # A list left by an earlier, partly failed attempt is resumed as is:
    # only the workloads that were not restarted yet are retried.
    if [ -s "$d/restart-list" ]; then
        restart_list_entries "$d"
        return
    fi
    # The API objects go through files, not shell variables or --argjson:
    # the ReplicaSet list alone exceeds the kernel's argument-size limit.
    timeout 20 kubectl get pv -o json > "$d/pv.json" 2>/dev/null || return 1
    timeout 20 kubectl get pods --all-namespaces --field-selector="spec.nodeName=$node" \
        -o json > "$d/pods.json" 2>/dev/null || return 1
    timeout 20 kubectl get replicasets --all-namespaces -o json > "$d/rs.json" 2>/dev/null || return 1
    claims=$(jq -c --arg mp "$MOUNT_POINT" '
        [.items[] | select((.spec.hostPath.path // "") | startswith($mp))
         | .spec.claimRef | select(. != null) | "\(.namespace)/\(.name)"]' "$d/pv.json") || return 1
    if [ "$claims" = "[]" ]; then
        echo "restart_nfs_workloads_on_node: no NFS PVs found — refusing to guess, will retry"
        return 1
    fi
    # One line per workload: "<kind> <namespace> <name>", deduplicated.
    jq -r --arg mp "$MOUNT_POINT" --argjson claims "$claims" --slurpfile rs "$d/rs.json" '
        ($rs[0].items | map({key: "\(.metadata.namespace)/\(.metadata.name)",
                          value: ((.metadata.ownerReferences // [])[0])}) | from_entries) as $rsowner
        | .items[] | . as $p
        | select(any(.spec.volumes[]?;
              ((.hostPath.path // "") | startswith($mp))
              or ((.persistentVolumeClaim.claimName // null) as $c
                  | $c != null and ($claims | index("\($p.metadata.namespace)/\($c)")) != null)))
        | ((.metadata.ownerReferences // [])[0]) as $o
        | if $o == null then "pod \(.metadata.namespace) \(.metadata.name)"
          elif $o.kind == "ReplicaSet" then
            ($rsowner["\(.metadata.namespace)/\($o.name)"]) as $d
            | if $d != null and $d.kind == "Deployment" then "deployment \(.metadata.namespace) \($d.name)"
              else "pod \(.metadata.namespace) \(.metadata.name)" end
          elif $o.kind == "StatefulSet" then "statefulset \(.metadata.namespace) \($o.name)"
          else "pod \(.metadata.namespace) \(.metadata.name)" end' "$d/pods.json" | sort -u > "$d/restart-list.tmp" \
        || { rm -f "$d/restart-list.tmp"; return 1; }
    rm -f "$d/pv.json" "$d/pods.json" "$d/rs.json"
    mv "$d/restart-list.tmp" "$d/restart-list"
    restart_list_entries "$d"
}

# restart_list_entries DIR — restart every workload in DIR/restart-list and
# keep only the entries that failed; returns non-zero while any are left.
restart_list_entries() {
    local d="$1" rc=0 kind ns name
    : > "$d/restart-list.left"
    while read -r kind ns name; do
        [ -n "$kind" ] || continue
        if [ "$kind" = pod ]; then
            echo "Deleting NFS pod $ns/$name (no Deployment/StatefulSet owner)"
            timeout 20 kubectl delete pod -n "$ns" "$name" --wait=false 2>&1 && continue
        else
            echo "Rollout-restarting $kind $ns/$name (stale NFS handles after a repair)"
            timeout 20 kubectl rollout restart "$kind" -n "$ns" "$name" 2>&1 && continue
        fi
        echo "$kind $ns $name" >> "$d/restart-list.left"
        rc=1
    done < "$d/restart-list"
    mv "$d/restart-list.left" "$d/restart-list"
    [ -s "$d/restart-list" ] && rc=1
    [ "$rc" -eq 0 ] && rm -f "$d/restart-list"
    return $rc
}

fix_mount () {
    # Deadline checked between (and inside the umount loop of) the steps
    # below. A step started just before it can still run to its own bound, so
    # the worst case is ~60s plus one bounded step (<=35s for the mount),
    # comfortably inside the unit's TimeoutStartSec.
    local deadline=$(( SECONDS + 60 ))

    check_deadline() {
        if (( SECONDS >= deadline )); then
            echo "fix_mount: 60-second deadline exceeded — giving up"
            return 1
        fi
        return 0
    }

    echo "Attempting to repair NFS mount $MOUNT_POINT ($(nfs_mount_count) mount(s) present)"

    # --- Step 1: kill D-state processes so umount can detach cleanly ---
    kill_pinning_processes

    check_deadline || return 1

    # --- Step 2: detach every mount on the path. Never mount on top of a
    # leftover: a stacked mount hides the old one without releasing it, and
    # pods keep writing through whichever layer they bound. ---
    if ! detach_all_mounts; then
        echo "Refusing to mount: $(nfs_mount_count) mount(s) still on $MOUNT_POINT"
        return 1
    fi

    check_deadline || return 1

    # --- Step 3: restart stunnel to refresh the TLS transport ---
    # The most common root cause of mount hangs is a stale stunnel client
    # session (e.g. after a cluster-wide reboot or CARP failover). Restarting
    # stunnel tears down the old TCP connection and forces a fresh TLS
    # handshake before the mount call below.
    echo "Restarting stunnel to refresh TLS transport"
    if timeout 20 systemctl restart stunnel 2>/dev/null; then
        echo "stunnel restarted successfully"
    else
        echo "stunnel restart failed — mount may fail too"
    fi
    # Give stunnel two seconds to establish the new connection before mounting.
    sleep 2

    check_deadline || return 1

    # --- Step 4: fresh mount with the fstab options (hard) ---
    # The repair used to mount soft,timeo=50 so a dead server could not wedge
    # this script, which left every repaired node silently on a soft mount
    # (writes can fail with EIO under load) while fresh boots were hard. Now
    # it uses fstab's options like a boot does, bounded by timeout -k: the
    # Linux NFS client waits in TASK_KILLABLE, so the SIGKILL does end a mount
    # stuck on an unreachable server, and a failed mount counts toward the
    # reboot escalation as before.
    echo "Attempting to mount $MOUNT_POINT"
    # Success means the mount is back AND writable: a stat alone passed on an
    # export that still could not write, and every such "success" would reset
    # the fail counter and trigger the workload restarts below.
    if timeout -k 5 30 mount "$MOUNT_POINT" 2>/dev/null \
        && timeout 5s stat "$MOUNT_POINT" >/dev/null 2>&1 \
        && timeout 5s sh -c "echo \$\$ > '$MOUNT_POINT/.healthcheck.$(hostname)' && rm -f '$MOUNT_POINT/.healthcheck.$(hostname)'" 2>/dev/null; then
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
    if timeout 20 kubectl cordon "$node" 2>&1; then
        echo "Node $node cordoned successfully"
    else
        echo "kubectl cordon failed (will reboot anyway)"
    fi

    # systemd-journald flushes on SIGTERM, which systemctl reboot sends to
    # all services before the node goes down — the message above will survive.
    # Reset the counter first: it lives in /var/lib and survives the reboot,
    # and a count still at the threshold would make the first failing probe
    # after boot escalate again without a single repair attempt -- r0-r2
    # rebooting in a loop (and dropping etcd quorum) through a long storage
    # outage.
    write_fail_count 0
    echo "Initiating systemctl reboot to recover broken NFS mount"
    systemctl reboot
}

# run_fix_mount_with_counter — call fix_mount and update the consecutive-
# failure counter.  On success, counter is reset to 0.  On failure, counter
# is incremented; if it reaches NFS_FAIL_THRESHOLD, escalate_reboot is called.
run_fix_mount_with_counter() {
    # Count the attempt as a failure BEFORE running it and undo that on
    # success: if systemd kills a wedged run at TimeoutStartSec, the failure
    # is already recorded, so repeated wedged runs still reach the reboot
    # escalation instead of silently never counting.
    local pre
    pre=$(read_fail_count)
    if (( pre >= NFS_FAIL_THRESHOLD )); then
        # Earlier attempts in this boot already used up the budget without
        # escalating, which only happens when they were killed mid-repair
        # (wedged): escalate_reboot resets the counter before rebooting.
        escalate_reboot
        return
    fi
    write_fail_count $(( pre + 1 ))
    if fix_mount; then
        # Repair succeeded — reset the failure streak.
        write_fail_count 0
        echo "NFS repair succeeded; consecutive-failure counter reset to 0"
    else
        # Repair failed — increment the counter and check the threshold.
        # The pre-increment above already recorded this failure.
        local count
        count=$(read_fail_count)
        echo "NFS repair failed; consecutive failures: $count / $NFS_FAIL_THRESHOLD"

        if (( count >= NFS_FAIL_THRESHOLD )); then
            escalate_reboot
        fi
    fi
}

# PROBE_FAILED tracks whether any probe fired run_fix_mount_with_counter.
# If no probe fires, all checks passed cleanly and we can reset the counter.
PROBE_FAILED=0
# WRITE_PENDING marks a stat or write stall still below WRITE_FAIL_LIMIT: no
# repair yet, but not a clean run either, so the fail counter is left alone.
WRITE_PENDING=0

# The probes run in order and stop at the first failure: one repair per run.
# Each used to call fix_mount on its own, so a single outage could cycle the
# mount (and restart stunnel) three times in one run.
#
# A missing or stacked mount is repaired at once. A stalled stat or failed
# write probe only counts toward WRITE_FAIL_LIMIT consecutive runs first
# (streak kept in WRITE_FAIL_FILE for both): the repair's
# umount -f aborts every in-flight RPC on the superblock, handing EIO to
# every pod on the node even on a hard mount, which is worse than a single
# 5-second stall that the hard mount would have ridden out.
WRITE_FAIL_LIMIT=${WRITE_FAIL_LIMIT:-2}
# In /run, not STATE_DIR: a streak must be consecutive runs, and a stall left
# over from before a reboot says nothing about the mount after it.
WRITE_FAIL_FILE=/run/nfs-mount-monitor/write-fail-streak
read_write_streak() {
    local n=0
    [ -f "$WRITE_FAIL_FILE" ] && n=$(< "$WRITE_FAIL_FILE")
    [[ "$n" =~ ^[0-9]+$ ]] || n=0
    echo "$n"
}
HEALTHCHECK_FILE="$MOUNT_POINT/.healthcheck.$(hostname)"
if (( $(nfs_mount_count) == 0 )); then
    echo "NFS mount $MOUNT_POINT not found"
    PROBE_FAILED=1
elif ! timeout 2s stat "$MOUNT_POINT" >/dev/null 2>&1; then
    # A stat stall gets the same grace as a write stall (see
    # WRITE_FAIL_LIMIT): on 2026-09-25 23:23 one 2s stall on r1 triggered an
    # immediate umount -f and left every NFS pod on the node with stale
    # handles (Forgejo's SQLite went "bad file descriptor").
    streak=$(( $(read_write_streak) + 1 ))
    mkdir -p "$(dirname "$WRITE_FAIL_FILE")"
    echo "$streak" > "$WRITE_FAIL_FILE"
    if (( streak >= WRITE_FAIL_LIMIT )); then
        echo "NFS mount $MOUNT_POINT unresponsive ($streak consecutive runs)"
        PROBE_FAILED=1
    else
        echo "NFS stat probe stalled on $MOUNT_POINT ($streak/$WRITE_FAIL_LIMIT) — waiting for the next run"
        WRITE_PENDING=1
    fi
elif (( $(nfs_mount_count) > 1 )); then
    # A stacked mount left by an older version of this script: repair it so
    # the node ends up with exactly one (hard) mount.
    echo "NFS mount $MOUNT_POINT is stacked ($(nfs_mount_count) mounts)"
    PROBE_FAILED=1
# Write-probe: detect the "reads OK, writes hang" failure mode. A per-host
# filename prevents r0/r1/r2 from racing on the same file. 5s is far below
# the hard mount's retransmit timeout (timeo=600 = 60s), so a stalled server
# is caught here rather than after a minute.
elif ! timeout 5s sh -c "echo \$\$ > '$HEALTHCHECK_FILE' && rm -f '$HEALTHCHECK_FILE'" 2>/dev/null; then
    streak=$(( $(read_write_streak) + 1 ))
    mkdir -p "$(dirname "$WRITE_FAIL_FILE")"
    echo "$streak" > "$WRITE_FAIL_FILE"
    if (( streak >= WRITE_FAIL_LIMIT )); then
        echo "NFS writes hanging on $MOUNT_POINT ($streak consecutive runs)"
        PROBE_FAILED=1
    else
        echo "NFS write probe failed on $MOUNT_POINT ($streak/$WRITE_FAIL_LIMIT) — waiting for the next run"
        WRITE_PENDING=1
    fi
else
    rm -f "$WRITE_FAIL_FILE"
fi

if [ "$PROBE_FAILED" -eq 1 ]; then
    rm -f "$WRITE_FAIL_FILE"
    run_fix_mount_with_counter
fi

# If every probe passed cleanly (no repair attempt needed), reset the
# consecutive-failure counter so a previous partial failure streak does not
# lower the effective reboot threshold.  write_fail_count also refreshes the
# textfile metric so Prometheus always has a current sample.
if [ "$WRITE_PENDING" -eq 1 ]; then
    # Not clean, not repaired: keep the counter, but refresh the metric so
    # node_exporter never serves a stale sample.
    write_textfile_metric "$(read_fail_count)"
elif [ "$PROBE_FAILED" -eq 0 ]; then
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
    # Only pods that never got a running container are force-deleted here.
    # Running-but-NotReady pods (a database stuck on a stale handle) are
    # left to the rollout restart below: a force delete lets the ReplicaSet
    # start the replacement while the old container may still write through
    # the detached mount, i.e. two writers on one Postgres/SQLite directory.
    echo "Mount was fixed, checking for stuck pods on this node..."
    NODE=$(hostname)
    export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
    timeout 20 kubectl get pods --all-namespaces --field-selector="spec.nodeName=$NODE" \
      -o json 2>/dev/null | jq -r '
        .items[] |
        select(
          .status.phase == "Unknown" or
          .status.phase == "Pending" or
          (.status.containerStatuses // [] | any(.state.waiting.reason == "ContainerCreating"))
        ) | "\(.metadata.namespace) \(.metadata.name)"' | \
      while read ns pod; do
        echo "Deleting stuck pod $ns/$pod"
        timeout 20 kubectl delete pod -n "$ns" "$pod" --grace-period=0 --force 2>&1
      done

    # Every pod on this node that mounts the NFS export now holds handles
    # to the superblock the repair detached: its bind mounts are stale even
    # though it may look Ready (Forgejo on r1, 2026-09-25: SQLite "bad file
    # descriptor", 500s, while its HTTP health check passed). Restart them
    # all so they re-bind the fresh mount; they may land on any node.
    # The workload restart itself runs below, on a run where the mount is
    # healthy; the marker survives a run killed mid-way (TimeoutStartSec)
    # and a failed kubectl call, so the restart is retried until it succeeds.
    mkdir -p /run/nfs-mount-monitor && chmod 700 /run/nfs-mount-monitor
    rm -f /run/nfs-mount-monitor/restart-list  # a new repair: recompute
    touch "$RESTART_PENDING"

    # On a healthy remount, also ensure the fail counter is reset.
    write_fail_count 0
    echo "Stuck-pod cleanup done; consecutive-failure counter reset to 0"
fi

# Restart the NFS workloads on this node once after a repair (see
# restart_nfs_workloads_on_node). Only on a run whose probes all passed, so a
# node whose export still cannot write is not put into a restart loop, and at
# most once per RESTART_MIN_INTERVAL even if repairs keep succeeding.
if [ -e "$RESTART_PENDING" ] && [ "$PROBE_FAILED" -eq 0 ] && [ "$WRITE_PENDING" -eq 0 ]; then
    now=$(date +%s); last_ok=0; last_try=0
    [ -f "$RESTART_STAMP" ] && last_ok=$(stat -c %Y "$RESTART_STAMP" 2>/dev/null || echo 0)
    [ -f "$RESTART_TRY_STAMP" ] && last_try=$(stat -c %Y "$RESTART_TRY_STAMP" 2>/dev/null || echo 0)
    if (( now - last_ok < RESTART_MIN_INTERVAL )); then
        echo "NFS workload restart pending but throttled (last full restart $(( now - last_ok ))s ago)"
    elif (( now - last_try < RESTART_RETRY_INTERVAL )); then
        echo "NFS workload restart pending; retrying after backoff"
    else
        export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
        touch "$RESTART_TRY_STAMP"
        if restart_nfs_workloads_on_node "$(hostname)"; then
            rm -f "$RESTART_PENDING"
            touch "$RESTART_STAMP"
            echo "NFS workload restart done"
        else
            echo "NFS workload restart incomplete — will retry the rest"
        fi
    fi
fi

# Reap pods broken by a stale NFS bind-mount even when the node-level mount is
# healthy (the failure mode fix_mount cannot see).  Throttled to REAP_INTERVAL
# seconds because, unlike the probes above, it issues kubectl calls on every
# pass regardless of mount health.
REAP_STAMP="$STATE_DIR/last-reap"
last_reap=0
[ -f "$REAP_STAMP" ] && last_reap=$(stat -c %Y "$REAP_STAMP" 2>/dev/null || echo 0)
# Only on a clean run or right after a successful repair: while the node's own
# mount is failing or a write stall is pending, every opt-in pod's write probe
# fails too, and reaping them would be the very disruption the WRITE_PENDING
# grace run avoids (and each probe costs up to ~42s of this run).
if { { [ "$PROBE_FAILED" -eq 0 ] && [ "$WRITE_PENDING" -eq 0 ]; } || [ "$MOUNT_FIXED" -eq 1 ]; } \
    && (( $(date +%s) - last_reap >= REAP_INTERVAL )); then
    mkdir -p "$STATE_DIR"
    touch "$REAP_STAMP"
    reap_stale_nfs_pods
fi
