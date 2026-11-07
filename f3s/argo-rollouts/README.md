# Argo Rollouts

Progressive delivery controller (canary, blue-green), namespace `cicd` next
to ArgoCD, single instance. ArgoCD app `argocd-apps/cicd/argo-rollouts.yaml`;
`just install | upgrade | uninstall | status | logs` for manual Helm use.

ArgoCD owns the active deployment. On 2026-11-07 its upstream chart was upgraded
from 2.43.8 to 2.43.9 (controller v1.10.2); the Application is Synced/Healthy and
the controller is Ready. The older Helm release record is retained and does not
describe the current ArgoCD-managed version. Use the Application for upgrades.

```sh
kubectl get pods -n cicd -l app.kubernetes.io/name=argo-rollouts
kubectl get crd | grep rollout
```

The only user is the tracing-demo frontend `Rollout` (see
[`../tracing-demo/README.md`](../tracing-demo/README.md)). Needs the
`kubectl-argo-rollouts` plugin:

```sh
kubectl argo rollouts get rollout tracing-demo-frontend -n services --watch
kubectl argo rollouts status  tracing-demo-frontend -n services
kubectl argo rollouts promote tracing-demo-frontend -n services
kubectl argo rollouts abort   tracing-demo-frontend -n services
kubectl patch rollout tracing-demo-frontend -n services --type json \
  -p='[{"op": "replace", "path": "/spec/template/spec/containers/0/image", "value":"registry.lan.buetow.org:30001/tracing-demo-frontend:v2"}]'
```
