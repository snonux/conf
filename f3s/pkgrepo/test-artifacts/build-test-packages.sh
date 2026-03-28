#!/bin/sh
# Build a minimal hello-test package for FreeBSD or OpenBSD.
# Run on the target OS: ./build-test-packages.sh freebsd|openbsd
# The output directory will contain the package ready to copy to the repo PV.

set -e

usage() {
    echo "Usage: $0 freebsd|openbsd"
    exit 1
}

build_freebsd() {
    WORKDIR=$(mktemp -d)
    OUTDIR="$PWD/freebsd-output"
    mkdir -p "$OUTDIR"

    # Create the hello-test script
    mkdir -p "$WORKDIR/usr/local/bin"
    cat > "$WORKDIR/usr/local/bin/hello-test" <<'SCRIPT'
#!/bin/sh
echo "Hello from the custom FreeBSD package repo!"
SCRIPT
    chmod 755 "$WORKDIR/usr/local/bin/hello-test"

    # Create the package manifest
    cat > "$WORKDIR/+MANIFEST" <<'MANIFEST'
name: hello-test
version: "1.0"
origin: local/hello-test
comment: "Test package for the custom FreeBSD package repository"
desc: "A minimal hello-world package to verify the custom pkg repo works."
maintainer: "paul@buetow.org"
www: "https://buetow.org"
prefix: /usr/local
MANIFEST

    # Create the packing list
    cat > "$WORKDIR/+COMPACT_MANIFEST" <<'MANIFEST'
name: hello-test
version: "1.0"
origin: local/hello-test
comment: "Test package for the custom FreeBSD package repository"
desc: "A minimal hello-world package to verify the custom pkg repo works."
maintainer: "paul@buetow.org"
www: "https://buetow.org"
prefix: /usr/local
MANIFEST

    # Build the package
    mkdir -p "$OUTDIR/All"
    pkg create -M "$WORKDIR/+MANIFEST" -r "$WORKDIR" -o "$OUTDIR/All"

    # Generate the unsigned repo metadata
    pkg repo "$OUTDIR"

    rm -rf "$WORKDIR"

    echo ""
    echo "FreeBSD package built in: $OUTDIR"
    echo "Copy contents to the PV:"
    echo "  scp -r $OUTDIR/* <nfs-host>:/data/nfs/k3svolumes/pkgrepo/freebsd/FreeBSD:15:amd64/latest/"
}

build_openbsd() {
    WORKDIR=$(mktemp -d)
    OUTDIR="$PWD/openbsd-output"
    mkdir -p "$OUTDIR"

    # Create the hello-test script
    mkdir -p "$WORKDIR/usr/local/bin"
    cat > "$WORKDIR/usr/local/bin/hello-test" <<'SCRIPT'
#!/bin/sh
echo "Hello from the custom OpenBSD package repo!"
SCRIPT
    chmod 755 "$WORKDIR/usr/local/bin/hello-test"

    # Create the packing list
    cat > "$WORKDIR/packing-list" <<'PLIST'
@name hello-test-1.0
@comment Test package for the custom OpenBSD package repository
@bin usr/local/bin/hello-test
PLIST

    # Build the package
    cd "$WORKDIR"
    pkg_create -d "A minimal hello-world package to verify the custom pkg repo works." \
        -f "$WORKDIR/packing-list" \
        -B "$WORKDIR" \
        -p / \
        "$OUTDIR/hello-test-1.0.tgz"

    rm -rf "$WORKDIR"

    echo ""
    echo "OpenBSD package built in: $OUTDIR"
    echo "Copy contents to the PV:"
    echo "  scp $OUTDIR/hello-test-1.0.tgz <nfs-host>:/data/nfs/k3svolumes/pkgrepo/openbsd/7.7/packages/amd64/"
}

case "$1" in
    freebsd) build_freebsd ;;
    openbsd) build_openbsd ;;
    *) usage ;;
esac
