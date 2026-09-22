#!/bin/ksh
#
# rocky-kernel-audit — daily vulnerability audit of the Raspberry Pi kernel on
# the Rocky pi2/pi3 hosts (task 752).
#
# Why this exists: pi2/pi3 boot the SIG AltArch package raspberrypi2-kernel4
# (altarch-rockyrpi repo), not the Rocky "kernel" package. Rocky publishes no
# updateinfo/errata for it, so an empty `dnf updateinfo --security` says
# nothing about the kernel's safety. This script maps the running kernel's
# upstream base version to the OSV "Linux" ecosystem export — the Linux kernel
# CNA's CVE records (cvelistV5) converted by OSV.dev, with per-stable-branch
# introduced/fixed version ranges — and records a status:
#
#   OK          every kernel-CNA CVE in the feed was assessed; none affects
#               the running upstream version
#   VULNERABLE  at least one CVE range covers the running version
#   UNKNOWN     coverage unavailable: unknown/non-AltArch kernel, feed never
#               fetched, feed stale, corrupt or truncated (below the floor or
#               >20% fewer records than the last assessment, until the lower
#               count has held for DROP_ACCEPT_RUNS consecutive runs), a CVE
#               record that cannot be assessed while none is known to affect,
#               or the state (status, new-CVE list/history, baseline) cannot
#               be written
#
# Alerting (the Pis have no MTA, plan §9.5/§13): only UNKNOWN — broken
# coverage — exits non-zero (3) and so fails the systemd oneshot unit, with an
# err-priority journal line. VULNERABLE is the steady state while no fixed
# AltArch kernel exists, so it exits 0 and is reported against a baseline
# (the affected list of the previous successful assessment): CVEs newly
# affecting the kernel, and any status change, are logged at warning
# priority and listed in $NEW_FILE (plus the dated $HISTORY_FILE); an
# unchanged VULNERABLE result is a notice. Journal priorities use the
# sd-daemon "<N>" stdout prefix; every line is also appended to the shared
# /var/log/unattended-upgrade.log.
#
# Operator overrides (as root, in $STATE_DIR): delete baseline-cve-records
# to accept a lower feed record count at once (the drop guard is off until
# the next successful run records a new count); delete affected-cves to
# re-baseline (the next run reports every affected CVE as new once).
#
# Coverage caveats — the affected count is an UPPER bound for kernel-CNA CVEs
# and misses CVEs from other CNAs:
#   - OSV's ECOSYSTEM ranges start at the stable branch base (X.Y.0 or 0),
#     not the commit that introduced the bug, so CVEs introduced after 6.1.31
#     on the 6.1 branch still count as affecting it;
#   - kernel config and architecture (drivers not built for the Pi) are not
#     modelled, nor are downstream Raspberry Pi patches;
#   - CVEs assigned by other CNAs (mostly pre-2024) are not in the feed.
# OK therefore means "no known kernel-CNA CVE", and VULNERABLE means "at
# least one kernel-CNA CVE whose fix is not in this upstream version".
#
# The regular Rocky package audit/update path (unattended-upgrade-rocky) is
# untouched; this is a separate unit so a feed outage cannot block dnf.
#
# Rocky ships AT&T ksh93u+m — use typeset, not local. Note that in ksh93 a
# typeset inside a POSIX-style name() function is NOT local (only
# `function name {}` scopes it); helper variables here are therefore given
# names that never collide with the result globals. The script also runs
# under bash for the test harness (tests/rocky-kernel-audit.ksh).

PATH=/usr/bin:/bin:/usr/sbin:/sbin:/usr/local/bin:/usr/local/sbin
if [ -n "${ROCKY_KERNEL_AUDIT_TEST_PATH:-}" ]; then
	PATH="${ROCKY_KERNEL_AUDIT_TEST_PATH}:$PATH"
fi
export PATH
set -o pipefail

umask 077

readonly FEED_URL=https://osv-vulnerabilities.storage.googleapis.com/Linux/all.zip
readonly KERNEL_PKG=raspberrypi2-kernel4
readonly FETCH_TIMEOUT=900
# The kernel CNA publishes or amends records almost daily and OSV re-exports
# continuously; no record modified within this window means the feed (or our
# cached copy of it) stopped moving.
readonly MAX_FEED_AGE_HOURS=${ROCKY_KERNEL_AUDIT_TEST_MAX_AGE_HOURS:-72}
# Truncation guards. The absolute floor catches a broken first download
# (~15.8k CVE records in 2026-09); after that, a drop of more than
# MAX_RECORD_DROP_PCT against the record count of the last successful
# assessment ($BASELINE_RECORDS) counts as a truncated export too. The kernel
# CNA rejects few records, so the count only grows in normal operation. A
# legitimate shrink recovers by itself: once the lower count has been seen
# on DROP_ACCEPT_RUNS consecutive fresh runs (each within DROP_HOLD_PCT of
# the previous one, tracked in $DROP_FILE) it is accepted, logged at warning
# priority, and becomes the new baseline count. A one-off drop followed by
# a normal run resets the streak.
readonly MIN_CVE_RECORDS=${ROCKY_KERNEL_AUDIT_TEST_MIN_RECORDS:-10000}
readonly MAX_RECORD_DROP_PCT=20
readonly DROP_ACCEPT_RUNS=3
readonly DROP_HOLD_PCT=95
# Days of dated new-CVE batches kept in $HISTORY_FILE.
readonly HISTORY_DAYS=90
# How many new CVE ids the warning line names before pointing at $NEW_FILE.
readonly NEW_IDS_IN_LOG=10

LOG=${ROCKY_KERNEL_AUDIT_TEST_LOG:-/var/log/unattended-upgrade.log}
STATE_DIR=${ROCKY_KERNEL_AUDIT_TEST_STATE_DIR:-/var/lib/rocky-kernel-audit}
LOCK=$STATE_DIR/lock
FEED=$STATE_DIR/osv-linux-all.zip
STATUS_FILE=$STATE_DIR/status
# Baseline: affected CVEs of the last successful (OK/VULNERABLE) assessment.
AFFECTED_FILE=$STATE_DIR/affected-cves
AFFECTED_NEXT=$STATE_DIR/affected-cves.next
BASELINE_RECORDS=$STATE_DIR/baseline-cve-records
# "<count> <streak>" of a record-count drop still awaiting acceptance.
DROP_FILE=$STATE_DIR/drop-candidate
# new-cves: the latest run's batch (empty when nothing is new).
# new-cves.history: "<YYYY-MM-DD> <CVE>" lines for HISTORY_DAYS, so a batch
# stays visible after the next (quiet) run overwrites new-cves.
NEW_FILE=$STATE_DIR/new-cves
HISTORY_FILE=$STATE_DIR/new-cves.history
RESULTS=$STATE_DIR/results.tmp

# jq filter over the concatenated OSV records. Per record it prints
# "<A|N|U> <id> <modified>": A = a Linux/Kernel ECOSYSTEM range (or explicit
# version list) covers $kver, N = assessed and not affected, U = no
# assessable range (none present, or an event version that does not parse).
# Versions compare as 4-component numeric arrays ("6.2-rc1" → 6.2.0.0).
#
# Events are evaluated per range in version order (the OSV algorithm:
# introduced turns affected on, fixed/after last_affected off), with one
# branch-aware correction: a range can list several stable-branch fixes, e.g.
# [introduced 5.16.0, fixed 6.1.75, fixed 6.6.14]. Plain OSV evaluation would
# call 6.2–6.6.13 fixed by 6.1.75; here a fixed event only closes the range
# for its own major.minor branch, except the range's last fixed event, which
# also covers every later branch (they inherit that fix).
# shellcheck disable=SC2016 # jq variables, not shell expansions
readonly JQ_FILTER='
def vparse:
	(tostring | capture("^(?<v>[0-9]+(\\.[0-9]+)*)").v? // null)
	| if . == null then null
	  else (split(".") | map(tonumber) + [0, 0, 0, 0])[:4] end;
def range_hits($v):
	[.events[]? | to_entries[0] | {k: .key, v: (.value | vparse)}]
	| if length == 0 or any(.[]; .v == null) then null
	  else sort_by(.v) | to_entries
	  | ([.[] | select(.value.k == "fixed") | .key] | max) as $last
	  | reduce .[] as $x (false;
		$x.value as $e
		| if $e.k == "introduced" and $v >= $e.v then true
		  elif $e.k == "fixed" and $v >= $e.v
		    and ($x.key == $last or ($e.v[:2]) == ($v[:2])) then false
		  elif $e.k == "last_affected" and $v > $e.v then false
		  else . end)
	  end;
($kver | vparse) as $v
| select(has("withdrawn") | not)
| [.affected[]? | select(.package.ecosystem == "Linux"
	and .package.name == "Kernel")] as $aff
| [$aff[] | ((.versions // []) | index([$kver]) != null)] as $listed
| [$aff[] | .ranges[]? | select(.type == "ECOSYSTEM") | range_hits($v)]
	as $hits
| ([$aff[] | (.versions // []) | length] | add // 0) as $nlisted
| (if any($listed[]; .) or any($hits[]; . == true) then "A"
   elif any($hits[]; . == null)
	or (($hits | length) == 0 and $nlisted == 0) then "U"
   else "N" end) as $s
| "\($s) \(.id) \(.modified)"
'

# Result fields, filled in by the steps below and written by write_status.
status=UNKNOWN
previous_status=none
reason=""
krel=""
kver=""
installed_newest=unknown
repo_newest=unknown
fetch=skipped
feed_newest=none
typeset -i cves=0 affected=0 unassessable=0 new_count=0 baseline_cves=0
typeset -i drop_streak=0
# Set by commit_baseline when it fails: which state file could not be written.
commit_error=""

# log <priority> <message>: shared log file plus stdout with the sd-daemon
# priority prefix (3 err, 4 warning, 5 notice, 6 info) the journal parses.
log() {
	typeset prio=$1
	shift
	printf '[%s] kernel-audit: %s\n' "$(date '+%Y-%m-%dT%H:%M:%S')" "$*" \
		>>"$LOG"
	printf '<%s>%s\n' "$prio" "$*"
}

# Whole-run lock; steal one older than 2 h (a killed run). $STATE_DIR must
# exist (main checks it, so a missing directory is not mistaken for a lock).
acquire_lock() {
	mkdir "$LOCK" 2>/dev/null && return 0
	[ -n "$(find "$LOCK" -mmin +120 2>/dev/null)" ] || return 1
	rmdir "$LOCK" 2>/dev/null && mkdir "$LOCK" 2>/dev/null
}

# Sets krel/kver. Fails (with reason) unless the running kernel is a
# $KERNEL_PKG build whose upstream base version parses as X.Y.Z.
detect_kernel() {
	krel=$(uname -r)
	kver=${krel%%-*}
	if ! rpm -q "$KERNEL_PKG-$krel" >/dev/null 2>&1; then
		reason="running kernel $krel is not a $KERNEL_PKG package"
		return 1
	fi
	if ! printf '%s\n' "$kver" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$'; then
		reason="cannot derive an upstream version from kernel $krel"
		return 1
	fi
	installed_newest=$(rpm -q --qf '%{VERSION}-%{RELEASE}\n' "$KERNEL_PKG" \
		2>/dev/null | sort -V | tail -1)
	# Informational only: from the metadata cache the daily dnf run keeps
	# fresh; "unknown" when the cache is unavailable.
	repo_newest=$(dnf -q --cacheonly repoquery --latest-limit=1 \
		--qf '%{VERSION}-%{RELEASE}' "$KERNEL_PKG" 2>/dev/null | tail -1)
	[ -n "$repo_newest" ] || repo_newest=unknown
	return 0
}

# Refreshes $FEED with a conditional GET (If-Modified-Since the cached copy,
# -R keeps the server's mtime). fetch = updated | unchanged | failed |
# corrupt (skipped when no fetch ran). A failed or corrupt download keeps the
# previous copy; its freshness is then judged by decide_status.
fetch_feed() {
	typeset part
	part=$FEED.part
	rm -f "$part"
	if [ -f "$FEED" ]; then
		curl -fsS -R --max-time "$FETCH_TIMEOUT" -z "$FEED" \
			-o "$part" "$FEED_URL" 2>/dev/null
	else
		curl -fsS -R --max-time "$FETCH_TIMEOUT" -o "$part" "$FEED_URL" \
			2>/dev/null
	fi || {
		rm -f "$part"
		fetch=failed
		log 4 "WARNING: feed download failed ($FEED_URL)"
		return 0
	}
	if [ ! -s "$part" ]; then
		rm -f "$part"
		fetch=unchanged
	elif unzip -tqq "$part" >/dev/null 2>&1; then
		mv "$part" "$FEED"
		fetch=updated
	else
		rm -f "$part"
		fetch=corrupt
		log 4 "WARNING: downloaded feed is not a valid zip, kept previous copy"
	fi
	return 0
}

# Evaluates every record for $kver into $RESULTS; fails on a corrupt archive
# or a jq error (pipefail).
evaluate_feed() {
	unzip -p "$FEED" 2>/dev/null \
		| jq -r --arg kver "$kver" "$JQ_FILTER" >"$RESULTS" 2>/dev/null
}

# Counts CVE records (other ids, e.g. the legacy GSD-*, are ignored), finds
# the newest record modification time and writes the candidate affected list
# (C-sorted for comm) to $AFFECTED_NEXT; commit_baseline adopts it.
summarize_results() {
	typeset counts
	counts=$(awk '
		$2 ~ /^CVE-/ {
			n++
			if ($1 == "A") a++
			if ($1 == "U") u++
			if ($3 > newest) newest = $3
		}
		END { printf "%d %d %d %s\n", n, a, u, (newest == "" ? "none" : newest) }
	' "$RESULTS") || return 1
	read -r cves affected unassessable feed_newest <<<"$counts"
	awk '$1 == "A" && $2 ~ /^CVE-/ { print $2 }' "$RESULTS" \
		| LC_ALL=C sort >"$AFFECTED_NEXT"
}

# Age of the newest record in hours; fails when it cannot be parsed.
feed_age_hours() {
	typeset ts epoch
	ts=${feed_newest%Z}
	ts=${ts%%.*}
	epoch=$(date -u -d "${ts}Z" +%s 2>/dev/null) || return 1
	printf '%d\n' $((($(date -u +%s) - epoch) / 3600))
}

# Drop guard (see MAX_RECORD_DROP_PCT). Fails with reason while a >20% drop
# against the baseline count is unconfirmed; succeeds when there is no drop
# or the lower count has held for DROP_ACCEPT_RUNS consecutive runs. Only
# fresh, above-floor feeds reach it, so stale runs do not count.
check_record_drop() {
	typeset prev_count prev_streak
	drop_streak=1
	if [ $((cves * 100)) -ge $((baseline_cves * (100 - MAX_RECORD_DROP_PCT))) ]
	then
		drop_streak=0
		rm -f "$DROP_FILE"
		return 0
	fi
	if [ -f "$DROP_FILE" ] && read -r prev_count prev_streak <"$DROP_FILE"; then
		case $prev_count:$prev_streak in
		*[!0-9:]* | :* | *:) ;;
		*) [ $((cves * 100)) -ge $((prev_count * DROP_HOLD_PCT)) ] \
			&& drop_streak=$((prev_streak + 1)) ;;
		esac
	fi
	# Recorded even when accepting: commit_baseline removes it, so an
	# accepted run that ends UNKNOWN for another reason still counts.
	printf '%d %d\n' "$cves" "$drop_streak" >"$DROP_FILE" \
		|| log 4 "WARNING: cannot record the drop streak in $DROP_FILE"
	if [ "$drop_streak" -ge "$DROP_ACCEPT_RUNS" ]; then
		log 4 "WARNING: accepting $cves CVE records (was $baseline_cves) after $drop_streak consecutive runs"
		return 0
	fi
	reason="feed shrank from $baseline_cves to $cves CVE records"
	reason="$reason (> ${MAX_RECORD_DROP_PCT}% drop, run $drop_streak of"
	reason="$reason $DROP_ACCEPT_RUNS before it is accepted): truncated"
	return 1
}

# Sets status/reason from the counts. Staleness and truncation make the
# result UNKNOWN even when CVEs match: a stale feed must not look audited.
decide_status() {
	typeset -i age
	if [ "$cves" -lt "$MIN_CVE_RECORDS" ]; then
		reason="feed holds $cves CVE records (< $MIN_CVE_RECORDS): truncated"
		return 0
	fi
	age=$(feed_age_hours) || {
		reason="cannot parse newest record time '$feed_newest'"
		return 0
	}
	if [ "$age" -gt "$MAX_FEED_AGE_HOURS" ]; then
		reason="feed stale: newest record ${age}h old (> ${MAX_FEED_AGE_HOURS}h)"
		return 0
	fi
	check_record_drop || return 0
	if [ "$affected" -gt 0 ]; then
		status=VULNERABLE
		reason="$affected of $cves kernel CVEs affect upstream $kver (upper bound)"
	elif [ "$unassessable" -gt 0 ]; then
		reason="$unassessable of $cves CVE records have no assessable range"
	else
		status=OK
		reason="none of $cves kernel CVEs affects upstream $kver"
	fi
}

# A usable baseline is C-sorted (what comm needs) and holds only CVE ids.
baseline_valid() {
	LC_ALL=C sort -c "$1" 2>/dev/null \
		&& ! grep -qvE '^CVE-[0-9]+-[0-9]+$' "$1"
}

# After a successful assessment: $NEW_FILE = CVEs not in the previous
# baseline (all of them on the first run). An invalid baseline (hand-edited,
# corrupt) is treated as missing with a warning rather than failing every
# run: UNKNOWN runs never replace it, so it would never recover. The
# baseline itself is replaced only by commit_baseline, after the status
# record and the history are written, so a run that fails to record either
# reports the same CVEs as new next time; UNKNOWN runs never touch it
# either, so CVEs that appear during an outage are still reported once
# coverage returns.
compute_new() {
	typeset base
	base=$AFFECTED_FILE
	if [ ! -f "$base" ]; then
		base=/dev/null
	elif ! baseline_valid "$base"; then
		log 4 "WARNING: baseline $AFFECTED_FILE is unsorted or malformed, treating it as empty"
		base=/dev/null
	fi
	LC_ALL=C comm -13 "$base" "$AFFECTED_NEXT" >"$NEW_FILE.tmp" \
		&& mv "$NEW_FILE.tmp" "$NEW_FILE" || return 1
	new_count=$(wc -l <"$NEW_FILE")
}

# Rebuilds $HISTORY_FILE atomically: the entries of the last HISTORY_DAYS
# plus this run's new CVEs dated today (same-day repeats collapse). On
# failure the previous history stays as it was.
update_history() {
	typeset today cutoff
	today=$(date -u +%F)
	cutoff=$(date -u -d "-$HISTORY_DAYS days" +%F) || return 1
	{
		[ -f "$HISTORY_FILE" ] && awk -v c="$cutoff" '$1 >= c' "$HISTORY_FILE"
		awk -v d="$today" '{ print d, $0 }' "$NEW_FILE"
	} | awk '!seen[$0]++' >"$HISTORY_FILE.tmp" \
		&& mv "$HISTORY_FILE.tmp" "$HISTORY_FILE"
}

# After an OK/VULNERABLE result: records the new CVEs in the history FIRST,
# then adopts this run's affected list and record count as the baseline and
# clears a pending drop streak. A history failure therefore leaves the
# baseline untouched (the CVEs are reported again next run); commit_error
# names the file that failed.
commit_baseline() {
	case $status in
	OK | VULNERABLE) ;;
	*) return 0 ;;
	esac
	if ! update_history; then
		rm -f "$HISTORY_FILE.tmp"
		commit_error="cannot update the new-CVE history $HISTORY_FILE"
		commit_error="$commit_error, baseline left unchanged"
		return 1
	fi
	if ! mv "$AFFECTED_NEXT" "$AFFECTED_FILE"; then
		commit_error="cannot update the baseline $AFFECTED_FILE"
		return 1
	fi
	if ! { printf '%d\n' "$cves" >"$BASELINE_RECORDS.tmp" \
		&& mv "$BASELINE_RECORDS.tmp" "$BASELINE_RECORDS"; }; then
		commit_error="cannot update the baseline count $BASELINE_RECORDS"
		return 1
	fi
	rm -f "$DROP_FILE"
}

# Reads the record count of the last successful assessment (0 = none, or
# not a number: the drop guard is then off and only the floor applies).
read_baseline_records() {
	typeset n
	[ -f "$BASELINE_RECORDS" ] || return 0
	n=$(cat "$BASELINE_RECORDS" 2>/dev/null)
	case $n in
	'' | *[!0-9]*) log 4 "WARNING: ignoring malformed $BASELINE_RECORDS" ;;
	*) baseline_cves=$n ;;
	esac
}

# Runs the pipeline; any failure leaves status UNKNOWN with a reason.
run_audit() {
	detect_kernel || return 0
	fetch_feed
	if [ ! -f "$FEED" ]; then
		reason="no feed copy available (never fetched)"
		return 0
	fi
	if ! evaluate_feed; then
		reason="cannot evaluate feed (corrupt archive or jq failure)"
		return 0
	fi
	summarize_results || { reason="cannot summarize results"; return 0; }
	decide_status
	[ "$status" = UNKNOWN ] && return 0
	compute_new || {
		status=UNKNOWN
		reason="cannot write the new-CVE list in $STATE_DIR"
	}
}

# key=value status record for operators and later monitoring checks. Written
# to a temp file first: a failed write (full SD card) keeps the old record.
write_status() {
	cat >"$STATUS_FILE.tmp" <<EOF && mv "$STATUS_FILE.tmp" "$STATUS_FILE"
checked_at=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
status=$status
previous_status=$previous_status
reason=$reason
kernel_running=$krel
upstream_version=$kver
kernel_installed_newest=$installed_newest
kernel_repo_newest=$repo_newest
source=$FEED_URL
fetch=$fetch
feed_newest_record=$feed_newest
cve_records=$cves
affected=$affected
new_cves=$new_count
unassessable=$unassessable
affected_list=$AFFECTED_FILE
new_list=$NEW_FILE
new_history=$HISTORY_FILE
baseline_cve_records=$baseline_cves
drop_streak=$drop_streak
EOF
}

# Logs the result at the priority described in the header and exits.
report() {
	typeset summary ids
	summary="$status: $reason (kernel ${krel:-?}, installed newest"
	summary="$summary $installed_newest, repo newest $repo_newest, fetch $fetch)"
	[ "$status" = UNKNOWN ] && { log 3 "$summary"; exit 3; }
	if [ "$status" != "$previous_status" ]; then
		log 4 "status changed: $previous_status -> $status"
	fi
	if [ "$new_count" -gt 0 ]; then
		ids=$(head -n "$NEW_IDS_IN_LOG" "$NEW_FILE" | tr '\n' ' ')
		log 4 "NEW: $new_count CVE(s) newly affect the kernel: ${ids}(list: $NEW_FILE)"
	fi
	case $status in
	OK) log 6 "$summary" ;;
	*) log 5 "$summary; list: $AFFECTED_FILE" ;;
	esac
	exit 0
}

main() {
	if ! mkdir -p "$STATE_DIR"; then
		log 3 "UNKNOWN: cannot create state directory $STATE_DIR"
		exit 3
	fi
	if ! acquire_lock; then
		log 6 "skipped, another run holds $LOCK"
		exit 0
	fi
	trap 'rm -f "$RESULTS" "$AFFECTED_NEXT"; rmdir "$LOCK" 2>/dev/null' EXIT
	if [ -f "$STATUS_FILE" ]; then
		previous_status=$(sed -n 's/^status=//p' "$STATUS_FILE")
	fi
	[ -n "$previous_status" ] || previous_status=none
	read_baseline_records
	run_audit
	if ! write_status; then
		rm -f "$STATUS_FILE.tmp"
		log 3 "UNKNOWN: cannot write $STATUS_FILE (result was $status: $reason)"
		exit 3
	fi
	# The status is written before the baseline so a failed status write
	# does not consume the new CVEs; a failed commit (history or baseline)
	# then rewrites the record as UNKNOWN, naming the file, so it matches
	# the failed unit.
	if ! commit_baseline; then
		reason="$commit_error (assessed $status: $reason)"
		status=UNKNOWN
		write_status || rm -f "$STATUS_FILE.tmp"
	fi
	report
}

main "$@"
