# Wallabag Helm Chart

This chart deploys Wallabag.

## Prerequisites

Before installing the chart, you must manually create the following directories on your host system to be used by the persistent volumes:

- `/data/nfs/k3svolumes/wallabag/data`
- `/data/nfs/k3svolumes/wallabag/images`

## Installing the Chart

To install the chart with the release name `my-release`, run the following command:

```bash
helm install wallabag . --namespace services --create-namespace
```
