# Deploy and verify bgtutor on f3s

The current deployment uses the standalone `~/git/bgtutor-mcp` checkout,
release 0.30.0, and `~/git/bgtutor-assets/citizenship-test`. The public connector
URL and existing bearer secret stay the same. See `README.md` for image builds,
content upload and smoke-test commands; see server `CITIZENSHIP.md` for teaching.

## Deploy an update

1. Run server race tests and validate the prepared citizenship library.
2. For an application release, bump/tag the version. For a runtime-only rebuild,
   retain appVersion and use a NEW immutable image tag after test-target and
   nonroot fixture validation; never overwrite an existing release tag.
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

Earth default kubectl now uses the persistent wg2 SSH tunnel; use that context
when wg2 is active. The separate wg0 context still requires an active mesh. SSH to the FreeBSD hosts
as `paul@f0.wg0` or `paul@f1.wg0`, using `doas` for NFS writes. LAN aliases
also work on the LAN. The NFS host must be CARP master before uploading.

## Checks

```sh
kubectl -n cicd get application bgtutor
kubectl -n services get pods -l app=bgtutor
kubectl -n services get pvc bgtutor-data-pvc
kubectl -n services logs -l app=bgtutor -c validate-citizenship
kubectl -n services exec deployment/bgtutor -- /usr/local/bin/bgtutor --version
kubectl -n services port-forward svc/bgtutor-service 18098:80
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
Forgejo. ArgoCD restores the previous image and mode. For the current runtime-only
rebuild, restore the accepted0.30.0 image in citizenship mode. The original
first citizenship migration formerly used0.1.2/podcast; that historical
configuration is not the rollback target for this runtime update. Do not delete the ArgoCD app or
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
