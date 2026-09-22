#!/bin/ksh
# Exercise rocky-kernel-audit.sh (task 752) against synthetic OSV feeds
# without a Pi: uname/rpm/dnf/curl are faked, unzip/jq/zip/date are real.
# Runs the script with ksh when installed, else with bash (the script uses
# only syntax both accept; set TEST_SHELL to force one). TEST_EXTRA_PATH
# adds directories to the script's PATH (e.g. unzip/zip shims on a host
# that lacks them; the script otherwise resets PATH).
#
# Covers the range evaluation (fixed and last_affected boundaries, several
# ranges, -rc versions, explicit version lists, unparsable events, the
# branch-aware multi-fix ranges), the baseline alerting (new CVEs, status
# changes, UNKNOWN runs leaving the baseline alone) and the negative paths
# that must never report clean: stale feed, unreachable source (with and
# without a cached copy), corrupt download, truncated feed, unassessable
# records, unknown kernel, uncreatable state dir, failed status write.

set -eu

script_dir=$(cd "$(dirname "$0")/.." && pwd)
script="$script_dir/rocky-kernel-audit.sh"
shell=${TEST_SHELL:-$(command -v ksh || command -v bash)}
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
script_path="$fake${TEST_EXTRA_PATH:+:$TEST_EXTRA_PATH}"

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
# "unchanged" (HTTP 304: nothing written, exit 0). Every argument list is
# appended to FAKE_CURL_LOG so the tests can assert the conditional GET.
cat >"$fake/curl" <<'EOF'
#!/bin/sh
printf '%s\n' "$*" >>"${FAKE_CURL_LOG:?}"
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

# record <dir> <id> <modified> <affected-json> [extra-json-fields]
record() {
	printf '{"id":"%s","modified":"%s",%s"affected":[%s]}' \
		"$2" "$3" "${5:-}" "$4" >"$1/$2.json"
}
# kernel <ranges-json> [extra-affected-fields]
kernel() {
	printf '{"package":{"ecosystem":"Linux","name":"Kernel"},%s"ranges":[%s]}' \
		"${2:-}" "$1"
}
eco() {
	printf '{"type":"ECOSYSTEM","events":[%s]}' "$1"
}

# kind <dir> <modified> <kind>: one synthetic record per kind.
kind() {
	typeset d=$1 m=$2
	case $3 in
	affected) record "$d" CVE-2024-0001 "$m" \
		"$(kernel "$(eco '{"introduced":"0"},{"fixed":"6.1.78"}')")" ;;
	newer) record "$d" CVE-2024-0002 "$m" \
		"$(kernel "$(eco '{"introduced":"6.2.0"},{"fixed":"6.6.17"}')")" ;;
	older) record "$d" CVE-2024-0003 "$m" \
		"$(kernel "$(eco '{"introduced":"0"},{"fixed":"6.1.20"}')")" ;;
	last) record "$d" CVE-2024-0004 "$m" \
		"$(kernel "$(eco '{"introduced":"6.1.0"},{"last_affected":"6.1.31"}')")" ;;
	gitonly) record "$d" CVE-2024-0005 "$m" \
		'{"ranges":[{"type":"GIT","events":[{"introduced":"abc"}]}]}' ;;
	withdrawn) record "$d" CVE-2024-0006 "$m" \
		"$(kernel "$(eco '{"introduced":"0"}')")" \
		'"withdrawn":"2025-01-01T00:00:00Z",' ;;
	affected2) record "$d" CVE-2024-0007 "$m" \
		"$(kernel "$(eco '{"introduced":"6.1.0"},{"fixed":"6.1.90"}')")" ;;
	fixedat) record "$d" CVE-2024-0008 "$m" \
		"$(kernel "$(eco '{"introduced":"6.1.0"},{"fixed":"6.1.31"}')")" ;;
	laterange) record "$d" CVE-2024-0009 "$m" "$(kernel "$(eco \
		'{"introduced":"0"},{"fixed":"5.10.209"}'),$(eco \
		'{"introduced":"5.11.0"},{"fixed":"5.15.148"}'),$(eco \
		'{"introduced":"5.16.0"},{"fixed":"6.1.78"}')")" ;;
	rcfix) record "$d" CVE-2024-0010 "$m" \
		"$(kernel "$(eco '{"introduced":"0"},{"fixed":"6.2-rc1"}')")" ;;
	rcintro) record "$d" CVE-2024-0011 "$m" \
		"$(kernel "$(eco '{"introduced":"6.2-rc1"}')")" ;;
	badevent) record "$d" CVE-2024-0012 "$m" \
		"$(kernel "$(eco '{"introduced":"abc"}')")" ;;
	listhit) record "$d" CVE-2024-0013 "$m" \
		'{"package":{"ecosystem":"Linux","name":"Kernel"},"versions":["6.1.31"]}' ;;
	listmiss) record "$d" CVE-2024-0014 "$m" \
		'{"package":{"ecosystem":"Linux","name":"Kernel"},"versions":["6.1.20"]}' ;;
	multifix) record "$d" CVE-2024-0015 "$m" "$(kernel "$(eco \
		'{"introduced":"5.16.0"},{"fixed":"6.1.75"},{"fixed":"6.6.14"}')")" ;;
	gsd) record "$d" GSD-2021-0001 "$m" \
		"$(kernel "$(eco '{"introduced":"0"}')")" ;;
	*) printf 'unknown record kind %s\n' "$3" >&2; exit 1 ;;
	esac
}

# feed <name> <modified> <record-kind>... builds $work/<name>.zip.
feed() {
	typeset name=$1 mod=$2 k dir
	shift 2
	dir="$work/src-$name"
	mkdir -p "$dir"
	for k in "$@"; do
		kind "$dir" "$mod" "$k"
	done
	(cd "$dir" && zip -qj "$work/$name.zip" ./*.json)
}

feed vuln "$fresh" affected newer older withdrawn gsd
feed vulnmore "$fresh" affected affected2 newer older
feed clean "$fresh" newer older gsd
feed last "$fresh" last newer
feed fixedat "$fresh" fixedat newer
feed laterange "$fresh" laterange newer
feed rc "$fresh" rcfix rcintro
feed badevent "$fresh" badevent newer
feed lists "$fresh" listhit listmiss
feed multifix "$fresh" multifix
feed unassessable "$fresh" newer gitonly
feed stale "$stale" affected newer
printf 'not a zip\n' >"$work/corrupt.zip"

state="$work/state"
log="$work/audit.log"
curl_log="$work/curl.log"

# run_audit <curl-mode> [VAR=value...]; sets rc, keeps the state dir.
run_audit() {
	typeset mode=$1
	shift
	rc=0
	: >"$curl_log"
	env ROCKY_KERNEL_AUDIT_TEST_PATH="$script_path" \
		ROCKY_KERNEL_AUDIT_TEST_LOG="$log" \
		ROCKY_KERNEL_AUDIT_TEST_STATE_DIR="$state" \
		ROCKY_KERNEL_AUDIT_TEST_MIN_RECORDS=1 \
		FAKE_CURL_LOG="$curl_log" FAKE_CURL="$mode" "$@" \
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

# has <file> <regex> <label> / hasnt: grep assertions with a failure label.
has() { grep -q -- "$2" "$1" || fail "$3"; }
hasnt() { ! grep -q -- "$2" "$1" || fail "$3"; }
field() { has "$state/status" "^$1\$" "$2: want $1"; }

fresh_state() { rm -rf "$state"; }

# --- range evaluation --------------------------------------------------

fresh_state
run_audit "$work/vuln.zip"
expect 0 VULNERABLE vulnerable
field 'affected=1' 'affected count (range or withdrawn handling)'
field 'cve_records=3' 'GSD or withdrawn record counted'
has "$state/affected-cves" '^CVE-2024-0001$' 'affected list'
has "$work/out" '^<5>VULNERABLE' 'VULNERABLE not reported at notice'
has "$log" 'kernel-audit: VULNERABLE' 'not in shared log'

# audit_one <feed> <want-rc> <want-status> <label> [VAR=value...]
audit_one() {
	typeset f=$1 want_rc=$2 want_status=$3 label=$4
	shift 4
	fresh_state
	run_audit "$work/$f.zip" "$@"
	expect "$want_rc" "$want_status" "$label"
}

audit_one clean 0 OK clean
[ ! -s "$state/affected-cves" ] || fail 'clean run lists CVEs'
has "$work/out" '^<6>OK' 'OK not reported at info'
audit_one last 0 VULNERABLE 'last_affected boundary'
audit_one fixedat 0 OK 'running == fixed is not affected'
audit_one laterange 0 VULNERABLE 'only a later range matches'
audit_one rc 0 VULNERABLE '-rc versions'
field 'affected=1' '6.2-rc1 fix affects 6.1.31, 6.2-rc1 intro does not'
has "$state/affected-cves" '^CVE-2024-0010$' 'rc fix record'
audit_one badevent 3 UNKNOWN 'unparsable event is unassessable'
field 'unassessable=1' 'unparsable event count'
audit_one lists 0 VULNERABLE 'explicit versions list'
field 'affected=1' 'versions hit/miss'
field 'unassessable=0' 'versions miss is assessed, not unassessable'

# [introduced 5.16.0, fixed 6.1.75, fixed 6.6.14]: plain OSV evaluation
# would call 6.2-6.6.13 fixed by 6.1.75.
audit_one multifix 0 VULNERABLE 'multi-fix, 6.1.31'
audit_one multifix 0 VULNERABLE 'multi-fix, 6.4.5 (branch never fixed)' \
	FAKE_KREL=6.4.5-v8.1.el9.altarch
audit_one multifix 0 VULNERABLE 'multi-fix, 6.6.13' \
	FAKE_KREL=6.6.13-v8.1.el9.altarch
audit_one multifix 0 OK 'multi-fix, 6.1.80' FAKE_KREL=6.1.80-v8.1.el9.altarch
audit_one multifix 0 OK 'multi-fix, 6.6.14' FAKE_KREL=6.6.14-v8.1.el9.altarch
audit_one multifix 0 OK 'multi-fix, 6.8.1 inherits the last fix' \
	FAKE_KREL=6.8.1-v8.1.el9.altarch

# --- baseline alerting -------------------------------------------------

fresh_state
run_audit "$work/vuln.zip"
expect 0 VULNERABLE 'first run'
field 'new_cves=1' 'first run: everything is new'
has "$work/out" '^<4>NEW: 1 CVE(s) newly affect the kernel: CVE-2024-0001' \
	'first run NEW warning'
has "$work/out" '^<4>status changed: none -> VULNERABLE' 'first status change'
hasnt "$curl_log" ' -z ' 'first fetch must be unconditional'

run_audit "$work/vuln.zip"
expect 0 VULNERABLE 'unchanged rerun'
field 'new_cves=0' 'rerun: nothing new'
hasnt "$work/out" '^<4>' 'unchanged VULNERABLE must not warn'
has "$curl_log" ' -z ' 'cached copy must use a conditional GET'

run_audit "$work/vulnmore.zip"
expect 0 VULNERABLE 'new CVE'
field 'new_cves=1' 'one new CVE'
has "$state/new-cves" '^CVE-2024-0007$' 'new CVE listed'
hasnt "$state/new-cves" 'CVE-2024-0001' 'baseline CVE reported as new'
has "$work/out" '^<4>NEW: 1 CVE(s) newly affect the kernel: CVE-2024-0007' \
	'new CVE warning'

cp "$state/affected-cves" "$work/baseline"
run_audit "$work/stale.zip"
expect 3 UNKNOWN 'stale feed after a baseline'
has "$work/out" '^<3>UNKNOWN: feed stale' 'UNKNOWN not reported at err'
cmp -s "$state/affected-cves" "$work/baseline" \
	|| fail 'UNKNOWN run changed the baseline'
run_audit "$work/vulnmore.zip"
expect 0 VULNERABLE 'coverage restored'
field 'new_cves=0' 'coverage restored: baseline kept'
field 'previous_status=UNKNOWN' 'previous status'
has "$work/out" '^<4>status changed: UNKNOWN -> VULNERABLE' \
	'status change after UNKNOWN'

run_audit "$work/clean.zip"
expect 0 OK 'fixed kernel'
has "$work/out" '^<4>status changed: VULNERABLE -> OK' 'status change to OK'

# --- coverage failures -------------------------------------------------

audit_one unassessable 3 UNKNOWN unassessable
audit_one stale 3 UNKNOWN 'stale feed'
has "$state/status" '^reason=feed stale' 'stale reason'

fresh_state
run_audit down
expect 3 UNKNOWN 'unreachable, no cache'
has "$state/status" 'never fetched' 'unreachable reason'
field 'fetch=failed' 'unreachable fetch field'

# A cached copy stays usable while it is fresh ...
fresh_state
run_audit "$work/clean.zip"
run_audit down
expect 0 OK 'unreachable, fresh cache'
field 'fetch=failed' 'fetch failure not recorded'
has "$work/out" '^<4>WARNING: feed download failed' 'fetch warning'
run_audit unchanged
expect 0 OK 'not modified (304)'
field 'fetch=unchanged' 'unchanged not recorded'

# ... and a corrupt download never replaces it ...
run_audit "$work/corrupt.zip"
expect 0 OK 'corrupt download keeps cache'
field 'fetch=corrupt' 'corrupt fetch not recorded'

# ... but a stale cached copy is not usable.
fresh_state
run_audit "$work/stale.zip"
run_audit down
expect 3 UNKNOWN 'unreachable, stale cache'

audit_one clean 3 UNKNOWN 'truncated feed' ROCKY_KERNEL_AUDIT_TEST_MIN_RECORDS=5000
has "$state/status" 'truncated' 'truncated reason'

audit_one clean 3 UNKNOWN 'unknown kernel' FAKE_RPM=no \
	FAKE_KREL=5.14.0-687.el9.aarch64
has "$state/status" 'is not a raspberrypi2-kernel4 package' \
	'unknown kernel reason'
field 'fetch=skipped' 'no fetch runs for an unknown kernel'

fresh_state
mkdir -p "$state/lock"
run_audit "$work/vuln.zip"
[ "$rc" -eq 0 ] || fail "locked run rc=$rc"
has "$work/out" 'skipped, another run holds' 'lock message'

# A failed status write (full disk; here the temp path is a directory) must
# fail as UNKNOWN and keep the previous record.
fresh_state
run_audit "$work/clean.zip"
cp "$state/status" "$work/status.before"
mkdir "$state/status.tmp"
run_audit "$work/vuln.zip"
[ "$rc" -eq 3 ] || fail "failed status write rc=$rc, want 3"
cmp -s "$state/status" "$work/status.before" \
	|| fail 'failed status write replaced the record'
has "$work/out" '^<3>UNKNOWN: cannot write' 'status write error'
rmdir "$state/status.tmp"
run_audit "$work/vuln.zip"
expect 0 VULNERABLE 'after a failed status write'
field 'new_cves=1' 'unrecorded run must not consume the new CVEs'

# An uncreatable state directory must not pass as "another run holds it".
printf 'file\n' >"$work/not-a-dir"
rc=0
env ROCKY_KERNEL_AUDIT_TEST_PATH="$script_path" ROCKY_KERNEL_AUDIT_TEST_LOG="$log" \
	ROCKY_KERNEL_AUDIT_TEST_STATE_DIR="$work/not-a-dir/state" \
	FAKE_CURL_LOG="$curl_log" FAKE_CURL="$work/clean.zip" \
	"$shell" "$script" >"$work/out" 2>&1 || rc=$?
[ "$rc" -eq 3 ] || fail "uncreatable state dir rc=$rc, want 3"

printf 'rocky kernel audit tests passed (%s)\n' "$shell"
