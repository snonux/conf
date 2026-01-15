# Argo Rollouts Deployment Checklist

Quick checklist for deploying and testing Argo Rollouts with canary demo.

## Installation

- [ ] Read `ARGO-ROLLOUTS-SUMMARY.md` - understand what was created
- [ ] Ensure kubectl access to f3s cluster
- [ ] Ensure ArgoCD is running
- [ ] Navigate to `/home/paul/git/conf/f3s/argo-rollouts`
- [ ] Run `just install`
- [ ] Verify controller: `kubectl get pods -n cicd -l app.kubernetes.io/name=argo-rollouts`
- [ ] Verify CRD: `kubectl get crd | grep rollout`
- [ ] (Optional) Install plugin: 
  ```bash
  curl -LO https://github.com/argoproj/argo-rollouts/releases/latest/download/kubectl-argo-rollouts-linux-amd64
  chmod +x kubectl-argo-rollouts-linux-amd64
  sudo install -m 755 kubectl-argo-rollouts-linux-amd64 /usr/local/bin/kubectl-argo-rollouts
  kubectl argo rollouts version
  ```

## ArgoCD Integration

- [ ] Push changes to git-server:
  ```bash
  cd /home/paul/git/conf/f3s
  git add -A && git commit -m "feat: add Argo Rollouts"
  git push r0 master
  ```
- [ ] Verify ArgoCD app:
  ```bash
  kubectl get application argo-rollouts -n cicd
  argocd app get argo-rollouts
  ```
- [ ] Verify tracing-demo app:
  ```bash
  kubectl get application tracing-demo -n cicd
  argocd app get tracing-demo
  ```

## Rollout Verification

- [ ] Check rollout exists: `kubectl get rollout tracing-demo-frontend -n services`
- [ ] Verify status: `kubectl describe rollout tracing-demo-frontend -n services`
- [ ] Expected: `Status: Healthy` with `3/3 replicas` in stable state
- [ ] Check pods: `kubectl get pods -n services -l app=tracing-demo-frontend`
- [ ] All 3 pods should be `Running`

## Demo: Basic Canary Rollout

**Expected: 0-15s: canary starting, 15-60s: observing, 60-90s: promoting**

### Terminal 1: Watch Rollout
```bash
cd /home/paul/git/conf/f3s/tracing-demo
just rollout-watch
```
- [ ] Command runs and connects to cluster
- [ ] Waiting for rollout to start

### Terminal 2: Trigger Rollout
```bash
kubectl patch rollout tracing-demo-frontend -n services \
  --type='json' \
  -p='[{"op":"add","path":"/spec/template/spec/containers/0/env/-","value":{"name":"ROLLOUT_V","value":"'$(date +%s)'"}}]'
```
- [ ] Patch command successful
- [ ] Terminal 1 shows change immediately

### Terminal 1: Observe Progress
- [ ] See `Step: 0/3, SetWeight: 33`
- [ ] 1 canary pod becoming ready
- [ ] 3 stable pods still running
- [ ] After ~15 sec: canary pod ready
- [ ] After ~60 sec: auto-promotion starts
- [ ] After ~90 sec: all 3 pods running new version
- [ ] Status shows `Healthy`

## Demo: Abort/Rollback

**Expected: Stop rollout and keep old version running**

### Terminal 1: Watch Rollout
```bash
just rollout-watch
```

### Terminal 2: Trigger Rollout
```bash
kubectl patch rollout tracing-demo-frontend -n services \
  --type='json' \
  -p='[{"op":"add","path":"/spec/template/spec/containers/0/env/-","value":{"name":"ROLLOUT_V2","value":"'$(date +%s)'"}}]'
```

### Terminal 3: Abort at Canary Step (after 20 seconds)
```bash
cd /home/paul/git/conf/f3s/tracing-demo
just rollout-abort
```
- [ ] Abort command accepted
- [ ] Terminal 1 shows `Status: Aborted`
- [ ] Canary pods terminate
- [ ] Old 3 pods continue running
- [ ] Verify with: `just rollout-status`

## Demo: Load Testing

**Expected: Generate traffic while rollout happens**

### Terminal 1: Watch Rollout
```bash
just rollout-watch
```

### Terminal 2: Start Load Test
```bash
just load-test &
```
- [ ] Requests being sent

### Terminal 3: Trigger Rollout
```bash
kubectl patch rollout tracing-demo-frontend -n services \
  --type='json' \
  -p='[{"op":"add","path":"/spec/template/spec/containers/0/env/-","value":{"name":"ROLLOUT_V3","value":"'$(date +%s)'"}}]'
```
- [ ] Rollout progresses with active traffic
- [ ] Both old and new pods serve requests during canary phase

## Monitoring

- [ ] Check status: `kubectl argo rollouts status tracing-demo-frontend -n services`
- [ ] Detailed info: `kubectl argo rollouts describe rollout tracing-demo-frontend -n services`
- [ ] Pod details: `kubectl get pods -n services -l app=tracing-demo-frontend -o wide`
- [ ] View logs: `just logs-frontend`
- [ ] View history: `just rollout-history`

## Grafana (Optional)

- [ ] Open Grafana: https://grafana.f3s.buetow.org
- [ ] Navigate to Explore → Tempo datasource
- [ ] Query: `{ resource.service.name = "frontend" }`
- [ ] See traces from old and new versions during rollout

## Integration with Git (GitOps)

- [ ] Edit rollout config:
  ```bash
  nano /home/paul/git/conf/f3s/tracing-demo/helm-chart/templates/frontend-rollout.yaml
  ```
- [ ] Change any settings (e.g., duration, setWeight)
- [ ] Commit and push:
  ```bash
  git add -A && git commit -m "chore: adjust canary settings"
  git push r0 master
  ```
- [ ] ArgoCD auto-syncs within 3 minutes (or force):
  ```bash
  kubectl annotate application tracing-demo -n cicd argocd.argoproj.io/refresh=normal --overwrite
  ```
- [ ] New settings take effect on next rollout trigger

## Post-Demo

- [ ] Abort any stuck rollouts: `just rollout-abort`
- [ ] Verify stable state: `just rollout-status` shows `Healthy`
- [ ] Review documentation:
  - [ ] `ARGO-ROLLOUTS-SUMMARY.md` - architecture
  - [ ] `ROLLOUTS-SETUP.md` - detailed scenarios
  - [ ] `README-ROLLOUTS.md` - quick reference
  - [ ] `tracing-demo/ROLLOUTS-DEMO.md` - technical details

## Troubleshooting

### Controller not running
```bash
kubectl get pods -n cicd -l app.kubernetes.io/name=argo-rollouts
kubectl logs -n cicd -l app.kubernetes.io/name=argo-rollouts
```
- [ ] Pod running and ready

### Rollout not deployed
```bash
kubectl get rollout tracing-demo-frontend -n services
kubectl describe rollout tracing-demo-frontend -n services
```
- [ ] Check events section for errors

### Canary pods in ImagePullBackoff
- [ ] Use env var patch instead (don't change image tag):
  ```bash
  kubectl patch rollout tracing-demo-frontend -n services \
    --type='json' \
    -p='[{"op":"add","path":"/spec/template/spec/containers/0/env/-","value":{"name":"ROLLOUT_V","value":"'$(date +%s)'"}}]'
  ```

### Rollout stuck in Progressing
```bash
kubectl describe rollout tracing-demo-frontend -n services
kubectl get pods -n services -l app=tracing-demo-frontend
```
- [ ] Check pod readiness probes
- [ ] Check pod resource requests/limits
- [ ] Check controller logs

## Next Steps

- [ ] Run through all demo scenarios multiple times
- [ ] Modify rollout settings and observe behavior
- [ ] Monitor with Prometheus/Grafana
- [ ] Extend to other services (middleware, backend)
- [ ] Optional: Install Istio for advanced traffic routing
- [ ] Optional: Deploy Flagger for automated analysis

---

**Setup Complete When:**
- ✅ Controller running in `cicd` namespace
- ✅ Rollout deployed in `services` namespace
- ✅ One full demo executed (0-90 seconds)
- ✅ Can abort and retry
- ✅ Team trained on canary deployments
