# WebDAV Kubernetes Deployment

This directory contains the Kubernetes configuration for deploying an Apache WebDAV server to a k3s cluster. It shares the same data directory as File Browser.

## Prerequisites

### 1. File Browser must be deployed first

This WebDAV server reuses the `filebrowser-data-pvc` persistent volume claim. Ensure File Browser is already deployed:

```bash
cd ../filebrowser
just install
```

### 2. Create the htpasswd secret

Generate a password file and create the Kubernetes secret:

```bash
# Install htpasswd if not available
# On Fedora: dnf install httpd-tools
# On Debian/Ubuntu: apt install apache2-utils

# Generate htpasswd file (replace USERNAME and PASSWORD)
htpasswd -cb /tmp/webdav.htpasswd USERNAME PASSWORD

# Create the secret
kubectl create secret generic webdav-htpasswd \
    --from-file=webdav.htpasswd=/tmp/webdav.htpasswd \
    -n services

# Clean up
rm /tmp/webdav.htpasswd
```

To add additional users:

```bash
htpasswd -b /tmp/webdav.htpasswd ANOTHER_USER ANOTHER_PASSWORD
kubectl delete secret webdav-htpasswd -n services
kubectl create secret generic webdav-htpasswd \
    --from-file=webdav.htpasswd=/tmp/webdav.htpasswd \
    -n services
kubectl rollout restart deployment/webdav -n services
```

## Deployment

```bash
just install
```

## Configuration

WebDAV will be accessible at: `http://webdav.f3s.buetow.org`

The WebDAV root (`/webdav`) serves files from `/data/nfs/k3svolumes/filebrowser/data` - the same directory as File Browser.

## Storage

Uses the same persistent volume as File Browser:
- **data** (50Gi): Shared with File Browser at `/data/nfs/k3svolumes/filebrowser/data`

## Permissions

Runs with UID/GID 1000:1000, matching File Browser's permissions.

## Justfile Commands

- `just install` - Install WebDAV using Helm
- `just upgrade` - Upgrade the WebDAV deployment
- `just delete` - Uninstall WebDAV from the cluster

## WebDAV Client Access

Connect using any WebDAV client with:
- URL: `https://webdav.f3s.buetow.org/webdav/` (after TLS offloading via relayd)
- Username/Password: As configured in the htpasswd secret


