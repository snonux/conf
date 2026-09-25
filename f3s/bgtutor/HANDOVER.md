# Handover: deploy and verify bgtutor on f3s

For an agent (or person) with access to Paul's workstation: the `~/git`
checkouts, Docker, `kubectl`/`argocd` for the f3s cluster, SSH to the NFS
server (`paul@f0` with `doas`; the f-hosts take no root logins) and the gonf
controller setup for the frontends. The first deployment followed this file on
2026-09-25; the steps below carry the corrections from that run. Work through
them in order; each has a check. Stop and report if a check fails. Don't
improvise around it.

The kubeconfig talks to `r0.wg0.wan.buetow.org:6443`. On the LAN with `wg0`
down that address has no route; point kubectl at the LAN name instead, e.g.
`alias kubectl='kubectl --server https://r0.lan.buetow.org:6443'`. Bare `f0`
resolves to port 22 in `~/.ssh/config`; `f0.lan.buetow.org` does not (all of
`*.buetow.org` maps to port 2, the OpenBSD frontends), so use `f0` or pass
`-p 22`.

## What bgtutor is

An MCP server (Go, Streamable HTTP, stateless) that serves prepared English
podcast episodes to a voice AI (Claude or ChatGPT). The AI fetches one
paragraph at a time, translates it into Bulgarian live and saves unknown words
to a notebook. Endpoints:

| Path | Auth | Purpose |
|---|---|---|
| `/mcp` | bearer token required | MCP JSON-RPC (POST; GET is 401 without token) |
| `/healthz` | none | liveness/readiness, returns `ok` |
| anything else | none | 404 |

Tools: `list_episodes`, `get_paragraph`, `save_vocabulary`, `list_vocabulary`.

### Security model (verify, don't assume)

- **Authentication:** one shared secret, `BGTUTOR_TOKEN`, from the Kubernetes
  secret `bgtutor-secret`. Accepted as `Authorization: Bearer <token>`
  (scheme case-insensitive) or as `?token=<token>` on the URL, because
  connector UIs often only take a URL. Compared in constant time. A missing
  or wrong token gets `401` with `WWW-Authenticate: Bearer`. The server
  refuses to start on a non-localhost address without a token.
- **Authorization:** there is none beyond the token. It's a single-user
  service: the token grants full access, including writes to the vocabulary
  notebook. There are no scopes or users. That's intended for v1.
- **Transport:** TLS ends at relayd on the OpenBSD frontends (blowfish,
  fishfinger). Traffic from there to traefik and the pod is plain HTTP inside
  the WireGuard mesh, the same as every other f3s service.
- **Known trade-offs to confirm are acceptable:** the `?token=` form can end
  up in access logs (relayd, traefik). The SDK's DNS-rebinding check is off
  whenever a token is set, because the token already covers that threat.
  There is no rate limiting; a 256-bit random token makes brute force
  pointless.
- **Episode ids** must match `^[a-z0-9][a-z0-9-]{0,79}$`, so path traversal
  through `episode_id` is rejected.

## Source

| Repo | PR / branch | Contents |
|---|---|---|
| github.com/snonux/totalrecall | #1, `bgtutor-mcp-server` | server code (`cmd/bgtutor`, `internal/bgtutor`), `bgtutor/Dockerfile`, test episode `bgtutor/data/episodes/001-cooking-basics` |
| github.com/snonux/conf | #1, `bgtutor-chart` | `f3s/bgtutor/` (chart, Justfiles, README, this file, `smoke-test.sh`), `f3s/argocd-apps/services/bgtutor.yaml`, `bgtutor-mcp.f3s.buetow.org` in `gonf/frontends/data.go` |

Merge totalrecall#1 first; the image is built from it. If Paul hasn't merged
yet, build from the `bgtutor-mcp-server` branch and deploy from the
`bgtutor-chart` branch only after asking him. ArgoCD tracks `master` of the
in-cluster Forgejo copy of conf, so conf#1 has to be merged and pushed to
Forgejo before ArgoCD sees the chart.

Fixed names used below: namespace `services`, Deployment and label
`app=bgtutor`, Service `bgtutor-service`, PVC `bgtutor-data-pvc`, PV
`bgtutor-data-pv`, secret `bgtutor-secret` (key `BGTUTOR_TOKEN`), ArgoCD app
`bgtutor` in namespace `cicd`, image
`registry.lan.buetow.org:30001/bgtutor:0.1.0` (pushed via
`r0.lan.buetow.org:30001`), NFS path `/data/nfs/k3svolumes/bgtutor/data`.

## 1. Storage on NFS

```sh
ssh paul@f0 'doas mkdir -p /data/nfs/k3svolumes/bgtutor/data/episodes /data/nfs/k3svolumes/bgtutor/data/vocabulary \
  && doas touch /data/nfs/k3svolumes/bgtutor/data/.nfs-sentinel \
  && doas chmod 644 /data/nfs/k3svolumes/bgtutor/data/.nfs-sentinel'
```

Seed the test episode so there is something to serve:

```sh
cd ~/git/totalrecall && git checkout main && git pull   # or the PR branch
go run ./cmd/bgtutor validate 001-cooking-basics          # expect "ready"
just -f ~/git/conf/f3s/bgtutor/Justfile upload-episode ~/git/totalrecall/bgtutor/data/episodes/001-cooking-basics
```

Check: `ssh paul@f0 'doas ls -la /data/nfs/k3svolumes/bgtutor/data /data/nfs/k3svolumes/bgtutor/data/episodes/001-cooking-basics'`
shows `.nfs-sentinel` (0644), `episodes/`, `vocabulary/`, `meta.json` and
`paragraphs.json`. The pod runs as root, so root-owned directories are fine.
If `f0` isn't the current NFS server (the CARP master, `ifconfig | grep
MASTER`), use whichever host `f3s/docs/nfs-sentinel-initcontainer.md` names
and pass it as the second argument of `upload-episode`.

## 2. Token secret

```sh
kubectl create secret generic bgtutor-secret -n services \
  --from-literal=BGTUTOR_TOKEN="$(openssl rand -hex 32)"
```

Check: `kubectl get secret bgtutor-secret -n services -o jsonpath='{.data.BGTUTOR_TOKEN}' | base64 -d | wc -c`
prints `64`. Never commit, log or paste the token anywhere except to Paul
directly. Store it in his vault if he has an entry convention (ask him).

## 3. Image

```sh
cd ~/git/conf/f3s/bgtutor
just build-push      # docker build -f ~/git/totalrecall/bgtutor/Dockerfile ... && push
```

Check: `curl -s http://r0.lan.buetow.org:30001/v2/bgtutor/tags/list` lists
`0.1.0` (use https or the registry's usual access if plain http is refused).
A local run should refuse to start without a token and serve with one:

```sh
docker run --rm bgtutor:0.1.0 ; echo "exit=$?"          # expect: refusing to listen ... exit=1
# A real-length token: with a short one like "test" the "token prefix only"
# check fails, because the first 8 characters are the whole token.
export BGTUTOR_TOKEN="$(openssl rand -hex 32)"
docker run --rm -d --name bgt -e BGTUTOR_TOKEN -p 18080:8080 \
  -v ~/git/totalrecall/bgtutor/data:/data bgtutor:0.1.0
./smoke-test.sh http://127.0.0.1:18080   # expect all ok
docker rm -f bgt
```

## 4. Deploy with ArgoCD

After conf#1 is merged, push `master` to the Forgejo remote the usual way
(`f3s/argocd-apps/README.md`: "push to Forgejo, then apply the changed
manifest"), then:

```sh
cd ~/git/conf
# Plain "master" is ambiguous here: there is also a remote named master.
git push forgejo refs/heads/master:refs/heads/master
kubectl apply -f f3s/argocd-apps/services/bgtutor.yaml
cd f3s/bgtutor && just sync && just status
```

Checks:
- `kubectl get application bgtutor -n cicd` shows `Synced` and `Healthy`.
- `kubectl get pod -n services -l app=bgtutor` shows `Running`, and
  `kubectl logs -n services -l app=bgtutor -c nfs-check-data` is empty (init
  container `Completed`). `Init:CrashLoopBackOff` means the sentinel is
  missing or NFS isn't mounted on that node.
- `kubectl logs -n services -l app=bgtutor -c bgtutor` shows
  `serving /data at http://:8080/mcp (auth: bearer token)`.
- `kubectl get pvc bgtutor-data-pvc -n services` is `Bound` to `bgtutor-data-pv`.

## 5. Test inside the LAN

```sh
export BGTUTOR_TOKEN="$(kubectl get secret bgtutor-secret -n services -o jsonpath='{.data.BGTUTOR_TOKEN}' | base64 -d)"
cd ~/git/conf/f3s/bgtutor
just port-forward 18081 &                                  # svc/bgtutor-service -> localhost:18081
./smoke-test.sh http://127.0.0.1:18081 --write
./smoke-test.sh https://bgtutor.f3s.lan.buetow.org
```

Both must end with `0 failed`. Don't forward to 8080: a local `player`
process usually holds it, the port-forward then dies silently and the smoke
test hits the wrong server (401s without `WWW-Authenticate`). `--write` adds
one entry, `smoketest-ябълка`, to the real notebook. Tell Paul, or remove it by editing
`vocabulary/saved.json` on the NFS server while the pod is scaled to 0.

## 6. Public HTTPS on the frontends

`bgtutor-mcp.f3s.buetow.org` is in `f3sHosts` in `gonf/frontends/data.go` (with
expected HTTPS check status `HTTP/1.1 404`, since `/` has no page). Converge
the frontends from the conf root, following `frontends/README.md`:

```sh
./gonf.sh cluster frontends frontends_nsd           # DNS record
./gonf.sh cluster frontends frontends_httpd         # ACME challenge + cluster-down fallback
./gonf.sh cluster frontends frontends_acme          # ACME config for the new name
./gonf.sh cluster frontends frontends_acme_invoke   # request the certificate (by name only)
./gonf.sh cluster frontends frontends_relayd        # route the host with its keypair
./gonf.sh cluster frontends frontends_gogios        # HTTPS monitoring for the new site
```

Preview first with `./gonf.sh push -n -- -p 2 rex@blowfish.buetow.org <task>`
if unsure. Running the whole `frontends` aggregate plus `frontends_acme_invoke`
and then `frontends_relayd` again does the same job.

Checks:
- `dig +short bgtutor-mcp.f3s.buetow.org` resolves like `goprecords.f3s.buetow.org`.
- `curl -sv https://bgtutor-mcp.f3s.buetow.org/healthz` returns `ok` with a
  valid Let's Encrypt certificate for `bgtutor-mcp.f3s.buetow.org` (no `-k`).
- Check both frontends if DNS points at both (`--resolve` with each IP).
  If it points only at blowfish (like `goprecords`), fishfinger serving a
  `foo.zone` certificate for `bgtutor-mcp.f3s.buetow.org` is expected: `acme.sh`
  requests a site certificate only on the frontend the name resolves to and
  copies a placeholder in elsewhere so relayd can load the keypair.
  `standby.bgtutor-mcp.f3s.buetow.org` gets a real certificate there (and a 404
  from traefik, same as `standby.goprecords`).
- `frontends_gogios` always previews `pkg_add -u gogios` (the package is
  `IsLatest`); it upgrades only if pkgrepo has a newer build.

## 7. Test over the internet

```sh
./smoke-test.sh https://bgtutor-mcp.f3s.buetow.org
```

It must end with `0 failed`. These are the authentication checks it runs;
all of them must pass through relayd and traefik, not just locally:

| Request | Expected |
|---|---|
| `GET /healthz`, no token | 200 |
| POST `/mcp`, no token / wrong token / empty `Bearer ` / token only as basic auth / wrong `?token=` / first 8 chars of the token | 401 |
| GET `/mcp`, no token | 401 |
| POST `/mcp` with `Authorization: Bearer <token>`, with `bearer` lowercase, or `?token=<token>` | 200 |
| any 401 | header `WWW-Authenticate: Bearer` |

Extra checks the script doesn't do:
- `curl -s -o /dev/null -w '%{http_code}\n' http://bgtutor-mcp.f3s.buetow.org/mcp`:
  plain HTTP must not serve MCP with a token (expect a redirect, the frontends'
  default for port 80, or 401; never 200).
- After the test, grep the relayd and traefik access logs for the token value.
  If the `?token=` requests show up there, tell Paul, so he can decide between
  header-only clients or log scrubbing. Pull the logs back and grep locally
  (`ssh … 'doas cat …' | grep -c -F "$BGTUTOR_TOKEN"`) so the token never
  lands on a remote command line. As of 2026-09-25 there were no matches:
  relayd (443) logs no request URLs and traefik's access log is off. The one
  URL log is httpd's port-80 access log on the frontends, which keeps query
  strings, so a client using `http://…/mcp?token=…` would leave the token
  there before the 302 to https.
- `kubectl exec` is not possible (distroless, no shell). That's expected.

## 8. Real session with a voice AI

- Claude: Settings > Connectors > Add custom connector, URL
  `https://bgtutor-mcp.f3s.buetow.org/mcp?token=<token>` (Paul adds it; the
  token stays with him).
- ChatGPT: turn on Developer mode (in Settings, under the apps/connectors
  advanced options; needs a paid plan), then add a connector with the same URL.

Ask the assistant to list the episodes and play `001-cooking-basics`. It
should read paragraph 1 in Bulgarian, go on only when asked, stop at
paragraph 9 (`is_last`), and save a word when asked. Afterwards
`./smoke-test.sh` still passes and `list_vocabulary` shows the saved word.

## Rollback

```sh
kubectl delete -f ~/git/conf/f3s/argocd-apps/services/bgtutor.yaml   # finalizer prunes the app
```

The PV has `Retain`, so episodes and the notebook stay on NFS. To take the
public host offline, remove it from `f3sHosts` and re-run
`frontends_relayd`, `frontends_nsd` and `frontends_gogios`.

## Troubleshooting

| Symptom | Likely cause |
|---|---|
| Pod `Pending`, PV not bound | `/data/nfs/k3svolumes/bgtutor/data` missing (PV is `type: Directory`) |
| `Init:CrashLoopBackOff` | `.nfs-sentinel` missing, or NFS unmounted on the node |
| Container exits `refusing to listen on :8080 without BGTUTOR_TOKEN` | secret missing or key name not `BGTUTOR_TOKEN` |
| `ImagePullBackOff` | image not pushed, or tag differs between Justfile and deployment |
| Public URL 502/503 | relayd not reloaded with the new host, or pod not ready |
| Public URL shows the "cluster down" page | relayd has no route yet; re-run `frontends_relayd` |
| `list_episodes` shows `ready: false` | read `not_ready_reason`; run `bgtutor validate <id>` on the folder |

## Report back to Paul

Keep it short: what was done, the smoke-test summary lines (LAN and public),
the ArgoCD status, whether the token shows up in access logs, and anything
you skipped or that failed. Don't include the token.
