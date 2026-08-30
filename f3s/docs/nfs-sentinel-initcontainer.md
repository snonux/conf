# NFS Sentinel InitContainer Pattern

This pattern makes f3s NFS-backed `hostPath` PersistentVolumes fail loudly when
a k3s node has lost its `/data/nfs/k3svolumes` mount.

## Problem

Most f3s Helm charts use static PVs backed by `hostPath` directories under
`/data/nfs/k3svolumes`. Many use one child directory per volume:

```text
/data/nfs/k3svolumes/<app>/<volume>
```

Some charts use the app directory itself as the PV root:

```text
/data/nfs/k3svolumes/<app>
```

That path is an NFS mount on r0, r1, and r2. If the NFS mount disappears on a
node, kubelet can still bind-mount the local XFS directory at the same path into
a pod. The workload then starts successfully and writes state to the local
shadow directory. When NFS later returns, the running pod continues to see the
wrong backing directory and state appears empty or lost.

This happened to Wallabag on 2026-05-17. The desired behavior is for the pod to
refuse startup when the bind mount resolves to local XFS instead of NFS.

## Pattern

For every NFS-backed app data directory, create a sentinel file on the NFS
server and require each workload to prove the sentinel exists through the same
PVC it will later use.

The sentinel file:

```text
<actual-pv-root>/.nfs-sentinel
```

Rules:

- The file is empty.
- Mode is `0644`.
- It exists only at the actual NFS-backed PV root.
- It must not be created on the local XFS fallback directory on r0, r1, or r2.
- Add one initContainer per NFS-backed workload volume.
- Mount the same PVC in the initContainer at `/mnt`, read-only.
- The initContainer exits non-zero if `/mnt/.nfs-sentinel` is missing.

Place the sentinel at the directory named by the PV's `hostPath.path`, not at a
derived path. For example, if the PV root is
`/data/nfs/k3svolumes/registry`, the sentinel is
`/data/nfs/k3svolumes/registry/.nfs-sentinel`. Do not add a child volume
directory unless the PV actually points at one.

Example:

```yaml
initContainers:
- name: nfs-check-data
  image: busybox:stable
  command:
  - sh
  - -c
  - |
    test -f /mnt/.nfs-sentinel || (
      echo "ERROR: NFS sentinel missing at /mnt/.nfs-sentinel"
      echo "refusing to start; node likely has NFS unmounted"
      echo "pod would otherwise bind-mount the local-XFS shadow"
      exit 1
    )
  volumeMounts:
  - name: data
    mountPath: /mnt
    readOnly: true
```

For a chart with multiple NFS-backed PVCs, repeat the initContainer with a
unique name and the matching volume name:

```yaml
initContainers:
- name: nfs-check-data
  image: busybox:stable
  command:
  - sh
  - -c
  - |
    test -f /mnt/.nfs-sentinel || (
      echo "ERROR: NFS sentinel missing at /mnt/.nfs-sentinel"
      echo "refusing to start; node likely has NFS unmounted"
      echo "pod would otherwise bind-mount the local-XFS shadow"
      exit 1
    )
  volumeMounts:
  - name: data
    mountPath: /mnt
    readOnly: true
- name: nfs-check-media
  image: busybox:stable
  command:
  - sh
  - -c
  - |
    test -f /mnt/.nfs-sentinel || (
      echo "ERROR: NFS sentinel missing at /mnt/.nfs-sentinel"
      echo "refusing to start; node likely has NFS unmounted"
      echo "pod would otherwise bind-mount the local-XFS shadow"
      exit 1
    )
  volumeMounts:
  - name: media
    mountPath: /mnt
    readOnly: true
```

## Why This Works

Kubernetes mounts the PVC into the initContainer before starting the main
container. If the node's `hostPath` resolves to the local fallback directory, the
sentinel file is absent and the initContainer fails. The pod stays in
`Init:CrashLoopBackOff`, which is visible in normal `kubectl get pods` output.

After NFS is repaired, delete the failed pod so the replacement pod re-runs the
initContainer against the restored mount:

```sh
kubectl delete pod -n <namespace> <pod-name>
```

The existing `nfs-mount-monitor` repairs stale or missing node mounts and
already force-deletes pods stuck in `Unknown`, `Pending`, or
`ContainerCreating`. A later task can extend it to clean up sentinel-blocked
`Init:CrashLoopBackOff` pods after the mount is healthy again.

## Deployment Checklist

For each NFS-backed chart:

1. Identify every `hostPath` PV under `/data/nfs/k3svolumes` and record the
   exact `hostPath.path` value. This exact path is the PV root.
2. Create a sentinel on f0 for every PV root:

   ```sh
   ssh root@f0 '
     touch <actual-pv-root>/.nfs-sentinel &&
     chmod 644 <actual-pv-root>/.nfs-sentinel
   '
   ```

3. Add one `nfs-check-<volume>` initContainer per mounted NFS PVC in the owned
   Deployment or StatefulSet manifest.
4. Commit and push the chart change; ArgoCD will sync it.
5. Identify the workload's actual selector labels from the manifest, usually
   `spec.selector.matchLabels` for Deployments and StatefulSets. Do not assume
   the selector is `app=<chart>`; examples in this repository include
   `app=docker-registry`, `app=jellyfin-server`, and
   `app=koreader-sync-server`, and some charts have multiple workloads.
6. Verify the workload with those actual selector labels:

   ```sh
   kubectl get pod -n <namespace> -l '<selector-labels>'
   kubectl describe pod -n <namespace> <pod-name>
   kubectl logs -n <namespace> <pod-name> -c nfs-check-<volume>
   ```

Expected result: the workload is `Running`, and each `nfs-check-*`
initContainer is `Completed`.

## Scope

Apply the pattern only where this repository owns the workload manifest. Do not
patch upstream Helm chart internals through ArgoCD values unless a separate task
explicitly scopes that work.

Feasible charts:

| Chart | Notes |
| --- | --- |
| `anki-sync-server` | Owned workload manifest. |
| `apache` | Owned workload manifest. |
| `audiobookshelf` | Owned workload manifest. |
| `filebrowser` | Owned workload manifest. |
| `goprecords` | Owned workload manifest. |
| `immich` | Only `helm-chart/templates/postgres.yaml` is owned here. |
| `jellyfin` | Owned workload manifest. |
| `keybr` | Owned workload manifest. |
| `kobo-sync-server` | Owned workload manifest. |
| `miniflux` | Owned workload manifest. |
| `navidrome` | Owned workload manifest. |
| `opodsync` | Owned workload manifest. |
| `pkgrepo` | Owned workload manifest. |
| `player` | Owned workload manifest. |
| `radicale` | Owned workload manifest. |
| `registry` | Owned workload manifest. |
| `syncthing` | Owned workload manifest. |
| `wallabag` | Owned workload manifest. |
| `xplayer` | Owned workload manifest. |

Not feasible in this pattern pass:

| Chart | Reason |
| --- | --- |
| `prometheus` | Upstream chart wrapped by ArgoCD. |
| `loki` | Upstream chart wrapped by ArgoCD. |
| `tempo` | Upstream chart wrapped by ArgoCD. |
| `argocd` | Upstream chart wrapped by ArgoCD. |
| `pihole` | Upstream or historical chart usage; current state is on pi2/pi3. |

## Notes For Existing InitContainers

Older `wait-for-nfs` style checks that mount `/data/nfs/k3svolumes` and wait for
a global marker prove only that the top-level mount path exists. They do not
prove that the specific PVC backing directory mounted into the workload is the
NFS directory rather than a local shadow. Replace those checks with the
per-volume sentinel pattern when migrating a chart.

`strategy.type: Recreate` is still useful for singleton stateful workloads
because it avoids overlapping pod instances on the same `hostPath` data
directory. It does not replace the sentinel check; the two protections cover
different failure modes.
