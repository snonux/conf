#!/usr/bin/env bash

set -euo pipefail

if [[ $# -ne 5 ]]; then
    echo "usage: $0 <x86_64|aarch64> <version> <dtail-src> <asset-dir> <out-dir>" >&2
    exit 1
fi

rpm_arch=$1
raw_version=$2
dtail_src=$3
asset_dir=$4
out_dir=$5

case "$rpm_arch" in
    x86_64)
        goarch=amd64
        ;;
    aarch64)
        goarch=arm64
        ;;
    *)
        echo "unsupported rpm arch: $rpm_arch" >&2
        exit 1
        ;;
esac

rpm_version=${raw_version%%-*}
rpm_release=1
if [[ "$raw_version" == *-* ]]; then
    suffix=${raw_version#"$rpm_version"-}
    suffix=${suffix//[^A-Za-z0-9.+~]/.}
    rpm_release="1.${suffix}"
fi

workdir=$(mktemp -d)
trap 'rm -rf "$workdir"' EXIT

pkgroot="$workdir/root"
topdir="$workdir/rpmbuild"
mkdir -p \
    "$pkgroot/usr/local/bin" \
    "$pkgroot/etc/dserver" \
    "$pkgroot/usr/lib/systemd/system" \
    "$pkgroot/usr/share/licenses/dtail" \
    "$topdir/BUILD" \
    "$topdir/BUILDROOT" \
    "$topdir/RPMS" \
    "$topdir/SOURCES" \
    "$topdir/SPECS" \
    "$topdir/SRPMS"

if [[ -n "${DTAIL_PREBUILT_ROOT:-}" ]]; then
    cp -a "$DTAIL_PREBUILT_ROOT/." "$pkgroot/"
else
    for bin in dserver dcat dgrep dmap dtail dtailhealth; do
        (
            cd "$dtail_src"
            CGO_ENABLED=0 GOOS=linux GOARCH="$goarch" go build -tags nozstd -o "$pkgroot/usr/local/bin/$bin" "./cmd/$bin/main.go"
        )
    done

    install -m 0644 "$asset_dir/dtail.json" "$pkgroot/etc/dserver/dtail.json"
    install -m 0755 "$asset_dir/dserver-update-key-cache.sh" "$pkgroot/usr/local/bin/dserver-update-key-cache.sh"
    install -m 0644 "$asset_dir/dserver.service" "$pkgroot/usr/lib/systemd/system/dserver.service"
    install -m 0644 "$asset_dir/dserver-update-keycache.service" "$pkgroot/usr/lib/systemd/system/dserver-update-keycache.service"
    install -m 0644 "$asset_dir/dserver-update-keycache.timer" "$pkgroot/usr/lib/systemd/system/dserver-update-keycache.timer"
    install -m 0644 "$dtail_src/LICENSE" "$pkgroot/usr/share/licenses/dtail/LICENSE"
fi

cp -a "$pkgroot" "$topdir/SOURCES/root"

cat >"$topdir/SPECS/dtail.spec" <<EOF
Name:           dtail
Version:        $rpm_version
Release:        $rpm_release
Summary:        Distributed log tail and grep tool
License:        Apache-2.0
URL:            https://codeberg.org/snonux/dtail
BuildArch:      $rpm_arch
Requires(pre):  shadow-utils
Requires(post): systemd
Requires(preun): systemd
Requires(postun): systemd

%description
DTail is a distributed DevOps tool for tailing, grepping, catting, and
mapping across many remote machines at once via SSH.

%prep

%build

%install
mkdir -p %{buildroot}
cp -a %{_sourcedir}/root/. %{buildroot}/

%pre
getent group dserver >/dev/null || groupadd -r dserver
getent passwd dserver >/dev/null || useradd -r -g dserver -d /var/lib/dserver -s /sbin/nologin -c "DTail server" dserver
exit 0

%post
systemctl daemon-reload >/dev/null 2>&1 || true

%preun
if [ \$1 -eq 0 ]; then
    systemctl --no-reload disable --now dserver-update-keycache.timer >/dev/null 2>&1 || true
fi

%postun
systemctl daemon-reload >/dev/null 2>&1 || true

%files
%license /usr/share/licenses/dtail/LICENSE
%dir /etc/dserver
%config(noreplace) /etc/dserver/dtail.json
/usr/local/bin/dserver
/usr/local/bin/dcat
/usr/local/bin/dgrep
/usr/local/bin/dmap
/usr/local/bin/dtail
/usr/local/bin/dtailhealth
/usr/local/bin/dserver-update-key-cache.sh
/usr/lib/systemd/system/dserver.service
/usr/lib/systemd/system/dserver-update-keycache.service
/usr/lib/systemd/system/dserver-update-keycache.timer

%changelog
* Sat Apr 11 2026 Paul Buetow <paul@buetow.org> - $rpm_version-$rpm_release
- Package DTail for Rocky Linux 9
EOF

mkdir -p "$out_dir"
rpmbuild --quiet \
    --define "_topdir $topdir" \
    --target "$rpm_arch" \
    -bb "$topdir/SPECS/dtail.spec"
cp "$topdir/RPMS/$rpm_arch"/*.rpm "$out_dir/"
