#!/bin/sh
# Read-only kernel comparison. No guest, updater, service or reboot commands.
set -eu
PATH=${KERNEL_METRICS_TEST_PATH:-/bin:/usr/bin:/sbin:/usr/sbin}
export PATH
umask 077

mode=${1:-}
case $mode in
    '') output=/var/tmp/node_exporter/kernel_reboot.prom ;;
    --stdout) output= ;;
    *) exit 64 ;;
esac

now=$(date +%s)
success=1
installed=$(freebsd-version -k 2>/dev/null) || success=0
running=$(freebsd-version -r 2>/dev/null) || success=0
# Treat empty or malformed output as failed coverage, never as "no reboot".
for version in "$installed" "$running"; do
    case $version in
        ''|*[!a-zA-Z0-9._+-]*) success=0 ;;
    esac
    if ! printf '%s\n' "$version" | LC_ALL=C grep -Eq \
        '^[0-9]+\.[0-9]+-(RELEASE|STABLE|CURRENT|BETA[0-9]+|RC[0-9]+)(-p[0-9]+)?$'; then
        success=0
    fi
done

metric() {
    printf '# HELP node_freebsd_%s %s\n' "$1" "$2"
    printf '# TYPE node_freebsd_%s gauge\n' "$1"
    printf 'node_freebsd_%s %s\n' "$1" "$3"
}

emit() {
    metric kernel_check_success 'Whether the most recent kernel check succeeded.' "$success"
    metric kernel_check_timestamp_seconds 'Unix time of the most recent kernel check.' "$now"
    if [ "$success" -eq 1 ]; then
        pending=0
        [ "$installed" = "$running" ] || pending=1
        metric kernel_reboot_pending 'Whether installed and running kernels differ.' "$pending"
    fi
}

if [ -z "$output" ]; then
    emit
else
    temporary=$(mktemp "${output}.XXXXXX")
    trap 'rm -f "$temporary"' EXIT HUP INT TERM
    emit >"$temporary"
    chmod 644 "$temporary"
    mv -f "$temporary" "$output"
fi
[ "$success" -eq 1 ]
