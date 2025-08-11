# Radicale Helm Chart

This chart deploys a gpodder sync server using Radicale.

## Prerequisites

Before installing the chart, you must manually create the following directories on your host system to be used by the persistent volumes:

- `/data/nfs/k3svolumes/radicale/collections`
- `/data/nfs/k3svolumes/radicale/auth`

## Installing the Chart

To install the chart with the release name `radicale`, run the following command:

```bash
helm install radicale . --namespace services --create-namespace
```
