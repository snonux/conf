# f3s FreeBSD USB ZFS Keys

The f-hosts keep ZFS raw encryption keys on a UFS USB stick mounted at `/keys`.
Do not mount that stick from `/etc/fstab`: a missing or corrupt USB stick must
not block the FreeBSD base OS from booting.

## Installed Files

- `/usr/local/sbin/f3s-mount-keys` mounts the USB key stick on demand.
- `/usr/local/sbin/f3s-load-zfs-keys` is a manual recovery helper that mounts
  `/keys`, then loads datasets from `zfskeys_datasets`.
- `/etc/rc.d/f3skeys` runs before FreeBSD's built-in `zfskeys` service.

The rc service deliberately returns success if the USB stick is missing or
fails `fsck_ufs -p`. Boot continues; encrypted datasets remain locked until the
stick is repaired and keys are loaded manually.

## Host Configuration

On each f-host:

```sh
doas install -o root -g wheel -m 0555 f3s-mount-keys /usr/local/sbin/f3s-mount-keys
doas install -o root -g wheel -m 0555 f3s-load-zfs-keys /usr/local/sbin/f3s-load-zfs-keys
doas install -o root -g wheel -m 0555 f3skeys.rc /etc/rc.d/f3skeys
doas sysrc f3skeys_enable=YES
doas sysrc zfskeys_enable=YES
```

Comment out any `/keys` line in `/etc/fstab`, for example:

```fstab
# /dev/da0 /keys ufs rw 0 2
```

If possible, label the UFS filesystem `F3S_KEYS` and let the script mount
`/dev/ufs/F3S_KEYS`. The script still falls back to `/dev/da0` for the current
single-stick host layout.

Current boot key-load datasets:

```sh
# f0
doas sysrc zfskeys_datasets="zdata/enc zdata/enc/nfsdata zroot/bhyve zroot/garage"

# f1
doas sysrc zfskeys_datasets="zdata/enc zroot/bhyve zroot/garage zdata/sink/f0/zdata/enc/nfsdata"

# f2
doas sysrc zfskeys_datasets="zdata/enc zroot/bhyve zroot/garage zroot/sink/f3/zroot/bhyve/freebsd"

# f3
doas sysrc zfskeys_datasets="zroot/bhyve"
```

Replicated encrypted sinks use file keylocations so boot can load them without
a prompt:

```sh
# f1
doas zfs set keylocation=file:///keys/f0.lan.buetow.org:zdata.key \
  zdata/sink/f0/zdata/enc/nfsdata

# f2
doas zfs set keylocation=file:///keys/f3.lan.buetow.org:bhyve.key \
  zroot/sink/f3/zroot/bhyve/freebsd
```

## Verification

```sh
doas service f3skeys start
mount | grep ' /keys '
doas service zfskeys status
doas /usr/local/sbin/f3s-load-zfs-keys
rcorder /etc/rc.d/* /usr/local/etc/rc.d/* | grep -E 'f3skeys|zfskeys|zfs$'
```

After a reboot, verify:

```sh
mount | grep ' /keys '
sysrc -n f3skeys_enable
sysrc -n zfskeys_enable
sysrc -n zfskeys_datasets
zfs list -H -o name,encryption,keylocation,keystatus,mounted |
  awk '$2 != "off" { print }'
```

`zroot/sink/f3/zroot/bhyve/freebsd` on f2 has `mountpoint=none`; its key should
be available after boot, but the dataset is not expected to be mounted.
