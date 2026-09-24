#!/usr/local/bin/ksh93
#
# unattended-upgrade-freebsd — unattended package updates for FreeBSD
# hypervisors f0–f3 (hourly stamp-gated, Rocky on-demand pattern).
#
#   daily   once-per-day pkg upgrade (stamp-gated) + every-tick reboot check
#
# Cron-driven (per-host minute offsets). Partner ping gates reboots only
# (not pkg) so a sibling powered off on purpose does not block patches.
# f3 never auto-reboots. Companion:
# frontends/docs/unattended-upgrades.md (plan record:
# docs/archive/frontends/docs/unattended-upgrades-freebsd.plan.md)
#
# FreeBSD shells/ksh package ships /usr/local/bin/ksh93 by default (KSH
# option off). Use typeset, not local.

PATH=/usr/bin:/bin:/usr/sbin:/sbin:/usr/local/bin:/usr/local/sbin
export PATH

umask 077

LOG=/var/log/unattended-upgrade.log
LOCK=/var/run/unattended-upgrade.lock
STAMP_DIR=/var/lib/unattended-upgrade
STAMP=$STAMP_DIR/last-daily
REBOOT_STAMP=$STAMP_DIR/last-reboot
SERVICES=/etc/unattended-upgrade-services

readonly LOOKUP_TIMEOUT=10
readonly HEALTH_TRIES=3
readonly HEALTH_RETRY_SLEEP=2
readonly VM_STOP_WAIT_TRIES=60
readonly VM_STOP_WAIT_SLEEP=5

mode=${1:-}
case $mode in
daily) ;;
*) print -u2 "usage: $0 daily"; exit 64 ;;
esac

log() {
	printf '[%s] %s\n' "$(date '+%Y-%m-%dT%H:%M:%S')" "$*" | tee -a "$LOG"
}

# Reboot partners only. f3 has none (never auto-reboots).
# Weekday stagger is offset from r0/r1/r2 so hypervisor and guest do not
# share a reboot day (f0→%3==2, f1→0, f2→1; r-nodes use 1/2/0).
host=$(hostname -s)
case $host in
f0) PARTNERS="192.168.1.131 192.168.1.132"; ALLOW_REBOOT=1; REBOOT_SLOT=2 ;;
f1) PARTNERS="192.168.1.130 192.168.1.132"; ALLOW_REBOOT=1; REBOOT_SLOT=0 ;;
f2) PARTNERS="192.168.1.130 192.168.1.131"; ALLOW_REBOOT=1; REBOOT_SLOT=1 ;;
f3) PARTNERS=; ALLOW_REBOOT=0; REBOOT_SLOT= ;;
*)  PARTNERS=; ALLOW_REBOOT=0; REBOOT_SLOT= ;;
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
			# FreeBSD: -t is seconds; -W is milliseconds (do not copy Linux -W3).
			if timeout $LOOKUP_TIMEOUT ping -c1 -t 3 "$ip" >/dev/null 2>&1; then
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

# Custom FreeBSD pkg repo is k3s-backed; relayd serves "Server turned off"
# when the backend sleeps. Probe the listing; skip custom (not official)
# when it is down.
pkgrepo_up() {
	typeset body abi
	abi=$(pkg config ABI 2>/dev/null) || return 1
	body=$(timeout $LOOKUP_TIMEOUT curl -fsS \
		"https://pkgrepo.f3s.buetow.org/freebsd/${abi}/latest/" \
		2>/dev/null) || return 1
	[ -n "$body" ] || return 1
	printf '%s' "$body" | grep -q "Server turned off" && return 1
	# Any package index / listing content counts as up.
	printf '%s' "$body" | grep -qiE 'packagesite|\.pkg|meta\.conf|data'
}

kernel_reboot_pending() {
	typeset installed running
	installed=$(freebsd-version -k 2>/dev/null) || return 1
	running=$(freebsd-version -r 2>/dev/null) || return 1
	[ -n "$installed" ] && [ -n "$running" ] && [ "$installed" != "$running" ]
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

write_stamp() {
	typeset tmp
	tmp=$STAMP.tmp.$$
	mkdir -p "$STAMP_DIR"
	printf '%s\n' "$today" >"$tmp" && mv "$tmp" "$STAMP"
}

stop_vms_then_reboot() {
	typeset -i attempt
	log "stopping bhyve guests (vm stopall) before reboot"
	vm stopall >/dev/null 2>&1 || log "WARNING: vm stopall returned non-zero"
	attempt=1
	while [ $attempt -le $VM_STOP_WAIT_TRIES ]; do
		if ! vm list 2>/dev/null | grep -q Running; then
			break
		fi
		attempt=$((attempt + 1))
		sleep $VM_STOP_WAIT_SLEEP
	done
	if vm list 2>/dev/null | grep -q Running; then
		log "WARNING: guests still Running after wait — refusing reboot"
		return 1
	fi
	mkdir -p "$STAMP_DIR"
	printf '%s\n' "$today" >"$REBOOT_STAMP.tmp.$$" \
		&& mv "$REBOOT_STAMP.tmp.$$" "$REBOOT_STAMP"
	sync
	sleep 2
	rmdir "$LOCK" 2>/dev/null
	trap - EXIT
	# reboot(8), never shutdown -r — bypasses rc.shutdown watchdog (single-user trap).
	log "rebooting"
	reboot
}

# Reboot check every tick. Partner gate + weekday slot for f0–f2 only.
maybe_reboot() {
	if [ "$ALLOW_REBOOT" -ne 1 ]; then
		if kernel_reboot_pending; then
			log "kernel reboot pending (installed=$(freebsd-version -k), running=$(freebsd-version -r)) — f3 never auto-reboots"
		fi
		return 0
	fi
	if [ -f "$REBOOT_STAMP" ] && [ "$(cat "$REBOOT_STAMP")" = "$today" ]; then
		return 0
	fi
	if ! kernel_reboot_pending; then
		return 0
	fi
	if [ -n "$PARTNERS" ] && ! partners_up; then
		log "reboot deferred: partner(s) not reachable"
		return 0
	fi
	if [ -n "$REBOOT_SLOT" ]; then
		[ $(($(date +%u) % 3)) -eq "$REBOOT_SLOT" ] || return 0
	fi
	log "kernel reboot pending (installed=$(freebsd-version -k), running=$(freebsd-version -r))"
	stop_vms_then_reboot || return 0
}

restart_services() {
	typeset name
	[ -f "$SERVICES" ] || return 0
	while IFS= read -r name || [ -n "$name" ]; do
		case $name in
		''|\#*) continue ;;
		esac
		# Never restart vm/network from this list — guest lifecycle is reboot-only.
		case $name in
		vm|vm_network|netif|routing|devd) continue ;;
		esac
		if [ -x "/etc/rc.d/$name" ] || [ -x "/usr/local/etc/rc.d/$name" ]; then
			log "restarting $name"
			service "$name" restart >/dev/null 2>&1 \
				|| log "WARNING: service $name restart failed"
		else
			log "WARNING: service $name not found, skipped"
		fi
	done <"$SERVICES"
}

# Stamp already today → skip the update attempt, still check reboot.
if [ "$last" = "$today" ]; then
	maybe_reboot
	exit 0
fi

mkdir -p "$STAMP_DIR"

disable_custom=""
if pkgrepo_up; then
	:
else
	log "WARNING: custom FreeBSD pkgrepo not operational — official FreeBSD-ports repos only this window"
	disable_custom=1
fi

# Official-only fallback: pkg -r filters by repo NAME. On FreeBSD 15.x the
# stock /etc/pkg/FreeBSD.conf repos are FreeBSD-ports and
# FreeBSD-ports-kmods — nothing is named FreeBSD (the pre-15 default), so
# the old '-r FreeBSD' matched zero repos and every fallback window failed
# rc=3 "No repositories are enabled" while the custom repo was down
# (observed on f3/f2 2026-09-17..19: 43 resp. 12 failed windows).
if [ -n "$disable_custom" ]; then
	out=$(ASSUME_ALWAYS_YES=yes pkg upgrade -y -r FreeBSD-ports -r FreeBSD-ports-kmods 2>&1)
	rc=$?
else
	out=$(ASSUME_ALWAYS_YES=yes pkg upgrade -y 2>&1)
	rc=$?
fi
if [ "$rc" -ne 0 ]; then
	log "pkg upgrade FAILED (rc=$rc) — not stamped, will retry next hour"
	printf '%s\n' "$out" | tee -a "$LOG"
	maybe_reboot
	exit 1
fi

if [ -n "$out" ] && ! printf '%s\n' "$out" | grep -qiE 'Your packages are up to date|already the latest|No packages'; then
	log "pkg upgrade:"
	printf '%s\n' "$out" | tee -a "$LOG"
fi

# Restarts before stamp so a crash mid-restart leaves the day unstamped.
restart_services

write_stamp
maybe_reboot
exit 0
