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
# unsynchronised clock, unreadable btime, needs-restarting errors, the
# once-per-day stamp and the partner gate must all prevent the reboot.

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

cat >"$fake/ping" <<'EOF'
#!/bin/sh
[ "${FAKE_PING:-up}" = up ]
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
cat >"$fake/rpm" <<'EOF'
#!/bin/sh
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
	printf 'cpu 1 2 3\nbtime %s\nprocesses 1\n' "$BOOT" >"$work/stat"
	default_rpm_db >"$work/rpmdb"
	listing glibc systemd dbus-broker linux-firmware >"$work/nr.out"
	nr_rc=1
	: >"$work/systemctl.log"
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

# 6. Clock not NTP-synchronised: btime is untrustworthy, defer.
reset_case
sed -i "s/^glibc $BEFORE/glibc $AFTER/" "$work/rpmdb"
fake_ntp=no
run_script || fail "unsynced clock: script failed"
expect_no_reboot "unsynced clock"
grep -q 'reboot check deferred: clock not NTP-synchronised' "$state/log" \
	|| fail "unsynced clock: deferral not logged"

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

# 11. Host without the Pi kernel (r-node): no kernel check, stale listing.
reset_case
grep -v '^raspberrypi2-kernel4 ' "$work/rpmdb" >"$work/rpmdb.new"
mv "$work/rpmdb.new" "$work/rpmdb"
fake_host=pi3
fake_krel=5.14.0-687.49.1.el9_8.x86_64
run_script || fail "no pi kernel: script failed"
expect_no_reboot "no pi kernel"

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
[ "$(cat "$state/last-daily")" = "$(date +%F)" ] \
	|| fail "daily path: daily stamp missing"

printf '%s\n' 'unattended-upgrade-rocky reboot tests passed'
