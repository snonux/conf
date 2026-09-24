# Navidrome

Subsonic-compatible music server, `deluan/navidrome:0.61.2`, port 4533.
Namespace `services`, ArgoCD app `navidrome`, pinned to r1
(`nodeSelector kubernetes.io/hostname: r1.lan.buetow.org`, shared music PVC
with [beets-art](../../beets-art/helm-chart/README.md)).

| Ingress | Path |
|---|---|
| `https://navidrome.f3s.buetow.org` | OpenBSD relayd, WireGuard, Let's Encrypt |
| `https://navidrome.f3s.lan.buetow.org` | CARP VIP 192.168.1.138, self-signed LAN CA ([LAN access](../../docs/lan-access-setup-guide.md)) |

| Volume | Size | Mount |
|---|---|---|
| `/data/nfs/k3svolumes/navidrome/data` | 10Gi | `/data` (SQLite DB, cache) |
| `/data/nfs/k3svolumes/navidrome/music` | 200Gi | `/music` |

Env in `templates/deployment.yaml`: `ND_SCANSCHEDULE=1h`, `ND_LOGLEVEL=info`,
`ND_BASEURL=""`. Add music by copying into the music dir on the NFS server;
the hourly scan finds it. Users are managed in the web UI (first visit creates
the admin).

```sh
just status | logs [lines] | port-forward [4533] | sync | argocd-status | restart
```

Clients: play:Sub, substreamer (iOS); DSub, Ultrasonic, substreamer
(Android); Sublime Music, Sonixd.
