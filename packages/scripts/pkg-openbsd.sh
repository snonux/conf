#!/bin/sh
# Build and sign an OpenBSD package from a pre-compiled binary.
# Run on an OpenBSD host (e.g. fishfinger). Called by the Makefile via SSH.
# The signed .tgz is left in /tmp/<name>-pkg/out/ for the Makefile to retrieve.
#
# Arguments:
#   $1 — package name (e.g. gogios)
#   $2 — version (e.g. 1.4.1)
#   $3 — one-line comment
#   $4 — description

set -e

NAME="$1"
VERSION="$2"
COMMENT="$3"
DESC="$4"

WORKDIR="/tmp/${NAME}-pkg"
doas rm -rf "$WORKDIR"
mkdir -p "$WORKDIR/stage/usr/local/bin" "$WORKDIR/out"

# Place the pre-compiled binary
cp "/tmp/${NAME}" "$WORKDIR/stage/usr/local/bin/${NAME}"
chmod 755 "$WORKDIR/stage/usr/local/bin/${NAME}"

# Packing list
printf 'usr/local/bin/%s\n' "$NAME" > "$WORKDIR/plist"

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
