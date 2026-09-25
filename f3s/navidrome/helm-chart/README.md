# Navidrome

Subsonic-compatible music server, `deluan/navidrome:0.61.2`, port 4533.
Namespace `services`, ArgoCD app `navidrome`, runs on any r-node (both
volumes are NFS hostPath; shared music PVC with
[beets-art](../../beets-art/helm-chart/README.md)).

| Ingress | Path |
|---|---|
| `https://navidrome.f3s.buetow.org` | OpenBSD relayd, WireGuard, Let's Encrypt |
| `https://navidrome.f3s.lan.buetow.org` | CARP VIP 192.168.1.138, self-signed LAN CA ([LAN access](../../docs/lan-access-setup-guide.md)) |

| Volume | Size | Mount |
|---|---|---|
| `/data/nfs/k3svolumes/navidrome/data` (`navidrome-data-nfs-pvc`) | 10Gi | `/data` (SQLite DB) |
| `/data/nfs/k3svolumes/navidrome/music` (`navidrome-music-pvc`, RWX) | 200Gi | `/music` |
| node-local `emptyDir` | 1Gi | `/cache` (`ND_CACHEFOLDER`, image/transcode cache, disposable) |

Until 2026-09-25 `/data` was a local-path PVC on r1 with a `nodeSelector`;
it moved back to NFS so r1 is no longer a single point of failure, with the
cache kept node-local (the reason it had left NFS: ~19 s image-cache init).

Env in `templates/deployment.yaml`: `ND_SCANSCHEDULE=1h`, `ND_LOGLEVEL=info`,
`ND_BASEURL=""`, `ND_CACHEFOLDER=/cache`. Add music by copying into the music dir on the NFS server;
the hourly scan finds it. Users are managed in the web UI (first visit creates
the admin).

```sh
just status | logs [lines] | port-forward [4533] | sync | argocd-status | restart
```

Clients: play:Sub, substreamer (iOS); DSub, Ultrasonic, substreamer
(Android); Sublime Music, Sonixd.
