# linkding Helm Chart

This chart deploys linkding.

## Prerequisites

Before installing the chart, you must manually create the following directory on your host system to be used by the persistent volume:

- `/data/nfs/k3svolumes/linkding/data`

## Installing the Chart

To install the chart with the release name `linkding`, run the following command:

```bash
helm install linkding . --namespace services --create-namespace
```