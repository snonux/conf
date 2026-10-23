#!/bin/sh
# One-operation rollback for the ho3 repair, installed only after approval.
set -eu
PATH=/bin:/usr/bin:/sbin:/usr/sbin:/usr/local/bin:/usr/local/sbin
export PATH
umask 077

work=/root/ho3-code-cert-20261023
ssl_dir=/etc/ssl
config_dir=/etc

fail() {
    printf 'Certificate rollback failed: %s\n' "$1" >&2
    exit 1
}

[ "$(id -u)" -eq 0 ] || fail 'root required'
host=$(hostname -s)
case "$host" in
    blowfish|fishfinger) ;;
    *) fail 'unexpected frontend' ;;
esac
for file in previous.relayd.conf previous.gogios.json; do
    [ -f "$work/$file" ] && [ ! -L "$work/$file" ] ||
        fail 'missing or symlinked configuration backup'
done
if [ "$host" = fishfinger ]; then
    for file in previous.fullchain.pem previous.key; do
        [ -f "$work/$file" ] && [ ! -L "$work/$file" ] ||
            fail 'missing or symlinked certificate backup'
    done
fi

relay_tmp=
gogios_tmp=
chain_tmp=
key_tmp=
cleanup() {
    for file in "$relay_tmp" "$gogios_tmp" "$chain_tmp" "$key_tmp"; do
        [ -z "$file" ] || rm -f "$file"
    done
}
trap cleanup 0
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

# Stage every old file before replacing any live file. Each temporary file
# lives beside its destination, so publication uses same-filesystem rename.
relay_tmp=$(mktemp "$config_dir/.ho3-relayd.XXXXXXXXXX")
gogios_tmp=$(mktemp "$config_dir/.ho3-gogios.XXXXXXXXXX")
cp -p "$work/previous.relayd.conf" "$relay_tmp"
cp -p "$work/previous.gogios.json" "$gogios_tmp"
if [ "$host" = fishfinger ]; then
    chain_tmp=$(mktemp "$ssl_dir/.ho3-chain.XXXXXXXXXX")
    key_tmp=$(mktemp "$ssl_dir/private/.ho3-key.XXXXXXXXXX")
    cp -p "$work/previous.fullchain.pem" "$chain_tmp"
    cp -p "$work/previous.key" "$key_tmp"
    mv -f "$key_tmp" "$ssl_dir/private/code.f3s.buetow.org.key"
    key_tmp=
    mv -f "$chain_tmp" "$ssl_dir/code.f3s.buetow.org.fullchain.pem"
    chain_tmp=
fi
mv -f "$relay_tmp" "$config_dir/relayd.conf"
relay_tmp=
mv -f "$gogios_tmp" "$config_dir/gogios.json"
gogios_tmp=

# Never activate an invalid restored configuration. Recovery can start a
# stopped daemon only when the preflight recorded that it was running.
timeout 30 relayd -n -f "$config_dir/relayd.conf" >/dev/null 2>&1 ||
    fail 'restored configuration did not validate; daemon was not reloaded'
running() {
    check_status=0
    timeout 10 rcctl check relayd >/dev/null 2>&1 || check_status=$?
    case "$check_status" in
        0) return 0 ;;
        1) return 1 ;;
        *) fail 'relayd status is unknown; refusing to start it' ;;
    esac
}
if running; then
    if ! timeout 30 rcctl reload relayd >/dev/null 2>&1; then
        # A refused reload with the old process still alive is an error,
        # not a reason to restart it. A dead process needs baseline recovery.
        if running; then
            fail 'relayd reload failed'
        fi
    fi
fi
if ! running; then
    [ -f "$work/relayd-was-running" ] && [ ! -L "$work/relayd-was-running" ] ||
        fail 'relayd is stopped and no running baseline was recorded'
    timeout 30 rcctl start relayd >/dev/null 2>&1 ||
        fail 'could not restore the previously running relayd'
    running || fail 'relayd did not stay running'
fi
printf 'Previous frontend configuration restored; private backups retained.\n'
