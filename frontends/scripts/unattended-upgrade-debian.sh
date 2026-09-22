#!/bin/bash
#
# unattended-upgrade-debian — unattended package updates for the Debian 13
# Raspberry Pis pi2/pi3 (task l82; f3s/docs/pi2-pi3-os-replacement.md §5, §7).
#
#   daily   once-per-day apt update + unattended-upgrade (stamp-gated) and
#           needrestart service restarts, plus an every-tick reboot check
#
# The Debian counterpart of unattended-upgrade-rocky.sh, keeping its
# design: timer-driven with NO script jitter (the per-host OnCalendar
# offsets keep pi2 and pi3 apart), a partner ping gate (the other Pi-hole
# must answer before this one updates or reboots), at most one unattended
# reboot per calendar day, atomic stamps, and stable WARNING markers. The
# package selection lives in apt, not here: unattended-upgrade installs only
# the origins in /etc/apt/apt.conf.d/52unattended-upgrade-gonf (Debian
# security, Debian point releases, Docker CE), which gonf manages together
# with this script (gonf/debian/unattended.go).
#
# Reboot signals, all independent of the wall clock: a newer kernel than the
# running one (needrestart -k), /run/reboot-required (written by the
# unattended-upgrades kernel postinst hook), and services still running
# with outdated libraries that needrestart does not restart on its own
# (dbus, systemd-logind, docker, gettys, ...: its override_rc defaults).
# /run is a tmpfs and needrestart inspects the running processes, so none of
# them can survive a reboot — the Rocky p82 reboot loop (install times
# compared with a pre-NTP boot time) has no counterpart here. A kernel that
# is still not running after its reboot is warned about, not rebooted for
# again.
#
# The Pis have no RTC: until NTP steps the clock after boot, the wall clock
# starts at the systemd build epoch, so "today" (every stamp) and apt's
# Release-file date checks are wrong. Both the update and the reboot check
# therefore wait until the clock was NTP-synchronised once this boot
# (latched per boot id, so a later NTP blip does not block them).
#
# Destination guard: the script refuses to run unless /etc/os-release says
# ID=debian, so a mistaken install on a Rocky host does nothing (gonf also
# guards its deployment with /etc/debian_version).
#
# No set -e: every command's status is checked explicitly (ping, apt and
# needrestart failures are expected outcomes that must be logged, not abort
# the run half-way). The UNATTENDED_UPGRADE_TEST_* variables exist only for
# the test harness (tests/unattended-upgrade-debian.bash).

set -uo pipefail

PATH=/usr/bin:/bin:/usr/sbin:/sbin:/usr/local/bin:/usr/local/sbin
if [[ -n "${UNATTENDED_UPGRADE_TEST_PATH:-}" ]]; then
	PATH="${UNATTENDED_UPGRADE_TEST_PATH}:$PATH"
fi
export PATH

umask 077

declare -r LOG=${UNATTENDED_UPGRADE_TEST_LOG:-/var/log/unattended-upgrade.log}
declare -r LOCK=${UNATTENDED_UPGRADE_TEST_LOCK:-/run/unattended-upgrade.lock}
declare -r STAMP_DIR=${UNATTENDED_UPGRADE_TEST_STAMP_DIR:-/var/lib/unattended-upgrade}
declare -r OS_RELEASE=${UNATTENDED_UPGRADE_TEST_OS_RELEASE:-/etc/os-release}
declare -r DT_MODEL=${UNATTENDED_UPGRADE_TEST_DT_MODEL:-/proc/device-tree/model}
declare -r BOOT_ID_FILE=${UNATTENDED_UPGRADE_TEST_BOOT_ID:-/proc/sys/kernel/random/boot_id}
declare -r REBOOT_FLAG=${UNATTENDED_UPGRADE_TEST_REBOOT_FLAG:-/run/reboot-required}

declare -r STAMP=$STAMP_DIR/last-daily
declare -r REBOOT_STAMP=$STAMP_DIR/last-reboot
# The expected kernel (needrestart KEXP) of the last reboot done for it.
declare -r KERNEL_REBOOT_STAMP=$STAMP_DIR/last-kernel-reboot
# Day of the last "kernel still not running after its reboot" warning.
declare -r KERNEL_WARN_STAMP=$STAMP_DIR/last-kernel-warning
# Boot id of the boot whose clock has been NTP-synchronised at least once.
declare -r CLOCK_LATCH=$STAMP_DIR/clock-synced-boot
# "<boot id> <day>" of the last "clock not NTP-synchronised" warning.
declare -r CLOCK_WARN_STAMP=$STAMP_DIR/last-clock-warning

# Our own oneshot unit: needrestart must never restart it mid-run (its
# conf.d override says so too; this filter is the defence in depth).
declare -r SELF_UNIT=unattended-upgrade-debian.service

declare -ri LOOKUP_TIMEOUT=10
declare -ri HEALTH_TRIES=3
declare -ri HEALTH_RETRY_SLEEP=2

log() {
	printf '[%s] %s\n' "$(date '+%Y-%m-%dT%H:%M:%S')" "$*" | tee -a "$LOG"
}

# Journal-only message (stdout of the oneshot unit): for per-tick notes
# that would otherwise add a line to $LOG every hour.
note() {
	printf '%s\n' "$*"
}

# Refuse to run anywhere but Debian. Prints the found ID on refusal.
debian_host() {
	local id
	id=$(sed -n 's/^ID=//p' "$OS_RELEASE" 2>/dev/null | tr -d '"'"'")
	if [[ "$id" == debian ]]; then
		return 0
	fi
	printf 'WARNING: unattended-upgrade-debian refused: %s has ID=%s, not debian\n' \
		"$OS_RELEASE" "${id:-<none>}" >&2
	return 1
}

# Partner IPs (the Pis do not resolve piN.lan.buetow.org). The gate
# requires ALL listed partners pingable.
partners_for() {
	case $1 in
	pi2) printf '192.168.1.128\n' ;;
	pi3) printf '192.168.1.127\n' ;;
	*) ;;
	esac
}

partners_up() {
	local ip
	local -i attempt ok
	[[ -n "$PARTNERS" ]] || return 1
	for ip in $PARTNERS; do
		ok=0
		for ((attempt = 1; attempt <= HEALTH_TRIES; attempt++)); do
			if timeout "$LOOKUP_TIMEOUT" ping -c1 -W3 "$ip" >/dev/null 2>&1; then
				ok=1
				break
			fi
			((attempt < HEALTH_TRIES)) && sleep "$HEALTH_RETRY_SLEEP"
		done
		((ok == 1)) || return 1
	done
	return 0
}

# Prints the first line of file $1 (empty when missing or unreadable).
read_stamp() {
	local line=
	[[ -r "$1" ]] && IFS= read -r line <"$1"
	printf '%s' "$line"
}

# Write $2 (one line) to file $1 via temp + mv, so a crash never leaves a
# half-written stamp. Errors are silent: a missing stamp only repeats work.
write_atomic() {
	local tmp=$1.tmp.$$
	mkdir -p "${1%/*}" 2>/dev/null
	{ printf '%s\n' "$2" >"$tmp" && mv "$tmp" "$1"; } 2>/dev/null
}

# 0 when timedatectl reports the wall clock as NTP-synchronised.
clock_synced() {
	[[ "$(timedatectl show -p NTPSynchronized --value 2>/dev/null)" == yes ]]
}

# 0 on a Raspberry Pi (device-tree model), i.e. a host without an RTC.
rtc_less_host() {
	[[ -r "$DT_MODEL" ]] && tr -d '\0' <"$DT_MODEL" | grep -q 'Raspberry Pi'
}

# 0 when the wall clock can be trusted for dates and apt's Release checks:
# always on a host with an RTC; on a Pi once the clock was NTP-synchronised
# during this boot, latched per boot id in $CLOCK_LATCH so a later NTP blip
# does not matter (the clock was stepped right and only drifts slowly).
clock_trusted() {
	local boot
	rtc_less_host || return 0
	boot=$(read_stamp "$BOOT_ID_FILE")
	if [[ -n "$boot" && "$(read_stamp "$CLOCK_LATCH")" == "$boot" ]]; then
		return 0
	fi
	clock_synced || return 1
	[[ -z "$boot" ]] || write_atomic "$CLOCK_LATCH" "$boot"
	return 0
}

# Log the not-yet-synchronised skip once per boot and day. The marker text
# "WARNING: unattended-upgrade skipped: clock not NTP-synchronised" is the
# same as on Rocky so one check can key off both.
warn_clock_unsynced() {
	local key
	key="$(read_stamp "$BOOT_ID_FILE") $today"
	if [[ "$(read_stamp "$CLOCK_WARN_STAMP")" == "$key" ]]; then
		note "skipped daily: clock not NTP-synchronised yet (not stamped)"
		return 0
	fi
	log "WARNING: unattended-upgrade skipped: clock not NTP-synchronised since boot (not stamped, retrying hourly)"
	write_atomic "$CLOCK_WARN_STAMP" "$key"
}

# Classify the kernel into K_STATE (with K_RUNNING, K_EXPECTED) from
# needrestart's batch kernel check (NEEDRESTART-KSTA: 1 current, 2 ABI
# compatible upgrade pending, 3 version upgrade pending, 0 unknown):
#   unknown  needrestart failed or could not tell (no kernel reboot; the
#            /run/reboot-required flag still counts)
#   current  the newest installed kernel is running
#   pending  a newer kernel is installed: a reboot is due
#   stuck    this host already rebooted once for K_EXPECTED and still runs
#            K_RUNNING (the new kernel failed to boot or is not the one the
#            firmware loads). Rebooting again would only repeat that daily,
#            so this only warns (warn_kernel_mismatch).
kernel_classify() {
	local out sta
	K_STATE=unknown
	K_RUNNING=
	K_EXPECTED=
	out=$(needrestart -b -k 2>/dev/null) || return 0
	sta=$(sed -n 's/^NEEDRESTART-KSTA: *//p' <<<"$out")
	K_RUNNING=$(sed -n 's/^NEEDRESTART-KCUR: *//p' <<<"$out")
	K_EXPECTED=$(sed -n 's/^NEEDRESTART-KEXP: *//p' <<<"$out")
	case $sta in
	1) K_STATE=current ;;
	2 | 3)
		[[ -n "$K_EXPECTED" ]] || return 0
		if [[ "$(read_stamp "$KERNEL_REBOOT_STAMP")" == "$K_EXPECTED" ]]; then
			K_STATE=stuck
		else
			K_STATE=pending
		fi
		;;
	*) ;;
	esac
}

# Log (at most once a day) that the reboot for K_EXPECTED did not bring it
# up. Called before the reboot gates, so a partner outage cannot hide it.
warn_kernel_mismatch() {
	[[ "$(read_stamp "$KERNEL_WARN_STAMP")" != "$today" ]] || return 0
	log "WARNING: already rebooted for kernel $K_EXPECTED but $K_RUNNING is still running — check /boot/firmware and the boot log; not rebooting for it again"
	write_atomic "$KERNEL_WARN_STAMP" "$today"
}

# Prints the services needrestart lists as still using outdated libraries
# (one per line, our own unit excluded). After restart_services these are
# the ones its override_rc keeps from being restarted automatically.
# Prints nothing when needrestart fails.
stale_services() {
	local out
	out=$(needrestart -b -r l -l 2>/dev/null) || return 0
	sed -n 's/^NEEDRESTART-SVC: *//p' <<<"$out" | grep -vxF "$SELF_UNIT"
}

# At most one unattended reboot per calendar day (a backstop should a
# reboot reason ever survive the reboot), and only while the partner is up.
# Returns 0 when this tick may reboot.
reboot_window_open() {
	[[ "$(read_stamp "$REBOOT_STAMP")" != "$today" ]] || return 1
	if ! partners_up; then
		log "reboot check deferred: partner(s) not reachable"
		return 1
	fi
	return 0
}

# Decide whether a reboot is required (K_STATE must be set by
# kernel_classify). 0 = reboot (reason in REBOOT_REASON, a kernel target in
# REBOOT_KERNEL), 1 = not required.
reboot_needed() {
	local pkgs services
	REBOOT_REASON=
	REBOOT_KERNEL=
	if [[ "$K_STATE" == pending ]]; then
		REBOOT_REASON="kernel $K_EXPECTED installed, running $K_RUNNING"
		REBOOT_KERNEL=$K_EXPECTED
		return 0
	fi
	if [[ -e "$REBOOT_FLAG" ]]; then
		pkgs=$(sort -u "$REBOOT_FLAG.pkgs" 2>/dev/null | tr '\n' ' ')
		REBOOT_REASON="$REBOOT_FLAG present${pkgs:+ (${pkgs% })}"
		return 0
	fi
	services=$(stale_services | tr '\n' ' ')
	if [[ -n "$services" ]]; then
		REBOOT_REASON="services still on outdated libraries: ${services% }"
		return 0
	fi
	return 1
}

# Stamp the day (and the target kernel of a kernel reboot, so a mismatch
# that survives the reboot is not rebooted for again), release the lock and
# reboot.
do_reboot() {
	log "rebooting: $REBOOT_REASON"
	write_atomic "$REBOOT_STAMP" "$today"
	[[ -z "$REBOOT_KERNEL" ]] \
		|| write_atomic "$KERNEL_REBOOT_STAMP" "$REBOOT_KERNEL"
	sync
	sleep 2
	rmdir "$LOCK" 2>/dev/null
	trap - EXIT
	systemctl reboot
}

# The reboot check. Needs a trusted clock ("today" is wrong before the
# first NTP sync of a Pi boot): main only calls it after its clock gate.
# The stuck-kernel warning comes before the gates so that it shows even
# while a partner is down or the host already rebooted today.
maybe_reboot() {
	kernel_classify
	[[ "$K_STATE" != stuck ]] || warn_kernel_mismatch
	reboot_window_open || return 0
	if reboot_needed; then
		do_reboot
	fi
	return 0
}

# apt-get update. A failed run is not stamped (retried next tick); fetch
# warnings (a source such as download.docker.com unreachable, its old lists
# kept) are logged and the run goes on, so Debian security fixes are not
# held back by a third-party repository.
apt_update() {
	local out rc
	out=$(apt-get -q update 2>&1)
	rc=$?
	if ((rc != 0)); then
		log "apt-get update FAILED (rc=$rc) — not stamped, will retry next hour"
		printf '%s\n' "$out" | tee -a "$LOG"
		return 1
	fi
	if grep -qE '^(W|E|Err):' <<<"$out"; then
		log "WARNING: apt-get update reported fetch problems, continuing with the lists it has:"
		grep -E '^(W|E|Err):' <<<"$out" | tee -a "$LOG"
	fi
	return 0
}

# unattended-upgrade (origins from apt.conf.d). NEEDRESTART_SUSPEND keeps
# needrestart's dpkg hook quiet during it; restart_services runs it once
# afterwards instead of once per dpkg step.
run_upgrade() {
	local out rc
	out=$(NEEDRESTART_SUSPEND=1 unattended-upgrade -v 2>&1)
	rc=$?
	if ((rc != 0)); then
		log "unattended-upgrade FAILED (rc=$rc) — not stamped, will retry next hour"
		printf '%s\n' "$out" | tee -a "$LOG"
		return 1
	fi
	if grep -q '^Packages that will be upgraded: *[^ ]' <<<"$out"; then
		log "unattended-upgrade:"
		printf '%s\n' "$out" | tee -a "$LOG"
	fi
	return 0
}

# Restart the services running outdated libraries. needrestart -r a skips
# what its override_rc protects (dbus, systemd-logind, docker, gettys, and
# our own unit through the gonf conf.d file); reboot_needed treats those
# leftovers as a reboot reason. A failure is logged, not fatal.
restart_services() {
	local out
	if ! out=$(NEEDRESTART_MODE=a DEBIAN_FRONTEND=noninteractive \
		needrestart -r a -l 2>&1); then
		log "WARNING: needrestart -r a failed"
	fi
	if grep -q 'Restarting services' <<<"$out"; then
		log "needrestart:"
		printf '%s\n' "$out" | tee -a "$LOG"
	fi
}

# Take the whole-job lock (a directory), stealing one older than 2 h.
# Exits 0 when another run holds it. The EXIT trap releases it.
acquire_lock() {
	if ! mkdir "$LOCK" 2>/dev/null; then
		if [[ -z "$(find "$LOCK" -mmin +120 2>/dev/null)" ]]; then
			log "skipped daily, another run holds the lock"
			exit 0
		fi
		if ! { rmdir "$LOCK" && mkdir "$LOCK" 2>/dev/null; }; then
			log "stale lock unreadable, skipping daily"
			exit 0
		fi
	fi
	trap 'rmdir "$LOCK" 2>/dev/null' EXIT
}

# The once-a-day update behind the partner gate. Returns 0 when the day was
# updated and stamped or skipped by the partner gate, 1 when apt failed.
# Restarts run before the stamp, so a crash mid-restart leaves the day
# unstamped and the next tick retries.
daily_update() {
	if ! partners_up; then
		if [[ -n "$PARTNERS" ]]; then
			log "skipped daily update: partner(s) not reachable (not stamped)"
		else
			log "skipped daily update: no partner configured for $(hostname -s) (not stamped)"
		fi
		return 0
	fi
	apt_update || return 1
	run_upgrade || return 1
	restart_services
	write_atomic "$STAMP" "$today"
}

main() {
	local -i rc=0
	case ${1:-} in
	daily) ;;
	*)
		printf 'usage: %s daily\n' "$0" >&2
		exit 64
		;;
	esac
	debian_host || exit 1

	PARTNERS=$(partners_for "$(hostname -s)")
	acquire_lock
	today=$(date +%F)

	# Clock gate (Pis only, see clock_trusted): before the first NTP sync
	# of this boot, "today" and apt's Release-date checks are wrong. Skip
	# the whole tick, stamping nothing; the next hourly tick retries. The
	# reboot check needs the same clock, so it is skipped as well.
	if ! clock_trusted; then
		warn_clock_unsynced
		exit 0
	fi

	# Stamp already today → skip the update, still check the reboot, which
	# also runs after a skipped or failed update (exit 1 for a failure).
	if [[ "$(read_stamp "$STAMP")" != "$today" ]]; then
		daily_update || rc=1
	fi
	maybe_reboot
	exit "$rc"
}

main "$@"
