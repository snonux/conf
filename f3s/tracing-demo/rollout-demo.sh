#!/bin/bash
# Quick Argo Rollouts demo script for tracing-demo frontend
# This script automates the demo workflow

set -e

NAMESPACE="services"
ROLLOUT_NAME="tracing-demo-frontend"
KUBE_CTX="$(kubectl config current-context)"

echo "==============================================="
echo "Argo Rollouts Demo for tracing-demo Frontend"
echo "==============================================="
echo ""
echo "Cluster: $KUBE_CTX"
echo "Namespace: $NAMESPACE"
echo "Rollout: $ROLLOUT_NAME"
echo ""

# Check if rollout exists
if ! kubectl get rollout "$ROLLOUT_NAME" -n "$NAMESPACE" &>/dev/null; then
  echo "ERROR: Rollout $ROLLOUT_NAME not found in $NAMESPACE namespace"
  echo "Make sure:"
  echo "  1. Argo Rollouts controller is installed (kubectl get pods -n cicd | grep argo-rollouts)"
  echo "  2. tracing-demo is deployed (kubectl get rollout -n $NAMESPACE)"
  exit 1
fi

# Check if kubectl argo rollouts plugin is installed
if ! kubectl argo rollouts version &>/dev/null; then
  echo "WARNING: kubectl argo rollouts plugin not installed"
  echo "Install it with:"
  echo "  curl -LO https://github.com/argoproj/argo-rollouts/releases/latest/download/kubectl-argo-rollouts-linux-amd64"
  echo "  sudo install -m 755 kubectl-argo-rollouts-linux-amd64 /usr/local/bin/kubectl-argo-rollouts"
  echo ""
fi

echo "Step 1: Display current rollout status"
echo "======================================="
kubectl argo rollouts status "$ROLLOUT_NAME" -n "$NAMESPACE"
echo ""

echo "Step 2: Start watching rollout (Press Ctrl+C to stop)"
echo "======================================================"
echo "This will show real-time rollout progress..."
echo ""
echo "In another terminal, run:"
echo "  kubectl patch rollout $ROLLOUT_NAME -n $NAMESPACE --type='json' -p='[{\"op\":\"replace\",\"path\":\"/spec/template/spec/containers/0/image\",\"value\":\"registry.lan.buetow.org:30001/tracing-demo-frontend:latest\"}]'"
echo ""
echo "Or commit and push a change to git to trigger via ArgoCD"
echo ""

kubectl argo rollouts get rollout "$ROLLOUT_NAME" -n "$NAMESPACE" --watch

echo ""
echo "Demo Complete!"
echo ""
