#!/bin/bash

# Create user and group if they don't exist
if ! id -u minvid &>/dev/null; then
    addgroup -g ${PGID} minvid
    adduser -D -u ${PUID} -G minvid minvid
fi

# Ensure data directory exists and has correct permissions
mkdir -p /app/data
chown -R minvid:minvid /app/data

# Execute the main command as minvid user
exec su-exec minvid "$@"
