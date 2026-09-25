# zusb: removable USB backup pool

`zusb` is a raidz2 pool over 4 x 1.8 TB USB-SATA disks (ASMT ASM235CM
bridges, ~5.4 TB usable), the offline backup target. It is plugged in about
once a quarter, loaded by hand, then exported and unplugged. Never
auto-imported. It is currently on f1, but scripts and key are on all f-hosts,
so it loads wherever it's plugged in.

- Encryption root `zusb/data/enc`, `keyformat=raw`,
  `keylocation=file:///keys/zusb.key` (32-byte key on the `F3S_KEYS` stick,
  see [`../keys/README.md`](../keys/README.md)). Children
  `zusb/data/enc/{backups,books,documents,games,git,mail,music,pictures,videos,yoga,...}`
  inherit it.
- Not in any host's `zfskeys_datasets`, so boot never waits for it.
- The old t450 passphrase key is obsolete (rekeyed on migration).

## Scripts

Both are host-independent and byte-identical on every f-host. Disks are found
by bridge serial `914000000A11`-`914000000A14`; bus and unit numbers are
resolved at runtime.

- `zusb-load`: mounts the key stick, if the pool is offline runs
  `usbconfig power_on`, waits for CAM, SCSI `START UNIT`, waits for all disks,
  `zpool import zusb`, loads the key, mounts everything. Skips the power
  sequence when already imported.
- `zusb-unload`: snapshots (`/opt/snonux/bin/zfs/zfs.snapshot` when `/opt` is
  mounted, else a timestamped fallback), exports, SCSI `STOP UNIT`,
  `usbconfig power_off`. Refuses to export if a disk can't be identified. A
  stop/power-off failure after export exits nonzero but the pool is already
  safe.

gonf installs both scripts to `/usr/local/bin` (0755 root:wheel) on every
f-host (`gonf/freebsd/zusb.go`, task `freebsd_zusb_scripts`, part of the
`freebsd` aggregate); it never runs them. Edit the repo copy, then:

```sh
./gonf.sh -n cluster freebsd-hosts freebsd_zusb_scripts
./gonf.sh cluster freebsd-hosts freebsd_zusb_scripts
```

The key is not managed by gonf. Put the same key on every stick (copy from a
stick that has it; never commit it):

```sh
doas mount -u -o rw /keys
doas install -o root -g wheel -m 0400 zusb.key /keys/zusb.key
doas mount -u -o ro /keys
```

## Use

```sh
doas /usr/local/bin/zusb-load
doas zfs list -r zusb
/opt/snonux/bin/backup/backup     # lives on the pool (zusb/data/opt)
doas /usr/local/bin/zusb-unload   # then unplug
```

The backup's S3 leg needs the AWS CLI on the hosting f-host and
`/root/.aws/credentials` symlinked to `/opt/snonux/secrets/aws.credentials`
(on the pool; `f3s-storage` skill, `references/backups.md`). Without it the
snapshot/export part still works.

The pool can also be unloaded remotely through `f3sctl` (`zusb-status`,
`zusb-unload`, see [`../f3sctl/README.md`](../f3sctl/README.md)).

## Check

```sh
sha256 /usr/local/bin/zusb-load /usr/local/bin/zusb-unload /keys/zusb.key   # same on all hosts
zpool status zusb
zfs list -r zusb -o name,keystatus,mounted,mountpoint
```
