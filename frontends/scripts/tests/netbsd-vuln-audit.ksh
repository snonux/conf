#!/bin/ksh
# Exercise netbsd-vuln-audit.sh (task 652) without a Pi: uname, pkg_admin,
# pkg_info, curl and logger are faked; gzip, awk, sort, comm and date are
# real. Runs the script with ksh when installed, else with bash (set
# TEST_SHELL to force one; on pi0/pi1 `ksh` is NetBSD's pdksh). The fake
# pkg_admin accepts a pkg-vulnerabilities list when it has the #FORMAT
# line and the closing PGP signature line, which stands in for the real
# SHA512 check (a truncated download loses that line).
#
# Covers the package audit (clean, findings, baseline deltas, the 304 and
# conditional GET, restoring a deleted list), the base audit (newer release,
# end-of-life series, advisory verdicts: release line, branch fix date,
# series line, unassessable), the per-component baseline, and the negative
# paths that must never report clean: never fetched, refresh failures past
# 36 h, corrupt and older downloads, a stale list, pkg_admin errors and
# unparsable output, no packages, incomplete or truncated pages, an index
# that lost an advisory, a missing advisory, a non-release kernel, a held
# lock, and failed status, history and baseline writes.

set -eu

script_dir=$(cd "$(dirname "$0")/.." && pwd)
script="$script_dir/netbsd-vuln-audit.sh"
shell=${TEST_SHELL:-$(command -v ksh || command -v bash)}
work=$(mktemp -d "${TMPDIR:-/tmp}/netbsd-vuln-audit-test.XXXXXX")
trap 'rm -rf "$work"' EXIT

fake="$work/bin"
web="$work/web"
vulndir="$work/pkgdb"
mkdir "$fake" "$web" "$vulndir"

cat >"$fake/uname" <<'EOF'
#!/bin/sh
case $1 in
-r) printf '%s\n' "${FAKE_REL:-11.0}" ;;
-v) printf '%s\n' "${FAKE_UNAMEV:-NetBSD 11.0 (GENERIC64) #0: Thu Jul 30 15:23:12 UTC 2026  mkrepro@mkrepro.NetBSD.org:/usr/src/sys/arch/evbarm/compile/GENERIC64}" ;;
*) printf 'NetBSD\n' ;;
esac
EOF

# FAKE_AUDIT: file printed by `pkg_admin audit` (none: clean);
# FAKE_AUDIT_ERR: text on stderr; FAKE_AUDIT_RC overrides the exit code
# (default 1 with findings, else 0, like pkg_install).
cat >"$fake/pkg_admin" <<'EOF'
#!/bin/sh
valid() {
	grep -q '^#FORMAT ' "$1" 2>/dev/null \
		&& grep -q '^-----END PGP SIGNATURE-----$' "$1"
}
case $1 in
config-var) [ "$2" = PKGVULNDIR ] && printf '%s\n' "${FAKE_PKGVULNDIR:?}" ;;
check-pkg-vulnerabilities) valid "$2" ;;
audit)
	if [ ! -f "$FAKE_PKGVULNDIR/pkg-vulnerabilities" ]; then
		echo "pkg_admin: Cannot open $FAKE_PKGVULNDIR/pkg-vulnerabilities" >&2
		exit 1
	fi
	[ -n "${FAKE_AUDIT_ERR:-}" ] && printf '%s\n' "$FAKE_AUDIT_ERR" >&2
	rc=0
	if [ -n "${FAKE_AUDIT:-}" ] && [ -s "$FAKE_AUDIT" ]; then
		cat "$FAKE_AUDIT"
		rc=1
	fi
	exit "${FAKE_AUDIT_RC:-$rc}"
	;;
*) exit 2 ;;
esac
EOF

cat >"$fake/pkg_info" <<'EOF'
#!/bin/sh
i=0
while [ "$i" -lt "${FAKE_PKGS:-3}" ]; do
	printf 'pkg%d-1.0 some package\n' "$i"
	i=$((i + 1))
done
EOF

cat >"$fake/logger" <<'EOF'
#!/bin/sh
printf '%s\n' "$*" >>"${FAKE_SYSLOG:?}"
EOF

# The fake web: every URL maps to a file in $web (vulns.gz, releases.html,
# index.html, NetBSD-SA*.txt.asc); a missing file is HTTP 404 (exit 22).
# FAKE_DOWN / FAKE_304: space-separated keys (vulns releases index adv, or
# an advisory file name) that fail to connect / answer 304 to a
# conditional GET. Every argument list is appended to FAKE_CURL_LOG.
cat >"$fake/curl" <<'EOF'
#!/bin/sh
printf '%s\n' "$*" >>"${FAKE_CURL_LOG:?}"
out= cond=
while [ $# -gt 1 ]; do
	case $1 in
	-o) out=$2; shift ;;
	-z) cond=1; shift ;;
	esac
	shift
done
url=$1
case $url in
*/pkg-vulnerabilities.gz) key=vulns file=vulns.gz ;;
*/releases/) key=releases file=releases.html ;;
*/advisories/) key=index file=index.html ;;
*/NetBSD-SA*) key=adv file=${url##*/} ;;
*) exit 6 ;;
esac
case " ${FAKE_DOWN:-} " in *" $key "* | *" $file "*) exit 7 ;; esac
if [ -n "$cond" ]; then
	case " ${FAKE_304:-} " in *" $key "* | *" $file "*) exit 0 ;; esac
fi
[ -f "$FAKE_WEB/$file" ] || exit 22
cp "$FAKE_WEB/$file" "$out"
EOF

# FAKE_MV_FAIL=<basename>: mv onto that name fails; every other mv is real.
real_mv=$(command -v mv)
cat >"$fake/mv" <<EOF
#!/bin/sh
if [ -n "\${FAKE_MV_FAIL:-}" ]; then
	for last in "\$@"; do :; done
	case \$last in
	*/"\$FAKE_MV_FAIL") exit 1 ;;
	esac
fi
exec $real_mv "\$@"
EOF
chmod +x "$fake"/*

# --- fixtures ------------------------------------------------------------

today_slash=$(date -u +%Y/%m/%d)
old_slash=$(date -u -d '-40 days' +%Y/%m/%d)

# vulns <revision> <YYYY/MM/DD> [truncated]: publishes a list in $web.
vulns() {
	{
		printf -- '-----BEGIN PGP SIGNED MESSAGE-----\nHash: SHA1\n\n'
		# shellcheck disable=SC2016 # a literal $NetBSD header
		printf '# $NetBSD: pkg-vulnerabilities,v 1.%s %s 15:02:56 taca Exp $\n' \
			"$1" "$2"
		printf '#FORMAT 1.1.0\nlibxml2<2.15.2 denial-of-service https://x/CVE-1\n'
		[ "${3:-}" = truncated ] \
			|| printf -- '-----BEGIN PGP SIGNATURE-----\nabc\n-----END PGP SIGNATURE-----\n'
	} | gzip -c >"$web/vulns.gz"
}

# releases <series:newest>...: the Supported Releases section, followed by
# a -current section that names an unsupported series (must be ignored).
releases() {
	typeset s
	{
		printf '<html><body>\n<div class="sect1" id="supported">\n'
		printf '<h2 class="titlepage">Supported Releases</h2>\n'
		for s in "$@"; do
			printf '<a href="formal-%s/">NetBSD %s.x</a> series:\n' \
				"${s%%:*}" "${s%%:*}"
			printf '<li><a href="formal-%s/NetBSD-%s.html">NetBSD %s</a></li>\n' \
				"${s%%:*}" "${s#*:}" "${s#*:}"
		done
		printf '</div>\n<div class="sect1" id="current">\n'
		printf '<a href="formal-12/NetBSD-12.0.html">x</a>\n</div></body>\n</html>\n'
	} >"$web/releases.html"
}

# index <advisory-name>...: the advisory directory listing.
index() {
	typeset a
	{
		printf '<html><body><table>\n'
		for a in "$@"; do
			printf '<tr><td><a href="%s.txt.asc">%s.txt.asc</a>\n' "$a" "$a"
		done
		printf '</table>\n</body></html>\n'
	} >"$web/index.html"
}

# advisory <name> <version-block-lines> <fixed-block-lines>
advisory() {
	{
		printf -- '-----BEGIN PGP SIGNED MESSAGE-----\n\n'
		printf '\t\tNetBSD Security Advisory %s\n\nTopic:\t\tx\n\n' "$1"
		printf 'Version:\t%b\n\nSeverity:\tx\n\n' "$2"
		printf 'Fixed:\t\t%b\n\nAbstract\n========\n' "$3"
	} >"$web/$1.txt.asc"
}

# The default healthy web: current list, 11.0 newest of a supported
# series, an index of old advisories plus in-scope ones that do not
# concern 11.0.
old_advisories="NetBSD-SA1998-001 NetBSD-SA2019-001 NetBSD-SA2023-001"
default_web() {
	rm -f "$web"/*
	vulns 794 "$today_slash"
	releases 11:11.0 10:10.2
	# Out of scope (before 2024): never fetched, so no file is needed.
	advisory NetBSD-SA2024-001 \
		'NetBSD-current:\taffected\n\t\tNetBSD 10.0_RC4:\taffected' \
		'NetBSD-current:\t2023-09-30\n\t\tNetBSD-10 branch:\t2024-02-17'
	advisory NetBSD-SA2024-002 \
		'NetBSD-current:\taffected\n\t\tNetBSD 10.0:\taffected' \
		'NetBSD-current:\t2024-07-01\n\t\tNetBSD-10 branch:\t2024-07-01'
	# shellcheck disable=SC2086 # word list
	index $old_advisories NetBSD-SA2024-001 NetBSD-SA2024-002
}

state="$work/state"
log="$work/audit.log"
syslog="$work/syslog"
curl_log="$work/curl.log"
audit_out="$work/audit.findings"

# run [VAR=value...]: runs the script, sets rc, keeps the state dir.
run() {
	rc=0
	: >"$curl_log"
	: >"$syslog"
	env NETBSD_VULN_AUDIT_TEST_PATH="$fake" \
		NETBSD_VULN_AUDIT_TEST_LOG="$log" \
		NETBSD_VULN_AUDIT_TEST_STATE_DIR="$state" \
		NETBSD_VULN_AUDIT_TEST_MIN_ADVISORIES=3 \
		FAKE_PKGVULNDIR="$vulndir" FAKE_WEB="$web" \
		FAKE_CURL_LOG="$curl_log" FAKE_SYSLOG="$syslog" \
		FAKE_AUDIT="$audit_out" ${1+"$@"} \
		"$shell" "$script" >"$work/out" 2>&1 || rc=$?
}

fail() {
	printf 'FAIL: %s\n' "$*" >&2
	cat "$work/out" >&2
	[ -f "$state/status" ] && cat "$state/status" >&2
	exit 1
}

expect() {
	typeset want_rc="$1" want_status="$2" label="$3"
	[ "$rc" -eq "$want_rc" ] || fail "$label: rc=$rc, want $want_rc"
	grep -qx "status=$want_status" "$state/status" \
		|| fail "$label: status is not $want_status"
}

# has <file> <regex> <label> / hasnt: grep assertions with a failure label.
has() { grep -q -- "$2" "$1" || fail "$3"; }
hasnt() { ! grep -q -- "$2" "$1" || fail "$3"; }
field() { has "$state/status" "^$1\$" "$2: want $1"; }

# fresh: empty state, empty pkgdb, default web, no package findings.
fresh() {
	rm -rf "$state"
	rm -f "$vulndir"/*
	: >"$audit_out"
	default_web
}

# age_contact <source> <hours>: pretends the last contact was <hours> ago.
age_contact() {
	printf '%d\n' $(($(date -u +%s) - $2 * 3600)) >"$state/contact.$1"
}

finding() {
	printf 'Package %s has a %s vulnerability, see %s\n' "$1" "$2" "$3" \
		>>"$audit_out"
}

# --- package audit -------------------------------------------------------

fresh
run
expect 0 OK 'clean run'
field 'pkg_status=OK' 'pkg component'
field 'base_status=OK' 'base component'
field 'pkg_vulnerabilities_revision=1.794' 'list revision'
field 'installed_packages=3' 'package count'
[ -f "$vulndir/pkg-vulnerabilities" ] || fail 'list not installed in PKGVULNDIR'
has "$work/out" '^warning: status changed: none -> OK' 'first status change'
has "$syslog" '^-p daemon.info -t netbsd-vuln-audit -- OK' 'OK not at info'
has "$log" 'vuln-audit: OK' 'not in the shared log'
hasnt "$curl_log" ' -z ' 'first fetch must be unconditional'
hasnt "$curl_log" 'NetBSD-SA2023-001' 'out-of-scope advisory fetched'

run
expect 0 OK 'quiet rerun'
[ ! -s "$work/out" ] || fail 'quiet OK run printed (cron would mail)'
has "$curl_log" 'pkg-vulnerabilities.gz' 'list not refreshed'
has "$curl_log" ' -z .*pkg-vulnerabilities.gz' 'cached list must use a conditional GET'

finding perl-5.42.3 symlink-attack https://nvd/CVE-2011-4116
finding libxml2-2.15.1 denial-of-service https://nvd/CVE-2025-8732
run
expect 0 VULNERABLE 'package findings'
field 'pkg_findings=2' 'package finding count'
field 'new_findings=2' 'first findings are new'
has "$state/findings" '^pkg perl https://nvd/CVE-2011-4116$' 'finding key'
has "$work/out" '^warning: NEW: 2 finding(s): pkg libxml2' 'NEW warning'
has "$work/out" '^warning: status changed: OK -> VULNERABLE' 'status change'
has "$syslog" 'daemon.notice .*VULNERABLE' 'VULNERABLE not at notice'
has "$state/pkg-audit.out" 'Package perl-5.42.3' 'raw audit output kept'

run
expect 0 VULNERABLE 'unchanged findings'
field 'new_findings=0' 'rerun: nothing new'
[ ! -s "$work/out" ] || fail 'unchanged VULNERABLE printed'

# A package update keeps the pkgbase key: no new finding for a still
# vulnerable newer version; a fixed one is resolved.
: >"$audit_out"
finding perl-5.42.4 symlink-attack https://nvd/CVE-2011-4116
run
expect 0 VULNERABLE 'package updated'
field 'new_findings=0' 'version bump must not re-alert'
field 'resolved=1' 'fixed advisory resolved'
has "$state/resolved-findings" 'CVE-2025-8732' 'resolved list'
has "$syslog" 'daemon.notice .*RESOLVED: 1 finding' 'resolved notice'
[ ! -s "$work/out" ] || fail 'a resolved finding alone must not print'

today=$(date -u +%F)
has "$state/new-findings.history" "^$today pkg libxml2 " 'history batch'
[ ! -s "$state/new-findings" ] || fail 'new-findings not reset by a quiet run'
printf '2000-01-01 pkg old https://x\n' >>"$state/new-findings.history"
finding bash-5.0 remote-code-execution https://nvd/CVE-1
run
hasnt "$state/new-findings.history" 'pkg old ' 'history not pruned'
[ "$(grep -c 'pkg libxml2' "$state/new-findings.history")" -eq 1 ] \
	|| fail 'history lost or duplicated a batch'

# --- package coverage failures -------------------------------------------

fresh
rm -f "$web/vulns.gz"
run
expect 3 UNKNOWN 'list never fetched'
field 'pkg_status=UNKNOWN' 'pkg component unknown'
field 'base_status=OK' 'base still assessed'
has "$work/out" '^err: UNKNOWN: pkgs UNKNOWN: no pkg-vulnerabilities list' \
	'never-fetched err'
has "$syslog" 'daemon.err' 'UNKNOWN not at err'

fresh
run
run FAKE_DOWN=vulns
expect 0 OK 'one failed refresh is tolerated'
field 'fetch_pkg_vulnerabilities=failed' 'fetch failure recorded'
has "$work/out" '^warning: WARNING: download failed' 'fetch warning'
age_contact vulns 40
run FAKE_DOWN=vulns
expect 3 UNKNOWN 'refresh failing past 36 h'
has "$state/status" 'pkg-vulnerabilities source not refreshed for 40h' \
	'stale contact reason'
run FAKE_304=vulns
expect 0 OK 'a 304 answer counts as contact'
field 'fetch_pkg_vulnerabilities=unchanged' '304 not recorded'

# A corrupt or truncated download never replaces the installed list and
# does not count as contact.
vulns 795 "$today_slash" truncated
age_contact vulns 30
run
expect 0 OK 'truncated download keeps the list'
field 'fetch_pkg_vulnerabilities=corrupt' 'truncated download recorded'
field 'pkg_vulnerabilities_revision=1.794' 'truncated list installed'
age_contact vulns 40
run
expect 3 UNKNOWN 'truncated downloads past 36 h'
printf 'not gzip\n' >"$web/vulns.gz"
run
field 'fetch_pkg_vulnerabilities=corrupt' 'non-gzip download recorded'

# An older revision from a stale mirror is not installed (but the server
# answered, so it counts as contact).
fresh
run
vulns 700 "$today_slash"
age_contact vulns 40
run
expect 0 OK 'older server copy'
field 'fetch_pkg_vulnerabilities=older' 'older copy recorded'
field 'pkg_vulnerabilities_revision=1.794' 'older copy installed'
has "$work/out" 'older than the installed one' 'older copy warning'

# A list whose own date is older than 30 days means the source stalled.
fresh
vulns 794 "$old_slash"
run
expect 3 UNKNOWN 'stale list'
has "$state/status" 'days old (> 30): source stale' 'stale list reason'

# A list that passes the format check but lacks the revision header must
# not get a date from nowhere.
fresh
run
sed '/^# .NetBSD: pkg-vulnerabilities/d' "$vulndir/pkg-vulnerabilities" >"$work/nohdr"
cp "$work/nohdr" "$vulndir/pkg-vulnerabilities"
run FAKE_304=vulns
expect 3 UNKNOWN 'list without a revision header'
has "$state/status" 'no .NetBSD revision header' 'missing header reason'

# A deleted list is restored from the cache even on a 304.
fresh
run
rm "$vulndir/pkg-vulnerabilities"
run FAKE_304=vulns
expect 0 OK 'deleted list restored'
[ -f "$vulndir/pkg-vulnerabilities" ] || fail 'list not restored'
has "$work/out" 'WARNING: restored' 'restore warning'

fresh
run FAKE_AUDIT_ERR='pkg_admin: database corrupt'
expect 3 UNKNOWN 'pkg_admin stderr'
has "$state/status" 'pkg_admin audit failed (rc=0): pkg_admin: database corrupt' \
	'pkg_admin error reason'
printf 'garbage line\n' >"$audit_out"
run
expect 3 UNKNOWN 'unparsable audit output'
has "$state/status" 'unparsable pkg_admin audit output' 'unparsable reason'
: >"$audit_out"
run FAKE_AUDIT_RC=1
expect 3 UNKNOWN 'non-zero audit without findings'
run FAKE_PKGS=0
expect 3 UNKNOWN 'no installed packages'
has "$state/status" 'pkg_info lists no installed packages' 'no packages reason'

# --- base audit ----------------------------------------------------------

fresh
releases 11:11.1 10:10.2
run
expect 0 VULNERABLE 'newer release in the series'
field 'series_newest=11.1' 'newest release'
has "$state/findings" '^base release NetBSD-11.1$' 'release finding'
field 'pkg_status=OK' 'packages unaffected by base findings'

fresh
releases 13:13.0 12:12.1
run
expect 0 VULNERABLE 'end-of-life series'
has "$state/findings" '^base eol NetBSD-11.x$' 'eol finding'
field 'supported_series=13 12' 'supported series (section only)'

# advisory_case <label> <want-status> <version-lines> <fixed-lines>
advisory_case() {
	typeset label="$1" want="$2"
	fresh
	advisory NetBSD-SA2026-001 "$3" "$4"
	# shellcheck disable=SC2086 # word list
	index $old_advisories NetBSD-SA2024-001 NetBSD-SA2024-002 NetBSD-SA2026-001
	run
	case $want in
	UNKNOWN) expect 3 UNKNOWN "$label" ;;
	*) expect 0 "$want" "$label" ;;
	esac
}

advisory_case 'release line affected' VULNERABLE \
	'NetBSD-current:\taffected\n\t\tNetBSD 11.0:\taffected\n\t\tNetBSD 10.2:\taffected' \
	'NetBSD-current:\t2026-08-01\n\t\tNetBSD-11 branch:\t2026-08-02'
has "$state/findings" '^base advisory NetBSD-SA2026-001$' 'advisory finding'
has "$work/out" 'NEW: 1 finding(s): base advisory NetBSD-SA2026-001' \
	'advisory NEW warning'
advisory_case 'release line unaffected' OK \
	'NetBSD-current:\taffected\n\t\tNetBSD 11.0:\tunaffected' \
	'NetBSD-current:\t2026-08-01'
advisory_case 'branch fixed after the build' VULNERABLE \
	'NetBSD-current:\taffected\n\t\tNetBSD 11.0_RC1:\taffected' \
	'NetBSD-current:\t2026-08-01\n\t\tNetBSD-11 branch:\t2026-08-02'
advisory_case 'branch fixed before the build' OK \
	'NetBSD-current:\taffected\n\t\tNetBSD 11.0_RC1:\taffected' \
	'NetBSD-current:\t2026-05-01\n\t\tNetBSD-11 branch:\tMay 2, 2026'
advisory_case 'long-form branch date after the build' VULNERABLE \
	'NetBSD-current:\taffected\n\t\tNetBSD 11.*:\taffected' \
	'NetBSD-11 branch:\tAugust 3rd, 2026'
advisory_case 'series line without a branch date' VULNERABLE \
	'NetBSD-current:\taffected\n\t\tNetBSD 11.*:\taffected' \
	'NetBSD-current:\t2026-08-01'
advisory_case 'series line wins over nothing, branch N/A' OK \
	'NetBSD-current:\taffected\n\t\tNetBSD 11.*:\taffected' \
	'NetBSD-11 branch:\tN/A'
advisory_case 'other series only' OK \
	'NetBSD-current:\taffected\n\t\tNetBSD 10.2:\taffected' \
	'NetBSD-10 branch:\t2026-08-02'
advisory_case 'release candidate only, no branch line' UNKNOWN \
	'NetBSD-current:\taffected\n\t\tNetBSD 11.0_RC2:\taffected' \
	'NetBSD-current:\t2026-08-01'
has "$state/status" 'cannot assess advisories: NetBSD-SA2026-001' \
	'unassessable reason'
field 'pkg_status=OK' 'packages still assessed'
advisory_case 'unparsable branch date' UNKNOWN \
	'NetBSD-current:\taffected' 'NetBSD-11 branch:\tsoon'

# An advisory without a Version block cannot be assessed.
fresh
printf 'NetBSD Security Advisory\nno blocks\n' >"$web/NetBSD-SA2026-002.txt.asc"
# shellcheck disable=SC2086 # word list
index $old_advisories NetBSD-SA2024-001 NetBSD-SA2024-002 NetBSD-SA2026-002
run
expect 3 UNKNOWN 'advisory without a Version block'

# A listed advisory that cannot be downloaded (and was never cached).
fresh
# shellcheck disable=SC2086 # word list
index $old_advisories NetBSD-SA2024-001 NetBSD-SA2024-002 NetBSD-SA2026-009
run
expect 3 UNKNOWN 'advisory never downloaded'
has "$work/out" 'cannot refresh advisories: NetBSD-SA2026-009' \
	'advisory download warning'

# A cached advisory survives a failed refresh.
fresh
run
run FAKE_DOWN=NetBSD-SA2024-002.txt.asc
expect 0 OK 'cached advisory, failed refresh'
has "$work/out" 'cannot refresh advisories: NetBSD-SA2024-002' \
	'cached advisory refresh warning'

# --- base coverage failures ----------------------------------------------

fresh
printf '<html>no section</html>\n' >"$web/releases.html"
run
expect 3 UNKNOWN 'releases page without the section'
field 'fetch_releases=corrupt' 'incomplete page recorded'
has "$state/status" 'releases page never fetched successfully' \
	'never-fetched releases reason'

fresh
releases 11:11.0
run
printf '<html><h2>Supported Releases</h2>\n' >"$web/releases.html"
run
expect 0 OK 'truncated releases page keeps the cached copy'
field 'fetch_releases=corrupt' 'truncated page recorded'
age_contact releases 40
run
expect 3 UNKNOWN 'releases page not refreshed past 36 h'

fresh
printf '<html><body>\nSupported Releases\nnothing\n</body></html>\n' \
	>"$web/releases.html"
run
expect 3 UNKNOWN 'releases section without series'
has "$state/status" 'cannot parse the supported releases' 'parse reason'

fresh
index NetBSD-SA2024-001 NetBSD-SA2024-002
run
expect 3 UNKNOWN 'index below the floor'
has "$state/status" 'lists 2 advisories (< 3): truncated' 'floor reason'

fresh
run
# shellcheck disable=SC2086 # word list
index $old_advisories NetBSD-SA2024-001 NetBSD-SA2025-001
advisory NetBSD-SA2025-001 'NetBSD 10.2:\taffected' 'NetBSD-10 branch:\tN/A'
run
expect 3 UNKNOWN 'index lost a cached advisory'
has "$state/status" 'no longer lists NetBSD-SA2024-002.txt.asc' 'lost reason'
rm "$state/advisories/NetBSD-SA2024-002.txt.asc"
run
expect 0 OK 'operator accepted the removal'

fresh
run FAKE_REL=11.0_STABLE
expect 3 UNKNOWN 'non-release kernel'
has "$state/status" 'kernel 11.0_STABLE is not a formal release' \
	'non-release reason'
run FAKE_UNAMEV='NetBSD 11.0 (GENERIC64) #0'
expect 3 UNKNOWN 'unparsable build date'

# --- per-component baseline ----------------------------------------------

# Base UNKNOWN: new package findings still alert and are committed; the
# base baseline lines stay until base is assessed again.
fresh
releases 11:11.1 10:10.2
run
has "$state/findings" '^base release NetBSD-11.1$' 'base baseline'
finding bash-5.0 remote-code-execution https://nvd/CVE-1
run FAKE_DOWN=releases
age_contact releases 40
run FAKE_DOWN=releases
expect 3 UNKNOWN 'base unknown, pkgs assessed'
has "$state/findings" '^base release NetBSD-11.1$' 'base baseline dropped'
has "$state/findings" '^pkg bash ' 'package finding not committed'
run
expect 0 VULNERABLE 'base back'
field 'new_findings=0' 'base finding re-reported after coverage returned'

# Pkgs UNKNOWN while a new base finding appears: base alerts, the package
# baseline stays and its finding is not reported as resolved.
fresh
finding bash-5.0 remote-code-execution https://nvd/CVE-1
run
rm -f "$web/vulns.gz"
age_contact vulns 40
releases 11:11.1 10:10.2
run
expect 3 UNKNOWN 'pkgs unknown, base assessed'
field 'new_findings=1' 'base finding alerted during a package outage'
field 'resolved=0' 'package finding resolved by an outage'
has "$state/findings" '^pkg bash ' 'package baseline dropped'

# --- baseline and state failures -----------------------------------------

fresh
finding bash-5.0 remote-code-execution https://nvd/CVE-1
run
printf 'pkg bash https://nvd/CVE-1\nbase release NetBSD-11.1\n' \
	>"$state/findings"
run
has "$work/out" 'WARNING: baseline .* unsorted or malformed' \
	'invalid baseline warning'
field 'new_findings=1' 'invalid baseline counts as empty'
printf 'garbage\n' >"$state/findings"
run
field 'new_findings=1' 'malformed baseline counts as empty'

# A history write failure: its own reason, baseline untouched, the
# findings are reported (and recorded once) by the next run.
fresh
finding bash-5.0 remote-code-execution https://nvd/CVE-1
run FAKE_MV_FAIL=new-findings.history
expect 3 UNKNOWN 'history write failure'
has "$state/status" '^reason=cannot update the new-finding history' \
	'history failure reason'
has "$work/out" 'NEW: 1 finding' 'failed-commit run must still alert'
[ ! -f "$state/findings" ] || fail 'baseline committed despite history failure'
run
expect 0 VULNERABLE 'after a history failure'
field 'new_findings=1' 'history failure absorbed the finding'
[ "$(grep -c 'pkg bash' "$state/new-findings.history")" -eq 1 ] \
	|| fail 'history entry missing or duplicated'

# A baseline commit failure after the history was written keeps the first
# alerting date and does not duplicate the history line.
fresh
finding bash-5.0 remote-code-execution https://nvd/CVE-1
run FAKE_MV_FAIL=findings
expect 3 UNKNOWN 'baseline commit failure'
has "$state/status" '^reason=cannot update the baseline' 'baseline reason'
yesterday=$(date -u -d '-1 day' +%F)
sed "s/^[0-9-]* /$yesterday /" "$state/new-findings.history" >"$work/h"
cp "$work/h" "$state/new-findings.history"
run
field 'new_findings=1' 'failed commit consumed the finding'
has "$state/new-findings.history" "^$yesterday pkg bash " \
	'history must keep the first alerting date'
[ "$(grep -c 'pkg bash' "$state/new-findings.history")" -eq 1 ] \
	|| fail 'history duplicated a re-reported finding'

# A failed resolved-list write names that file and still alerts.
fresh
finding bash-5.0 remote-code-execution https://nvd/CVE-1
run FAKE_MV_FAIL=resolved-findings
expect 3 UNKNOWN 'resolved-list write failure'
has "$state/status" 'cannot write the resolved list' 'resolved-list reason'
has "$work/out" 'NEW: 1 finding' 'resolved-list failure must still alert'

# A failed status write keeps the previous record and does not consume
# the new findings.
fresh
run
cp "$state/status" "$work/status.before"
mkdir "$state/status.tmp"
finding bash-5.0 remote-code-execution https://nvd/CVE-1
run
[ "$rc" -eq 3 ] || fail "failed status write rc=$rc, want 3"
cmp -s "$state/status" "$work/status.before" \
	|| fail 'failed status write replaced the record'
has "$work/out" '^err: UNKNOWN: cannot write' 'status write error'
rmdir "$state/status.tmp"
run
expect 0 VULNERABLE 'after a failed status write'
field 'new_findings=1' 'unrecorded run consumed the finding'

fresh
mkdir -p "$state/lock"
run
[ "$rc" -eq 0 ] || fail "locked run rc=$rc"
has "$syslog" 'skipped, another run holds' 'lock message'
[ ! -s "$work/out" ] || fail 'a lock skip must not print'

# An uncreatable state directory must not pass as "another run holds it".
printf 'file\n' >"$work/not-a-dir"
rc=0
env NETBSD_VULN_AUDIT_TEST_PATH="$fake" NETBSD_VULN_AUDIT_TEST_LOG="$log" \
	NETBSD_VULN_AUDIT_TEST_STATE_DIR="$work/not-a-dir/state" \
	FAKE_SYSLOG="$syslog" "$shell" "$script" >"$work/out" 2>&1 || rc=$?
[ "$rc" -eq 3 ] || fail "uncreatable state dir rc=$rc, want 3"

printf 'netbsd vuln audit tests passed (%s)\n' "$shell"
