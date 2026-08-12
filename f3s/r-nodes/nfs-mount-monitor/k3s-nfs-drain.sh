#!/bin/bash
# Keep the NFS transport alive until the pods have let go of it, then detach
# what they left behind. Runs as the ExecStop of k3s-nfs-drain.service.
#
# The problem it solves, measured on 2026-08-11:
#
#   22:39:43 r2 systemd-logind: Power key pressed short.
#   22:39:44 r2 stunnel: LOG5[main]: Terminated          <- one second in
#   22:44:12 r0 kernel: nfs: server 127.0.0.1 not responding, timed out (x30+)
#   22:42:43 r2 systemd: cri-containerd-....scope: Still around after SIGKILL.
#
# stunnel carries every NFS mount on this node (127.0.0.1:2323 -> the CARP
# storage VIP), and systemd stopped it one second into the shutdown -- while
# roughly sixty containers were still running and still writing to NFS.
# Their mounts are `hard`, so each blocked I/O retried instead of failing,
# every container took its full 90-second SIGKILL timeout to die, and the
# guest needed close to four minutes to power off. bhyve's bounded stop then
# SIGKILLed the whole VM at 240s, which is what put "check etcd health on the
# next boot" in the f3sctl job log.
#
# Why the existing ordering drop-ins did not cover this:
#
#   * data-nfs-k3svolumes.mount is After=stunnel.service, so THAT mount is
#     detached before stunnel goes -- and it duly was, at 22:39:44.
#   * k3s.service is After=data-nfs-k3svolumes.mount, so k3s stops first.
#
# Both hold, and both are beside the point. k3s.service runs with
# KillMode=process, so "k3s.service has stopped" means the k3s process is
# gone, NOT that the pods are: containerd's shims and the cri-containerd-*
# scopes systemd created for them are separate units, ordered against nothing,
# and they kept running for another three minutes. Nor is the fstab mount the
# one the pods use -- kubelet makes its own, one per volume, invisible to
# systemd:
#
#   127.0.0.1:/k3svolumes/prometheus/data/prometheus-db
#       on /var/lib/kubelet/pods/<uid>/volume-subpaths/prometheus-data-pv/...
#
# So this script waits for the shims to actually exit, and only then lets
# stunnel be stopped (systemd blocks stunnel's stop job until this ExecStop
# returns -- see the ordering comment in k3s-nfs-drain.service). Anything
# still mounted at that point is detached here, so the final
# systemd-shutdown sweep does not meet an NFS mount whose transport is about
# to disappear.
#
# Waiting alone is not enough, which the first version of this script proved
# on 2026-08-11: it waited passively for systemd's own scope teardown, that
# teardown did not finish inside sixty seconds, and the drain then detached
# NFS from under containers that were still running. Their mounts went lazy
# (busy), so dirty pages could never be flushed, and systemd-shutdown paid for
# it afterwards -- "Syncing filesystems and block devices - timed out, issuing
# SIGKILL" on r0, then minutes of unkillable D-state processes before the VM
# finally powered off. The guest shutdown got *worse*: f1 went from 10s to
# 4m9s.
#
# So the grace period is short, and what follows it is decisive: kill what is
# left through its cgroup, then unmount for real. Every one of those steps
# still happens while the tunnel is up, which is what makes the unmounts
# complete instead of hang.
#
# SIGKILLing a container is not gentle, but the comparison is not with a
# graceful stop -- systemd has already SIGTERMed these scopes and they have
# had their grace. It is with what happens otherwise: bhyve SIGKILLing the
# entire VM at 240s, which cuts power under the OS as well as the containers.
# Killing the process does not discard what it already wrote; the page cache
# is flushed by the unmount below, while NFS still answers.

set -u

# GRACE_TIMEOUT is how long containers get to exit on their own once NFS is
# answering again, before the kill below.
#
# Thirty seconds because that is Kubernetes' own default
# terminationGracePeriodSeconds: a pod that asks for the default grace should
# get all of it, and killing at twenty would cut every such pod ten seconds
# short of what it was promised. It is paid once per k3s guest per shutdown,
# and since the guests now go down in parallel it costs the rack thirty
# seconds in total, not thirty per host.
#
# A pod configured with a longer grace than this does not get it. That is
# deliberate: the budget for the whole guest is 240s (f3sctl's agent), and
# overrunning it means bhyve SIGKILLs the VM, which is worse for that pod
# than being killed here with its filesystem still healthy.
GRACE_TIMEOUT="${GRACE_TIMEOUT:-30}"

# KUBEPODS_SLICE is the cgroup every pod's container scope lives under, and
# killing it is how this script stops "the containers" without walking
# process trees the way k3s-killall.sh does. The cgroup is authoritative:
# nothing escapes it, and there is no race with a shim reparenting.
#
# k3s-killall.sh is deliberately not called here even though it would kill the
# same processes. It also deletes cni0/flannel interfaces, flushes iptables
# and removes /var/lib/cni -- none of which helps a shutdown, all of which is
# rebuilt at boot anyway, and each of which is one more thing that can go
# wrong on the path this unit exists to keep clear.
KUBEPODS_SLICE="${KUBEPODS_SLICE:-kubepods.slice}"

log() { logger -t k3s-nfs-drain -- "$@"; }

# shims_running reports whether any containerd shim is still alive. The shim
# is the parent of the container's processes, so it outliving the teardown is
# the honest signal that pods still hold their mounts.
shims_running() {
    pgrep -f 'containerd-shim' >/dev/null 2>&1
}

# nfs_mounts lists this node's NFS mount points, deepest first so a nested
# mount is never in the way of its parent.
#
# Read from /proc/self/mounts rather than mount(8): it cannot itself block on
# an unresponsive server, which is exactly the state this script exists for.
nfs_mounts() {
    awk '$3 ~ /^nfs/ { print $2 }' /proc/self/mounts | sort -r
}

wait_for_pods() {
    local started deadline
    started=$(date +%s)
    deadline=$(( started + GRACE_TIMEOUT ))

    if ! shims_running; then
        log "no containers left; nothing to drain"
        return 0
    fi

    log "waiting up to ${GRACE_TIMEOUT}s for containers to release their NFS mounts"
    while [[ $(date +%s) -lt $deadline ]]; do
        if ! shims_running; then
            log "all containers gone after $(( $(date +%s) - started ))s"
            return 0
        fi
        sleep 2
    done

    return 1
}

# kill_pods stops whatever outlasted the grace period, by cgroup.
#
# --kill-whom=all reaches every process in the slice, not just each scope's
# "main" one -- a container's main process exiting while its children linger
# is precisely the state that leaves an NFS mount busy.
#
# Failure is logged and tolerated: if the slice is already gone (the ordinary
# case on a healthy node) systemctl says so and there is nothing to do, and
# if it is not, the unmount below still has -f and -l to fall back on.
kill_pods() {
    if ! shims_running; then
        return 0
    fi

    log "WARNING: containers still running after ${GRACE_TIMEOUT}s; killing ${KUBEPODS_SLICE}"
    if ! systemctl kill --kill-whom=all --signal=SIGKILL "$KUBEPODS_SLICE" 2>/dev/null; then
        log "WARNING: could not kill ${KUBEPODS_SLICE} through systemd"
    fi

    # The kill is asynchronous; give the kernel a moment to reap the
    # processes so their mounts are no longer busy when the unmount runs.
    local deadline=$(( $(date +%s) + 10 ))
    while [[ $(date +%s) -lt $deadline ]] && shims_running; do
        sleep 1
    done
}

# detach_nfs unmounts what is left, in increasing order of violence.
#
# A plain umount is tried first because it is the only one that flushes. -f
# then gives up on an unresponsive server, and -l detaches the VFS node when
# even that is refused -- the mount is then gone from the namespace, which is
# all the final shutdown sweep needs. Failing every one of them is reported
# and tolerated: this runs during a shutdown, and there is nothing left to
# escalate to.
detach_nfs() {
    local mp
    for mp in $(nfs_mounts); do
        if umount "$mp" 2>/dev/null; then
            continue
        fi
        if umount -f "$mp" 2>/dev/null; then
            log "force-unmounted $mp"
            continue
        fi
        if umount -l "$mp" 2>/dev/null; then
            log "lazily detached $mp"
            continue
        fi
        log "WARNING: could not unmount $mp"
    done
}

wait_for_pods || kill_pods
detach_nfs
log "drain complete; the NFS transport may now be stopped"
