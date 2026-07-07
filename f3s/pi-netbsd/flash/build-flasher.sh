#!/bin/bash
# Stage B, step 2 — run on the Pi (as root via sudo). Installs the 95netbsdflash
# dracut module and builds a standalone /boot/flasher.img that the Pi firmware
# loads via config.txt. Does NOT arm or reboot (see README Stage B steps 3-4).
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
KVER=$(uname -r)
MODDIR=/usr/lib/dracut/modules.d/95netbsdflash
OUT=/boot/flasher.img

echo "== install dracut module 'netbsdflash' =="
mkdir -p "$MODDIR"
install -m0755 "$HERE/95netbsdflash/module-setup.sh" "$MODDIR/module-setup.sh"
install -m0755 "$HERE/95netbsdflash/flash.sh"        "$MODDIR/flash.sh"

echo "== keep an untouched backup of the default initramfs (first time only) =="
cp -n "/boot/initramfs-$KVER.img" "/boot/initramfs-$KVER.img.orig" 2>/dev/null || true

echo "== build $OUT for kernel $KVER =="
# ext4/vfat/mmc/nls are built-in on the RPi kernel, but --add-drivers is harmless
dracut --force --no-hostonly \
    --add netbsdflash \
    --add-drivers "vfat nls_cp437 nls_ascii ext4 mmc_block sdhci sdhci-iproc" \
    "$OUT" "$KVER"

ls -la "$OUT"
echo "== sanity: hook present in initramfs =="
lsinitrd "$OUT" | grep -E 'pre-mount/99-flash.sh' && echo "hook OK" || { echo "HOOK MISSING"; exit 1; }
echo "== done. Arm with a /boot/config.txt + /boot/netbsd-flash-mode (see README). =="
