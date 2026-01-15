# Argo Rollouts Demo Scripts

Automated scripts to demonstrate Argo Rollouts canary deployments.

## Quick Start

```bash
cd /home/paul/git/conf/f3s/tracing-demo

# Interactive menu (easiest)
just demo-menu

# Or run specific demos
just demo-canary      # Full canary rollout (90s)
just demo-abort       # Test abort/rollback
just demo-reset       # Clean up between demos
```

## Scripts

### demo-canary-rollout.sh

Full automated canary deployment demo.

**What it does:**
1. Checks prerequisites (controller, rollout, plugin)
2. Shows current state
3. Triggers rollout by adding env var
4. Monitors progress in real-time (~90 seconds)
5. Shows final state

**Timeline:**
```
0-15s:   Canary pod launching (Step 0/3, SetWeight 33%)
15-60s:  Observing canary (Step 1/3, paused)
60-90s:  Auto-promoting (Step 2/3, SetWeight 100%)
~90s:    Complete (Status Healthy)
```

**Run:**
```bash
./demo-canary-rollout.sh
# or
just demo-canary
```

**Expected output:**
```
=== Checking Prerequisites ===
ℹ Cluster: k3s
✓ Argo Rollouts controller running
✓ Rollout tracing-demo-frontend found
✓ kubectl argo rollouts plugin available

=== Current Rollout State ===
Healthy

=== Triggering Canary Rollout ===
✓ Rollout triggered (v=1768504739)

=== Monitoring Rollout Progress ===
[01:30s] Healthy | Step 3/3 | Weight 100% | Replicas: 3 (updated:3 ready:3)

=== Demo Summary ===
✓ Demo complete!
```

### demo-abort-rollout.sh

Demonstrates aborting a rollout mid-canary.

**What it does:**
1. Triggers a new canary rollout
2. Waits 20 seconds for canary pod to be ready
3. Aborts the rollout
4. Shows that old version continues running

**Timeline:**
```
0-5s:    Canary pod launching
5-20s:   Waiting for ready
20s:     Abort issued
~20s+:   Canary pods terminated, old pods continue
```

**Run:**
```bash
./demo-abort-rollout.sh
# or
just demo-abort
```

**Shows:**
- Canary starting normally
- Mid-rollout abort is safe
- Old pods never interrupted
- Zero downtime

### demo-reset.sh

Resets rollout to clean state between demos.

**What it does:**
1. Aborts any in-progress rollout
2. Removes demo env vars
3. Waits for stabilization
4. Returns to clean state

**Run:**
```bash
./demo-reset.sh
# or
just demo-reset
```

Use between demo runs to avoid env var accumulation.

### demo-menu.sh

Interactive menu for choosing demos.

**Features:**
- Select demo scenario
- Check rollout status
- Watch live updates
- Exit cleanly

**Run:**
```bash
./demo-menu.sh
# or
just demo-menu
```

**Options:**
```
1) Run full canary rollout demo (~90s)
2) Abort rollout demo (~20s)
3) Reset rollout
4) Check status
5) Watch live (real-time)
0) Exit
```

## Usage Examples

### First Time - Full Demo

```bash
cd /home/paul/git/conf/f3s/tracing-demo
just demo-menu

# Select option 1: Run full canary rollout demo
# Watch it progress from canary → observe → promote
```

### Test Abort Behavior

```bash
just demo-menu

# Select option 2: Abort rollout demo
# See canary start, then abort mid-rollout
```

### Run Full Sequence

```bash
# Demo 1: Canary rollout
just demo-canary

# Clean up
just demo-reset

# Demo 2: Abort behavior
just demo-abort

# Clean up
just demo-reset

# Check final state
just rollout-status
```

### Watch Live (No Automation)

```bash
# Start in one terminal
just demo-menu
# Select 5: Watch live

# In another terminal, trigger manually
kubectl patch rollout tracing-demo-frontend -n services \
  --type='json' \
  -p='[{"op":"add","path":"/spec/template/spec/containers/0/env/-","value":{"name":"ROLLOUT_V","value":"'$(date +%s)'"}}]'
```

## Requirements

- kubectl configured for f3s cluster
- Argo Rollouts controller installed (`cd argo-rollouts && just install`)
- kubectl argo rollouts plugin installed
- jq (for parsing JSON)

## Troubleshooting

### "Argo Rollouts controller not found"

Install it:
```bash
cd /home/paul/git/conf/f3s/argo-rollouts
just install
```

### "Rollout not found"

Apply the rollout:
```bash
kubectl apply -f helm-chart/templates/frontend-rollout.yaml
```

### "Plugin not installed"

Install it:
```bash
curl -LO https://github.com/argoproj/argo-rollouts/releases/latest/download/kubectl-argo-rollouts-linux-amd64
sudo install -m 755 kubectl-argo-rollouts-linux-amd64 /usr/local/bin/kubectl-argo-rollouts
```

### Rollout stuck / loops

This shouldn't happen with ArgoCD ignoreDifferences configured. Check:
```bash
kubectl get application tracing-demo -n cicd -o yaml | grep -A 10 ignoreDifferences
```

If ArgoCD is reverting patches, disable auto-sync:
```bash
kubectl patch application tracing-demo -n cicd \
  --type='json' \
  -p='[{"op":"replace","path":"/spec/syncPolicy/automated","value":null}]'
```

Then re-enable after demo:
```bash
kubectl patch application tracing-demo -n cicd \
  --type='json' \
  -p='[{"op":"replace","path":"/spec/syncPolicy/automated","value":{"prune":true,"selfHeal":true}}]'
```

## Advanced

### Manual Triggers (Without Scripts)

If you want to trigger rollouts manually:

```bash
# Trigger with env var (used by scripts)
kubectl patch rollout tracing-demo-frontend -n services \
  --type='json' \
  -p='[{"op":"add","path":"/spec/template/spec/containers/0/env/-","value":{"name":"ROLLOUT_V","value":"'$(date +%s)'"}}]'

# Watch progress
kubectl argo rollouts get rollout tracing-demo-frontend -n services --watch

# Promote early (skip waiting)
kubectl argo rollouts promote tracing-demo-frontend -n services

# Abort rollout
kubectl argo rollouts abort tracing-demo-frontend -n services
```

### Modify Canary Settings

Edit `/home/paul/git/conf/f3s/tracing-demo/helm-chart/templates/frontend-rollout.yaml` to change:
- `duration: 1m` → longer observation time
- `setWeight: 33` → different traffic percentage
- `replicas: 3` → more/fewer pods

Then commit and apply:
```bash
git add -A && git commit -m "chore: adjust canary"
git push r0 master
kubectl annotate application tracing-demo -n cicd argocd.argoproj.io/refresh=normal --overwrite
```

## See Also

- `ROLLOUTS-DEMO.md` - Technical details
- `ROLLOUTS-SETUP.md` - Setup guide with 5 scenarios
- `README-ROLLOUTS.md` - Quick reference
- `ARGO-ROLLOUTS-SUMMARY.md` - Architecture overview
