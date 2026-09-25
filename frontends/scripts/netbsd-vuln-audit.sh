#!/bin/ksh
#
# netbsd-vuln-audit — daily vulnerability audit of the NetBSD pi0/pi1 hosts
# (task 652). Two components, each with its own coverage status:
#
#   pkgs  pkgsrc packages: refreshes the pkgsrc-security pkg-vulnerabilities
#         list (HTTPS, conditional GET, format + embedded SHA512 hash checked
#         by `pkg_admin check-pkg-vulnerabilities`), installs it where
#         pkg_admin looks for it (PKGVULNDIR, i.e. /usr/pkg/pkgdb) and runs
#         `pkg_admin audit` over every installed package.
#   base  NetBSD base system and kernel: the installed formal release
#         (`uname -r`, e.g. 11.0) is checked against the "Supported Releases"
#         section of https://www.netbsd.org/releases/ (series end of life, or
#         a newer release of the same series) and against the NetBSD security
#         advisories (NetBSD-SA*) that can concern it.
#
# Result, per component and overall (the worse of the two):
#
#   OK          fresh, complete data; nothing found
#   VULNERABLE  at least one finding (a package advisory, an end-of-life
#               package, a base advisory naming the installed release, an
#               unsupported series, or a newer release of the installed
#               series)
#   UNKNOWN     coverage unavailable: a source never fetched or not refreshed
#               within MAX_CONTACT_AGE_HOURS, an invalid/truncated download
#               with no valid copy, a pkg-vulnerabilities list older than
#               MAX_VULNS_AGE_DAYS, pkg_admin failing or printing output this
#               script cannot parse, no installed packages visible, a
#               releases page or advisory index that cannot be parsed (or
#               lost an advisory seen before), an advisory that cannot be
#               assessed, a non-release kernel, a clock that is not
#               NTP-synchronised or lies behind a recorded time, or state
#               that cannot be written
#
# Alerting (the Pis have no MTA; plan = docs/archive/frontends/docs/
# unattended-upgrades-pi.plan.md, §9.5/§14): only UNKNOWN — broken
# coverage — exits non-zero (3). VULNERABLE is expected to be the steady
# state (e.g. perl's permanent CVE-2011-4116 entry), so it exits 0 and is
# reported against a baseline, the findings of the last run that assessed
# the component: findings not in the baseline and any status change are
# logged at warning priority and listed in $NEW_FILE (plus the dated
# $HISTORY_FILE, one line per finding with the date of the run that first
# alerted it); findings that went away are a notice in $RESOLVED_FILE. Every
# line goes to the shared /var/log/unattended-upgrade.log (tag
# "vuln-audit:") and to syslog (tag netbsd-vuln-audit, facility daemon);
# only err and warning lines are printed on stdout, for an interactive run.
# The root cron job (gonf VulnAuditCron) discards stdout, since those lines
# are in the log and syslog already and root's local mail is never read.
#
# The baseline is per component: a run that assesses only one component
# (the other is UNKNOWN) reports and commits that component's findings and
# leaves the other one's baseline lines untouched, so findings that appear
# during an outage are still reported once coverage returns.
#
# Operator overrides (as root, in $STATE_DIR): delete `findings` to
# re-baseline (every current finding is reported as new once); delete a
# cached advisories/NetBSD-SA*.txt.asc to accept that the index no longer
# lists it.
#
# Remediation stays manual on purpose: pkgsrc packages are updated by
# unattended-upgrade-netbsd (pkgin), base-system and kernel updates are
# release upgrades done by hand (sysupgrade), see plan §3.
#
# Shell: NetBSD's /bin/ksh (pdksh) has no pipefail and no here-strings, so
# pipelines whose status matters write to files first. typeset in a
# name() function is local in pdksh and bash but NOT in ksh93, which the
# test harness (tests/netbsd-vuln-audit.ksh) also uses, so helper variables
# get names that never collide across functions or with the result globals.

PATH=/usr/pkg/sbin:/usr/pkg/bin:/usr/bin:/bin:/usr/sbin:/sbin
if [ -n "${NETBSD_VULN_AUDIT_TEST_PATH:-}" ]; then
	PATH="${NETBSD_VULN_AUDIT_TEST_PATH}:$PATH"
fi
export PATH

umask 077

readonly VULNS_URL=https://cdn.NetBSD.org/pub/NetBSD/packages/vulns/pkg-vulnerabilities.gz
readonly RELEASES_URL=https://www.netbsd.org/releases/
readonly ADVISORY_URL=https://cdn.NetBSD.org/pub/NetBSD/security/advisories/
readonly FETCH_TIMEOUT=300
# The audit runs daily: one failed refresh is tolerated (a warning), the
# second in a row makes the component UNKNOWN.
readonly MAX_CONTACT_AGE_HOURS=36
# pkgsrc-security revises pkg-vulnerabilities several times a week; a list
# whose own $NetBSD date is older than this means the source stopped moving.
readonly MAX_VULNS_AGE_DAYS=${NETBSD_VULN_AUDIT_TEST_MAX_VULNS_AGE_DAYS:-30}
# Floor for the advisory index (289 advisories in 2026-09; never shrinks).
readonly MIN_ADVISORIES=${NETBSD_VULN_AUDIT_TEST_MIN_ADVISORIES:-250}
# Advisories older than the kernel build year minus this cannot name the
# installed release or its branch (a branch is cut at most ~15 months
# before its .0 release), so they are not fetched.
readonly ADVISORY_LOOKBACK_YEARS=2
# The Pis have no RTC: after a power-off the clock starts behind until ntpd
# syncs. Every age check depends on it, so the run waits up to
# CLOCK_WAIT_SECS for ntpd's sync (ntpq -c rv); without it both components
# are UNKNOWN and no contact is recorded.
readonly CLOCK_WAIT_SECS=${NETBSD_VULN_AUDIT_TEST_CLOCK_WAIT_SECS:-300}
# unattended-upgrade-netbsd's whole-job lock (a directory). It is taken
# around pkg_info + pkg_admin audit only (seconds), so the audit never
# reads a pkgdb that pkgin is changing and the upgrade job, which skips a
# held lock, is not kept out by the network fetches. A lock held longer
# than UPGRADE_LOCK_WAIT_SECS skips the whole run (warning, exit 0, status
# record untouched); the next daily run retries. Like the upgrade job, a
# lock older than 2 h is stolen (a crashed run).
readonly UPGRADE_LOCK=${NETBSD_VULN_AUDIT_TEST_UPGRADE_LOCK:-/var/run/unattended-upgrade.lock}
readonly UPGRADE_LOCK_WAIT_SECS=${NETBSD_VULN_AUDIT_TEST_UPGRADE_LOCK_WAIT_SECS:-1800}
# Days of dated new-finding batches kept in $HISTORY_FILE.
readonly HISTORY_DAYS=90
# How many findings a NEW/RESOLVED line names before pointing at the list.
readonly IDS_IN_LOG=10

LOG=${NETBSD_VULN_AUDIT_TEST_LOG:-/var/log/unattended-upgrade.log}
STATE_DIR=${NETBSD_VULN_AUDIT_TEST_STATE_DIR:-/var/db/netbsd-vuln-audit}
LOCK=$STATE_DIR/lock
STATUS_FILE=$STATE_DIR/status
# Baseline: sorted "pkg <pkgbase> <url>" / "base <kind> <id>" lines of the
# last run that assessed each component.
FINDINGS=$STATE_DIR/findings
FINDINGS_NEXT=$STATE_DIR/findings.next
PKG_NEXT=$STATE_DIR/findings.pkg.next
BASE_NEXT=$STATE_DIR/findings.base.next
# new-findings: the latest run's batch; new-findings.history: "<YYYY-MM-DD>
# <finding>" for HISTORY_DAYS, one line per finding (first alerting date).
NEW_FILE=$STATE_DIR/new-findings
HISTORY_FILE=$STATE_DIR/new-findings.history
RESOLVED_FILE=$STATE_DIR/resolved-findings
PKG_AUDIT_OUT=$STATE_DIR/pkg-audit.out
VULNS_CACHE=$STATE_DIR/pkg-vulnerabilities.gz
RELEASES_CACHE=$STATE_DIR/releases.html
ADV_DIR=$STATE_DIR/advisories
ADV_INDEX=$ADV_DIR/index.html
TMP=$STATE_DIR/tmp

# Result fields, filled in by the steps below and written by write_status.
status=UNKNOWN
previous_status=none
pkg_status=UNKNOWN
pkg_reason="not assessed"
base_status=UNKNOWN
base_reason="not assessed"
vulns_file=unknown
vulns_revision=unknown
vulns_date=unknown
fetch_vulns=skipped
fetch_releases=skipped
fetch_advisories=skipped
release=unknown
build_date=unknown
supported_series=unknown
series_newest=unknown
typeset -i installed_pkgs=0 pkg_findings=0 base_findings=0
typeset -i advisories_listed=0 advisories_in_scope=0
typeset -i new_count=0 resolved_count=0
clock_synced=unknown
# yes while this run holds $UPGRADE_LOCK (released by the EXIT trap too).
upgrade_lock_held=no
# Set by fetch_url: updated | unchanged | failed.
fetched=""
# Set by commit_baseline when it fails: which state file could not be written.
commit_error=""

# log <priority> <message>: shared log file, syslog, and stdout for err and
# warning only (a quiet interactive run prints nothing).
log() {
	typeset prio="$1"
	shift
	printf '[%s] vuln-audit: %s\n' "$(date '+%Y-%m-%dT%H:%M:%S')" "$*" \
		>>"$LOG"
	logger -p "daemon.$prio" -t netbsd-vuln-audit -- "$*" 2>/dev/null
	case $prio in
	err | warning) printf '%s: %s\n' "$prio" "$*" ;;
	esac
}

# Whole-run lock; steal one older than 2 h (a killed run). $STATE_DIR must
# exist (main checks it, so a missing directory is not mistaken for a lock).
acquire_lock() {
	mkdir "$LOCK" 2>/dev/null && return 0
	[ -n "$(find "$LOCK" -mmin +120 2>/dev/null)" ] || return 1
	rmdir "$LOCK" 2>/dev/null && mkdir "$LOCK" 2>/dev/null
}

# fetch_url <url> <cache> <part>: downloads <url> to <part>, conditionally
# (If-Modified-Since the cached copy, -R keeps the server's mtime) when
# <cache> exists. Sets fetched = updated (validate and adopt <part>),
# unchanged (HTTP 304, nothing written) or failed.
fetch_url() {
	typeset f_url="$1" f_cache="$2" f_part="$3"
	rm -f "$f_part"
	if [ -f "$f_cache" ]; then
		curl -fsS -R --max-time "$FETCH_TIMEOUT" -z "$f_cache" \
			-o "$f_part" "$f_url" 2>/dev/null
	else
		curl -fsS -R --max-time "$FETCH_TIMEOUT" -o "$f_part" "$f_url" \
			2>/dev/null
	fi || {
		rm -f "$f_part"
		fetched=failed
		return 0
	}
	if [ -s "$f_part" ]; then
		fetched=updated
	else
		rm -f "$f_part"
		fetched=unchanged
	fi
}

# record_contact <source>: the source answered with valid (or unchanged)
# data now. contact_age_hours <source> prints the hours since; fails when
# the source never answered, and prints -1 when the stamp lies in the
# future (the clock went backwards): a negative age must never pass as
# fresh.
record_contact() {
	date -u +%s >"$STATE_DIR/contact.$1.tmp" \
		&& mv "$STATE_DIR/contact.$1.tmp" "$STATE_DIR/contact.$1"
}
contact_age_hours() {
	typeset since
	since=$(cat "$STATE_DIR/contact.$1" 2>/dev/null)
	case $since in
	'' | *[!0-9]*) return 1 ;;
	esac
	typeset -i now_secs
	now_secs=$(date -u +%s)
	if [ "$now_secs" -lt "$since" ]; then
		printf '%d\n' -1
		return 0
	fi
	printf '%d\n' $(((now_secs - since) / 3600))
}

# fresh_contact <source> <label>: fails with a reason in $fresh_reason
# unless the source answered within MAX_CONTACT_AGE_HOURS.
fresh_contact() {
	typeset -i contact_age
	contact_age=$(contact_age_hours "$1") || {
		fresh_reason="$2 never fetched successfully"
		return 1
	}
	if [ "$contact_age" -lt 0 ]; then
		fresh_reason="$2 last contact lies in the future: clock behind"
		return 1
	fi
	if [ "$contact_age" -gt "$MAX_CONTACT_AGE_HOURS" ]; then
		fresh_reason="$2 not refreshed for ${contact_age}h (> ${MAX_CONTACT_AGE_HOURS}h)"
		return 1
	fi
}

# --- pkgs: pkgsrc packages ---------------------------------------------

# vulns_header <file>: prints "<revision> <YYYY-MM-DD> <HH:MM:SS>" from the
# list's "# $NetBSD: pkg-vulnerabilities,v 1.<rev> <date> <time>" line.
vulns_header() {
	# shellcheck disable=SC2016 # a literal $NetBSD, not an expansion
	sed -n 's|^# \$NetBSD: pkg-vulnerabilities,v 1\.\([0-9][0-9]*\) \([0-9/]*\) \([0-9:]*\) .*|\1 \2 \3|p' \
		"$1" 2>/dev/null | sed -n '1s|/|-|gp'
}

# unpack_vulns <gz> <out>: decompresses and verifies a downloaded list
# (format and the embedded SHA512 hash, which also catches truncation).
unpack_vulns() {
	gzip -dc "$1" >"$2" 2>/dev/null \
		&& pkg_admin check-pkg-vulnerabilities "$2" >/dev/null 2>&1
}

# install_vulns <verified file>: atomically replaces $vulns_file (0644, as
# pkg_admin fetch-pkg-vulnerabilities would leave it).
install_vulns() {
	cp "$1" "$vulns_file.tmp" && chmod 644 "$vulns_file.tmp" \
		&& mv "$vulns_file.tmp" "$vulns_file" && return 0
	rm -f "$vulns_file.tmp"
	return 1
}

# revision_older <new> <old>: true when <new> has a lower revision than
# <old> (the server handed out an older copy, e.g. from a stale mirror).
revision_older() {
	typeset nrev orev
	nrev=$(vulns_header "$1" | awk '{ print $1 }')
	orev=$(vulns_header "$2" | awk '{ print $1 }')
	[ -n "$nrev" ] && [ -n "$orev" ] && [ "$nrev" -lt "$orev" ]
}

# refresh_vulns: conditional download of the list into $VULNS_CACHE and,
# once verified, into $vulns_file. fetch_vulns = updated | unchanged |
# failed | corrupt | older. A failed, corrupt or older download keeps the
# installed copy; only a verified answer counts as contact.
refresh_vulns() {
	typeset part="$TMP/vulns.gz" plain="$TMP/pkg-vulnerabilities"
	fetch_url "$VULNS_URL" "$VULNS_CACHE" "$part"
	fetch_vulns=$fetched
	case $fetched in
	failed) log warning "WARNING: download failed ($VULNS_URL)" ;;
	unchanged) record_contact vulns ;;
	updated)
		if ! unpack_vulns "$part" "$plain"; then
			fetch_vulns=corrupt
			log warning "WARNING: downloaded pkg-vulnerabilities fails verification, kept the previous copy"
		elif [ -f "$vulns_file" ] && revision_older "$plain" "$vulns_file"; then
			fetch_vulns=older
			log warning "WARNING: server copy of pkg-vulnerabilities is older than the installed one, kept it"
			record_contact vulns
		elif install_vulns "$plain" && mv "$part" "$VULNS_CACHE"; then
			record_contact vulns
		else
			log warning "WARNING: cannot install $vulns_file"
		fi
		;;
	esac
	restore_vulns
}

# restore_vulns: reinstalls the list from $VULNS_CACHE when the installed
# copy is missing or invalid (deleted by hand, damaged) but the cache is
# good, so a 304 answer does not leave pkg_admin without a list.
restore_vulns() {
	typeset cached_plain="$TMP/pkg-vulnerabilities"
	[ -f "$VULNS_CACHE" ] || return 0
	[ -f "$vulns_file" ] && pkg_admin check-pkg-vulnerabilities \
		"$vulns_file" >/dev/null 2>&1 && return 0
	unpack_vulns "$VULNS_CACHE" "$cached_plain" \
		&& install_vulns "$cached_plain" \
		&& log warning "WARNING: restored $vulns_file from $VULNS_CACHE"
}

# check_vulns_list: fails with pkg_reason unless the installed list is
# present, valid, recently confirmed with the server and not too old.
check_vulns_list() {
	typeset hdr epoch
	typeset -i age
	if [ ! -f "$vulns_file" ]; then
		pkg_reason="no pkg-vulnerabilities list at $vulns_file"
		return 1
	fi
	if ! pkg_admin check-pkg-vulnerabilities "$vulns_file" >/dev/null 2>&1
	then
		pkg_reason="$vulns_file fails pkg_admin check-pkg-vulnerabilities"
		return 1
	fi
	hdr=$(vulns_header "$vulns_file")
	# An empty date would parse as "today" in date -d: never let it pass.
	if [ -z "$hdr" ]; then
		pkg_reason="no \$NetBSD revision header in $vulns_file"
		return 1
	fi
	vulns_revision=1.${hdr%% *}
	vulns_date=${hdr#* }
	epoch=$(date -u -d "$vulns_date" +%s 2>/dev/null) || {
		pkg_reason="cannot parse the \$NetBSD header of $vulns_file"
		return 1
	}
	# The list is dated when pkgsrc-security commits it, so it can only be
	# newer than now if the clock is behind (1 h slack for server skew).
	if [ "$(date -u +%s)" -lt $((epoch - 3600)) ]; then
		pkg_reason="pkg-vulnerabilities $vulns_revision is dated $vulns_date, in the future: clock behind"
		return 1
	fi
	age=$((($(date -u +%s) - epoch) / 86400))
	if [ "$age" -gt "$MAX_VULNS_AGE_DAYS" ]; then
		pkg_reason="pkg-vulnerabilities $vulns_revision is ${age} days old (> $MAX_VULNS_AGE_DAYS): source stale"
		return 1
	fi
	fresh_contact vulns "pkg-vulnerabilities source" && return 0
	pkg_reason=$fresh_reason
	return 1
}

# run_pkg_audit: runs pkg_admin audit (under $UPGRADE_LOCK) and turns
# every "Package <name> has a <type> vulnerability, see <url>" line into
# "pkg <pkgbase> <url>" and every "Package <name> has reached end-of-life
# (eol), see <url>/eol-packages" line (CHECK_END_OF_LIFE=yes, the NetBSD
# 11 default) into "pkg <pkgbase> eol" in $PKG_NEXT. Anything else on
# stdout or stderr is an error: a silent failure must not look like a
# clean audit.
run_pkg_audit() {
	typeset -i audit_rc=0
	typeset audit_err
	pkg_admin audit >"$PKG_AUDIT_OUT" 2>"$TMP/audit.err" || audit_rc=$?
	if [ -s "$TMP/audit.err" ]; then
		audit_err=$(head -n 1 "$TMP/audit.err")
		pkg_reason="pkg_admin audit failed (rc=$audit_rc): $audit_err"
		return 1
	fi
	if ! awk '
		/^Package [^ ]+ has an? .+ vulnerability, see [^ ]+$/ {
			n = $2; sub(/-[^-]*$/, "", n); print "pkg", n, $NF; next
		}
		/^Package [^ ]+ has reached end-of-life \(eol\), see [^ ]+$/ {
			n = $2; sub(/-[^-]*$/, "", n); print "pkg", n, "eol"; next
		}
		{ bad = 1 }
		END { exit bad }
	' "$PKG_AUDIT_OUT" >"$TMP/pkg.raw"; then
		pkg_reason="unparsable pkg_admin audit output (see $PKG_AUDIT_OUT)"
		return 1
	fi
	LC_ALL=C sort -u "$TMP/pkg.raw" >"$PKG_NEXT" || return 1
	pkg_findings=$(wc -l <"$PKG_NEXT")
	if [ "$pkg_findings" -eq 0 ] && [ "$audit_rc" -ne 0 ]; then
		pkg_reason="pkg_admin audit exited $audit_rc without findings"
		return 1
	fi
}

# take_upgrade_lock: takes $UPGRADE_LOCK, polling for up to
# UPGRADE_LOCK_WAIT_SECS; when the upgrade job keeps it longer, the run is
# skipped (exit 0, status untouched) rather than reported UNKNOWN.
take_upgrade_lock() {
	typeset -i lock_waited=0 lock_poll=60
	[ "$UPGRADE_LOCK_WAIT_SECS" -lt "$lock_poll" ] \
		&& lock_poll=$UPGRADE_LOCK_WAIT_SECS
	[ "$lock_poll" -gt 0 ] || lock_poll=1
	while :; do
		if mkdir "$UPGRADE_LOCK" 2>/dev/null \
			|| { [ -n "$(find "$UPGRADE_LOCK" -mmin +120 2>/dev/null)" ] \
				&& rmdir "$UPGRADE_LOCK" 2>/dev/null \
				&& mkdir "$UPGRADE_LOCK" 2>/dev/null; }; then
			upgrade_lock_held=yes
			return 0
		fi
		if [ "$lock_waited" -ge "$UPGRADE_LOCK_WAIT_SECS" ]; then
			log warning "WARNING: skipped, unattended-upgrade holds $UPGRADE_LOCK (waited ${lock_waited}s); the next run retries"
			exit 0
		fi
		sleep "$lock_poll"
		lock_waited=$((lock_waited + lock_poll))
	done
}

release_upgrade_lock() {
	[ "$upgrade_lock_held" = yes ] || return 0
	rmdir "$UPGRADE_LOCK" 2>/dev/null
	upgrade_lock_held=no
}

# assess_pkgs: sets pkg_status/pkg_reason and, when assessed, $PKG_NEXT.
assess_pkgs() {
	typeset dir
	dir=$(pkg_admin config-var PKGVULNDIR 2>/dev/null)
	if [ -z "$dir" ]; then
		pkg_reason="cannot query PKGVULNDIR from pkg_admin"
		return 0
	fi
	vulns_file=$dir/pkg-vulnerabilities
	refresh_vulns
	check_vulns_list || return 0
	take_upgrade_lock
	pkg_info >"$TMP/pkgs" 2>/dev/null
	installed_pkgs=$(wc -l <"$TMP/pkgs")
	if [ "$installed_pkgs" -eq 0 ]; then
		release_upgrade_lock
		pkg_reason="pkg_info lists no installed packages"
		return 0
	fi
	if ! run_pkg_audit; then
		release_upgrade_lock
		return 0
	fi
	release_upgrade_lock
	pkg_status=OK
	pkg_reason="no advisory for $installed_pkgs packages (list $vulns_revision of $vulns_date)"
	if [ "$pkg_findings" -gt 0 ]; then
		pkg_status=VULNERABLE
		pkg_reason="$pkg_findings advisories for installed packages (list $vulns_revision of $vulns_date; details $PKG_AUDIT_OUT)"
	fi
}

# --- base: NetBSD release and security advisories ----------------------

# Evaluates one NetBSD-SA advisory for release $rel (series $major, built
# $built as YYYY-MM-DD) and prints A (affects it), N (does not) or U
# (cannot tell). First match wins:
#   1. a "Version:" line for the release itself ("NetBSD 11.0:"):
#      affected/vulnerable (also "partially affected" and "affected prior
#      to <date>": a release is not patched in place) -> A;
#      unaffected/not affected/N/A -> N; anything else -> U;
#   2. the "Fixed:" line "NetBSD-11 branch:": a date (ISO, or the older
#      "March 9, 2021" style) after the kernel build date -> A, on or
#      before it -> N; N/A or not affected -> N; anything else -> U;
#   3. a "Version:" line for the series ("NetBSD 11.*:" / "NetBSD 11:"),
#      judged like 1 (only reached when no branch fix date exists);
#   4. the series named anywhere else in those blocks (e.g. only
#      "NetBSD 11.0_RC1"), or no Version block at all -> U;
#   5. otherwise the advisory does not concern the series -> N.
# shellcheck disable=SC2016 # awk fields, not shell expansions
readonly ADVISORY_AWK='
# isodate(s): YYYY-MM-DD from "2024-07-01..." or "[Thu, ]March 9[th], 2021";
# "" when s holds no complete date.
function isodate(s,    n, t, i, m, d, y) {
	if (s ~ /^[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]/) return substr(s, 1, 10)
	n = split(s, t, /[ ,]+/); m = d = y = ""
	for (i = 1; i <= n; i++) {
		if (t[i] ~ /^[0-9][0-9][0-9][0-9]$/) y = t[i]
		else if (t[i] ~ /^[0-9][0-9]?(st|nd|rd|th)?$/) d = t[i] + 0
		else if (length(t[i]) >= 3 && index("janfebmaraprmayjunjulaugsepoctnovdec", substr(t[i], 1, 3)) % 3 == 1)
			m = (index("janfebmaraprmayjunjulaugsepoctnovdec", substr(t[i], 1, 3)) + 2) / 3
	}
	if (m == "" || d == "" || y == "") return ""
	return sprintf("%s-%02d-%02d", y, m, d)
}
function verdict(w) {
	if (w ~ /^(unaffected|not affected|not vulnerable|n\/a)/) return "N"
	if (w ~ /^(partially )?(affected|vulnerable)/) return "A"
	return "U"
}
BEGIN { blk = ""; sawv = 0; vr = ""; fr = ""; sr = ""; men = 0 }
{ sub(/\r$/, "") }
/^Version:/ { blk = "v"; sawv = 1; sub(/^Version:[ \t]*/, "") }
/^Fixed:/ { blk = "f"; sub(/^Fixed:[ \t]*/, "") }
/^[ \t]*$/ { blk = ""; next }
blk == "" { next }
{
	line = $0; sub(/^[ \t]+/, "", line)
	if (line ~ ("^NetBSD[ -]" major "([^0-9]|$)")) men = 1
	w = line; sub(/^[^:]*:[ \t]*/, "", w); w = tolower(w)
	if (blk == "v" && index(line, "NetBSD " rel ":") == 1) vr = verdict(w)
	if (blk == "v" && (index(line, "NetBSD " major ".*:") == 1 \
	    || index(line, "NetBSD " major ":") == 1)) sr = verdict(w)
	if (blk == "f" && index(line, "NetBSD-" major " branch:") == 1) {
		d = isodate(w)
		if (d != "") fr = (d > built) ? "A" : "N"
		else if (verdict(w) == "N") fr = "N"
		else fr = "U"
	}
}
END {
	if (!sawv) r = "U"
	else if (vr != "") r = vr
	else if (fr != "") r = fr
	else if (sr != "") r = sr
	else if (men) r = "U"
	else r = "N"
	print r
}
'

# detect_release: sets release/major/build_date from the running kernel;
# fails with base_reason unless it is a formal release (X.Y) with a
# parsable build date.
detect_release() {
	release=$(uname -r)
	case $release in
	[0-9]*.[0-9]*) ;;
	*) release="" ;;
	esac
	case $release in
	*[!0-9.]* | *.*.* | '')
		base_reason="kernel $(uname -r) is not a formal release: advisories cannot be mapped"
		return 1
		;;
	esac
	major=${release%%.*}
	build_date=$(uname -v | awk '{
		for (i = 1; i <= NF - 4; i++) {
			m = index("JanFebMarAprMayJunJulAugSepOctNovDec", $i)
			if (length($i) == 3 && m && (m - 1) % 3 == 0 \
			    && $(i + 4) ~ /^[0-9][0-9][0-9][0-9]$/) {
				printf "%s-%02d-%02d\n", $(i + 4), (m + 2) / 3, $(i + 1)
				exit
			}
		}
	}')
	[ -n "$build_date" ] && return 0
	build_date=unknown
	base_reason="cannot parse the kernel build date from uname -v"
	return 1
}

# page_complete <file>: the last line with anything but blanks, tabs or
# carriage returns ends in </html> (trailing blanks/CR allowed).
page_complete() {
	awk '{ sub(/[ \t\r]+$/, "") } /[^ \t]/ { l = $0 }
		END { exit l !~ /<\/html>$/ }' "$1"
}

# refresh_page <url> <cache> <marker>: refreshes a page whose download
# counts only when it contains <marker> and ends with </html> followed by
# nothing but whitespace (a truncated page never replaces the cache). The
# cdn.NetBSD.org index ends in CRLF lines plus an empty "\r\n" line, so
# carriage returns count as whitespace. Leaves the result in $fetched
# (corrupt for a failed check); <source> contact is recorded by the caller.
refresh_page() {
	typeset page_url="$1" page_cache="$2" page_marker="$3"
	typeset page_part="$TMP/page"
	fetch_url "$page_url" "$page_cache" "$page_part"
	case $fetched in
	failed) log warning "WARNING: download failed ($page_url)" ;;
	updated)
		if grep -q "$page_marker" "$page_part" \
			&& page_complete "$page_part"; then
			mv "$page_part" "$page_cache" || fetched=failed
		else
			fetched=corrupt
			log warning "WARNING: $page_url returned an incomplete page, kept the previous copy"
		fi
		;;
	esac
}

# supported_section: the "Supported Releases" part of the releases page.
supported_section() {
	awk '/Supported Releases/ { p = 1 } p && /class="sect1"/ { exit } p' \
		"$RELEASES_CACHE"
}

# matches <regex> <file>: every match of <regex> in <file>, one per line.
# awk -v processes backslash escapes, so regexes use [.] rather than \.
matches() {
	awk -v re="$1" '{
		while (match($0, re)) {
			print substr($0, RSTART, RLENGTH)
			$0 = substr($0, RSTART + RLENGTH)
		}
	}' "$2"
}

# assess_releases: appends "base eol" / "base release" findings to
# $BASE_NEXT; fails with base_reason when the page cannot be parsed.
assess_releases() {
	typeset newest
	supported_section >"$TMP/supported"
	supported_series=$(matches 'formal-[0-9]+/"' "$TMP/supported" \
		| tr -dc '0-9\n' | sort -nr -u | tr '\n' ' ' | sed 's/ $//')
	if [ -z "$supported_series" ]; then
		base_reason="cannot parse the supported releases from $RELEASES_URL"
		return 1
	fi
	case " $supported_series " in
	*" $major "*) ;;
	*)
		series_newest=none
		printf 'base eol NetBSD-%s.x\n' "$major" >>"$BASE_NEXT"
		return 0
		;;
	esac
	newest=$(matches "NetBSD-${major}[.][0-9]+[.]html" "$TMP/supported" \
		| sed 's/^NetBSD-//;s/\.html$//' | sort -t. -k2,2n | tail -n 1)
	if [ -z "$newest" ]; then
		base_reason="no NetBSD $major.x release listed as supported"
		return 1
	fi
	series_newest=$newest
	[ "${newest#*.}" -gt "${release#*.}" ] \
		&& printf 'base release NetBSD-%s\n' "$newest" >>"$BASE_NEXT"
	return 0
}

# advisories_in_index: advisory file names in the index, sorted.
advisories_in_index() {
	matches 'NetBSD-SA[0-9][0-9][0-9][0-9]-[0-9]+[.]txt[.]asc' "$ADV_INDEX" \
		| LC_ALL=C sort -u
}

# scope_advisories: writes the in-scope names (year >= build year minus
# ADVISORY_LOOKBACK_YEARS) to $TMP/scope; fails with base_reason when the
# index is below the floor or lost an in-scope advisory seen before.
scope_advisories() {
	typeset -i first="$((${build_date%%-*} - ADVISORY_LOOKBACK_YEARS))"
	typeset f lost=""
	advisories_in_index >"$TMP/index"
	advisories_listed=$(wc -l <"$TMP/index")
	if [ "$advisories_listed" -lt "$MIN_ADVISORIES" ]; then
		base_reason="advisory index lists $advisories_listed advisories (< $MIN_ADVISORIES): truncated"
		return 1
	fi
	awk -v y="$first" 'substr($0, 10, 4) + 0 >= y' "$TMP/index" \
		>"$TMP/scope"
	advisories_in_scope=$(wc -l <"$TMP/scope")
	for f in "$ADV_DIR"/NetBSD-SA*.txt.asc; do
		[ -f "$f" ] || continue
		grep -qx "${f##*/}" "$TMP/index" || lost="$lost ${f##*/}"
	done
	[ -z "$lost" ] && return 0
	base_reason="advisory index no longer lists${lost} (delete the cached copy to accept)"
	return 1
}

# fetch_advisories: refreshes every in-scope advisory (conditional GET).
# A failed refresh (download, content check or move into the cache) of a
# cached advisory is a warning; one never fetched is left missing and
# later counts as unassessable.
fetch_advisories() {
	typeset adv_name adv_failed=""
	while IFS= read -r adv_name; do
		fetch_url "$ADVISORY_URL$adv_name" "$ADV_DIR/$adv_name" "$TMP/adv"
		case $fetched in
		updated)
			if grep -q 'NetBSD Security Advisory' "$TMP/adv"; then
				# A failed move keeps the previous copy, if any; one
				# never cached then counts as unassessable.
				mv "$TMP/adv" "$ADV_DIR/$adv_name" \
					|| adv_failed="$adv_failed $adv_name"
			else
				adv_failed="$adv_failed $adv_name"
			fi
			;;
		failed) adv_failed="$adv_failed $adv_name" ;;
		esac
	done <"$TMP/scope"
	[ -z "$adv_failed" ] \
		|| log warning "WARNING: cannot refresh advisories:$adv_failed"
}

# assess_advisories: appends "base advisory <id>" findings to $BASE_NEXT;
# fails with base_reason when any in-scope advisory cannot be assessed.
assess_advisories() {
	typeset name verdict unknown=""
	scope_advisories || return 1
	fetch_advisories
	while IFS= read -r name; do
		verdict=U
		[ -f "$ADV_DIR/$name" ] && verdict=$(awk -v rel="$release" \
			-v major="$major" -v built="$build_date" "$ADVISORY_AWK" \
			"$ADV_DIR/$name")
		case $verdict in
		A) printf 'base advisory %s\n' "${name%.txt.asc}" >>"$BASE_NEXT" ;;
		N) ;;
		*) unknown="$unknown ${name%.txt.asc}" ;;
		esac
	done <"$TMP/scope"
	[ -z "$unknown" ] && return 0
	base_reason="cannot assess advisories:$unknown"
	return 1
}

# assess_base: sets base_status/base_reason and, when assessed, $BASE_NEXT.
assess_base() {
	: >"$BASE_NEXT"
	detect_release || return 0
	refresh_page "$RELEASES_URL" "$RELEASES_CACHE" 'Supported Releases'
	fetch_releases=$fetched
	case $fetched in updated | unchanged) record_contact releases ;; esac
	refresh_page "$ADVISORY_URL" "$ADV_INDEX" 'NetBSD-SA'
	fetch_advisories=$fetched
	case $fetched in updated | unchanged) record_contact advisories ;; esac
	if ! fresh_contact releases "releases page" \
		|| ! fresh_contact advisories "advisory index"; then
		base_reason=$fresh_reason
		return 0
	fi
	assess_releases || return 0
	assess_advisories || return 0
	LC_ALL=C sort -u "$BASE_NEXT" >"$TMP/base" && mv "$TMP/base" "$BASE_NEXT"
	base_findings=$(wc -l <"$BASE_NEXT")
	base_status=OK
	base_reason="NetBSD $release (built $build_date) supported, newest $series_newest, no advisory among $advisories_in_scope since $((${build_date%%-*} - ADVISORY_LOOKBACK_YEARS))"
	if [ "$base_findings" -gt 0 ]; then
		base_status=VULNERABLE
		base_reason="$base_findings base findings for NetBSD $release (built $build_date): $(awk '{ print $3 }' "$BASE_NEXT" | tr '\n' ' ' | sed 's/ $//')"
	fi
}

# --- baseline, status record and report --------------------------------

# assessed_components: "pkg|base" regex of the components with a result.
assessed_components() {
	typeset re=""
	[ "$pkg_status" != UNKNOWN ] && re=pkg
	[ "$base_status" != UNKNOWN ] && re=${re:+$re|}base
	printf '%s\n' "$re"
}

# A usable baseline is C-sorted (what comm needs) and holds only finding
# lines. An invalid one (hand-edited, corrupt) is treated as empty with a
# warning instead of failing every run.
baseline_valid() {
	LC_ALL=C sort -c "$1" 2>/dev/null \
		&& ! grep -qvE '^(pkg|base) [^ ]+ [^ ]+$' "$1"
}

# compute_new: for the assessed components only (both lists are emptied
# when nothing was assessed), $NEW_FILE = findings not in the baseline,
# $RESOLVED_FILE = baseline findings gone, and
# $FINDINGS_NEXT = the next baseline (unassessed components keep their old
# lines). Returns 1 when the new list cannot be written, 2 when only the
# resolved list or the next baseline cannot (the new findings still alert).
compute_new() {
	typeset re base="$FINDINGS"
	re=$(assessed_components)
	if [ -z "$re" ]; then
		# Nothing assessed: this run found nothing new or resolved, so the
		# batch files must not keep an older run's lists (new_findings=0).
		: >"$NEW_FILE.tmp" && mv "$NEW_FILE.tmp" "$NEW_FILE" || return 1
		: >"$RESOLVED_FILE.tmp" && mv "$RESOLVED_FILE.tmp" "$RESOLVED_FILE" \
			|| return 2
		return 0
	fi
	if [ ! -f "$base" ]; then
		base=/dev/null
	elif ! baseline_valid "$base"; then
		log warning "WARNING: baseline $FINDINGS is unsorted or malformed, treating it as empty"
		base=/dev/null
	fi
	grep -E "^($re) " "$base" >"$TMP/old"
	grep -vE "^($re) " "$base" >"$TMP/keep"
	[ "$pkg_status" != UNKNOWN ] && cat "$PKG_NEXT" >>"$TMP/next.raw"
	[ "$base_status" != UNKNOWN ] && cat "$BASE_NEXT" >>"$TMP/next.raw"
	LC_ALL=C sort -u "$TMP/next.raw" >"$TMP/next" || return 1
	LC_ALL=C comm -13 "$TMP/old" "$TMP/next" >"$NEW_FILE.tmp" \
		&& mv "$NEW_FILE.tmp" "$NEW_FILE" || return 1
	new_count=$(wc -l <"$NEW_FILE")
	LC_ALL=C comm -23 "$TMP/old" "$TMP/next" >"$RESOLVED_FILE.tmp" \
		&& mv "$RESOLVED_FILE.tmp" "$RESOLVED_FILE" || return 2
	resolved_count=$(wc -l <"$RESOLVED_FILE")
	cat "$TMP/keep" "$TMP/next" | LC_ALL=C sort -u >"$FINDINGS_NEXT"
	[ -s "$FINDINGS_NEXT" ] || [ ! -s "$TMP/next" ] || return 2
}

# Rebuilds $HISTORY_FILE atomically: entries of the last HISTORY_DAYS plus
# this run's new findings dated today, one line per finding keeping the
# OLDEST date, so a finding re-reported after a failed commit keeps the
# date of the run that first alerted it.
update_history() {
	typeset today cutoff
	today=$(date -u +%F)
	cutoff=$(date -u -d "-$HISTORY_DAYS days" +%F) || return 1
	{
		[ -f "$HISTORY_FILE" ] && awk -v c="$cutoff" '$1 >= c' "$HISTORY_FILE"
		awk -v d="$today" '{ print d, $0 }' "$NEW_FILE"
	} >"$TMP/history" || return 1
	awk '{ k = $0; sub(/^[^ ]+ /, "", k) } !seen[k]++' "$TMP/history" \
		>"$HISTORY_FILE.tmp" && mv "$HISTORY_FILE.tmp" "$HISTORY_FILE"
}

# After a run that assessed at least one component: records the new
# findings in the history FIRST, then adopts $FINDINGS_NEXT as the
# baseline. A history failure leaves the baseline untouched, so the
# findings are reported again next run; commit_error names the file.
commit_baseline() {
	[ -n "$(assessed_components)" ] || return 0
	[ -f "$FINDINGS_NEXT" ] || return 0
	if ! update_history; then
		rm -f "$HISTORY_FILE.tmp"
		commit_error="cannot update the new-finding history $HISTORY_FILE, baseline left unchanged"
		return 1
	fi
	if ! mv "$FINDINGS_NEXT" "$FINDINGS"; then
		commit_error="cannot update the baseline $FINDINGS"
		return 1
	fi
}

# combine_status: overall status is the worse of the two components.
combine_status() {
	case "$pkg_status $base_status" in
	*UNKNOWN*) status=UNKNOWN ;;
	*VULNERABLE*) status=VULNERABLE ;;
	*) status=OK ;;
	esac
	reason="pkgs $pkg_status: $pkg_reason; base $base_status: $base_reason"
}

# key=value status record for operators and later monitoring checks,
# written via a temp file so a failed write keeps the old record.
write_status() {
	cat >"$STATUS_FILE.tmp" <<EOF && mv "$STATUS_FILE.tmp" "$STATUS_FILE"
checked_at=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
clock_synced=$clock_synced
status=$status
previous_status=$previous_status
reason=$reason
pkg_status=$pkg_status
pkg_reason=$pkg_reason
base_status=$base_status
base_reason=$base_reason
installed_packages=$installed_pkgs
pkg_vulnerabilities=$vulns_file
pkg_vulnerabilities_revision=$vulns_revision
pkg_vulnerabilities_date=$vulns_date
fetch_pkg_vulnerabilities=$fetch_vulns
release=$release
kernel_build_date=$build_date
supported_series=$supported_series
series_newest=$series_newest
fetch_releases=$fetch_releases
fetch_advisory_index=$fetch_advisories
advisories_listed=$advisories_listed
advisories_in_scope=$advisories_in_scope
pkg_findings=$pkg_findings
base_findings=$base_findings
new_findings=$new_count
resolved=$resolved_count
findings_list=$FINDINGS
new_list=$NEW_FILE
new_history=$HISTORY_FILE
resolved_list=$RESOLVED_FILE
pkg_audit_output=$PKG_AUDIT_OUT
EOF
}

# first_ids <file>: the first IDS_IN_LOG lines of <file> joined by ", ".
first_ids() {
	head -n "$IDS_IN_LOG" "$1" | awk '{ $1 = $1 } 1' ORS=', ' \
		| sed 's/, $//'
}

# Logs the result at the priorities described in the header and exits.
report() {
	typeset summary
	summary="$status: $reason"
	# Batches first: a run whose commit then failed (UNKNOWN) still alerts
	# on what it found.
	[ "$new_count" -gt 0 ] && log warning \
		"NEW: $new_count finding(s): $(first_ids "$NEW_FILE") (list: $NEW_FILE)"
	[ "$resolved_count" -gt 0 ] && log notice \
		"RESOLVED: $resolved_count finding(s) gone: $(first_ids "$RESOLVED_FILE") (list: $RESOLVED_FILE)"
	[ "$status" = UNKNOWN ] && { log err "$summary"; exit 3; }
	[ "$status" != "$previous_status" ] \
		&& log warning "status changed: $previous_status -> $status"
	case $status in
	OK) log info "$summary" ;;
	*) log notice "$summary; list: $FINDINGS" ;;
	esac
	exit 0
}

# clock_in_sync: ntpd reports a synchronised clock (the first `ntpq -c rv`
# line carries sync_<source> other than sync_unspec, and no leap_alarm).
clock_in_sync() {
	typeset ntp_rv
	ntp_rv=$(ntpq -c rv 2>/dev/null | head -n 1)
	case $ntp_rv in
	*leap_alarm* | *sync_unspec*) return 1 ;;
	*sync_*) return 0 ;;
	esac
	return 1
}

# wait_for_clock: polls clock_in_sync every 30 s for CLOCK_WAIT_SECS.
wait_for_clock() {
	typeset -i clock_waited=0
	while ! clock_in_sync; do
		if [ "$clock_waited" -ge "$CLOCK_WAIT_SECS" ]; then
			clock_synced=no
			return 1
		fi
		sleep 30
		clock_waited=$((clock_waited + 30))
	done
	clock_synced=yes
}

# run_audit: both components (only with a synchronised clock), the
# combined status and the deltas.
run_audit() {
	typeset -i delta_rc=0
	: >"$PKG_NEXT"
	: >"$BASE_NEXT"
	if wait_for_clock; then
		assess_pkgs
		assess_base
	else
		pkg_reason="clock not NTP-synchronised (waited ${CLOCK_WAIT_SECS}s): ages cannot be judged"
		base_reason=$pkg_reason
	fi
	combine_status
	compute_new || delta_rc=$?
	case $delta_rc in
	0) return 0 ;;
	1) reason="cannot write the new-finding list in $STATE_DIR ($reason)" ;;
	*) reason="cannot write the resolved list or next baseline in $STATE_DIR ($reason)" ;;
	esac
	status=UNKNOWN
	rm -f "$FINDINGS_NEXT"
}

main() {
	if ! mkdir -p "$STATE_DIR" "$ADV_DIR"; then
		log err "UNKNOWN: cannot create state directory $STATE_DIR"
		exit 3
	fi
	if ! acquire_lock; then
		log info "skipped, another run holds $LOCK"
		exit 0
	fi
	trap 'release_upgrade_lock; rm -rf "$TMP" "$FINDINGS_NEXT" "$PKG_NEXT" "$BASE_NEXT"; rmdir "$LOCK" 2>/dev/null' EXIT
	rm -rf "$TMP"
	if ! mkdir "$TMP"; then
		log err "UNKNOWN: cannot create $TMP"
		exit 3
	fi
	[ -f "$STATUS_FILE" ] \
		&& previous_status=$(sed -n 's/^status=//p' "$STATUS_FILE")
	[ -n "$previous_status" ] || previous_status=none
	run_audit
	if ! write_status; then
		rm -f "$STATUS_FILE.tmp"
		log err "UNKNOWN: cannot write $STATUS_FILE (result was $status: $reason)"
		exit 3
	fi
	# Status before baseline: a failed status write does not consume the new
	# findings; a failed commit rewrites the record as UNKNOWN, naming the
	# file, so it matches the exit code.
	if ! commit_baseline; then
		reason="$commit_error (assessed $status: $reason)"
		status=UNKNOWN
		write_status || rm -f "$STATUS_FILE.tmp"
	fi
	report
}

main "$@"
