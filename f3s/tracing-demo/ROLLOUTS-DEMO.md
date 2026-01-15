# Argo Rollouts Demo Guide for Tracing-Demo

This guide demonstrates progressive delivery using Argo Rollouts with the tracing-demo frontend service.

## Prerequisites

- Argo Rollouts installed in `cicd` namespace
- ArgoCD synced with the latest conf.git
- tracing-demo-frontend rollout deployed
- kubectl argo rollouts plugin installed

### Install kubectl argo rollouts plugin

```bash
curl -LO https://github.com/argoproj/argo-rollouts/releases/latest/download/kubectl-argo-rollouts-linux-amd64
chmod +x kubectl-argo-rollouts-linux-amd64
sudo mv kubectl-argo-rollouts-linux-amd64 /usr/local/bin/kubectl-argo-rollouts
```

## Demo Workflow

### 1. Verify Current State

```bash
# Check frontend rollout
kubectl get rollout tracing-demo-frontend -n services

# Get detailed status
kubectl argo rollouts status tracing-demo-frontend -n services
kubectl argo rollouts get rollout tracing-demo-frontend -n services
```

Expected output shows 2 stable replicas, 0 canary.

### 2. Generate Load (Optional but Recommended)

In a separate terminal, generate traffic to the frontend:

```bash
# Port-forward frontend
kubectl port-forward -n services svc/frontend-service 5000:5000 &

# Send requests in a loop
while true; do
  curl http://localhost:5000/api/process -s | jq .
  sleep 1
done
```

Or use the load test:

```bash
cd /home/paul/git/conf/f3s/tracing-demo
just load-test &
```

### 3. Trigger a New Rollout

Simulate updating the frontend image (e.g., new version):

```bash
# Method 1: Patch the rollout to trigger new image
kubectl patch rollout tracing-demo-frontend -n services \
  --type='json' \
  -p='[{"op":"replace","path":"/spec/template/spec/containers/0/image","value":"registry.lan.buetow.org:30001/tracing-demo-frontend:latest"}]'
```

Or, more realistically, push a new image tag and update git:

```bash
cd /home/paul/git/conf/f3s/tracing-demo/helm-chart/templates
# Edit frontend-rollout.yaml and change image tag
# Then commit and push to git-server
git commit -am "chore: update frontend image"
git push origin master
```

ArgoCD will auto-sync and apply the new image, triggering the rollout.

### 4. Watch the Rollout Progress

**Terminal 1: Real-time rollout status (refreshes)**

```bash
kubectl argo rollouts get rollout tracing-demo-frontend -n services --watch
```

You'll see:
```
NAME                           KIND        STATUS     AGE    INFO
tracing-demo-frontend          Rollout     Progressing  2m    canary step 1/3
tracing-demo-frontend-abc123   ReplicaSet  ✓ canary    2m    2/2 replicas
tracing-demo-frontend-xyz789   ReplicaSet  ✓ stable    5m    2/2 replicas
```

**Terminal 2: Pod status**

```bash
kubectl get pods -n services -l app=tracing-demo-frontend -w
```

Shows new canary pods being created, old stable pods remaining.

**Terminal 3: Service endpoints (traffic split)**

```bash
watch -n 1 'kubectl get endpoints -n services frontend-service'
```

During canary, both old and new endpoints visible (50/50 traffic).

### 5. Key Rollout States

**Progressing (Canary Step 1: 50% Traffic)**
- Duration: 0-2 minutes
- New canary replicas (1 out of 2) serve traffic
- Old stable replicas (1 out of 2) serve traffic
- Health checks and error rates monitored

**Paused (Canary Step 2: Hold)**
- Duration: 2 minutes
- Allows observing new version behavior
- Watch metrics in Grafana/Prometheus
- Can manually promote or abort

**Full Promotion (Canary Step 3: 100% Traffic)**
- After 2 minutes, auto-promotes to 100%
- New replicas become stable
- Old replicas terminated

### 6. Monitor Canary Behavior

While rollout is progressing, check application health:

**Check logs of new canary pods:**

```bash
# Get canary replica set revision
CANARY_REVISION=$(kubectl get rs -n services -l app=tracing-demo-frontend --sort-by='.metadata.creationTimestamp' | tail -1 | awk '{print $1}')

# View logs
kubectl logs -n services -l app=tracing-demo-frontend,controller-revision-hash=$CANARY_REVISION --tail=50 -f
```

**Check Prometheus metrics:**

```bash
# Query frontend endpoint availability
curl -s 'http://localhost:9090/api/v1/query?query=up{job="tracing-demo-frontend"}' | jq

# Query request error rate
curl -s 'http://localhost:9090/api/v1/query?query=rate(http_requests_total{job="tracing-demo-frontend",status=~"5.."}[5m])' | jq
```

**Check traces in Grafana Tempo:**

Navigate to Grafana Explore → Tempo, query:
```
{ resource.service.name = "frontend" }
```

Watch traces from both old and new versions.

### 7. Manual Promotion (Skip Waiting)

If canary looks healthy, skip the 2-minute wait:

```bash
kubectl argo rollouts promote tracing-demo-frontend -n services
```

Immediately promotes to 100% traffic, completes rollout.

### 8. Abort/Rollback

If canary is unhealthy, abort the rollout:

```bash
kubectl argo rollouts abort tracing-demo-frontend -n services
```

This:
- Stops the rollout progression
- Terminates canary replicas
- Keeps previous stable version running
- Allows investigation and retry

### 9. View Rollout History

```bash
kubectl argo rollouts history tracing-demo-frontend -n services

# Get details of specific revision
kubectl argo rollouts history tracing-demo-frontend -n services --revision=2
```

## Demo Variations

### A. Inject a Failure

Simulate unhealthy canary by making requests fail:

```bash
# Get canary pod name
CANARY_POD=$(kubectl get pods -n services -l app=tracing-demo-frontend -o jsonpath='{.items[1].metadata.name}')

# Inject failure (e.g., kill process)
kubectl exec -n services $CANARY_POD -- sh -c 'kill 1' &

# Watch error rate spike in terminal watching rollout
# Rollout will stall at canary step, waiting for stable metrics
```

### B. Compare Old vs New via Load Testing

```bash
# Terminal 1: Watch rollout at 50% traffic
kubectl argo rollouts get rollout tracing-demo-frontend -n services --watch

# Terminal 2: Run load test
cd /home/paul/git/conf/f3s/tracing-demo && just load-test

# Terminal 3: Check if latency differs between old/new
kubectl logs -n services -l app=tracing-demo-frontend --tail=20 --timestamps=true
```

### C. Long-running Canary

Edit frontend-rollout.yaml to increase pause duration:

```yaml
- pause:
    duration: 10m  # Observe for 10 minutes
```

Allows extended monitoring and confidence building.

## Architecture Notes

### No Service Mesh Required

This demo uses **native Kubernetes service routing** (simple round-robin). Traffic splitting happens at the pod replica level:

- Stable ReplicaSet: 2 pods (or 1 out of 2)
- Canary ReplicaSet: 0 pods (or 1 out of 2)
- Service selects both ReplicaSets
- K8s load-balancer distributes traffic proportionally

**To get more sophisticated traffic splitting (header-based, percentage-based), install:**
- **Istio** with VirtualService/DestinationRule
- **Linkerd** with Rollout extension
- **Flagger** for automated canary analysis

### Advanced Rollout Strategies

Once comfortable with canary, try:

**Blue-Green** (instant switch, easy rollback):
```yaml
strategy:
  blueGreen:
    activeSlotSelector: stable
    autoPromotionEnabled: true
    autoPromotionSeconds: 120
```

**A/B Testing** (route by header):
```yaml
strategy:
  canary:
    trafficRouting:
      istio:
        virtualService:
          name: frontend
          routes:
          - name: primary  # 95% traffic
          - name: canary   # 5% traffic, routed by header
```

## Troubleshooting

### Rollout Stuck in Progressing

```bash
# Check rollout conditions
kubectl describe rollout tracing-demo-frontend -n services

# Check controller logs
kubectl logs -n cicd -l app.kubernetes.io/name=argo-rollouts --tail=50
```

Common causes:
- ReplicaSet not becoming ready (image pull error, resource limits)
- Health probe failing
- ArgoCD out of sync

### Traffic Not Splitting 50/50

Native K8s balancing may not be exactly 50/50 due to:
- Connection pooling by clients
- Load balancer algorithm
- Pod restart timing

For precise traffic splitting, use Istio or Linkerd.

### View Rollout in ArgoCD UI

1. Open ArgoCD: https://argocd.f3s.buetow.org
2. Click tracing-demo application
3. Expand frontend-rollout resource
4. See real-time status and sync history

## References

- [Argo Rollouts Canary Guide](https://argoproj.github.io/argo-rollouts/features/canary/)
- [Argo Rollouts Kubectl Plugin](https://argoproj.github.io/argo-rollouts/getting-started/#using-kubectl-with-argo-rollouts)
- [Progressive Delivery Patterns](https://argoproj.github.io/argo-rollouts/concepts/)
