#!/bin/sh
# Run as an ordinary FreeBSD user, using only an exclusive /tmp workspace.
# Arguments: checked bundle, packaging script, already assembled package.
set -eu
umask 077
test "$#" -eq 3
bundle=$1
packager=$2
artifact=$3
test "$(id -u)" -ne 0
installed_before=$(pkg query '%v' dtail)
binary_before=$(sha256 -q /usr/local/bin/dserver)
work=$(mktemp -d /tmp/mp3-native-test.XXXXXXXXXX)
trap 'rm -rf "$work"' EXIT
trap 'exit 1' HUP INT TERM

# Verify the actual native assembler rejects tampering and symlink inputs.
cp -R "$bundle" "$work/bad"
printf 'corruption\n' >> "$work/bad/dserver"
if sh "$packager" assemble "$work/bad" "$work/bad-out"; then exit 70; fi
test ! -e "$work/bad-out"
cp "$bundle/dserver" "$work/bad/dserver"
rm "$work/bad/dserver-update-key-cache.sh"
printf 'synthetic-sentinel\n' > "$work/sentinel"
sentinel_before=$(sha256 -q "$work/sentinel")
ln -s "$work/sentinel" "$work/bad/dserver-update-key-cache.sh"
if sh "$packager" assemble "$work/bad" "$work/symlink-out"; then exit 71; fi
test ! -e "$work/symlink-out"
test "$(sha256 -q "$work/sentinel")" = "$sentinel_before"

# Real repository indexing must keep an unrelated existing package.
mkdir -p "$work/repo/All" "$work/other/meta" "$work/other/stage/usr/local/share/mp3"
printf 'unrelated package fixture\n' > "$work/other/stage/usr/local/share/mp3/sentinel"
cat > "$work/other/meta/+MANIFEST" <<'MANIFEST'
{"name":"mp3-sentinel","version":"1.0","origin":"local/mp3-sentinel","comment":"fixture","desc":"fixture","maintainer":"fixture@example.invalid","www":"https://example.invalid","prefix":"/usr/local"}
MANIFEST
printf '@owner root\n@group wheel\nshare/mp3/sentinel\n' > "$work/other/plist"
pkg create -m "$work/other/meta" -p "$work/other/plist" -r "$work/other/stage" -o "$work/repo/All"
pkg repo "$work/repo"
sh "$packager" publish "$artifact" "$work/repo"
tar -xOf "$work/repo/packagesite.pkg" packagesite.yaml > "$work/catalog"
grep -q '"name"[[:space:]]*:[[:space:]]*"mp3-sentinel"' "$work/catalog"
grep -q '"name"[[:space:]]*:[[:space:]]*"dtail"' "$work/catalog"
version=$(pkg query -F "$artifact" '%v')
published="$work/repo/All/dtail-$version.pkg"
artifact_before=$(sha256 -q "$published")
catalog_before=$(sha256 -q "$work/repo/packagesite.pkg")
if sh "$packager" publish "$artifact" "$work/repo"; then exit 72; fi
test "$(sha256 -q "$published")" = "$artifact_before"
test "$(sha256 -q "$work/repo/packagesite.pkg")" = "$catalog_before"

# Exercise the legacy configuration migration in a separate package DB/root.
# INSTALL_AS_USER and -r keep all installation writes in our owned workspace.
mkdir -p "$work/legacy/meta" "$work/legacy/stage/usr/local/etc/dserver" "$work/legacy/out" "$work/root"
sed 's/"version": "[^"]*"/"version": "4.3.2-ng"/' "$bundle/+MANIFEST" > "$work/legacy/meta/+MANIFEST"
printf '{"old":"default"}\n' > "$work/legacy/stage/usr/local/etc/dserver/dtail.json"
printf '@owner root\n@group wheel\netc/dserver/dtail.json\n' > "$work/legacy/plist"
pkg create -m "$work/legacy/meta" -p "$work/legacy/plist" -r "$work/legacy/stage" -o "$work/legacy/out"
sandbox_pkg() {
    INSTALL_AS_USER=true pkg -r "$work/root" -o RUN_SCRIPTS=false \
        -o HANDLE_RC_SCRIPTS=false -o REPO_AUTOUPDATE=false "$@"
}
sandbox_pkg add "$work/legacy/out/dtail-4.3.2-ng.pkg"
printf '{"host":"preserved","bind":"192.0.2.10"}\n' > "$work/root/usr/local/etc/dserver/dtail.json"
cp "$work/root/usr/local/etc/dserver/dtail.json" "$work/saved-config"
config_before=$(sha256 -q "$work/saved-config")
sandbox_pkg add "$artifact"
# Old unmarked config may be removed by pkg: restore the explicit saved copy.
install -m 644 "$work/saved-config" "$work/root/usr/local/etc/dserver/dtail.json"
test "$(sha256 -q "$work/root/usr/local/etc/dserver/dtail.json")" = "$config_before"
test "$(sha256 -q "$work/root/usr/local/etc/dserver/dtail.json.sample")" = "$(sha256 -q "$bundle/dtail.json.sample")"
test "$(sha256 -q "$work/root/usr/local/bin/dserver")" = "$(sha256 -q "$bundle/dserver")"
test "$(sandbox_pkg query '%v' dtail)" = "$version"
sandbox_pkg check -s dtail
test ! -e "$work/root/var/db/dserver"
test ! -e "$work/root/var/run/dserver"
test "$(pkg query '%v' dtail)" = "$installed_before"
test "$(sha256 -q /usr/local/bin/dserver)" = "$binary_before"
printf '{"native_checksum_negative":true,"native_symlink_negative":true,"repository_keeps_unrelated_package":true,"duplicate_version_rejected_without_changes":true,"sandbox_legacy_upgrade_and_config_restore":true,"sandbox_pkg_checksums_pass":true,"host_package_and_binary_unchanged":true}\n'
