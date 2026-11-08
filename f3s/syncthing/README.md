# Syncthing

Web UI `https://syncthing.f3s.buetow.org`, sync port 22000. Namespace
`services`, ArgoCD app `syncthing`.

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
Automatic sync is held in the live and local Application until this matching
chart is published. Do not sync from the old remote source in the meantime.
