# Jellyfin Kubernetes Deployment

This directory contains the Kubernetes configuration for deploying [Jellyfin](https://jellyfin.org/) - a free software media system that puts you in control of your media and data.

## Architecture

Jellyfin is a single-component deployment consisting of:
- **Server**: Main media server with web interface and API

## Prerequisites

1. **Create storage directory on the NFS server**:
   ```bash
   for host in f0 f1 f2; do
     ssh paul@$host "doas mkdir -p /data/nfs/k3svolumes/jellyfin"
     ssh paul@$host "doas chown -R 911:911 /data/nfs/k3svolumes/jellyfin/"
   done
   ```

## Deployment

1. **Install the custom resources** (PVs, PVCs, ingress):
   ```bash
   just install-resources
   ```

2. **Install Jellyfin using Helm** (or ArgoCD):
   ```bash
   just sync
   ```

3. **Check deployment status**:
   ```bash
   just status
   ```

   Wait for all pods to be in `Running` state (may take a few minutes for image pulls).

## Access

Once deployed, Jellyfin will be available at: **https://jelly.f3s.buetow.org**

Default setup instructions:
1. Navigate to the URL above
2. Complete the setup wizard on first access
3. Configure libraries and preferences

## Storage

Persistent storage is configured with:
- **Data**: Main configuration and metadata at `/data/nfs/k3svolumes/jellyfin`
- **Media**: Mount your media directories from other NFS sources as needed

## Maintenance

### Restart Jellyfin
```bash
just restart
```

### View logs
```bash
just logs
```

### Port forward for local access
```bash
just port-forward
```

### Uninstall (keeps data)
```bash
kubectl delete application jellyfin -n cicd
```

## Troubleshooting

### Check pod logs
```bash
kubectl logs -n services -l app=jellyfin-server --tail=100
```

### Verify persistent volumes
```bash
kubectl get pv,pvc -n services | grep jellyfin
```
