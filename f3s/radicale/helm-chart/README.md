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
