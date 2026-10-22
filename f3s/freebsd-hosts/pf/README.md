# f-host PF rollout (task rm3)

`gonf/freebsd/firewall.go` owns PF on **f0–f3** through
`freebsd_firewall_config`. The canonical rules are `pf.conf` here; the old
`relayd/pf.conf`, `freebsd_relayd_pf` and `freebsd_relayd_rc_conf` are retired.
Relayd depends on the firewall task. The `freebsd` aggregate includes it.

**PF deployed to f0–f2, 2026-10-22.** User approved the guarded f1/f0/f2 rollout
and CARP tests; f3 remains unapproved to wake/reboot and pending inspection.
 The rm3 safety annotation requires
explicit user approval before **any live change**, including deployment,
module loads, firewall activation, rollback preparation and CARP failover.
Obtain approval for the concrete procedure below before running it. Do not
deploy the whole `freebsd` aggregate for this change.

## Policy and evidence

Inbound defaults to block; outbound keeps state. `lo0` stays skipped for S7.
On `re0`, sources in `192.168.1.0/24`, `fd22:c702:acb7::/64` and `fe80::/10`
can reach host addresses. LAN trust deliberately includes stunnel TCP 2323
from r0–r2, Garage TCP 3900/3901/3903 from f-hosts/r-nodes and UPS TCP 3551.
WireGuard `wg0` remains trusted, including frontend Garage access and zrepl
TCP 8888. Encrypted WireGuard UDP **56709** remains reachable on `re0` over
both IP families. CARP protocol 112 advertisements between f0/f1, LAN VIP
TCP 80/443, ICMP and IPv6 errors/ND/SLAAC/MLD are allowed. Other global IPv6
TCP/UDP traffic is blocked. Dynamic `(re0)` address matching follows SLAAC
changes without opening additional services.

The bhyve bridge remains unfiltered: both `net.link.bridge.pfil_member`
and `net.link.bridge.pfil_bridge` are pinned to 0 live, in sysctl.conf and
as loader tunables. Host-destined IP traffic still reaches PF. Do not add
`set skip on re0`, enable bridge filtering, or restart a VM. See FreeBSD's
[bridge implementation](https://cgit.freebsd.org/src/tree/sys/net/if_bridge.c)
and [advanced networking handbook](https://docs.freebsd.org/en/books/handbook/advanced-networking/).
Guest exposure is handled separately by tasks qm3/sm3.

Read-only baseline on 2026-10-22:

| Check | Result |
|---|---|
| f0/f1 PF | Enabled, `pass in quick all`; f0 MASTER, f1 BACKUP |
| f2 PF | Module absent; pf_enable/pflog_enable NO |
| f0–f2 bridge | vm-public with re0/tap0; both pfil settings 0 |
| f0–f2 WG | wg0, UDP 56709 |
| r0–r2 NFS | Hard NFSv4.2 mount via localhost:2323; mount directory stat succeeds |
| f0–f2 candidate | New rules parse with `pfctl -nf -` on f0/f1; f2 requires module load first |
| f3 | Unreachable on bounded LAN and WG SSH; **inspection and deployment still required** |
| f0 LAN ULA | Already marked `detached`; ULA ping fails before this change |

Do not attribute the existing detached-ULA failure to PF or claim an end-to-end
ULA success without resolving it. Do not wake/reboot f3 without approval.

## Preflight (read only)

From the repository root, record the commit and build/inspect the plan:

```sh
git rev-parse HEAD
./gonf.sh -list | rg freebsd_firewall
./gonf.sh plan -redacted freebsd_firewall_config
```

For each host, inspect actual interfaces, bridge settings, CARP state and WG
listen port before deploying. Use LAN SSH below; if unavailable try
`fN.wg0.wan.buetow.org` with the same explicit port and bounded options.
If neither path works, stop that host's rollout. Never infer f3's topology
solely from the other hosts. Do not print wg0.conf, rc.conf or CARP keys.

```sh
host=f1
ssh -oBatchMode=yes -oConnectTimeout=5 -oConnectionAttempts=1 \
  -oServerAliveInterval=5 -oServerAliveCountMax=1 -p22 \
  "paul@$host.lan.buetow.org" sh <<'SH'
hostname
ifconfig -l
ifconfig re0 | sed -n '/inet /p;/inet6 /p;/carp:/p'
ifconfig vm-public | sed -n '/member:/p'
sysctl net.link.bridge.pfil_member net.link.bridge.pfil_bridge
doas -n wg show wg0 listen-port
doas -n pfctl -si
SH
```

On f0/f1, validate the source without writing a remote file:

```sh
ssh -oBatchMode=yes -oConnectTimeout=5 -p22 paul@f1.lan.buetow.org \
  'doas -n pfctl -nf -' < f3s/freebsd-hosts/pf/pf.conf
```

FreeBSD 15's parse-only pfctl needs PF's netlink family. Failure on f2/f3
without the module is expected; Gonf loads the module (without enabling PF)
before validating its candidate during the approved apply.

## Approved rollout: one host at a time

Use **f1 (current BACKUP), f0, f2, f3**; recheck roles and reverse the pair
if f0 is the actual BACKUP. Complete each host's checks before proceeding.
Keep two authenticated SSH sessions open, preferably LAN and WG, with JetKVM
available. Keep the old sessions while testing **new** connections.

First arm a 15-minute automatic rollback **on the current host**. This saves
root-only configuration copies without displaying their contents. The fixed
directory intentionally refuses reuse: archive a previous completed guard
only after confirming it has finished and is no longer needed.

```sh
host=f1
ssh -oBatchMode=yes -oConnectTimeout=5 -p22 "paul@$host.lan.buetow.org" \
  'doas -n sh' <<'SH'
set -eu
d=/root/pf-rm3-rollback
mkdir -m 700 "$d"
cp -p /etc/rc.conf "$d/rc.conf"
cp -p /etc/sysctl.conf "$d/sysctl.conf"
cp -p /boot/loader.conf "$d/loader.conf"
if [ -f /etc/pf.conf ]; then cp -p /etc/pf.conf "$d/pf.conf"; fi
if pfctl -s Running >/dev/null 2>&1; then touch "$d/pf-enabled"; fi
if service pflog onestatus >/dev/null 2>&1; then touch "$d/pflog-running"; fi
sysctl -n net.link.bridge.pfil_member > "$d/pfil_member"
sysctl -n net.link.bridge.pfil_bridge > "$d/pfil_bridge"
cat > "$d/rollback" <<'ROLLBACK'
#!/bin/sh
set -eu
d=/root/pf-rm3-rollback
cp -p "$d/rc.conf" /etc/rc.conf
cp -p "$d/sysctl.conf" /etc/sysctl.conf
cp -p "$d/loader.conf" /boot/loader.conf
if [ -f "$d/pf.conf" ]; then
  cp -p "$d/pf.conf" /etc/pf.conf
else
  rm -f /etc/pf.conf
fi
if [ -f "$d/pf-enabled" ]; then
  pfctl -nf /etc/pf.conf
  pfctl -f /etc/pf.conf
else
  if pfctl -s Running >/dev/null 2>&1; then pfctl -d; fi
fi
rm -f /var/db/gonf-pf.sha256
sysctl "net.link.bridge.pfil_member=$(cat "$d/pfil_member")"
sysctl "net.link.bridge.pfil_bridge=$(cat "$d/pfil_bridge")"
if [ ! -f "$d/pflog-running" ] && service pflog onestatus >/dev/null 2>&1; then
  service pflog onestop
fi
ROLLBACK
chmod 700 "$d/rollback"
nohup sh -c 'sleep 900; [ -f /root/pf-rm3-rollback/accepted ] || exec /root/pf-rm3-rollback/rollback' \
  > "$d/watchdog.log" 2>&1 < /dev/null &
echo "$!" > "$d/watchdog.pid"
SH

# Apply only to the one registered host whose guard is armed.
./gonf.sh push -privilege=doas -- -p22 -oBatchMode=yes \
  -oConnectTimeout=5 -oServerAliveInterval=5 -oServerAliveCountMax=2 \
  "paul@$host.lan.buetow.org" freebsd_firewall_config
```

`push` requires explicit SSH target, port and privilege; `push f1` does not
inherit the registered host settings and fails with privilege=none. For a
necessary WG fallback use the explicit equivalent:

```sh
./gonf.sh push -privilege=doas -- -p22 -oBatchMode=yes \
  -oConnectTimeout=5 "paul@$host.wg0.wan.buetow.org" freebsd_firewall_config
```

Gonf validates before publishing the candidate, loads using `pfctl -f`, and
only enables PF if disabled. It never flushes states or restarts PF/VMs/NFS.
Reloading an already-enabled PF preserves its existing states. **First
activation has no existing PF states**: established host TCP sessions may
drop because the pass rules expect an initial SYN. The two pre-activation
f2 SSH sessions did close during the observed rollout; fresh LAN and WG SSH
worked immediately afterward. The independent rollback timer remains the
recovery path if reconnection fails. Do not promise old-session survival
when enabling PF for the first time.
A root-owned `/var/db/gonf-pf.sha256` records the file's checksum only after
a successful load, so a failed load is retried even if the file is already
published. If any step fails, stop and investigate; use the guard to roll
back. After an intentional manual ruleset replacement, remove that checksum
before the next approved Gonf apply so the canonical rules are loaded again.

Immediate manual rollback from an existing session or JetKVM:

```sh
doas -n sh /root/pf-rm3-rollback/rollback
```

For f2/f3 rollback restores their original disabled state and leaves the PF
module loaded harmlessly. It restores rc.conf, sysctl.conf and loader.conf
as well as the original rules. Do not make unrelated configuration changes
while the guard is armed because those files would be restored in full.

## Required acceptance before disarming each guard

1. New SSH over LAN IPv4 **and WG** succeeds to the changed host; check LAN
   ULA/link-local and record the known detached-ULA baseline separately.
   `doas pfctl -si` reports Enabled; `doas pfctl -sr` shows the default block.
   `sysctl net.link.bridge.pfil_member net.link.bridge.pfil_bridge` remains 0.
   Reapply the same single-host task: no config/rules/service changes.
2. On **each r0–r2**, `findmnt -T /data/nfs/k3svolumes` still shows the hard
   NFS mount, and `timeout 5 stat /data/nfs/k3svolumes` succeeds. For an
   uncached read/write check (approved live test), create a unique scratch
   file under that mount on r0, read it from r1/r2, then remove it. Do not
   restart stunnel or remount NFS. `kubectl get nodes` remains Ready and
   `ssh -p22 root@rN.lan.buetow.org true` works for all three guests.
   A concrete scratch check (repeat while the alternate CARP host serves):

   ```sh
   scratch="/data/nfs/k3svolumes/.rm3-pf-check-$(date +%s)"
   ssh -oBatchMode=yes -oConnectTimeout=5 -p22 root@r0.lan.buetow.org \
     "timeout 5 sh -ec 'printf rm3-pf-ok | dd of=$scratch conv=fsync status=none'"
   for node in r0 r1 r2; do
     ssh -oBatchMode=yes -oConnectTimeout=5 -p22 "root@$node.lan.buetow.org" \
       "timeout 5 cat $scratch"
   done
   ssh -oBatchMode=yes -oConnectTimeout=5 -p22 root@r0.lan.buetow.org \
     "timeout 5 rm $scratch"
   ```

3. From LAN, `nc -z -w3 192.168.1.138 80` and port 443 succeed. Check an
   actual application through Traefik with its usual Host/SNI; Garage S3
   still works from r-nodes/frontends and cluster health remains healthy.
   `nc -z -w3 192.168.1.130 3901` from f1 checks an RPC transport, while
   `ssh rex@blowfish.buetow.org 'nc -z -w3 192.168.2.130 3900'` checks the
   frontend mesh path. Do not print Garage tokens.
4. `doas zrepl status --mode raw` shows successful recent f0→f1 and f3→f2
   replication; observe a new successful interval (1 min / 10 min). Verify
   the f1 `nfs.LIVE_CHECK` canary age remains under ~660 seconds. If f3 is
   unavailable, f3→f2 cannot pass; do not mark rm3 complete.
5. From **blowfish's public interface**, obtain each host's current global
   IPv6 from the preflight, verify `route -n get -inet6 ADDRESS` uses `vio0`,
   then run `nc -6 -z -w3 ADDRESS PORT` for **22, 111, 2049, 2222**. All
   fresh probes must fail on every f-host. A failure caused solely by qm3's
   router filtering is insufficient evidence of host PF: check block rule
   counters / `doas tcpdump -n -i pflog0` during probes, or use a LAN probe
   forced to a **global IPv6 source**. Previously accepted PF states survive
   reload; use fresh connections, and do not flush NFS/SSH states.

After checks on that host pass, cancel its watchdog **before** moving to the
next host. Keep the backup for manual recovery:

```sh
ssh -oBatchMode=yes -oConnectTimeout=5 -p22 "paul@$host.lan.buetow.org" \
  'doas -n touch /root/pf-rm3-rollback/accepted'
```

After both CARP members pass, perform the **explicitly approved** failover
test using the f3s skill's `references/storage/carp.md` procedure: disable
f0 auto-failback, demote the current MASTER, verify peer becomes MASTER,
then repeat NFS **reads** and VIP 80/443 application checks. The CARP hook
intentionally keeps f1's replica `readonly=on` while MASTER; confirm writes
are rejected there and never make the receiver writable for this test.
Prepare a scratch file on f0 and wait for replication before takeover; after
restoring f0, verify fresh writes and remove the scratch file.
Restore the original MASTER and auto-failback state. Do not change the CARP
password, restart relayd/NFS/stunnel, or force failover during preparation.
Arm a fresh guard if changing either firewall again during this test.

For the verified baseline of f0 MASTER/f1 BACKUP, first record whether
auto-failback is already disabled, then use these commands. If roles differ,
adapt the procedure to the recorded original state before executing it. Arm
an independent timed restore of **both** the original skew and MASTER/auto-
failback state before changing either member. The PF guard restores rules,
not CARP roles. Do not repeat this test while the NFS issue below is unresolved:

```sh
ssh -p22 paul@f0.lan.buetow.org 'doas -n carp'
ssh -p22 paul@f0.lan.buetow.org 'doas -n carp auto-failback disable'
# Temporarily lengthen f0's election timeout so f1 can take over.
# carp backup alone can let lower-skew f0 elect itself again first.
ssh -p22 paul@f0.lan.buetow.org \
  'doas -n ifconfig re0 vhid 1 advskew 254 state backup'
ssh -p22 paul@f0.lan.buetow.org 'doas -n carp state'
ssh -p22 paul@f1.lan.buetow.org 'doas -n carp state'
# Require f0 BACKUP / f1 MASTER; verify replicated scratch reads, expected
# read-only rejection of writes, and VIP app checks. Do not require writes
# to succeed on the intentionally read-only backup.
# Restore the recorded original skew (0 in the observed baseline).
ssh -p22 paul@f0.lan.buetow.org 'doas -n ifconfig re0 vhid 1 advskew 0'
ssh -p22 paul@f0.lan.buetow.org 'doas -n carp master'
ssh -p22 paul@f0.lan.buetow.org 'doas -n carp state'
ssh -p22 paul@f1.lan.buetow.org 'doas -n carp state'
# Require f0 MASTER / f1 BACKUP and working NFS/VIP again.
# Only if auto-failback was originally enabled:
ssh -p22 paul@f0.lan.buetow.org 'doas -n carp auto-failback enable'
```

Acceptance includes all four hosts, new WG handshakes, working guests and
both replication streams. A scheduled reboot can later verify boot ordering;
it needs its own approval and must preserve the storage pair's availability.


## Live rollout evidence — 2026-10-22

- **f1** applied 12:27:00 EEST, **f0** 12:29:11. Each used two independent
  LAN/WG root sessions and the 15-minute rollback guard; guards were accepted
  after checks. Explicit SSH target/doas/port commands above succeeded.
  Each reapply reported `5 ok, 0 changed, 5 skipped`. The initial short
  `push f1` failed locally with privilege=none before applying anything.
- On each host, new LAN/WG SSH works, public IPv6 TCP 22/111/2049/2222 via
  blowfish `vio0` is blocked, PF block counters increase, bridge hooks stay 0,
  and boot enables are YES. Public TCP22 was open immediately before rollout.
  Cross-host link-local IPv6 ping/TCP22 works. A guest cannot reach its own
  host's link-local address even on f2 **before** PF; no PF ICMPv6 drops were
  observed for that existing colocated-guest path.
- r0–r2 mount/stat and unique scratch write/read/cleanup passed before CARP
  testing. Miniflux through the VIP returns HTTP200; VIP80/443 transport works;
  Garage reports all three nodes healthy and frontend WG S3 connections pass.
  f0→f1 replication completed after each PF load (12:27:14 and 12:29:15), with
  no replication plan/step error. Existing pruning errors concern held older
  snapshots; no snapshot holds were changed.
- **CARP networking transitioned**, with an independent 180-second restore
  guard: temporarily setting f0 advskew254/BACKUP let f1 become MASTER. Its
  NFS/stunnel services ran and its replica correctly remained readonly=on.
  **NFS failover acceptance failed:** the first uncached read of a scratch
  file already replicated to f1 returned `Stale file handle`. All three
  clients logged NFSv4 open-file state-recovery error -116 during takeover.
  The test aborted before the f1 write-rejection and VIP application checks.
  Exact UTC syslog timeline: f0 BACKUP 09:35:52 (services stopped 09:35:53),
  f1 MASTER 09:35:55 (services started 09:35:56), f0 MASTER restored 09:35:57
  (services started 09:35:58), f1 BACKUP 09:35:57 (services stopped 09:35:58).
  Client ESTALE recovery messages occurred at 09:35:56–57 UTC.
- f0 was promptly restored to **MASTER, advskew0, auto-failback enabled**;
  f1 is BACKUP. No manual NFS/WG/VM restart, remount or readonly change was
  attempted. Pools are healthy, VIP app returns HTTP200, new file creation+
  file-level fsync and uncached reads pass across all three r-nodes. A
  filesystem-wide `sync -f` returned EIO after takeover, potentially surfacing
  earlier workload writeback errors; existing application open handles need
  separate recovery assessment. Do not claim seamless NFS/application failover.
- Necessary test recovery restarted **only Forgejo** at 09:43:20 UTC to reopen
  stale SQLite handles. Its replacement rolled out 1/1; `/explore/repos` and
  `snonux/conf.git/info/refs?service=git-upload-pack` returned 200, LAN
  `git ls-remote` succeeded, and fresh storage-error counts stayed zero.
  `/api/healthz` alone had returned 200 while repository operations failed,
  so it was not used as the recovery oracle. No other workload was restarted.
- **f2** applied 09:44:53 UTC after that recovery, under its own dual-session
  and timed-rollback guard. First activation loaded PF/pflog; reapply reported
  `5 ok, 0 changed, 5 skipped`. New LAN/WG SSH, cross-host link-local TCP22,
  all four public IPv6 rejections, increasing PF block counters, bridge0,
  boot enables YES, NFS file fsync/direct reads across r0–r2, Ready nodes,
  Garage health/frontend S3 and Miniflux VIP200 passed. The two original
  pre-activation SSH sessions closed as noted above; no lockout occurred.
  The guard was accepted. f2's WG zrepl TCP8888 listener is reachable;
  actual f3→f2 replication remains unverified.
- New WG handshakes after rollout were observed at 09:45:17 UTC on f0/f1
  and 09:45:21 on f2; f0→f1 replication completed 09:45:14 with no plan error,
  and the replicated canary age was 353 seconds. Unique test files were
  removed after checks. f0 remains MASTER/skew0/auto-failback enabled and
  f1 BACKUP; no further failover was attempted.
- f3 inspection and deployment remain pending.
  f3→f2 replication is unverified while f3 is unreachable. No f3 wake/reboot
  is authorized. The overall rm3 task remains incomplete; follow-up **ao3**
  tracks the failed NFS failover acceptance and recovery limitation.
