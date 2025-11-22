
# File Browser Kubernetes Deployment

This directory contains the Kubernetes configuration for deploying File Browser to a k3s cluster.

## Deployment

To deploy File Browser, use the Justfile commands:

```bash
just install
```

## Prerequisites

Before deploying, ensure the following are set up:

### 1. Create Persistent Volume Directories

The deployment requires three persistent volumes. Create the directories on the k3s node:

```bash
mkdir -p /data/nfs/k3svolumes/filebrowser/data
mkdir -p /data/nfs/k3svolumes/filebrowser/database
mkdir -p /data/nfs/k3svolumes/filebrowser/config
chown -R 1000:1000 /data/nfs/k3svolumes/filebrowser/
chmod -R 775 /data/nfs/k3svolumes/filebrowser/
```

## Configuration

File Browser will be accessible at: `http://filebrowser.f3s.buetow.org`

Default credentials:
- Username: `admin`
- Password: `admin`

**Important:** Change the default password after first login!

## Storage

The deployment uses three persistent volumes on NFS:
- **data** (50Gi): Stores files that can be browsed and managed
- **database** (1Gi): Stores the File Browser database
- **config** (1Gi): Stores configuration files

## Justfile Commands

- `just install` - Install File Browser using Helm
- `just upgrade` - Upgrade the File Browser deployment
- `just delete` - Uninstall File Browser from the cluster
