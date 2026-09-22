#!/bin/ksh
# Exercise rocky-kernel-audit.sh (task 752) against synthetic OSV feeds
# without a Pi: uname/rpm/dnf/curl are faked, unzip/jq/zip/date are real.
# Runs the script with ksh when installed, else with bash (the script uses
# only syntax both accept). Covers OK, VULNERABLE, the last_affected
# boundary, and the negative paths that must never report clean: stale feed,
# unreachable source (with and without a cached copy), corrupt download,
# truncated feed, unassessable records and an unknown kernel.

set -eu

script_dir=$(cd "$(dirname "$0")/.." && pwd)
script="$script_dir/rocky-kernel-audit.sh"
shell=$(command -v ksh || command -v bash)
work=$(mktemp -d "${TMPDIR:-/tmp}/rocky-kernel-audit-test.XXXXXX")
trap 'rm -rf "$work"' EXIT

for tool in unzip zip jq; do
	command -v "$tool" >/dev/null || {
		printf 'missing test dependency: %s\n' "$tool" >&2
		exit 1
	}
done

fake="$work/bin"
mkdir "$fake"

cat >"$fake/uname" <<'EOF'
#!/bin/sh
printf '%s\n' "${FAKE_KREL:-6.1.31-v8.1.el9.altarch}"
EOF

# rpm -q <pkg>-<krel> succeeds only when FAKE_RPM=yes; the --qf listing
# prints the installed versions.
cat >"$fake/rpm" <<'EOF'
#!/bin/sh
[ "${FAKE_RPM:-yes}" = yes ] || exit 1
case $* in
*--qf*) printf '6.1.23-v8.1.el9.altarch\n6.1.31-v8.1.el9.altarch\n' ;;
esac
exit 0
EOF

cat >"$fake/dnf" <<'EOF'
#!/bin/sh
printf '6.1.31-v8.1.el9.altarch\n'
EOF

# FAKE_CURL: a zip path to "download", "down" (connection failure) or
# "unchanged" (HTTP 304: nothing written, exit 0).
cat >"$fake/curl" <<'EOF'
#!/bin/sh
out=
while [ $# -gt 0 ]; do
	case $1 in
	-o) out=$2; shift ;;
	esac
	shift
done
case ${FAKE_CURL:?} in
down) exit 7 ;;
unchanged) exit 0 ;;
*) cp "$FAKE_CURL" "$out" ;;
esac
EOF
chmod +x "$fake/uname" "$fake/rpm" "$fake/dnf" "$fake/curl"

fresh=$(date -u -d '-1 hour' '+%Y-%m-%dT%H:%M:%S.123456Z')
stale=$(date -u -d '-100 hours' '+%Y-%m-%dT%H:%M:%SZ')

# record <dir> <id> <modified> <ranges-json> [extra-json-fields]
record() {
	printf '{"id":"%s","modified":"%s",%s"affected":[%s]}' \
		"$2" "$3" "${5:-}" "$4" >"$1/$2.json"
}
kernel() {
	printf '{"package":{"ecosystem":"Linux","name":"Kernel"},"ranges":[%s]}' "$1"
}
eco() {
	printf '{"type":"ECOSYSTEM","events":[%s]}' "$1"
}

# feed <name> <modified> <record-kind>... builds $work/<name>.zip.
feed() {
	typeset name=$1 mod=$2 kind dir
	shift 2
	dir="$work/src-$name"
	mkdir -p "$dir"
	for kind in "$@"; do
		case $kind in
		affected) record "$dir" CVE-2024-0001 "$mod" \
			"$(kernel "$(eco '{"introduced":"0"},{"fixed":"6.1.78"}')")" ;;
		newer) record "$dir" CVE-2024-0002 "$mod" \
			"$(kernel "$(eco '{"introduced":"6.2.0"},{"fixed":"6.6.17"}')")" ;;
		older) record "$dir" CVE-2024-0003 "$mod" \
			"$(kernel "$(eco '{"introduced":"0"},{"fixed":"6.1.20"}')")" ;;
		last) record "$dir" CVE-2024-0004 "$mod" \
			"$(kernel "$(eco '{"introduced":"6.1.0"},{"last_affected":"6.1.31"}')")" ;;
		gitonly) record "$dir" CVE-2024-0005 "$mod" \
			'{"ranges":[{"type":"GIT","events":[{"introduced":"abc"}]}]}' ;;
		withdrawn) record "$dir" CVE-2024-0006 "$mod" \
			"$(kernel "$(eco '{"introduced":"0"}')")" \
			'"withdrawn":"2025-01-01T00:00:00Z",' ;;
		gsd) record "$dir" GSD-2021-0001 "$mod" \
			"$(kernel "$(eco '{"introduced":"0"}')")" ;;
		esac
	done
	(cd "$dir" && zip -qj "$work/$name.zip" ./*.json)
}

feed vuln "$fresh" affected newer older withdrawn gsd
feed clean "$fresh" newer older gsd
feed last "$fresh" last newer
feed unassessable "$fresh" newer gitonly
feed stale "$stale" affected newer
printf 'not a zip\n' >"$work/corrupt.zip"

state="$work/state"
log="$work/audit.log"

# run_audit <curl-mode> [VAR=value...]; sets rc, keeps the state dir.
run_audit() {
	typeset mode=$1
	shift
	rc=0
	env ROCKY_KERNEL_AUDIT_TEST_PATH="$fake" \
		ROCKY_KERNEL_AUDIT_TEST_LOG="$log" \
		ROCKY_KERNEL_AUDIT_TEST_STATE_DIR="$state" \
		ROCKY_KERNEL_AUDIT_TEST_MIN_RECORDS=1 \
		FAKE_CURL="$mode" "$@" \
		"$shell" "$script" >"$work/out" 2>&1 || rc=$?
}

fail() {
	printf 'FAIL: %s\n' "$*" >&2
	cat "$work/out" >&2
	[ -f "$state/status" ] && cat "$state/status" >&2
	exit 1
}

expect() {
	typeset want_rc=$1 want_status=$2 label=$3
	[ "$rc" -eq "$want_rc" ] || fail "$label: rc=$rc, want $want_rc"
	grep -qx "status=$want_status" "$state/status" \
		|| fail "$label: status is not $want_status"
}

fresh_state() { rm -rf "$state"; }

fresh_state
run_audit "$work/vuln.zip"
expect 2 VULNERABLE vulnerable
grep -qx 'affected=1' "$state/status" || fail 'affected count (range or withdrawn handling)'
grep -qx 'cve_records=3' "$state/status" || fail 'GSD or withdrawn record counted'
grep -qx 'CVE-2024-0001' "$state/affected-cves" || fail 'affected list'
grep -q '^<3>VULNERABLE' "$work/out" || fail 'no err-priority journal line'
grep -q 'kernel-audit: VULNERABLE' "$log" || fail 'not in shared log'

fresh_state
run_audit "$work/clean.zip"
expect 0 OK clean
[ ! -s "$state/affected-cves" ] || fail 'clean run lists CVEs'

fresh_state
run_audit "$work/last.zip"
expect 2 VULNERABLE 'last_affected boundary'

fresh_state
run_audit "$work/unassessable.zip"
expect 3 UNKNOWN unassessable

fresh_state
run_audit "$work/stale.zip"
expect 3 UNKNOWN 'stale feed'
grep -q '^reason=feed stale' "$state/status" || fail 'stale reason'

fresh_state
run_audit down
expect 3 UNKNOWN 'unreachable, no cache'
grep -q 'never fetched' "$state/status" || fail 'unreachable reason'

# A cached copy stays usable while it is fresh ...
fresh_state
run_audit "$work/clean.zip"
run_audit down
expect 0 OK 'unreachable, fresh cache'
grep -qx 'fetch=failed' "$state/status" || fail 'fetch failure not recorded'
grep -q 'WARNING: feed download failed' "$log" || fail 'fetch warning'
run_audit unchanged
expect 0 OK 'not modified (304)'
grep -qx 'fetch=unchanged' "$state/status" || fail 'unchanged not recorded'

# ... and a corrupt download never replaces it.
run_audit "$work/corrupt.zip"
expect 0 OK 'corrupt download keeps cache'
grep -qx 'fetch=corrupt' "$state/status" || fail 'corrupt fetch not recorded'

# ... but a stale cached copy does not.
fresh_state
run_audit "$work/stale.zip"
run_audit down
expect 3 UNKNOWN 'unreachable, stale cache'

fresh_state
run_audit "$work/clean.zip" ROCKY_KERNEL_AUDIT_TEST_MIN_RECORDS=5000
expect 3 UNKNOWN 'truncated feed'
grep -q 'truncated' "$state/status" || fail 'truncated reason'

fresh_state
run_audit "$work/clean.zip" FAKE_RPM=no FAKE_KREL=5.14.0-687.el9.aarch64
expect 3 UNKNOWN 'unknown kernel'
grep -q 'is not a raspberrypi2-kernel4 package' "$state/status" \
	|| fail 'unknown kernel reason'

fresh_state
mkdir -p "$state/lock"
run_audit "$work/vuln.zip"
[ "$rc" -eq 0 ] || fail "locked run rc=$rc"
grep -q 'skipped, another run holds' "$work/out" || fail 'lock message'

# An uncreatable state directory must not pass as "another run holds it".
printf 'file\n' >"$work/not-a-dir"
rc=0
env ROCKY_KERNEL_AUDIT_TEST_PATH="$fake" ROCKY_KERNEL_AUDIT_TEST_LOG="$log" \
	ROCKY_KERNEL_AUDIT_TEST_STATE_DIR="$work/not-a-dir/state" \
	FAKE_CURL="$work/clean.zip" "$shell" "$script" >"$work/out" 2>&1 || rc=$?
[ "$rc" -eq 3 ] || fail "uncreatable state dir rc=$rc, want 3"

printf '%s\n' 'rocky kernel audit tests passed'
