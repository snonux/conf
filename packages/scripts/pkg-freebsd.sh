#!/bin/sh
# Build a FreeBSD package from a pre-compiled binary and publish it to the
# repo PV. Runs on the storage host (f0), called by the Makefile via SSH from
# a private job directory that holds this script, freebsd-repo.sh and the
# binary (named after the package).
#
# Publication goes through freebsd-repo.sh: it places the package in All/
# and regenerates the catalog over EVERY package in the repository. Running
# `pkg repo` over the work directory instead (as this script did until h54)
# produced a catalog listing only the new package, and copying it over the
# repository hid dtail and gogios from the f-hosts.
#
# Arguments:
#   $1 — package name (e.g. gogios)
#   $2 — version (e.g. 1.4.1)
#   $3 — one-line comment
#   $4 — description
#   $5 — maintainer email
#   $6 — project URL
#   $7 — repository path (e.g. /data/nfs/k3svolumes/pkgrepo/freebsd/FreeBSD:15:amd64/latest)

set -eu

fail() { printf 'pkg-freebsd: %s\n' "$*" >&2; exit 1; }

test "$#" -eq 7 || fail 'Usage: pkg-freebsd.sh NAME VERSION COMMENT DESC MAINTAINER WWW REPOSITORY'
NAME="$1"
VERSION="$2"
COMMENT="$3"
DESC="$4"
MAINTAINER="$5"
WWW="$6"
PV_DEST="$7"

# NAME and VERSION end up in paths and in the package file name.
case "$NAME" in ''|.*|*[!A-Za-z0-9._+-]*) fail "Invalid package name: $NAME";; esac
case "$VERSION" in ''|*[!A-Za-z0-9._+-]*) fail "Invalid version: $VERSION";; esac

JOB=$(dirname -- "$0")
BINARY="$JOB/$NAME"
REPO_TOOL="$JOB/freebsd-repo.sh"
test -f "$BINARY" || fail "Missing binary $BINARY"
test -f "$REPO_TOOL" || fail "Missing $REPO_TOOL (copy it next to this script)"

# A fresh work directory inside the private job directory; the Makefile
# removes the whole job directory after a successful run.
WORKDIR=$(mktemp -d "$JOB/work.XXXXXXXXXX")
mkdir -p "$WORKDIR/stage/usr/local/bin" "$WORKDIR/out"

# Place the pre-compiled binary
cp "$BINARY" "$WORKDIR/stage/usr/local/bin/${NAME}"
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

# Build the package unprivileged: without @owner/@group in the plist pkg
# create records root:wheel anyway (checked on f0, pkg 2.8.4). Only the
# publication below needs root, to write the root-owned repository.
pkg create -M "$WORKDIR/+MANIFEST" -p "$WORKDIR/plist" -r "$WORKDIR/stage" -o "$WORKDIR/out"
ARTIFACT="$WORKDIR/out/${NAME}-${VERSION}.pkg"
test -f "$ARTIFACT" || fail "pkg create did not produce $ARTIFACT"

# Place it in All/ and re-index the whole repository
doas /bin/sh "$REPO_TOOL" add "$ARTIFACT" "$PV_DEST"
echo "FreeBSD package ${NAME}-${VERSION} published to repo"
