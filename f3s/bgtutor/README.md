# bgtutor on f3s

The standalone [bgtutor-mcp](https://github.com/snonux/bgtutor-mcp) server runs
in citizenship mode at `https://bgtutor-mcp.f3s.buetow.org/mcp`.
Release: **0.30.0**. Prepared content comes from
[bgtutor-assets](https://github.com/snonux/bgtutor-assets), `citizenship-test/`.

The connected AI teaches interactive classes using 36 grammar chapters,
reading/listening practice and interview preparation. Six complete practice
and official sample exams have separate answer-key grading. See the server's
`CITIZENSHIP.md` for the teaching plan and all eleven tools. Podcast mode remains
available by starting a separate server with `BGTUTOR_MODE=podcast`.

## Runtime

- Namespace `services`, Deployment `bgtutor`, Service `bgtutor-service`.
- Image `registry.lan.buetow.org:30001/bgtutor:0.30.0`.
- Secret `bgtutor-secret`, key `BGTUTOR_TOKEN`; retain it when updating.
- NFS volume `/data/nfs/k3svolumes/bgtutor/data`, mounted at `/data`.
- Prepared content is in `citizenship-test/`; personal files are
  `vocabulary/saved.json` and `citizenship-progress/saved.json`.
- A sentinel init container checks NFS, followed by `validate-citizenship`,
  which validates prepared tests before the app starts.
- One replica with `Recreate` prevents concurrent personal-data writers.
- ArgoCD app `bgtutor` in `cicd` follows conf `master` on Forgejo.

Requests to `/mcp` require `Authorization: Bearer <token>` or
`?token=<token>`. `/healthz` is public. TLS ends at the f3s frontends;
LAN ingress is `https://bgtutor.f3s.lan.buetow.org/mcp`.
Access logs omit headers, query strings and request bodies.

## Update image and content

Validate the assets with the new binary first. Confirm the CARP master on
f0/f1 before uploading. The commands below assume roaming over WireGuard,
with f0 master; use LAN names and your LAN kubeconfig when on the LAN.

```sh
cd ~/git/bgtutor-mcp
go test -race ./...
go run ./cmd/bgtutor validate --mode citizenship --data-dir ../bgtutor-assets
cd ~/git/conf/f3s/bgtutor
BGTUTOR_REGISTRY=r0.wg0.wan.buetow.org:30001 just build-push
just upload-citizenship /home/paul/git/bgtutor-assets/citizenship-test f0.wg0
```

Builds use rootless Podman and the standalone source checkout. Override
`CONTAINER_ENGINE`, `BGTUTOR_SRC` or `BGTUTOR_REGISTRY` if needed. The upload
copies only prepared content and leaves learner progress, vocabulary and the
NFS sentinel intact. All PDFs, prepared JSON and video/subtitle assets travel
with the folder. Content is reloaded on requests, so content-only updates need
no restart. Uploading without `--delete` also retains existing files: remove
retired prepared files explicitly when needed.

For a new release, change the image tag in `docker-image/Justfile`, both images
in `helm-chart/templates/deployment.yaml`, and `helm-chart/Chart.yaml` together.
Validate with `helm lint`, `helm template` and a Kubernetes server dry run.
Commit only the workload files and push conf master to Forgejo; ArgoCD deploys
it automatically. See `HANDOVER.md` for verification and rollback.

## Verify

```sh
kubectl --context=wg0 -n cicd annotate application bgtutor argocd.argoproj.io/refresh=hard --overwrite
kubectl --context=wg0 -n services rollout status deployment/bgtutor --timeout=120s
kubectl --context=wg0 -n services logs -l app=bgtutor -c validate-citizenship
export BGTUTOR_TOKEN="$(kubectl --context=wg0 -n services get secret bgtutor-secret -o jsonpath='{.data.BGTUTOR_TOKEN}' | base64 -d)"
python3 smoke-test-citizenship.py https://bgtutor-mcp.f3s.buetow.org --write
unset BGTUTOR_TOKEN
```

The smoke test checks authentication, mode/version, all tools, every grammar
chapter, listening assets, all exams, question feedback, incomplete-submission
rejection and the coverage plan. `--write` saves then deletes a unique temporary
vocabulary item. It does not record fake mastery or mock scores. The older
`smoke-test.sh` targets podcast mode and is not appropriate for this deployment.

Refresh the connector's discovered tools after updating from podcast mode.
Keep the existing token; ask the connected AI to start Bulgarian citizenship
preparation. The server's initialization instructions direct its live teaching.
