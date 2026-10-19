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
