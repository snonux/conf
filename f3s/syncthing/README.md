# Syncthing Kubernetes Deployment

This directory contains the Kubernetes configuration for deploying Syncthing.

## Deployment

To deploy Syncthing, apply the Kubernetes manifests in this directory:

```bash
make apply
```

## Configuration

The deployment uses two persistent volumes:
- `syncthing-config-pv`: for the syncthing configuration. Mapped to `/data/nfs/k3svolumes/syncthing/config` on the host.
- `syncthing-data-pv`: for the syncthing data. Mapped to `/data/nfs/k3svolumes/syncthing/data` on the host.

The web UI is available at http://syncthing.f3s.buetow.org.
The data port is exposed on port 22000.
