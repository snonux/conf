
# Anki Sync Server Kubernetes Deployment

This directory contains the Kubernetes configuration for deploying the Anki Sync Server.

## Deployment

To deploy the Anki Sync Server, apply the Kubernetes manifests in this directory:

```bash
make apply
```

## Secret Management

The deployment uses a Kubernetes secret to manage the `SYNC_USER1` environment variable. This secret is not included in the repository for security reasons. You must create it manually in the `services` namespace.

### Creating the Secret

To create the secret, use the following `kubectl` command:

```bash
kubectl create secret generic anki-sync-server-secret --from-literal=SYNC_USER1='paul:SECRETPASSWORD' -n services
```

Replace `paul:SECRETPASSWORD` with your desired username and password.

### Updating the Secret

To update the secret, you can delete and recreate it, or use `kubectl edit`:

```bash
kubectl edit secret anki-sync-server-secret -n services
```
