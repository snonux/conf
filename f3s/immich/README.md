# Immich Kubernetes Deployment

This directory contains the Kubernetes configuration for deploying [Immich](https://immich.app/) - a self-hosted photo and video backup solution.

## Architecture

Immich consists of several components:
- **Server**: Main API and web interface
- **Machine Learning**: AI-powered face recognition, object detection, and smart search
- **Valkey**: Redis-compatible cache for job queues
- **PostgreSQL**: Database with pgvector extension for AI features

## Prerequisites

1. **Create storage directories on the host**:
   ```bash
   for host in f0 f1 f2; do
     ssh paul@$host "doas mkdir -p /data/nfs/k3svolumes/immich/{library,ml-cache,valkey,postgres}"
     ssh paul@$host "doas chown -R 911:911 /data/nfs/k3svolumes/immich/"
   done
   ```

2. **Create a secure database password secret** (REQUIRED before deployment):
   ```bash
   kubectl create secret generic immich-db-secret \
     --from-literal=password='YOUR_SECURE_PASSWORD_HERE' \
     -n services
   ```

   **Important**:
   - Use a strong, unique password
   - This secret is NOT included in the repository for security reasons
   - The secret must be created before deploying, as PostgreSQL will use it during database initialization

## Deployment

⚠️ **Important**: Complete all prerequisites above before deploying, especially creating the database secret!

1. **Install the custom resources** (PVs, PVCs, PostgreSQL, middleware):
   ```bash
   just install-resources
   ```

2. **Install Immich using Helm**:
   ```bash
   just install
   ```

3. **Check deployment status**:
   ```bash
   just status
   ```

   Wait for all pods to be in `Running` state (may take a few minutes for image pulls).

## Access

Once deployed, Immich will be available at: **https://immich.f3s.buetow.org**

Default setup instructions:
1. Navigate to the URL above
2. Create your admin account on first access
3. Follow the setup wizard to configure your preferences

## Storage

Persistent storage is configured with the following volumes:
- **Library**: 500GB - Main photo/video storage at `/data/nfs/k3svolumes/immich/library`
- **ML Cache**: 10GB - Machine learning models at `/data/nfs/k3svolumes/immich/ml-cache`
- **PostgreSQL**: 20GB - Database storage at `/data/nfs/k3svolumes/immich/postgres`
- **Valkey**: 1GB - Cache/queue data at `/data/nfs/k3svolumes/immich/valkey`

## Maintenance

### Upgrade Immich to latest version
```bash
just upgrade
```

### Redeploy after configuration changes

If you modified any configuration files (values.yaml, templates, etc.):

1. **Update custom resources** (PVs, PostgreSQL, middleware, etc.):
   ```bash
   kubectl apply -f helm-chart/templates/ --namespace services
   ```

2. **Upgrade Immich with new values**:
   ```bash
   just upgrade
   ```

3. **Restart specific components** (if needed):
   ```bash
   # Restart server
   kubectl rollout restart deployment/immich-server -n services

   # Restart all Immich components
   kubectl rollout restart deployment -l app.kubernetes.io/instance=immich -n services
   ```

### Update database password secret

To change the database password after deployment:

1. **Delete existing secret**:
   ```bash
   kubectl delete secret immich-db-secret -n services
   ```

2. **Create new secret with updated password**:
   ```bash
   kubectl create secret generic immich-db-secret \
     --from-literal=password='YOUR_NEW_PASSWORD' \
     -n services
   ```

3. **Update PostgreSQL password and restart**:
   ```bash
   # Connect to PostgreSQL and change password
   kubectl exec -n services -it deployment/immich-postgres -- \
     psql -U immich -d immich -c "ALTER USER immich WITH PASSWORD 'YOUR_NEW_PASSWORD';"

   # Restart Immich components to use new password
   kubectl rollout restart deployment -l app.kubernetes.io/instance=immich -n services
   kubectl rollout restart deployment/immich-postgres -n services
   ```

### Uninstall (keeps data)
```bash
just delete
```

### Complete removal (deletes all data)
```bash
just delete-all
```

## Troubleshooting

### Check pod logs
```bash
kubectl logs -n services -l app.kubernetes.io/instance=immich --tail=100
```

### Check PostgreSQL connection
```bash
kubectl exec -n services -it deployment/immich-postgres -- psql -U immich -d immich -c '\l'
```

### Verify persistent volumes
```bash
kubectl get pv,pvc -n services | grep immich
```

## Quick Reference

### Common redeployment workflow

After making changes to configuration files:

```bash
# 1. Apply template changes (if any)
kubectl apply -f helm-chart/templates/ --namespace services

# 2. Upgrade Helm release
just upgrade

# 3. Check status
just status
```

### Force restart all Immich components

```bash
kubectl rollout restart deployment -l app.kubernetes.io/instance=immich -n services
kubectl rollout restart deployment/immich-postgres -n services
```
