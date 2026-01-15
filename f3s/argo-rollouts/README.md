# Argo Rollouts Deployment for f3s Cluster

Argo Rollouts is a Kubernetes controller for progressive delivery strategies including canary, blue-green, and A/B testing deployments.

## Overview

- **Namespace**: `cicd` (alongside ArgoCD)
- **Deployment Mode**: Single instance
- **CRD**: Rollout custom resource for progressive deployments
- **Integration**: Works with ArgoCD for GitOps-based rollouts

## Installation

```bash
just install
```

## Verification

```bash
just status
```

Check that the rollouts controller pod is running:
```bash
kubectl get pods -n cicd -l app.kubernetes.io/name=argo-rollouts
```

Check CRD is installed:
```bash
kubectl get crd | grep rollout
```

## Demo: Tracing-Demo Frontend Rollout

The frontend service uses a Canary strategy:

1. Deploy new version
2. Send 50% traffic to new version
3. Monitor for 2 minutes
4. If successful, shift 100% traffic
5. If failures detected, rollback

### Watch Rollout Progress

```bash
# Real-time status
kubectl argo rollouts get rollouts tracing-demo-frontend -n services --watch

# Full rollout status
kubectl argo rollouts status tracing-demo-frontend -n services

# Describe rollout details
kubectl describe rollout tracing-demo-frontend -n services
```

### Trigger a New Rollout

Update the frontend image tag in git (or use kubectl):

```bash
# Patch to trigger new rollout
kubectl patch rollout tracing-demo-frontend -n services \
  --type json -p='[{"op": "replace", "path": "/spec/template/spec/containers/0/image", "value":"registry.lan.buetow.org:30001/tracing-demo-frontend:v2"}]'
```

Or via git commit and ArgoCD sync.

### Manual Promotion (Skip Canary Steps)

```bash
kubectl argo rollouts promote tracing-demo-frontend -n services
```

### Abort/Rollback

```bash
kubectl argo rollouts abort tracing-demo-frontend -n services
```

## References

- [Argo Rollouts Documentation](https://argoproj.github.io/argo-rollouts/)
- [Canary Strategy Guide](https://argoproj.github.io/argo-rollouts/features/canary/)
- [ArgoCD Integration](https://argoproj.github.io/argo-rollouts/generated/notification-services/argocd/)
