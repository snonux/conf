#!/bin/ksh
#
# unattended-upgrade-netbsd — unattended package updates for NetBSD pi0/pi1.
#
#   pkgs    pkgin upgrade + probe-gated custom fleet pkg_add -u, restart daemons
#   reboot  reboot if the on-disk /netbsd kernel differs from the booted one
#
# Phase 1: no base-system updates (sysupdate not in pkgin; see plan §3).
# Cron-driven with jitter. Logs to /var/log/unattended-upgrade.log (no MTA
# on the Pis — journal/log observation only). Companion plan:
# frontends/docs/unattended-upgrades-pi.plan.md
#
# Every mode is gated on the partner Pi serving the static site marker over
# HTTP. Patching or rebooting the only healthy static-site host would take
# f3s.buetow.org / snonux.foo down behind relayd.

# Non-interactive NetBSD root shells lack pkgsrc and /sbin — pkgin, sysctl,
# and reboot would otherwise be "not found".
PATH=/usr/bin:/bin:/usr/sbin:/sbin:/usr/pkg/bin:/usr/pkg/sbin:/usr/local/bin:/usr/local/sbin
export PATH

umask 077

LOG=/var/log/unattended-upgrade.log
SERVICES=/etc/unattended-upgrade-services
LOCK=/var/run/unattended-upgrade.lock

readonly LOOKUP_TIMEOUT=10
readonly HEALTH_TRIES=3
readonly HEALTH_RETRY_SLEEP=2
# Static-site marker (verified live 2026-09-16). bozohttpd -X means a
# non-empty body alone could be a directory listing of a broken docroot.
readonly PARTNER_MARKER='Hello, it works'

mode=${1:-}
case $mode in
pkgs|reboot) ;;
*) print -u2 "usage: $0 pkgs|reboot"; exit 64 ;;
esac

log() {
	printf '[%s] %s\n' "$(date '+%Y-%m-%dT%H:%M:%S')" "$*" | tee -a "$LOG"
}

# Jitter only for cron runs (no tty); manual runs are instant.
[ -t 0 ] || sleep $((RANDOM % 1200))

case $(hostname -s) in
pi0) PARTNER=pi1.lan.buetow.org ;;
pi1) PARTNER=pi0.lan.buetow.org ;;
*) PARTNER= ;;
esac

# Partner must serve the static-site marker over HTTP on port 80. DNS for
# *.lan.buetow.org comes from /etc/hosts on these Pis (wg0 names do not
# resolve here). Failures are retried HEALTH_TRIES times.
partner_up() {
	[ -n "$PARTNER" ] || return 1
	local -i attempt=1
	local -i ok=0
	while [ $attempt -le $HEALTH_TRIES ]; do
		if timeout $LOOKUP_TIMEOUT ftp -o - \
			"http://$PARTNER/" 2>/dev/null \
			| grep -q "$PARTNER_MARKER"; then
			ok=1
			break
		fi
		attempt=$((attempt + 1))
		[ $attempt -le $HEALTH_TRIES ] && sleep $HEALTH_RETRY_SLEEP
	done
	[ $ok -eq 1 ]
}

# Custom fleet pkgrepo is k3s-backed; relayd serves an HTTP-200
# "Server turned off" page when the backend sleeps. Require a non-empty
# listing without that marker and with .tgz entries.
pkgrepo_up() {
	local body
	local ver arch
	ver=$(uname -r)
	arch=$(uname -p)
	body=$(timeout $LOOKUP_TIMEOUT ftp -o - \
		"https://pkgrepo.f3s.buetow.org/netbsd/${ver}/packages/${arch}/" \
		2>/dev/null) || return 1
	[ -n "$body" ] || return 1
	printf '%s' "$body" | grep -q "Server turned off" && return 1
	printf '%s' "$body" | grep -q "\.tgz"
}

# Whole-job lock. Steal locks older than 2 h (crash / power loss).
# Log to $LOG (no MTA on the Pis — syslog-only skips are easy to miss).
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

if ! partner_up; then
	if [ -n "$PARTNER" ]; then
		log "skipped $mode: partner $PARTNER not operational"
	else
		log "skipped $mode: no partner configured for $(hostname)"
	fi
	exit 0
fi

# Compare the on-disk kernel (what /netbsd) with the booted version
# (sysctl kern.version line 1). Do NOT use /var/run/dmesg.boot line 1 —
# on NetBSD that is the copyright line. Trim whitespace; fail-safe = no
# reboot when either side is empty.
kernel_reboot_pending() {
	local ondisk booted
	ondisk=$(what /netbsd 2>/dev/null | grep -m1 NetBSD \
		| sed 's/^[[:space:]]*//;s/[[:space:]]*$//')
	booted=$(sysctl -n kern.version 2>/dev/null | sed -n '1p' \
		| sed 's/^[[:space:]]*//;s/[[:space:]]*$//')
	[ -n "$ondisk" ] && [ -n "$booted" ] && [ "$ondisk" != "$booted" ]
}

restart_services() {
	[ -f "$SERVICES" ] || return 0
	while IFS= read -r svc; do
		[ -n "$svc" ] || continue
		case $svc in \#*) continue ;; esac
		if [ -x "/etc/rc.d/$svc" ]; then
			if /etc/rc.d/"$svc" status >/dev/null 2>&1; then
				log "restarting $svc"
				/etc/rc.d/"$svc" restart >/dev/null 2>&1 \
					|| log "WARNING: /etc/rc.d/$svc restart failed"
			else
				log "not running: $svc — skipped"
			fi
		else
			log "no rc.d script: $svc — skipped"
		fi
	done <"$SERVICES"
}

case $mode in

pkgs)
	for mp in / /usr /var; do
		avail=$(df -k "$mp" | awk 'NR==2 {print $4}')
		if [ "${avail:-0}" -lt 262144 ]; then
			log "pkgs aborted: only ${avail}KB free on $mp (need 256MB)"
			exit 1
		fi
	done

	# Official pkgsrc via pkgin (custom fleet repo is NOT in
	# repositories.conf — pkgin never touches dtail/f3sctl).
	# pkgin always prints "calculating dependencies...done." — a non-empty
	# body is NOT a change. Real no-ops end with "nothing to do."
	pkgin_changed=""
	out=$(pkgin -y upgrade 2>&1)
	rc=$?
	if [ "$rc" -ne 0 ]; then
		log "pkgin -y upgrade FAILED (rc=$rc) — investigate"
		printf '%s\n' "$out" | tee -a "$LOG"
		exit 1
	fi
	if printf '%s\n' "$out" | grep -q 'nothing to do'; then
		: # clean no-op
	else
		pkgin_changed=1
		log "pkgin -y upgrade:"
		printf '%s\n' "$out" | tee -a "$LOG"
	fi

	# Custom fleet packages: bare-stem pkg_add -u with PKG_PATH (same
	# wildcard semantics as OpenBSD). Probe-gated — skip with WARNING
	# when k3s is asleep (not a hard failure). No-op lines look like
	# "already recorded as installed"; anything else counts as a change.
	ver=$(uname -r)
	arch=$(uname -p)
	custom="https://pkgrepo.f3s.buetow.org/netbsd/${ver}/packages/${arch}/"
	custom_changed=""
	custom_failed=""
	if pkgrepo_up; then
		PKG_PATH="$custom"
		export PKG_PATH
		cout=$(pkg_add -u dtail f3sctl 2>&1)
		crc=$?
		if [ "$crc" -ne 0 ]; then
			log "pkg_add -u dtail f3sctl FAILED (rc=$crc) — investigate"
			printf '%s\n' "$cout" | tee -a "$LOG"
			custom_failed=1
			# Partial applies may have updated one stem before failing —
			# bounce daemons so any new binaries are picked up.
			custom_changed=1
		elif [ -n "$cout" ] \
			&& printf '%s\n' "$cout" | grep -qv 'already recorded as installed'
		then
			custom_changed=1
			log "pkg_add -u dtail f3sctl:"
			printf '%s\n' "$cout" | tee -a "$LOG"
		fi
	else
		log "WARNING: ${custom} not operational — skipping custom-repo packages this window"
	fi

	# Restart when pkgsrc changed even if custom pkg_add failed — otherwise
	# updated binaries keep running old code until a later clean window.
	if [ -n "$pkgin_changed" ] || [ -n "$custom_changed" ]; then
		restart_services
	fi
	[ -z "$custom_failed" ] || exit 1
	;;

reboot)
	if kernel_reboot_pending; then
		log "rebooting to activate the new kernel"
		sync
		sleep 2
		rmdir "$LOCK" 2>/dev/null
		trap - EXIT
		reboot
	fi
	;;
esac

exit 0
