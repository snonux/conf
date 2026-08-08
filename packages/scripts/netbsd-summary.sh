#!/bin/sh
# Regenerate pkg_summary.gz across EVERY package in the NetBSD repo.
# Run on a NetBSD host (pi0). Called by the Makefile via SSH.
#
# Why this is separate from the per-package build: pkg_summary describes a
# repository, not a package. Generating it from only the package just built
# silently drops every other package from the index -- which is exactly what
# happened while dtail was the only NetBSD package and the summary was written
# by pkg-dtail-netbsd.sh. The moment a second package existed, publishing
# either one would have hidden the other from pkgin.
#
# The Makefile stages every .tgz currently in the repo into $INDIR before
# calling this, so the summary always reflects the repository as a whole.
#
# Arguments:
#   $1 — directory holding every published .tgz (default /tmp/netbsd-repo-pkgs)

set -e

PATH=/usr/sbin:/usr/bin:/bin:$PATH
export PATH

INDIR="${1:-/tmp/netbsd-repo-pkgs}"

if [ ! -d "$INDIR" ]; then
    echo "Error: $INDIR does not exist" >&2
    exit 1
fi

cd "$INDIR"

# Guard against publishing an empty index: that would make pkgin believe the
# repository has no packages at all.
count=$(ls -1 ./*.tgz 2>/dev/null | wc -l | tr -d ' ')
if [ "$count" -eq 0 ]; then
    echo "Error: no .tgz files in $INDIR; refusing to write an empty summary" >&2
    exit 1
fi

# Write to a temp file first: piping pkg_info straight into gzip would mask a
# pkg_info failure (set -e does not cover the left side of a pipe here) and
# publish a truncated summary.
pkg_info -X ./*.tgz > pkg_summary.tmp
gzip -9 < pkg_summary.tmp > pkg_summary.gz
rm -f pkg_summary.tmp

echo "pkg_summary.gz regenerated from $count package(s) in $INDIR"
