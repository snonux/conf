# Radicale Helm Chart

This chart deploys Radicale (CalDAV/CardDAV), `radicale.f3s.buetow.org`.

## Prerequisites

Before installing the chart, you must manually create the following directories on your host system to be used by the persistent volumes:

- `/data/nfs/k3svolumes/radicale/collections`
- `/data/nfs/k3svolumes/radicale/auth`

## Installing the Chart

Manual install (normally ArgoCD deploys it):

```bash
helm install radicale . --namespace services --create-namespace
```

## Image

Radicale 3.9.0 is built from `../docker-image` (Alpine 3.24.4, UID 1000).
The image contains configuration only; the existing PVC supplies `/auth/htpasswd`.

```sh
podman build -t r0.lan.buetow.org:30001/radicale:3.9.0 ../docker-image
podman push --tls-verify=false r0.lan.buetow.org:30001/radicale:3.9.0
```

## Verified upgrade, 2026-11-08

The tested local image, published registry image and running CRI config ID
match. A quiesced auth/collections archive was downloaded and SHA-256 checked
before startup (`~/.local/state/conf/j34-app-upgrades-20261107/`). All 669
existing content objects retain their hashes.

A unique temporary DAV account verified MKCALENDAR, PUT/GET, PROPFIND,
a time-range REPORT and deletion. Anonymous and wrong-password requests were
rejected. Its account, event, calendar and owned user directory were removed;
the original authentication file bytes were restored. Existing credentials
and calendars were not reset.

Automatic sync was held during the 2026-11-08 upgrade until this chart was
published; e54 restored prune+selfHeal on 2026-11-09 after verifying Git equals
live. Restore the
previous image and selected backup directories with the writer stopped if
rollback is needed; never roll back the shared NFS dataset for one app.
