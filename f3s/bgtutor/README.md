# bgtutor on f3s

Bulgarian Podcast Tutor MCP server (`bgtutor serve`, a sub-project of
`github.com/snonux/totalrecall`). A voice AI (Claude or ChatGPT) connects to it
as a custom connector and fetches prepared podcast episodes paragraph by
paragraph.

- Public URL for the connector: `https://bgtutor.f3s.buetow.org/mcp`
  (TLS on the frontends via relayd, `bgtutor.f3s.buetow.org` is in
  `@f3s_hosts` in `frontends/Rexfile`)
- LAN: `https://bgtutor.f3s.lan.buetow.org/mcp`
- Every `/mcp` request needs the bearer token (`Authorization: Bearer <token>`
  or `?token=<token>` on the URL). `/healthz` is open for probes.

## Initial setup

### 1. Storage

On the current CARP storage master (check with `ifconfig | grep MASTER` on f0/f1):

```sh
doas mkdir -p /data/nfs/k3svolumes/bgtutor/data/episodes /data/nfs/k3svolumes/bgtutor/data/vocabulary
doas touch /data/nfs/k3svolumes/bgtutor/data/.nfs-sentinel
doas chmod 0644 /data/nfs/k3svolumes/bgtutor/data/.nfs-sentinel
```

The PV uses `type: Directory`, so the pod will not schedule until it exists.

### 2. Token secret

```sh
kubectl create secret generic bgtutor-secret -n services \
  --from-literal=BGTUTOR_TOKEN="$(openssl rand -hex 32)"
kubectl get secret bgtutor-secret -n services -o jsonpath='{.data.BGTUTOR_TOKEN}' | base64 -d
```

### 3. Image

```sh
cd /home/paul/git/conf/f3s/bgtutor
just build-push   # builds /home/paul/git/totalrecall/bgtutor/Dockerfile
```

Pushed as `r0.lan.buetow.org:30001/bgtutor:0.1.0`, pulled as
`registry.lan.buetow.org:30001/bgtutor:0.1.0`. Bump the tag in
`docker-image/Justfile` and `helm-chart/templates/deployment.yaml` together.

### 4. Deploy

```sh
kubectl apply -f ../argocd-apps/services/bgtutor.yaml
just status
```

Then re-run rex for the frontends so `bgtutor.f3s.buetow.org` gets its DNS
record, certificate and relayd keypair.

## Adding episodes

Prepare an episode in the totalrecall checkout with a coding agent (Claude
Code, Codex, or a Claude cloud session) following `bgtutor/PREPARE.md`; no API
key is needed. Then check it, publish it and copy it into the library:

```sh
cd /home/paul/git/totalrecall
go run ./cmd/bgtutor validate 002-morning-news
go run ./cmd/bgtutor publish 002-morning-news
cd /home/paul/git/conf/f3s/bgtutor
just upload-episode /home/paul/git/totalrecall/bgtutor/data/episodes/002-morning-news
BGTUTOR_TOKEN=... just episodes
```

The server re-scans the library on every request, so no restart is needed.
The learner's vocabulary notebook lives at `vocabulary/saved.json` on the same
volume.

## Connecting a voice AI

- Claude: Settings > Connectors > Add custom connector, URL
  `https://bgtutor.f3s.buetow.org/mcp?token=<token>`.
- ChatGPT: enable Developer Mode, then add a connector with the same URL.
