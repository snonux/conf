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
| `monitoring/` | `monitoring` | alloy (upstream chart), prometheus (multi-source), pushgateway; disabled: grafana-ingress, loki, tempo, trivy-operator |
| `services/` | `services` | anki-sync-server, audiobookshelf, beets-art, filebrowser, forgejo, goprecords, immich, ipv6test, jellyfin, keybr, kobo-sync-server, miniflux, navidrome, opodsync, pihole, player, protonbridge, radicale, shuriken, syncthing, wallabag, webdav, xplayer, ychat; disabled: apache, tracing-demo |

Sync policy everywhere: automated, `prune: true`, `selfHeal: true`,
`CreateNamespace=false`, retry 3x with backoff. Manual cluster edits get
reverted.

`prometheus.yaml` combines the upstream kube-prometheus-stack chart with
`f3s/prometheus/manifests` and orders them with sync waves: 0 PVs/RBAC,
1 Secrets/ConfigMaps, 3 PrometheusRules, 4 dashboard ConfigMaps, 10 PostSync
Grafana restart.

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
