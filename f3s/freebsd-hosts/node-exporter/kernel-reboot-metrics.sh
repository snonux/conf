#!/bin/sh
# Read-only kernel comparison. No guest, updater, service or reboot commands.
set -eu
PATH=${KERNEL_METRICS_TEST_PATH:-/bin:/usr/bin:/sbin:/usr/sbin}
export PATH
umask 077

mode=${1:-}
case $mode in
    '') output=${KERNEL_METRICS_TEST_OUTPUT:-/var/tmp/node_exporter/kernel_reboot.prom} ;;
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
    # The collector is owned by nobody, who can replace even root-owned
    # entries there. Keep every write and chmod in a root-private directory
    # under trusted, sticky /var/tmp, on the collector's filesystem.
    publish_parent=${KERNEL_METRICS_TEST_PUBLISH_PARENT:-/var/tmp}
    publish=$(mktemp -d "$publish_parent/kernel-reboot-metrics.XXXXXXXX")
    trap 'rm -rf -- "$publish"' 0
    trap 'exit 1' HUP INT TERM
    emit >"$publish/kernel_reboot.prom"
    chmod 644 "$publish/kernel_reboot.prom"
    (
        target_dir=${output%/*}
        cd -P "$target_dir" || exit 1
        [ "$(pwd -P)" = "$target_dir" ] || exit 1
        source_dev=$(stat -f %d "$publish") || exit 1
        target_dev=$(stat -f %d .) || exit 1
        # Refuse mv's cross-filesystem copy fallback onto a public pathname.
        [ "$source_dev" = "$target_dev" ] || exit 1
        # Rename relative to the pinned directory, replacing a destination
        # symlink even when it points to a directory. Never write through it.
        mv -f "$publish/kernel_reboot.prom" .
    )
fi
[ "$success" -eq 1 ]
