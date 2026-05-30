#!/bin/sh
# CARP state change control script

HOSTNAME=`hostname`

if [ ! -f /data/nfs/nfs.DO_NOT_REMOVE ]; then
    logger '/data/nfs not mounted, mounting it now!'
    if ! /usr/local/sbin/f3s-mount-keys; then
        logger 'could not mount /keys; refusing to load ZFS keys for NFS'
        exit 1
    fi
    if [ "$HOSTNAME" = 'f0.lan.buetow.org' ]; then
        zfs load-key -L file:///keys/f0.lan.buetow.org:zdata.key zdata/enc/nfsdata
        zfs set mountpoint=/data/nfs zdata/enc/nfsdata
    else
        zfs load-key -L file:///keys/f0.lan.buetow.org:zdata.key zdata/sink/f0/zdata/enc/nfsdata
        zfs set mountpoint=/data/nfs zdata/sink/f0/zdata/enc/nfsdata
        zfs mount zdata/sink/f0/zdata/enc/nfsdata
        zfs set readonly=on zdata/sink/f0/zdata/enc/nfsdata
    fi
    service nfsd stop 2>&1
    service mountd stop 2>&1
fi

case "$2" in
    MASTER)
        logger "CARP state changed to MASTER, starting services"
        service rpcbind start >/dev/null 2>&1
        service mountd start >/dev/null 2>&1
        service nfsd start >/dev/null 2>&1
        service nfsuserd start >/dev/null 2>&1
        service stunnel restart >/dev/null 2>&1
        logger "CARP MASTER: NFS and stunnel services started"
        ;;
    BACKUP)
        logger "CARP state changed to BACKUP, stopping services"
        service stunnel stop >/dev/null 2>&1
        service nfsd stop >/dev/null 2>&1
        service mountd stop >/dev/null 2>&1
        service nfsuserd stop >/dev/null 2>&1
        logger "CARP BACKUP: NFS and stunnel services stopped"
        ;;
    *)
        logger "CARP state changed to $2 (unhandled)"
        ;;
esac
