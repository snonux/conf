#!/bin/ksh
# Exercise the real audit branch without an OpenBSD host. On a developer
# machine lacking ksh, `bash -n` plus shellcheck -s ksh validates syntax and
# this harness can be invoked with bash because it uses only ksh/POSIX syntax.

set -eu

script_dir=$(cd "$(dirname "$0")/.." && pwd)
script="$script_dir/unattended-upgrade.sh"
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
if [ "${AUDIT_REPOSITORY:-up}" = down ]; then
    exit 1
fi
printf 'dtail-1.0.tgz\n'
EOF

cat >"$fake/pkg_add" <<'EOF'
#!/bin/sh
if [ "${PKG_CACHE+x}" = x ]; then
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
*)
    printf 'unknown AUDIT_RESULT\n' >&2
    exit 64
    ;;
esac
EOF

chmod +x "$fake/sleep" "$fake/logger" "$fake/timeout" "$fake/ftp" "$fake/pkg_add"

run_audit() {
    log="$work/audit.log"
    lock="$work/audit.lock"
    rm -f "$log"
    rmdir "$lock" 2>/dev/null || true
    UNATTENDED_UPGRADE_TEST_PATH="$fake" \
        UNATTENDED_UPGRADE_TEST_LOG="$log" \
        UNATTENDED_UPGRADE_TEST_LOCK="$lock" \
        PKG_CACHE=must-not-be-used \
        AUDIT_RESULT="$1" AUDIT_REPOSITORY="${2:-up}" \
        bash "$script" audit
}

if ! run_audit clean; then
    printf '%s\n' 'clean audit failed' >&2
    exit 1
fi
grep -q 'package audit clean' "$work/audit.log"

if run_audit pending; then
    printf '%s\n' 'pending package audit unexpectedly succeeded' >&2
    exit 1
fi
grep -q 'package audit found' "$work/audit.log"
grep -q 'demo-1.0->1.1: ok' "$work/audit.log"

if run_audit failure; then
    printf '%s\n' 'failed package manager audit unexpectedly succeeded' >&2
    exit 1
fi
grep -q 'package audit FAILED: pkg_add -Iun rc=1' "$work/audit.log"

if run_audit clean down; then
    printf '%s\n' 'custom repository failure unexpectedly succeeded' >&2
    exit 1
fi
grep -q 'custom packages cannot be audited' "$work/audit.log"

log="$work/audit.log"
lock="$work/audit.lock"
rm -f "$log"
mkdir "$lock"
if UNATTENDED_UPGRADE_TEST_PATH="$fake" \
    UNATTENDED_UPGRADE_TEST_LOG="$log" \
    UNATTENDED_UPGRADE_TEST_LOCK="$lock" \
    bash "$script" audit; then
    printf '%s\n' 'locked audit unexpectedly succeeded' >&2
    exit 1
fi
grep -q 'no audit result was produced' "$log"

printf '%s\n' 'unattended package audit tests passed'
