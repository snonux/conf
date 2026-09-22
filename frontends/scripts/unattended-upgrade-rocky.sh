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
# The Pi kernel version of the last reboot done for a kernel mismatch.
KERNEL_REBOOT_STAMP=$STAMP_DIR/last-kernel-reboot
# Day of the last "kernel still not running after its reboot" warning.
KERNEL_WARN_STAMP=$STAMP_DIR/last-kernel-warning
# Boot id of the boot whose clock has been NTP-synchronised at least once
# (Pis only, see clock_trusted).
CLOCK_LATCH=$STAMP_DIR/clock-synced-boot
# "<boot id> <day>" of the last "clock not NTP-synchronised" warning.
CLOCK_WARN_STAMP=$STAMP_DIR/last-clock-warning
PROC_STAT=${UNATTENDED_UPGRADE_TEST_PROC_STAT:-/proc/stat}
BOOT_ID_FILE=${UNATTENDED_UPGRADE_TEST_BOOT_ID:-/proc/sys/kernel/random/boot_id}

# The pi2/pi3 SIG AltArch kernel. needs-restarting only knows the stock
# kernel names (kernel, kernel-rt), so a Pi kernel update is detected by
# comparing the running release with the most recently installed package.
# Its presence also marks a host as RTC-less (see pi_host).
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

# 0 on the Raspberry Pis (the Pi kernel package is installed). They have no
# RTC: until chronyd steps the clock after boot, the wall clock starts at
# the systemd build epoch, so dates, rpm install times and btime are all
# wrong. r0/r1/r2 are VMs with an RTC-backed clock and are not gated.
pi_host() {
	rpm -q --quiet "$PI_KERNEL_PKG" 2>/dev/null
}

# Write $2 (one line) to file $1 via temp + mv, so a crash never leaves a
# half-written stamp. Errors are silent: a missing stamp only repeats work.
write_atomic() {
	typeset wa_tmp
	wa_tmp=$1.tmp.$$
	mkdir -p "${1%/*}" 2>/dev/null
	{ printf '%s\n' "$2" >"$wa_tmp" && mv "$wa_tmp" "$1"; } 2>/dev/null
}

# 0 when the wall clock can be trusted for dates, rpm install times and
# btime. Always on r0/r1/r2 (RTC-backed; a stopped chronyd there must not
# defer anything). On the RTC-less Pis: once the clock has been
# NTP-synchronised during this boot — latched per boot id in $CLOCK_LATCH,
# so a later NTP blip (server unreachable, NTPSynchronized=no) does not
# matter: the clock was stepped right and only drifts slowly from there.
clock_trusted() {
	typeset ct_boot
	pi_host || return 0
	ct_boot=$(cat "$BOOT_ID_FILE" 2>/dev/null)
	if [ -n "$ct_boot" ] && [ -f "$CLOCK_LATCH" ] \
		&& [ "$(cat "$CLOCK_LATCH")" = "$ct_boot" ]; then
		return 0
	fi
	clock_synced || return 1
	[ -z "$ct_boot" ] || write_atomic "$CLOCK_LATCH" "$ct_boot"
	return 0
}

# Log the not-yet-synchronised skip once per boot and day. The marker text
# "WARNING: unattended-upgrade skipped: clock not NTP-synchronised" is
# stable so a future check can key off it (plan §9.5).
warn_clock_unsynced() {
	typeset wc_key
	wc_key="$(cat "$BOOT_ID_FILE" 2>/dev/null) $today"
	if [ -f "$CLOCK_WARN_STAMP" ] \
		&& [ "$(cat "$CLOCK_WARN_STAMP")" = "$wc_key" ]; then
		note "skipped $mode: clock not NTP-synchronised yet (not stamped)"
		return 0
	fi
	log "WARNING: unattended-upgrade skipped: clock not NTP-synchronised since boot (not stamped, retrying hourly)"
	write_atomic "$CLOCK_WARN_STAMP" "$wc_key"
}

# Newest install time (epoch s) of any installed version of package $1;
# prints nothing when rpm cannot tell (not installed, rpm error).
newest_install_epoch() {
	rpm -q --qf '%{INSTALLTIME}\n' "$1" 2>/dev/null \
		| grep -E '^[0-9]+$' | sort -n | tail -n 1
}

# Classify the Pi kernel into PK_STATE (with PK_NEWEST, PK_RUNNING):
#   none     no $PI_KERNEL_PKG (r0/r1/r2)
#   current  the kernel installed last is running
#   pending  another kernel was installed last: a reboot is due
#   stuck    this host already rebooted once for PK_NEWEST and still runs
#            PK_RUNNING — a config.txt added later pins a different image,
#            or the new one fails to boot and the firmware fell back.
#            Rebooting again would only repeat that daily, so this only
#            warns (warn_kernel_mismatch).
# Assumption: the Pi firmware boots the kernel image the latest package
# transaction wrote to /boot — pi2/pi3 have no /boot/config.txt, so the
# firmware boots /boot/kernel8.img, which each kernel package's posttrans
# overwrites. Ties in install time (one transaction) are broken by version.
pi_kernel_classify() {
	PK_STATE=none
	PK_NEWEST=
	PK_RUNNING=
	pi_host || return 0
	PK_NEWEST=$(rpm -q --qf '%{INSTALLTIME} %{VERSION}-%{RELEASE}\n' \
		"$PI_KERNEL_PKG" 2>/dev/null | sort -k1,1n -k2,2V | tail -n 1)
	PK_NEWEST=${PK_NEWEST#* }
	PK_RUNNING=$(uname -r)
	if [ -z "$PK_NEWEST" ] || [ "$PK_NEWEST" = "$PK_RUNNING" ]; then
		PK_STATE=current
	elif [ -f "$KERNEL_REBOOT_STAMP" ] \
		&& [ "$(cat "$KERNEL_REBOOT_STAMP")" = "$PK_NEWEST" ]; then
		PK_STATE=stuck
	else
		PK_STATE=pending
	fi
}

# Log (at most once a day) that the reboot for PK_NEWEST did not bring it
# up. Called before the reboot gates, so a partner outage cannot hide it.
warn_kernel_mismatch() {
	if [ -f "$KERNEL_WARN_STAMP" ] \
		&& [ "$(cat "$KERNEL_WARN_STAMP")" = "$today" ]; then
		return 0
	fi
	log "WARNING: already rebooted for $PI_KERNEL_PKG $PK_NEWEST but $PK_RUNNING is still running — check /boot/config.txt and the boot; not rebooting for it again"
	write_atomic "$KERNEL_WARN_STAMP" "$today"
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
# 2 (reason in REBOOT_REASON) when btime is unreadable — deferred to a
# later tick. A Pi whose clock was never synchronised this boot never gets
# here (maybe_reboot requires clock_trusted).
confirm_core_updates() {
	typeset cc_boot cc_pkgs cc_pkg cc_inst cc_newer="" cc_stale=""
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

# Decide whether a reboot is genuinely required (PK_STATE must be set by
# pi_kernel_classify). 0 = reboot (reason in REBOOT_REASON, a kernel target
# in REBOOT_KERNEL), 1 = not required, 2 = cannot decide yet (deferred).
# A needs-restarting error (rc other than 0/1) never reboots, as before.
reboot_needed() {
	typeset rn_out rn_rc
	REBOOT_REASON=
	REBOOT_KERNEL=
	if [ "$PK_STATE" = pending ]; then
		REBOOT_REASON="$PI_KERNEL_PKG $PK_NEWEST installed, running $PK_RUNNING"
		REBOOT_KERNEL=$PK_NEWEST
		return 0
	fi
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

# Stamp the day (and the target kernel of a kernel reboot, so a mismatch
# that survives the reboot is not rebooted for again), release the lock and
# reboot.
do_reboot() {
	log "rebooting: $REBOOT_REASON"
	write_atomic "$REBOOT_STAMP" "$today"
	[ -z "$REBOOT_KERNEL" ] \
		|| write_atomic "$KERNEL_REBOOT_STAMP" "$REBOOT_KERNEL"
	sync
	sleep 2
	rmdir "$LOCK" 2>/dev/null
	trap - EXIT
	systemctl reboot
}

# The reboot check. Needs a trusted clock (before the first NTP sync of a
# Pi boot, "today" and btime are wrong; the main flow warns about that).
# The stuck-kernel warning comes before the gates so that it shows even
# while a partner is down or the host already rebooted today.
maybe_reboot() {
	typeset mr_rc
	clock_trusted || return 0
	pi_kernel_classify
	[ "$PK_STATE" != stuck ] || warn_kernel_mismatch
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
	write_atomic "$STAMP" "$today"
}

# Stamp already today → skip the update attempt, still check reboot (which
# is not blocked by a later NTP blip: clock_trusted latches per boot).
if [ "$last" = "$today" ]; then
	maybe_reboot
	exit 0
fi

# Clock gate for the update (Pis only, see clock_trusted): until the
# RTC-less clock has been NTP-synchronised once this boot, "today" and
# every rpm install time would be wrong. A dnf run now would record core
# updates as installed before the real boot, so after the clock steps the
# reboot check would call them stale and never reboot for them. Skip the
# run, stamping nothing; the next hourly tick retries. (The timer has
# OnBootSec=10min and no time-sync.target ordering; chrony-wait is not
# enabled on these hosts.)
if ! clock_trusted; then
	warn_clock_unsynced
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
