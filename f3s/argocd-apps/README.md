# ArgoCD Applications

One ArgoCD `Application` manifest per workload, grouped by directory. All
Applications live in namespace `cicd` and pull from
`http://forgejo.services.svc.cluster.local/snonux/conf.git` at `master`,
path `f3s/<app>/helm-chart` unless noted. A `.yaml.disabled` file is kept but
not applied.

| Dir | Target namespace | Applications |
|---|---|---|
| `cicd/` | `cicd` | argo-rollouts |
| `infra/` | `infra` (cert-manager: `cert-manager`, traefik-config: `kube-system`) | cert-manager, pkgrepo, registry, traefik-config |
| `monitoring/` | `monitoring` | prometheus (multi-source), pushgateway; disabled: alloy, grafana-ingress, loki, tempo, trivy-operator |
| `services/` | `services` | anki-sync-server, bgtutor, beets-art, filebrowser, forgejo, goprecords, immich, ipv6test, keybr, kobo-sync-server, miniflux, motioneye, navidrome, player, protonbridge, radicale, shuriken, syncthing, wallabag, webdav, xplayer, ychat; disabled: apache, opodsync (retired; storage retained), pihole (runs in Docker on pi2/pi3), tracing-demo |

Sync policy everywhere: automated, `prune: true`, `selfHeal: true`,
`CreateNamespace=false`, retry 3x with backoff (immich: `prune: false`).
Manual cluster edits get reverted. Exception: the `network-policies/`
templates are `.yaml.disabled`; their eleven live Applications are manual
(no automated policy) and pin an exact published commit, see
[`../network-policies/README.md`](../network-policies/README.md).

A live rollout ahead of its published source holds the app with
`automated.enabled: false` (live and in the manifest, with a comment saying
why). Restore `prune: true, selfHeal: true` only once Argo is Synced at a
published revision that contains the change, i.e. Git renders exactly the
live state; otherwise selfHeal reverts the rollout. Task e54 restored the
twelve j34-era holds this way on 2026-11-09; none is held now.

`prometheus.yaml` combines the upstream kube-prometheus-stack chart with
`f3s/prometheus/manifests` and orders them with sync waves: 0 PV,
1 Secrets/ConfigMaps, 3 PrometheusRules, 4 dashboard ConfigMaps.

Disabling an app: rename it to `.yaml.disabled`, push, then
`kubectl delete application <name> -n cicd`; the
`resources-finalizer.argocd.argoproj.io` finalizer deletes its resources.
Retained hostPath PVs keep their NFS data.

## Operate

There is no app-of-apps: push to Forgejo, then apply the changed manifest.

```sh
kubectl apply -f f3s/argocd-apps/services/miniflux.yaml
kubectl apply -f f3s/argocd-apps/ -R        # everything (skips *.disabled)
argocd app list
argocd app get miniflux
argocd app sync miniflux
```

Most app directories also have `just sync` (sets the ArgoCD refresh
annotation) and `just argocd-status`.

## Rebuild from scratch

1. Bootstrap k3s.
2. `kubectl create namespace cicd monitoring services infra`
3. Install ArgoCD ([`../argocd/`](../argocd/README.md)).
4. Forgejo must serve `snonux/conf` before the rest can sync.
5. `kubectl apply -f f3s/argocd-apps/ -R`
