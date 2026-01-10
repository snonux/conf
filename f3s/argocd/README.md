# ArgoCD Deployment for f3s Cluster

ArgoCD is a declarative, GitOps continuous delivery tool for Kubernetes.

## Overview

This deployment follows f3s cluster patterns:
- **Namespace**: `cicd` (new namespace for CI/CD tooling)
- **Deployment Mode**: Non-HA single instance
- **Persistence**: 10Gi hostPath volume for repo-server
- **Ingress**: Traefik at argocd.f3s.buetow.org
- **Monitoring**: ServiceMonitor integration with Prometheus

## Architecture

ArgoCD components deployed:
- **argocd-server**: Web UI and API server (1 replica)
- **argocd-repo-server**: Repository management and manifest generation (1 replica, with PVC)
- **argocd-application-controller**: Monitors applications and manages deployments (1 replica)
- **argocd-redis**: Cache for application state (1 replica)
- **argocd-applicationset-controller**: Multi-app management (1 replica)
- **argocd-dex-server**: Disabled (no SSO/OAuth needed)

## Prerequisites

Before installation, ensure storage directory exists on cluster nodes:

```bash
# SSH to each Rocky Linux k3s node (r0, r1, r2)
ssh root@r0
mkdir -p /data/nfs/k3svolumes/argocd/repo-server
chmod 777 /data/nfs/k3svolumes/argocd/repo-server

# Repeat for r1, r2
ssh root@r1
mkdir -p /data/nfs/k3svolumes/argocd/repo-server
chmod 777 /data/nfs/k3svolumes/argocd/repo-server

ssh root@r2
mkdir -p /data/nfs/k3svolumes/argocd/repo-server
chmod 777 /data/nfs/k3svolumes/argocd/repo-server
```

## Installation

Deploy ArgoCD using the Justfile:

```bash
just install
```

This will:
1. Add the Argo Helm repository
2. Create persistent volume and claim
3. Install ArgoCD Helm chart in `cicd` namespace
4. Create Traefik ingress for the UI
5. Display access instructions

## Access ArgoCD

### Web UI

URL: http://argocd.f3s.buetow.org

**Default credentials:**
- Username: `admin`
- Password: Retrieve with `just get-password`

```bash
just get-password
```

### ArgoCD CLI

Install the CLI:

```bash
curl -sSL -o argocd-linux-amd64 https://github.com/argoproj/argo-cd/releases/latest/download/argocd-linux-amd64
sudo install -m 555 argocd-linux-amd64 /usr/local/bin/argocd
rm argocd-linux-amd64
```

Login:

```bash
argocd login argocd.f3s.buetow.org --insecure
# Enter username: admin
# Enter password: (from just get-password)
```

## Management

### Check Status

```bash
just status
```

### View Logs

```bash
just logs
```

### Upgrade ArgoCD

```bash
just upgrade
```

### Uninstall

```bash
just uninstall
```

**Warning**: This will delete all ArgoCD resources including applications, but the persistent volume data will be retained.

## Post-Deployment Configuration

### 1. Setup Self-Hosted Git Repository

ArgoCD in the f3s cluster uses the self-hosted git-server for all application manifests. The configuration repository (conf.git) must be available on the git-server before deploying applications.

**Ensure conf.git is synced to git-server:**

```bash
# Using gitsyncer (recommended for keeping repos in sync)
gitsyncer sync repo conf --backup --no-releases

# Or manually push to git-server
cd /path/to/conf
git remote add r0 ssh://git@r0:30022/repos/conf.git
git push r0 master
```

**Verify repository is accessible:**

```bash
# Via SSH
git ls-remote ssh://git@r0:30022/repos/conf.git

# Via HTTP (used by ArgoCD)
curl -s "http://git-server.cicd.svc.cluster.local/conf.git/info/refs?service=git-upload-pack" | head -5
```

**ArgoCD Repository Configuration:**

ArgoCD applications use HTTP to fetch from the self-hosted git-server:
- **Repository URL**: `http://git-server.cicd.svc.cluster.local/conf.git`
- **No authentication required** (internal cluster access)
- **Auto-sync enabled** for most applications

Example application manifest:
```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-app
  namespace: cicd
spec:
  project: default
  source:
    repoURL: http://git-server.cicd.svc.cluster.local/conf.git
    targetRevision: master
    path: f3s/my-app/helm-chart
  destination:
    server: https://kubernetes.default.svc
    namespace: my-namespace
```

**Important**: Always push changes to git-server (r0) before ArgoCD can sync them. Changes pushed only to external git hosts (Codeberg/GitHub) will not be picked up by ArgoCD.

See `/home/paul/git/conf/f3s/git-server/helm-chart/README.md` for more details on the git-server setup.

### 2. Change Admin Password

**Important**: Change the default admin password immediately after first login.

Using the Web UI:
1. Login to http://argocd.f3s.buetow.org
2. Click on "User Info" in the left sidebar
3. Click "Update Password"

Using the CLI:

```bash
argocd login argocd.f3s.buetow.org --insecure
argocd account update-password
```

### 3. Add Additional Git Repositories

For public repositories:

```bash
argocd repo add https://github.com/argoproj/argocd-example-apps.git
```

For private repositories (HTTPS):

```bash
argocd repo add https://github.com/yourusername/yourrepo.git \
  --username git \
  --password ghp_yourGitHubPersonalAccessToken
```

For private repositories (SSH):

```bash
argocd repo add git@github.com:yourusername/yourrepo.git \
  --ssh-private-key-path ~/.ssh/id_rsa
```

### 4. Create Your First Application

Using the CLI:

```bash
argocd app create guestbook \
  --repo https://github.com/argoproj/argocd-example-apps.git \
  --path guestbook \
  --dest-server https://kubernetes.default.svc \
  --dest-namespace default
```

Using the Web UI:
1. Click "+ NEW APP" button
2. Fill in application details
3. Click "CREATE"

### 5. Sync an Application

```bash
argocd app sync guestbook
```

Or enable auto-sync:

```bash
argocd app set guestbook --sync-policy automated
```

## Monitoring

ArgoCD metrics are automatically scraped by Prometheus via ServiceMonitor.

View metrics in Grafana: http://grafana.f3s.buetow.org

**Recommended Grafana Dashboards:**
- ArgoCD (ID: 14584) - https://grafana.com/grafana/dashboards/14584
- ArgoCD Application Metrics (ID: 19993) - https://grafana.com/grafana/dashboards/19993

Import dashboards:
1. Go to Grafana → Dashboards → Import
2. Enter dashboard ID
3. Select Prometheus datasource
4. Click "Import"

## Troubleshooting

### Check All Pods are Running

```bash
kubectl get pods -n cicd
```

Expected output:
```
NAME                                                READY   STATUS    RESTARTS   AGE
argocd-application-controller-0                     1/1     Running   0          5m
argocd-applicationset-controller-xxx                1/1     Running   0          5m
argocd-redis-xxx                                    1/1     Running   0          5m
argocd-repo-server-xxx                              1/1     Running   0          5m
argocd-server-xxx                                   1/1     Running   0          5m
```

### Check Persistent Volume Binding

```bash
kubectl get pv argocd-repo-server-pv
kubectl get pvc -n cicd argocd-repo-server-pvc
```

The PVC should be in `Bound` status.

### Access Server Logs

```bash
kubectl logs -n cicd -l app.kubernetes.io/name=argocd-server
```

### Check Ingress

```bash
kubectl describe ingress -n cicd argocd-server-ingress
```

### Application Not Syncing

1. Check repo-server logs:
   ```bash
   kubectl logs -n cicd -l app.kubernetes.io/name=argocd-repo-server
   ```

2. Check application controller logs:
   ```bash
   kubectl logs -n cicd -l app.kubernetes.io/name=argocd-application-controller
   ```

3. Verify repository credentials:
   ```bash
   argocd repo list
   ```

### Reset Admin Password

If you forget the admin password:

```bash
# Delete the initial admin secret
kubectl -n cicd delete secret argocd-initial-admin-secret

# Restart the server to regenerate it
kubectl -n cicd rollout restart deployment argocd-server

# Wait for restart
kubectl -n cicd rollout status deployment argocd-server

# Get new password
just get-password
```

## Common ArgoCD Operations

### List All Applications

```bash
argocd app list
```

### Get Application Details

```bash
argocd app get <app-name>
```

### Delete an Application

```bash
argocd app delete <app-name>
```

### View Application Sync History

```bash
argocd app history <app-name>
```

### Rollback an Application

```bash
argocd app rollback <app-name> <revision-id>
```

## Security Considerations

1. **TLS**: Server runs in insecure mode with TLS termination at Traefik ingress
2. **RBAC**: Configure ArgoCD projects and RBAC policies for team access
3. **Secret Management**: Consider using sealed-secrets or external-secrets operator
4. **Repository Access**: Use SSH keys or personal access tokens (not passwords)
5. **Network Policies**: Consider implementing NetworkPolicy for pod-to-pod communication restrictions

## Backup and Restore

### Backup ArgoCD Configuration

```bash
# Backup all ArgoCD resources
kubectl get applications,appprojects,secrets -n cicd -o yaml > argocd-backup.yaml

# Backup repo-server data (on cluster node)
ssh root@r0
tar czf argocd-repo-backup.tar.gz /data/nfs/k3svolumes/argocd/repo-server
```

### Restore from Backup

```bash
# Restore ArgoCD resources
kubectl apply -f argocd-backup.yaml

# Restore repo-server data (on cluster node)
ssh root@r0
tar xzf argocd-repo-backup.tar.gz -C /
```

## Upgrading ArgoCD

Check for updates:

```bash
helm repo update
helm search repo argo/argo-cd --versions
```

Upgrade to latest version:

```bash
just upgrade
```

Upgrade to specific version:

```bash
helm upgrade argocd argo/argo-cd --namespace cicd -f values.yaml --version X.Y.Z
```

## References

- ArgoCD Documentation: https://argo-cd.readthedocs.io/
- ArgoCD GitHub: https://github.com/argoproj/argo-cd
- Helm Chart: https://github.com/argoproj/argo-helm
- Example Apps: https://github.com/argoproj/argocd-example-apps
