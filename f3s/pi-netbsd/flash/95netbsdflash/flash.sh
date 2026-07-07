#!/bin/sh
# dracut pre-mount hook (SOURCED by dracut init — must not call exit/return at
# top level in a way that aborts init; we keep all action inside an `if`).
#
# Trigger: a file `netbsd-flash-mode` (contents "dryrun" or "real") on the FAT
# /boot partition. If absent, this hook does nothing and normal boot continues.
#
# SAFETY: the very first thing we do once triggered is DISARM — delete
# config.txt and the trigger from the FAT partition — so that if anything later
# fails, a power-cycle boots normally back into Rocky (config.txt gone => the Pi
# firmware loads no initramfs).

FATDEV=/dev/mmcblk0p1
ROOTDEV=/dev/mmcblk0p3
DISK=/dev/mmcblk0

nf_log() { echo "netbsdflash: $*"; echo "netbsdflash: $*" > /dev/kmsg 2>/dev/null; }
nf_reboot() { sync; sleep 2; reboot -f 2>/dev/null; sleep 2; echo b > /proc/sysrq-trigger 2>/dev/null; sleep 15; }

mkdir -p /nf_fat /nf_root /nf_ram 2>/dev/null
NF_MODE=""
if mount -t vfat "$FATDEV" /nf_fat 2>/dev/null; then
    [ -f /nf_fat/netbsd-flash-mode ] && NF_MODE=$(tr -d ' \r\n' < /nf_fat/netbsd-flash-mode 2>/dev/null)
fi

if [ -n "$NF_MODE" ]; then
    nf_log "TRIGGERED mode=$NF_MODE"
    # --- DISARM FIRST (self-heal on any later failure) ---
    rm -f /nf_fat/config.txt /nf_fat/netbsd-flash-mode 2>/dev/null
    sync
    echo "FLASHER-RAN mode=$NF_MODE up=$(cat /proc/uptime 2>/dev/null)" > /nf_fat/flash-evidence.txt 2>/dev/null
    sync
    nf_log "disarmed; evidence written"

    # --- stage golden image from ext4 root into RAM ---
    NF_OK=1
    if ! mount -t ext4 -o ro "$ROOTDEV" /nf_root 2>/dev/null; then
        nf_log "FATAL mount root"; echo "FATAL mount-root" >> /nf_fat/flash-evidence.txt; NF_OK=0
    fi
    GZ=""
    for cand in /nf_root/home/paul/netbsd-*-golden.img.gz /nf_root/root/netbsd-*-golden.img.gz; do
        [ -f "$cand" ] && { GZ="$cand"; break; }
    done
    if [ "$NF_OK" = 1 ] && [ -z "$GZ" ]; then
        nf_log "FATAL image-missing"; echo "FATAL image-missing" >> /nf_fat/flash-evidence.txt; NF_OK=0
    fi
    if [ "$NF_OK" = 1 ]; then
        mount -t tmpfs -o size=550m tmpfs /nf_ram 2>/dev/null
        if cp "$GZ" /nf_ram/g.gz 2>/dev/null; then
            nf_log "staged $(wc -c < /nf_ram/g.gz 2>/dev/null) bytes in RAM"
        else
            nf_log "FATAL copy-to-RAM"; echo "FATAL copy-RAM" >> /nf_fat/flash-evidence.txt; NF_OK=0
        fi
    fi
    umount /nf_root 2>/dev/null
    sync

    if [ "$NF_OK" = 1 ] && [ "$NF_MODE" = dryrun ]; then
        bytes=$(gzip -dc /nf_ram/g.gz 2>/dev/null | wc -c); rc=$?
        nf_log "DRYRUN bytes=$bytes rc=$rc"
        echo "DRYRUN-RESULT bytes=$bytes rc=$rc" >> /nf_fat/flash-evidence.txt
        sync; umount /nf_fat 2>/dev/null
        nf_log "DRYRUN done; rebooting to Rocky"
        nf_reboot
    elif [ "$NF_OK" = 1 ]; then
        echo "REAL-START up=$(cat /proc/uptime 2>/dev/null)" >> /nf_fat/flash-evidence.txt
        sync; umount /nf_fat 2>/dev/null
        nf_log "REAL: writing image onto $DISK"
        gzip -dc /nf_ram/g.gz 2>/dev/null | dd of="$DISK" bs=4M conv=fsync 2>&1 | tail -2 | while read -r l; do nf_log "dd: $l"; done
        sync
        nf_log "REAL flash complete; rebooting into NetBSD"
        nf_reboot
    else
        # a FATAL occurred; config.txt already removed, so reboot -> Rocky
        sync; umount /nf_fat 2>/dev/null
        nf_log "aborted; rebooting to Rocky"
        nf_reboot
    fi
else
    # not triggered: clean up and let the normal boot proceed
    umount /nf_fat 2>/dev/null
fi
