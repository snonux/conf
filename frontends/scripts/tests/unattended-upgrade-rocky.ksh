#!/bin/ksh
# Exercise the reboot decision of unattended-upgrade-rocky.sh (task p82)
# without a Rocky host: needs-restarting, rpm, uname, timedatectl,
# systemctl, hostname, ping, dnf and curl are faked; /proc/stat is a
# fixture. Runs the script with ksh when installed, else with bash (the
# script uses only syntax both accept; set TEST_SHELL to force one).
#
# Regression covered: needs-restarting -r takes the boot time from
# systemd's UnitsLoadStartTimestamp, which on the RTC-less pi2/pi3 is the
# systemd build epoch, so it listed glibc/systemd/... after every boot and
# the Pis rebooted daily. The script must reboot only when a listed package
# was installed after the kernel boot time (btime), or when a newer Pi
# kernel is installed than the one running. Negative paths: stale listings,
# unsynchronised Pi clock (which also skips dnf), unreadable btime,
# needs-restarting errors, a kernel mismatch that survived its reboot, the
# once-per-day stamp, the partner gate and the r-node weekday stagger must
# all prevent the reboot.

set -eu

script_dir=$(cd "$(dirname "$0")/.." && pwd)
script="$script_dir/unattended-upgrade-rocky.sh"
shell=${TEST_SHELL:-$(command -v ksh || command -v bash)}
work=$(mktemp -d "${TMPDIR:-/tmp}/unattended-rocky-test.XXXXXX")
trap 'rm -rf "$work"' EXIT

fake="$work/bin"
state="$work/state"
mkdir "$fake"

# Boot at epoch 2000000000; install times below/above it are stale/fresh.
readonly BOOT=2000000000
readonly BEFORE=1999990000
readonly AFTER=2000000500

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

# date +%u answers $FAKE_WEEKDAY (r-node stagger); everything else is real.
cat >"$fake/date" <<EOF
#!/bin/sh
if [ "\$1" = +%u ] && [ -n "\${FAKE_WEEKDAY:-}" ]; then
	printf '%s\\n' "\$FAKE_WEEKDAY"
	exit 0
fi
exec $(command -v date) "\$@"
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

cat >"$fake/uname" <<'EOF'
#!/bin/sh
case $1 in
-r) printf '%s\n' "${FAKE_KREL:-6.1.31-v8.1.el9.altarch}" ;;
*) printf 'aarch64\n' ;;
esac
EOF

cat >"$fake/systemctl" <<'EOF'
#!/bin/sh
printf '%s\n' "$*" >>"${FAKE_SYSTEMCTL_LOG:?}"
EOF

cat >"$fake/curl" <<'EOF'
#!/bin/sh
printf 'dtail-1.0.rpm\n'
EOF

cat >"$fake/dnf" <<'EOF'
#!/bin/sh
printf '%s\n' "$*" >>"${FAKE_DNF_LOG:?}"
printf 'Dependencies resolved.\nNothing to do.\nComplete!\n'
EOF

# -s: no units to restart. -r: prints $FAKE_NR_OUT, exits $FAKE_NR_RC.
cat >"$fake/needs-restarting" <<'EOF'
#!/bin/sh
case $1 in
-s) exit 0 ;;
-r) [ -f "${FAKE_NR_OUT:?}" ] && cat "$FAKE_NR_OUT"; exit "${FAKE_NR_RC:?}" ;;
esac
exit 64
EOF

# rpm -q [--quiet | --qf FMT] NAME against $FAKE_RPM_DB lines
# "<name> <installtime> <version-release>" (one per installed version).
# Like real rpm, an unknown package prints a message on stdout, exits 1.
# Every query is also appended to $FAKE_RPM_LOG.
cat >"$fake/rpm" <<'EOF'
#!/bin/sh
printf '%s\n' "$*" >>"${FAKE_RPM_LOG:?}"
shift
mode=$1
fmt=
case $mode in
--quiet) shift ;;
--qf) fmt=$2; shift 2 ;;
esac
rows=$(awk -v n="$1" '$1 == n' "${FAKE_RPM_DB:?}")
if [ -z "$rows" ]; then
	[ "$mode" = --quiet ] || printf 'package %s is not installed\n' "$1"
	exit 1
fi
[ "$mode" = --quiet ] && exit 0
case $fmt in
*VERSION*) printf '%s\n' "$rows" | awk '{ print $2 " " $3 }' ;;
*) printf '%s\n' "$rows" | awk '{ print $2 }' ;;
esac
EOF

chmod +x "$fake"/*

# A needs-restarting -r listing as printed by dnf-plugins-core 4.3.0.
listing() {
	printf 'Core libraries or services have been updated since boot-up:\n'
	for pkg in "$@"; do
		printf '  * %s\n' "$pkg"
	done
	printf '\nReboot is required to fully utilize these updates.\n'
	printf 'More information: https://access.redhat.com/solutions/27943\n'
}

# Default package database: the four core packages installed before boot
# and the running Pi kernel as the newest one (an older one kept).
default_rpm_db() {
	cat <<EOF
glibc $BEFORE 2.34-275.el9_8
systemd $BEFORE 252-67.el9_8.6.rocky.0.1
dbus-broker $BEFORE 28-9.el9_8
linux-firmware $BEFORE 20260804-161.2.el9_8
raspberrypi2-kernel4 1686026979 6.1.23-v8.1.el9.altarch
raspberrypi2-kernel4 1774786351 6.1.31-v8.1.el9.altarch
EOF
}

# Reset the state for one case: fresh stamp dir (daily stamp for today
# unless $1 is "fresh-day", so only the reboot check runs), /proc/stat
# fixture, default package DB, stale four-package listing with rc 1, and
# the fake knobs (plain variables passed explicitly by run_script: ksh93
# does not export prefix assignments on a POSIX function call).
reset_case() {
	rm -rf "$state"
	mkdir "$state"
	[ "${1:-}" = fresh-day ] || date +%F >"$state/last-daily"
	fake_ntp=yes
	fake_ping=up
	fake_host=pi2
	fake_krel=6.1.31-v8.1.el9.altarch
	fake_weekday=
	printf 'cpu 1 2 3\nbtime %s\nprocesses 1\n' "$BOOT" >"$work/stat"
	default_rpm_db >"$work/rpmdb"
	listing glibc systemd dbus-broker linux-firmware >"$work/nr.out"
	nr_rc=1
	: >"$work/systemctl.log"
	: >"$work/dnf.log"
	: >"$work/rpm.log"
	: >"$work/ping.log"
	: >"$work/stdout"
}

run_script() {
	UNATTENDED_UPGRADE_TEST_PATH="$fake" \
		UNATTENDED_UPGRADE_TEST_LOG="$state/log" \
		UNATTENDED_UPGRADE_TEST_LOCK="$work/lock" \
		UNATTENDED_UPGRADE_TEST_STAMP_DIR="$state" \
		UNATTENDED_UPGRADE_TEST_PROC_STAT="$work/stat" \
		FAKE_SYSTEMCTL_LOG="$work/systemctl.log" \
		FAKE_RPM_DB="$work/rpmdb" \
		FAKE_NR_OUT="$work/nr.out" FAKE_NR_RC="$nr_rc" \
		FAKE_NTP="$fake_ntp" FAKE_PING="$fake_ping" \
		FAKE_HOST="$fake_host" FAKE_KREL="$fake_krel" \
		FAKE_WEEKDAY="$fake_weekday" FAKE_DNF_LOG="$work/dnf.log" \
		FAKE_RPM_LOG="$work/rpm.log" FAKE_PING_LOG="$work/ping.log" \
		"$shell" "$script" daily >"$work/stdout" 2>&1
}

fail() {
	printf 'FAIL: %s\n' "$1" >&2
	for f in "$state/log" "$work/stdout" "$work/systemctl.log"; do
		[ -f "$f" ] && { printf -- '--- %s\n' "$f" >&2; cat "$f" >&2; }
	done
	exit 1
}

expect_reboot() {
	grep -qx 'reboot' "$work/systemctl.log" || fail "$1: no reboot"
	[ "$(cat "$state/last-reboot")" = "$(date +%F)" ] \
		|| fail "$1: reboot stamp missing"
	grep -q "rebooting: $2" "$state/log" || fail "$1: reason '$2' not logged"
}

expect_no_reboot() {
	if grep -q 'reboot' "$work/systemctl.log"; then
		fail "$1: unexpected reboot"
	fi
	[ ! -f "$state/last-reboot" ] || fail "$1: reboot stamp written"
}

# 1. The p82 regression: everything listed was installed before boot.
reset_case
run_script || fail "stale listing: script failed"
expect_no_reboot "stale listing"
grep -q 'no reboot: needs-restarting -r listed glibc systemd dbus-broker linux-firmware, all installed before boot' \
	"$work/stdout" || fail "stale listing: note missing"
[ ! -s "$state/log" ] || fail "stale listing: hourly note leaked into the log"

# 2. glibc updated after boot: genuine reboot.
reset_case
sed -i "s/^glibc $BEFORE/glibc $AFTER/" "$work/rpmdb"
run_script || fail "fresh glibc: script failed"
expect_reboot "fresh glibc" 'core packages updated since boot: glibc$'

# 3. Several installed versions: the newest install time counts.
reset_case
printf 'systemd %s 252-68\n' "$AFTER" >>"$work/rpmdb"
run_script || fail "multi-version: script failed"
expect_reboot "multi-version" 'core packages updated since boot: systemd$'

# 4. A listed package rpm cannot resolve is treated as fresh (conservative).
reset_case
listing glibc dbus-daemon >"$work/nr.out"
run_script || fail "unknown install time: script failed"
expect_reboot "unknown install time" 'core packages updated since boot: dbus-daemon$'

# 5. rc 1 without a parsable package list: trust needs-restarting.
reset_case
printf 'Reboot is required.\n' >"$work/nr.out"
run_script || fail "no package list: script failed"
expect_reboot "no package list" 'needs-restarting -r reports a reboot (no package list)'

# 6. Pi clock not NTP-synchronised: the whole run is skipped, even with a
# genuine update pending and even the daily dnf (install times would be
# recorded with the pre-sync clock) — nothing stamped, retried next tick.
reset_case fresh-day
sed -i "s/^glibc $BEFORE/glibc $AFTER/" "$work/rpmdb"
fake_ntp=no
run_script || fail "unsynced clock: script failed"
expect_no_reboot "unsynced clock"
grep -q 'skipped daily: clock not NTP-synchronised yet (not stamped)' \
	"$state/log" || fail "unsynced clock: skip not logged"
[ ! -s "$work/dnf.log" ] || fail "unsynced clock: dnf ran"
[ ! -f "$state/last-daily" ] || fail "unsynced clock: daily stamp written"

# 7. btime missing from /proc/stat: defer.
reset_case
printf 'cpu 1 2 3\n' >"$work/stat"
run_script || fail "no btime: script failed"
expect_no_reboot "no btime"
grep -q 'reboot check deferred: cannot read btime' "$state/log" \
	|| fail "no btime: deferral not logged"

# 8. needs-restarting reports nothing pending.
reset_case
nr_rc=0
: >"$work/nr.out"
run_script || fail "nothing pending: script failed"
expect_no_reboot "nothing pending"

# 9. needs-restarting itself fails: never reboot on an error.
reset_case
nr_rc=2
sed -i "s/^glibc $BEFORE/glibc $AFTER/" "$work/rpmdb"
run_script || fail "needs-restarting error: script failed"
expect_no_reboot "needs-restarting error"
grep -q 'needs-restarting -r failed (rc=2)' "$work/stdout" \
	|| fail "needs-restarting error: note missing"

# 10. A newer Pi kernel installed last: reboot even with nothing listed.
reset_case
nr_rc=0
: >"$work/nr.out"
printf 'raspberrypi2-kernel4 %s 6.1.40-v8.1.el9.altarch\n' "$AFTER" \
	>>"$work/rpmdb"
run_script || fail "new pi kernel: script failed"
expect_reboot "new pi kernel" \
	'raspberrypi2-kernel4 6.1.40-v8.1.el9.altarch installed, running 6.1.31-v8.1.el9.altarch'
[ "$(cat "$state/last-kernel-reboot")" = 6.1.40-v8.1.el9.altarch ] \
	|| fail "new pi kernel: target kernel not stamped"

# 10a. Same install time (one transaction): the version decides, not the
# lexical order (6.1.9 sorts after 6.1.10 as text). Running the newer one
# means nothing is pending.
reset_case
nr_rc=0
: >"$work/nr.out"
grep -v '^raspberrypi2-kernel4 ' "$work/rpmdb" >"$work/rpmdb.new"
mv "$work/rpmdb.new" "$work/rpmdb"
printf 'raspberrypi2-kernel4 %s 6.1.%s-v8.1.el9.altarch\n' \
	"$AFTER" 10 "$AFTER" 9 >>"$work/rpmdb"
fake_krel=6.1.10-v8.1.el9.altarch
run_script || fail "kernel tie: script failed"
expect_no_reboot "kernel tie"

# 10b. Already rebooted once for this kernel and it still is not running
# (config.txt pins another image, or it fell back): warn once a day, no
# second reboot — and the core-package check still runs.
reset_case
nr_rc=0
: >"$work/nr.out"
printf 'raspberrypi2-kernel4 %s 6.1.40-v8.1.el9.altarch\n' "$AFTER" \
	>>"$work/rpmdb"
printf '6.1.40-v8.1.el9.altarch\n' >"$state/last-kernel-reboot"
run_script || fail "kernel mismatch persists: script failed"
expect_no_reboot "kernel mismatch persists"
grep -q 'WARNING: already rebooted for raspberrypi2-kernel4 6.1.40-v8.1.el9.altarch but 6.1.31-v8.1.el9.altarch is still running' \
	"$state/log" || fail "kernel mismatch persists: warning missing"
run_script || fail "kernel mismatch persists (2nd tick): script failed"
[ "$(grep -c 'WARNING: already rebooted' "$state/log")" -eq 1 ] \
	|| fail "kernel mismatch persists: warning repeated the same day"
nr_rc=1
listing glibc systemd >"$work/nr.out"
sed -i "s/^glibc $BEFORE/glibc $AFTER/" "$work/rpmdb"
run_script || fail "kernel mismatch + glibc: script failed"
expect_reboot "kernel mismatch + glibc" 'core packages updated since boot: glibc$'

# 10c. A different, newer target than the stamped one reboots again.
reset_case
nr_rc=0
: >"$work/nr.out"
printf 'raspberrypi2-kernel4 %s 6.1.40-v8.1.el9.altarch\n' "$AFTER" \
	>>"$work/rpmdb"
printf '6.1.35-v8.1.el9.altarch\n' >"$state/last-kernel-reboot"
run_script || fail "new kernel target: script failed"
expect_reboot "new kernel target" \
	'raspberrypi2-kernel4 6.1.40-v8.1.el9.altarch installed'

# 11. r-nodes: no Pi kernel package. r0 pings both siblings, reboots only
# on its weekdays (date +%u % 3 == 1), never runs the Pi kernel query, and
# its RTC-backed clock is not NTP-gated (a stopped chronyd must not defer a
# genuine reboot forever).
r_node_case() {
	reset_case "${1:-}"
	grep -v '^raspberrypi2-kernel4 ' "$work/rpmdb" >"$work/rpmdb.new"
	mv "$work/rpmdb.new" "$work/rpmdb"
	fake_host=r0
	fake_krel=5.14.0-687.49.1.el9_8.x86_64
	fake_weekday=4
}
expect_no_pi_kernel_query() {
	if grep -q -- '--qf.*raspberrypi2-kernel4' "$work/rpm.log"; then
		fail "$1: Pi kernel check ran on an r-node"
	fi
}

r_node_case
run_script || fail "r0 stale: script failed"
expect_no_reboot "r0 stale"
expect_no_pi_kernel_query "r0 stale"
for ip in 192.168.1.121 192.168.1.122; do
	grep -qx "$ip" "$work/ping.log" || fail "r0 stale: partner $ip not pinged"
done

r_node_case
sed -i "s/^glibc $BEFORE/glibc $AFTER/" "$work/rpmdb"
fake_ntp=no
run_script || fail "r0 fresh glibc, unsynced: script failed"
expect_reboot "r0 fresh glibc, unsynced" \
	'core packages updated since boot: glibc$'
expect_no_pi_kernel_query "r0 fresh glibc"

r_node_case
sed -i "s/^glibc $BEFORE/glibc $AFTER/" "$work/rpmdb"
fake_weekday=2
run_script || fail "r0 off-day: script failed"
expect_no_reboot "r0 off-day"

r_node_case
sed -i "s/^glibc $BEFORE/glibc $AFTER/" "$work/rpmdb"
fake_ping=192.168.1.122
run_script || fail "r0 one sibling down: script failed"
expect_no_reboot "r0 one sibling down"
grep -q 'reboot check deferred: partner(s) not reachable' "$state/log" \
	|| fail "r0 one sibling down: deferral not logged"

r_node_case fresh-day
fake_ntp=no
run_script || fail "r0 daily, unsynced: script failed"
[ -s "$work/dnf.log" ] || fail "r0 daily, unsynced: dnf did not run"
[ "$(cat "$state/last-daily")" = "$(date +%F)" ] \
	|| fail "r0 daily, unsynced: daily stamp missing"

# 12. Already rebooted today: the once-per-day backstop holds.
reset_case
sed -i "s/^glibc $BEFORE/glibc $AFTER/" "$work/rpmdb"
date +%F >"$state/last-reboot"
run_script || fail "rebooted today: script failed"
grep -q 'reboot' "$work/systemctl.log" && fail "rebooted today: unexpected reboot"

# 13. Partner down: the partner gate holds.
reset_case
sed -i "s/^glibc $BEFORE/glibc $AFTER/" "$work/rpmdb"
fake_ping=down
run_script || fail "partner down: script failed"
expect_no_reboot "partner down"
grep -q 'reboot check deferred: partner(s) not reachable' "$state/log" \
	|| fail "partner down: deferral not logged"

# 14. Full daily path (no stamp yet) with the stale listing: the update
# runs and stamps the day, and still no reboot.
reset_case fresh-day
run_script || fail "daily path: script failed"
expect_no_reboot "daily path"
grep -q '^-y upgrade' "$work/dnf.log" || fail "daily path: dnf did not run"
[ "$(cat "$state/last-daily")" = "$(date +%F)" ] \
	|| fail "daily path: daily stamp missing"

printf '%s\n' 'unattended-upgrade-rocky reboot tests passed'
