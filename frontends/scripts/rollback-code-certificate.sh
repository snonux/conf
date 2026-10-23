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
install -o root -g wheel -m 0644 "$work/previous.relayd.conf" "$relay_tmp"
install -o root -g wheel -m 0744 "$work/previous.gogios.json" "$gogios_tmp"
if [ "$host" = fishfinger ]; then
    chain_tmp=$(mktemp "$ssl_dir/.ho3-chain.XXXXXXXXXX")
    key_tmp=$(mktemp "$ssl_dir/private/.ho3-key.XXXXXXXXXX")
    install -o root -g wheel -m 0644 "$work/previous.fullchain.pem" "$chain_tmp"
    install -o root -g wheel -m 0600 "$work/previous.key" "$key_tmp"
    mv -f "$key_tmp" "$ssl_dir/private/code.f3s.buetow.org.key"
    key_tmp=
    mv -f "$chain_tmp" "$ssl_dir/code.f3s.buetow.org.fullchain.pem"
    chain_tmp=
fi
mv -f "$relay_tmp" "$config_dir/relayd.conf"
relay_tmp=
mv -f "$gogios_tmp" "$config_dir/gogios.json"
gogios_tmp=

# Never reload an invalid restored configuration or start a stopped service.
relayd -n -f "$config_dir/relayd.conf" >/dev/null 2>&1 ||
    fail 'restored configuration did not validate; daemon was not reloaded'
rcctl check relayd >/dev/null 2>&1 || fail 'relayd is not running'
rcctl reload relayd >/dev/null 2>&1 || fail 'relayd reload failed'
printf 'Previous frontend configuration restored; private backups retained.\n'
