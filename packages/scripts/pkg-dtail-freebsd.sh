#!/bin/sh
# Assemble without privilege/install/publication; publish is an explicit action.
set -eu

fail() { printf '%s\n' "$*" >&2; exit 1; }

assemble() {
    bundle=$1
    output=$2
    test -d "$bundle" && test ! -L "$bundle" || fail 'Invalid bundle directory'
    test -f "$bundle/SHA256" && test ! -L "$bundle/SHA256" || fail 'Missing bundle checksums'
    expected='dserver dcat dgrep dmap dtail dtailhealth dtail.json.sample dserver-update-key-cache.sh dserver.rc build-info.json +MANIFEST'
    count=0
    while read -r digest name extra; do
        case " $expected " in *" $name "*) ;; *) fail 'Unexpected bundle member';; esac
        test -n "$name" && test -z "$extra" || fail 'Invalid checksum record'
        test ! -L "$bundle/$name" && test -f "$bundle/$name" || fail "Unsafe bundle member $name"
        test "$(sha256 -q "$bundle/$name")" = "$digest" || fail "Checksum mismatch: $name"
        count=$((count+1))
    done < "$bundle/SHA256"
    test "$count" -eq 11 || fail 'Incomplete bundle checksums'
    # Each required member must occur exactly once, not just any eleven records.
    for name in $expected; do
        test "$(awk -v name="$name" '$2 == name {n++} END {print n+0}' "$bundle/SHA256")" -eq 1 || fail "Missing/duplicate bundle member $name"
    done
    mkdir -m 700 "$output" || fail 'Output must be a new directory'
    work=$(mktemp -d /tmp/dtail-freebsd-pkg.XXXXXXXXXX)
    trap 'rm -rf "$work"' EXIT
    trap 'exit 1' HUP INT TERM
    mkdir -p "$work/stage/usr/local/bin" "$work/stage/usr/local/etc/rc.d" \
        "$work/stage/usr/local/etc/dserver" "$work/stage/usr/local/share/doc/dtail" \
        "$work/meta"
    for bin in dserver dcat dgrep dmap dtail dtailhealth; do
        install -m 755 "$bundle/$bin" "$work/stage/usr/local/bin/$bin"
    done
    install -m 755 "$bundle/dserver-update-key-cache.sh" "$work/stage/usr/local/bin/dserver-update-key-cache.sh"
    install -m 755 "$bundle/dserver.rc" "$work/stage/usr/local/etc/rc.d/dserver"
    install -m 644 "$bundle/dtail.json.sample" "$work/stage/usr/local/etc/dserver/dtail.json.sample"
    install -m 644 "$bundle/build-info.json" "$work/stage/usr/local/share/doc/dtail/build-info.json"
    cp "$bundle/+MANIFEST" "$work/meta/+MANIFEST"
    cat > "$work/plist" <<'PLIST'
@owner root
@group wheel
bin/dserver
bin/dcat
bin/dgrep
bin/dmap
bin/dtail
bin/dtailhealth
bin/dserver-update-key-cache.sh
etc/rc.d/dserver
etc/dserver/dtail.json.sample
share/doc/dtail/build-info.json
PLIST
    pkg create -m "$work/meta" -p "$work/plist" -r "$work/stage" -o "$output"
}

publish() {
    artifact=$1
    destination=$2
    test ! -L "$artifact" && test -f "$artifact" || fail 'Invalid artifact'
    test -d "$destination" && test ! -L "$destination" || fail 'Repository must already exist'
    test -d "$destination/All" && test ! -L "$destination/All" || fail 'Repository All directory must already exist'
    name=$(pkg query -F "$artifact" '%n')
    version=$(pkg query -F "$artifact" '%v')
    test "$name" = dtail || fail 'Not a DTail package'
    case "$version" in ''|*[!A-Za-z0-9.+_-]*) fail 'Invalid package version';; esac
    case "$version" in *_*) ;; *) fail 'An explicit package revision is required';; esac
    revision=${version##*_}
    case "$revision" in ''|0*|*[!0-9]*) fail 'Invalid package revision';; esac
    final="$destination/All/dtail-$version.pkg"
    test ! -e "$final" && test ! -L "$final" || fail 'This package version is already published; increment DTAIL_FREEBSD_REVISION'
    lock="$destination/.dtail-publish.lock"
    mkdir "$lock" || fail 'Another DTail publication is running'
    temporary=''
    trap 'test -z "$temporary" || rm -f "$temporary"; rmdir "$lock"' EXIT
    trap 'exit 1' HUP INT TERM
    temporary=$(mktemp "$destination/All/.dtail-publish.XXXXXXXXXX")
    cp "$artifact" "$temporary"
    test "$(sha256 -q "$artifact")" = "$(sha256 -q "$temporary")" || fail 'Copied artifact checksum mismatch'
    chmod 644 "$temporary"
    # Exclusive same-filesystem publication; never overwrite an existing version.
    ln "$temporary" "$final" || fail 'Package version appeared during publication'
    rm "$temporary"
    temporary=''
    # Index the entire existing repository, retaining all unrelated packages.
    pkg repo "$destination"
    printf 'Published %s\n' "$final"
}

case "${1:-}" in
    assemble) test "$#" -eq 3 || fail 'Usage: assemble BUNDLE NEW_OUTPUT_DIR'; assemble "$2" "$3";;
    publish) test "$#" -eq 3 || fail 'Usage: publish ARTIFACT REPOSITORY'; publish "$2" "$3";;
    *) fail 'Usage: pkg-dtail-freebsd.sh assemble|publish ...';;
esac
