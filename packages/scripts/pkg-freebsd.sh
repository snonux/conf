#!/bin/sh
# Build a FreeBSD package from a pre-compiled binary and upload to the repo PV.
# Run on a FreeBSD host (e.g. f0). Called by the Makefile via SSH.
#
# Arguments:
#   $1 — package name (e.g. gogios)
#   $2 — version (e.g. 1.4.1)
#   $3 — one-line comment
#   $4 — description
#   $5 — maintainer email
#   $6 — project URL
#   $7 — PV destination path (e.g. /data/nfs/k3svolumes/pkgrepo/freebsd/FreeBSD:15:amd64/latest)

set -e

NAME="$1"
VERSION="$2"
COMMENT="$3"
DESC="$4"
MAINTAINER="$5"
WWW="$6"
PV_DEST="$7"

WORKDIR="/tmp/${NAME}-pkg"
rm -rf "$WORKDIR"
mkdir -p "$WORKDIR/stage/usr/local/bin" "$WORKDIR/out/All"

# Place the pre-compiled binary
cp "/tmp/${NAME}" "$WORKDIR/stage/usr/local/bin/${NAME}"
chmod 755 "$WORKDIR/stage/usr/local/bin/${NAME}"

# Packing list — files relative to prefix
printf 'bin/%s\n' "$NAME" > "$WORKDIR/plist"

# Package manifest
cat > "$WORKDIR/+MANIFEST" <<MANIFEST
name: ${NAME}
version: "${VERSION}"
origin: local/${NAME}
comment: "${COMMENT}"
desc: "${DESC}"
maintainer: "${MAINTAINER}"
www: "${WWW}"
prefix: /usr/local
MANIFEST

# Build and regenerate repo metadata
doas pkg create -M "$WORKDIR/+MANIFEST" -p "$WORKDIR/plist" -r "$WORKDIR/stage" -o "$WORKDIR/out/All"
doas pkg repo "$WORKDIR/out"
doas cp -Rf "$WORKDIR/out/"* "$PV_DEST/"

# Clean up
rm -rf "$WORKDIR" "/tmp/${NAME}" "/tmp/pkg-freebsd.sh"
echo "FreeBSD package ${NAME}-${VERSION} uploaded to repo"
