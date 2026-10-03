# Deploy and verify bgtutor on f3s

The current deployment uses the standalone `~/git/bgtutor-mcp` checkout,
release 0.30.0, and `~/git/bgtutor-assets/citizenship-test`. The public connector
URL and existing bearer secret stay the same. See `README.md` for image builds,
content upload and smoke-test commands; see server `CITIZENSHIP.md` for teaching.

## Deploy an update

1. Run server race tests and validate the prepared citizenship library.
2. Bump the server version, tag/push its release and build/push that image.
3. Check f0/f1 CARP state and upload the complete prepared citizenship folder
   to the active NFS storage host. Retain `.nfs-sentinel`, `episodes/`,
   `vocabulary/` and `citizenship-progress/`.
4. Update both deployment images, chart version/appVersion and build tag.
   Run Helm lint/render and `kubectl apply --dry-run=server` on the render.
5. Commit the reviewed workload files. Push conf master to GitHub and Forgejo
   (`git push forgejo refs/heads/master:refs/heads/master`). ArgoCD tracks
   Forgejo master; a manual Deployment patch is reverted by self-healing.
6. Refresh ArgoCD, wait for `Synced`/`Healthy` and one ready app pod. Inspect
   both init containers: NFS sentinel check, then citizenship validation.
7. Run the citizenship smoke test through a local port-forward and public TLS,
   with the existing secret supplied in the environment. Use `--write` once
   to check personal-data writes; its unique vocabulary entry is cleaned up.

Use explicit `kubectl --context=wg0` while roaming. SSH to the FreeBSD hosts
as `paul@f0.wg0` or `paul@f1.wg0`, using `doas` for NFS writes. LAN aliases
also work on the LAN. The NFS host must be CARP master before uploading.

## Checks

```sh
kubectl --context=wg0 -n cicd get application bgtutor
kubectl --context=wg0 -n services get pods -l app=bgtutor
kubectl --context=wg0 -n services get pvc bgtutor-data-pvc
kubectl --context=wg0 -n services logs -l app=bgtutor -c validate-citizenship
kubectl --context=wg0 -n services exec deployment/bgtutor -- /usr/local/bin/bgtutor --version
kubectl --context=wg0 -n services port-forward svc/bgtutor-service 18098:80
```

The distroless container has no shell or tar; direct binary execution works.
Use rsync on the NFS host for assets, rather than `kubectl cp`. Preserve learner
files during updates. The storage declaration is retained and the Deployment
uses `Recreate`, because vocabulary and progress have a single writer.

The production smoke test deliberately avoids saving fake exam results or
mastery. Complete grading and progress persistence are covered by the server
race tests and isolated local MCP sessions. Reconnect or refresh tool discovery
in the AI connector after mode changes.

## Rollback

Revert the workload release commit on conf master and push the revert to
Forgejo. ArgoCD restores the previous image and mode. For the first citizenship
release, this restores `0.1.2` in podcast mode. Do not delete the ArgoCD app or
PVC to roll back. Prepared assets and learner files stay on NFS.

## Troubleshooting

| Symptom | Check |
|---|---|
| Pod Pending | NFS mounted, PV/PVC Bound, Directory path exists |
| nfs-check-data fails | NFS mount and `.nfs-sentinel` |
| validate-citizenship fails | Init logs; validate local assets and re-upload |
| ImagePullBackOff | New tag exists in registry and matches both image fields |
| App rejects listener | Existing secret/key `BGTUTOR_TOKEN` is present |
| Public 502/503 | Deployment readiness, service endpoints, existing frontend route |
| Old tools appear | ArgoCD revision/image/mode, then connector tool refresh |

Do not print tokens or authenticated URLs in reports. Report release version,
ArgoCD/rollout status, content counts, smoke results and any remaining failures.
