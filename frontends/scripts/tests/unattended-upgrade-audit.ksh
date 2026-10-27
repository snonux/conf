#!/bin/ksh
# Exercise the real audit and package-reporting branches without an OpenBSD
# host. On a developer machine lacking ksh, `bash -n` plus shellcheck -s ksh
# validates syntax. The harness uses ksh/POSIX syntax; TEST_SHELL selects the
# shell for the script under test (bash by default, /bin/ksh on OpenBSD).

set -eu

script_dir=$(cd "$(dirname "$0")/.." && pwd)
script="$script_dir/unattended-upgrade.sh"
shell=${TEST_SHELL:-bash}
work=$(mktemp -d "${TMPDIR:-/tmp}/unattended-audit-test.XXXXXX")
trap 'rm -rf "$work"' EXIT

fake="$work/bin"
mkdir "$fake"

cat >"$fake/sleep" <<'EOF'
#!/bin/sh
exit 0
EOF

cat >"$fake/logger" <<'EOF'
#!/bin/sh
printf '%s\n' "$*" >&2
EOF

cat >"$fake/timeout" <<'EOF'
#!/bin/sh
shift
exec "$@"
EOF

cat >"$fake/ftp" <<'EOF'
#!/bin/sh
case $* in
*/index.txt) printf 'Welcome to fishfinger.buetow.org\n'; exit 0 ;;
esac
if [ "${AUDIT_REPOSITORY:-up}" = down ]; then
    exit 1
fi
printf 'dtail-1.0.tgz\n'
EOF

cat >"$fake/hostname" <<'EOF'
#!/bin/sh
printf 'blowfish\n'
EOF

cat >"$fake/df" <<'EOF'
#!/bin/sh
printf 'Filesystem 1K-blocks Used Available Capacity Mounted\n'
printf 'disk 2097152 0 2097152 0%% /\n'
EOF

cat >"$fake/rcctl" <<'EOF'
#!/bin/sh
exit 0
EOF

cat >"$fake/pkg_add" <<'EOF'
#!/bin/sh
printf '%s\n' "$*" >>"${AUDIT_CALLS:?}"
if [ "$1" = -Iun ] && [ "${PKG_CACHE+x}" = x ]; then
    printf 'PKG_CACHE leaked into audit\n' >&2
    exit 90
fi
case ${AUDIT_RESULT:-clean} in
clean)
    printf 'quirks-7.194 signed on 2026-06-10T19:46:25Z\n'
    ;;
pending)
    printf 'quirks-7.194 signed on 2026-06-10T19:46:25Z\n'
    printf 'demo-1.0->1.1: ok\n'
    ;;
failure)
    printf 'mirror unavailable\n' >&2
    exit 1
    ;;
deferred|mixed-failure|missing-official|unexpected-status)
    printf 'quirks-7.194 signed on 2026-06-10T19:46:25Z\n'
    printf "Couldn't find updates for dtail-4.3.2p1-ng gogios-1.7.1\n"
    case $AUDIT_RESULT in
    mixed-failure) printf 'pkg_add: official mirror unavailable\n' ;;
    missing-official) printf "Couldn't find updates for curl-8.23.0\n" ;;
    unexpected-status) exit 2 ;;
    esac
    exit 1
    ;;
*)
    printf 'unknown AUDIT_RESULT\n' >&2
    exit 64
    ;;
esac
EOF

chmod +x "$fake/"*

run_audit() {
    log="$work/audit.log"
    lock="$work/audit.lock"
    rm -f "$log"
    : >"$work/pkg_add.calls"
    rmdir "$lock" 2>/dev/null || true
    UNATTENDED_UPGRADE_TEST_PATH="$fake" \
        UNATTENDED_UPGRADE_TEST_LOG="$log" \
        UNATTENDED_UPGRADE_TEST_LOCK="$lock" \
        PKG_CACHE=must-not-be-used \
        AUDIT_CALLS="$work/pkg_add.calls" \
        AUDIT_RESULT="$1" AUDIT_REPOSITORY="${2:-up}" \
        "$shell" "$script" "${3:-audit}" >"$work/output" 2>&1
}

if ! run_audit clean; then
    printf '%s\n' 'clean audit failed' >&2
    exit 1
fi
grep -q 'package audit clean' "$work/audit.log"
test ! -s "$work/output"

if run_audit pending; then
    printf '%s\n' 'pending package audit unexpectedly succeeded' >&2
    exit 1
fi
grep -q 'package audit found' "$work/audit.log"
grep -q 'demo-1.0->1.1: ok' "$work/audit.log"
grep -q 'package audit found' "$work/output"

if run_audit failure; then
    printf '%s\n' 'failed package manager audit unexpectedly succeeded' >&2
    exit 1
fi
grep -q 'package audit FAILED: pkg_add -Iun rc=1' "$work/audit.log"
grep -q 'mirror unavailable' "$work/output"

if ! run_audit clean down; then
    printf '%s\n' 'expected custom repository outage failed' >&2
    exit 1
fi
grep -q 'package audit SKIPPED' "$work/audit.log"
grep -q 'custom packages cannot be audited' "$work/audit.log"
test ! -s "$work/output"
test ! -s "$work/pkg_add.calls"
if grep -q 'package audit clean' "$work/audit.log"; then
    printf '%s\n' 'skipped audit was reported clean' >&2
    exit 1
fi

for result in clean deferred; do
    run_audit "$result" down pkgs
    test ! -s "$work/output"
    grep -q 'skipping custom-repo packages' "$work/audit.log"
    grep -q '^-Iu$' "$work/pkg_add.calls"
done
grep -q "Couldn't find updates for dtail" "$work/audit.log"

for result in failure mixed-failure missing-official unexpected-status; do
    if run_audit "$result" down pkgs; then
        printf 'package failure %s unexpectedly succeeded\n' "$result" >&2
        exit 1
    fi
    grep -q 'pkg_add -u FAILED' "$work/output"
done

run_audit pending down pkgs
grep -q 'demo-1.0->1.1: ok' "$work/output"
if run_audit deferred up pkgs; then
    printf '%s\n' 'missing packages with an available repository were ignored' >&2
    exit 1
fi
grep -q 'pkg_add -u FAILED' "$work/output"

log="$work/audit.log"
lock="$work/audit.lock"
rm -f "$log"
mkdir "$lock"
if UNATTENDED_UPGRADE_TEST_PATH="$fake" \
    UNATTENDED_UPGRADE_TEST_LOG="$log" \
    UNATTENDED_UPGRADE_TEST_LOCK="$lock" \
    "$shell" "$script" audit >"$work/output" 2>&1; then
    printf '%s\n' 'locked audit unexpectedly succeeded' >&2
    exit 1
fi
grep -q 'no audit result was produced' "$log"
grep -q 'package audit FAILED' "$work/output"

printf '%s\n' 'unattended package audit and mail-reporting tests passed'
