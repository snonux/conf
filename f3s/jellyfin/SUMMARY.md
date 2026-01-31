# Jellyfin Deployment Summary

## Objective
Deploy Jellyfin 10.11.6 (latest stable) with proper reverse proxy configuration through relayd and Traefik, accessible at `https://jellyfin.f3s.buetow.org` and alternate ports 8096/8920 for Android app compatibility.

## Configuration Implemented

### 1. Kubernetes Resources
- **Deployment**: Jellyfin server using `jellyfin/jellyfin:latest` image
- **Service**: NodePort service exposing ports 30096 and 30920 for direct internal access
- **PersistentVolumes/Claims**: Three volumes for config, libraries, and data
- **Pod Resources**: 100m CPU request, 256Mi RAM request; 2000m CPU limit, 2Gi RAM limit

### 2. Reverse Proxy Configuration (relayd)

#### Frontend Setup
- **TLS Termination**: Relayd listens on ports 443 (IPv4/IPv6) with Let's Encrypt certificates
- **Header Forwarding**: 
  - `X-Forwarded-For: $REMOTE_ADDR` (client IP)
  - `X-Forwarded-Proto: https` (protocol indication)
- **Multiple Ports**: Added separate relay rules for ports 8096 and 8920 to support Android app discovery attempts

#### Routing Rules
- **Port 443**: Routes to Jellyfin NodePort 30096 via `f3s_jellyfin` backend table
- **Ports 8096/8920**: Dual IPv4/IPv6 relays also forward to NodePort 30096
- **Host Routing**: Explicit match for `jellyfin.f3s.buetow.org` hostname

### 3. Jellyfin Network Configuration
- **RequireHttps**: false (TLS handled by relayd)
- **EnableHttps**: false (no self-signed certs)
- **PublicPort**: 443 (external port users connect to)
- **KnownProxies**: 
  - 10.0.0.0/8 (Kubernetes cluster CIDR)
  - 192.168.0.0/16 (relayd/frontend subnet)
- **EnablePublishedServerUriByRequest**: false

### 4. Certificate Chain
- **Full Chain**: Relayd presents complete certificate chain (leaf + R12 intermediate)
- **Validation**: Confirmed with `openssl s_client` showing 2 certificates
- **Auto-renewal**: Let's Encrypt certificates on relayd

## Issues Encountered & Solutions

### Issue 1: Database Migration Failures
- **Problem**: Upgrading from 10.8.13 → 10.11.6 directly caused database corruption
- **Solution**: Requires upgrade path 10.8.13 → 10.10.7 → 10.11.6
- **Status**: Settled on `jellyfin:latest` (10.11.6) with clean database

### Issue 2: ConfigMap Read-Only Mount
- **Problem**: Network.xml mounted as read-only ConfigMap; newer Jellyfin versions need to write during migration
- **Solution**: Removed ConfigMap mount, let Jellyfin manage network.xml from PVC
- **Result**: Cleaner configuration, Jellyfin can self-manage settings

### Issue 3: Android App "Unsupported version or product" Error
- **Root Cause**: 
  - Missing full certificate chain from relayd → Android app SSL validation failure
  - App attempting alternate ports (8096, 8920) that weren't exposed
- **Solution**: 
  - Added relayd relays for ports 8096 and 8920
  - Ensured full cert chain is presented
  - App should now connect to any of the three ports

### Issue 4: NFS Storage Read-Only (CURRENT BLOCKER)
- **Problem**: `/data/nfs/k3svolumes/jellyfin/*` directories mounted read-only
- **Error**: `chown` and pod writes fail with "Read-only file system"
- **Status**: PVCs remain Pending; pods cannot start
- **Resolution Required**: NFS mount needs to be remounted as read-write on f0/f1/f2 hosts

## Current Deployment Status

✅ **Complete**
- Kubernetes manifests fully configured
- ArgoCD Application re-enabled with proper git URL
- Relayd configuration updated and deployed
- Certificate chain verified
- All networking rules in place

❌ **Blocked**
- PersistentVolumes cannot bind to PVCs (read-only NFS)
- Jellyfin pod remains in Pending state
- Cannot proceed with testing until NFS is writable

## Next Steps

1. **Fix NFS Mount** (blocking issue)
   ```bash
   # On f0, f1, f2 - remount /data with write permissions
   doas mount -uw /data
   # Or check NFS export configuration
   ```

2. **Deploy & Test**
   - Once NFS is writable, pods will automatically start via ArgoCD
   - Test connectivity: `curl https://jellyfin.f3s.buetow.org/System/Info/Public`
   - Test Android app with manual URL entry

3. **Configure Jellyfin** (post-deployment)
   - Run setup wizard
   - Add media libraries
   - Configure transcoding if needed
   - Verify Android app can connect

## Key Files

- **Deployment**: `jellyfin/helm-chart/templates/deployment.yaml`
- **Persistent Storage**: `jellyfin/helm-chart/templates/persistent-volume.yaml`
- **Relayd Config**: `/home/paul/git/conf/frontends/etc/relayd.conf.tpl` (lines ~15-130)
- **ArgoCD App**: Created via kubectl in services namespace

## Testing Commands

```bash
# Check pod status
kubectl get pods -n services -l app=jellyfin-server

# View logs
kubectl logs -n services -l app=jellyfin-server

# Test API endpoint
curl https://jellyfin.f3s.buetow.org/System/Info/Public

# Verify certificate chain
echo | openssl s_client -servername jellyfin.f3s.buetow.org -connect jellyfin.f3s.buetow.org:443 | grep "BEGIN CERTIFICATE" | wc -l
# Should output: 2

# Check PVC binding
kubectl get pvc -n services | grep jellyfin
```

## Notes

- Latest version (10.11.6) requires database >= 10.9.11
- Android app compatibility improved in 10.10.7+
- Relayd provides full TLS termination, reducing complexity vs. Traefik double-proxy
- NodePort approach bypasses Traefik, avoiding header forwarding issues
