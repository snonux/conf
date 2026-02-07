# Self-Hosted Git Server with SSH and CGit Web UI

A self-hosted git repository solution for the f3s k3s cluster, replacing external Codeberg dependency.

## Components

- **SSH Git Server**: Alpine-based container with OpenSSH and git for repository access
- **CGit Web UI**: Browse repositories at `http://cgit.f3s.buetow.org`
- **Single Pod Design**: Both containers share storage via ReadWriteMany PVC
- **Persistent SSH Host Keys**: Keys are stored in NFS and persist across pod restarts

## Architecture

```
┌────────────────────────────────────────┐
│     Pod: git-server (cicd namespace)   │
├────────────────────────────────────────┤
│ Container 1: SSH Git Server            │
│  - Port 22 (SSH)                       │
│  - User: git (UID 1000)                │
│                                        │
│ Container 2: cgit + nginx              │
│  - Port 8080 (HTTP)                    │
│  - User: www-data (UID 33)             │
│                                        │
│ Shared Volume: /repos (5Gi, NFS)       │
└────────────────────────────────────────┘
```

## Network Access

- **Internal (ArgoCD)**: `git-server.cicd.svc.cluster.local:22`
- **External SSH**: NodePort 30022 on any cluster node
- **Web UI**: `http://cgit.f3s.buetow.org`

## Initial Setup

### 1. Build and Push Docker Image

```bash
cd docker-image
just f3s
```

### 2. Setup Storage on Cluster Nodes

```bash
ssh root@r0
mkdir -p /data/nfs/k3svolumes/git-server/repos
chown -R 1000:33 /data/nfs/k3svolumes/git-server
chmod -R 0755 /data/nfs/k3svolumes/git-server
```

### 3. Initialize Repository

Clone the existing Codeberg repository as a bare repo:

```bash
ssh root@r0
cd /data/nfs/k3svolumes/git-server/repos
git clone --bare https://codeberg.org/snonux/conf.git conf.git
chown -R 1000:33 conf.git
chmod -R 0755 conf.git
```

### 4. Create SSH Key Secrets

**IMPORTANT**: Secrets must be created manually in Kubernetes, NOT stored in git.

#### For ArgoCD Access

Generate SSH key pair:

```bash
ssh-keygen -t ed25519 -C "argocd@f3s.cluster" -f /tmp/argocd-git-key -N ""
```

Create authorized_keys secret for git-server:

```bash
# Save public key to file
cat /tmp/argocd-git-key.pub > /tmp/authorized_keys

# Create secret in Kubernetes
kubectl create secret generic git-server-authorized-keys \
  --from-file=authorized_keys=/tmp/authorized_keys \
  -n cicd
```

Create private key secret for ArgoCD (needed later):

```bash
kubectl create secret generic argocd-git-ssh-key \
  --from-file=sshPrivateKey=/tmp/argocd-git-key \
  -n cicd
```

#### For User Push Access

To add additional SSH keys for users to push:

```bash
# Get current authorized_keys
kubectl get secret git-server-authorized-keys -n cicd -o jsonpath='{.data.authorized_keys}' | base64 -d > /tmp/authorized_keys

# Add your SSH public key
echo "ssh-ed25519 AAAAC3Nza... user@host" >> /tmp/authorized_keys

# Update secret
kubectl create secret generic git-server-authorized-keys \
  --from-file=authorized_keys=/tmp/authorized_keys \
  -n cicd \
  --dry-run=client -o yaml | kubectl apply -f -

# Restart git-server to pick up new keys
kubectl rollout restart deployment/git-server -n cicd
```

### 5. Deploy via ArgoCD

```bash
kubectl apply -f /home/paul/git/conf/f3s/argocd-apps/cicd/git-server.yaml
```

Or commit and push the ArgoCD Application manifest to let ArgoCD sync automatically.

### 6. Verify Deployment

```bash
# Check pod status
kubectl get pods -n cicd -l app=git-server

# Check logs
kubectl logs -n cicd -l app=git-server -c git-server --tail=50
kubectl logs -n cicd -l app=git-server -c cgit --tail=50

# Test cgit web UI
curl -I http://cgit.f3s.buetow.org
```

## Repository URLs

### For ArgoCD (Internal)

```
ssh://git@git-server.cicd.svc.cluster.local/repos/conf.git
```

### For Users (External)

```bash
# Via NodePort (direct)
git clone ssh://git@r0:30022/repos/conf.git

# Via SSH config alias
# Add to ~/.ssh/config:
Host f3s-git
  HostName r0.f3s.buetow.org
  Port 30022
  User git
  IdentityFile ~/.ssh/id_f3s_git

# Then clone with:
git clone f3s-git:/repos/conf.git
```

## Managing Repositories

### Add New Repository

```bash
ssh root@r0
cd /data/nfs/k3svolumes/git-server/repos
git init --bare newrepo.git
chown -R 1000:33 newrepo.git
chmod -R 0755 newrepo.git
```

The new repository will automatically appear in cgit (scan-path feature).

### Remove Repository

```bash
ssh root@r0
rm -rf /data/nfs/k3svolumes/git-server/repos/oldrepo.git
```

## Troubleshooting

### Git Push Fails with Permission Denied

1. Check if your SSH key is in authorized_keys:
   ```bash
   kubectl get secret git-server-authorized-keys -n cicd -o jsonpath='{.data.authorized_keys}' | base64 -d
   ```

2. Verify git-server pod is running:
   ```bash
   kubectl get pods -n cicd -l app=git-server
   ```

3. Check SSH logs:
   ```bash
   kubectl logs -n cicd -l app=git-server -c git-server -f
   ```

### CGit Shows No Repositories

1. Check if repos exist in storage:
   ```bash
   ssh root@r0 ls -la /data/nfs/k3svolumes/git-server/repos/
   ```

2. Check cgit container logs:
   ```bash
   kubectl logs -n cicd -l app=git-server -c cgit
   ```

3. Verify cgit configuration:
   ```bash
   kubectl get configmap cgit-config -n cicd -o yaml
   ```

### ArgoCD Can't Clone Repository

1. Verify ArgoCD SSH key secret exists:
   ```bash
   kubectl get secret argocd-git-ssh-key -n cicd
   ```

2. Check if ArgoCD public key is in authorized_keys:
   ```bash
   kubectl get secret git-server-authorized-keys -n cicd -o jsonpath='{.data.authorized_keys}' | base64 -d
   ```

3. Test SSH connection from ArgoCD repo-server:
   ```bash
   kubectl exec -n cicd deploy/argocd-repo-server -- \
     ssh -T git@git-server.cicd.svc.cluster.local
   ```

## Backup and Recovery

Backups are handled by ZFS snapshots at the storage layer (`/data/nfs/k3svolumes/git-server`).

To recover:
1. Restore ZFS snapshot
2. Redeploy git-server application via ArgoCD

## Security Notes

- SSH keys are restricted to git-shell only (no shell access)
- git-server container runs as non-root user (UID 1001)
- cgit container has read-only access to repositories
- All container capabilities dropped for enhanced security
- Secrets managed via Kubernetes Secrets, never committed to git
- SSH host keys stored in NFS but copied to local emptyDir at startup (OpenSSH security requirement)

## Monitoring

View logs:

```bash
# SSH server logs
kubectl logs -n cicd -l app=git-server -c git-server -f

# CGit web server logs
kubectl logs -n cicd -l app=git-server -c cgit -f
```

Check resource usage:

```bash
kubectl top pod -n cicd -l app=git-server
```
