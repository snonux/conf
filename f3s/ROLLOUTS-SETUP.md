# Argo Rollouts Setup and Demo Guide

This guide covers the complete setup and demonstration of Argo Rollouts with the tracing-demo application.

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

The following ArgoCD Applications will be auto-synced:

- **argo-rollouts.yaml** - Installs Argo Rollouts controller
- **tracing-demo.yaml** - Now uses Rollout (frontend) + Deployments (middleware, backend)

Force ArgoCD to sync:
```bash
argocd app sync argo-rollouts
argocd app sync tracing-demo
```

Or wait for auto-sync (default: 3 minutes).

### 4. Verify Rollout is Deployed

```bash
kubectl get rollout tracing-demo-frontend -n services
kubectl describe rollout tracing-demo-frontend -n services
```

Expected status: `Stable` with `2/2 replicas`.

## Demo Scenarios

### Scenario 1: Basic Canary Rollout (Guided)

**Duration**: ~5-10 minutes

**Objective**: Observe frontend rollout from 50% → 100% traffic with auto-promotion.

#### Step 1: Prepare Terminals

Terminal 1 - Watch rollout progress:
```bash
cd /home/paul/git/conf/f3s/tracing-demo
just rollout-watch
```

Terminal 2 - Generate load:
```bash
cd /home/paul/git/conf/f3s/tracing-demo
just load-test &
```

Terminal 3 - Trigger rollout:
```bash
# Will use this in next step
```

#### Step 2: Trigger Rollout (Terminal 3)

Simulate updating the frontend image:

```bash
kubectl patch rollout tracing-demo-frontend -n services \
  --type='json' \
  -p='[{"op":"replace","path":"/spec/template/spec/containers/0/image","value":"registry.lan.buetow.org:30001/tracing-demo-frontend:latest"}]'
```

Or via git (more GitOps-like):

```bash
cd /home/paul/git/conf/f3s
# Edit tracing-demo/helm-chart/templates/frontend-rollout.yaml (change image tag)
git add -A
git commit -m "chore: update frontend image for demo"
git remote add r0 ssh://git@r0:30022/repos/conf.git 2>/dev/null || true
git push r0 master

# Trigger ArgoCD sync
kubectl annotate application tracing-demo -n cicd argocd.argoproj.io/refresh=normal --overwrite
```

#### Step 3: Observe Rollout (Terminal 1)

Watch the output:

```
NAME                           KIND        STATUS     AGE    INFO
tracing-demo-frontend          Rollout     Progressing  0s    canary step 1/3
tracing-demo-frontend-abc123   ReplicaSet  ✓ canary    5s    1/1 replicas
tracing-demo-frontend-xyz789   ReplicaSet  ✓ stable    5m    2/2 replicas

NAME                           KIND        STATUS     AGE    INFO
tracing-demo-frontend          Rollout     Progressing  2m5s  canary step 2/3
tracing-demo-frontend-abc123   ReplicaSet  ✓ canary    2m    1/1 replicas (ready)
tracing-demo-frontend-xyz789   ReplicaSet  ✓ stable    5m    2/2 replicas

NAME                           KIND        STATUS     AGE    INFO
tracing-demo-frontend          Rollout     Progressing  4m10s canary step 3/3
tracing-demo-frontend-abc123   ReplicaSet  ✓ canary    4m    2/2 replicas (ready, updated)
tracing-demo-frontend-xyz789   ReplicaSet  ✓ stable    5m    0/2 replicas (pending termination)

NAME                           KIND        STATUS  AGE    INFO
tracing-demo-frontend          Rollout     ✓ Healthy  4m20s
tracing-demo-frontend-abc123   ReplicaSet  ✓ stable  4m    2/2 replicas
```

**Timeline:**
- **0-2 min**: Step 1 (setWeight: 50) - 1 canary pod, 2 stable pods, 50/50 traffic
- **2-4 min**: Step 2 (pause: 2m) - Waiting for user or auto-promotion
- **4+ min**: Step 3 (setWeight: 100) - All 2 canary pods promoted, old pods terminated
- **4:20 min**: Complete - New version fully deployed

#### Step 4: Observe Behavior (Optional)

Check request latency/errors during rollout:

```bash
# View logs from both old and new pods
kubectl logs -n services -l app=tracing-demo-frontend --timestamps=true | tail -20

# Check if any requests failed during transition
grep -i "error\|exception" <(kubectl logs -n services -l app=tracing-demo-frontend)
```

View traces in Grafana:
1. Navigate to https://grafana.f3s.buetow.org
2. Explore → Tempo
3. Query: `{ resource.service.name = "frontend" }`
4. See traces from both old and new versions

### Scenario 2: Manual Promotion (Skip Waiting)

**Duration**: ~2 minutes

**Objective**: Demonstrate manual control - don't wait for auto-promotion.

#### Setup

Trigger rollout (same as Scenario 1):
```bash
kubectl patch rollout tracing-demo-frontend -n services \
  --type='json' \
  -p='[{"op":"replace","path":"/spec/template/spec/containers/0/image","value":"registry.lan.buetow.org:30001/tracing-demo-frontend:latest"}]'
```

Watch:
```bash
just rollout-watch
```

#### Promote Early

After canary looks healthy (step 1 complete, ~30 seconds):

```bash
cd /home/paul/git/conf/f3s/tracing-demo
just rollout-promote
```

This skips the 2-minute pause and immediately promotes to 100%.

### Scenario 3: Abort/Rollback

**Duration**: ~3 minutes

**Objective**: Demonstrate rollback if canary fails.

#### Setup & Trigger

Same as Scenario 1.

#### Simulate Failure

While at canary step 1 (50% traffic), introduce a failure:

```bash
# Get one of the new canary pods
CANARY_POD=$(kubectl get pods -n services -l app=tracing-demo-frontend -o name | tail -1)

# Kill it to simulate crash
kubectl delete $CANARY_POD -n services
```

Watch in Terminal 1 - the rollout may stall or fail health checks.

#### Abort

```bash
cd /home/paul/git/conf/f3s/tracing-demo
just rollout-abort
```

This:
- Stops the rollout
- Terminates canary replicas
- Restores stable version with 2 pods
- Allows investigation

Verify:
```bash
just rollout-status
```

Expected: `Rollout has been aborted. Stable ReplicaSet: 2/2 replicas`

### Scenario 4: Observability - Prometheus Metrics

**Duration**: ~5 minutes (during any rollout)

**Objective**: Monitor rollout via Prometheus metrics.

During a running rollout:

```bash
# Port-forward Prometheus
kubectl port-forward -n monitoring svc/prometheus 9090:9090 &

# Open browser: http://localhost:9090
```

Query useful metrics:

```promql
# Rollout replica counts
kube_statefulset_replicas{statefulset=~".*frontend.*"}
kube_replicaset_created{replicaset=~".*frontend.*"}

# Pod status during rollout
kube_pod_status_phase{namespace="services", pod=~".*frontend.*"}

# Request latency (if your app exports metrics)
rate(http_requests_total{job="frontend"}[5m])

# Error rate
rate(http_requests_total{job="frontend", status=~"5.."}[5m])
```

### Scenario 5: GitOps Flow (Realistic)

**Duration**: ~10 minutes

**Objective**: Demonstrate GitOps workflow - git commit triggers rollout via ArgoCD.

#### Step 1: Modify Frontend Code

```bash
cd /home/paul/git/conf/f3s/tracing-demo/docker/frontend
# Edit app.py (e.g., change response message)
# Commit and push
git add -A
git commit -m "feat: update frontend message"
git push origin master
```

#### Step 2: Rebuild and Push Image

```bash
cd /home/paul/git/conf/f3s/tracing-demo
just build-push
```

This creates new Docker image tagged with latest commit hash or timestamp.

#### Step 3: Update Helm Chart

```bash
# Edit frontend-rollout.yaml with new image tag
nano /home/paul/git/conf/f3s/tracing-demo/helm-chart/templates/frontend-rollout.yaml
# Change image: registry.lan.buetow.org:30001/tracing-demo-frontend:NEWTAG

git add -A
git commit -m "chore: update frontend rollout image to latest"
git remote add r0 ssh://git@r0:30022/repos/conf.git 2>/dev/null || true
git push r0 master
```

#### Step 4: ArgoCD Syncs Automatically

Wait 3 minutes or force sync:
```bash
argocd app sync tracing-demo --prune
```

ArgoCD detects the new image in git and updates the rollout.

#### Step 5: Watch Rollout Progress

```bash
just rollout-watch
```

The canary strategy executes: 50% → wait 2min → 100%.

## Monitoring Dashboard

Create a Grafana dashboard to visualize rollout progress:

1. Open Grafana: https://grafana.f3s.buetow.org
2. Dashboards → New → Create
3. Add panels:

**Panel 1: Rollout Status**
```promql
kube_rollout_status_current_step{rollout="tracing-demo-frontend"}
```

**Panel 2: Replica Counts**
```promql
topk(2, kube_replicaset_replicas{replicaset=~"tracing-demo-frontend.*"})
```

**Panel 3: Pod Age**
```promql
time() - kube_pod_created{namespace="services", pod=~"tracing-demo-frontend.*"}
```

**Panel 4: Request Rate**
```promql
rate(http_requests_total{job="tracing-demo-frontend"}[1m])
```

## Advanced: Custom Analysis

To add automated health checks during canary (e.g., error rate thresholds), integrate with **Flagger**:

```yaml
apiVersion: flagger.app/v1beta1
kind: Canary
metadata:
  name: tracing-demo-frontend
spec:
  targetRef:
    apiVersion: argoproj.io/v1alpha1
    kind: Rollout
    name: tracing-demo-frontend
  progressDeadlineSeconds: 300
  service:
    port: 5000
  analysis:
    interval: 1m
    threshold: 2
    maxWeight: 50
    stepWeight: 10
    metrics:
    - name: error_rate
      thresholdRange:
        max: 1  # Max 1% error rate
```

This requires installing **Flagger** and requires a service mesh (Istio/Linkerd).

## Troubleshooting

### Rollout Stuck in Progressing

```bash
kubectl describe rollout tracing-demo-frontend -n services
```

Check for:
- Pod failures (CrashLoopBackOff)
- Image pull errors
- Resource exhaustion
- Health probe failures

### Canary Pods Not Becoming Ready

```bash
kubectl get pods -n services -l app=tracing-demo-frontend -o wide
kubectl logs -n services -l app=tracing-demo-frontend --tail=50
```

### ArgoCD Not Syncing Rollout Changes

```bash
kubectl get application tracing-demo -n cicd -o jsonpath='{.status.sync.status}'
argocd app sync tracing-demo
```

### kubectl argo rollouts Plugin Issues

```bash
kubectl argo rollouts version

# If not installed or outdated:
curl -LO https://github.com/argoproj/argo-rollouts/releases/latest/download/kubectl-argo-rollouts-linux-amd64
sudo install -m 755 kubectl-argo-rollouts-linux-amd64 /usr/local/bin/kubectl-argo-rollouts
```

## Next Steps

1. **Try all scenarios** to understand rollout behavior
2. **Deploy Istio** for advanced traffic management (weighted routing, header-based)
3. **Add Prometheus queries** to monitor rollout metrics
4. **Implement Flagger** for automated analysis and rollback
5. **Migrate other services** to Rollout (start with low-risk apps)

## References

- [Argo Rollouts Canary Strategy](https://argoproj.github.io/argo-rollouts/features/canary/)
- [Argo Rollouts Blue-Green Strategy](https://argoproj.github.io/argo-rollouts/features/bluegreen/)
- [Flagger Documentation](https://flagger.app/)
- [Istio VirtualService](https://istio.io/latest/docs/reference/config/networking/virtual-service/)
