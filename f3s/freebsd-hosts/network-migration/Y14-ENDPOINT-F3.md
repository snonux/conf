# Reverse first slice: f3 referring to r2 — root review proposal

Prepared 2026-11-05 after all three r2-direction native proofs, cleanup and
archive. Root reserves a separate guarded router PF window first. This file
did not itself authorize installation or a live reverse trial. No endpoint
tooling had been installed on f3 at this pre-provision checkpoint. Subsequent
native results are recorded in [Y14-ENDPOINT-NATIVE.md](Y14-ENDPOINT-NATIVE.md).

## Fresh read-only preparation

At 14:34–14:36 UTC, f3's endpoint helper/config/state/two rc.d paths were
absent, cron was active, and package python312 supplied Python 3.12.15 at
`/usr/local/bin/python3.12`. FreeBSD guest was stopped, rocky running/AUTO,
and the parent address journal had no pending state. New strict admission
against fresh actual f3 saved/runtime configurations passed all interface and
peer fields using the reviewed exact-prefix kernel projection. Actual r2
public identity matched generator `keys/r2/pub.key`; no field was changed.

r2's postboot updater ran 14:34:44–14:34:45 successfully/inactive, next 14:45.
Recheck that actual run/next time before any trial, and avoid pending endpoint
work across potential destination reboot. Do not delay or alter its independent
schedule. Refresh router/wg2 paths after root's separate PF window is accepted
with no router journal pending.

Private exact artifact/installation manifest:
`~/.local/state/conf/y14-preparation/endpoint/f3-reverse-reviewed/f3-reverse-install-manifest.json`.
All code is frozen from reviewed git object 4d71e3f, not a moving checkout.

## Exact scope and proposed installation

Scope ID `y14_f3_r2_expiry_20261105`, protocol canary, role peer, interface wg0;
existing peer public identity
`Y8Jsk4sVuofNP+u18PkBhw3c/9T0t+gujVL9G4jsC08=`;
OLD `192.168.1.122:56709`, NEW `10.20.0.122:56709`;
source SHA256 `41386d059b09101a4a4f8e2a5315345a2d6180ae111fb483ada527d9a9e653d0`;
saved `/usr/local/etc/wireguard/wg0.conf`, wg `/usr/local/bin/wg`.

Install only these currently absent root:wheel files:

| Destination | Mode | SHA256 |
|---|---|---|
| /usr/local/sbin/gonf-endpoint-rollback | 0755 | 0a8b54b416cd493f04f14c5e8ea095ed70c8ae969044b4af664baa153616c197 |
| /etc/rc.d/gonf_endpoint_rollback | 0755 | 930b9a7e0905c340db21df7a26c704143c7548cbf9305599888281927f018623 |
| /etc/rc.d/gonf_endpoint_runtime | 0755 | c07d9dfb0680bb007bda6d29969280d0f46babb683e1495eeac64c11ca89ea77 |
| /usr/local/etc/gonf-endpoint.json | 0600 | 6829a8045160889f608babee47083260484f8510e2275378443e4dbf52e7af82 |

Create only private root:wheel 0700 `/var/db/gonf-endpoint` and empty pending.
Preserve before-images/absence receipts privately. Existing unmanaged system
`/etc/crontab` receives an append-only marker and this exact line; do not edit
root's gonf-managed crontab or task 914:

```cron
* * * * * root /usr/local/bin/python3.12 /usr/local/sbin/gonf-endpoint-rollback watchdog /var/db/gonf-endpoint/pending
```

Before crontab SHA256 2b5087f799f4b0c1bc806a1bb5da4a8b17cb8096af949bb1dbecdd5c522d83b1;
append-only candidate SHA256 6c21abf36edb61f91259bed1d50066a36260f4e2aa263c1811dc3c65fc70a768.
Refuse/review if the actual before-file changed; preserve all other bytes,
ownership, mode and metadata. No service/interface restart is part of provisioning.

Every operator action, hook and cron uses package-owned Python3.12 explicitly;
never invoke the helper's env-python3 shebang on f3 or create a python3 symlink.
Native Python compile, rc shell syntax, rcorder across system/local scripts and
cron readiness must pass. Early hook must precede netif/wireguard; runtime hook
must follow wireguard and precede LOGIN. Execute harmless scope/readiness with
the exact interpreter and prove native cron command dispatch before trusting
expiry. No permission file is created merely because provisioning passed.

## Only the first inner-expiry trial proposed here

After root's router transaction is confirmed/no pending, refresh old/new literal
SSH to f3/r2, old defaults/dual addresses/laptop 10.99.0.2/32 return via
192.168.1.2, route/source and
actual UDP56709 arrival, native UDP firewall allowances, private semantic
snapshots, source OLD digest and service/scheduler health. Use the same owned
y14 health-probe UID rules as the r2 direction. Preserve all existing public
gateway endpoints, keys, AllowedIPs and known standalone-guest baseline.

Generate fresh host-clock canary permission bound to the exact config, inner
now+120/outer now+240, and fresh apply-approved ID. Snapshot/apply only the r2
Endpoint on f3. Verify saved/live NEW, only selected endpoint semantic delta,
fresh bidirectional overlay IPv4/IPv6/handshake/transfer, management and required
cluster/NFS/DNS/Service/TLS health. Then close the origin and probe SSH sessions.

Allow independent native cron expiry. With its one-minute cadence, allow the
post-inner tick; reconnect and independently observe restored saved OLD bytes
and actual captured runtime OLD **before any manual restore and before outer
expiry**. Otherwise report the actual failure; do not relabel outer recovery
as inner proof. Only then explicitly restore twice, check all protected fields
and services, archive the complete scope with the helper lock held, and reapply
the exact dormant installation with no new record/authorization. Delete only
owned diagnostic UIDs.

The prior r2 lease used a 15-second timer; do not reuse its +15-second observation
assumption for FreeBSD's one-minute cron. Native confirmation/outer and actual
f3 boot remain separate root slice reviews. No canary confirm, production
release, source YAML promotion, primary/default change or parent reboot is
included in this first reverse expiry proposal.

## Failure and provisioning rollback

Keep both destination aliases throughout. Use only the scoped helper's restore
actions; retain failed records and cron for retries. Do not replace an entire
tunnel or modify AllowedIPs to pass admission. After a successful scoped
restoration, preserve/archive evidence before removing a record.

If installation must be undone, require no pending record, restore only the
exact reviewed system-crontab before-image if its current content is still the
exact candidate (refuse unexpected third-party drift), and remove only the
four introduced helper/config/rc paths. Keep private archives; do not delete
the state tree or unrelated cron/rc entries. Existing wireguard startup, host
networking, VM policy and managed unattended configuration stay with their
current owners.
