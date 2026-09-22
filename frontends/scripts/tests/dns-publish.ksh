#!/bin/ksh
# Exercise how dns-publish.ksh (task j82) makes the running NSD pick up a
# commit, without an OpenBSD host. The script's fixed /var/nsd paths are
# rewritten into a scratch tree; hostname, rcctl, nsd-control, sleep, the NSD
# checkers, install(1) and gonf's dns-zone-* subcommands are faked. The fake
# rcctl keeps NSD's running state in a file and refuses to start NSD while
# the live nsd.conf contains BROKEN (or while FAIL_START is set). Requires
# a ksh, OpenBSD's or ksh93 (both scripts use print; bash cannot run them);
# the harness itself avoids ksh93-only syntax.
#
# Covers: first publication and a key or nsd.conf change restart NSD; a
# zone-only change reloads it; an unchanged run does nothing; a failed
# install rolls back and restarts; a restart that leaves NSD stopped rolls
# back and starts NSD on the restored set; a rollback that cannot start NSD
# keeps the journal and refuses later publications until NSD starts; a
# stopped NSD is left alone; a finished run releases the publication lock;
# a live publisher's lock makes a Gonf (role-less) run wait and fail, a
# failover (-r) run exit 0, and is never stolen.

set -eu

script_dir=$(cd "$(dirname "$0")/.." && pwd)
work=$(mktemp -d "${TMPDIR:-/tmp}/dns-publish-test.XXXXXX")
trap 'rm -rf "$work"' EXIT

fake=$work/bin
root=$work/root
log=$work/actions.log
state=$work/nsd.state
lock=$root/gonf-publisher/lock
journal=$root/gonf-publisher/journal
mkdir -p "$fake" "$root/etc/gonf-publisher/zones"
print running >"$state"

sed \
    -e "s|/var/nsd|$root|g" \
    -e "s|/usr/local/bin/gonf|$fake/gonf|" \
    -e "s|^readonly LOCK_WAIT_SECONDS=.*|readonly LOCK_WAIT_SECONDS=3|" \
    "$script_dir/dns-publish.ksh" >"$work/dns-publish.ksh"

cat >"$fake/hostname" <<'EOF'
#!/bin/sh
echo blowfish
EOF

cat >"$fake/sleep" <<'EOF'
#!/bin/sh
exit 0
EOF

cat >"$fake/rcctl" <<EOF
#!/bin/sh
start() {
    if [ -n "\${FAIL_START:-}" ] || grep -q BROKEN "$root/etc/nsd.conf" 2>/dev/null; then
        echo stopped >"$state"
        return 1
    fi
    echo running >"$state"
}
case \$1 in
check) [ "\$(cat "$state")" = running ] ;;
start) echo "rcctl \$*" >>"$log"; start ;;
restart) echo "rcctl \$*" >>"$log"; echo stopped >"$state"; start ;;
*) echo "rcctl \$*" >>"$log" ;;
esac
EOF

cat >"$fake/nsd-control" <<EOF
#!/bin/sh
echo "nsd-control \$*" >>"$log"
EOF

for checker in nsd-checkconf nsd-checkzone; do
    printf '#!/bin/sh\nexit 0\n' >"$fake/$checker"
done

# install -m mode -o owner -g group source destination; ownership is ignored
# because the test does not run as root. FAIL_INSTALL makes the key install
# fail, so the commit has to roll back.
cat >"$fake/install" <<'EOF'
#!/bin/sh
while getopts "m:o:g:" option; do :; done
shift $((OPTIND - 1))
case $1 in
*/key.conf) [ -z "${FAIL_INSTALL:-}" ] || exit 1 ;;
esac
cp "$1" "$2"
EOF

# Zones carry their serial on a "serial N" line: dns-zone-serial prints it
# and dns-zone-equivalent compares the files without it, as the real
# subcommands ignore the SOA serial.
cat >"$fake/gonf" <<'EOF'
#!/bin/sh
case $1 in
dns-zone-serial) awk '$1 == "serial" { print $2 }' "$3" ;;
dns-zone-equivalent)
    grep -v '^serial ' "$3" >"$3.cmp"
    grep -v '^serial ' "$4" >"$4.cmp"
    cmp -s "$3.cmp" "$4.cmp"; status=$?
    rm -f "$3.cmp" "$4.cmp"
    exit "$status"
    ;;
*) exit 2 ;;
esac
EOF
chmod +x "$fake"/*

inputs=$root/etc/gonf-publisher
cat >"$inputs/publisher.conf" <<'EOF'
DEFAULT_ROLE="fishfinger"
MASTER_NAME="fishfinger"
MASTER_IPV4="192.0.2.1"
MASTER_IPV6="2001:db8::1"
STANDBY_NAME="blowfish"
STANDBY_IPV4="192.0.2.2"
STANDBY_IPV6="2001:db8::2"
PUBLISHER_FQDN="blowfish.example.org"
ZONES="example.org"
REMOVED_ZONES=""
EOF
printf 'key:\n\tname: k1\n' >"$inputs/key.conf"
printf 'include: "%s/etc/key.conf"\n' "$root" >"$inputs/nsd.conf"
printf 'serial @SERIAL@\nwww A @MASTER_IPV4@\n' >"$inputs/zones/example.org.zone.tpl"

failures=0
status=0

# check name command...: record a failure unless the command succeeds.
check() {
    name=$1
    shift
    if "$@"; then
        print "ok   $name"
    else
        print "FAIL $name"
        failures=$((failures + 1))
    fi
}

# run_case name expected-status expected-actions [VAR=value...] [-- args...]:
# run the publisher with the VAR=value assignments exported and args passed
# to it, and compare its exit status and the NSD actions it logged (one per
# line, "" for none). Positional parameters only, so this also parses under
# OpenBSD's ksh (no ksh93 arrays).
run_case() {
    name=$1 want_status=$2 expected=$3
    shift 3
    : >"$log"
    if (
        while [ $# -gt 0 ] && [ "$1" != -- ]; do
            export "${1?}"
            shift
        done
        [ $# -eq 0 ] || shift
        PATH="$fake:$PATH" exec ksh "$work/dns-publish.ksh" "$@"
    ) >>"$work/output" 2>&1; then
        status=0
    else
        status=$?
    fi
    actual=$(cat "$log")
    if [ "$actual" = "$expected" ] && [ "$status" -eq "$want_status" ]; then
        print "ok   $name"
    else
        print "FAIL $name: expected $want_status '$expected', got $status '$actual'"
        failures=$((failures + 1))
    fi
}

running() { [ "$(cat "$state")" = running ]; }
stopped() { ! running; }

run_case "first publication restarts" 0 "rcctl restart nsd"
run_case "unchanged inputs do nothing" 0 ""
printf 'serial @SERIAL@\nwww A @STANDBY_IPV4@\n' >"$inputs/zones/example.org.zone.tpl"
run_case "zone-only change reloads" 0 "nsd-control reload"
printf '\nzone:\n\tname: "example.org"\n' >>"$inputs/nsd.conf"
run_case "nsd.conf change restarts" 0 "rcctl restart nsd"
printf 'key:\n\tname: k2\n' >"$inputs/key.conf"
run_case "key change restarts" 0 "rcctl restart nsd"
printf 'key:\n\tname: k3\n' >"$inputs/key.conf"
run_case "failed install rolls back with a restart" 1 "rcctl restart nsd" FAIL_INSTALL=1
check "rollback restored the previous key" grep -q 'name: k2' "$root/etc/key.conf"
check "a complete rollback drops its journal" test ! -e "$journal"
printf 'key:\n\tname: k2\n' >"$inputs/key.conf"

# A restart that stops NSD and cannot start it on the new configuration.
cp "$inputs/nsd.conf" "$work/good-nsd.conf"
print '# BROKEN' >>"$inputs/nsd.conf"
run_case "a failed restart rolls back and starts NSD" 1 "rcctl restart nsd
rcctl start nsd"
check "NSD runs on the restored configuration" running
check "the broken configuration is not live" test "$(grep -c BROKEN "$root/etc/nsd.conf")" -eq 0

# The rollback cannot start NSD either: the journal must survive, and later
# runs must refuse to publish until NSD starts again.
run_case "a rollback that cannot start NSD fails" 1 "rcctl restart nsd
rcctl start nsd" FAIL_START=1
check "NSD is still stopped" stopped
check "the incomplete journal is kept" test -f "$journal/incomplete"
run_case "a later run retries the rollback and refuses" 1 "rcctl start nsd" FAIL_START=1
check "the journal is still kept" test -f "$journal/incomplete"
cp "$work/good-nsd.conf" "$inputs/nsd.conf"
run_case "once NSD starts the rollback completes" 0 "rcctl start nsd"
check "NSD runs again" running
check "the recovered journal is gone" test ! -e "$journal"

print stopped >"$state"
printf 'key:\n\tname: k4\n' >"$inputs/key.conf"
run_case "stopped NSD is left alone" 0 ""
check "stopped NSD stays stopped" stopped
print running >"$state"
run_case "published key is now current" 0 ""
check "a finished run releases its lock" test ! -e "$lock"

# A lock owned by a live process (this test's shell) must stop a publication
# that has work to do, and must survive it.
mkdir -m 700 "$lock"
{
    print "pid=$$"
    print "start=$(ps -o lstart= -p $$ | awk '{ $1 = $1; print }')"
    print "token=0123456789abcdef"
} >"$lock/owner"
printf 'serial @SERIAL@\nwww A @MASTER_IPV4@\n' >"$inputs/zones/example.org.zone.tpl"
run_case "a Gonf run waits for a live lock, then fails" 75 ""
run_case "a failover run skips a live lock" 0 "" -- -r fishfinger
check "the live publisher's lock is kept" test -f "$lock/owner"
rm -rf "$lock"
run_case "the change is published once the lock is gone" 0 "nsd-control reload"

# The publisher's own messages are shown only on failure.
if [ "$failures" -ne 0 ]; then
    cat "$work/output"
    print "$failures failure(s)"
    exit 1
fi
print "all dns-publish cases passed"
