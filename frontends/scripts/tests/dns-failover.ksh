#!/bin/ksh
# Exercise dns-failover.ksh's KISS HA decision without an OpenBSD host. The
# script's fixed /var/nsd and publisher paths are rewritten into a scratch
# tree; date (for `+%U`), timeout, ftp, sleep, logger and the publisher are
# faked. FAKE_WEEK sets the UTC week, DOWN lists "<fqdn>:<4|6>" health checks
# that fail, and the fake publisher only logs the role it was asked for.
# Requires a ksh, OpenBSD's or ksh93; the harness avoids ksh93-only syntax.
#
# Covers: both healthy keeps the weekly preferred role (odd week Standby,
# even week Master, a zero-padded week number); a preferred role failing over
# either address family fails over to the healthy peer; a recovered
# preferred role takes its role back; both failing leaves DNS unchanged and
# exits non-zero; a host without state starts on the preferred role without
# publishing.

set -eu

script_dir=$(cd "$(dirname "$0")/.." && pwd)
work=$(mktemp -d "${TMPDIR:-/tmp}/dns-failover-test.XXXXXX")
trap 'rm -rf "$work"' EXIT

fake=$work/bin
root=$work/root
log=$work/actions.log
mkdir -p "$fake" "$root/etc/gonf-publisher" "$root/gonf-publisher"

sed \
    -e "s|/usr/local/bin/dns-publish.ksh|$fake/dns-publish|" \
    -e "s|/var/nsd|$root|g" \
    "$script_dir/dns-failover.ksh" >"$work/dns-failover.ksh"

cat >"$root/etc/gonf-publisher/publisher.conf" <<'EOF'
DEFAULT_ROLE="fishfinger"
MASTER_NAME="fishfinger"
STANDBY_NAME="blowfish"
EOF

cat >"$fake/date" <<'EOF'
#!/bin/sh
case "$*" in
*%U*) echo "${FAKE_WEEK:?}" ;;
*) exec /bin/date "$@" ;;
esac
EOF

cat >"$fake/timeout" <<'EOF'
#!/bin/sh
shift
exec "$@"
EOF

# ftp -<proto> -o - https://<fqdn>/index.txt
cat >"$fake/ftp" <<'EOF'
#!/bin/sh
proto=${1#-}
url=$4
fqdn=${url#https://}
fqdn=${fqdn%%/*}
case " ${DOWN:-} " in
*" $fqdn:$proto "*) exit 1 ;;
esac
echo "Welcome to $fqdn!"
EOF

printf '#!/bin/sh\nexit 0\n' >"$fake/sleep"
printf '#!/bin/sh\nexit 0\n' >"$fake/logger"
cat >"$fake/dns-publish" <<EOF
#!/bin/sh
echo "publish \$*" >>"$log"
EOF
chmod +x "$fake"/*

failures=0

# run_case name expected-status expected-actions committed-role [VAR=value...]:
# commit the role (or none for "-"), run the failover with the assignments
# exported, and compare its exit status and the publisher calls it made.
run_case() {
    name=$1 want_status=$2 expected=$3 committed=$4
    shift 4
    : >"$log"
    if [ "$committed" = - ]; then
        rm -f "$root/gonf-publisher/state"
    else
        print "role=$committed" >"$root/gonf-publisher/state"
    fi
    if (
        for assignment in "$@"; do
            export "${assignment?}"
        done
        PATH="$fake:$PATH" exec ksh "$work/dns-failover.ksh"
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

run_case "odd week, both healthy, Standby kept" 0 "" blowfish FAKE_WEEK=45
run_case "even week, both healthy, Master kept" 0 "" fishfinger FAKE_WEEK=46
run_case "a new odd week rotates to Standby" 0 "publish -r blowfish" fishfinger FAKE_WEEK=45
run_case "a new even week rotates to Master" 0 "publish -r fishfinger" blowfish FAKE_WEEK=46
run_case "a zero-padded even week is Master" 0 "publish -r fishfinger" blowfish FAKE_WEEK=08
run_case "preferred down over IPv4 fails over" 0 "publish -r fishfinger" blowfish \
    FAKE_WEEK=45 "DOWN=blowfish.buetow.org:4"
run_case "preferred down over IPv6 fails over" 0 "publish -r fishfinger" blowfish \
    FAKE_WEEK=45 "DOWN=blowfish.buetow.org:6"
run_case "a failed-over role is kept while preferred is down" 0 "" fishfinger \
    FAKE_WEEK=45 "DOWN=blowfish.buetow.org:4 blowfish.buetow.org:6"
run_case "a recovered preferred role takes over again" 0 "publish -r blowfish" fishfinger \
    FAKE_WEEK=45
run_case "both down leaves DNS unchanged" 1 "" blowfish \
    FAKE_WEEK=45 "DOWN=blowfish.buetow.org:4 fishfinger.buetow.org:6"
run_case "no state starts on the preferred role without publishing" 0 "" - FAKE_WEEK=45
run_case "no state fails over like a committed preferred role" 0 "publish -r fishfinger" - \
    FAKE_WEEK=45 "DOWN=blowfish.buetow.org:4"

# The script's own messages are shown only on failure.
if [ "$failures" -ne 0 ]; then
    cat "$work/output"
    print "$failures failure(s)"
    exit 1
fi
print "all dns-failover cases passed"
