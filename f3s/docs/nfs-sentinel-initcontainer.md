# NFS sentinel initContainer

Most f3s charts use static `hostPath` PVs under `/data/nfs/k3svolumes`
(`<app>/<volume>` or just `<app>`), an NFS mount on r0/r1/r2. If a node loses
the mount, kubelet happily bind-mounts the local XFS directory underneath and
the app starts with empty state (Wallabag, 2026-05-17). The sentinel makes the
pod refuse to start instead.

## Rules

- Empty file `.nfs-sentinel`, mode 0644, at the exact `hostPath.path` of each
  PV (not a derived child dir).
- Create it on the NFS server only, never in the local XFS fallback on r0-r2.
- One initContainer per NFS-backed volume, mounting the same PVC read-only at
  `/mnt`, failing when `/mnt/.nfs-sentinel` is missing.
- Image `busybox:1.36.1` everywhere (the digest `busybox:stable` resolved to
  on 2026-09-25, sha256:73aaf090...). Never float `stable`/`latest`; bump all
  charts together.

```yaml
initContainers:
- name: nfs-check-data          # one per volume: nfs-check-<volume>
  image: busybox:1.36.1   # pinned; bump deliberately across all charts
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

A missing mount shows as `Init:CrashLoopBackOff`. After NFS is back, delete
the pod so the replacement re-checks. `nfs-mount-monitor` (gonf
`rnodes_nfs_mount_monitor`) repairs mounts and force-deletes pods stuck in
`Unknown`/`Pending`/`ContainerCreating`, but not sentinel-blocked ones.

## Adding it to a chart

1. List every `hostPath` PV and its exact path.
2. `ssh root@f0 'touch <pv-root>/.nfs-sentinel && chmod 644 <pv-root>/.nfs-sentinel'`
3. Add an `nfs-check-<volume>` initContainer per NFS PVC.
4. Push; ArgoCD syncs.
5. Verify with the workload's real selector (from `spec.selector.matchLabels`;
   e.g. `app=docker-registry`, `app=jellyfin-server`,
   `app=koreader-sync-server`, not always `app=<chart>`):

   ```sh
   kubectl get pod -n <ns> -l '<selector>'
   kubectl logs -n <ns> <pod> -c nfs-check-<volume>
   ```

   Expect `Running` and every `nfs-check-*` `Completed`.

## Scope

Only for workload manifests this repo owns: anki-sync-server, apache,
audiobookshelf, filebrowser, forgejo, goprecords, immich (only
`templates/postgres.yaml`), jellyfin, keybr, kobo-sync-server, miniflux,
navidrome, opodsync, pkgrepo, player, radicale, registry, syncthing,
wallabag, xplayer.

Not covered: upstream charts wrapped by ArgoCD (prometheus, loki, tempo,
argocd) and pihole (runs on pi2/pi3).

Old `wait-for-nfs` checks that only test the top-level
`/data/nfs/k3svolumes` mount don't prove the PVC directory is on NFS; replace
them. `strategy.type: Recreate` on singletons is still needed (prevents two
pods on one hostPath) and is a separate protection.
