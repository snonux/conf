# NetBSD 11 dual-node acceptance

Final acceptance was performed on 2026-08-06/07 for `pi0` and `pi1`. Both
Raspberry Pi 3B+ nodes now run the formal NetBSD 11.0 evbarm/aarch64 release.
`pi0` remains the static-content source of truth and `pi1` pulls its docroot
hourly.

## Verified state

Both nodes independently passed these checks over direct SSH port 22:

- `uname` reported NetBSD 11.0, `GENERIC64`, evbarm; `uname -p` reported
  aarch64. `mue0` was active at 1000baseT with the expected LAN address
  (`192.168.1.125` or `.126`) and `tun0` had the expected mesh address
  (`192.168.2.203` or `.204`).
- NPF reported active filtering and a loaded configuration. The rules allow
  TCP 22, 80, and 2222 on `mue0`, allow mesh traffic on `tun0`, and block by
  default. SSH, bozohttpd, WireGuard, uptimed, dserver, and cron were running.
- WireGuard had current handshakes with both relayd frontends. All five routed
  vhosts (`f3s.buetow.org`, `www.f3s.buetow.org`,
  `standby.f3s.buetow.org`, `snonux.foo`, and `www.snonux.foo`) returned HTTP
  200 from each node. The discovered 12 `index.html` files matched after sync.
  `/fotos/` and `/scifi/` returned 200 for the f3s vhosts; their expected 404
  under the separate snonux vhost was also confirmed.
- The `dtail-4.3.2ng` NetBSD package was installed, dserver listened on TCP
  2222, and its rc.d service survived reboot. Uptimed retained Linux, NetBSD
  10.1, and NetBSD 11.0 history. Root cron retained the hourly goprecords
  upload and dserver key-cache jobs; only pi1's paul crontab contained the
  hourly source pull.
- The goprecords uploader, token, records file, and hourly cron were present.
  A manual production upload succeeded from both nodes. The first pi0 attempt
  exposed a stale uploader `PATH` without `/usr/pkg/bin`, so cron could not
  find curl; its installed script was corrected to match pi1 and the repeated
  upload succeeded.

The pi1 pull initially exposed a recovery regression: pi0's reconstructed
`authorized_keys` lacked pi1's dedicated sync key, and the rsync SSH command
could offer too many forwarded-agent identities. The pi1 public key was
restored on pi0 and the pull was pinned to
`IdentitiesOnly=yes -i /home/paul/.ssh/id_ed25519`. A full 4.3 GiB source-to-
staging synchronization then completed successfully. This preserves pi0 as
source of truth and is the preferred recovery pattern after rebuilding either
node: restore peer authentication explicitly, pin unattended SSH to its key,
verify helper-script paths after a base recovery, then run one full pull and a
manual goprecords upload before relying on cron.

## Reboot, failover, and soak

Each node was rebooted unattended with `shutdown -r now`. Both returned on SSH
with NetBSD 11.0 and all required rc.d services. The reboot checks and public
failover checks were separate gates: stopping pi0's bozohttpd took pi0 out of
the static backend pool, after which pi1 passed the complete 30-request matrix
through both public relayd addresses (`23.88.35.144` and `46.23.94.99`); pi0
was then immediately restored. Rebooting pi1 took it out of the pool and pi0
passed the same complete matrix before pi1 was immediately verified after
return. An earlier matrix started during pi0's reboot health transition had
one timeout and was not counted as the successful pi0-down gate. HTTP 200 was
required for site roots and valid f3s paths; the snonux `/fotos/` and `/scifi/`
404s are expected vhost isolation, not failover failures.

The pair subsequently completed a 13-minute 12-second, 40-round soak (a
15-second pause after each round; checks added the remaining runtime). Every
round checked SSH and seven rc.d services on both nodes plus representative
f3s and snonux requests through both frontends; all 40 rounds passed. This is a
focused acceptance soak, not the previously waived 72-hour pi1 soak.

Boot logs show the Raspberry Pi's known lack of a TOD clock, initial ntp time
step/unsynchronized messages, device-tree autoconfiguration warnings, and one
initial `sdmmc0` direct-I/O probe error per boot. Both cards then identify at
SDR25, mount the FFS root, use `/stand/evbarm/11.0/modules`, synchronize time,
and run normally. No recurring post-boot SD, FFS, kernel/module, time, or
service error was observed during acceptance.

## Package repositories and recovery references

The official pkgsrc repository is:

```text
https://cdn.NetBSD.org/pub/pkgsrc/packages/NetBSD/aarch64/11.0/All
```

The custom DTail package and summary are under:

```text
https://pkgrepo.f3s.buetow.org/netbsd/11.0/packages/aarch64/
```

The package Makefile now defaults `dtail-netbsd` to 11.0 and packages natively
on pi0; `dtail-netbsd11` remains a compatibility alias.
If rollback requires republishing the old repository, boot a retained 10.1
clone on the selected native build host and invoke:

```sh
make dtail-netbsd NETBSD_VERSION=10.1 \
    NETBSD_BUILD_HOST=paul@pi0.lan.buetow.org
```

Do not package for 10.1 on a running 11.0 host: `pkg_create` must match the
target NetBSD release.

The file-level recovery archives remain available:

- `fishfinger:/home/rex/backups/pi1-20260803`
- `fishfinger:/home/rex/backups/pi0-20260806T0712`

After explicit approval on 2026-08-07, the rollback images were deleted:

- pi0 full microSD image
  `/home/paul/Downloads/pi0-recovery-20260806/pi0-microsd-before-wheel-fix.img`
- pi1 boot-partition image
  `fishfinger:/home/rex/backups/pi1-20260803/pi1-boot-partition.img`

The remaining `.tar.gz` archives are file-level recovery references, not disk
images.

The one-time upgrade and recovery records are retained as incident history,
not active runbooks:

- [`NETBSD-11-PI1-UPGRADE-INCIDENT.md`](NETBSD-11-PI1-UPGRADE-INCIDENT.md)
- [`PI0-NETBSD-11-RECOVERY-INCIDENT.md`](PI0-NETBSD-11-RECOVERY-INCIDENT.md)

## External documentation

The external `~/.agents/skills` f3s, Raspberry Pi, package-repository, and
DTail references were synchronized after acceptance to describe the verified
NetBSD 11.0 state, current package URLs, and the major-upgrade recovery lessons
above.
