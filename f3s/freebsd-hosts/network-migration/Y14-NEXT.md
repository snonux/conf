# y14 next native slices — proposal for root coordination

Prepared 2026-11-05 after standalone guest expiry/boot/confirmation. These
operations require separate root coordination per slice. The first r2-to-f3
installation/expiry slice was approved after guest former-deadline observations
and FreeBSD's graceful return to its normal stopped state; task y14 annotations
record its actual execution. Later slices remain proposals.
Only one address or endpoint transaction may be pending anywhere in this pilot.
No generator source promotion, all-host rollout, physical work or old-address
retirement is included.

## 1. Independent endpoint canaries, with old physical primaries retained

Use generator commit 511aa23 and doc correction 9c97b26:
`~/git/wireguardmeshgenerator/endpoint/NATIVE-CANARY.md`. Start r2 referring
to f3; restore/archive every trial before the reverse f3-to-r2 direction.
Neither standalone guest is an endpoint canary. The FreeBSD guest's existing
identity mismatch remains untouched.

Preflight each direction afresh:

- Check f3/r2 and both guests have no address journals; confirm both endpoint
  paths have no previous record; inspect actual unattended service/lock/next
  execution. f3's verified policy forbids automatic reboot, but any actual
  interfering service writer still matters. r2 can reboot, including its
  OnBootSec window after a controlled reboot.
- Verify destination accepted old/new IPv4 plus native ULA, old default .1.1,
  laptop 10.99.0.2/32 return through 192.168.1.2, and fresh authenticated SSH
  to both literal addresses. From the referring host, capture route/source
  selection, native UDP56709 permission, handshake and traffic baselines.
- Capture private saved/runtime wg0 semantics and the exact authoritative YAML
  digest. Bind destination identity by matching actual saved/live public peer
  identity and generator public identity. Never read private keys into logs.
  Refuse unrelated drift. Source YAML remains old throughout these canaries.
- Validate interpreter ownership/version: r2 `/usr/bin/python3`; f3 package
  python312's `/usr/local/bin/python3.12` (no hand-made python3 symlink).

Review concrete root-owned helper/config/unit installation diffs before
applying. Existing endpoint paths are currently unmanaged; install only the
reviewed generator assets at the exact README paths. Retain before-images and
reapply the same scoped installation with no record present; never run the
whole-mesh installer. Configure no production approval marker and no controller
transaction. A separate unique private scope is required per native trial.

Linux installs endpoint.py at `/usr/local/sbin/gonf-endpoint-rollback`, config
`/usr/local/etc/gonf-endpoint.json`, private `/var/lib/gonf-endpoint/pending`,
and the four reviewed systemd units. Inspect SELinux labels and actual
wg-quick@wg0 startup ordering; activate the native watchdog/boot/runtime units
only after review. FreeBSD uses the same executable/config paths, private
`/var/db/gonf-endpoint/pending`, both reviewed system rc.d hooks and the exact
system-crontab Python3.12 watchdog line from the current README. Verify rcorder
and cron execution. No existing wg0 startup file or tunnel configuration is
replaced by provisioning.

For each direction, perform all three separate trials:

1. Inner expiry: canary protocol, inner 120 seconds/outer 240 seconds, private
   root-owned `canary-approved.json` and fresh `apply-approved` bound to the
   scope. Snapshot/apply only that one endpoint. Verify saved/live semantics,
   bidirectional fresh traffic, handshake/transfer progress, overlay IPv4/IPv6
   and required cluster/storage/ingress health, then close the trial SSH
   sessions. Let the native inner watchdog restore. Fresh old management must
   show saved OLD and the exact captured runtime endpoint; run restore twice.
2. Confirmation/outer expiry: new scope, same bounded deadlines, repeat fresh
   acceptance and create only `canary-confirm-approved`. Confirm, then observe
   NEW beyond the inner deadline plus watcher cadence with fresh traffic.
   Disconnect and deliberately let the outer lease restore even the confirmed
   canary. Reconnect, compare exact captured state and archive privately.
3. Boot: new scope, unconfirmed apply, then a coordinated actual native reboot
   within its outer lease. r2 reboots alone with the other two etcd members
   healthy. Before f3 reboot, gracefully stop all of its running guests and
   verify VM state; reboot f3 normally, then verify rocky autostarts and
   freebsd remains stopped. Prove saved OLD restoration before wg0 startup,
   runtime restoration after wg0, real changed boot identity and full health.
   A timer expiry after a slow boot is not a substitute for early boot proof.

After each complete direction, reapply only the dormant endpoint tooling with
no pending record and prove no endpoint mutation or new authorization. Archive
scope/config/evidence privately and remove all canary assertions. Keep YAML
digest unchanged. Production recovery approval is a later separate assertion.

These isolated tests do not satisfy the full f3 endpoint migration. The actual
persisted referring set is f0–f2/r0–r2 and must be freshly revalidated. Old-only
f0/r0 currently route .20 through .1.1. Do not omit them from a production
manifest, issue false reachability approval, or commit an r2-only source
change. A complete source migration waits for their separately scoped routing
or dual Stage work; that is outside this one-host rollout.

## 2. f3 primary/gateway trial under a source-owned transport guard

Only after endpoint canaries are fully restored/archived, obtain root review of
the prepared `f3s/freebsd-hosts/pf/pf.conf` diff and exact scoped native preview.
The narrow outbound guard denies .20.133 UDP56709 to the six old-LAN referring
addresses before pass-out, preventing unreachable learned endpoint changes.
It changes no key, peer, source YAML or endpoint.

Immediately before loading the guard, capture complete `pfctl -ss -vv` output
privately and inspect both original and translated tuples for every guarded
destination/UDP56709. **Any relevant existing state means refuse/wait for its
natural expiry.** Do not flush all states or assume a new filter rule overrides
state admission. The earlier 12:09 capture is stale preparation, not permission.
Preserve unrelated PF states and bridge bypass. Native `pfctl -nf` and the
scoped `freebsd_firewall_config` preview must pass. Apply that owner to f3 only;
verify installed bytes/rule, fresh old/new SSH and unchanged old-source wg0
transport before arming a primary transaction.

The guard is deliberately dormant under accepted old-source transport and
remains in the owner source across rollback/reapply. An address rollback needs
no successful filter deletion to restore old-source packets. If loading the
guard itself has unexpected effects, restore the privately saved PF file/rules
through the same owner immediately, with states preserved; do not proceed.

Arm `gonf-network-migration promote shared 900` on f3, snapshotting its accepted
dual Stage. Keep old .1.133, local ULA and laptop return via .1.2. No isolated
profile or .98 gateway is permitted. While pending:

- Hold fresh authenticated old/new literal SSH and prove independent new
  reconnects. Confirm actual primary .20.133/default .20.1 and retained old IP.
- Observe UDP outer source/handshakes/counter progress for every old-LAN
  referring peer. Capture before/after public-key-indexed endpoints privately
  on both sides. If the guard drops a new-source packet or any required path
  fails, immediately restore the address transaction; never modify remote
  endpoints to make this trial pass. Recheck old source/traffic after restore.
- Prove new-source public IPv4 HTTPS explicitly bound to .20.133 while router
  packet observation shows ingress source .20.133 and egress translation to
  .1.2. This is an acceptance check inside the guarded trial. Do not claim it
  from old-primary egress or wg2 SSH. Make no router configuration change.
- Check configured DNS, direct Pi-hole DNS, wg0 overlay IPv4/IPv6, native local
  ULA diagnostics, current zrepl/VM state and cluster/NFS/ingress health. Keep
  the documented controller wildcard DNS limitation separate.

First deliberately expire a bounded primary trial and compare restored Stage,
then explicitly restore twice. A separate unconfirmed actual f3 reboot trial
must restore accepted Stage before netif; gracefully stop guests beforehand,
preserve rocky autostart/freebsd stopped policy and check service health after
return. Neither trial shares an endpoint journal. Only then run a distinct
final Promote, confirm from new SSH with independent old access held and
required checks passing, observe beyond its former deadline and perform scoped
owner reapply. Keep the PF source guard until a separately reviewed endpoint
and referring-host return-path stage makes it unnecessary.

## 3. Standalone guest primary/gateway trials, serially

After f3 acceptance and no journal pending, repeat the same native
expiry/explicit-restore/actual-boot/final-confirm/reapply contract for rocky,
then freebsd. Each new snapshot is its accepted dual Stage; old .123/.139 and
laptop /32 via .1.2 remain. Use the existing standalone helper branches:
rocky has no k3s operation; both keep native physical IPv6 disabled/no guest
ULA. Recheck the guest's actual updater before every arm/reboot. Its new
default .20.1 is tested by explicitly new-bound public IPv4 HTTPS and router
NAT observation while pending, then DNS and required literal service access.

Their current wg0 configurations contain public gateway endpoints only, so
there is no old-LAN peer endpoint substitution in either guest. Verify that
fact privately before relying on it. Preserve exact configuration/identity:
rocky overlay IPv4/IPv6 must remain healthy; freebsd's unchanged known-broken
identity/zero-handshake baseline is recorded honestly. Do not treat that
separate repair as permission to rotate keys. Return freebsd to its normal
stopped state after accepted persistence checks, leaving rocky running/AUTO.

Each phase stops at its accepted boundary for root coordination of the next
slice. No operation in this proposal alone makes full endpoint source promotion,
old aliases' retirement or physical port1 cutover eligible.
