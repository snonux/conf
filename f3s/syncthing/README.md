# Syncthing

Web UI `https://syncthing.f3s.buetow.org`, sync port 22000 (in the cluster;
the laptop reaches it as NodePort 32200, see below). Namespace `services`,
ArgoCD app `syncthing`.

| PV | hostPath |
|---|---|
| `syncthing-config-pv` | `/data/nfs/k3svolumes/syncthing/config` |
| `syncthing-data-pv` | `/data/nfs/k3svolumes/syncthing/data` |

```sh
just status | logs [lines] | port-forward [8384] | sync | argocd-status | restart
```

On 2026-10-19, version 2.1.6 resumed real transfers after cluster startup.
The `Galaxy Tab S10 FE DCIM` folder still needs four files (20,226,448 bytes):
`Camera/20260124_234412.jpg`, `20260124_234441.jpg`, `20260124_234452.jpg`,
and `20260124_234511.jpg` (all under `Camera/`). The REST file records show
no local copy and no connected source; their global versions originate from
the disconnected Galaxy Tab S10 FE. They need that source to reconnect.
The user asked to ignore these pending files for now on task qf3; replication
is not fully complete. No files or database records were deleted or reset.

Monitoring: Gogios check `Check Syncthing` on the frontends runs
`tools/check_syncthing` against https://syncthing.f3s.buetow.org every 30
minutes (paused while f3s is taken down). It pages (CRITICAL) on a stuck
folder, a transfer out of sync for over 1h, or a missing/rejected API key.
Its ignore list `/etc/check_syncthing/ignore` holds
`Galaxy Tab S10 FE DCIM; max-items=4; max-bytes=20226448` for this case,
so only a further backlog in that folder alerts. `Check Syncthing
Freshness` (same plugin, `-mode freshness`, every 30 minutes) warns when no
peer sent any data for 24h and pages after 24h more, and warns when
`earth.lan.buetow.org` or `Pixel 7 Pro Google` stays unseen for 72h; the
ignore list's `device: Galaxy Tab S10 FE` keeps the tablet out. Deployed by
`./gonf.sh cluster frontends frontends_gogios` (`gonf/frontends/syncthing.go`)
after `mage buildOpenBSD` in `tools/check_syncthing`; the API key comes from
`gonf/secrets/frontends/etc/check_syncthing/api.key` (untracked; copy of
`<apikey>` in the config PV's `config.xml`).

## Verified upgrade, 2026-11-08

Syncthing v2.1.7 (LinuxServer v2.1.7-ls236) is deployed. The running immutable
image config ID and authenticated REST version were verified. The same two
peers reconnected; all 12 folders settled idle. Device identity, folder/device
configuration, GUI credentials and API key are preserved, as are global/local
file and byte totals. The known tablet backlog remains four files / 20,226,448
bytes, with its existing four pull errors; it was not reset or hidden.

A quiesced config/index archive was downloaded and SHA-256 verified before
startup, under `~/.local/state/conf/j34-app-upgrades-20261107/`. Stop the writer
before restoring it with the previous image; preserve current synced data.
The automatic-sync hold for this upgrade ended once Argo was Synced/Healthy at a
published revision containing the chart (60f88c2). n44 set `enabled: true` in
Git; after the push its `finish-after-push.py` applied the manifest on
2026-11-09, so automated sync is live again.

## Direct path for the laptop (n44, 2026-11-09)

Earth (the laptop) used to reach f3s only through a public relay at about
25 KB/s; a 157 MB backlog took over 10 hours. At home, with wg2 up, it now
connects directly over the lab:

| Layer | Permit (only the laptop's wg2 identity 10.99.0.2, TCP only) |
|---|---|
| Service `syncthing-lan` | NodePort **32200** -> pod 22000, `externalTrafficPolicy: Local` |
| NetworkPolicy `syncthing-web` | second rule: `10.99.0.2/32` -> TCP 22000 |
| r-node `f3s_guard` | `ip saddr 10.99.0.2 ip daddr 10.20.0.0/24 tcp dport 32200` (`f3s/r-nodes/source-guard/migration.nft`) |
| triggerfish PF | `pass in on wg2 ... from 10.99.0.2 to { r0, r1, r2 } port 32200` (`gonf/triggerfish/profiles.go`) |

`Local` keeps the client's real source address: the receiving node DNATs to
the pod without SNAT, so the policy can name 10.99.0.2 instead of node/CNI
addresses. Node/CNI trust would also admit ordinary pods through the NodePort
(the co3 bypass). Only the node that runs the pod answers. The others drop
the dial, so earth lists all three nodes.

Earth's device entry for `f3s.lan.buetow.org` has the addresses
`tcp://10.20.0.120:32200`, `tcp://10.20.0.121:32200`,
`tcp://10.20.0.122:32200` and `dynamic`, and earth's options carry
`alwaysLocalNets = 10.20.0.0/24`, so the link counts as LAN (priority 10). Set
this through earth's GUI or REST config; it is not managed by gonf. `dynamic`
and the enabled relays stay: away from home, the static dials fail and the
relay carries the sync. f3s itself is unchanged and sees earth as
`10.99.0.2` (WAN). There is no public port-forward. 22000 and 32200 are closed
on the home line's public IPv4 and the router's IPv6 address (probed from
blowfish).

Verified 2026-11-09:

- Earth's log shows `connection.type=tcp-client connection.lan=true
  connection.prio=10` to `10.20.0.120:32200`, and f3s shows `tcp-server` from
  `10.99.0.2:22000`.
- A 32 MiB file in `Sync with earth` (shared by earth and f3s only) reached
  f3s in 11 s, scan included. The SHA-256 matched inside the pod. Its
  deletion propagated, and no versioning copy remains.
- Roaming was simulated with an earth-local nft drop of the three
  `:32200` destinations (config unchanged) and a forced reconnect. The relay
  came back, but only after about 7 minutes: the first relay Hello failed
  with EOF. Removing the drop let Syncthing dial the static address within
  35 s, and the relay connection was closed as `replaced`.
- Router trial `n44-syncthing-32200-20261109-a` was confirmed. The laptop
  is still refused on 32201 and 6443, and pi0 (a lab host the node guard
  trusts) times out on 32200 because the NetworkPolicy denies it.

The phones are not covered. Phone wg2 (10.99.0.3) has no permit, and on
Wi-Fi without wg2 there is no lab route, so they still use the relay. Giving
a phone the same path needs the same four layers for its own address.

Receipts: `~/.local/state/conf/n44-syncthing-direct-20261109/`.
