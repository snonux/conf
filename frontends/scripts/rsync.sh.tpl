#!/bin/sh

set -eu
PATH=/usr/local/bin:/usr/bin:/bin
export PATH
umask 022

# Only the dedicated account's cron may receive this third-party tree.
[ "$(hostname -s)" = fishfinger ] || exit 0
if [ "$(id -u)" -eq 0 ] || [ "$(id -un)" != _joernsync ]; then
    echo "rsync.sh: run as _joernsync, never root" >&2
    exit 1
fi

# Do not preserve sender ownership, group, mode, devices or special files.
# Pin IPv4 to the source allowlist and retain only links inside the tree.
exec rsync -4 -rlt --safe-links --delete --timeout=120 --contimeout=15 \
    rsync://192.168.2.110/joernshtdocs/ /var/www/htdocs/joern/
