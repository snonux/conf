#!/bin/sh
# Build a NetBSD dtail package from pre-compiled binaries.
# Run on a NetBSD host (e.g. pi0). Called by the Makefile via SSH.
# The .tgz and a matching pkg_summary.gz are left in /tmp/dtail-netbsd-pkg/out/
# for the Makefile to retrieve.
#
# Note: the pkg_summary.gz only describes the packages built here. If the
# NetBSD repo ever holds more than dtail, regenerate the summary across all
# .tgz files in the repo directory instead.
#
# The package deliberately ships no install script for the dserver user/group:
# creating them is a documented one-time install step (see the pkgrepo skill's
# dtail-package.md), matching how the FreeBSD package handles it.
#
# Arguments:
#   $1 — version (NetBSD-safe, no dashes — e.g. 4.3.2ng)

set -e

# Non-interactive SSH shells on NetBSD lack /usr/sbin in PATH (pkg_* live there)
PATH=/usr/sbin:/usr/bin:/bin:$PATH
export PATH

VERSION="$1"
if [ -z "$VERSION" ]; then
    echo "Error: version argument missing (would build dtail-.tgz)" >&2
    exit 1
fi
NAME="dtail"
COMMENT="Distributed log tail and grep tool"
DESC="DTail is a distributed DevOps tool for tailing, grepping, catting, and
mapping across many remote machines at once via SSH."

WORKDIR="/tmp/${NAME}-netbsd-pkg"
rm -rf "$WORKDIR"
mkdir -p \
    "$WORKDIR/stage/usr/local/bin" \
    "$WORKDIR/stage/etc/dserver" \
    "$WORKDIR/stage/etc/rc.d" \
    "$WORKDIR/out"

# Binaries (cross-compiled linux→netbsd/arm64 with nozstd; .zst logs not supported)
for bin in dserver dcat dgrep dmap dtail dtailhealth; do
    cp "/tmp/dtail-netbsd-binaries/${bin}" "$WORKDIR/stage/usr/local/bin/${bin}"
    chmod 755 "$WORKDIR/stage/usr/local/bin/${bin}"
done

# Key cache helper (sh-compatible; walks /home/ on NetBSD)
cp "/tmp/dtail-netbsd-binaries/dserver-update-key-cache.sh" \
    "$WORKDIR/stage/usr/local/bin/dserver-update-key-cache.sh"
chmod 555 "$WORKDIR/stage/usr/local/bin/dserver-update-key-cache.sh"

# Config (absolute CacheDir/HostKeyFile paths — rc.d starts daemons with cwd /)
cp "/tmp/dtail-netbsd-binaries/dtail.json" "$WORKDIR/stage/etc/dserver/dtail.json"
chmod 644 "$WORKDIR/stage/etc/dserver/dtail.json"

# rc.d script (NetBSD rc.subr style)
cp "/tmp/dtail-netbsd-binaries/dserver.rc" "$WORKDIR/stage/etc/rc.d/dserver"
chmod 755 "$WORKDIR/stage/etc/rc.d/dserver"

# Packing list — paths relative to the / install prefix. @owner/@group make
# pkg_add install root-owned files even though staging happens as paul.
cat > "$WORKDIR/plist" <<'PLIST'
@owner root
@group wheel
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

# Comment and description files
printf '%s\n' "$COMMENT" > "$WORKDIR/comment"
printf '%s\n' "$DESC" > "$WORKDIR/desc"

# Build info — pkg_add checks these against the target host
cat > "$WORKDIR/build-info" <<BUILDINFO
MACHINE_ARCH=$(uname -p)
OPSYS=NetBSD
OS_VERSION=$(uname -r)
PKGTOOLS_VERSION=$(pkg_info -V)
BUILDINFO

# Build the package: files are staged under $WORKDIR/stage (-p) but install
# relative to / (-I), mirroring the OpenBSD package layout.
pkg_create \
    -B "$WORKDIR/build-info" \
    -c "$WORKDIR/comment" \
    -d "$WORKDIR/desc" \
    -f "$WORKDIR/plist" \
    -I / \
    -p "$WORKDIR/stage" \
    "$WORKDIR/out/${NAME}-${VERSION}.tgz"

# Repo metadata for pkgin (pkg_add itself doesn't need it). Write the summary
# to a temp file first: piping pkg_info straight into gzip would mask a
# pkg_info failure (set -e without pipefail) and publish a truncated summary.
( cd "$WORKDIR/out" && \
    pkg_info -X ./*.tgz > pkg_summary.tmp && \
    gzip -9 < pkg_summary.tmp > pkg_summary.gz && \
    rm pkg_summary.tmp )

echo "NetBSD package ${NAME}-${VERSION} built in $WORKDIR/out/"
