# Git Server Helm Chart

Self-hosted git server for the f3s Kubernetes cluster with SSH access, HTTP git-http-backend, and cgit web UI.

## Components

### 1. SSH Git Server
- **Port**: 22 (internal), 30022 (NodePort external)
- **User**: git (UID 1001, GID 33)
- **Shell**: git-shell (restricted)
- **Image**: `registry.lan.buetow.org:30001/git-server:1.0`

### 2. cgit Web UI
- **URL**: https://cgit.f3s.buetow.org
- **Port**: 80 (HTTP, proxied via Traefik)
- **Image**: `joseluisq/alpine-cgit:latest`
- **Features**: Repository browsing, syntax highlighting

### 3. git-http-backend
- **Internal**: `http://git-server.cicd.svc.cluster.local/<repo>.git`
- **External**: `https://cgit.f3s.buetow.org/<repo>.git`
- **Used by**: ArgoCD for syncing applications
- **FastCGI**: nginx + fcgiwrap + git-http-backend

## Storage

**PVC**: `git-server-pvc` (5Gi, ReadWriteMany)
**NFS Path**: `/data/nfs/k3svolumes/git-server/repos/`
**Ownership**: UID 1001:GID 33 (git:www-data)
**Permissions**: Group writable (g+w) for shared access

Repository structure:
```
/data/nfs/k3svolumes/git-server/repos/
├── conf.git/           # ArgoCD configuration repository
├── gitsyncer.git/      # Gitsyncer tool
├── algorithms.git/     # Other repositories...
└── ...
```

## Access Methods

### SSH Access (Push/Pull)

**External** (from your workstation):
```bash
# Via SSH config alias
git clone git-server:/repos/<repo>.git

# Or directly
git clone ssh://git@r0:30022/repos/<repo>.git
```

**SSH Configuration** (`~/.ssh/config`):
```
Host git-server
HostName r0
Port 30022
User git
```

**Add SSH Key**:
```bash
# Add your public key to authorized_keys secret
kubectl edit secret git-server-authorized-keys -n cicd
```

### HTTP Access (Clone/Fetch)

**Internal** (from cluster, e.g., ArgoCD):
```bash
git clone http://git-server.cicd.svc.cluster.local/<repo>.git
```

**External** (via Traefik ingress):
```bash
git clone https://cgit.f3s.buetow.org/<repo>.git
```

### Web UI

**Browse repositories**: https://cgit.f3s.buetow.org

Features:
- Repository listing with descriptions
- Browse files and commits
- Syntax highlighting
- Commit history
- Diff viewing

## Gitsyncer Integration

Gitsyncer syncs repositories from Codeberg/GitHub to the self-hosted git-server as a backup location.

### Configuration

**Gitsyncer Config** (`~/.config/gitsyncer/config.json`):
```json
{
  "organizations": [
    {
      "host": "git@codeberg.org",
      "name": "snonux"
    },
    {
      "host": "git@github.com",
      "name": "snonux"
    },
    {
      "host": "ssh://git@r0:30022/repos",
      "backupLocation": true
    }
  ]
}
```

**Note**: The config uses explicit NodePort (30022) on cluster node r0. You could also use an SSH alias (see below) with `git@git-server:/repos` for shorter syntax.

### SSH Config for Gitsyncer (Optional)

If you prefer using an SSH alias instead of the explicit URL, add to `~/.ssh/config`:
```
Host git-server
HostName r0
Port 30022
User git
```

### Syncing Repositories

**Sync single repository**:
```bash
gitsyncer sync repo <repo-name> --backup --no-releases
```

**Sync all repositories**:
```bash
gitsyncer sync bidirectional --backup --no-releases
```

**Example**:
```bash
# Sync gitsyncer itself to the backup
gitsyncer sync repo gitsyncer --backup --no-releases
```

### Creating New Repositories

⚠️ **Important**: Gitsyncer cannot auto-create repositories due to git-shell security restrictions.

**Manually create repository first**:
```bash
ssh root@r0 "cd /data/nfs/k3svolumes/git-server/repos && \
  git init --bare <repo-name>.git && \
  chown -R 1001:33 <repo-name>.git && \
  chmod -R g+w <repo-name>.git"
```

**Then sync with gitsyncer**:
```bash
gitsyncer sync repo <repo-name> --backup --no-releases
```

### Adding Repository Descriptions

Repository descriptions appear in cgit web UI.

**Update single repository**:
```bash
ssh root@r0 "echo 'Your repository description' > /data/nfs/k3svolumes/git-server/repos/<repo-name>.git/description"
```

**Bulk update from gitsyncer cache**:
```bash
# Gitsyncer maintains descriptions cache
cat /home/paul/git/gitsyncer-workdir/.gitsyncer-descriptions-cache.json

# Script to update all descriptions (run on r0)
jq -r 'to_entries[] | "\(.key)\t\(.value)"' /tmp/cache.json | while IFS=$'\t' read -r repo desc; do
  echo "$desc" > /data/nfs/k3svolumes/git-server/repos/${repo}.git/description
done
```

## ArgoCD Integration

ArgoCD applications use HTTP to fetch from the git-server.

**Application manifest example**:
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

**Update existing application**:
```bash
# Change from SSH to HTTP
kubectl edit application <app-name> -n cicd

# Old: ssh://git@git-server.cicd.svc.cluster.local/repos/conf.git
# New: http://git-server.cicd.svc.cluster.local/conf.git
```

## Deployment

**Deploy via ArgoCD**:
```bash
kubectl apply -f /home/paul/git/conf/f3s/argocd-apps/cicd/git-server.yaml
```

**Manual deploy**:
```bash
cd /home/paul/git/conf/f3s/git-server/helm-chart
helm upgrade --install git-server . -n cicd
```

**Verify deployment**:
```bash
kubectl get pods -n cicd -l app=git-server
kubectl logs -n cicd -l app=git-server -c git-server
kubectl logs -n cicd -l app=git-server -c cgit
```

## Security

### Container Security Contexts

**Pod-level**:
- `fsGroup: 33` (www-data)

**git-server container**:
- `runAsUser: 1001` (git)
- `runAsGroup: 33` (www-data)
- `allowPrivilegeEscalation: false`
- `capabilities.drop: ["ALL"]`

**cgit container**:
- `runAsUser: 33` (www-data)
- `runAsGroup: 33` (www-data)
- `allowPrivilegeEscalation: false`
- `capabilities.drop: ["ALL"]`
- `HOME: /tmp` (for git config writes)

### SSH Security

- **git-shell**: Restricts available commands (git-receive-pack, git-upload-pack, git-upload-archive)
- **Authorized keys**: Managed via Kubernetes secret
- **No password auth**: SSH key authentication only
- **No root access**: git user only

### HTTP Security

- **No authentication**: Currently unauthenticated (internal cluster access)
- **TLS**: External access via Traefik with TLS termination
- **Timeouts**: 300s read/send timeout for large clones

### File Permissions

**Repositories**:
- Owner: UID 1001 (git user in container)
- Group: GID 33 (www-data)
- Permissions: Group writable (g+w)

**Why group writable?**
- git-server container (UID 1001) can write
- cgit container (UID 33) can read
- External SSH pushes create files with client UID but preserve GID 33

## Troubleshooting

### SSH Push Fails with Permission Denied

**Symptom**:
```
fatal: unable to write file ./objects/xx/yyy: Permission denied
```

**Fix**:
```bash
# Fix ownership and permissions
ssh root@r0 "cd /data/nfs/k3svolumes/git-server/repos && \
  chown -R 1001:33 <repo>.git && \
  chmod -R g+w <repo>.git"
```

### HTTP Clone Returns 403 Forbidden

**Symptom**:
```
fatal: unable to access 'http://...': The requested URL returned error: 403
```

**Fix**:
```bash
# Enable HTTP operations in repo config
ssh root@r0 "git config --file /data/nfs/k3svolumes/git-server/repos/<repo>.git/config http.receivepack true && \
  git config --file /data/nfs/k3svolumes/git-server/repos/<repo>.git/config http.uploadpack true"
```

### HTTP Clone Returns 500 Error

**Symptom**:
```
fatal: unable to access 'http://...': The requested URL returned error: 500
```

**Cause**: Git safe.directory check or ownership issues

**Fix**:
```bash
# Check cgit container logs
kubectl logs -n cicd -l app=git-server -c cgit --tail=50

# Common error: "fatal: detected dubious ownership"
# Already fixed in deployment with:
#   git config --global --add safe.directory /repos/<repo>.git
#   HOME=/tmp (for cgit container)
```

### Repository Not Visible in cgit

**Symptom**: Repository exists but doesn't show in web UI

**Cause**: cgit scans `/repos/` directory

**Verify**:
```bash
# Check scan-path in cgit config
kubectl get configmap cgit-config -n cicd -o yaml

# Should show: scan-path=/repos
```

**Fix**: Restart cgit container
```bash
kubectl delete pod -n cicd -l app=git-server
```

### SSH Host Key Changed

**Symptom**:
```
WARNING: REMOTE HOST IDENTIFICATION HAS CHANGED!
```

**Cause**: Pod restart generates new SSH host keys (stored in emptyDir)

**Fix**:
```bash
# Update known_hosts
ssh-keygen -R "[r0]:30022"
ssh-keyscan -p 30022 r0 >> ~/.ssh/known_hosts
```

**For ArgoCD**: Update SSH known_hosts ConfigMap
```bash
# Get current host keys
kubectl exec -n cicd -c git-server $(kubectl get pod -n cicd -l app=git-server -o name) -- sh -c "for key in /etc/ssh/ssh_host_*_key.pub; do ssh-keygen -l -f \$key && cat \$key; done"

# Update argocd/git-server-known-hosts.yaml with new keys
```

### Gitsyncer Cannot Create Repository

**Symptom**:
```
fatal: unrecognized command 'mkdir -p repos/<repo>.git && cd repos/<repo>.git && git init --bare'
```

**Cause**: git-shell restricts available commands (security feature)

**Solution**: Manually create repository first (see "Creating New Repositories" section above)

## Performance

### Repository Sizes

Current setup: 39 repositories, ~100MB total

**Limits**:
- PVC: 5Gi (plenty of room for growth)
- nginx timeouts: 300s (supports large clones)
- FastCGI buffering: disabled (streaming responses)

### Backup Strategy

**Primary**: Codeberg/GitHub (gitsyncer sync sources)
**Backup**: Self-hosted git-server (backup location)
**Sync frequency**: Manual (via gitsyncer commands)
**Storage**: ZFS snapshots on NFS backend (FreeBSD host)

## Maintenance

### Update Repository Descriptions

```bash
# From gitsyncer cache
scp ~/git/gitsyncer-workdir/.gitsyncer-descriptions-cache.json root@r0:/tmp/

# Run update script on r0
ssh root@r0 "jq -r 'to_entries[] | \"\\(.key)\\t\\(.value)\"' /tmp/.gitsyncer-descriptions-cache.json | while IFS=\$'\\t' read -r repo desc; do echo \"\$desc\" > /data/nfs/k3svolumes/git-server/repos/\${repo}.git/description 2>/dev/null && echo \"✓ \$repo\" || echo \"✗ \$repo\"; done"
```

### Add New SSH Key

```bash
# Edit secret
kubectl edit secret git-server-authorized-keys -n cicd

# Add new public key line to authorized_keys data
# Restart pod to reload
kubectl delete pod -n cicd -l app=git-server
```

### Check Repository Statistics

```bash
# Repository count
ssh root@r0 "ls -1 /data/nfs/k3svolumes/git-server/repos/*.git | wc -l"

# Total size
ssh root@r0 "du -sh /data/nfs/k3svolumes/git-server/repos/"

# List repositories with sizes
ssh root@r0 "du -sh /data/nfs/k3svolumes/git-server/repos/*.git"
```

### Monitor Logs

```bash
# SSH git-server logs
kubectl logs -n cicd -l app=git-server -c git-server -f

# cgit/nginx logs
kubectl logs -n cicd -l app=git-server -c cgit -f

# ArgoCD repo-server logs (git fetch operations)
kubectl logs -n cicd deployment/argocd-repo-server -f | grep git-server
```

## Resources

- **cgit**: https://git.zx2c4.com/cgit/
- **git-http-backend**: https://git-scm.com/docs/git-http-backend
- **gitsyncer**: https://codeberg.org/snonux/gitsyncer
- **ArgoCD**: https://argo-cd.readthedocs.io/

## Files

- `Chart.yaml` - Helm chart metadata
- `templates/deployment.yaml` - Main deployment (git-server + cgit)
- `templates/service.yaml` - ClusterIP and NodePort services
- `templates/ingress.yaml` - Traefik ingress for cgit
- `templates/persistent-volume.yaml` - PV and PVC for repository storage
- `templates/configmap-cgit.yaml` - cgit configuration
- `templates/secret-ssh-keys.yaml` - SSH authorized_keys

## Contributing

When making changes:

1. Update deployment.yaml if changing container configurations
2. Test SSH and HTTP access after changes
3. Verify ArgoCD can still sync applications
4. Check cgit web UI still works
5. Update this README with new information

## License

Part of the f3s cluster configuration repository.
