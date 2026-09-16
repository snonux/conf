#!/bin/ksh
#
# unattended-upgrade-rocky — unattended package updates for Rocky pi2/pi3
# (and later r0/r1/r2 on the same pattern).
#
#   daily   once-per-day dnf upgrade (stamp-gated) + every-tick reboot check
#
# Timer-driven (NO script jitter — per-host OnCalendar offsets are the
# anti-coincidence mechanism). Logs to the journal via stdout and to
# /var/log/unattended-upgrade.log. Companion plan:
# frontends/docs/unattended-upgrades-pi.plan.md §12.
#
# Rocky ships AT&T ksh93u+m — use typeset, not local (local is a pdkshism).

PATH=/usr/bin:/bin:/usr/sbin:/sbin:/usr/local/bin:/usr/local/sbin
export PATH

umask 077

LOG=/var/log/unattended-upgrade.log
LOCK=/var/run/unattended-upgrade.lock
STAMP_DIR=/var/lib/unattended-upgrade
STAMP=$STAMP_DIR/last-daily
REBOOT_STAMP=$STAMP_DIR/last-reboot

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
# At most one unattended reboot per calendar day: on these Rocky Pis,
# needs-restarting -r can still report pending after a fresh reboot
# (dbus/glibc/linux-firmware/systemd), which would otherwise loop with
# OnBootSec. Weekday stagger for r0/r1/r2 so two k3s nodes never reboot
# the same day (date +%u % 3: Mon/Thu/Sun→r0, Tue/Fri→r1, Wed/Sat→r2).
maybe_reboot() {
	if [ -f "$REBOOT_STAMP" ] && [ "$(cat "$REBOOT_STAMP")" = "$today" ]; then
		return 0
	fi
	if ! partners_up; then
		log "reboot check deferred: partner(s) not reachable"
		return 0
	fi
	case $(hostname -s) in
	r0) [ $(($(date +%u) % 3)) -eq 1 ] || return 0 ;;
	r1) [ $(($(date +%u) % 3)) -eq 2 ] || return 0 ;;
	r2) [ $(($(date +%u) % 3)) -eq 0 ] || return 0 ;;
	esac
	# needs-restarting -r: exit 0 = no reboot needed, exit 1 = reboot required.
	needs-restarting -r >/dev/null 2>&1
	rc=$?
	if [ "$rc" -eq 1 ]; then
		log "rebooting: needs-restarting -r reports pending updates"
		mkdir -p "$STAMP_DIR"
		printf '%s\n' "$today" >"$REBOOT_STAMP.tmp.$$" \
			&& mv "$REBOOT_STAMP.tmp.$$" "$REBOOT_STAMP"
		sync
		sleep 2
		rmdir "$LOCK" 2>/dev/null
		trap - EXIT
		systemctl reboot
	fi
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
