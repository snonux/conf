#!/bin/sh
# Build a NetBSD package from a single pre-compiled Go binary.
# Run on a NetBSD host (pi0) -- pkg_create needs a matching NetBSD.
# Called by the Makefile via SSH. The .tgz is left in /tmp/<name>-netbsd-pkg/out/
# for the Makefile to retrieve.
#
# This is the generic counterpart to pkg-freebsd.sh and pkg-openbsd.sh; the
# multi-binary DTail package has its own script (pkg-dtail-netbsd.sh).
#
# Deliberately does NOT write pkg_summary.gz. The summary must describe every
# package in the repository, not just the one built here, so it is regenerated
# separately by the Makefile's netbsd-summary target after the upload. Writing
# a single-package summary here is what used to silently drop dtail from the
# repo index whenever anything else was published.
#
# Arguments:
#   $1 — package name
#   $2 — version (NetBSD-safe, no dashes — e.g. 0.1.0)
#   $3 — one-line comment
#   $4 — longer description

set -e

# Non-interactive SSH shells on NetBSD lack /usr/sbin in PATH (pkg_* live there)
PATH=/usr/sbin:/usr/bin:/bin:$PATH
export PATH

NAME="$1"
VERSION="$2"
COMMENT="$3"
DESC="$4"

if [ -z "$NAME" ] || [ -z "$VERSION" ]; then
    echo "Error: name and version are required (would build '-.tgz')" >&2
    exit 1
fi

case "$VERSION" in
*-*)
    # A dash separates name from version in pkg naming, so pkg_add would
    # misparse the result.
    echo "Error: NetBSD package versions must not contain a dash: $VERSION" >&2
    exit 1
    ;;
esac

WORKDIR="/tmp/${NAME}-netbsd-pkg"
rm -rf "$WORKDIR"
mkdir -p "$WORKDIR/stage/usr/local/bin" "$WORKDIR/out"

cp "/tmp/${NAME}-netbsd" "$WORKDIR/stage/usr/local/bin/${NAME}"
chmod 555 "$WORKDIR/stage/usr/local/bin/${NAME}"

# Packing list — paths relative to the / install prefix. @owner/@group make
# pkg_add install root-owned files even though staging happens as an ordinary
# user.
cat > "$WORKDIR/plist" <<PLIST
@owner root
@group wheel
usr/local/bin/${NAME}
PLIST

# f3sctl is additionally exposed to bozohttpd as a CGI. bozohttpd only executes
# programs inside its cgibin directory, and that directory is deliberately kept
# outside /var/www/html so the hourly pi0->pi1 content rsync never touches the
# binary and it can never appear in a directory index.
#
# A symlink, not a second copy. The CGI is the half that actually runs power
# jobs, and while pkg_add updated both copies happily enough, two independent
# 9MB files at two paths are two things that can disagree -- and on
# 2026-08-11 they did: a hand-installed binary in /usr/local/bin left the CGI
# on the previous release, so a whole rack shutdown silently ran the old code
# and the run had to be repeated. One inode cannot drift from itself, and it
# halves what the package costs on a Pi.
#
# The link is relative so it stays correct wherever the / prefix is staged,
# and it is not dangling inside the staging tree either: ../../bin/f3sctl
# resolves to the copy staged above.
#
# bozohttpd executes it without complaint -- verified on pi0 against the live
# server, which answered f3sctl's own 401 JSON body through the symlink.
if [ "$NAME" = "f3sctl" ]; then
    mkdir -p "$WORKDIR/stage/usr/local/libexec/cgi-bin"
    ln -sf "../../bin/${NAME}" "$WORKDIR/stage/usr/local/libexec/cgi-bin/${NAME}"
    echo "usr/local/libexec/cgi-bin/${NAME}" >> "$WORKDIR/plist"
fi

printf '%s\n' "${COMMENT:-$NAME}" > "$WORKDIR/comment"
printf '%s\n' "${DESC:-$NAME}" > "$WORKDIR/desc"

# Build info — pkg_add checks these against the target host
cat > "$WORKDIR/build-info" <<BUILDINFO
MACHINE_ARCH=$(uname -p)
OPSYS=NetBSD
OS_VERSION=$(uname -r)
PKGTOOLS_VERSION=$(pkg_info -V)
BUILDINFO

# Files are staged under $WORKDIR/stage (-p) but install relative to / (-I),
# mirroring the FreeBSD and OpenBSD package layouts.
pkg_create \
    -B "$WORKDIR/build-info" \
    -c "$WORKDIR/comment" \
    -d "$WORKDIR/desc" \
    -f "$WORKDIR/plist" \
    -I / \
    -p "$WORKDIR/stage" \
    "$WORKDIR/out/${NAME}-${VERSION}.tgz"

echo "NetBSD package ${NAME}-${VERSION} built in $WORKDIR/out/"
