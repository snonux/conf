# f3s zusb — Removable USB Backup Pool

`zusb` is a **4-disk raidz2 ZFS pool on 1.8 TB USB-SATA disks** (ASMT ASM235CM
bridges) used as the **offline backup storage device**. It is plugged in and
loaded **roughly once per quarter** to back up data, then exported and
unplugged again. Because it is offline most of the time, it is **not**
auto-imported or auto-mounted at boot — it is loaded manually with
`zusb-load`.

The pool is hosted on whichever f-host it is currently plugged into (f1 today).
The load/unload scripts and the encryption key are deployed to **all f-hosts**
(f0/f1/f2/f3), so the disk stack can be re-plugged to any host and loaded there
without any per-host setup.

## Pool & Encryption

- Pool: `zusb` (raidz2 over 4 × 1.8 TB USB disks, ~5.4 TB usable).
- Encryption root: `zusb/data/enc`, `keyformat=raw`,
  `keylocation=file:///keys/zusb.key` — the raw 32-byte key lives on the
  `F3S_KEYS` UFS USB stick at `/keys/zusb.key`, exactly like the other f-host
  ZFS keys (see [`../keys/README.md`](../keys/README.md)). All child datasets
  (`zusb/data/enc/{backups,books,documents,games,git,mail,music,pictures,
  videos,yoga,…}`) inherit from this root.
- `zusb` is deliberately **not** listed in any host's `zfskeys_datasets`, so
  boot-time key loading skips it and a missing/offline pool never blocks boot.

`zusb/data/enc` was rekeyed from its original passphrase key (t450's
`zroot/secret/zroot.enc.key`) to the raw key on the stick when the pool was
migrated from t450 to the f3s hosts. The old passphrase key on t450 is now
obsolete for `zusb`.

## Installed Files

- `/usr/local/bin/zusb-load` — mounts the `F3S_KEYS` stick, powers on and starts
  the four USB disks when the pool is offline, imports `zusb`, loads the
  encryption key for `zusb/data/enc` from `/keys/zusb.key`, and mounts all
  datasets.
- `/usr/local/bin/zusb-unload` — snapshots `zusb` for safety (via
  `/opt/snonux/bin/zfs/zfs.snapshot` when `/opt` is mounted, with a timestamped
  fallback), exports the pool, stops all four disks, and powers off their USB
  devices so the disks can be unplugged safely. It identifies the disks by
  their stable USB-SATA bridge serials rather than host-specific device names.

Both scripts are host-independent (they reference only `zusb`, `zusb/data/enc`,
and `/keys/zusb.key`) and are byte-identical across all f-hosts.

## Host Configuration

On each f-host install the scripts:

```sh
doas install -o root -g wheel -m 0755 zusb-load   /usr/local/bin/zusb-load
doas install -o root -g wheel -m 0755 zusb-unload /usr/local/bin/zusb-unload
```

Put the raw key on every host's `F3S_KEYS` stick (`/keys` is mounted read-only
at boot, so remount read-write to add it). The same 32-byte key file goes on
all four sticks:

```sh
doas mount -u -o rw /keys
doas install -o root -g wheel -m 0400 zusb.key /keys/zusb.key
doas mount -u -o ro /keys
```

The raw key is **not** committed to this repo (it is secret key material). It
is copied host-to-host from a stick that already has it.

## Usage

```sh
doas /usr/local/bin/zusb-load     # plug the disks in, then load + mount
doas zfs list -r zusb
# ... run the quarterly backup ...
doas /usr/local/bin/zusb-unload   # snapshot + export + power off, then unplug
```

The unload script first sends SCSI `STOP UNIT` to park/spin down each disk and
then runs `usbconfig power_off` for only these four bridge serials:
`914000000A11` through `914000000A14`. USB bus and disk unit numbers are
resolved at runtime because they can change after a reboot or re-plug. If a
disk cannot be identified, the script refuses to export; if stopping or
powering off fails after export, it exits nonzero and reports that the pool is
already safely exported.

When the pool is offline, `zusb-load` performs the inverse operation for the
same four serials: `usbconfig power_on`, wait for CAM discovery, SCSI `START
UNIT`, wait for every disk to become ready, then `zpool import`. This also works
when the stack has been re-plugged and its disks are already powered. If `zusb`
is already imported, the power/start sequence is skipped.

The quarterly backup itself is driven by `/opt/snonux/bin/backup/backup`
(which travels on the pool under `zusb/data/opt`). Its S3 sync leg needs the
AWS CLI installed on the hosting f-host and `/root/.aws/credentials` symlinked
to `/opt/snonux/secrets/aws.credentials` (also on the pool). See the
`f3s-storage` skill, `references/backups.md` → "AWS CLI setup on a FreeBSD
host", for the install steps. Without the AWS CLI the snapshot-export part of
the backup still works; only the offsite S3 sync is skipped/fails.

## Verification

```sh
ls -l /usr/local/bin/zusb-load /usr/local/bin/zusb-unload
ls -l /keys/zusb.key
sha256 /usr/local/bin/zusb-load /usr/local/bin/zusb-unload   # identical on all hosts
sha256 /keys/zusb.key                                          # identical on all sticks
```

After `zusb-load`:

```sh
zpool status zusb
zfs list -r zusb -o name,keystatus,mounted,mountpoint
```

## Origin

Ported from `t450:/root/bin/zusb-load.csh` and `zusb-unload.csh`. The t450
flow unlocked a passphrase-protected `zroot/secret` keystore and then read
`/zroot/secret/zroot.enc.key`; on the f-hosts the raw key lives directly on the
`F3S_KEYS` stick, matching the existing f-host ZFS key scheme. The t450
unload's `zfs.snapshot zroot` step was dropped (f-host `zroot` is the host boot
pool, not part of this workflow).
