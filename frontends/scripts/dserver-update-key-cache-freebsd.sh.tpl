#!/bin/sh
# Refresh the dserver SSH key cache from user authorized_keys files.
# Called by /usr/local/etc/periodic/daily/200.dserver-update-key-cache.

CACHEDIR=/var/run/dserver/cache
DSERVER_USER=dserver
DSERVER_GROUP=dserver

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
