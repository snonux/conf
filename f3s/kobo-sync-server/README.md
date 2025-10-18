# Kobo Sync Server

This directory contains the Helm chart for deploying the [koreader-sync-server](https://github.com/koreader/koreader-sync-server).

## Prerequisites

Before installing the chart, you must manually create the following directory on your `f0` host system to be used by the persistent volume:

- `/data/nfs/k3svolumes/koreader-sync-server/data`

To do so, run the following command on `f0`:

```bash
mkdir -p /data/nfs/k3svolumes/koreader-sync-server/data
```

## Deployment

To deploy the koreader-sync-server to the k3s cluster, you can use the `Justfile` in this directory.

### Install

To install the Helm chart, run the following command:

```bash
just install
```

### Upgrade

To upgrade the Helm chart, run the following command:

```bash
just upgrade
```

### Delete

To delete the Helm chart, run the following command:

```bash
just delete
```
