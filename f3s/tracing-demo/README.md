# Tracing demo

Three Flask services instrumented with OpenTelemetry, for Tempo and Argo
Rollouts demos. `tracing-demo.f3s.buetow.org` (LAN:
`tracing-demo.f3s.lan.buetow.org`), namespace `services`. The ArgoCD app is
disabled (`argocd-apps/services/tracing-demo.yaml.disabled`), as is Tempo.

```
client -> frontend :5000 -> middleware :5001 -> backend :5002
                     \_______________ OTLP gRPC -> Alloy :4317 -> Tempo
```

| Service | Endpoints |
|---|---|
| frontend | `/`, `/health`, `GET|POST /api/process` |
| middleware | `/`, `/health`, `POST /api/transform` |
| backend | `/`, `/health`, `GET /api/data` (fake DB) |

Auto-instrumented Flask and Requests plus manual spans; resource attributes
`service.name`, `service.namespace=tracing-demo`, version. Code in
`docker/{frontend,middleware,backend}/app.py`.

The frontend is an Argo `Rollout`: canary 33%, 1 minute pause, 100%.

## Commands

```sh
just build-push           # docker-image-Justfile -> r0.lan.buetow.org:30001/tracing-demo-*:latest
just sync | argocd-status | status | restart
just logs | logs-frontend | logs-middleware | logs-backend
just test | load-test | check-traces
just port-forward-frontend | port-forward-middleware | port-forward-backend
just rollout-watch | rollout-status | rollout-info | rollout-promote | rollout-abort | rollout-history
just demo-canary | demo-abort | demo-reset | demo-menu | rollout-demo
```

TraceQL: `{ resource.service.namespace = "tracing-demo" }`,
`{ duration > 200ms }`, `{ status = error }`.

No traces: check the pods, then
`kubectl logs -n monitoring -l app.kubernetes.io/name=alloy | grep -i otlp`
and the Tempo logs.
