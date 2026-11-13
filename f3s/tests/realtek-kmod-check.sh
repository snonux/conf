#!/bin/sh
# Offline test of f3s/freebsd-hosts/realtek/realtek-kmod-check (conf task
# t74), the guard that keeps an f-host from booting the Realtek vendor re(4)
# module (realtek-re-kmod) into a kernel of another minor release. pkg,
# freebsd-version and logger are faked; loader.conf and the module file are
# fixtures. Runs with any POSIX sh: sh f3s/tests/realtek-kmod-check.sh
#
# Positive: no loader line (in-tree driver) passes; a matching 15.1 build
# passes for the next boot and for -r; the version fallback works without
# the FreeBSD_version annotation. Negative: a 15.2 kernel installed (minor
# upgrade pending) or running 15.0 fails, as do a missing package, a
# missing module, a module owned by another package, failed checksums, an
# unparseable build version, if_re_name pointing elsewhere and a bad
# argument (usage, 64). Loader spellings: an unquoted/lowercase/indented
# line and lines in loader.conf.local or loader.conf.d count as loading the
# module; if_re_load="NO" does not.

set -eu

repo=$(cd "$(dirname "$0")/../.." && pwd)
check=$repo/f3s/freebsd-hosts/realtek/realtek-kmod-check
work=$(mktemp -d "${TMPDIR:-/tmp}/realtek-kmod-check-test.XXXXXX")
trap 'rm -rf "$work"' EXIT
fake=$work/bin
mkdir "$fake"

# freebsd-version -k/-r answer $FAKE_K/$FAKE_R.
cat >"$fake/freebsd-version" <<'EOF'
#!/bin/sh
case $1 in
-k) echo "$FAKE_K" ;;
-r) echo "$FAKE_R" ;;
*) exit 1 ;;
esac
EOF
# logger records the message so the syslog path is checked too.
cat >"$fake/logger" <<'EOF'
#!/bin/sh
echo "$*" >>"$FAKE_SYSLOG"
EOF
# pkg: FAKE_INSTALLED=no (not installed), FAKE_ANNOT (FreeBSD_version or
# empty), FAKE_VERSION, FAKE_OWNER (pkg which output), FAKE_CHECKSUM=bad.
cat >"$fake/pkg" <<'EOF'
#!/bin/sh
case $1 in
query)
	[ "$FAKE_INSTALLED" = yes ] || exit 1
	case $2 in
	'%n') echo realtek-re-kmod ;;
	'%v') echo "$FAKE_VERSION" ;;
	'%At %Av')
		echo "build_timestamp 2026-10-26T21:00:18+0000"
		[ -z "$FAKE_ANNOT" ] || echo "FreeBSD_version $FAKE_ANNOT" ;;
	esac ;;
which) [ -n "$FAKE_OWNER" ] && echo "$FAKE_OWNER" ;;
check) [ "$FAKE_CHECKSUM" = ok ] ;;
*) exit 1 ;;
esac
EOF
chmod 755 "$fake"/*

loader=$work/loader.conf
module=$work/if_re.ko
vendor_lines='if_re_load="YES"	# Realtek vendor re(4)
if_re_name="'$module'"
hw.re.s5wol="1"'

failures=0

# reset restores the healthy 15.1 fixture.
reset() {
	FAKE_K=15.1-RELEASE-p2 FAKE_R=15.1-RELEASE-p2 FAKE_INSTALLED=yes
	FAKE_ANNOT=1501000 FAKE_VERSION=1102.01.1501000_2
	FAKE_OWNER=realtek-re-kmod-1102.01.1501000_2 FAKE_CHECKSUM=ok
	FAKE_SYSLOG=$work/syslog
	export FAKE_K FAKE_R FAKE_INSTALLED FAKE_ANNOT FAKE_VERSION FAKE_OWNER FAKE_CHECKSUM FAKE_SYSLOG
	: >"$FAKE_SYSLOG"
	printf 'zfs_load="YES"\n%s\n' "$vendor_lines" >"$loader"
	: >"$module"
}

# expect NAME RC PATTERN [ARGS]: run the check, compare exit code and output.
expect() {
	name=$1 want=$2 pattern=$3
	shift 3
	rc=0
	out=$(REALTEK_CHECK_TEST_PATH=$fake REALTEK_CHECK_TEST_LOADER_CONF=$loader \
		REALTEK_CHECK_TEST_MODULE=$module sh "$check" "$@" 2>&1) || rc=$?
	if [ "$rc" -ne "$want" ] || ! printf '%s\n' "$out" | grep -q -- "$pattern"; then
		echo "FAIL $name: rc=$rc want=$want output: $out"
		failures=$((failures + 1))
		return
	fi
	if [ "$want" -eq 1 ] && ! grep -q -- "$pattern" "$FAKE_SYSLOG"; then
		echo "FAIL $name: failure not logged to syslog"
		failures=$((failures + 1))
		return
	fi
	echo "ok   $name"
}

reset; printf 'zfs_load="YES"\n#if_re_load="YES"\n' >"$loader"
expect "in-tree driver (no or commented line) passes" 0 "in-tree re(4) boots"
reset; FAKE_K=15.2-RELEASE; printf 'zfs_load="YES"\n' >"$loader"
expect "in-tree driver passes even across a minor upgrade" 0 "in-tree"
reset
expect "matching 15.1 build passes for the next boot" 0 "OK, realtek-re-kmod matches"
reset
expect "-r passes on a matching kernel" 0 "ready" -r
reset; FAKE_ANNOT=''
expect "version fallback without annotation passes" 0 "ready" -r
reset; FAKE_K=15.2-RELEASE
expect "minor upgrade pending (15.2 installed) fails" 1 "kernel is 15.2-RELEASE"
reset; FAKE_K=15.2-RELEASE
expect "-r fails when the installed kernel moved on" 1 "kernel is 15.2-RELEASE" -r
reset; FAKE_R=15.0-RELEASE-p4
expect "-r fails when the running kernel differs" 1 "kernel is 15.0-RELEASE-p4" -r
reset; FAKE_K=15.1-RELEASE-p3
expect "patch level change passes" 0 "OK"
reset; FAKE_INSTALLED=no
expect "package not installed fails" 1 "is not installed" -r
reset; rm -f "$module"
expect "missing module fails" 1 "is missing"
reset; FAKE_OWNER=''
expect "module owned by no package fails" 1 "does not belong" -r
reset; FAKE_OWNER=realtek-re-kmod198-198.00.1501000
expect "module of another package fails" 1 "does not belong" -r
reset; FAKE_CHECKSUM=bad
expect "checksum mismatch fails" 1 "checksums" -r
reset; FAKE_ANNOT='' FAKE_VERSION=garbage
expect "unparseable build version fails" 1 "cannot read the FreeBSD_version" -r
reset; FAKE_K=garbage
expect "unparseable kernel release fails" 1 "cannot parse kernel release" -r
reset; printf 'if_re_load="YES"\nif_re_name="/boot/kernel/if_re.ko"\n' >"$loader"
expect "if_re_name elsewhere fails" 1 "if_re_name is not"
reset; printf 'zfs_load="YES"\n  IF_RE_LOAD=yes # unquoted, upper key\nif_re_name=%s\n' "$module" >"$loader"; FAKE_K=15.2-RELEASE
expect "unquoted, lowercase-value, indented if_re_load is seen" 1 "kernel is 15.2-RELEASE"
reset; printf 'zfs_load="YES"\nif_re_load="NO"\n' >"$loader"; FAKE_K=15.2-RELEASE
expect "if_re_load NO counts as in-tree" 0 "in-tree"
reset; printf 'zfs_load="YES"\n' >"$loader"; mkdir -p "$work/loader.conf.d"
printf '%s\n' "$vendor_lines" >"$work/loader.conf.d/realtek.conf"; FAKE_K=15.2-RELEASE
expect "if_re_load in loader.conf.d is seen" 1 "kernel is 15.2-RELEASE"
rm -r "$work/loader.conf.d"
reset; printf 'zfs_load="YES"\n' >"$loader"; printf '%s\n' "$vendor_lines" >"$loader.local"; FAKE_K=15.2-RELEASE
expect "if_re_load in loader.conf.local is seen" 1 "kernel is 15.2-RELEASE"
rm -f "$loader.local"
reset; printf 'zfs_load="YES"\n' >"$loader"
expect "-r passes before the first deploy (no loader lines yet)" 0 "ready" -r
reset
expect "bad argument is a usage error" 64 "usage" -x

if [ "$failures" -ne 0 ]; then
	echo "$failures failure(s)"
	exit 1
fi
echo "all realtek-kmod-check cases passed"
