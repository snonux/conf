#!/bin/ksh
# Refresh the dserver SSH key cache from user authorized_keys files.
# OpenBSD variant: called from the dserver rc.d rc_pre (because /var/run is
# wiped by /etc/rc at boot) and from a daily /etc/daily.local entry added by
# the Rex 'dtail' task — see the pkgrepo skill's dtail-package.md.

CACHEDIR=/var/run/dserver/cache
DSERVER_USER=_dserver
DSERVER_GROUP=_dserver

echo 'Updating SSH key cache'

ls /home/ | while read remoteuser; do
    keysfile="/home/$remoteuser/.ssh/authorized_keys"

    if [ -f "$keysfile" ]; then
        cachefile="$CACHEDIR/$remoteuser.authorized_keys"
        echo "Caching $keysfile -> $cachefile"

        cp "$keysfile" "$cachefile"
        chown "$DSERVER_USER:$DSERVER_GROUP" "$cachefile"
        chmod 600 "$cachefile"
    fi
done

# Remove stale cache entries for users whose authorized_keys no longer exist
find "$CACHEDIR" -name '*.authorized_keys' -type f | while read cachefile; do
    remoteuser=$(basename "$cachefile" .authorized_keys)
    if [ ! -f "/home/$remoteuser/.ssh/authorized_keys" ]; then
        echo "Deleting obsolete cache file $cachefile"
        rm "$cachefile"
    fi
done

echo 'All set...'
