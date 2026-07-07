#!/bin/bash
# dracut module: 95netbsdflash (Pi-firmware initramfs variant)
# Loaded by the Pi firmware via config.txt `initramfs flasher.img followkernel`.
# A pre-mount hook (runs in the initramfs, before the real root is mounted)
# reads a trigger file from the FAT /boot partition and flashes NetBSD onto the
# SD from a RAM copy of the golden image staged on the ext4 root.

check() { return 0; }
depends() { return 0; }

installkernel() {
    # vfat to read /boot (config.txt/trigger), ext4 to read the staged image,
    # mmc drivers for the SD.
    instmods vfat nls_cp437 nls_ascii ext4 mmc_block sdhci sdhci-iproc
}

install() {
    inst_multiple gzip gunzip dd sync sleep sh cat wc mount umount mkdir rm ls cp grep tr
    inst_hook pre-mount 99 "$moddir/flash.sh"
}
