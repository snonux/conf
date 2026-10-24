#!/bin/sh
# Refresh dserver keys without following daemon-controlled destinations.
# Keep this portable body identical in the four package variants.
set -eu
umask 077
PATH=/usr/bin:/bin:/usr/sbin:/sbin:/usr/local/bin
SHELL=/bin/sh
export PATH SHELL

RUNDIR=/var/run/dserver
CACHEDIR=$RUNDIR/cache
CACHE_ROOT=no
DSERVER_GROUP=_dserver

_fail() { echo "dserver key cache: $*" >&2; exit 1; }
[ "$(id -u)" -eq 0 ] || _fail 'must run as root'
groupid=$(id -g "$DSERVER_GROUP")

# stat defaults to lstat on both GNU and BSD systems.
case $(uname -s) in
    Linux) _stat() { stat -c '%u:%g:%a:%h' "$1"; } ;;
    *) _stat() { stat -f '%u:%g:%Lp:%l' "$1"; } ;;
esac

_secure_dir() {
    path=$1
    if [ ! -e "$path" ] && [ ! -L "$path" ]; then
        # mkdir is exclusive: fail if another process creates the path.
        mkdir -m 700 "$path"
        chown "root:$DSERVER_GROUP" "$path"
        chmod 750 "$path"
    fi
    [ ! -L "$path" ] && [ -d "$path" ] || _fail "unsafe directory: $path"
    metadata=$(_stat "$path")
    case $metadata in
        "0:$groupid:750:"*) ;;
        *) _fail "unsafe ownership/mode: $path; see the 3n3 rollout runbook" ;;
    esac
}

# Never repair a daemon-owned tree with recursive chown/chmod: an attacker
# could rename entries underneath those operations. Migrate it while stopped.
_secure_dir "$RUNDIR"
_secure_dir "$CACHEDIR"
lock=$RUNDIR/key-cache.lock
mkdir -m 700 "$lock" || _fail 'another refresh is running (or stale lock)'
stage=
_cleanup() {
    [ -z "$stage" ] || rm -rf "$stage"
    rmdir "$lock"
}
trap _cleanup EXIT
trap 'exit 1' HUP INT TERM
stage=$(mktemp -d "$CACHEDIR/.refresh.XXXXXXXXXX")

_source_ok() {
    [ ! -L "$home_dir" ] && [ ! -L "$home_dir/.ssh" ] &&
        [ ! -L "$keysfile" ] && [ -f "$keysfile" ] || return 1
    source_metadata=$(_stat "$keysfile") || return 1
    # A single link also rejects borrowed hardlinks to other users' files.
    case $source_metadata in "$userid:"*":1") return 0 ;; esac
    return 1
}

_cache_keys() {
    remoteuser=$1
    home_dir=$2
    # Restrict names before embedding the path in su's command string.
    case $remoteuser in ''|-*|*[!a-zA-Z0-9_.-]*) return ;; esac
    userid=$(id -u "$remoteuser" 2>/dev/null) || return 0
    keysfile=$home_dir/.ssh/authorized_keys
    _source_ok || return 0
    cachefile=$stage/$remoteuser.authorized_keys
    # The source user, never root, opens the mutable source path. A swap
    # after validation can therefore read only what that user can read.
    if su -m "$remoteuser" -c "exec /bin/cat '$keysfile'" >"$cachefile"; then
        if _source_ok; then
            chown "root:$DSERVER_GROUP" "$cachefile"
            chmod 640 "$cachefile"
            return 0
        fi
    fi
    rm -f "$cachefile"
}

if [ "$CACHE_ROOT" = yes ]; then
    _cache_keys root /root
fi
for home_dir in /home/*; do
    [ -d "$home_dir" ] || continue
    remoteuser=${home_dir##*/}
    # /home/root must not replace the genuine root cache.
    [ "$remoteuser" != root ] || continue
    _cache_keys "$remoteuser" "$home_dir"
done

# Protected directories make rename safe. Reject directories rather than
# letting mv move the candidate inside one. Symlinks/hardlinks are replaced,
# never opened, chowned or chmodded. Invalid/deleted sources revoke old keys.
for cachefile in "$CACHEDIR"/*.authorized_keys; do
    [ -e "$cachefile" ] || [ -L "$cachefile" ] || continue
    [ ! -d "$cachefile" ] || _fail "directory in cache: $cachefile"
    name=${cachefile##*/}
    if [ ! -f "$stage/$name" ]; then
        rm -f "$cachefile"
    fi
done
for cachefile in "$stage"/*.authorized_keys; do
    [ -f "$cachefile" ] || continue
    target=$CACHEDIR/${cachefile##*/}
    # Atomic replacement leaves readers with the previous or new file.
    mv -f "$cachefile" "$target"
done
