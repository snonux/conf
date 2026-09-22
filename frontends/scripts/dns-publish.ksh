#!/bin/ksh

# The sole writer for effective frontend DNS zones. Gonf installs immutable
# inputs below /var/nsd/etc/gonf-publisher and invokes this command without a
# role; dns-failover.ksh invokes it with a health-verified role. A publication
# snapshots every affected live input before a single change, so an interrupted
# run can restore a complete known-good set before accepting more work.

set -eu

readonly PUBLISHER=blowfish
readonly INPUT_DIR=/var/nsd/etc/gonf-publisher
readonly INPUT_ZONES=$INPUT_DIR/zones
readonly INPUT_KEY=$INPUT_DIR/key.conf
readonly STATE_DIR=/var/nsd/gonf-publisher
readonly STATE=$STATE_DIR/state
readonly JOURNAL=$STATE_DIR/journal
readonly LOCK=$STATE_DIR/lock
readonly ZONE_DIR=/var/nsd/zones/master
readonly STAGE_DIR=/var/nsd/zones/gonf-publisher
readonly LIVE_KEY=/var/nsd/etc/key.conf
readonly LIVE_CONFIG=/var/nsd/etc/nsd.conf
readonly GONF=/usr/local/bin/gonf
# A Gonf-driven (role-less) publication waits this many seconds for a running
# publisher, then fails so the apply reports that nothing was published.
readonly LOCK_WAIT_SECONDS=60

usage() {
    print -u2 "usage: dns-publish.ksh [-r master-role]"
    exit 64
}

role=
while getopts "r:" option; do
    case $option in
    r) role=$OPTARG ;;
    *) usage ;;
    esac
done
shift $((OPTIND - 1))
[ $# -eq 0 ] || usage

[ "$(hostname -s)" = "$PUBLISHER" ] || exit 0
[ -r "$INPUT_DIR/publisher.conf" ] || { print -u2 "missing publisher inputs"; exit 1; }
[ -r "$INPUT_DIR/nsd.conf" ] || { print -u2 "missing publisher NSD configuration"; exit 1; }
[ -r "$INPUT_KEY" ] || { print -u2 "missing publisher NSD key"; exit 1; }
# shellcheck disable=SC1091 # controller-installed immutable publisher input
. "$INPUT_DIR/publisher.conf"
: "${DEFAULT_ROLE:?missing default role}"
: "${MASTER_NAME:?missing master name}"
: "${MASTER_IPV4:?missing master IPv4}"
: "${MASTER_IPV6:?missing master IPv6}"
: "${STANDBY_NAME:?missing standby name}"
: "${STANDBY_IPV4:?missing standby IPv4}"
: "${STANDBY_IPV6:?missing standby IPv6}"
: "${ZONES:?missing zone list}"
REMOVED_ZONES=${REMOVED_ZONES-}

requested_role=$role
case ${role:-$DEFAULT_ROLE} in
"$MASTER_NAME"|"$STANDBY_NAME") role=${role:-$DEFAULT_ROLE} ;;
*) print -u2 "invalid requested DNS role"; exit 1 ;;
esac

valid_zone_name() {
    case $1 in
    ""|.*|*.|*..*|*[!abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789.-]*) return 1 ;;
    *) return 0 ;;
    esac
}

all_zone_names() {
    typeset seen="" zone
    for zone in $ZONES $REMOVED_ZONES; do
        valid_zone_name "$zone" || { print -u2 "invalid DNS zone name: $zone"; return 1; }
        case " $seen " in
        *" $zone "*) ;;
        *) print -r -- "$zone"; seen="$seen $zone" ;;
        esac
    done
}

validate_inputs() {
    typeset zone
    for zone in $ZONES; do
        valid_zone_name "$zone" || { print -u2 "invalid DNS zone name: $zone"; return 1; }
        [ -r "$INPUT_ZONES/$zone.zone.tpl" ] || { print -u2 "missing DNS template: $zone"; return 1; }
    done
    for zone in $REMOVED_ZONES; do
        valid_zone_name "$zone" || { print -u2 "invalid removed DNS zone name: $zone"; return 1; }
        case " $ZONES " in *" $zone "*) print -u2 "zone is both active and removed: $zone"; return 1 ;; esac
    done
}

# The owner record is deliberately published only after the lock directory is
# created. A contender must never remove a directory without a complete owner
# record: that short interval belongs to the process which won mkdir(1).
#
# PID alone is not an identity: it can be reused after a crashed publisher. The
# recorded lstart value distinguishes that reused PID. The token makes cleanup
# reject a lock recreated by another invocation, even if it happens to have the
# same PID and start time representation.
LOCK_HELD=no
LOCK_START=
LOCK_TOKEN=

process_start() {
    typeset pid=$1 raw start
    raw=$(ps -o lstart= -p "$pid" 2>/dev/null) || return 1
    start=$(print -r -- "$raw" | awk '
        NR == 1 {
            sub(/^[[:space:]]+/, "")
            gsub(/[[:space:]]+/, " ")
            if ($0 != "") print
            exit
        }
    ')
    case $start in
    ""|*[![:alnum:]:\ ]*) return 1 ;;
    esac
    print -r -- "$start"
}

new_lock_token() {
    typeset token
    token=$(dd if=/dev/urandom bs=16 count=1 2>/dev/null | od -An -tx1 | tr -d ' \n')
    case $token in
    ????????*) print -r -- "$token" ;;
    *) return 1 ;;
    esac
}

# Print the complete owner as pid|start|token. Invalid or partially-written
# metadata is intentionally not a stale lock: recovery then requires an
# operator rather than risking the active publisher's transaction.
lock_owner() {
    [ -r "$LOCK/owner" ] || return 1
    awk '
        /^pid=[0-9]+$/ {
            if (++pids != 1) bad = 1
            pid = substr($0, 5)
            next
        }
        /^start=[[:alnum:]: ]+$/ {
            if (++starts != 1) bad = 1
            start = substr($0, 7)
            next
        }
        /^token=[[:xdigit:]]+$/ {
            if (++tokens != 1) bad = 1
            token = substr($0, 7)
            next
        }
        { bad = 1 }
        END {
            if (!bad && pids == 1 && starts == 1 && tokens == 1)
                print pid "|" start "|" token
            else
                exit 1
        }
    ' "$LOCK/owner"
}

# The "|" separator must be quoted: a bare | inside these patterns is pattern
# alternation in OpenBSD's ksh and in ksh93, so ${owner%%|*} expanded to ""
# and every owner record, even a live publisher's, looked stale.
split_lock_owner() {
    typeset owner=$1 rest
    LOCK_OWNER_PID=${owner%%"|"*}
    rest=${owner#*"|"}
    LOCK_OWNER_START=${rest%%"|"*}
    LOCK_OWNER_TOKEN=${rest#*"|"}
    [ "$LOCK_OWNER_PID" != "$owner" ] && [ "$LOCK_OWNER_START" != "$rest" ]
}

lock_is_ours() {
    typeset owner current_start
    [ "$LOCK_HELD" = yes ] || return 1
    owner=$(lock_owner) || return 1
    split_lock_owner "$owner" || return 1
    current_start=$(process_start "$$") || return 1
    [ "$LOCK_OWNER_PID" = "$$" ] &&
        [ "$LOCK_OWNER_START" = "$current_start" ] &&
        [ "$LOCK_OWNER_TOKEN" = "$LOCK_TOKEN" ]
}

release_lock() {
    lock_is_ours || return 0
    rm -rf "$LOCK"
    LOCK_HELD=no
}

# Return 0 only when the complete owner is demonstrably stale, 1 while it is
# active, and 2 when its identity cannot be verified safely.
lock_is_stale() {
    typeset owner current_start
    owner=$(lock_owner) || return 2
    split_lock_owner "$owner" || return 2
    if kill -0 "$LOCK_OWNER_PID" 2>/dev/null; then
        current_start=$(process_start "$LOCK_OWNER_PID") || return 2
        [ "$LOCK_OWNER_START" = "$current_start" ] && return 1
        return 0
    fi

    # A signal denial can look like a dead PID. If ps can still see a process,
    # do not reclaim it; without a reliable identity probe, conservatively
    # leave the lock in place.
    ps -p "$LOCK_OWNER_PID" -o pid= >/dev/null 2>&1 && return 2
    return 0
}

write_lock_owner() {
    typeset temporary=$STATE_DIR/.lock-owner.$LOCK_TOKEN
    umask 077
    {
        print -r -- "pid=$$"
        print -r -- "start=$LOCK_START"
        print -r -- "token=$LOCK_TOKEN"
    } >"$temporary" || return 1
    if ! mv "$temporary" "$LOCK/owner"; then
        rm -f "$temporary"
        return 1
    fi
}

acquire_lock() {
    typeset attempt=0 stale_lock status
    LOCK_START=$(process_start "$$") || {
        print -u2 "cannot identify DNS publisher process start time"
        return 2
    }
    LOCK_TOKEN=$(new_lock_token) || {
        print -u2 "cannot create DNS publisher lock token"
        return 2
    }

    while [ "$attempt" -lt 3 ]; do
        if mkdir -m 700 "$LOCK" 2>/dev/null; then
            if ! write_lock_owner; then
                # No other publisher can claim this still-existing directory.
                # rmdir only removes our empty, unpublished acquisition.
                rmdir "$LOCK" 2>/dev/null || :
                print -u2 "cannot publish DNS lock ownership"
                return 2
            fi
            LOCK_HELD=yes
            trap 'release_lock' EXIT
            trap 'release_lock; exit 1' HUP INT TERM
            return 0
        fi

        if lock_is_stale; then
            status=0
        else
            status=$?
        fi
        case $status in
        1)
            print "dns publication already in progress"
            return 1
            ;;
        2)
            print -u2 "DNS publication lock ownership is incomplete or unverifiable; refusing recovery"
            return 2
            ;;
        esac

        # Rename is atomic. Only the process that moved this exact stale lock
        # may remove it; another contender can then win mkdir without being
        # deleted by a late stale-lock cleanup.
        stale_lock=$STATE_DIR/.stale-lock.$LOCK_TOKEN
        if mv "$LOCK" "$stale_lock" 2>/dev/null; then
            print -u2 "removing stale DNS publication lock"
            rm -rf "$stale_lock" || return 2
        fi
        attempt=$((attempt + 1))
    done
    print -u2 "cannot acquire DNS publication lock"
    return 2
}

committed_role() {
    [ -f "$STATE" ] || { print "$DEFAULT_ROLE"; return; }
    awk -F= -v master="$MASTER_NAME" -v standby="$STANDBY_NAME" '
        /^[[:space:]]*$/ { next }
        $1 == "role" { if (++n != 1 || ($2 != master && $2 != standby)) exit 1; role = $2; next }
        { exit 1 }
        END { if (n != 1) exit 1; print role }
    ' "$STATE"
}

serial_for() {
    typeset zone=$1 file=$ZONE_DIR/$1.zone serial
    [ -f "$file" ] || { print 1; return; }
    serial=$("$GONF" dns-zone-serial "$zone" "$file") || return 1
    if [ "$serial" -eq 4294967295 ]; then
        print 0
    else
        print $((serial + 1))
    fi
}

render_zone() {
    typeset zone=$1 serial=$2 output=$3 master_ipv4 master_ipv6 standby_ipv4 standby_ipv6
    if [ "$role" = "$MASTER_NAME" ]; then
        master_ipv4=$MASTER_IPV4; master_ipv6=$MASTER_IPV6
        standby_ipv4=$STANDBY_IPV4; standby_ipv6=$STANDBY_IPV6
    else
        master_ipv4=$STANDBY_IPV4; master_ipv6=$STANDBY_IPV6
        standby_ipv4=$MASTER_IPV4; standby_ipv6=$MASTER_IPV6
    fi
    sed \
        -e "s|@SERIAL@|$serial|g" \
        -e "s|@MASTER_IPV4@|$master_ipv4|g" \
        -e "s|@MASTER_IPV6@|$master_ipv6|g" \
        -e "s|@STANDBY_IPV4@|$standby_ipv4|g" \
        -e "s|@STANDBY_IPV6@|$standby_ipv6|g" \
        "$INPUT_ZONES/$zone.zone.tpl" >"$output"
}

render_candidate_config() {
    sed \
        -e "s|$LIVE_KEY|$STAGE_DIR/key.conf|g" \
        -e 's|zonefile: "master/|zonefile: "gonf-publisher/|g' \
        "$INPUT_DIR/nsd.conf" >"$STAGE_DIR/nsd.conf"
}

# validate_candidate is called as 'validate_candidate || ...', which disables
# set -e for every command inside it (POSIX: set -e does not apply to a
# command that is not the last in a && / || list, and that reaches into
# function bodies called from such a context). Each check below must
# therefore be tested explicitly with '|| return 1': relying on set -e, or on
# a check being the function's last statement, would silently accept a zone
# or configuration that NSD itself would reject.
validate_candidate() {
    typeset zone
    for zone in $ZONES; do
        nsd-checkzone "$zone" "$STAGE_DIR/$zone.zone" || return 1
    done
    nsd-checkconf "$STAGE_DIR/nsd.conf" || return 1
}

install_atomic() {
    typeset source=$1 destination=$2 mode=$3 group=$4 temporary=$2.new.$$
    rm -f "$temporary"
    install -m "$mode" -o root -g "$group" "$source" "$temporary" || return 1
    mv -f "$temporary" "$destination"
}

snapshot_file() {
    typeset source=$1 label=$2 snapshot=$3
    if [ -f "$source" ]; then
        print present >"$snapshot/present/$label"
        cp -p "$source" "$snapshot/files/$label"
    else
        print absent >"$snapshot/present/$label"
    fi
}

snapshot_zone() {
    typeset zone=$1 snapshot=$2 source=$ZONE_DIR/$1.zone label=zone.$1
    print "zone=$zone" >>"$snapshot/manifest"
    if [ -f "$source" ]; then
        print present >"$snapshot/present/$label"
        cp -p "$source" "$snapshot/zones/$zone.zone"
    else
        print absent >"$snapshot/present/$label"
    fi
}

create_journal() {
    typeset temporary=$STATE_DIR/.journal.$$.new zone
    [ ! -e "$JOURNAL" ] || { print -u2 "unexpected DNS publication journal"; return 1; }
    rm -rf "$temporary"
    mkdir -p "$temporary/files" "$temporary/present" "$temporary/zones" || return 1
    : >"$temporary/manifest"
    for zone in $(all_zone_names); do
        snapshot_zone "$zone" "$temporary" || return 1
    done
    snapshot_file "$LIVE_KEY" key "$temporary" || return 1
    snapshot_file "$LIVE_CONFIG" config "$temporary" || return 1
    snapshot_file "$STATE" state "$temporary" || return 1
    # Whether NSD ran before this commit decides whether the commit and any
    # rollback must leave it running (see nsd_was_running).
    if rcctl check nsd >/dev/null 2>&1; then
        print yes >"$temporary/nsd-running"
    else
        print no >"$temporary/nsd-running"
    fi || return 1
    : >"$temporary/complete"
    mv "$temporary" "$JOURNAL" || return 1
    : >"$JOURNAL/incomplete"
    sync || return 1
}

restore_file() {
    typeset source=$1 destination=$2 marker=$3 temporary=$2.rollback.$$
    case $(cat "$marker") in
    present) cp -p "$source" "$temporary" && mv -f "$temporary" "$destination" ;;
    absent) rm -f "$destination" ;;
    *) print -u2 "invalid DNS journal marker: $marker"; return 1 ;;
    esac
}

rollback() {
    typeset zone
    [ -f "$JOURNAL/complete" ] && [ -f "$JOURNAL/incomplete" ] || return 1
    while IFS='=' read -r field zone; do
        [ "$field" = zone ] && valid_zone_name "$zone" || return 1
        restore_file "$JOURNAL/zones/$zone.zone" "$ZONE_DIR/$zone.zone" "$JOURNAL/present/zone.$zone" || return 1
    done <"$JOURNAL/manifest"
    restore_file "$JOURNAL/files/key" "$LIVE_KEY" "$JOURNAL/present/key" || return 1
    restore_file "$JOURNAL/files/config" "$LIVE_CONFIG" "$JOURNAL/present/config" || return 1
    restore_file "$JOURNAL/files/state" "$STATE" "$JOURNAL/present/state" || return 1
    # The failed commit may have installed a new key or nsd.conf, and a failed
    # restart may have left NSD stopped. If NSD ran before the commit, it is
    # (re)started on the restored set and must be running afterwards; if not,
    # the rollback is incomplete and the journal is kept, so every later run
    # retries this rollback and refuses to publish until NSD runs again.
    nsd_was_running || return 0
    if rcctl check nsd >/dev/null 2>&1; then
        rcctl restart nsd
    else
        rcctl start nsd
    fi
    nsd_running || { print -u2 "NSD is not running after the DNS rollback"; return 1; }
}

# nsd_was_running reports whether NSD ran before the journaled commit. A
# journal without the marker falls back to the daemon's current state.
nsd_was_running() {
    [ -f "$JOURNAL/nsd-running" ] || { nsd_running; return; }
    [ "$(cat "$JOURNAL/nsd-running")" = yes ]
}

nsd_running() {
    rcctl check nsd >/dev/null 2>&1
}

# apply_nsd makes the running daemon serve the committed files. "reload" is
# enough when only zone files changed: `nsd-control reload` rereads zone
# files but not nsd.conf or the TSIG key include. "restart" is required when
# the key or nsd.conf changed (added/removed zones, notify/provide-xfr, key
# rotation, server options), as in the Rex recipe; `nsd-control reconfig`
# would miss server: options, so it is not used. A failed reload or restart,
# or NSD not running afterwards, fails the commit and triggers the rollback.
apply_nsd() {
    typeset how=$1
    # NSD that was not running before the commit (first installation, or
    # stopped by the operator) is left alone: a Gonf publication is followed
    # by Service[nsd], which starts it; a failover publication from cron
    # leaves it stopped.
    nsd_was_running || return 0
    case $how in
    reload) nsd-control reload || return 1 ;;
    restart) rcctl restart nsd || return 1 ;;
    *) print -u2 "invalid NSD apply mode: $how"; return 1 ;;
    esac
    nsd_running || { print -u2 "NSD is not running after the DNS $how"; return 1; }
}

# wait_for_lock acquires the publication lock. A failover run (-r) treats a
# running publisher as a normal no-op, as before. A Gonf-driven run retries
# for LOCK_WAIT_SECONDS and then fails with 75 (EX_TEMPFAIL), so the apply
# does not report success for a publication that never happened.
wait_for_lock() {
    typeset waited=0 status
    while :; do
        if acquire_lock; then
            return 0
        else
            status=$?
        fi
        [ "$status" -eq 1 ] || exit "$status"
        [ -z "$requested_role" ] || exit 0
        if [ "$waited" -ge "$LOCK_WAIT_SECONDS" ]; then
            print -u2 "DNS publication lock still held after ${LOCK_WAIT_SECONDS}s; nothing published"
            exit 75
        fi
        sleep 1
        waited=$((waited + 1))
    done
}

recover() {
    [ -d "$JOURNAL" ] || return 0
    if [ -f "$JOURNAL/incomplete" ]; then
        print -u2 "recovering incomplete DNS publication"
        rollback || { print -u2 "DNS rollback failed; refusing publication"; return 1; }
    elif [ ! -f "$JOURNAL/complete" ]; then
        print -u2 "malformed DNS publication journal; refusing publication"
        return 1
    fi
    rm -rf "$JOURNAL"
}

write_state() {
    print "role=$role" >"$STATE_DIR/state.new.$$"
    mv -f "$STATE_DIR/state.new.$$" "$STATE"
}

commit() {
    # Not named "mode": install_atomic has a typeset mode, and a POSIX-style
    # function's typeset is global in ksh93 (the test harness's shell).
    typeset zone apply=reload
    for zone in $ZONES; do
        [ ! -f "$STAGE_DIR/.changed-$zone" ] || install_atomic "$STAGE_DIR/$zone.zone" "$ZONE_DIR/$zone.zone" 0644 wheel || return 1
    done
    for zone in $REMOVED_ZONES; do
        [ ! -f "$STAGE_DIR/.remove-$zone" ] || rm -f "$ZONE_DIR/$zone.zone" || return 1
    done
    if [ -f "$STAGE_DIR/.changed-key" ]; then
        install_atomic "$INPUT_KEY" "$LIVE_KEY" 0640 _nsd || return 1
        apply=restart
    fi
    if [ -f "$STAGE_DIR/.changed-config" ]; then
        install_atomic "$INPUT_DIR/nsd.conf" "$LIVE_CONFIG" 0640 _nsd || return 1
        apply=restart
    fi
    write_state || return 1
    apply_nsd "$apply"
}

main() {
    typeset previous changed=0 zone serial status
    mkdir -p "$STATE_DIR" "$ZONE_DIR" "$STAGE_DIR"
    # Incomplete or unverifiable lock ownership is a hard failure so cron and
    # Gonf report the required intervention.
    wait_for_lock
    validate_inputs || exit 1
    recover || exit 1
    previous=$(committed_role) || { print -u2 "malformed DNS publisher state"; exit 1; }
    [ -n "$requested_role" ] || role=$previous

    rm -rf "$STAGE_DIR"
    mkdir -p "$STAGE_DIR"
    cp "$INPUT_KEY" "$STAGE_DIR/key.conf"
    for zone in $ZONES; do
        serial=$(serial_for "$zone") || { print -u2 "invalid apex SOA serial in $zone"; exit 1; }
        render_zone "$zone" "$serial" "$STAGE_DIR/$zone.zone"
        if [ ! -f "$ZONE_DIR/$zone.zone" ]; then
            changed=1; : >"$STAGE_DIR/.changed-$zone"
        elif "$GONF" dns-zone-equivalent "$zone" "$STAGE_DIR/$zone.zone" "$ZONE_DIR/$zone.zone"; then
            :
        else
            status=$?
            [ "$status" -eq 1 ] || { print -u2 "cannot compare $zone candidate"; exit 1; }
            changed=1; : >"$STAGE_DIR/.changed-$zone"
        fi
    done
    for zone in $REMOVED_ZONES; do
        [ ! -f "$ZONE_DIR/$zone.zone" ] || { changed=1; : >"$STAGE_DIR/.remove-$zone"; }
    done
    cmp -s "$INPUT_KEY" "$LIVE_KEY" || { changed=1; : >"$STAGE_DIR/.changed-key"; }
    render_candidate_config
    cmp -s "$INPUT_DIR/nsd.conf" "$LIVE_CONFIG" || { changed=1; : >"$STAGE_DIR/.changed-config"; }
    [ "$role" = "$previous" ] && [ "$changed" -eq 0 ] && exit 0

    validate_candidate || { print -u2 "DNS candidate validation failed"; exit 1; }
    create_journal || { print -u2 "cannot create DNS publication journal"; exit 1; }
    if ! commit; then
        # A complete rollback restored the previous set, which is committed
        # again, so its journal is dropped; an incomplete one keeps it.
        if rollback; then
            rm -rf "$JOURNAL"
        else
            print -u2 "DNS rollback failed; keeping the journal"
        fi
        exit 1
    fi
    rm -f "$JOURNAL/incomplete"
    sync
    rm -rf "$JOURNAL"
}

main
