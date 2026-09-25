#!/bin/sh
#
# ZFS pool and dataset metrics for the node_exporter textfile collector.
#
# Runs every minute from root's crontab on f0-f3 (gonf task
# freebsd_monitoring_zfs_metrics) and writes $FINAL_FILE, which node_exporter
# (node_exporter_textfile_dir in rc.conf) exposes to Prometheus. Output is
# discarded by the cron line, so the script must never need its stdout.
#
# Until 2026-09-25 this was a hand-copied, unversioned script (owned by paul)
# on f0-f2 only. Changes against that version (task kk2), metric names and
# labels unchanged:
#   - The temp file is a mktemp in the textfile directory, removed on any
#     exit; the old fixed "$FINAL_FILE.$$" name leaked when a run was killed
#     (f0 carried a stray zfs_pools.prom.<pid> from August).
#   - A failing zpool/zfs command keeps the previous file instead of
#     publishing a truncated one (node_textfile_mtime_seconds shows staleness).
#   - Each metric family's HELP/TYPE lines are directly followed by its
#     samples, as the text exposition format requires (they were emitted
#     up front and the samples interleaved per pool).
#   - Pools whose names contain "-" are no longer skipped in the I/O section.
#   - PATH is set explicitly, so zpool/zfs are found under any cron PATH.
#
# Known caveat: "zpool iostat -Hp" without an interval prints the AVERAGE
# operations and bytes per second since the pool was imported, not
# cumulative totals, so the zfs_pool_{read,write}_*_total series are not
# real counters (rate() over them is meaningless).

PATH=/sbin:/bin:/usr/sbin:/usr/bin
export PATH

TEXTFILE_DIR="/var/tmp/node_exporter"
FINAL_FILE="$TEXTFILE_DIR/zfs_pools.prom"

mkdir -p "$TEXTFILE_DIR" || exit 1

# The temp name must not end in .prom, or node_exporter would read it
# half-written. Same directory as $FINAL_FILE, so the mv is an atomic rename.
TMP_FILE=$(mktemp "$TEXTFILE_DIR/.zfs_pools.XXXXXX") || exit 1
trap 'rm -f "$TMP_FILE"' EXIT
trap 'exit 1' HUP INT TERM

# Collect everything first: a failing command aborts before anything is
# published.
pools=$(zpool list -Hp -o name,size,allocated,free,capacity,health) || exit 1
iostat=$(zpool iostat -Hp) || exit 1
datasets=$(zfs list -Hp -t filesystem -o name,used,available,referenced,logicalused) || exit 1

# family NAME TYPE HELP: print a metric family header.
family() {
    echo "# HELP $1 $3"
    echo "# TYPE $1 $2"
}

# pool_metric NAME TYPE HELP FIELD INPUT: print a pool-labelled family whose
# value is the tab-separated column FIELD of INPUT (column 1 is the pool).
pool_metric() {
    family "$1" "$2" "$3"
    printf '%s\n' "$5" | awk -F '\t' -v m="$1" -v f="$4" \
        'NF && $1 != "" { printf "%s{pool=\"%s\"} %s\n", m, $1, $f }'
}

# dataset_metric NAME HELP FIELD: print a dataset family whose value is
# column FIELD of $datasets; an empty value is exported as 0. The name needs
# no label escaping: ZFS names cannot contain a backslash or double quote.
dataset_metric() {
    family "$1" gauge "$2"
    printf '%s\n' "$datasets" | awk -F '\t' -v m="$1" -v f="$3" '
        NF && $1 != "" {
            pool = $1; sub(/\/.*/, "", pool)
            v = $f; if (v == "") v = 0
            printf "%s{pool=\"%s\",dataset=\"%s\"} %s\n", m, pool, $1, v
        }'
}

# pool_health: map zpool health states onto numbers (0 = ONLINE).
pool_health() {
    family zfs_pool_health gauge "Health status of ZFS pool"
    printf '%s\n' "$pools" | awk -F '\t' '
        NF && $1 != "" {
            h = 6
            if ($6 == "ONLINE") h = 0
            else if ($6 == "DEGRADED") h = 1
            else if ($6 == "FAULTED") h = 2
            else if ($6 == "OFFLINE") h = 3
            else if ($6 == "UNAVAIL") h = 4
            else if ($6 == "REMOVED") h = 5
            printf "zfs_pool_health{pool=\"%s\"} %s\n", $1, h
        }'
}

{
    # Pool space (zpool list -p prints capacity as a bare number).
    pool_metric zfs_pool_size_bytes gauge "Total size of ZFS pool in bytes" 2 "$pools"
    pool_metric zfs_pool_allocated_bytes gauge "Allocated space in ZFS pool in bytes" 3 "$pools"
    pool_metric zfs_pool_free_bytes gauge "Free space in ZFS pool in bytes" 4 "$pools"
    pool_metric zfs_pool_capacity_percent gauge "Capacity percentage of ZFS pool" 5 "$pools"
    pool_health

    # I/O (columns: name alloc free read_ops write_ops read_bw write_bw; see
    # the caveat in the header).
    pool_metric zfs_pool_read_operations_total counter "Total read operations since pool creation" 4 "$iostat"
    pool_metric zfs_pool_write_operations_total counter "Total write operations since pool creation" 5 "$iostat"
    pool_metric zfs_pool_read_bytes_total counter "Total bytes read since pool creation" 6 "$iostat"
    pool_metric zfs_pool_write_bytes_total counter "Total bytes written since pool creation" 7 "$iostat"

    dataset_metric zfs_dataset_used_bytes "Used space in ZFS dataset in bytes" 2
    dataset_metric zfs_dataset_available_bytes "Available space in ZFS dataset in bytes" 3
    dataset_metric zfs_dataset_referenced_bytes "Referenced space in ZFS dataset in bytes" 4
    dataset_metric zfs_dataset_logicalused_bytes "Logical used space in ZFS dataset in bytes" 5
} > "$TMP_FILE" || exit 1

# mktemp creates 0600; node_exporter runs as nobody and must read the file.
chmod 0644 "$TMP_FILE" && mv "$TMP_FILE" "$FINAL_FILE"
