# Argo Rollouts Demo - Technical Details

Detailed technical walkthrough of Argo Rollouts canary strategy for tracing-demo frontend.

## Quick Demo (90 seconds)

### Setup

```bash
# Terminal 1: Watch the rollout
cd /home/paul/git/conf/f3s/tracing-demo
just rollout-watch

# Terminal 2: Trigger the rollout (after Terminal 1 is watching)
kubectl patch rollout tracing-demo-frontend -n services \
  --type='json' \
  -p='[{"op":"add","path":"/spec/template/spec/containers/0/env/-","value":{"name":"ROLLOUT_V","value":"'$(date +%s)'"}}]'
```

### Execution Timeline

**t=0-15s: Canary Launch**
```bash
# Terminal 1 shows:
Name:            tracing-demo-frontend
Status:          ◌ Progressing
Strategy:        Canary
  Step:          0/3
  SetWeight:     33
  ActualWeight:  0
Images:          (new) tracing-demo-frontend (canary)
                 (old) tracing-demo-frontend (stable)
Replicas:
  Desired:       3
  Current:       4  # 3 stable + 1 canary being created
  Updated:       1
  Ready:         3  # 3 stable pods ready, canary still starting
```

**t=15-60s: Canary Observation**
```bash
# After canary pod becomes ready (~15 seconds)
Status:          ◌ Progressing
  Step:          1/3    # Now in pause step
  SetWeight:     33
  ActualWeight:  33     # Actual weight achieved
Replicas:
  Desired:       3
  Current:       4
  Updated:       1
  Ready:         4      # All 4 pods (3 stable + 1 canary) ready
  Available:     4
```

Service routes traffic:
- **Old version**: 3 pods → ~67% traffic
- **New version**: 1 pod → ~33% traffic

**t=60s: Auto-Promotion**
```bash
# After 1 minute pause duration
Status:          ◌ Progressing
  Step:          2/3    # Now promoting
  SetWeight:     100
Replicas:
  Desired:       3
  Current:       4
  Updated:       3      # All 3 new pods created
  Ready:         3      # 3 new pods ready
  Available:     3
```

Old pods terminate, new pods scale up.

**t=90s: Complete**
```bash
Status:          ✔ Healthy
  Step:          3/3    # Complete
  SetWeight:     100
  ActualWeight:  100
Replicas:
  Desired:       3
  Current:       3
  Updated:       3
  Ready:         3
  Available:     3
Images:          tracing-demo-frontend (stable, new version)
```

## Configuration Details

Location: `/home/paul/git/conf/f3s/tracing-demo/helm-chart/templates/frontend-rollout.yaml`

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  name: tracing-demo-frontend
  namespace: services
spec:
  replicas: 3          # Total desired pods
  strategy:
    canary:
      steps:
      # Step 1: Send 33% traffic to new version
      - setWeight: 33
      # Step 2: Wait 1 minute, then auto-promote
      - pause:
          duration: 1m
      # Step 3: Promote to 100% traffic
      - setWeight: 100
  
  selector:
    matchLabels:
      app: tracing-demo-frontend
  template:
    # Same pod spec as Deployment
    metadata:
      labels:
        app: tracing-demo-frontend
    spec:
      containers:
      - name: frontend
        image: registry.lan.buetow.org:30001/tracing-demo-frontend:latest
        # ... rest of container spec
```

## ReplicaSet Behavior

### During Canary (Step 1)

**Stable ReplicaSet (revision 1)**
- Desired: 3
- Current: 3
- Ready: 3
- Label: `app=tracing-demo-frontend`

**Canary ReplicaSet (revision 2)**
- Desired: 1
- Current: 1
- Ready: 1 (after 10-15 seconds)
- Label: `app=tracing-demo-frontend`

**Service Routing**
```yaml
selector:
  app: tracing-demo-frontend  # Selects ALL replicas (both RS)
```

Traffic split happens at pod replica level:
- 3 stable pods serve ~67% of requests
- 1 canary pod serves ~33% of requests

### After Promotion (Step 3)

**Old ReplicaSet (revision 1)**
- Desired: 0
- Current: 0
- Terminating...

**New ReplicaSet (revision 2)**
- Desired: 3
- Current: 3
- Ready: 3
- Label: `app=tracing-demo-frontend`

Service now routes 100% to new version (3 pods).

## Monitoring Canary

### kubectl Commands

Real-time progress:
```bash
kubectl argo rollouts get rollout tracing-demo-frontend -n services --watch
```

Detailed status:
```bash
kubectl argo rollouts describe rollout tracing-demo-frontend -n services
```

History:
```bash
kubectl argo rollouts history tracing-demo-frontend -n services
```

### Pod Status

Watch pods during rollout:
```bash
kubectl get pods -n services -l app=tracing-demo-frontend -w
```

See which revision:
```bash
kubectl get pods -n services -l app=tracing-demo-frontend -o wide \
  -o custom-columns=NAME:.metadata.name,READY:.status.ready,REVISION:.metadata.labels.controller-revision-hash
```

### Logs

All pods (old and new):
```bash
kubectl logs -n services -l app=tracing-demo-frontend -f
```

Just canary pod (during step 1):
```bash
# Find the newest pod
CANARY=$(kubectl get pods -n services -l app=tracing-demo-frontend -o jsonpath='{.items[-1].metadata.name}')
kubectl logs -n services $CANARY -f
```

Just stable pods (old version):
```bash
kubectl logs -n services -l app=tracing-demo-frontend,controller-revision-hash=<old-hash> -f
```

### Events

Check rollout events:
```bash
kubectl describe rollout tracing-demo-frontend -n services | grep -A 20 Events:
```

Check pod events:
```bash
kubectl describe pod -n services -l app=tracing-demo-frontend
```

## Health Checks During Canary

Container has liveness and readiness probes:

```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 5000
  initialDelaySeconds: 10
  periodSeconds: 10

readinessProbe:
  httpGet:
    path: /health
    port: 5000
  initialDelaySeconds: 5
  periodSeconds: 5
```

Argo Rollouts waits for readinessProbe to succeed before considering a pod "Ready". Only when canary pod is Ready does setWeight 33 take effect.

If readiness fails, canary pod stays in `0/1 Ready` state indefinitely (until timeout or manual abort).

## Traffic Flow

### Without Service Mesh (Current)

Kubernetes Service-based load balancing (round-robin):

1. Client sends request
2. `kubectl get endpoints frontend-service` returns:
   ```
   NAME               ENDPOINTS
   frontend-service   10.42.1.100,10.42.1.101,10.42.1.102,10.42.2.1
   ```
3. Service load-balancer picks a pod randomly
   - ~67% hit old pods (3 out of 4)
   - ~33% hit new pod (1 out of 4)

### With Service Mesh (Istio/Linkerd)

Would enable advanced routing:
- Precise percentage-based splits (50.5%, 49.5%)
- Header-based routing (route by user ID, etc.)
- Gradual step weights (5% → 10% → 25% → 50% → 100%)
- Automatic rollback on error rate thresholds

## Abort/Rollback

### Abort Current Rollout

```bash
kubectl argo rollouts abort tracing-demo-frontend -n services
```

Effect:
- Canary ReplicaSet scales to 0
- Old Stable ReplicaSet remains at 3 pods
- Status: `Degraded` with message "RolloutAborted"
- Next rollout will use the next revision

### Manual Rollback to Previous Revision

```bash
kubectl argo rollouts undo tracing-demo-frontend -n services
```

Or rollback to specific revision:
```bash
kubectl argo rollouts undo tracing-demo-frontend -n services --to-revision=3
```

## Modifying Rollout Configuration

### Change Pause Duration

```bash
kubectl patch rollout tracing-demo-frontend -n services \
  --type='json' \
  -p='[{"op":"replace","path":"/spec/strategy/canary/steps/1/pause/duration","value":"5m"}]'
```

### Change Weight

```bash
kubectl patch rollout tracing-demo-frontend -n services \
  --type='json' \
  -p='[{"op":"replace","path":"/spec/strategy/canary/steps/0/setWeight","value":50}]'
```

### Change Replicas

```bash
kubectl patch rollout tracing-demo-frontend -n services \
  -p='{"spec":{"replicas":5}}'
```

All changes take effect on next rollout trigger.

## Troubleshooting

### Canary Pod Won't Start

Check pod events:
```bash
kubectl describe pod -n services -l app=tracing-demo-frontend | tail -20
```

Common issues:
- **ImagePullBackoff**: Image doesn't exist (use env var patch instead)
- **Pending**: No resources available (check node capacity)
- **CrashLoopBackOff**: Application error (check logs)

### Readiness Probe Failing

Canary pod stays in `0/1 Ready`:
```bash
kubectl get pods -n services -l app=tracing-demo-frontend
# Shows: ... 0/1 Running ... (waiting for readiness)
```

Check probe:
```bash
curl http://CANARY_POD_IP:5000/health
```

Should return 200 OK.

### Rollout Stuck in Progressing

Check status message:
```bash
kubectl argo rollouts status tracing-demo-frontend -n services
# Output: "Progressing - more replicas need to be updated"
```

Issue: Canary pod not becoming ready within timeout. Abort and retry:
```bash
kubectl argo rollouts abort tracing-demo-frontend -n services
```

### Auto-Promotion Not Happening

Check if pause duration expired:
```bash
kubectl argo rollouts get rollout tracing-demo-frontend -n services

# Look for: Step: 1/3 with pause duration elapsed
```

If stuck at step 1, manually promote:
```bash
kubectl argo rollouts promote tracing-demo-frontend -n services
```

Or abort and retry:
```bash
kubectl argo rollouts abort tracing-demo-frontend -n services
```

## Advanced Topics

### Pre/Post-Promotion Hooks

Trigger scripts before/after promotion. Example:
```yaml
strategy:
  canary:
    steps:
    - setWeight: 33
    - pause:
        duration: 1m
        termination: RolloutAbortOnFailure  # Built-in hook support
    - setWeight: 100
```

### Analysis and Rollback

Integrate with external metrics (Prometheus, Datadog) to auto-rollback if thresholds violated. Requires Flagger or custom AnalysisTemplate.

### GitOps Workflow

Changes to rollout config in git auto-sync via ArgoCD:

1. Edit `/home/paul/git/conf/f3s/tracing-demo/helm-chart/templates/frontend-rollout.yaml`
2. Commit: `git commit -am "chore: adjust canary duration"`
3. Push: `git push r0 master`
4. ArgoCD syncs within 3 minutes
5. Next rollout uses new config

### Multiple Canary Steps

Progressive rollout with multiple weight changes:
```yaml
steps:
- setWeight: 10
- pause: {duration: 2m}
- setWeight: 25
- pause: {duration: 2m}
- setWeight: 50
- pause: {duration: 2m}
- setWeight: 100
```

Total time: ~6 minutes with gradual traffic increase.

## References

- [Argo Rollouts Canary Feature](https://argoproj.github.io/argo-rollouts/features/canary/)
- [Rollout Specification](https://argoproj.github.io/argo-rollouts/features/spec/)
- [kubectl-argo-rollouts Plug-in](https://argoproj.github.io/argo-rollouts/getting-started/#using-kubectl-with-argo-rollouts)
- [Progressive Delivery Patterns](https://www.weave.works/blog/what-is-progressive-delivery/)
