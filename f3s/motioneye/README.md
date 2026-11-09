# motionEye

Camera UI: https://eye.f3s.buetow.org (LAN: https://eye.f3s.lan.buetow.org).
Namespace `services`; ArgoCD application and deployment `motioneye`.

Runs the upstream motionEye 0.44.0 container pinned by digest. motionEyeOS
is an appliance OS and is not used inside Kubernetes. No cameras are configured.

## Login

Username `admin`. The generated password is held in the Kubernetes Secret
`services/motioneye-admin`, outside Git:

```sh
kubectl get secret motioneye-admin -n services -o jsonpath='{.data.password}' | base64 -d; echo
```

Both administrator and viewer access require authentication. The initial
viewer username is `viewer` with the same initial password; change or disable
it in the UI as desired. Changing a password in motionEye changes the persisted
configuration, not the bootstrap Secret; update the Secret separately if keeping
it as the credential record. Do not delete the persisted configuration to reset
a password: restore it from backup or update its password hash while stopped.

## Verified upgrade, 2026-11-08

Version 0.44.0 changes API authentication to session cookies: POST username
and password to `/login`, then send the returned cookie with API requests.
Legacy signature authentication and HTTP Basic authentication were removed.
Existing administrator and viewer credentials passed login before and after
a Recreate restart. Both stored passwords migrated to Argon2; no password
reset was performed. Wrong credentials return 401 and unauthenticated
configuration access is denied. The camera list remains empty, so capture,
recording and playback await cameras.

The release image is pinned by digest and its running immutable config ID
was checked against the staged image. A quiesced configuration archive is
stored privately under `~/.local/state/conf/j34-app-upgrades-20261107/`.
For rollback, stop the writer and restore that configuration with the old
image: 0.43.1 cannot use the new Argon2 credentials/session configuration.
Automatic sync was held until this matching chart was published; it was
restored (`prune: true, selfHeal: true`) on 2026-11-09 (task e54), after
Argo was Synced at the published commit 1fc3dfa.

## Storage and operation

Retained PVs bind explicitly to the config (1Gi) and media (100Gi) claims.
Both use `/data/nfs/k3svolumes/motioneye/{config,media}` on shared NFS.
The declared sizes are Kubernetes accounting, not filesystem quotas. There is
no camera retention policy yet: choose retention and recording quality when
adding cameras to control actual disk usage.

The initial deployment created these directories on a verified NFS mount,
mode 0700 for config and 0755 for media, with `.nfs-sentinel` in each.
`config/motion.conf` was initially seeded with legacy password hashes
(the upstream configuration format). The init container refuses to start if
storage sentinels or seeded authentication are missing; the liveness probe
checks both sentinels. The Recreate strategy keeps one writer across updates.

The upstream entrypoint starts as root and switches to its motion user (UID 0
in this image); no host devices, host networking or privileged mode are used.
The pod has no service-account token. NetworkPolicy admits only Traefik to
TCP8765; application egress is unrestricted, including future LAN camera streams.
Only the web UI is exposed; motion's control and raw streaming ports stay private.

```sh
kubectl get pods -n services -l app=motioneye
kubectl get pvc -n services motioneye-config-pvc motioneye-media-pvc
kubectl logs -n services deployment/motioneye
kubectl port-forward -n services service/motioneye 8765:8765
kubectl annotate application motioneye -n cicd argocd.argoproj.io/refresh=hard --overwrite
```

Deploy changes by pushing this chart to Forgejo master and applying
`f3s/argocd-apps/services/motioneye.yaml`. DNS, ACME and public TLS routing
come from the `eye.f3s.buetow.org` entry in `gonf/frontends/data.go`.

## Adding Reolink cameras

Enable RTSP on each camera, assign a stable LAN address, and create a dedicated
camera account. Add it in motionEye as a Network Camera using its RTSP URL and
credentials. A common Reolink main stream URL is
`rtsp://CAMERA-IP:554/Preview_01_main`; `_sub` selects the lower bandwidth
stream. Availability and exact paths depend on the camera model; check its
Reolink documentation. Prefer H.264 for compatibility. No credentials or camera
URLs belong in Git. Check network reachability, live video, a short recording,
and playback once the cameras arrive; those cannot be validated without cameras.

Sources: [upstream container](https://github.com/motioneye-project/motioneye/pkgs/container/motioneye),
[Reolink RTSP](https://support.reolink.com/hc/en-us/articles/900000630706-Introduction-to-RTSP/).
