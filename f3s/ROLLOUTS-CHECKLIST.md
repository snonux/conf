# Argo Rollouts Deployment Checklist

## Pre-Deployment Setup

- [ ] Read `ARGO-ROLLOUTS-SUMMARY.md` to understand what was created
- [ ] Ensure kubectl access to f3s cluster
- [ ] Ensure ArgoCD is running and accessible
- [ ] Git repository (conf.git) synced to git-server

## Installation

- [ ] Navigate to `/home/paul/git/conf/f3s/argo-rollouts`
- [ ] Run `just install` to deploy controller
- [ ] Verify controller running: `kubectl get pods -n cicd -l app.kubernetes.io/name=argo-rollouts`
- [ ] Verify CRD installed: `kubectl get crd | grep rollout`

## Optional: Install kubectl Plugin

- [ ] Download kubectl-argo-rollouts:
  ```bash
  curl -LO https://github.com/argoproj/argo-rollouts/releases/latest/download/kubectl-argo-rollouts-linux-amd64
  chmod +x kubectl-argo-rollouts-linux-amd64
  sudo install -m 755 kubectl-argo-rollouts-linux-amd64 /usr/local/bin/kubectl-argo-rollouts
  ```
- [ ] Verify: `kubectl argo rollouts version`

## ArgoCD Syncing

- [ ] Create/push `argocd-apps/cicd/argo-rollouts.yaml` to git
- [ ] Create/push `argocd-apps/services/tracing-demo.yaml` updates to git
- [ ] Force ArgoCD sync (wait 3 min or manual):
  ```bash
  argocd app sync argo-rollouts
  argocd app sync tracing-demo
  ```
- [ ] Verify tracing-demo application status: `argocd app get tracing-demo`

## Rollout Verification

- [ ] Check frontend rollout deployed: `kubectl get rollout tracing-demo-frontend -n services`
- [ ] Verify status: `kubectl describe rollout tracing-demo-frontend -n services`
- [ ] Expected: `Status: Healthy` with `2/2 replicas` in stable state
- [ ] Check pods running: `kubectl get pods -n services -l app=tracing-demo-frontend`

## Basic Demo (First Time)

### Terminal 1: Watch Rollout
```bash
cd /home/paul/git/conf/f3s/tracing-demo
just rollout-watch
```
- [ ] Command running and connected

### Terminal 2: Generate Load (Optional)
```bash
cd /home/paul/git/conf/f3s/tracing-demo
just load-test &
```
- [ ] Requests being sent to frontend

### Terminal 3: Trigger Rollout
Choose one method:

**Method A: Kubectl Patch (Fastest)**
```bash
kubectl patch rollout tracing-demo-frontend -n services \
  --type='json' \
  -p='[{"op":"replace","path":"/spec/template/spec/containers/0/image","value":"registry.lan.buetow.org:30001/tracing-demo-frontend:latest"}]'
```
- [ ] Executed successfully

**Method B: Git + ArgoCD (Most GitOps)**
```bash
cd /home/paul/git/conf/f3s
# Edit tracing-demo/helm-chart/templates/frontend-rollout.yaml (change image tag)
git add -A
git commit -m "chore: update frontend image for demo"
git remote add r0 ssh://git@r0:30022/repos/conf.git 2>/dev/null || true
git push r0 master
kubectl annotate application tracing-demo -n cicd argocd.argoproj.io/refresh=normal --overwrite
```
- [ ] Git push successful
- [ ] ArgoCD syncing (check web UI or CLI)

## Demo Observation

- [ ] Terminal 1 shows: "Progressing" → "canary step 1/3"
- [ ] After ~30 sec: New canary pod appears
- [ ] After ~2 min: "canary step 2/3" (pause)
- [ ] After ~4 min: "canary step 3/3" (100% traffic)
- [ ] After ~4:20 min: Status shows "Healthy"
- [ ] Old pods terminated, 2 new pods in stable state

## Monitoring (Optional)

- [ ] Check logs: `just logs-frontend`
- [ ] Check Grafana Tempo for traces: https://grafana.f3s.buetow.org
  - [ ] Navigate to Explore → Tempo
  - [ ] Query: `{ resource.service.name = "frontend" }`
  - [ ] See traces from old and new versions
- [ ] Check Prometheus metrics: Port-forward and query

## Advanced Scenarios

### Scenario 1: Manual Promotion
- [ ] Trigger rollout (step above)
- [ ] After step 1 (30 sec), run:
  ```bash
  just rollout-promote
  ```
- [ ] Watch rollout skip step 2, immediately promote to 100%
- [ ] Verify: `just rollout-status` shows "Healthy"

### Scenario 2: Abort/Rollback
- [ ] Trigger rollout
- [ ] While progressing, run:
  ```bash
  just rollout-abort
  ```
- [ ] Watch canary pods terminate
- [ ] Old version continues running
- [ ] Verify: `just rollout-status` shows "Aborted"

### Scenario 3: Check History
- [ ] After any rollout:
  ```bash
  just rollout-history
  ```
- [ ] See previous revisions and their status

## Integration with CI/CD

- [ ] Image builds automatically on git push (or configured pipeline)
- [ ] New image pushed to registry: `registry.lan.buetow.org:30001/tracing-demo-frontend:NEWTAG`
- [ ] Git updated with new image tag
- [ ] ArgoCD detects change
- [ ] Rollout automatically triggered
- [ ] Canary strategy executes

## Post-Deployment

- [ ] Share documentation:
  - [ ] `ROLLOUTS-SETUP.md` - Complete setup guide
  - [ ] `tracing-demo/ROLLOUTS-DEMO.md` - Detailed walkthrough
  - [ ] `ARGO-ROLLOUTS-SUMMARY.md` - Architecture overview
- [ ] Add team to `kubectl argo rollouts` usage
- [ ] Consider next steps:
  - [ ] Deploy Istio for advanced traffic management
  - [ ] Add Flagger for automated analysis
  - [ ] Extend to other services (middleware, backend)
  - [ ] Create monitoring dashboards

## Troubleshooting Checklist

### Controller not running
- [ ] Check pod: `kubectl get pods -n cicd -l app.kubernetes.io/name=argo-rollouts`
- [ ] Check logs: `kubectl logs -n cicd -l app.kubernetes.io/name=argo-rollouts`
- [ ] Check CRD: `kubectl get crd | grep rollout`

### Rollout not deploying
- [ ] Check ArgoCD sync: `argocd app get tracing-demo`
- [ ] Check git changes pushed: `git log --oneline | head -5`
- [ ] Force sync: `argocd app sync tracing-demo --prune`

### Canary pods not starting
- [ ] Check pod status: `kubectl describe pod -n services <pod-name>`
- [ ] Check logs: `kubectl logs -n services <pod-name>`
- [ ] Check resource limits: `kubectl top pods -n services`
- [ ] Check image: `kubectl get pods -n services -o jsonpath='{.items[*].spec.containers[0].image}'`

### Rollout stuck in Progressing
- [ ] Check health probes: `kubectl get rollout tracing-demo-frontend -n services -o yaml | grep -A 10 health`
- [ ] Check replica status: `kubectl get rs -n services -l app=tracing-demo-frontend -o wide`
- [ ] Check controller logs: `kubectl logs -n cicd -l app.kubernetes.io/name=argo-rollouts --tail=50`

## Cleanup (If Needed)

- [ ] Stop rollout: `kubectl argo rollouts abort tracing-demo-frontend -n services`
- [ ] Rollback to previous: `kubectl rollout undo deployment/tracing-demo-frontend -n services` (if needed)
- [ ] Uninstall Argo Rollouts: `cd argo-rollouts && just uninstall`

---

**Setup complete when:**
- ✅ Argo Rollouts controller running in `cicd` namespace
- ✅ Frontend rollout deployed in `services` namespace
- ✅ ArgoCD recognizes rollout resource
- ✅ One demo run successful (git trigger or kubectl patch)
- ✅ Team can watch and manage rollouts
