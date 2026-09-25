#!/bin/sh
# CARP automatic failback script for f0
# Ensures f0 reclaims MASTER role after reboot when storage is ready.
#
# Runs every minute from root's cron on f0 and must stay silent on stdout and
# stderr: cron mails any output to root. Everything goes to $LOGFILE, and a
# failed promotion also goes to syslog at daemon.err so it is not lost in a
# file nobody reads.

LOGFILE="/var/log/carp-auto-failback.log"
MARKER_FILE="/data/nfs/nfs.DO_NOT_REMOVE"
BLOCK_FILE="/data/nfs/nfs.NO_AUTO_FAILBACK"

log_message() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') - $1" >> "$LOGFILE"
}

# "carp state" prints the bare state (MASTER, BACKUP, INIT). Parsing the
# human-readable output with awk '{print $NF}' broke whenever carp added its
# WARNING line (block file present): the last word of that line is not a state.
CURRENT_STATE=$(/usr/local/bin/carp state 2>/dev/null)

case "$CURRENT_STATE" in
MASTER)
    exit 0
    ;;
BACKUP)
    ;;
*)
    # INIT (interface not up yet, e.g. during boot) or unreadable. Promoting
    # from INIT does not stick: on 2026-09-25 11:13 f0 was promoted from INIT
    # during boot and was left in BACKUP. Wait for a settled BACKUP instead;
    # cron retries next minute.
    log_message "SKIP: CARP state is '${CURRENT_STATE:-unknown}', not BACKUP"
    exit 0
    ;;
esac

# Check if /data/nfs is mounted
if ! mount | grep -q "on /data/nfs "; then
    log_message "SKIP: /data/nfs not mounted"
    exit 0
fi

# Check if marker file exists (identifies this as primary storage)
if [ ! -f "$MARKER_FILE" ]; then
    log_message "SKIP: Marker file $MARKER_FILE not found"
    exit 0
fi

# Check if failback is blocked (for maintenance)
if [ -f "$BLOCK_FILE" ]; then
    log_message "SKIP: Failback blocked by $BLOCK_FILE"
    exit 0
fi

# All conditions met - promote to MASTER
log_message "CONDITIONS MET: Promoting to MASTER (was $CURRENT_STATE)"
/usr/local/bin/carp master >/dev/null 2>&1

sleep 2
NEW_STATE=$(/usr/local/bin/carp state 2>/dev/null)
log_message "Failback complete: State is now $NEW_STATE"

if [ "$NEW_STATE" = "MASTER" ]; then
    logger -t carp-auto-failback "CARP: f0 automatically reclaimed MASTER role"
else
    logger -p daemon.err -t carp-auto-failback \
        "CARP: failback to MASTER failed, state is ${NEW_STATE:-unknown}"
fi
