# Jellyfin

Media server, `https://jellyfin.f3s.buetow.org` (LAN:
`jellyfin.f3s.lan.buetow.org`). Namespace `services`, ArgoCD app
`jellyfin`, image `jellyfin/jellyfin:12.2`, selector `app=jellyfin-server`.

| PVC | hostPath | Mount |
|---|---|---|
| `jellyfin-config-pvc` | `/data/nfs/k3svolumes/jellyfin/config` | `/config` |
| `jellyfin-libraries-pvc` | `/data/nfs/k3svolumes/jellyfin/libraries` | `/media/libraries` |
| `jellyfin-data-pvc` | `/data/nfs/k3svolumes/jellyfin/data` | `/data` |

Owner 911:911. Each volume has an NFS sentinel check.

Networking: NodePorts 30096 (HTTP) and 30920. The OpenBSD relayd terminates
TLS on 443 and forwards `jellyfin.f3s.buetow.org` to table `f3s_jellyfin`
port 30096 directly, bypassing Traefik
(`gonf/frontends/assets/relayd.conf.tmpl`). relayd must send the full chain
(leaf + intermediate) or the Android app fails with "Unsupported version or
product". Jellyfin network settings: HTTPS off,
`PublicPort` 443, `KnownProxies` 10.0.0.0/8 and 192.168.0.0/16. Jellyfin
owns `network.xml` on the config PVC; don't mount it from a ConfigMap (the
migrations need to write it).

```sh
just status | logs [lines] | port-forward [8096] | sync | argocd-status | restart
curl https://jellyfin.f3s.buetow.org/System/Info/Public
echo | openssl s_client -servername jellyfin.f3s.buetow.org -connect jellyfin.f3s.buetow.org:443 | grep -c "BEGIN CERTIFICATE"   # expect 2
kubectl delete application jellyfin -n cicd    # uninstall, data stays
```

Upgrades from 10.8.x must go 10.8.13, 10.10.7, then 10.11.x; skipping
corrupts the DB (10.11 needs a DB from 10.9.11 or later).

Records: [`docs/archive/f3s/jellyfin/`](../../docs/archive/f3s/jellyfin/)
(deployment summary, yoga library quality survey).

## Upgrade to 12.2 (2026-10-17)

Jellyfin was stopped for the f0 ZFS snapshot
`zdata/enc/nfsdata@qf3-jellyfin-10.11.6-20261017`, then upgraded directly
from 10.11.6 to 12.2. Database migrations and startup completed, and both
public and LAN health checks passed. No third-party plugins were installed.

The required full library scan remains pending: both accounts saved in
foostore's `pass/infra/jellyfin.txt` were rejected at login. Run "Scan All
Libraries" after correcting the credentials. Authenticated playback has
not been verified in this upgrade run.
