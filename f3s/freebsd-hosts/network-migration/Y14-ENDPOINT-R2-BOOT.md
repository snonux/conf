# r2 endpoint canary actual boot — root review proposal

Prepared after r2's independent inner-expiry and confirmed outer-expiry trials
were restored, archived and reapplied with no pending record. The JetKVM
read-only inspection window is closed; root still coordinates this separate
boot slice. At this pre-execution checkpoint no actual boot action or diagnostic
drop-in had been installed. Subsequent causal native results are recorded in
[Y14-ENDPOINT-NATIVE.md](Y14-ENDPOINT-NATIVE.md).

Use frozen generator 4d71e3f helper SHA256
`0a8b54b416cd493f04f14c5e8ea095ed70c8ae969044b4af664baa153616c197`
and the unchanged four reviewed units. New private scope differs only by ID
`y14_r2_f3_boot_20261105`; same existing f3 identity, old .1.133:56709 to
.20.133:56709, protocol canary, same source SHA41386d… and fixed saved/wg paths.
Exact config bytes SHA256:
`eec3041212b0a32f884fc6bb5531f5541a86475c73497421095ff608f8637da6`.
Freeze/verify the source YAML remains OLD and no production controller marker.

## Preflight and evidence instrumentation

Full preboot health must pass three Ready nodes, all three members reporting an
etcd leader, API/Service, NFS consistency, cross-node TCP/UDP DNS and verified TLS.
Recheck r2's native updater inactive/actual next time immediately before arming
and before reboot. Last observed next was 14:45 UTC; after reboot its independent
OnBootSec run will move to about ten minutes after the new boot. Do not modify
that schedule. Reboot only r2's OS, leaving f2/f0/f1 and the other etcd members
untouched. f3 and its guests do not reboot in this slice.

After preboot health passes, delete only this task's r2 diagnostic pod with its
exact UID precondition **before arming**. It has restartPolicy Never; leaving
it across reboot would create a failed probe unrelated to production health.
Keep the r1 probe, and recreate a fresh owned r2 probe for postboot health.

To demonstrate saved OLD before wg0 loads, propose two temporary read-only
ExecStartPost assertions, installed only while pending is absent:

```ini
# /etc/systemd/system/gonf-endpoint-rollback-boot.service.d/90-y14-proof.conf
[Service]
ExecStartPost=/usr/bin/python3 -B /var/lib/gonf-endpoint/y14-proof/proof.py boot
```

```ini
# /etc/systemd/system/gonf-endpoint-rollback-runtime.service.d/90-y14-proof.conf
[Service]
ExecStartPost=/usr/bin/python3 -B /var/lib/gonf-endpoint/y14-proof/proof.py runtime
```

Exact private proof source:
`~/.local/state/conf/y14-preparation/endpoint/reviewed-4d71e3f/boot-proof.py`,
SHA256 `a37c9541490745886a9576e8408936257ee8a9e9550988119de8cd20fb65b7ce`.
It only reads state/configuration and prints a sanitized receipt; it does not
write keys, endpoints, routes, services or the journal. Root-owned private
`y14-proof/context.json` binds the unique scope and preboot kernel boot ID.
The boot assertion requires a changed kernel ID, phase boot-restored, saved
file byte-exact OLD and time **before the inner deadline**. The runtime assertion
requires phase restored, saved OLD, selected runtime exactly its captured
endpoint and strict projected non-Endpoint semantics before k3s startup.

The existing boot unit is before wg0; its ExecStartPost completes before that
unit succeeds. The existing runtime unit is before k3s. Thus native journal
ordering and the receipts measure the required surfaces at those boundaries,
rather than inferring early persistence restoration from a later runtime fix.
Restore SELinux labels, validate units/drop-ins and inspect actual ordering
before arming. The source-owned recovery helper/four unit bodies remain intact.

## Exact trial sequence

1. No pending address or endpoint journal; fresh old/new f3/r2 literal SSH,
   destination route/UDP rules, source digest and selected saved/live semantics.
   Replace only the dormant scope config with the reviewed boot ID/config;
   verify helper/config/four units and diagnostic bytes/modes.
2. Generate host-bound canary permission with inner now+120, outer now+240;
   write only canary-approved/apply-approved. Snapshot/apply. Immediately prove
   selected saved/live NEW and fresh bidirectional overlay IPv4/IPv6 traffic.
   Recheck fresh r2 old/new management and updater inactive/next execution.
3. While well before the inner deadline, request a normal `systemctl reboot`
   on r2 only. No canary confirm or production release. Capture the actual new
   kernel boot ID, boot proof OLD **before wg0**, runtime proof OLD **before
   k3s**, unit startup timestamps and old-management reconnect before inner
   expiry. An inner/outer timer restoration after a slow boot is not accepted
   as the requested early boot proof.
4. Compare saved exact original, selected actual runtime captured original,
   unchanged non-Endpoint fields, all old/new addresses/defaults and source
   digest. Full cluster/storage/DNS/Service/TLS and overlay health must pass,
   including fresh owned r2 probe. Explicitly restore twice, preserve/privately
   archive the complete scope with lock held and pending absent.
5. Remove only the two introduced proof drop-ins, daemon-reload and revalidate
   the four original units. Retain private proof/evidence, reapply the dormant
   six-file scope installation and verify no journal or authorization appears.
   Delete only owned probe UIDs. Report no source promotion or production
   endpoint acceptance. Reverse f3 canaries remain a subsequent root slice.

## Failure recovery

The diagnostic assertions add no network writer. If one fails, its existing
required dependency can hold wg0/k3s startup; physical NetworkManager and the
accepted old/new SSH paths remain independent. Keep the private journal and
independent watcher. Reconnect through literal physical management, inspect
the failure, remove only the two diagnostic drop-ins and daemon-reload.
Use the existing persistent/runtime restore protocol and start the original
wg0/runtime/k3s units in their reviewed dependency order if startup was held.
Never replace an entire stale tunnel merely to change an endpoint, delete a
pending journal, stop another etcd member or depend on a physical console.
An evidence failure is reported as a failure; it is not converted into a boot
pass after cleanup. Root coordinates any recovery before another native trial.
