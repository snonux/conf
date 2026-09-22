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
#               the running upstream version (exit 0)
#   VULNERABLE  at least one CVE range covers the running version (exit 2)
#   UNKNOWN     coverage unavailable: unknown/non-AltArch kernel, feed never
#               fetched, feed stale, corrupt or truncated, or a CVE record
#               that cannot be assessed while none is known to affect (exit 3)
#
# A non-zero exit fails the systemd oneshot unit (visible in
# `systemctl --failed`), and the summary goes to the journal at err priority
# (sd-daemon "<3>" prefix) and to the shared /var/log/unattended-upgrade.log —
# the Pis have no MTA, so journal/log/unit state is the alert surface (plan
# §9.5, frontends/docs/unattended-upgrades-pi.plan.md §13).
#
# Coverage caveat: the mapping is by upstream base version (6.1.31 for
# 6.1.31-v8.1.el9.altarch); downstream Raspberry Pi patches are not modelled,
# and CVEs assigned by other CNAs (mostly pre-2024) are not in the feed. So
# VULNERABLE is definitive, while OK means "no known kernel-CNA CVE".
#
# The regular Rocky package audit/update path (unattended-upgrade-rocky) is
# untouched; this is a separate unit so a feed outage cannot block dnf.
#
# Rocky ships AT&T ksh93u+m — use typeset, not local. The script also runs
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
# ~15.8k CVE records in 2026-09; far fewer means a truncated export.
readonly MIN_CVE_RECORDS=${ROCKY_KERNEL_AUDIT_TEST_MIN_RECORDS:-5000}

LOG=${ROCKY_KERNEL_AUDIT_TEST_LOG:-/var/log/unattended-upgrade.log}
STATE_DIR=${ROCKY_KERNEL_AUDIT_TEST_STATE_DIR:-/var/lib/rocky-kernel-audit}
LOCK=$STATE_DIR/lock
FEED=$STATE_DIR/osv-linux-all.zip
STATUS_FILE=$STATE_DIR/status
AFFECTED_FILE=$STATE_DIR/affected-cves
RESULTS=$STATE_DIR/results.tmp

# jq filter over the concatenated OSV records. Per record it prints
# "<A|N|U> <id> <modified>": A = a Linux/Kernel ECOSYSTEM range (or explicit
# version list) covers $kver, N = assessed and not affected, U = no
# assessable range. Events are evaluated per range in version order, the OSV
# algorithm: introduced turns affected on, fixed/after last_affected off.
# Versions compare as 4-component numeric arrays ("6.2-rc1" → 6.2.0.0).
# shellcheck disable=SC2016 # jq variables, not shell expansions
readonly JQ_FILTER='
def vparse:
  (tostring | capture("^(?<v>[0-9]+(\\.[0-9]+)*)").v? // null)
  | if . == null then null
    else (split(".") | map(tonumber) + [0, 0, 0, 0])[:4] end;
def range_hits($v):
  [.events[]? | to_entries[0] | {k: .key, v: (.value | vparse)}]
  | if any(.[]; .v == null) then null
    else sort_by(.v)
    | reduce .[] as $e (false;
        if $e.k == "introduced" and $v >= $e.v then true
        elif $e.k == "fixed" and $v >= $e.v then false
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
| (if any($listed[]; .) or any($hits[]; . == true) then "A"
   elif ($hits | length) == 0 or any($hits[]; . == null) then "U"
   else "N" end) as $s
| "\($s) \(.id) \(.modified)"
'

# Result fields, filled in by the steps below and written by write_status.
status=UNKNOWN
reason=""
krel=""
kver=""
installed_newest=unknown
repo_newest=unknown
fetch=failed
feed_newest=none
typeset -i cves=0 affected=0 unassessable=0

log() {
	printf '[%s] kernel-audit: %s\n' "$(date '+%Y-%m-%dT%H:%M:%S')" "$*" \
		>>"$LOG"
	printf '%s\n' "$*"
}

# Journal at err priority (systemd parses the "<3>" prefix on stdout).
log_err() {
	printf '[%s] kernel-audit: %s\n' "$(date '+%Y-%m-%dT%H:%M:%S')" "$*" \
		>>"$LOG"
	printf '<3>%s\n' "$*"
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
# corrupt. A failed or corrupt download keeps the previous copy; its
# freshness is then judged by decide_status like any other copy.
fetch_feed() {
	typeset tmp
	tmp=$FEED.part
	rm -f "$tmp"
	if [ -f "$FEED" ]; then
		curl -fsS -R --max-time "$FETCH_TIMEOUT" -z "$FEED" \
			-o "$tmp" "$FEED_URL" 2>/dev/null
	else
		curl -fsS -R --max-time "$FETCH_TIMEOUT" -o "$tmp" "$FEED_URL" \
			2>/dev/null
	fi || { rm -f "$tmp"; fetch=failed; return 0; }
	if [ ! -s "$tmp" ]; then
		rm -f "$tmp"
		fetch=unchanged
	elif unzip -tqq "$tmp" >/dev/null 2>&1; then
		mv "$tmp" "$FEED"
		fetch=updated
	else
		rm -f "$tmp"
		fetch=corrupt
		log_err "WARNING: downloaded feed is not a valid zip, kept previous copy"
	fi
	return 0
}

# Evaluates every record for $kver into $RESULTS; fails on a corrupt archive
# or a jq error (pipefail).
evaluate_feed() {
	unzip -p "$FEED" 2>/dev/null \
		| jq -r --arg kver "$kver" "$JQ_FILTER" >"$RESULTS" 2>/dev/null
}

# Counts CVE records (other ids, e.g. the legacy GSD-*, are ignored), writes
# the affected CVE list and finds the newest record modification time.
summarize_results() {
	typeset line
	line=$(awk '
		$2 ~ /^CVE-/ {
			n++
			if ($1 == "A") a++
			if ($1 == "U") u++
			if ($3 > newest) newest = $3
		}
		END { printf "%d %d %d %s\n", n, a, u, (newest == "" ? "none" : newest) }
	' "$RESULTS") || return 1
	read -r cves affected unassessable feed_newest <<<"$line"
	awk '$1 == "A" && $2 ~ /^CVE-/ { print $2 }' "$RESULTS" | sort -V \
		>"$AFFECTED_FILE.tmp" && mv "$AFFECTED_FILE.tmp" "$AFFECTED_FILE"
}

# Age of the newest record in hours; fails when it cannot be parsed.
feed_age_hours() {
	typeset ts epoch
	ts=${feed_newest%Z}
	ts=${ts%%.*}
	epoch=$(date -u -d "${ts}Z" +%s 2>/dev/null) || return 1
	printf '%d\n' $((($(date -u +%s) - epoch) / 3600))
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
	if [ "$affected" -gt 0 ]; then
		status=VULNERABLE
		reason="$affected of $cves kernel CVEs affect upstream $kver"
	elif [ "$unassessable" -gt 0 ]; then
		reason="$unassessable of $cves CVE records have no assessable range"
	else
		status=OK
		reason="none of $cves kernel CVEs affects upstream $kver"
	fi
}

# Runs the pipeline; any failure leaves status UNKNOWN with a reason.
run_audit() {
	detect_kernel || return 0
	fetch_feed
	[ "$fetch" = failed ] && log_err "WARNING: feed download failed ($FEED_URL)"
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
}

# key=value status record for operators and later monitoring checks.
write_status() {
	cat >"$STATUS_FILE.tmp" <<EOF
checked_at=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
status=$status
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
unassessable=$unassessable
affected_list=$AFFECTED_FILE
EOF
	mv "$STATUS_FILE.tmp" "$STATUS_FILE"
}

main() {
	typeset summary
	if ! mkdir -p "$STATE_DIR"; then
		log_err "UNKNOWN: cannot create state directory $STATE_DIR"
		exit 3
	fi
	if ! acquire_lock; then
		log "skipped, another run holds $LOCK"
		exit 0
	fi
	trap 'rm -f "$RESULTS"; rmdir "$LOCK" 2>/dev/null' EXIT
	run_audit
	write_status
	summary="$status: $reason (kernel ${krel:-?}, installed newest"
	summary="$summary $installed_newest, repo newest $repo_newest, fetch $fetch)"
	case $status in
	OK) log "$summary"; exit 0 ;;
	VULNERABLE) log_err "$summary; list: $AFFECTED_FILE"; exit 2 ;;
	*) log_err "$summary"; exit 3 ;;
	esac
}

main "$@"
