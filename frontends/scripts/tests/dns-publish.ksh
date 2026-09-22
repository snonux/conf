#!/bin/ksh
# Exercise how dns-publish.ksh (task j82) makes the running NSD pick up a
# commit, without an OpenBSD host. The script's fixed /var/nsd paths are
# rewritten into a scratch tree; hostname, rcctl, nsd-control, the NSD
# checkers, install(1) and gonf's dns-zone-* subcommands are faked. Requires
# ksh (the script uses print and typeset).
#
# Covers: first publication and a key or nsd.conf change restart NSD; a
# zone-only change reloads it; an unchanged run does nothing; a failed commit
# rolls back and restarts; nothing is applied while NSD is not running (that
# run also replays the rollback of the failed commit's journal); a finished
# run releases the publication lock, and a live publisher's lock blocks a
# second publisher without being stolen.

set -eu

script_dir=$(cd "$(dirname "$0")/.." && pwd)
work=$(mktemp -d "${TMPDIR:-/tmp}/dns-publish-test.XXXXXX")
trap 'rm -rf "$work"' EXIT

fake=$work/bin
root=$work/root
log=$work/actions.log
lock=$root/gonf-publisher/lock
mkdir -p "$fake" "$root/etc/gonf-publisher/zones"

sed \
    -e "s|/var/nsd|$root|g" \
    -e "s|/usr/local/bin/gonf|$fake/gonf|" \
    "$script_dir/dns-publish.ksh" >"$work/dns-publish.ksh"

cat >"$fake/hostname" <<'EOF'
#!/bin/sh
echo blowfish
EOF

cat >"$fake/rcctl" <<EOF
#!/bin/sh
case \$1 in
check) [ "\${NSD_RUNNING:-yes}" = yes ] ;;
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

# run_case name expected-actions [env assignments...]: run the publisher and
# compare the NSD actions it logged (one per line, "" for none).
run_case() {
    typeset name=$1 expected=$2 actual
    shift 2
    : >"$log"
    env PATH="$fake:$PATH" "$@" ksh "$work/dns-publish.ksh" >>"$work/output" 2>&1 || :
    actual=$(cat "$log")
    if [ "$actual" = "$expected" ]; then
        print "ok   $name"
    else
        print "FAIL $name: expected '$expected', got '$actual'"
        failures=$((failures + 1))
    fi
}

# check name command...: record a failure unless the command succeeds.
check() {
    typeset name=$1
    shift
    if "$@"; then
        print "ok   $name"
    else
        print "FAIL $name"
        failures=$((failures + 1))
    fi
}

run_case "first publication restarts" "rcctl restart nsd"
run_case "unchanged inputs do nothing" ""
printf 'serial @SERIAL@\nwww A @STANDBY_IPV4@\n' >"$inputs/zones/example.org.zone.tpl"
run_case "zone-only change reloads" "nsd-control reload"
printf '\nzone:\n\tname: "example.org"\n' >>"$inputs/nsd.conf"
run_case "nsd.conf change restarts" "rcctl restart nsd"
printf 'key:\n\tname: k2\n' >"$inputs/key.conf"
run_case "key change restarts" "rcctl restart nsd"
printf 'key:\n\tname: k3\n' >"$inputs/key.conf"
run_case "failed commit rolls back with a restart" "rcctl restart nsd" FAIL_INSTALL=1
check "rollback restored the previous key" grep -q 'name: k2' "$root/etc/key.conf"
run_case "stopped NSD is left alone" "" NSD_RUNNING=no
run_case "published key is now current" ""
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
run_case "a live publisher's lock blocks publication" ""
check "the live publisher's lock is kept" test -f "$lock/owner"
rm -rf "$lock"
run_case "the change is published once the lock is gone" "nsd-control reload"

# The publisher's own messages are shown only on failure.
if [ "$failures" -ne 0 ]; then
    cat "$work/output"
    print "$failures failure(s)"
    exit 1
fi
print "all dns-publish cases passed"
