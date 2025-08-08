# FreshRSS Helm Chart

This chart deploys FreshRSS using a single Deployment, Service, Ingress, and a hostPath-backed PersistentVolume/PersistentVolumeClaim for data.

## Prerequisites

Before installing the chart, you must manually create the hostPath directory used by the PersistentVolume (see `templates/persistent-volumes.yaml`):

- `/data/nfs/k3svolumes/freshrss/data`

Example commands:

```bash
sudo mkdir -p /data/nfs/k3svolumes/freshrss/data
# Ensure write permissions for the runtime user/group (nobody:nogroup = 65534:65534)
sudo chown -R 65534:65534 /data/nfs/k3svolumes/freshrss/data
```

## Installing the Chart

To install the chart with the release name `freshrss`, run:

```bash
helm install freshrss . --namespace services --create-namespace
```

## Access

- Ingress host: `freshrss.f3s.lan.buetow.org`
