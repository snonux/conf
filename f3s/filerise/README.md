
# FileRise Kubernetes Deployment

This directory contains the Kubernetes configuration for deploying FileRise to a k3s cluster.

## Deployment

To deploy FileRise, use the Justfile commands:

```bash
just install
```

## Prerequisites

Before deploying, ensure the following are set up:

### 1. Create Persistent Volume Directories

The deployment requires three persistent volumes. Create the directories on the k3s node:

```bash
mkdir -p /data/nfs/k3svolumes/filerise/uploads
mkdir -p /data/nfs/k3svolumes/filerise/users
mkdir -p /data/nfs/k3svolumes/filerise/metadata
mkdir -p /data/nfs/k3svolumes/filerise/config
chown -R 1000:1000 /data/nfs/k3svolumes/filerise/
chmod -R 775 /data/nfs/k3svolumes/filerise/
```

### 2. Create the Secret

FileRise uses a Kubernetes secret to manage the `PERSISTENT_TOKENS_KEY` environment variable. This secret is not included in the repository for security reasons. You must create it manually in the `services` namespace.

Create the secret with the following command:

```bash
kubectl create secret generic filerise-secret --from-literal=PERSISTENT_TOKENS_KEY='your_random_secure_key_here' -n services
```

Replace `your_random_secure_key_here` with a strong random string.

### Updating the Secret

To update the secret, you can delete and recreate it, or use `kubectl edit`:

```bash
kubectl edit secret filerise-secret -n services
```

## Configuration

FileRise will be accessible at: `http://filerise.f3s.buetow.org`

On first launch, you'll be guided through creating the initial admin user.

## Storage

The deployment uses four persistent volumes:
- **uploads** (50Gi): Stores uploaded files
- **users** (1Gi): Stores user data and configurations
- **metadata** (5Gi): Stores file metadata and database
- **config** (1Gi): Stores application configuration files

## Justfile Commands

- `just install` - Install FileRise using Helm
- `just upgrade` - Upgrade the FileRise deployment
- `just delete` - Uninstall FileRise from the cluster
