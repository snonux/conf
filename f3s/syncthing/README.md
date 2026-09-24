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
