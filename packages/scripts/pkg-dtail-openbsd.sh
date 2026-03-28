#!/bin/sh
# Build and sign an OpenBSD dtail package from pre-compiled binaries and config files.
# Run on an OpenBSD host (e.g. fishfinger). Called by the Makefile via SSH.
# The signed .tgz is left in /tmp/dtail-pkg/out/ for the Makefile to retrieve.
#
# Arguments:
#   $1 — version (e.g. 4.3.2)

set -e

VERSION="$1"
NAME="dtail"
COMMENT="Distributed log tail and grep tool"
DESC="DTail is a distributed DevOps tool for tailing, grepping, catting, and
mapping across many remote machines at once via SSH."

WORKDIR="/tmp/${NAME}-pkg"
doas rm -rf "$WORKDIR"
mkdir -p "$WORKDIR/stage/usr/local/bin"
mkdir -p "$WORKDIR/stage/etc/dserver"
mkdir -p "$WORKDIR/stage/etc/rc.d"
mkdir -p "$WORKDIR/out"

# Place the pre-compiled binaries
for bin in dserver dcat dgrep dmap dtail dtailhealth; do
    cp "/tmp/dtail-binaries/${bin}" "$WORKDIR/stage/usr/local/bin/${bin}"
    chmod 755 "$WORKDIR/stage/usr/local/bin/${bin}"
done

# Place the key cache helper script
cp "/tmp/dtail-binaries/dserver-update-key-cache.sh" \
    "$WORKDIR/stage/usr/local/bin/dserver-update-key-cache.sh"
chmod 500 "$WORKDIR/stage/usr/local/bin/dserver-update-key-cache.sh"

# Place the config file
cp "/tmp/dtail-binaries/dtail.json" "$WORKDIR/stage/etc/dserver/dtail.json"
chmod 644 "$WORKDIR/stage/etc/dserver/dtail.json"

# Place the rc script
cp "/tmp/dtail-binaries/dserver.rc" "$WORKDIR/stage/etc/rc.d/dserver"
chmod 755 "$WORKDIR/stage/etc/rc.d/dserver"

# Packing list — all files with absolute paths
cat > "$WORKDIR/plist" <<'PLIST'
usr/local/bin/dserver
usr/local/bin/dcat
usr/local/bin/dgrep
usr/local/bin/dmap
usr/local/bin/dtail
usr/local/bin/dtailhealth
usr/local/bin/dserver-update-key-cache.sh
etc/dserver/dtail.json
etc/rc.d/dserver
PLIST

# Description file
printf '%s\n' "$DESC" > "$WORKDIR/desc"

# Build the package
doas pkg_create \
    -D COMMENT="$COMMENT" \
    -d "$WORKDIR/desc" \
    -f "$WORKDIR/plist" \
    -B "$WORKDIR/stage" \
    -p / \
    "$WORKDIR/out/${NAME}-${VERSION}.tgz"

# Sign with signify if the key exists
if [ -f /etc/signify/custom-pkg.sec ]; then
    mkdir -p "$WORKDIR/signed"
    doas pkg_sign -s signify2 -s /etc/signify/custom-pkg.sec \
        -o "$WORKDIR/signed" "$WORKDIR/out/${NAME}-${VERSION}.tgz"
    mv "$WORKDIR/signed/${NAME}-${VERSION}.tgz" "$WORKDIR/out/${NAME}-${VERSION}.tgz"
    rm -rf "$WORKDIR/signed"
    echo "Package signed with signify"
else
    echo "Warning: /etc/signify/custom-pkg.sec not found, package is unsigned"
fi

echo "OpenBSD package ${NAME}-${VERSION} built in $WORKDIR/out/"
