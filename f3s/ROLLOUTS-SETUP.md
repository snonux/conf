# Argo Rollouts Setup and Demo Guide

Complete setup and demonstration of Argo Rollouts with the tracing-demo application. Canary strategy: 33% traffic (1 pod) for 1 minute, then auto-promote to 100%.

## Quick Setup

### 1. Install Argo Rollouts Controller

```bash
cd /home/paul/git/conf/f3s/argo-rollouts
just install
```

Verify installation:
```bash
kubectl get pods -n cicd -l app.kubernetes.io/name=argo-rollouts
kubectl get crd | grep rollout
```

### 2. Install kubectl Plugin (Optional but Recommended)

```bash
curl -LO https://github.com/argoproj/argo-rollouts/releases/latest/download/kubectl-argo-rollouts-linux-amd64
chmod +x kubectl-argo-rollouts-linux-amd64
sudo install -m 755 kubectl-argo-rollouts-linux-amd64 /usr/local/bin/kubectl-argo-rollouts
```

Verify:
```bash
kubectl argo rollouts version
```

### 3. Sync ArgoCD with New Applications

```bash
argocd app sync argo-rollouts
argocd app sync tracing-demo
```

### 4. Verify Rollout is Deployed

```bash
kubectl get rollout tracing-demo-frontend -n services
kubectl describe rollout tracing-demo-frontend -n services
```

Expected status: `Healthy` with `3/3 replicas` in stable state.

## Quick Demo (90 seconds)

### Terminal 1 - Watch Progress

```bash
cd /home/paul/git/conf/f3s/tracing-demo
just rollout-watch
```

Or use the kubectl command directly:
```bash
kubectl argo rollouts get rollout tracing-demo-frontend -n services --watch
```

### Terminal 2 - Trigger Rollout

Wait 10 seconds for Terminal 1 to start watching, then trigger:

```bash
kubectl patch rollout tracing-demo-frontend -n services \
  --type='json' \
  -p='[{"op":"add","path":"/spec/template/spec/containers/0/env/-","value":{"name":"ROLLOUT_V","value":"'$(date +%s)'"}}]'
```

### Watch the Timeline

**Terminal 1 will show:**

```
Step: 0/3
SetWeight: 33
Canary: 1 pod (new version) - starting
Stable: 3 pods (old version) - handling requests
```

→ After 15 seconds, canary pod becomes ready:

```
Step: 1/3
SetWeight: 33
Canary: 1 pod (new version) - ready, receiving 33% traffic
Stable: 3 pods (old version) - receiving 67% traffic
```

→ After ~60 seconds, auto-promotion begins:

```
Step: 2/3
SetWeight: 100
Canary scaling → Stable
```

→ After ~90 seconds, complete:

```
Status: Healthy
Replicas: 3/3 all running new version
```

## Demo Scenarios

### Scenario 1: Observe the Full Rollout

Just follow the "Quick Demo" above. Watch all three steps progress automatically over 90 seconds.

### Scenario 2: Abort Rollout (Simulate Failure)

**Terminal 1**: Watch the rollout
```bash
just rollout-watch
```

**Terminal 2**: Trigger rollout
```bash
kubectl patch rollout tracing-demo-frontend -n services \
  --type='json' \
  -p='[{"op":"add","path":"/spec/template/spec/containers/0/env/-","value":{"name":"ROLLOUT_V","value":"'$(date +%s)'"}}]'
```

**Terminal 3 (while at step 1)**: Abort the rollout
```bash
cd /home/paul/git/conf/f3s/tracing-demo
just rollout-abort
```

Result:
- Canary pods terminate
- Old 3 pods continue running
- Status shows "Aborted"

Verify:
```bash
just rollout-status
```

### Scenario 3: Load Testing During Rollout

**Terminal 1**: Watch rollout
```bash
just rollout-watch
```

**Terminal 2**: Start load test
```bash
just load-test &
```

**Terminal 3**: Trigger rollout
```bash
kubectl patch rollout tracing-demo-frontend -n services \
  --type='json' \
  -p='[{"op":"add","path":"/spec/template/spec/containers/0/env/-","value":{"name":"ROLLOUT_V","value":"'$(date +%s)'"}}]'
```

Load test will hit both old and new pods during the 1-minute canary window.

### Scenario 4: Check Logs During Rollout

**Terminal 1**: Watch rollout
```bash
just rollout-watch
```

**Terminal 2**: Trigger rollout
```bash
kubectl patch rollout tracing-demo-frontend -n services \
  --type='json' \
  -p='[{"op":"add","path":"/spec/template/spec/containers/0/env/-","value":{"name":"ROLLOUT_V","value":"'$(date +%s)'"}}]'
```

**Terminal 3**: Watch logs
```bash
kubectl logs -n services -l app=tracing-demo-frontend -f --tail=20
```

See logs from both old and new pods.

### Scenario 5: Monitor via Grafana Tempo (Distributed Tracing)

**Terminal 1**: Watch rollout
```bash
just rollout-watch
```

**Terminal 2**: Trigger rollout
```bash
kubectl patch rollout tracing-demo-frontend -n services \
  --type='json' \
  -p='[{"op":"add","path":"/spec/template/spec/containers/0/env/-","value":{"name":"ROLLOUT_V","value":"'$(date +%s)'"}}]'
```

**Terminal 3**: Open Grafana
1. Navigate to https://grafana.f3s.buetow.org
2. Go to Explore → Select "Tempo" datasource
3. Query: `{ resource.service.name = "frontend" }`
4. See traces from both old and new versions during canary phase

## Timeline Breakdown

| Time | Event | Status |
|------|-------|--------|
| 0s | Trigger rollout | Rollout starts |
| 0-5s | Canary pod created | `Step 0/3: SetWeight 33` |
| 5-15s | Canary pod becoming ready | Still not ready |
| 15s | Canary pod ready | `Step 1/3: SetWeight 33, canary ready` |
| 15-60s | Observing canary | Requests split 67/33 (old/new) |
| 60s | Auto-promotion triggered | `Step 2/3: SetWeight 100` |
| 60-70s | Scaling new pods | Canary → Stable |
| 70-80s | Terminating old pods | Old pods scaling down |
| ~90s | Complete | `Status: Healthy, 3/3 replicas` |

## Monitoring During Rollout

### kubectl Commands

Real-time status:
```bash
kubectl argo rollouts get rollout tracing-demo-frontend -n services --watch
```

Check specific details:
```bash
kubectl argo rollouts describe rollout tracing-demo-frontend -n services
kubectl argo rollouts history tracing-demo-frontend -n services
```

Pod status:
```bash
kubectl get pods -n services -l app=tracing-demo-frontend -o wide
```

### Prometheus Metrics

```bash
# Port-forward Prometheus
kubectl port-forward -n monitoring svc/prometheus 9090:9090
```

Then query:
```promql
# Pod counts during rollout
kube_replicaset_replicas{replicaset=~"tracing-demo-frontend.*"}

# Pod status
kube_pod_status_phase{namespace="services", pod=~"tracing-demo-frontend.*"}

# Pod age (shows which are old vs new)
time() - kube_pod_created{namespace="services", pod=~"tracing-demo-frontend.*"}
```

### Grafana Dashboards

1. Open Grafana: https://grafana.f3s.buetow.org
2. Explore → Tempo datasource
3. Query: `{ resource.service.name = "frontend" }`
4. See traces from old and new versions
5. Notice latency/error differences during rollout

## Rollout Configuration

Located in: `/home/paul/git/conf/f3s/tracing-demo/helm-chart/templates/frontend-rollout.yaml`

Key settings:
```yaml
replicas: 3  # 3 pods total
strategy:
  canary:
    steps:
    - setWeight: 33       # Send 1 pod (33%) to canary
    - pause:
        duration: 1m      # Wait 1 minute, then auto-promote
    - setWeight: 100      # Promote all to new version
```

To modify pause duration:
```bash
# Edit the file
nano /home/paul/git/conf/f3s/tracing-demo/helm-chart/templates/frontend-rollout.yaml

# Change duration: 1m to duration: 5m (for example)
# Then commit and push
git add -A && git commit -m "chore: extend canary pause to 5 minutes"
git push r0 master
```

ArgoCD will auto-sync the new rollout configuration.

## Troubleshooting

### Rollout shows "ErrImagePull" on canary pod

This happens if using an image tag that doesn't exist. The env var patch approach forces a rollout without changing the image, so use:

```bash
kubectl patch rollout tracing-demo-frontend -n services \
  --type='json' \
  -p='[{"op":"add","path":"/spec/template/spec/containers/0/env/-","value":{"name":"ROLLOUT_V","value":"'$(date +%s)'"}}]'
```

### Rollout stuck in "Progressing"

Check pod status:
```bash
kubectl describe rollout tracing-demo-frontend -n services
kubectl get pods -n services -l app=tracing-demo-frontend
```

Check controller logs:
```bash
kubectl logs -n cicd -l app.kubernetes.io/name=argo-rollouts --tail=50
```

### Controller not running

```bash
kubectl get pods -n cicd -l app.kubernetes.io/name=argo-rollouts
kubectl logs -n cicd -l app.kubernetes.io/name=argo-rollouts
```

### Auto-promotion not happening

Verify pause duration is set:
```bash
kubectl get rollout tracing-demo-frontend -n services -o yaml | grep -A 5 "pause:"
```

## Advanced: Modify Canary Parameters

### Increase observation time to 5 minutes

```bash
# Edit rollout YAML
nano /home/paul/git/conf/f3s/tracing-demo/helm-chart/templates/frontend-rollout.yaml

# Change:
#   - pause:
#       duration: 1m
# To:
#   - pause:
#       duration: 5m

git add -A && git commit -m "chore: extend canary pause to 5 minutes"
git push r0 master
```

### Reduce traffic weight to canary (more conservative)

```yaml
steps:
- setWeight: 10       # Only 10% traffic (0.3 pods worth)
- pause:
    duration: 2m      # Observe longer
- setWeight: 100
```

### Add health check analysis (requires Flagger or ArgoCD Analysis)

For automated rollback based on error rate thresholds, see `/home/paul/git/conf/f3s/ROLLOUTS-SETUP.md` → "Advanced: Custom Analysis" section.

## References

- [Argo Rollouts Canary Strategy](https://argoproj.github.io/argo-rollouts/features/canary/)
- [Argo Rollouts Best Practices](https://argoproj.github.io/argo-rollouts/best-practices/)
- [kubectl-argo-rollouts Plugin](https://argoproj.github.io/argo-rollouts/getting-started/#using-kubectl-with-argo-rollouts)
- [Flagger for Automated Analysis](https://flagger.app/)
