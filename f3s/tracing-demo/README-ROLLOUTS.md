# Argo Rollouts - Quick Reference

Progressive delivery (canary deployments) for the f3s cluster.

## TL;DR - Get Started in 5 Minutes

```bash
# 1. Install controller
cd /home/paul/git/conf/f3s/argo-rollouts
just install

# 2. Wait for ArgoCD sync (or force)
argocd app sync argo-rollouts
argocd app sync tracing-demo

# 3. Verify setup
cd /home/paul/git/conf/f3s/tracing-demo
just rollout-status

# 4. Run a demo (Terminal 1)
just rollout-watch

# 5. Trigger in another terminal (Terminal 2)
kubectl patch rollout tracing-demo-frontend -n services \
  --type='json' \
  -p='[{"op":"add","path":"/spec/template/spec/containers/0/env/-","value":{"name":"ROLLOUT_V","value":"'$(date +%s)'"}}]'

# 6. Watch progress in Terminal 1 (~90 seconds total)
```

Expected flow:
- 0-15 sec: **33% traffic** to canary (1 new pod, 3 old pods)
- 15-60 sec: **Monitor** (paused, observing canary health)
- 60+ sec: **Auto-promote to 100%** (scales all 3 pods to new version)
- ~90 sec: **Complete** (all 3 pods running new version)

## Files Created

### Setup & Installation
- `argo-rollouts/Justfile` - Install/manage controller
- `argo-rollouts/values.yaml` - Helm config
- `argocd-apps/cicd/argo-rollouts.yaml` - ArgoCD app

### Demo App Configuration
- `tracing-demo/helm-chart/templates/frontend-rollout.yaml` - Canary definition
- `tracing-demo/Justfile` - New `just rollout-*` commands
- `tracing-demo/rollout-demo.sh` - Demo automation script

### Documentation
- `ARGO-ROLLOUTS-SUMMARY.md` - **START HERE** - Full overview
- `ROLLOUTS-SETUP.md` - **DETAILED GUIDE** - 5 demo scenarios
- `ROLLOUTS-CHECKLIST.md` - **DEPLOYMENT CHECKLIST** - Step-by-step
- `tracing-demo/ROLLOUTS-DEMO.md` - Technical walkthrough
- `README-ROLLOUTS.md` - This file

## Why Canary Deployments?

**Old way (Deployment)**:
- 2 old pods → removed
- 2 new pods → created
- ~5 seconds of potential traffic loss
- No way to validate before 100% rollout

**New way (Rollout with Canary)**:
- 3 old pods → 3 old + 1 new (33% traffic to canary)
- Observe for 1 minute
- If healthy → automatically promote all 3 pods to new version
- If unhealthy → abort, revert to 3 old pods
- Zero downtime, validated before full rollout

## Common Commands

```bash
cd /home/paul/git/conf/f3s/tracing-demo

# Watch rollout progress (real-time)
just rollout-watch

# Check current status
just rollout-status

# Detailed info
just rollout-info

# Abort and rollback (prevents auto-promotion)
just rollout-abort

# View history
just rollout-history

# Generate load during rollout
just load-test
```

## What Happens During Canary

### Step 1: 33% Traffic (0-15 seconds)
```
Frontend Service
├── Stable ReplicaSet (old version): 3 pods → receives 67% traffic
└── Canary ReplicaSet (new version): 1 pod → receives 33% traffic
```

Monitor during this phase:
- Error rates
- Response latency
- Logs and traces
- Prometheus metrics

### Step 2: Pause (15-60 seconds)
```
Service pauses traffic shift, monitoring canary health:
- Auto-promotion after 1 minute if healthy
- Or abort: kubectl argo rollouts abort ... to stop
```

### Step 3: 100% Traffic (60+ seconds)
```
Frontend Service
├── Stable ReplicaSet (new version): 3 pods → receives 100% traffic
└── Canary ReplicaSet (old version): 0 pods → terminated
```

## Architecture

```
Git Commit (new image)
    ↓
Git Server (conf.git)
    ↓
ArgoCD detects change
    ↓
Updates Rollout resource
    ↓
Argo Rollouts Controller
    ↓
    ├─→ Scales Canary ReplicaSet (1 new pod)
    ├─→ Frontend Service routes 33/67 traffic
    ├─→ Monitors health/metrics for 1 minute
    └─→ Auto-promotes if healthy
        ├─→ If healthy: Scale to 3 new, remove old
        └─→ If abort: Remove canary, keep 3 old
```

## Demo Scenarios

See `ROLLOUTS-SETUP.md` for complete walkthrough of:

1. **Basic Canary** - Watch 50% → 100% progression
2. **Manual Promotion** - Skip waiting with `just rollout-promote`
3. **Abort/Rollback** - Fail canary and revert
4. **Prometheus Monitoring** - Track metrics during rollout
5. **GitOps Flow** - Commit code, watch auto-rollout

## Monitoring

### Command-line
```bash
# Real-time watch
kubectl argo rollouts get rollout tracing-demo-frontend -n services --watch

# Check metrics
kubectl top pods -n services -l app=tracing-demo-frontend
```

### Grafana
https://grafana.f3s.buetow.org

1. Explore → Tempo
2. Query: `{ resource.service.name = "frontend" }`
3. See traces from old and new versions

### Prometheus
```bash
# Port-forward
kubectl port-forward -n monitoring svc/prometheus 9090:9090
# Open http://localhost:9090

# Query pod status
kube_pod_status_phase{namespace="services", pod=~".*frontend.*"}
```

## Troubleshooting

**Controller not running?**
```bash
kubectl get pods -n cicd -l app.kubernetes.io/name=argo-rollouts
kubectl logs -n cicd -l app.kubernetes.io/name=argo-rollouts
```

**Rollout stuck?**
```bash
kubectl describe rollout tracing-demo-frontend -n services
kubectl get pods -n services -l app=tracing-demo-frontend
```

**Need plugin?**
```bash
curl -LO https://github.com/argoproj/argo-rollouts/releases/latest/download/kubectl-argo-rollouts-linux-amd64
sudo install -m 755 kubectl-argo-rollouts-linux-amd64 /usr/local/bin/kubectl-argo-rollouts
```

## Next Steps

1. Complete setup using `ROLLOUTS-CHECKLIST.md`
2. Run demo scenarios from `ROLLOUTS-SETUP.md`
3. Share with team
4. Optional: Add Istio for advanced traffic routing
5. Optional: Deploy Flagger for automated analysis
6. Migrate other services to Rollout

## Key Resources

| File | Purpose |
|------|---------|
| `ARGO-ROLLOUTS-SUMMARY.md` | Architecture & what was created |
| `ROLLOUTS-SETUP.md` | Complete setup & 5 demo scenarios |
| `ROLLOUTS-CHECKLIST.md` | Step-by-step deployment |
| `tracing-demo/ROLLOUTS-DEMO.md` | Technical details & troubleshooting |
| `argo-rollouts/README.md` | Controller installation guide |

## Support

- Argo Rollouts Docs: https://argoproj.github.io/argo-rollouts/
- Canary Strategy: https://argoproj.github.io/argo-rollouts/features/canary/
- Kubectl Plugin: https://argoproj.github.io/argo-rollouts/getting-started/#using-kubectl-with-argo-rollouts
