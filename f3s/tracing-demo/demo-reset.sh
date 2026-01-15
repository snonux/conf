#!/bin/bash
# Reset rollout to clean state

NAMESPACE="services"
ROLLOUT="tracing-demo-frontend"

RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m'

step() { echo -e "\n${BOLD}=== $1 ===${NC}"; }
success() { echo -e "${GREEN}✓${NC} $1"; }

step "Resetting Rollout"
echo ""
echo "This will:"
echo "  1. Abort any in-progress rollout"
echo "  2. Remove env vars added by demo scripts"
echo "  3. Return rollout to clean state"
echo ""

read -p "Reset? (y/n) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    exit 0
fi

step "Step 1: Aborting current rollout"
kubectl argo rollouts abort "$ROLLOUT" -n "$NAMESPACE" 2>/dev/null || true
sleep 2
success "Aborted"

step "Step 2: Removing demo env vars"
# Get the current env vars and reconstruct without demo ones
kubectl get rollout "$ROLLOUT" -n "$NAMESPACE" -o json | \
  jq '.spec.template.spec.containers[0].env |= map(select(.name | test("ROLLOUT_") | not))' | \
  kubectl apply -f - > /dev/null

sleep 2
success "Demo env vars removed"

step "Step 3: Verifying clean state"
echo "Waiting for rollout to stabilize..."
sleep 5

STATUS=$(kubectl argo rollouts status "$ROLLOUT" -n "$NAMESPACE")
echo "$STATUS"
echo ""

step "Reset Complete"
cat << 'EOF'
Rollout is now reset to:
  • Status: Healthy
  • No pending rollouts
  • No demo env vars
  • Ready for next demo

To run demo again:
  ./demo-canary-rollout.sh
EOF
echo ""
