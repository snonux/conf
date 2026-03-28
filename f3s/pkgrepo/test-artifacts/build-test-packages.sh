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

    # Create the hello-test script under a staging root
    STAGEDIR="$WORKDIR/stage"
    mkdir -p "$STAGEDIR/usr/local/bin"
    cat > "$STAGEDIR/usr/local/bin/hello-test" <<'SCRIPT'
#!/bin/sh
echo "Hello from the custom FreeBSD package repo!"
SCRIPT
    chmod 755 "$STAGEDIR/usr/local/bin/hello-test"

    # Create the packing list
    cat > "$WORKDIR/plist" <<'PLIST'
bin/hello-test
PLIST

    # Create the package manifest with files reference
    cat > "$WORKDIR/+MANIFEST" <<MANIFEST
name: hello-test
version: "1.0"
origin: local/hello-test
comment: "Test package for the custom FreeBSD package repository"
desc: "A minimal hello-world package to verify the custom pkg repo works."
maintainer: "paul@buetow.org"
www: "https://buetow.org"
prefix: /usr/local
MANIFEST

    # Build the package using plist and staging directory
    mkdir -p "$OUTDIR/All"
    pkg create -M "$WORKDIR/+MANIFEST" -p "$WORKDIR/plist" -r "$STAGEDIR" -o "$OUTDIR/All"

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

    # Detect OpenBSD version for the repo path
    OSVER=$(uname -r)

    # Create the hello-test script under a staging root
    STAGEDIR="$WORKDIR/stage"
    mkdir -p "$STAGEDIR/usr/local/bin"
    cat > "$STAGEDIR/usr/local/bin/hello-test" <<'SCRIPT'
#!/bin/sh
echo "Hello from the custom OpenBSD package repo!"
SCRIPT
    chmod 755 "$STAGEDIR/usr/local/bin/hello-test"

    # Create the packing list
    cat > "$WORKDIR/packing-list" <<'PLIST'
usr/local/bin/hello-test
PLIST

    # Create the description file
    cat > "$WORKDIR/desc" <<'DESC'
A minimal hello-world package to verify the custom pkg repo works.
DESC

    # Build the package
    pkg_create \
        -D COMMENT="Test package for the custom OpenBSD package repository" \
        -d "$WORKDIR/desc" \
        -f "$WORKDIR/packing-list" \
        -B "$STAGEDIR" \
        -p / \
        "$OUTDIR/hello-test-1.0.tgz"

    # Sign with signify if the key exists
    if [ -f /etc/signify/custom-pkg.sec ]; then
        mkdir -p "$OUTDIR/signed"
        doas pkg_sign -s signify2 -s /etc/signify/custom-pkg.sec \
            -o "$OUTDIR/signed" "$OUTDIR/hello-test-1.0.tgz"
        mv "$OUTDIR/signed/hello-test-1.0.tgz" "$OUTDIR/hello-test-1.0.tgz"
        rm -rf "$OUTDIR/signed"
        echo "Package signed with signify"
    else
        echo "Warning: /etc/signify/custom-pkg.sec not found, package is unsigned"
    fi

    rm -rf "$WORKDIR"

    echo ""
    echo "OpenBSD package built in: $OUTDIR"
    echo "OpenBSD version detected: $OSVER"
    echo "Copy contents to the PV:"
    echo "  scp $OUTDIR/hello-test-1.0.tgz <nfs-host>:/data/nfs/k3svolumes/pkgrepo/openbsd/$OSVER/packages/amd64/"
}

case "$1" in
    freebsd) build_freebsd ;;
    openbsd) build_openbsd ;;
    *) usage ;;
esac
