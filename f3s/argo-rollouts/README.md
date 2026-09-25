# Argo Rollouts

Progressive delivery controller (canary, blue-green), namespace `cicd` next
to ArgoCD, single instance. ArgoCD app `argocd-apps/cicd/argo-rollouts.yaml`;
`just install | upgrade | uninstall | status | logs` for manual Helm use.

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
