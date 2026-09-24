#!/bin/ksh
#
# unattended-upgrade — unattended security-only updates for OpenBSD.
#
#   base    apply base-system errata via syspatch(8)
#   pkgs    update packages via pkg_add(1) -u, restart affected daemons
#   audit   audit installed packages against current OpenBSD package metadata
#   reboot  reboot if a patched kernel is pending
#
# Intended for root's crontab. Everything it prints is mailed to root by
# cron AND appended to /var/log/unattended-upgrade.log (rotated via
# newsyslog(8)). Silence = clean no-op; mail = change or failure.
# Companion docs: frontends/docs/unattended-upgrades*.md
#
# Update and reboot modes are gated on the partner frontend being operational
# (same https://<host>/index.txt health check as dns-failover.ksh, see the
# KISS high-availability write-up): patching or rebooting the only healthy
# host would take the sites down completely. The read-only audit is ungated
# so an outage cannot hide package-security status.

# /sbin is needed for reboot(8) (and other base system tools). The test-only
# prefix makes the audit harness exercise this exact script with fake commands;
# it is never configured by the deployed root crontab.
if [ -n "${UNATTENDED_UPGRADE_TEST_PATH:-}" ]; then
    PATH="${UNATTENDED_UPGRADE_TEST_PATH}:/usr/bin:/bin:/usr/sbin:/sbin:/usr/local/bin:/usr/local/sbin"
else
    PATH=/usr/bin:/bin:/usr/sbin:/sbin:/usr/local/bin:/usr/local/sbin
fi
export PATH

# Everything the script creates (lock dir, log lines via tee -a, needs-reboot
# flag) is root-only; tee's default 0644 log file would contradict the 600
# mode declared in the newsyslog rotation entry until the first rotation.
umask 077

LOG=${UNATTENDED_UPGRADE_TEST_LOG:-/var/log/unattended-upgrade.log}
SERVICES=/etc/unattended-upgrade-services
LOCK=${UNATTENDED_UPGRADE_TEST_LOCK:-/var/run/unattended-upgrade.lock}

# Same health-check constants as dns-failover.ksh.
readonly LOOKUP_TIMEOUT=10
readonly HEALTH_TRIES=3
readonly HEALTH_RETRY_SLEEP=2

mode=${1:-}
case $mode in
base|pkgs|audit|reboot) ;;
*) print -u2 "usage: $0 base|pkgs|audit|reboot"; exit 64 ;;
esac

# Emit to stdout (cron mails it) and append the same lines to the log file.
log() {
    printf '[%s] %s\n' "$(date '+%Y-%m-%dT%H:%M:%S')" "$*" | tee -a "$LOG"
}

# A clean audit is evidence worth retaining but not a daily mail: cron's
# normal silence remains the success signal. Findings and failures use log(),
# which both records them and makes root's cron mail alert visible.
audit_clean() {
    printf '[%s] package audit clean: pkg_add -Iun found no pending packages or quirks warnings\n' \
        "$(date '+%Y-%m-%dT%H:%M:%S')" >>"$LOG" \
        || { print -u2 "package audit FAILED: cannot append to $LOG"; return 1; }
}

# A normal pkg_add update probe always prints the signed quirks timestamp.
# It is evidence that the metadata was fetched, not an outstanding package or
# advisory finding. Keep every other line for the operator to investigate.
audit_findings() {
    printf '%s\n' "$1" | sed \
        '/^quirks-[^[:space:]]* signed on [^[:space:]]*$/d'
}

# Jitter only for cron runs (no tty); manual runs are instant.
[ -t 0 ] || sleep $((RANDOM % 1200))

# The partner of this host. Unknown hostnames skip the upgrade: without a
# known partner there is no proof that a second healthy server exists.
case $(hostname -s) in
blowfish) PARTNER=fishfinger.buetow.org ;;
fishfinger) PARTNER=blowfish.buetow.org ;;
*) PARTNER= ;;
esac

# Operational check of the partner frontend, same check as the failover in
# dns-failover.ksh (KISS high-availability with OpenBSD): fetch the partner's
# /index.txt over IPv4 AND IPv6 with ftp(1) and expect its Welcome banner.
# Every lookup is wrapped in timeout(1) — a hung query must not wedge the
# job — and each family must pass HEALTH_TRIES consecutive attempts, so a
# brief blip does not pause the upgrades. Residual risk: the partner may go
# down just after a passing check; the once-per-minute dns-failover covers
# the sites meanwhile, and this window is accepted.
partner_up() {
    [ -n "$PARTNER" ] || return 1
    local proto
    local -i attempt
    for proto in 4 6; do
        local -i ok=0
        attempt=1
        while [ $attempt -le $HEALTH_TRIES ]; do
            if timeout $LOOKUP_TIMEOUT ftp -$proto -o - \
                "https://$PARTNER/index.txt" 2>/dev/null \
                | grep -q "Welcome to $PARTNER"; then
                ok=1
                break
            fi
            attempt=$((attempt + 1))
            [ $attempt -le $HEALTH_TRIES ] && sleep $HEALTH_RETRY_SLEEP
        done
        [ $ok -eq 1 ] || return 1
    done
    return 0
}

# Operational check of the custom fleet repo (k3s-backed). The relayd
# fronting it serves an HTML "Server turned off" page with HTTP 200 when
# the k3s backend is down, so an HTTP-200 body alone proves nothing: the
# repo is considered up only when the fetched listing is non-empty, free
# of the down page, and contains package entries.
pkgrepo_up() {
    local body
    body=$(timeout $LOOKUP_TIMEOUT ftp -o - \
        "https://pkgrepo.f3s.buetow.org/openbsd/$(uname -r)/packages/amd64/" \
        2>/dev/null) || return 1
    [ -n "$body" ] || return 1
    printf '%s' "$body" | grep -q "Server turned off" && return 1
    printf '%s' "$body" | grep -q "\.tgz"
}

lock_unavailable() {
    if [ "$mode" = audit ]; then
        log "package audit FAILED: another unattended-upgrade job holds $LOCK; no audit result was produced"
        exit 1
    fi
    logger "unattended-upgrade: skipped $mode, another run holds the lock"
    exit 0
}

# Whole-job lock. A previous run killed mid-flight (crash, power loss)
# leaves the dir behind — OpenBSD does not wipe /var/run at boot — so
# steal locks older than 2 h instead of skipping forever.
if ! mkdir "$LOCK" 2>/dev/null; then
    if [ -n "$(find "$LOCK" -mmin +120 2>/dev/null)" ]; then
        if ! rmdir "$LOCK" || ! mkdir "$LOCK" 2>/dev/null; then
            lock_unavailable
        fi
    else
        lock_unavailable
    fi
fi
trap 'rmdir "$LOCK" 2>/dev/null' EXIT

# Refuse to upgrade while the partner is down (or unknown): this host could
# be the only one serving the sites, and a patch, daemon restart, or reboot
# would then cause total downtime. The flag and the patches simply wait for
# a later window in which the partner is healthy again.
if [ "$mode" != audit ] && ! partner_up; then
    if [ -n "$PARTNER" ]; then
        log "skipped $mode: partner $PARTNER not operational"
    else
        log "skipped $mode: no partner configured for $(hostname)"
    fi
    exit 0
fi

# Wording-independent reboot-pending detection, KARL-safe. Comparing /bsd's
# mtime with /var/run/dmesg.boot does NOT work: OpenBSD re-links /bsd at
# every boot (KARL), so /bsd is always the newer file and the mtime check
# would demand a reboot after every non-kernel patch too. Compare build
# versions instead: dmesg.boot records the booted kernel, what(1) reads
# /bsd's — the two differ only while a relinked (patched) kernel is waiting
# to be booted. Fail-safe: unknown state (no version found) means NO reboot.
# This check is the SOLE reboot decision — no runtime flag state exists.
kernel_reboot_pending() {
    [ -f /var/run/dmesg.boot ] || return 1
    booted=$(sed -n '1{s/[[:space:]]*$//;p;}' /var/run/dmesg.boot)
    ondisk=$(what /bsd 2>/dev/null | grep -m1 'OpenBSD' | sed 's/^[[:space:]]*//')
    [ -n "$ondisk" ] && [ "$booted" != "$ondisk" ]
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
    # Quirk: on a backlog the FIRST syspatch run may install only
    # 001_syspatch — the tool's own update — and exit 2 with errata still
    # pending. Re-run once with the updated tool so a single base run
    # clears the whole backlog; no manual follow-up syspatch is needed.
    before=$(syspatch -l)
    out=""
    rc=0
    for attempt in 1 2; do
        step=$(syspatch 2>&1)
        rc=$?
        if [ -n "$step" ]; then
            out="${out}${step}
"
        fi
        case $rc in
        0|2) ;;
        *) break ;;   # hard failure — stop re-running
        esac
        if [ "$(syspatch -c 2>/dev/null | wc -l | tr -d ' ')" -eq 0 ] \
            || [ $attempt -eq 2 ]; then
            break
        fi
        log "syspatch updated its own tool — running again with the new one"
    done
    # Judge success by the patch-list diff, not the last exit code: the
    # self-update quirk makes the final rc unreliable (2 while patches were
    # applied in an earlier iteration).
    case $rc in
    0|2)
        if [ "$before" = "$(syspatch -l)" ]; then
            exit 0      # clean no-op: silent
        fi
        log "syspatch applied base patches:"
        printf '%s\n' "$out" | tee -a "$LOG"
        if kernel_reboot_pending \
            || printf '%s\n' "$out" | grep -qi reboot; then
            log "kernel patched — reboot queued for the next 'reboot' window"
        else
            log "non-kernel base patch — restarting base daemons"
            restart_services
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
    # 256MB per mountpoint: errata packages are tens of MB; the original 1GB
    # threshold was unreachable on the frontends' small / and /var (impl doc
    # section 8 acceptance requires the pkgs run to work).
    for mp in / /usr /var; do
        avail=$(df -k "$mp" | awk 'NR==2 {print $4}')
        if [ "${avail:-0}" -lt 262144 ]; then
            log "pkgs aborted: only ${avail}KB free on $mp (need 256MB)"
            exit 1
        fi
    done
    # Root crontabs have no /root/.profile (where gonf task
    # frontends_pkg_repo exports the interactive PKG_PATH), so PKG_PATH
    # must include the custom fleet repo alongside the official
    # installurl tree, or pkg_add -u fails on custom packages (dserver,
    # dtail, gogios, ...). 'installpath' resolves installurl(5) and keeps
    # the automatic packages-stable errata search.
    #
    # The custom repo is k3s-backed; when it is down (relayd "Server turned
    # off" page) skip its packages for this window and update the official
    # errata tree only — unattended upgrades must not fail because the
    # cluster is down. The custom packages are picked up by a later window
    # once the repo is back.
    ver=$(uname -r)
    custom="https://pkgrepo.f3s.buetow.org/openbsd/${ver}/packages/amd64/"
    customSkipped=""
    if pkgrepo_up; then
        PKG_PATH="installpath:${custom}"
        export PKG_PATH
    else
        log "WARNING: ${custom} not operational — skipping custom-repo packages this window"
        PKG_PATH="installpath"
        export PKG_PATH
        customSkipped=1
    fi
    out=$(pkg_add -Iu 2>&1)
    rc=$?
    if [ "$rc" -ne 0 ]; then
        if [ -n "$customSkipped" ] \
            && printf '%s\n' "$out" | grep -q "Couldn't find updates"; then
            # Expected with the custom repo skipped: the unresolvable stems
            # are the custom packages (dtail, gogios, dserver, ...).
            log "WARNING: pkg_add -u rc=$rc with the custom repo skipped — official updates applied, custom packages deferred"
            printf '%s\n' "$out" | tee -a "$LOG"
            exit 0
        fi
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

audit)
    # OpenBSD has no generic CVE database for native packages. The supported
    # package audit is pkg_add's update resolution, including its signed
    # quirks metadata: packages(7) documents that quirks identifies older
    # packages with security issues which cannot be updated. Run this after
    # the normal pkgs job so its mandatory quirks refresh has already run.
    #
    # Unlike pkgs, an unavailable custom repo is a failure here. Falling back
    # to installpath would hide the status of dserver, dtail, gogios, and
    # other fleet packages, which is not an auditable all-packages result.
    ver=$(uname -r)
    custom="https://pkgrepo.f3s.buetow.org/openbsd/${ver}/packages/amd64/"
    if ! pkgrepo_up; then
        log "package audit FAILED: ${custom} not operational; custom packages cannot be audited"
        exit 1
    fi
    PKG_PATH="installpath:${custom}"
    export PKG_PATH
    # pkg_add -n may still populate a caller-supplied PKG_CACHE. This audit
    # must not mutate the host, so deliberately disable that optional cache.
    unset PKG_CACHE
    out=$(pkg_add -Iun 2>&1)
    rc=$?
    if [ "$rc" -ne 0 ]; then
        log "package audit FAILED: pkg_add -Iun rc=$rc — investigate"
        printf '%s\n' "$out" | tee -a "$LOG"
        exit 1
    fi
    findings=$(audit_findings "$out")
    if [ -z "$findings" ]; then
        audit_clean || exit 1
        exit 0
    fi
    log "package audit found pending packages or an OpenBSD quirks advisory:"
    printf '%s\n' "$findings" | tee -a "$LOG"
    exit 1
    ;;

reboot)
    # No runtime flag: the KARL-safe version compare IS the pending state.
    # A patched-but-not-booted kernel (from this automation or a manual
    # syspatch) is picked up here; an up-to-date kernel is a silent no-op.
    if kernel_reboot_pending; then
        log "rebooting to activate the patched kernel"
        sync
        sleep 2
        rmdir "$LOCK" 2>/dev/null   # deterministic release (EXIT trap not guaranteed under reboot(8))
        trap - EXIT
        reboot
    fi
    ;;
esac

exit 0
