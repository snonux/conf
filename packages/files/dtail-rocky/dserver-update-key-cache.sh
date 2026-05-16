#!/usr/bin/env bash

set -euo pipefail

declare -r CACHEDIR=/var/run/dserver/cache
declare -r DSERVER_USER=dserver
declare -r DSERVER_GROUP=dserver

cache_keys() {
    local remoteuser=$1
    local home_dir=$2
    local keysfile=$home_dir/.ssh/authorized_keys
    local cachefile=$CACHEDIR/$remoteuser.authorized_keys

    if [[ -f "$keysfile" ]]; then
        echo "Caching $keysfile -> $cachefile"
        cp "$keysfile" "$cachefile"
        chown "$DSERVER_USER:$DSERVER_GROUP" "$cachefile"
        chmod 600 "$cachefile"
    fi
}

expected_key_path() {
    local remoteuser=$1

    if [[ "$remoteuser" == "root" ]]; then
        printf '%s\n' /root/.ssh/authorized_keys
        return
    fi

    printf '/home/%s/.ssh/authorized_keys\n' "$remoteuser"
}

echo "Updating SSH key cache"

mkdir -p "$CACHEDIR"

cache_keys root /root

while IFS= read -r remoteuser; do
    cache_keys "$remoteuser" "/home/$remoteuser"
done < <(find /home -mindepth 1 -maxdepth 1 -type d -printf '%f\n' | sort)

find "$CACHEDIR" -name '*.authorized_keys' -type f | while read -r cachefile; do
    remoteuser=$(basename "$cachefile" | cut -d. -f1)
    keysfile=$(expected_key_path "$remoteuser")

    if [[ ! -f "$keysfile" ]]; then
        echo "Deleting obsolete cache file $cachefile"
        rm -f "$cachefile"
    fi
done

echo "All set..."
