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

Radicale 3.8.3 is built from `../docker-image` (Alpine 3.24.2, UID 1000).
The image contains configuration only; the existing PVC supplies `/auth/htpasswd`.

```sh
podman build -t r0.lan.buetow.org:30001/radicale:3.8.3 ../docker-image
podman push --tls-verify=false r0.lan.buetow.org:30001/radicale:3.8.3
```
