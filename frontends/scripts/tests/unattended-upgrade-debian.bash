#!/bin/bash
# Exercise unattended-upgrade-debian.sh (task l82) without a Debian Pi:
# hostname, ping, timedatectl, systemctl, apt-get, unattended-upgrade,
# needrestart, sleep, sync and timeout are faked; /etc/os-release, the
# device-tree model, the boot id and /run/reboot-required are fixtures.
#
# Positive paths: the daily run (apt-get update, unattended-upgrade with
# needrestart's dpkg hook suspended, one needrestart -r a pass, day stamp),
# and each reboot signal (pending kernel, /run/reboot-required, services
# left on outdated libraries). Negative paths that must neither update nor
# reboot: a non-Debian host (the destination guard), a Pi clock never
# NTP-synchronised this boot, a stale clock latch, the partner gate, no
# partner configured, the once-per-day reboot backstop, a kernel that
# survived its reboot (warns once a day instead), a failed needrestart
# kernel check, only our own unit listed as stale, failed apt-get update /
# unattended-upgrade runs (not stamped), a held lock and a bad mode.

set -euo pipefail

script_dir=$(cd "$(dirname "$0")/.." && pwd)
script="$script_dir/unattended-upgrade-debian.sh"
work=$(mktemp -d "${TMPDIR:-/tmp}/unattended-debian-test.XXXXXX")
trap 'rm -rf "$work"' EXIT

fake="$work/bin"
state="$work/state"
mkdir "$fake"

for tool in sleep sync; do
	printf '#!/bin/sh\nexit 0\n' >"$fake/$tool"
done

cat >"$fake/hostname" <<'EOF'
#!/bin/sh
printf '%s\n' "${FAKE_HOST:-pi2}"
EOF

# Records each pinged IP; FAKE_PING=down fails all, FAKE_PING=<ip> that one.
cat >"$fake/ping" <<'EOF'
#!/bin/sh
for ip; do :; done
printf '%s\n' "$ip" >>"${FAKE_PING_LOG:?}"
[ "${FAKE_PING:-up}" != down ] && [ "${FAKE_PING:-up}" != "$ip" ]
EOF

cat >"$fake/timeout" <<'EOF'
#!/bin/sh
shift
exec "$@"
EOF

cat >"$fake/timedatectl" <<'EOF'
#!/bin/sh
printf '%s\n' "${FAKE_NTP:-yes}"
EOF

cat >"$fake/systemctl" <<'EOF'
#!/bin/sh
printf '%s\n' "$*" >>"${FAKE_SYSTEMCTL_LOG:?}"
EOF

# apt-get update: FAKE_APT_UPDATE=ok|warn|fail.
cat >"$fake/apt-get" <<'EOF'
#!/bin/sh
printf '%s\n' "$*" >>"${FAKE_APT_LOG:?}"
case ${FAKE_APT_UPDATE:-ok} in
warn)
	printf 'Hit:1 http://deb.debian.org/debian trixie InRelease\n'
	printf 'Err:2 https://download.docker.com/linux/debian trixie InRelease\n'
	printf 'W: Some index files failed to download.\n'
	;;
fail) printf 'E: Could not get lock /var/lib/apt/lists/lock\n'; exit 100 ;;
*) printf 'Hit:1 http://deb.debian.org/debian trixie InRelease\n' ;;
esac
EOF

# unattended-upgrade: records NEEDRESTART_SUSPEND; FAKE_UU=none|upgrade|fail.
cat >"$fake/unattended-upgrade" <<'EOF'
#!/bin/sh
printf 'suspend=%s %s\n' "${NEEDRESTART_SUSPEND:-}" "$*" >>"${FAKE_UU_LOG:?}"
case ${FAKE_UU:-none} in
upgrade) printf 'Packages that will be upgraded: libc6 openssl\n' ;;
fail) printf 'An error occurred: dpkg returned an error code\n'; exit 1 ;;
*) printf 'No packages found that can be upgraded unattended and no pending auto-removals\n' ;;
esac
EOF

# needrestart: -k prints the kernel status from FAKE_KSTA/KCUR/KEXP (exit
# FAKE_NR_K_RC); "-r a" records the restart pass; "-r l -l" lists the
# services in $FAKE_NR_SVC (one unit per line).
cat >"$fake/needrestart" <<'EOF'
#!/bin/sh
printf '%s\n' "$*" >>"${FAKE_NR_LOG:?}"
case " $* " in
*" -k "*)
	[ "${FAKE_NR_K_RC:-0}" -eq 0 ] || exit "$FAKE_NR_K_RC"
	printf 'NEEDRESTART-VER: 3.8\n'
	printf 'NEEDRESTART-KCUR: %s\n' "${FAKE_KCUR:?}"
	printf 'NEEDRESTART-KEXP: %s\n' "${FAKE_KEXP:?}"
	printf 'NEEDRESTART-KSTA: %s\n' "${FAKE_KSTA:?}"
	;;
*" -r a "*) printf 'Restarting services...\n systemctl restart cron.service\n' ;;
*" -r l "*)
	printf 'NEEDRESTART-VER: 3.8\n'
	while IFS= read -r unit; do
		[ -n "$unit" ] && printf 'NEEDRESTART-SVC: %s\n' "$unit"
	done <"${FAKE_NR_SVC:?}"
	;;
esac
exit 0
EOF

chmod +x "$fake"/*

readonly KCUR=6.12.107+deb13-arm64
readonly KNEW=6.12.110+deb13-arm64
readonly CLOCK_MARKER='WARNING: unattended-upgrade skipped: clock not NTP-synchronised'

# Reset the state for one case: fresh stamp dir (daily stamp for today
# unless $1 is "fresh-day", so only the reboot check runs), Debian
# os-release, a Pi device-tree model, boot id, no reboot flag, the current
# kernel running, no stale services, and the fake knobs.
reset_case() {
	rm -rf "$state" "$work/lock"
	mkdir "$state"
	[[ "${1:-}" == fresh-day ]] || date +%F >"$state/last-daily"
	printf 'PRETTY_NAME="Debian GNU/Linux 13 (trixie)"\nID=debian\n' \
		>"$work/os-release"
	printf 'Raspberry Pi 3 Model B Plus Rev 1.3\0' >"$work/model"
	printf 'boot-a\n' >"$work/boot_id"
	rm -f "$work/reboot-required" "$work/reboot-required.pkgs"
	: >"$work/svc"
	fake_ntp=yes
	fake_ping=up
	fake_host=pi2
	fake_apt=ok
	fake_uu=none
	fake_ksta=1
	fake_kexp=$KCUR
	fake_nr_k_rc=0
	for f in systemctl apt uu nr ping stdout stderr; do
		: >"$work/$f.log"
	done
}

run_script() {
	UNATTENDED_UPGRADE_TEST_PATH="$fake" \
		UNATTENDED_UPGRADE_TEST_LOG="$state/log" \
		UNATTENDED_UPGRADE_TEST_LOCK="$work/lock" \
		UNATTENDED_UPGRADE_TEST_STAMP_DIR="$state" \
		UNATTENDED_UPGRADE_TEST_OS_RELEASE="$work/os-release" \
		UNATTENDED_UPGRADE_TEST_DT_MODEL="$work/model" \
		UNATTENDED_UPGRADE_TEST_BOOT_ID="$work/boot_id" \
		UNATTENDED_UPGRADE_TEST_REBOOT_FLAG="$work/reboot-required" \
		FAKE_SYSTEMCTL_LOG="$work/systemctl.log" FAKE_APT_LOG="$work/apt.log" \
		FAKE_UU_LOG="$work/uu.log" FAKE_NR_LOG="$work/nr.log" \
		FAKE_PING_LOG="$work/ping.log" FAKE_NR_SVC="$work/svc" \
		FAKE_NTP="$fake_ntp" FAKE_PING="$fake_ping" FAKE_HOST="$fake_host" \
		FAKE_APT_UPDATE="$fake_apt" FAKE_UU="$fake_uu" \
		FAKE_KSTA="$fake_ksta" FAKE_KCUR="$KCUR" FAKE_KEXP="$fake_kexp" \
		FAKE_NR_K_RC="$fake_nr_k_rc" \
		bash "$script" "${1:-daily}" >"$work/stdout.log" 2>"$work/stderr.log"
}

fail() {
	printf 'FAIL: %s\n' "$1" >&2
	for f in "$state/log" "$work/stdout.log" "$work/stderr.log" \
		"$work/systemctl.log"; do
		[[ -f "$f" ]] && { printf -- '--- %s\n' "$f" >&2; cat "$f" >&2; }
	done
	exit 1
}

expect_reboot() {
	grep -qx 'reboot' "$work/systemctl.log" || fail "$1: no reboot"
	[[ "$(cat "$state/last-reboot")" == "$(date +%F)" ]] \
		|| fail "$1: reboot stamp missing"
	grep -qF "rebooting: $2" "$state/log" || fail "$1: reason '$2' not logged"
}

expect_no_reboot() {
	if grep -q 'reboot' "$work/systemctl.log"; then
		fail "$1: unexpected reboot"
	fi
	[[ ! -f "$state/last-reboot" ]] || fail "$1: reboot stamp written"
}

expect_no_update() {
	[[ ! -s "$work/apt.log" ]] || fail "$1: apt-get ran"
	[[ ! -s "$work/uu.log" ]] || fail "$1: unattended-upgrade ran"
}

expect_not_stamped() {
	[[ ! -f "$state/last-daily" ]] || fail "$1: daily stamp written"
}

# 1. Full daily path, nothing pending: update, upgrade with the dpkg hook
# suspended, one restart pass, stamp; no reboot.
reset_case fresh-day
run_script || fail "daily path: script failed"
grep -qx -- '-q update' "$work/apt.log" || fail "daily path: no apt-get update"
grep -qx 'suspend=1 -v' "$work/uu.log" \
	|| fail "daily path: unattended-upgrade not run with NEEDRESTART_SUSPEND=1"
grep -qx -- '-r a -l' "$work/nr.log" || fail "daily path: no needrestart -r a"
[[ "$(cat "$state/last-daily")" == "$(date +%F)" ]] \
	|| fail "daily path: daily stamp missing"
expect_no_reboot "daily path"
grep -qx 192.168.1.128 "$work/ping.log" || fail "daily path: pi3 not pinged"
[[ ! -e "$work/lock" ]] || fail "daily path: lock left behind"

# 1a. Upgrades are logged; a quiet no-op day is not.
grep -q 'unattended-upgrade:' "$state/log" \
	&& fail "no-op day: unattended-upgrade output logged"
reset_case fresh-day
fake_uu=upgrade
run_script || fail "upgrade day: script failed"
grep -q 'Packages that will be upgraded: libc6 openssl' "$state/log" \
	|| fail "upgrade day: upgrade not logged"

# 2. The destination guard: a Rocky host is refused before anything runs.
for id in 'ID="rocky"' 'ID=ubuntu' ''; do
	reset_case fresh-day
	printf '%s\n' "$id" >"$work/os-release"
	if run_script; then
		fail "non-Debian ($id): script succeeded"
	fi
	grep -q 'WARNING: unattended-upgrade-debian refused' "$work/stderr.log" \
		|| fail "non-Debian ($id): refusal not reported"
	expect_no_update "non-Debian ($id)"
	expect_no_reboot "non-Debian ($id)"
	[[ ! -s "$work/nr.log" && ! -s "$work/ping.log" ]] \
		|| fail "non-Debian ($id): probes ran"
	[[ ! -e "$work/lock" && ! -e "$state/log" ]] \
		|| fail "non-Debian ($id): lock or log created"
done
reset_case fresh-day
rm "$work/os-release"
if run_script; then
	fail "missing os-release: script succeeded"
fi
expect_no_update "missing os-release"

# 3. A newer kernel installed: reboot and remember the target.
reset_case
fake_ksta=3
fake_kexp=$KNEW
run_script || fail "new kernel: script failed"
expect_reboot "new kernel" "kernel $KNEW installed, running $KCUR"
[[ "$(cat "$state/last-kernel-reboot")" == "$KNEW" ]] \
	|| fail "new kernel: target kernel not stamped"
expect_no_update "new kernel (stamped day)"

# 3a. ABI-compatible kernel upgrade (KSTA 2) reboots too.
reset_case
fake_ksta=2
run_script || fail "abi kernel: script failed"
expect_reboot "abi kernel" "kernel $KCUR installed"

# 4. Already rebooted for that kernel and it still is not running: warn
# once a day (even with the partner down), no second reboot.
reset_case
fake_ksta=3
fake_kexp=$KNEW
printf '%s\n' "$KNEW" >"$state/last-kernel-reboot"
fake_ping=down
run_script || fail "stuck kernel: script failed"
expect_no_reboot "stuck kernel"
grep -qF "WARNING: already rebooted for kernel $KNEW but $KCUR is still running" \
	"$state/log" || fail "stuck kernel: warning missing"
fake_ping=up
run_script || fail "stuck kernel (2nd tick): script failed"
expect_no_reboot "stuck kernel (2nd tick)"
[[ "$(grep -c 'WARNING: already rebooted' "$state/log")" -eq 1 ]] \
	|| fail "stuck kernel: warning repeated the same day"
# ... but another reboot reason still reboots.
touch "$work/reboot-required"
run_script || fail "stuck kernel + flag: script failed"
expect_reboot "stuck kernel + flag" "$work/reboot-required present"

# 4a. A newer target than the stamped one reboots again.
reset_case
fake_ksta=3
fake_kexp=$KNEW
printf '6.12.108+deb13-arm64\n' >"$state/last-kernel-reboot"
run_script || fail "newer kernel target: script failed"
expect_reboot "newer kernel target" "kernel $KNEW installed"

# 5. /run/reboot-required with its package list.
reset_case
touch "$work/reboot-required"
printf 'linux-image-arm64\nlinux-image-6.12.110+deb13-arm64\nlinux-image-arm64\n' \
	>"$work/reboot-required.pkgs"
run_script || fail "reboot flag: script failed"
expect_reboot "reboot flag" \
	"$work/reboot-required present (linux-image-6.12.110+deb13-arm64 linux-image-arm64)"

# 6. Services needrestart does not restart on its own: reboot.
reset_case
printf 'dbus.service\nsystemd-logind.service\n' >"$work/svc"
run_script || fail "stale services: script failed"
expect_reboot "stale services" \
	'services still on outdated libraries: dbus.service systemd-logind.service'

# 6a. Only our own unit listed: no reboot.
reset_case
printf 'unattended-upgrade-debian.service\n' >"$work/svc"
run_script || fail "self only: script failed"
expect_no_reboot "self only"

# 7. A failed needrestart kernel check never reboots by itself.
reset_case
fake_nr_k_rc=1
run_script || fail "needrestart -k error: script failed"
expect_no_reboot "needrestart -k error"

# 8. Already rebooted today: the once-per-day backstop holds.
reset_case
touch "$work/reboot-required"
date +%F >"$state/last-reboot"
run_script || fail "rebooted today: script failed"
grep -q 'reboot' "$work/systemctl.log" && fail "rebooted today: unexpected reboot"

# 9. Partner down on a fresh day: no update, not stamped, reboot deferred.
reset_case fresh-day
touch "$work/reboot-required"
fake_ping=down
run_script || fail "partner down: script failed"
expect_no_update "partner down"
expect_not_stamped "partner down"
expect_no_reboot "partner down"
grep -q 'skipped daily update: partner(s) not reachable (not stamped)' \
	"$state/log" || fail "partner down: skip not logged"
grep -q 'reboot check deferred: partner(s) not reachable' "$state/log" \
	|| fail "partner down: deferral not logged"

# 9a. pi3 gates on pi2; an unknown host has no partner at all.
reset_case fresh-day
fake_host=pi3
run_script || fail "pi3: script failed"
grep -qx 192.168.1.127 "$work/ping.log" || fail "pi3: pi2 not pinged"
reset_case fresh-day
fake_host=pi9
touch "$work/reboot-required"
run_script || fail "no partner: script failed"
expect_no_update "no partner"
expect_not_stamped "no partner"
expect_no_reboot "no partner"
grep -q 'skipped daily update: no partner configured for pi9' "$state/log" \
	|| fail "no partner: skip not logged"

# 10. Pi clock never NTP-synchronised this boot: nothing runs, nothing is
# stamped; the WARNING marker is logged once per boot and day.
reset_case fresh-day
touch "$work/reboot-required"
fake_ntp=no
run_script || fail "unsynced clock: script failed"
expect_no_update "unsynced clock"
expect_not_stamped "unsynced clock"
expect_no_reboot "unsynced clock"
grep -q "$CLOCK_MARKER" "$state/log" || fail "unsynced clock: warning not logged"
[[ ! -f "$state/clock-synced-boot" ]] || fail "unsynced clock: latch written"
run_script || fail "unsynced clock (2nd tick): script failed"
[[ "$(grep -c "$CLOCK_MARKER" "$state/log")" -eq 1 ]] \
	|| fail "unsynced clock: warning repeated for the same boot and day"
grep -q 'clock not NTP-synchronised yet' "$work/stdout.log" \
	|| fail "unsynced clock: journal note missing on the 2nd tick"
printf 'boot-b\n' >"$work/boot_id"
run_script || fail "unsynced clock (new boot): script failed"
[[ "$(grep -c "$CLOCK_MARKER" "$state/log")" -eq 2 ]] \
	|| fail "unsynced clock: no warning for a new boot"

# 10a. The first synchronised tick latches the boot id.
reset_case fresh-day
run_script || fail "sync latch: script failed"
[[ "$(cat "$state/clock-synced-boot")" == boot-a ]] \
	|| fail "sync latch: boot id not latched"

# 10b. Synchronised earlier this boot, now an NTP blip: still reboots.
reset_case
printf 'boot-a\n' >"$state/clock-synced-boot"
touch "$work/reboot-required"
fake_ntp=no
run_script || fail "ntp blip: script failed"
expect_reboot "ntp blip" "$work/reboot-required present"

# 10c. A latch from an earlier boot is not trusted.
reset_case
printf 'boot-old\n' >"$state/clock-synced-boot"
touch "$work/reboot-required"
fake_ntp=no
run_script || fail "stale latch: script failed"
expect_no_reboot "stale latch"

# 10d. A host with an RTC (no Pi device-tree model) is not clock-gated.
reset_case fresh-day
rm "$work/model"
fake_ntp=no
run_script || fail "rtc host: script failed"
[[ -s "$work/uu.log" ]] || fail "rtc host: unattended-upgrade did not run"

# 11. apt-get update fails: no upgrade, not stamped, exit 1, but the
# reboot check still runs.
reset_case fresh-day
fake_apt=fail
touch "$work/reboot-required"
if run_script; then
	fail "apt update failure: script succeeded"
fi
[[ ! -s "$work/uu.log" ]] || fail "apt update failure: unattended-upgrade ran"
expect_not_stamped "apt update failure"
grep -q 'apt-get update FAILED (rc=100)' "$state/log" \
	|| fail "apt update failure: not logged"
expect_reboot "apt update failure" "$work/reboot-required present"

# 11a. Fetch warnings (a third-party source down): logged, run goes on.
reset_case fresh-day
fake_apt=warn
run_script || fail "apt update warnings: script failed"
grep -q 'WARNING: apt-get update reported fetch problems' "$state/log" \
	|| fail "apt update warnings: warning missing"
[[ -s "$work/uu.log" ]] || fail "apt update warnings: no upgrade"
[[ -f "$state/last-daily" ]] || fail "apt update warnings: not stamped"

# 12. unattended-upgrade fails: not stamped, no restart pass, exit 1.
reset_case fresh-day
fake_uu=fail
if run_script; then
	fail "upgrade failure: script succeeded"
fi
expect_not_stamped "upgrade failure"
grep -q 'unattended-upgrade FAILED (rc=1)' "$state/log" \
	|| fail "upgrade failure: not logged"
grep -qx -- '-r a -l' "$work/nr.log" && fail "upgrade failure: restart pass ran"

# 13. A fresh lock held by another run: skip everything.
reset_case fresh-day
mkdir "$work/lock"
run_script || fail "held lock: script failed"
expect_no_update "held lock"
grep -q 'skipped daily, another run holds the lock' "$state/log" \
	|| fail "held lock: skip not logged"
rmdir "$work/lock"

# 14. Unknown mode.
reset_case fresh-day
set +e
run_script weekly
rc=$?
set -e
[[ $rc -eq 64 ]] || fail "bad mode: exit $rc, want 64"
expect_no_update "bad mode"

printf '%s\n' 'unattended-upgrade-debian tests passed'
