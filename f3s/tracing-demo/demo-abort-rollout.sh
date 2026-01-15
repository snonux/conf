#!/bin/bash
# Argo Rollouts Abort Demo - Test rollback behavior

NAMESPACE="services"
ROLLOUT="tracing-demo-frontend"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m'

info() { echo -e "${BLUE}ℹ${NC} $1"; }
success() { echo -e "${GREEN}✓${NC} $1"; }
step() { echo -e "\n${BOLD}=== $1 ===${NC}"; }

step "Rollout Abort Demo"
echo ""
echo "This will:"
echo "  1. Trigger a new canary rollout"
echo "  2. Wait 20 seconds for canary to become ready"
echo "  3. Abort the rollout mid-canary"
echo "  4. Show that old version continues running"
echo ""

read -p "Start? (y/n) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    exit 0
fi

# Trigger rollout
step "Step 1: Triggering canary rollout..."
TRIGGER_VALUE="$(date +%s)"
kubectl patch rollout "$ROLLOUT" -n "$NAMESPACE" \
  --type='json' \
  -p='[{"op":"add","path":"/spec/template/spec/containers/0/env/-","value":{"name":"ROLLOUT_ABORT_TEST","value":"'$TRIGGER_VALUE'"}}]' \
  > /dev/null

success "Rollout triggered"
echo ""

# Wait for canary to start
step "Step 2: Waiting for canary pod to become ready..."
echo "Monitoring..."

WAITED=0
MAX_WAIT=30
while [ $WAITED -lt $MAX_WAIT ]; do
    READY=$(kubectl get rollout "$ROLLOUT" -n "$NAMESPACE" -o jsonpath='{.status.readyReplicas}')
    UPDATED=$(kubectl get rollout "$ROLLOUT" -n "$NAMESPACE" -o jsonpath='{.status.updatedReplicas}')
    STEP=$(kubectl get rollout "$ROLLOUT" -n "$NAMESPACE" -o jsonpath='{.status.currentStep}')
    
    printf "\r[%02ds] Ready: %s, Updated: %s, Step: %s  " $WAITED "$READY" "$UPDATED" "$STEP"
    
    # Check if canary is ready (4 total ready = 3 stable + 1 canary)
    if [ "$READY" = "4" ]; then
        echo ""
        success "Canary pod ready!"
        break
    fi
    
    sleep 1
    WAITED=$((WAITED + 1))
done
echo ""

# Abort the rollout
step "Step 3: Aborting rollout..."
kubectl argo rollouts abort "$ROLLOUT" -n "$NAMESPACE" > /dev/null
success "Rollout aborted"
echo ""

# Show status
step "Step 4: Checking status after abort"
sleep 2
kubectl argo rollouts status "$ROLLOUT" -n "$NAMESPACE"
echo ""

# Show pods
step "Pod Status After Abort"
echo "Notice: Canary pod is gone, 3 stable pods still running"
echo ""
kubectl get pods -n "$NAMESPACE" -l app="$ROLLOUT" -o wide --no-headers | awk '{print $1, $3, $4}'
echo ""

step "Summary"
cat << 'EOF'
What happened:
  ✓ Canary started (4 pods: 3 stable + 1 canary)
  ✓ Abort command issued while in canary phase
  ✓ Canary pods immediately terminated
  ✓ Old 3 stable pods continue serving traffic
  ✓ Status: Degraded (RolloutAborted)

Benefits of abort:
  • Zero downtime - old version never interrupted
  • Safe to stop at any point
  • No manual cleanup needed
  • Can retry with different version

To retry:
  ./demo-canary-rollout.sh
EOF
echo ""
