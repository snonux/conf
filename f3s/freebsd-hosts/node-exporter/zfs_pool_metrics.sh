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
#
# Pool I/O counters (task uk2): zfs_pool_{read,write}_{operations,bytes}_total
# used to come from "zpool iostat -Hp", which without an interval prints the
# AVERAGE operations and bytes per second since import, not totals, so rate()
# over them was meaningless. OpenZFS exposes no cumulative vdev counters on
# FreeBSD (kstat.zfs.<pool>.misc.iostats only counts I/O through the ARC and
# Direct I/O, and misses "zfs receive": 3.4 KB/s against 656 KB/s written on
# the zrepl sink f1 zdata, 2026-09-25). So the counters are now the devstat
# totals since boot ("iostat -I -x") of the disks behind the pool's leaf
# vdevs, summed per pool, which is the physical I/O "zpool iostat" reports
# (f0 zroot over 30 s: 4.7/26.7 read/write ops/s and 200 KB/s / 1.13 MB/s
# against 4/26 and 199 KB/s / 1.16 MB/s). Caveats:
#   - devstat covers whole disks, not partitions. Swap I/O is subtracted
#     (vm.stats.vm swap counters) when all swap lives on one disk, as on
#     f0-f3 (ada0p3 next to zroot on ada0p4); other partitions on a pool's
#     disk (efi, boot) are counted with the pool, and a disk shared by two
#     pools is counted for both.
#   - The counters restart at 0 on reboot, which rate() treats as a reset.

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

# disk_of DEV: print the devstat disk behind the device path DEV
# ("/dev/ada0p4" and "/dev/ada0p4.eli" -> "ada0"), resolving GEOM labels
# ("/dev/gpt/zfs0") through glabel first. Prints nothing if unresolvable.
disk_of() {
    dev=${1#/dev/}
    case $dev in
    */*) dev=$(glabel status -s | awk -v l="$dev" '$1 == l { print $3 }') ;;
    esac
    printf '%s\n' "$dev" | sed -n 's/^\([a-z][a-z]*[0-9][0-9]*\).*/\1/p'
}

# disks_of: read device paths on stdin (first field, other lines ignored) and
# print their distinct disks space-separated; fails if any path does not map
# to a disk. (No "| sort -u": the pipeline would hide the loop's status.)
disks_of() {
    list=" "
    while read -r dev _; do
        case $dev in /dev/*) ;; *) continue ;; esac
        disk=$(disk_of "$dev")
        [ -n "$disk" ] || return 1
        case $list in *" $disk "*) ;; *) list="$list$disk " ;; esac
    done
    echo $list
}

# swap_io: print "disk read_ops write_ops read_bytes write_bytes" of all swap
# I/O since boot when every swap device is on the same disk, else nothing
# (no swap, or several swap disks whose share cannot be told apart).
swap_io() {
    sdisks=$(swapctl -l | disks_of) || return 1
    case $sdisks in "" | *" "*) return 0 ;; esac
    vm=$(sysctl -n hw.pagesize vm.stats.vm.v_swapin vm.stats.vm.v_swapout \
        vm.stats.vm.v_swappgsin vm.stats.vm.v_swappgsout) || return 1
    printf '%s\n' "$vm" | awk -v d="$sdisks" '
        { v[NR] = $1 }
        END { printf "%s %s %s %.0f %.0f\n", d, v[2], v[3], v[4] * v[1], v[5] * v[1] }'
}

# pool_iostats: print one tab-separated line per pool in $pools: name, read
# ops, write ops, read bytes, write bytes since boot, summed over the devstat
# totals of the pool's disks minus swap_io (see "Pool I/O counters" in the
# header). A pool with an unresolvable vdev or a disk missing from iostat
# fails the whole run (the previous file stays published) rather than
# exporting a partial sum that rate() would read as a counter reset.
pool_iostats() {
    devstat=$(iostat -I -x -d) || return 1
    swap=$(swap_io) || return 1
    for pool in $(printf '%s\n' "$pools" | cut -f 1); do
        pdisks=$(zpool status -P "$pool" | disks_of) || return 1
        [ -n "$pdisks" ] || return 1
        # iostat -I -x columns: device r/i w/i kr/i kw/i ... (kr/kw in KiB).
        printf '%s\n' "$devstat" | awk -v pool="$pool" -v disks="$pdisks" -v swap="$swap" '
            BEGIN { n = split(disks, d, " "); for (i = 1; i <= n; i++) want[d[i]] = 1 }
            ($1 in want) && !($1 in seen) {
                seen[$1] = 1; found++
                r += $2; w += $3; rb += $4 * 1024; wb += $5 * 1024
            }
            END {
                if (found != n) exit 1
                split(swap, s, " ")
                if (s[1] in want) { r -= s[2]; w -= s[3]; rb -= s[4]; wb -= s[5] }
                printf "%s\t%.0f\t%.0f\t%.0f\t%.0f\n", pool, r, w, rb, wb
            }' || return 1
    done
}

# Collect everything first: a failing command aborts before anything is
# published.
pools=$(zpool list -Hp -o name,size,allocated,free,capacity,health) || exit 1
iostat=$(pool_iostats) || exit 1
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

    # I/O counters (columns of pool_iostats: name read_ops write_ops
    # read_bytes write_bytes; see "Pool I/O counters" in the header).
    pool_metric zfs_pool_read_operations_total counter "Read operations on the pool's disks since boot (devstat, swap excluded)" 2 "$iostat"
    pool_metric zfs_pool_write_operations_total counter "Write operations on the pool's disks since boot (devstat, swap excluded)" 3 "$iostat"
    pool_metric zfs_pool_read_bytes_total counter "Bytes read from the pool's disks since boot (devstat, swap excluded)" 4 "$iostat"
    pool_metric zfs_pool_write_bytes_total counter "Bytes written to the pool's disks since boot (devstat, swap excluded)" 5 "$iostat"

    dataset_metric zfs_dataset_used_bytes "Used space in ZFS dataset in bytes" 2
    dataset_metric zfs_dataset_available_bytes "Available space in ZFS dataset in bytes" 3
    dataset_metric zfs_dataset_referenced_bytes "Referenced space in ZFS dataset in bytes" 4
    dataset_metric zfs_dataset_logicalused_bytes "Logical used space in ZFS dataset in bytes" 5
} > "$TMP_FILE" || exit 1

# mktemp creates 0600; node_exporter runs as nobody and must read the file.
chmod 0644 "$TMP_FILE" && mv "$TMP_FILE" "$FINAL_FILE"
