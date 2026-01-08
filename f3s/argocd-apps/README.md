# ArgoCD Applications

This directory contains ArgoCD Application manifests that define all workloads deployed to the f3s cluster.

## Directory Structure

Applications are organized by Kubernetes namespace:

```
argocd-apps/
├── monitoring/          # Observability stack (namespace: monitoring)
│   ├── alloy.yaml       # Log collection (DaemonSet)
│   ├── grafana-ingress.yaml  # Grafana external access
│   ├── loki.yaml        # Log aggregation
│   ├── prometheus.yaml  # Metrics collection and monitoring (kube-prometheus-stack)
│   ├── pushgateway.yaml # Prometheus Pushgateway for metrics ingestion
│   └── tempo.yaml       # Distributed tracing
├── services/            # User-facing applications (namespace: services)
│   ├── anki-sync-server.yaml    # Anki flashcard synchronization
│   ├── audiobookshelf.yaml      # Audiobook/podcast streaming
│   ├── filebrowser.yaml         # Web-based file browser
│   ├── freshrss.yaml            # RSS feed reader
│   ├── immich.yaml              # Photo management
│   ├── keybr.yaml               # Typing practice
│   ├── kobo-sync-server.yaml    # KOReader sync server
│   ├── miniflux.yaml            # Minimalist RSS reader
│   ├── opodsync.yaml            # Podcast synchronization
│   ├── radicale.yaml            # CalDAV/CardDAV server
│   ├── syncthing.yaml           # File synchronization
│   ├── tracing-demo.yaml        # Distributed tracing demo app
│   ├── wallabag.yaml            # Read-it-later service
│   └── webdav.yaml              # WebDAV server
├── infra/               # Infrastructure services (namespace: infra)
│   └── registry.yaml    # Private Docker registry
└── test/                # Test/example applications (namespace: test)
    └── example-apache-volume-claim.yaml  # Example Apache with PVC

```

## Application Count by Namespace

- **monitoring**: 6 applications
- **services**: 13 applications
- **infra**: 1 application
- **test**: 1 application

**Total**: 21 applications

## Usage

### Apply all applications

```bash
# Apply all applications at once
kubectl apply -f argocd-apps/ -R

# Or apply by namespace
kubectl apply -f argocd-apps/monitoring/
kubectl apply -f argocd-apps/services/
kubectl apply -f argocd-apps/infra/
kubectl apply -f argocd-apps/test/
```

### View application status

```bash
# List all applications
argocd app list

# View specific application
argocd app get miniflux

# View by namespace (using labels)
argocd app list -l "namespace=monitoring"
```

### Sync all applications

```bash
# Sync all applications
argocd app sync -l "argocd.argoproj.io/instance"

# Sync specific namespace
argocd app sync -l "namespace=monitoring"
```

## Application Manifest Structure

Each Application manifest follows this pattern:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: <app-name>
  namespace: cicd  # ArgoCD runs in the cicd namespace
  finalizers:
    - resources-finalizer.argocd.argoproj.io
spec:
  project: default
  source:
    repoURL: https://codeberg.org/snonux/conf.git
    targetRevision: master
    path: f3s/<app-name>/helm-chart
  destination:
    server: https://kubernetes.default.svc
    namespace: <target-namespace>
  syncPolicy:
    automated:
      prune: true      # Delete resources removed from Git
      selfHeal: true   # Automatically revert manual changes
    syncOptions:
      - CreateNamespace=false
    retry:
      limit: 3
      backoff:
        duration: 5s
        factor: 2
        maxDuration: 1m
```

## Sync Policies

All applications use automated sync with:

- **prune: true** - Resources removed from Git are deleted from the cluster
- **selfHeal: true** - Manual changes in the cluster are automatically reverted to match Git

All applications use `prune: true`.

## Complex Applications

Some applications use advanced ArgoCD features:

### Multi-Source Applications

**prometheus.yaml** combines multiple sources:
- Upstream Helm chart from prometheus-community
- Custom manifests from Git (recording rules, dashboards, hooks)

### Sync Waves and Hooks

**prometheus.yaml** uses sync waves to control deployment order:
- Wave 0: PersistentVolumes, RBAC
- Wave 1: Secrets, ConfigMaps
- Wave 3: PrometheusRule CRDs (recording rules)
- Wave 4: Dashboard ConfigMaps
- Wave 10: PostSync hook (Grafana restart)

## Disaster Recovery

To rebuild the entire cluster from scratch:

1. Bootstrap k3s cluster
2. Create namespaces:
```bash
kubectl create namespace cicd
kubectl create namespace monitoring
kubectl create namespace services
kubectl create namespace infra
kubectl create namespace test
```

3. Install ArgoCD (see `/home/paul/git/conf/f3s/argocd/`)

4. Apply all Application manifests:
```bash
kubectl apply -f argocd-apps/ -R
```

5. ArgoCD automatically deploys all 21 applications

Total recovery time: ~30 minutes.

## See Also

- [ArgoCD Documentation](https://argo-cd.readthedocs.io)
- [f3s Configuration Repository](https://codeberg.org/snonux/conf/src/branch/master/f3s)
- Blog post: f3s: Kubernetes with FreeBSD - Part X: GitOps with ArgoCD
