# Audiobookshelf Helm Chart

This chart deploys Audiobookshelf.

## Prerequisites

Before installing the chart, you must manually create the following directories on your host system to be used by the persistent volumes:

- `/data/nfs/k3svolumes/audiobookshelf/config`
- `/data/nfs/k3svolumes/audiobookshelf/audiobooks`
- `/data/nfs/k3svolumes/audiobookshelf/podcasts`

## Installing the Chart

Manual install (normally ArgoCD deploys it):

```bash
helm install audiobookshelf . --namespace services --create-namespace
```
