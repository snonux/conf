#!/bin/bash
# Argo Rollouts Canary Demo - Fully Automated
# Simulates a complete canary rollout with monitoring

set -e

NAMESPACE="services"
ROLLOUT="tracing-demo-frontend"
KUBE_CTX="$(kubectl config current-context)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m'

info() { echo -e "${BLUE}ℹ${NC} $1"; }
success() { echo -e "${GREEN}✓${NC} $1"; }
error() { echo -e "${RED}✗${NC} $1"; exit 1; }
step() { echo -e "\n${BOLD}=== $1 ===${NC}"; }

# Check prerequisites
step "Checking Prerequisites"

info "Cluster: $KUBE_CTX"

if ! kubectl get pods -n cicd -l app.kubernetes.io/name=argo-rollouts &>/dev/null; then
    error "Argo Rollouts controller not found. Install: cd argo-rollouts && just install"
fi
success "Argo Rollouts controller running"

if ! kubectl get rollout "$ROLLOUT" -n "$NAMESPACE" &>/dev/null; then
    error "Rollout $ROLLOUT not found in $NAMESPACE"
fi
success "Rollout $ROLLOUT found"

if ! kubectl argo rollouts version &>/dev/null; then
    error "kubectl argo rollouts plugin not installed"
fi
success "kubectl argo rollouts plugin available"

# Current state
step "Current Rollout State"
kubectl argo rollouts status "$ROLLOUT" -n "$NAMESPACE"
echo ""

# Demo plan
step "Demo Plan"
cat << 'EOF'
Timeline:
  0-15s:  Canary pod starting (Step 0/3, SetWeight 33%)
 15-60s:  Canary observing (Step 1/3, paused)
 60-90s:  Auto-promoting (Step 2/3, SetWeight 100%)
~90s:     Complete (Status Healthy)

What will be shown:
  • Real-time rollout progress
  • Pod replica counts
  • Canary vs Stable pods
  • Step progression
EOF
echo ""

read -p "Start demo? (y/n) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    info "Cancelled"
    exit 0
fi

# Show initial pod state
step "Initial Pod State"
kubectl get pods -n "$NAMESPACE" -l app="$ROLLOUT" -o wide --no-headers | awk '{print $1, $3, $4}'
echo ""

# Trigger rollout
step "Triggering Canary Rollout"
info "Triggering rollout by adding env var..."

TRIGGER_VALUE="$(date +%s)"
kubectl patch rollout "$ROLLOUT" -n "$NAMESPACE" \
  --type='json' \
  -p='[{"op":"add","path":"/spec/template/spec/containers/0/env/-","value":{"name":"ROLLOUT_DEMO_V","value":"'$TRIGGER_VALUE'"}}]' \
  > /dev/null

success "Rollout triggered (v=$TRIGGER_VALUE)"
echo ""

# Monitor rollout
step "Monitoring Rollout Progress"
echo "Watching rollout for ~100 seconds..."
echo ""

START_TIME=$(date +%s)
MAX_WAIT=120
COMPLETE=0

while true; do
    ELAPSED=$(($(date +%s) - START_TIME))
    
    # Get status
    STATUS=$(kubectl argo rollouts status "$ROLLOUT" -n "$NAMESPACE" 2>/dev/null || echo "Error")
    ROLLOUT_INFO=$(kubectl get rollout "$ROLLOUT" -n "$NAMESPACE" -o jsonpath='{.status.phase},{.status.currentStep},{.spec.strategy.canary.steps | length},{.status.canary.weights.canary.weight // 0},{.status.replicas},{.status.updatedReplicas},{.status.readyReplicas}' 2>/dev/null || echo "Unknown,0,3,0,0,0,0")
    
    IFS=',' read -r PHASE STEP TOTAL_STEPS WEIGHT CURRENT UPDATED READY <<< "$ROLLOUT_INFO"
    
    # Print progress line
    printf "\r[%02d:%02ds] %s | Step %s/%s | Weight %s%% | Replicas: %s (updated:%s ready:%s)  " \
        $((ELAPSED / 60)) $((ELAPSED % 60)) "$PHASE" "$STEP" "$TOTAL_STEPS" "$WEIGHT" "$CURRENT" "$UPDATED" "$READY"
    
    # Check if complete
    if [ "$PHASE" = "Healthy" ] && [ "$STEP" = "$TOTAL_STEPS" ]; then
        echo ""
        COMPLETE=1
        break
    fi
    
    # Check if exceeded max wait
    if [ $ELAPSED -ge $MAX_WAIT ]; then
        echo ""
        error "Timeout waiting for rollout to complete"
    fi
    
    sleep 2
done

echo ""

if [ $COMPLETE -eq 1 ]; then
    success "Rollout completed successfully!"
    echo ""
fi

# Show final state
step "Final Rollout State"
kubectl argo rollouts get rollout "$ROLLOUT" -n "$NAMESPACE"
echo ""

# Show final pods
step "Final Pod State"
kubectl get pods -n "$NAMESPACE" -l app="$ROLLOUT" -o wide --no-headers | awk '{print $1, $3, $4}'
echo ""

# Summary
step "Demo Summary"
echo "Total time: ${ELAPSED}s"
echo ""
cat << 'EOF'
What happened:
  1. ✓ Canary pod created with new revision
  2. ✓ 1 canary pod + 3 stable pods = 33% traffic to new version
  3. ✓ Paused for 1 minute to observe metrics
  4. ✓ Auto-promoted to 100% (all 3 pods = new version)
  5. ✓ Old pods terminated

This is progressive delivery:
  • Zero downtime
  • Validated before full rollout
  • Automatic promotion if healthy
  • Easy rollback if issues

To test more:
  • Run again: ./demo-canary-rollout.sh
  • Abort: kubectl argo rollouts abort tracing-demo-frontend -n services
  • Check logs: kubectl logs -n services -l app=tracing-demo-frontend -f
  • View history: kubectl argo rollouts history tracing-demo-frontend -n services
EOF
echo ""
success "Demo complete!"
