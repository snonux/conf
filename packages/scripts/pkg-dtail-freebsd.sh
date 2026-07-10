#!/bin/sh
# Build a FreeBSD dtail package from pre-compiled binaries and upload to the repo PV.
# Run on f0 via SSH. Called by the Makefile.
#
# Arguments:
#   $1 — version (e.g. 4.3.2-ng)
#   $2 — PV destination (e.g. /data/nfs/k3svolumes/pkgrepo/freebsd/FreeBSD:15:amd64/latest)

set -e

VERSION="$1"
if [ -z "$VERSION" ]; then
    echo "Error: version argument missing (would build dtail-.tgz)" >&2
    exit 1
fi
PV_DEST="$2"
NAME="dtail"
COMMENT="Distributed log tail and grep tool"
DESC="DTail is a distributed DevOps tool for tailing, grepping, catting, and mapping across many remote machines at once via SSH."
MAINTAINER="paul@buetow.org"
WWW="https://codeberg.org/snonux/dtail"

WORKDIR="/tmp/${NAME}-freebsd-pkg"
rm -rf "$WORKDIR"
mkdir -p \
    "$WORKDIR/stage/usr/local/bin" \
    "$WORKDIR/stage/usr/local/etc/rc.d" \
    "$WORKDIR/stage/usr/local/etc/dserver" \
    "$WORKDIR/out/All"

# Binaries (cross-compiled linux→freebsd with nozstd; .zst logs not supported)
for bin in dserver dcat dgrep dmap dtail dtailhealth; do
    cp "/tmp/dtail-freebsd-binaries/${bin}" "$WORKDIR/stage/usr/local/bin/${bin}"
    chmod 755 "$WORKDIR/stage/usr/local/bin/${bin}"
done

# Key cache helper (sh-compatible; walks /home/ on FreeBSD)
cp "/tmp/dtail-freebsd-binaries/dserver-update-key-cache.sh" \
    "$WORKDIR/stage/usr/local/bin/dserver-update-key-cache.sh"
chmod 555 "$WORKDIR/stage/usr/local/bin/dserver-update-key-cache.sh"

# rc.d script (FreeBSD rc.subr style; config at /usr/local/etc/dserver/dtail.json)
cp "/tmp/dtail-freebsd-binaries/dserver.rc" \
    "$WORKDIR/stage/usr/local/etc/rc.d/dserver"
chmod 555 "$WORKDIR/stage/usr/local/etc/rc.d/dserver"

# Config
cp "/tmp/dtail-freebsd-binaries/dtail.json" \
    "$WORKDIR/stage/usr/local/etc/dserver/dtail.json"
chmod 644 "$WORKDIR/stage/usr/local/etc/dserver/dtail.json"

# Packing list — paths relative to /usr/local prefix
cat > "$WORKDIR/plist" <<'PLIST'
bin/dserver
bin/dcat
bin/dgrep
bin/dmap
bin/dtail
bin/dtailhealth
bin/dserver-update-key-cache.sh
etc/rc.d/dserver
etc/dserver/dtail.json
PLIST

# Package manifest (UCL)
cat > "$WORKDIR/+MANIFEST" <<MANIFEST
name: "${NAME}"
version: "${VERSION}"
origin: "local/${NAME}"
comment: "${COMMENT}"
desc: "${DESC}"
maintainer: "${MAINTAINER}"
www: "${WWW}"
prefix: /usr/local
MANIFEST

# Build package, regenerate repo metadata, copy to PV
doas pkg create -M "$WORKDIR/+MANIFEST" -p "$WORKDIR/plist" -r "$WORKDIR/stage" -o "$WORKDIR/out/All"
doas pkg repo "$WORKDIR/out"
doas cp -Rf "$WORKDIR/out/"* "$PV_DEST/"

rm -rf "$WORKDIR" /tmp/dtail-freebsd-binaries /tmp/pkg-dtail-freebsd.sh
echo "FreeBSD package ${NAME}-${VERSION} uploaded to repo"
