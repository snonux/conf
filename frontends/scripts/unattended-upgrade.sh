#!/bin/ksh
#
# unattended-upgrade — unattended security-only updates for OpenBSD.
#
#   base    apply base-system errata via syspatch(8)
#   pkgs    update packages via pkg_add(1) -u, restart affected daemons
#   reboot  reboot if a patched kernel is pending
#
# Intended for root's crontab. Everything it prints is mailed to root by
# cron AND appended to /var/log/unattended-upgrade.log (rotated via
# newsyslog(8)). Silence = clean no-op; mail = change or failure.
# Companion docs: frontends/docs/unattended-upgrades*.md

PATH=/usr/bin:/bin:/usr/sbin:/usr/local/bin:/usr/local/sbin
export PATH

LOG=/var/log/unattended-upgrade.log
SERVICES=/etc/unattended-upgrade-services
RUNDIR=/var/run/unattended-upgrade
LOCK=/var/run/unattended-upgrade.lock

mode=${1:-}
case $mode in
base|pkgs|reboot) ;;
*) print -u2 "usage: $0 base|pkgs|reboot"; exit 64 ;;
esac

mkdir -p "$RUNDIR" || exit 1

# Jitter only for cron runs (no tty); manual runs are instant.
[ -t 0 ] || sleep $((RANDOM % 1200))

# Whole-job lock. A previous run killed mid-flight (crash, power loss)
# leaves the dir behind — OpenBSD does not wipe /var/run at boot — so
# steal locks older than 2 h instead of skipping forever.
if ! mkdir "$LOCK" 2>/dev/null; then
    if [ -n "$(find "$LOCK" -mmin +120 2>/dev/null)" ]; then
        rmdir "$LOCK" && mkdir "$LOCK" 2>/dev/null \
            || { logger "unattended-upgrade: stale lock unreadable, skipping $mode"; exit 0; }
    else
        logger "unattended-upgrade: skipped $mode, another run holds the lock"
        exit 0
    fi
fi
trap 'rmdir "$LOCK" 2>/dev/null' EXIT

# Emit to stdout (cron mails it) and append the same lines to the log file.
log() {
    printf '[%s] %s\n' "$(date '+%Y-%m-%dT%H:%M:%S')" "$*" | tee -a "$LOG"
}

# Wording-independent kernel-patch detection: syspatch relinks /bsd, while
# /var/run/dmesg.boot is written at boot — if /bsd is newer, a reboot is due.
kernel_is_newer_than_boot() {
    [ -f /var/run/dmesg.boot ] && [ -n "$(find /bsd -newer /var/run/dmesg.boot 2>/dev/null)" ]
}

restart_services() {
    [ -f "$SERVICES" ] || return 0
    while IFS= read -r svc; do
        [ -n "$svc" ] || continue
        case $svc in \#*) continue ;; esac
        if rcctl check "$svc" >/dev/null 2>&1; then
            log "restarting $svc"
            rcctl restart "$svc" >/dev/null 2>&1 \
                || log "WARNING: rcctl restart $svc failed"
        else
            log "not running: $svc — skipped"
        fi
    done <"$SERVICES"
}

case $mode in

base)
    # syspatch rc: 0 = patches applied, 2 = clean no-op,
    # >0 = failure (mirror outage, release EOL, ...).
    before=$(syspatch -l)
    out=$(syspatch 2>&1)
    rc=$?
    case $rc in
    2) exit 0 ;;
    0)
        log "syspatch applied base patches:"
        printf '%s\n' "$out" | tee -a "$LOG"
        if [ "$before" != "$(syspatch -l)" ]; then
            if kernel_is_newer_than_boot \
                || printf '%s\n' "$out" | grep -qi reboot; then
                touch "$RUNDIR/needs-reboot"
                log "kernel patched — reboot queued for 'reboot' mode"
            else
                log "non-kernel base patch — restarting base daemons"
                restart_services
            fi
        fi
        ;;
    *)
        log "syspatch FAILED (rc=$rc) — see message below"
        printf '%s\n' "$out" | tee -a "$LOG"
        exit 1
        ;;
    esac
    ;;

pkgs)
    for mp in / /usr /var; do
        avail=$(df -k "$mp" | awk 'NR==2 {print $4}')
        if [ "${avail:-0}" -lt 1048576 ]; then
            log "pkgs aborted: only ${avail}KB free on $mp (need 1GB)"
            exit 1
        fi
    done
    out=$(pkg_add -Iu 2>&1)
    rc=$?
    if [ "$rc" -ne 0 ]; then
        log "pkg_add -u FAILED (rc=$rc) — investigate"
        printf '%s\n' "$out" | tee -a "$LOG"
        exit 1
    fi
    [ -z "$out" ] && exit 0     # nothing to update
    log "pkg_add -u updated:"
    printf '%s\n' "$out" | tee -a "$LOG"
    # quirks-only bumps happen regularly; they are no reason to bounce daemons.
    if printf '%s\n' "$out" | grep -v '^quirks-' | grep -q .; then
        restart_services
    fi
    ;;

reboot)
    if [ -f "$RUNDIR/needs-reboot" ]; then
        if kernel_is_newer_than_boot; then
            log "rebooting to activate the patched kernel"
            rm -f "$RUNDIR/needs-reboot"
            sync
            sleep 2
            reboot
        else
            log "stale needs-reboot flag removed (kernel already current)"
            rm -f "$RUNDIR/needs-reboot"
        fi
    fi
    ;;
esac

exit 0
