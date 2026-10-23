#!/bin/sh
# Scoped ro3 rollback. Install/run only as part of an approved live operation.
set -eu
PATH=/bin:/usr/bin:/sbin:/usr/sbin:/usr/local/bin:/usr/local/sbin
export PATH
umask 077

work=/root/ro3-peer-certs-20261023
ssl_dir=/etc/ssl
config_dir=/etc

fail() {
    printf 'Peer certificate rollback failed: %s\n' "$1" >&2
    exit 1
}

[ "$(id -u)" -eq 0 ] || fail 'root required'
case "$(hostname -s)" in
    blowfish)
        names='standby.ecat.buetow.org standby.alt.irregular.ninja standby.gogios.buetow.org' ;;
    fishfinger) names=gogios.buetow.org ;;
    *) fail 'unexpected frontend' ;;
esac
[ -d "$work" ] && [ ! -L "$work" ] || fail 'missing private workspace'
[ -f "$work/previous.gogios.json" ] &&
    [ ! -L "$work/previous.gogios.json" ] || fail 'missing configuration backup'
for name in $names; do
    for file in "$work/$name.fullchain.pem" "$work/$name.key"; do
        [ -f "$file" ] && [ ! -L "$file" ] || fail 'missing or linked backup'
    done
    [ -L "$ssl_dir/$name.crt" ] &&
        [ "$(readlink "$ssl_dir/$name.crt")" = "$ssl_dir/$name.fullchain.pem" ] ||
        fail 'certificate alias changed'
    for file in "$ssl_dir/$name.fullchain.pem" "$ssl_dir/private/$name.key"; do
        [ -f "$file" ] && [ ! -L "$file" ] || fail 'live file layout changed'
    done
done

# Stage every old file before replacing any live file. Keep key staging in
# private/ too, so a separate mount still gets same-filesystem rename.
lock=$work/rollback.lock
mkdir -m 700 "$lock" 2>/dev/null || fail 'rollback already running; inspect guard'
stage=
key_stage=
gogios_tmp=
cleanup() {
    [ -z "$stage" ] || rm -rf "$stage"
    [ -z "$key_stage" ] || rm -rf "$key_stage"
    [ -z "$gogios_tmp" ] || rm -f "$gogios_tmp"
    rmdir "$lock"
}
trap cleanup 0
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM
stage=$(mktemp -d "$ssl_dir/.ro3-rollback.XXXXXXXXXX")
key_stage=$(mktemp -d "$ssl_dir/private/.ro3-rollback.XXXXXXXXXX")
for name in $names; do
    cp -p "$work/$name.fullchain.pem" "$stage/$name.fullchain.pem"
    cp -p "$work/$name.key" "$key_stage/$name.key"
done
gogios_tmp=$(mktemp "$config_dir/.ro3-gogios.XXXXXXXXXX")
cp -p "$work/previous.gogios.json" "$gogios_tmp"
for name in $names; do
    mv -f "$key_stage/$name.key" "$ssl_dir/private/$name.key"
    mv -f "$stage/$name.fullchain.pem" "$ssl_dir/$name.fullchain.pem"
done
mv -f "$gogios_tmp" "$config_dir/gogios.json"
gogios_tmp=

# The relayd configuration is never written by this operation or rollback.
timeout 30 relayd -n -f "$config_dir/relayd.conf" >/dev/null 2>&1 ||
    fail 'native validation failed; daemon was not reloaded'
running() {
    check_status=0
    timeout 10 rcctl check relayd >/dev/null 2>&1 || check_status=$?
    case "$check_status" in
        0) return 0 ;;
        1) return 1 ;;
        *) fail 'unknown relayd status; refusing to start it' ;;
    esac
}
if running; then
    if ! timeout 30 rcctl reload relayd >/dev/null 2>&1; then
        if running; then
            fail 'relayd reload failed'
        fi
    fi
fi
if ! running; then
    [ -f "$work/relayd-was-running" ] && [ ! -L "$work/relayd-was-running" ] ||
        fail 'no running baseline; refusing to start relayd'
    timeout 30 rcctl start relayd >/dev/null 2>&1 || fail 'recovery start failed'
    running || fail 'relayd did not stay running'
fi
printf 'Scoped peer pairs and Gogios restored; private backups retained.\n'
