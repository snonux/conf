#!/bin/ksh
#
# unattended-upgrade-rocky — unattended package updates for Rocky hosts:
# pi2/pi3 and the k3s nodes r0/r1/r2 (same on-demand pattern).
#
#   daily   once-per-day dnf upgrade (stamp-gated) + every-tick reboot check
#
# Timer-driven (NO script jitter — per-host OnCalendar offsets are the
# anti-coincidence mechanism). Logs to the journal via stdout and to
# /var/log/unattended-upgrade.log. Companion plan:
# frontends/docs/unattended-upgrades-pi.plan.md §12.
#
# Rocky ships AT&T ksh93u+m — use typeset, not local (local is a pdkshism).
# Note that in ksh93 a typeset inside a POSIX-style name() function is NOT
# local; helper variables therefore get names that never collide with the
# globals. The script also runs under bash for the test harness
# (tests/unattended-upgrade-rocky.ksh); the UNATTENDED_UPGRADE_TEST_*
# variables exist only for it (fake tool dir, log, lock, stamp dir and
# /proc/stat).

PATH=/usr/bin:/bin:/usr/sbin:/sbin:/usr/local/bin:/usr/local/sbin
if [ -n "${UNATTENDED_UPGRADE_TEST_PATH:-}" ]; then
	PATH="${UNATTENDED_UPGRADE_TEST_PATH}:$PATH"
fi
export PATH

umask 077

LOG=${UNATTENDED_UPGRADE_TEST_LOG:-/var/log/unattended-upgrade.log}
LOCK=${UNATTENDED_UPGRADE_TEST_LOCK:-/var/run/unattended-upgrade.lock}
STAMP_DIR=${UNATTENDED_UPGRADE_TEST_STAMP_DIR:-/var/lib/unattended-upgrade}
STAMP=$STAMP_DIR/last-daily
REBOOT_STAMP=$STAMP_DIR/last-reboot
PROC_STAT=${UNATTENDED_UPGRADE_TEST_PROC_STAT:-/proc/stat}

# The pi2/pi3 SIG AltArch kernel. needs-restarting only knows the stock
# kernel names (kernel, kernel-rt), so a Pi kernel update is detected by
# comparing the running release with the most recently installed package.
readonly PI_KERNEL_PKG=raspberrypi2-kernel4

readonly LOOKUP_TIMEOUT=10
readonly HEALTH_TRIES=3
readonly HEALTH_RETRY_SLEEP=2

mode=${1:-}
case $mode in
daily) ;;
*) print -u2 "usage: $0 daily"; exit 64 ;;
esac

log() {
	printf '[%s] %s\n' "$(date '+%Y-%m-%dT%H:%M:%S')" "$*" | tee -a "$LOG"
}

# Journal-only message (stdout of the oneshot unit): for per-tick notes
# that would otherwise add a line to $LOG every hour.
note() {
	printf '%s\n' "$*"
}

# Partner IPs (pi2/pi3 do not resolve piN.lan.buetow.org). Gate requires
# ALL listed partners pingable. r0/r1/r2 use both siblings (plan §12).
case $(hostname -s) in
pi2) PARTNERS="192.168.1.128" ;;
pi3) PARTNERS="192.168.1.127" ;;
r0)  PARTNERS="192.168.1.121 192.168.1.122" ;;
r1)  PARTNERS="192.168.1.120 192.168.1.122" ;;
r2)  PARTNERS="192.168.1.120 192.168.1.121" ;;
*)   PARTNERS= ;;
esac

partners_up() {
	[ -n "$PARTNERS" ] || return 1
	typeset ip
	typeset -i attempt
	typeset -i ok
	for ip in $PARTNERS; do
		attempt=1
		ok=0
		while [ $attempt -le $HEALTH_TRIES ]; do
			if timeout $LOOKUP_TIMEOUT ping -c1 -W3 "$ip" >/dev/null 2>&1; then
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

# f3s-dtail RPM repo is k3s-backed; relayd serves "Server turned off" when
# the backend sleeps. Probe the listing; skip the repo (not the whole dnf
# run) when it is down.
pkgrepo_up() {
	typeset body arch
	arch=$(uname -m)
	body=$(timeout $LOOKUP_TIMEOUT curl -fsS \
		"https://pkgrepo.f3s.buetow.org/rockylinux/9/${arch}/" \
		2>/dev/null) || return 1
	[ -n "$body" ] || return 1
	printf '%s' "$body" | grep -q "Server turned off" && return 1
	printf '%s' "$body" | grep -qiE '\.rpm'
}

# Whole-job lock. Steal locks older than 2 h.
if ! mkdir "$LOCK" 2>/dev/null; then
	if [ -n "$(find "$LOCK" -mmin +120 2>/dev/null)" ]; then
		rmdir "$LOCK" && mkdir "$LOCK" 2>/dev/null \
			|| { log "stale lock unreadable, skipping $mode"; exit 0; }
	else
		log "skipped $mode, another run holds the lock"
		exit 0
	fi
fi
trap 'rmdir "$LOCK" 2>/dev/null' EXIT

today=$(date +%F)
last=""
[ -f "$STAMP" ] && last=$(cat "$STAMP")

# Reboot check runs every tick (not daily-gated). Partner gate applies.
# At most one unattended reboot per calendar day, as a backstop against a
# reboot loop with OnBootSec should a reboot reason survive the reboot
# (reboot_needed below removes the known cause, see confirm_core_updates).
# Weekday stagger for r0/r1/r2 so two k3s nodes never reboot the same day
# (date +%u % 3: Mon/Thu/Sun→r0, Tue/Fri→r1, Wed/Sat→r2).
# Returns 0 when this tick may reboot.
reboot_window_open() {
	if [ -f "$REBOOT_STAMP" ] && [ "$(cat "$REBOOT_STAMP")" = "$today" ]; then
		return 1
	fi
	if ! partners_up; then
		log "reboot check deferred: partner(s) not reachable"
		return 1
	fi
	case $(hostname -s) in
	r0) [ $(($(date +%u) % 3)) -eq 1 ] || return 1 ;;
	r1) [ $(($(date +%u) % 3)) -eq 2 ] || return 1 ;;
	r2) [ $(($(date +%u) % 3)) -eq 0 ] || return 1 ;;
	esac
	return 0
}

# Kernel boot time in epoch seconds (btime in /proc/stat). The kernel
# derives it from the current wall clock minus the time since boot on every
# read, so it is right as soon as the clock is.
kernel_boot_epoch() {
	awk '$1 == "btime" { print $2; exit }' "$PROC_STAT" 2>/dev/null
}

# 0 when chronyd/timesyncd reports the wall clock as NTP-synchronised.
clock_synced() {
	[ "$(timedatectl show -p NTPSynchronized --value 2>/dev/null)" = yes ]
}

# Newest install time (epoch s) of any installed version of package $1;
# prints nothing when rpm cannot tell (not installed, rpm error).
newest_install_epoch() {
	rpm -q --qf '%{INSTALLTIME}\n' "$1" 2>/dev/null \
		| grep -E '^[0-9]+$' | sort -n | tail -n 1
}

# 0 (reason in REBOOT_REASON) when the Pi kernel installed last is not the
# running one — the Pi bootloader loads the kernel the latest package
# transaction wrote to /boot. 1 on hosts without $PI_KERNEL_PKG (r0/r1/r2)
# or when the running kernel is current.
pi_kernel_pending() {
	typeset pk_newest pk_running
	rpm -q --quiet "$PI_KERNEL_PKG" 2>/dev/null || return 1
	pk_newest=$(rpm -q --qf '%{INSTALLTIME} %{VERSION}-%{RELEASE}\n' \
		"$PI_KERNEL_PKG" 2>/dev/null | sort -n | tail -n 1)
	pk_newest=${pk_newest#* }
	pk_running=$(uname -r)
	[ -n "$pk_newest" ] && [ "$pk_newest" != "$pk_running" ] || return 1
	REBOOT_REASON="$PI_KERNEL_PKG $pk_newest installed, running $pk_running"
	return 0
}

# Re-check the packages "needs-restarting -r" listed (its output in $1)
# against the kernel boot time. needs-restarting takes the boot time from
# systemd's UnitsLoadStartTimestamp, which on the RTC-less pi2/pi3 is taken
# before chronyd steps the clock: systemd starts the clock at its build
# epoch (2026-09-16 00:00:01 for systemd-252-67.el9_8.6), so every
# glibc/systemd/dbus-broker/linux-firmware installed after that date looked
# newer than every boot and the Pis rebooted daily (task p82).
# Returns 0 (reason in REBOOT_REASON) when a listed package really was
# installed after this boot, or when that cannot be ruled out (no parsable
# package list, unknown install time) — erring towards the reboot keeps
# genuine core updates effective. 1 when every listing predates the boot.
# 2 (reason in REBOOT_REASON) when btime cannot be trusted yet: the clock
# is not NTP-synchronised or btime is unreadable — deferred to a later tick.
confirm_core_updates() {
	typeset cc_boot cc_pkgs cc_pkg cc_inst cc_newer="" cc_stale=""
	if ! clock_synced; then
		REBOOT_REASON="clock not NTP-synchronised, boot time unknown"
		return 2
	fi
	cc_boot=$(kernel_boot_epoch)
	case $cc_boot in
	'' | *[!0-9]*)
		REBOOT_REASON="cannot read btime from $PROC_STAT"
		return 2
		;;
	esac
	cc_pkgs=$(printf '%s\n' "$1" | sed -n 's/^ *\* *//p')
	if [ -z "$cc_pkgs" ]; then
		REBOOT_REASON="needs-restarting -r reports a reboot (no package list)"
		return 0
	fi
	for cc_pkg in $cc_pkgs; do
		cc_inst=$(newest_install_epoch "$cc_pkg")
		if [ -z "$cc_inst" ] || [ "$cc_inst" -gt "$cc_boot" ]; then
			cc_newer="$cc_newer $cc_pkg"
		else
			cc_stale="$cc_stale $cc_pkg"
		fi
	done
	if [ -n "$cc_newer" ]; then
		REBOOT_REASON="core packages updated since boot:$cc_newer"
		return 0
	fi
	note "no reboot: needs-restarting -r listed$cc_stale, all installed before boot (btime $cc_boot)"
	return 1
}

# Decide whether a reboot is genuinely required. 0 = reboot (reason in
# REBOOT_REASON), 1 = not required, 2 = cannot decide yet (deferred).
# A needs-restarting error (rc other than 0/1) never reboots, as before.
reboot_needed() {
	typeset rn_out rn_rc
	REBOOT_REASON=
	pi_kernel_pending && return 0
	rn_out=$(needs-restarting -r 2>&1)
	rn_rc=$?
	case $rn_rc in
	0) return 1 ;;
	1) confirm_core_updates "$rn_out"; return $? ;;
	*)
		note "no reboot: needs-restarting -r failed (rc=$rn_rc)"
		return 1
		;;
	esac
}

# Stamp the day, release the lock and reboot.
do_reboot() {
	log "rebooting: $REBOOT_REASON"
	mkdir -p "$STAMP_DIR"
	printf '%s\n' "$today" >"$REBOOT_STAMP.tmp.$$" \
		&& mv "$REBOOT_STAMP.tmp.$$" "$REBOOT_STAMP"
	sync
	sleep 2
	rmdir "$LOCK" 2>/dev/null
	trap - EXIT
	systemctl reboot
}

maybe_reboot() {
	typeset mr_rc
	reboot_window_open || return 0
	reboot_needed
	mr_rc=$?
	case $mr_rc in
	0) do_reboot ;;
	2) log "reboot check deferred: $REBOOT_REASON" ;;
	esac
	return 0
}

write_stamp() {
	typeset tmp
	tmp=$STAMP.tmp.$$
	printf '%s\n' "$today" >"$tmp" && mv "$tmp" "$STAMP"
}

# Stamp already today → skip the update attempt, still check reboot.
if [ "$last" = "$today" ]; then
	maybe_reboot
	exit 0
fi

if ! partners_up; then
	if [ -n "$PARTNERS" ]; then
		log "skipped daily update: partner(s) not reachable (not stamped)"
	else
		log "skipped daily update: no partner configured for $(hostname) (not stamped)"
	fi
	maybe_reboot
	exit 0
fi

mkdir -p "$STAMP_DIR"

disable_dtail=""
if pkgrepo_up; then
	:
else
	log "WARNING: f3s-dtail repo not operational — dnf --disablerepo=f3s-dtail this window"
	disable_dtail=1
fi

if [ -n "$disable_dtail" ]; then
	out=$(dnf -y upgrade --disablerepo=f3s-dtail 2>&1)
	rc=$?
else
	out=$(dnf -y upgrade 2>&1)
	rc=$?
fi
if [ "$rc" -ne 0 ]; then
	log "dnf upgrade FAILED (rc=$rc) — not stamped, will retry next hour"
	printf '%s\n' "$out" | tee -a "$LOG"
	maybe_reboot
	exit 1
fi

if [ -n "$out" ] && ! printf '%s\n' "$out" | grep -qi 'Nothing to do'; then
	log "dnf upgrade:"
	printf '%s\n' "$out" | tee -a "$LOG"
fi

# Restart units that need it after library/package updates — before stamping
# so a crash mid-restart leaves the day unstamped and retries next hour.
# Skip our own oneshot unit: restarting it SIGTERMs this run before the stamp.
units=$(needs-restarting -s 2>/dev/null)
if [ -n "$units" ]; then
	printf '%s\n' "$units" | while IFS= read -r unit; do
		[ -n "$unit" ] || continue
		case $unit in
		unattended-upgrade-rocky.service) continue ;;
		esac
		log "restarting $unit"
		systemctl restart "$unit" >/dev/null 2>&1 \
			|| log "WARNING: systemctl restart $unit failed"
	done
fi

# Stamp only when the update attempt completed (success) and restarts were
# attempted. A no-op dnf still counts as completed.
write_stamp

maybe_reboot
exit 0
