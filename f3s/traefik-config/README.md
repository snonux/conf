# Traefik configuration

Argo application `traefik-config` owns the `kube-system/traefik`
HelmChartConfig and its forwarding/header middlewares. K3s owns the bundled
Traefik chart and Helm release; change this chart through GitOps.

The image override selects Traefik 3.7.17 by its immutable publisher digest
(task 244, 2026-11-08). The existing chart remains bundled with k3s; no CRD,
entrypoint, service, port, authentication or NetworkPolicy change is needed.

Only the TLS-terminating OpenBSD frontends at `192.168.2.110/32` and
`192.168.2.111/32` are trusted to supply forwarded headers on `web`.
LAN `websecure` trusts no client-supplied forwarded headers. Both entrypoints
delete separator aliases and the RFC `Forwarded` header. No PROXY protocol
parser is enabled. Do not broaden these trusted source ranges.

`web` keeps NodePort 31637 and `externalTrafficPolicy: Local`; frontend
relayd health checks choose nodes with ready local endpoints. TLS and
Prometheus configuration remain in the same HelmChartConfig.

Validate this configuration with `helm lint helm-chart`, render with
`helm template traefik-config helm-chart`, and server dry-run the result.
Before publishing an image update, pull its immutable reference on all three
nodes and verify the actual bundled chart renders only the intended change.
Argo applies HelmChartConfig; k3s's Helm controller performs the rollout.

Verify both public and LAN HTTPS routes, protected-route rejection, forged
forwarded-header rejection, TLS, metrics and unchanged service/route policy.
For rollback, restore the previous image pin through GitOps; do not rewrite
k3s's generated `traefik.yaml` or roll back application data.

Private 244 backup and acceptance receipts:
`~/.local/state/conf/244-traefik-20261108/` on Earth.
