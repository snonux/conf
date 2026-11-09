#!/bin/sh
# Native regression test for the FreeBSD repo catalog (task h54): the catalog
# must always list every package in All/, never just the one just published.
#
# Needs FreeBSD with pkg(8); run it as an ordinary user, e.g. on f0:
#   job=$(ssh -p 22 f0.lan.buetow.org 'mktemp -d /tmp/repo-test.XXXXXXXXXX')
#   scp -P 22 packages/scripts/freebsd-repo.sh packages/scripts/pkg-freebsd.sh \
#       packages/tests/test_freebsd_repo.sh "f0.lan.buetow.org:$job/"
#   ssh -p 22 f0.lan.buetow.org "sh $job/test_freebsd_repo.sh $job; rm -rf $job"
#
# Argument: the directory holding freebsd-repo.sh and pkg-freebsd.sh.
# Everything happens in a private mktemp workspace; no live repository is
# touched. The pkg-freebsd.sh case runs the production path, so it needs
# passwordless doas (as the Makefile does), but root only writes inside the
# workspace. The client checks use pkg(8) with INSTALL_AS_USER and private
# DB/cache/repo directories, so the host's package database is untouched.
set -eu

fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }

test "$#" -eq 1 || fail 'Usage: test_freebsd_repo.sh SCRIPTS_DIR'
scripts=$(cd "$1" && pwd)
tool=$scripts/freebsd-repo.sh
test -f "$tool" && test -f "$scripts/pkg-freebsd.sh" || fail 'scripts missing'
test "$(id -u)" -ne 0 || fail 'run as an ordinary user'
work=$(mktemp -d /tmp/freebsd-repo-test.XXXXXXXXXX)
trap 'rm -rf "$work"' EXIT
trap 'exit 1' HUP INT TERM
repo=$work/repo

# make_pkg NAME VERSION OUTDIR: a minimal one-file package.
make_pkg() {
    dir=$work/build-$1-$2
    mkdir -p "$dir/stage/usr/local/share/$1" "$dir/meta"
    printf '%s %s\n' "$1" "$2" > "$dir/stage/usr/local/share/$1/version"
    printf '{"name":"%s","version":"%s","origin":"local/%s",%s}\n' \
        "$1" "$2" "$1" '"comment":"fixture","desc":"fixture","maintainer":"fixture@example.invalid","www":"https://example.invalid","prefix":"/usr/local"' \
        > "$dir/meta/+MANIFEST"
    printf '@owner root\n@group wheel\nshare/%s/version\n' "$1" > "$dir/plist"
    pkg create -m "$dir/meta" -p "$dir/plist" -r "$dir/stage" -o "$3"
}

# entries ARCHIVE MEMBER: sorted name-version list of a catalog archive.
entries() {
    tar -xOf "$1" "$2" \
        | grep -o '"name":"[^"]*","origin":"[^"]*","version":"[^"]*"' \
        | sed 's/"name":"\([^"]*\)".*"version":"\([^"]*\)"/\1-\2/' | sort
}

# expect_catalog "name-version ...": both archives list exactly these.
expect_catalog() {
    # shellcheck disable=SC2086 # $1 is a space-separated list on purpose
    want=$(printf '%s\n' $1 | sort)
    test "$(entries "$repo/packagesite.pkg" packagesite.yaml)" = "$want" \
        || fail "packagesite lists $(entries "$repo/packagesite.pkg" packagesite.yaml | tr '\n' ' ')"
    test "$(entries "$repo/data.pkg" data)" = "$want" \
        || fail "data lists $(entries "$repo/data.pkg" data | tr '\n' ' ')"
}

catalog_sum() {
    for file in data.pkg packagesite.pkg meta.conf meta; do
        sha256 -q "$repo/$file"
    done
}

# must_fail_unchanged DESCRIPTION COMMAND...: fails, keeps the catalog, and
# leaves no staging directory or lock behind.
must_fail_unchanged() {
    what=$1
    shift
    before=$(catalog_sum)
    if "$@" > "$work/out" 2>&1; then fail "$what succeeded"; fi
    test "$(catalog_sum)" = "$before" || fail "$what changed the catalog"
    for leftover in "$repo"/.catalog.* "$repo"/All/.publish.*; do
        # The lock case creates .catalog.lock itself; it is checked below.
        case $leftover in */.catalog.lock) continue;; esac
        test ! -e "$leftover" || fail "$what left $leftover"
    done
}

# --- The h54 state: All/ holds two packages, the catalog lists only one.
mkdir -p "$repo/All" "$work/single/All"
make_pkg alpha 1.0 "$repo/All"
make_pkg beta 2.0 "$work/single/All"
cp "$work/single/All/beta-2.0.pkg" "$repo/All/"
pkg repo -q "$work/single"
cp "$work/single/data.pkg" "$work/single/packagesite.pkg" \
    "$work/single/meta.conf" "$work/single/meta" "$repo/"
expect_catalog 'beta-2.0'

# index regenerates over the full All/.
sh "$tool" index "$repo" > "$work/out"
expect_catalog 'alpha-1.0 beta-2.0'

# add places a third package and lists all three.
make_pkg gamma 3.0 "$work/new"
sh "$tool" add "$work/new/gamma-3.0.pkg" "$repo" > "$work/out"
test -f "$repo/All/gamma-3.0.pkg" || fail 'add did not place the package'
expect_catalog 'alpha-1.0 beta-2.0 gamma-3.0'

# --- Negative cases: each fails and keeps the published catalog.
must_fail_unchanged 'index without a repository' sh "$tool" index "$work/missing"
must_fail_unchanged 'usage error' sh "$tool" index
must_fail_unchanged 'unknown action' sh "$tool" remove "$repo"
mkdir "$work/no-all"
must_fail_unchanged 'index without All/' sh "$tool" index "$work/no-all"
mkdir -p "$work/empty/All"
must_fail_unchanged 'index of an empty All/' sh "$tool" index "$work/empty"
test ! -e "$work/empty/data.pkg" || fail 'empty All/ produced a catalog'
must_fail_unchanged 'add of a missing file' sh "$tool" add "$work/none.pkg" "$repo"
printf 'not a package\n' > "$work/new/notes.txt"
must_fail_unchanged 'add of a non-.pkg file' sh "$tool" add "$work/new/notes.txt" "$repo"
ln -s "$work/new/gamma-3.0.pkg" "$work/new/link-1.0.pkg"
must_fail_unchanged 'add of a symlink' sh "$tool" add "$work/new/link-1.0.pkg" "$repo"
test ! -e "$repo/All/link-1.0.pkg" || fail 'symlinked package was placed'
mkdir "$repo/.catalog.lock"
must_fail_unchanged 'index while locked' \
    env FREEBSD_REPO_LOCK_WAIT=1 sh "$tool" index "$repo"
rmdir "$repo/.catalog.lock"
must_fail_unchanged 'non-numeric lock wait' \
    env FREEBSD_REPO_LOCK_WAIT=x sh "$tool" index "$repo"
# pkg repo only warns about an unreadable package and exits 0: the coverage
# check must refuse a catalog that silently dropped it.
printf 'garbage\n' > "$repo/All/broken-1.0.pkg"
must_fail_unchanged 'index with an unreadable package' sh "$tool" index "$repo"
grep -q 'lacks broken-1.0.pkg' "$work/out" || fail 'unexpected failure reason'
rm "$repo/All/broken-1.0.pkg"
test ! -e "$repo/.catalog.lock" || fail 'lock left behind'

# --- The production publisher end to end (doas, as the Makefile runs it).
if doas -n true 2>/dev/null; then
    mkdir "$work/job"
    cp "$scripts/pkg-freebsd.sh" "$tool" "$work/job/"
    printf '#!/bin/sh\necho delta\n' > "$work/job/delta"
    sh "$work/job/pkg-freebsd.sh" delta 4.0 fixture fixture \
        fixture@example.invalid https://example.invalid "$repo" > "$work/out"
    expect_catalog 'alpha-1.0 beta-2.0 delta-4.0 gamma-3.0'
    publisher=yes
else
    publisher='skipped (no passwordless doas)'
fi

# --- A real client reads the catalog, and pkg fetch checks every checksum.
mkdir "$work/repos" "$work/root" "$work/db" "$work/cache"
printf 'fixture: { url: "file://%s", mirror_type: "NONE", signature_type: "NONE", enabled: yes }\n' \
    "$repo" > "$work/repos/fixture.conf"
client() {
    INSTALL_AS_USER=true pkg -R "$work/repos" -r "$work/root" \
        -o PKG_DBDIR="$work/db" -o PKG_CACHEDIR="$work/cache" "$@"
}
client update -f > "$work/out"
want=$(entries "$repo/packagesite.pkg" packagesite.yaml)
test "$(client rquery -a '%n-%v' | sort)" = "$want" || fail 'client sees other packages'
client fetch -y -o "$work/fetched" -a > "$work/out"
# A package that no longer matches its catalog checksum must be rejected.
printf 'x' >> "$repo/All/alpha-1.0.pkg"
if client fetch -y -o "$work/tampered" alpha > "$work/out" 2>&1; then
    fail 'pkg fetch accepted a package with a wrong checksum'
fi

printf 'PASS index/add list the full All/; negative cases keep the catalog; pkg-freebsd.sh: %s; client update/rquery/fetch ok\n' "$publisher"
