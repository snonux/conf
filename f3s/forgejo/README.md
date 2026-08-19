# Forgejo

Self-hosted git forge at `https://forge.f3s.buetow.org`, running in the `services`
namespace of the f3s k3s cluster.

## Relationship to the cgit git-server

**These two installs are deliberately independent.** Nothing here touches
`f3s/git-server/`:

| | cgit / git-server | Forgejo |
|---|---|---|
| Web UI | `c-git.f3s.buetow.org` | `forge.f3s.buetow.org` |
| Namespace | `cicd` | `services` |
| Repo storage | `/data/nfs/k3svolumes/git-server/repos` (80 bare repos) | `/data/nfs/k3svolumes/forgejo/data` (80 public `snonux` repositories preseeded) |
| SSH NodePort | 30022 | 30222 |

All 80 cgit repositories were preseeded in Forgejo on 2026-08-04. ArgoCD reads
`conf` anonymously from
`http://forgejo.services.svc.cluster.local/snonux/conf.git`. The cgit/git-server
became read-only on 2026-08-05 and remains available for browsing, clones and
rollback during the retention phase.

### Preseed migration report (2026-08-04)

- Authoritative source inventory: 80 bare repositories, 1,238,877,184 bytes
  (1.154 GiB) on disk; 79 newly preseeded and the previously migrated `conf`
  revalidated. All source and destination repositories passed strict, full
  `git fsck`.
- The inventory covered descriptions, symbolic HEAD, every ref/object ID,
  hooks, alternates and LFS indicators. There were no empty repositories,
  unsafe hooks, alternates, LFS indicators or refs outside heads/tags/notes.
- All safe refs in the 79 newly preseeded repositories match exactly. The
  already-live `conf` repository was deliberately not overwritten. Thirty-nine
  source repositories have a stale symbolic `HEAD` pointing at absent `master`;
  Forgejo therefore uses their existing pushed branch as the usable default
  rather than creating a synthetic `master` ref. The other 41 defaults match
  the source symbolic HEAD exactly.
- All 80 targets are public and visible through the anonymous API. Names and
  descriptions match the source. Validation included the two largest repos
  (`foo.zone` and `pages`), the tag-heavy `hexai`, mixed-case
  `Adv360-Pro-ZMK`, and `conf`. There was no empty source repository to sample.
- Bulk Git payload never traversed a laptop, mobile connection or the public
  endpoint. Inventory, fsck and pushes ran on `f0`, reading
  `/data/nfs/k3svolumes/git-server/repos/*.git` locally and pushing over the LAN
  directly to `ssh://git@r0.lan.buetow.org:30222/snonux/REPO.git`. Only small API
  metadata calls were made from the administration workstation.

### Final cutover and retention (2026-08-05)

- The legacy repository service became read-only at **2026-08-05T06:48:19Z**.
  The earliest retirement time, after 14 full days of successful read-only
  retention, is **2026-08-19T06:48:19Z**. Do not retire the service or delete its
  data before that timestamp, and require separate confirmation before deleting
  either the dataset or retained snapshots.
- Both legacy containers mount `/repos` read-only. cgit, anonymous smart HTTP
  fetches and rollback data remain available, while an actual HTTP push probe
  reached `git-receive-pack` and was rejected by the read-only filesystem. The
  pre-cutover snapshot is
  `zdata/enc/nfsdata@forgejo-cutover-pre-20260805T064657Z`. The matching
  authorized-keys file is retained on `f0` as root-owned mode 0600 at
  `/root/git-server-authorized_keys.20260805T064657Z`.
- The final gitsyncer pass completed both directions for every configured public
  repository discovered from Codeberg (47) and GitHub (46), including Forgejo
  backup pushes and API description updates. The installed 49-repository config
  validates and its Forgejo credential remains an owner-only mode 0600 regular
  file outside Git.
- Final inventory found the same 80 legacy names plus the expected Forgejo-only
  `failunderd`: 81 unique public, non-empty repositories. Names, descriptions,
  usable default branches and expected safe refs passed. The old service had 787
  safe refs and Forgejo had 798 after configured repositories' newer primary
  work and `failunderd`; every old ref was preserved or replaced by its reviewed
  current primary ref. Strict full `git fsck` passed for all 80 old and all 81
  Forgejo repositories. Anonymous HTTPS and public SSH both resolved `conf` at
  the cutover commit.
- All 32 ArgoCD Applications were Synced/Healthy. Thirty applications read
  `conf` from Forgejo; the external-chart applications retain their upstream
  sources, and the retiring `git-server` Application intentionally retains its
  Codeberg source. No live Application uses an old cgit URL. The read-only
  cutover commit advanced Forgejo `conf` and was observed by the Forgejo-backed
  Applications, proving reconciliation after the final backup.
- Remaining old-service references are intentional until retirement:
  `f3s/git-server/` defines the retained workload and rollback service;
  `f3s/argocd/git-server-{repo-creds,known-hosts}.yaml` preserves rollback
  credentials; `frontends/Rexfile` keeps the old DNS/TLS/monitoring names alive;
  and existing local `r0`/`r1`/`r2` Git remotes on port 30022 are rollback-only.
  Current gitsyncer configuration and live non-retiring ArgoCD Applications do
  not use those endpoints.

To restore writes during the retention window, revert the read-only cutover
commit in `f3s/git-server/helm-chart/templates/deployment.yaml`, push it to the
`git-server` Application's Codeberg source, wait for that Application to become
Synced/Healthy, and verify a controlled push. If Forgejo itself must be rolled
back, use the Application URL rollback in `f3s/argocd/README.md`; the retained
cgit data and credentials are deliberately left intact for that procedure.

## Architecture

```
Internet -> relayd (OpenBSD GW, TLS) -> WireGuard -> Traefik -> forgejo svc:80 -> pod:3000
git+ssh  -> NodePort 30222 -----------------------------------> pod:2222
                                                                  |
                                        /var/lib/gitea (repos, SQLite)  NFS -> ZFS
                                        /etc/gitea     (app.ini, secrets)
```

- Image `codeberg.org/forgejo/forgejo:16.0.1-rootless` — the rootless variant, so
  the whole pod runs as UID/GID 1000 with all capabilities dropped.
- SQLite, not PostgreSQL, with one writer (`replicas: 1` + `Recreate`). The
  database currently shares the NFS data volume. Forgejo defaults SQLite to WAL,
  which SQLite does not support on network filesystems; the bulk preseed exposed
  this as repeated `locking protocol` failures. The deployment explicitly uses
  `SQLITE_JOURNAL_MODE=DELETE` and a 60-second busy timeout. Rollback journaling
  avoids WAL's shared-memory requirement but does not make SQLite-on-NFS fully
  safe. Move the database to local block storage or PostgreSQL if lock failures
  recur.
- Both volumes carry the standard `.nfs-sentinel` guard so the pod refuses to
  start against the local-XFS shadow if a node has NFS unmounted.

## Initial setup

### 1. Create the storage directories

On the current CARP storage master (check with `ifconfig | grep MASTER` on f0/f1):

```sh
doas mkdir -p /data/nfs/k3svolumes/forgejo/data /data/nfs/k3svolumes/forgejo/config
doas touch    /data/nfs/k3svolumes/forgejo/data/.nfs-sentinel \
              /data/nfs/k3svolumes/forgejo/config/.nfs-sentinel
doas chown -R 1000:1000 /data/nfs/k3svolumes/forgejo
doas chmod 0750 /data/nfs/k3svolumes/forgejo/data /data/nfs/k3svolumes/forgejo/config
doas chmod 0644 /data/nfs/k3svolumes/forgejo/data/.nfs-sentinel \
                /data/nfs/k3svolumes/forgejo/config/.nfs-sentinel
```

The sentinel files are 0644 per `f3s/docs/nfs-sentinel-initcontainer.md` — do not
sweep them up in a recursive chmod of the directories.

The PVs use `type: Directory`, so the pod will not schedule until these exist.

### 2. Publish the hostname

`forge.f3s.buetow.org` must be added to `@f3s_hosts` in `frontends/Rexfile`. That
one array drives the DNS zone, the relayd routing rule, the ACME certificate and
the gogios monitoring checks.

Deploy order matters — relayd loads `tls keypair forge.f3s.buetow.org` at startup,
so the certificate has to exist first:

```sh
cd frontends
rex -H blowfish.buetow.org:2 nsd httpd acme acme_invoke relayd gogios
rex -H fishfinger.buetow.org:2 nsd httpd acme acme_invoke relayd gogios
```

`acme.sh` copies the `foo.zone` cert as a placeholder for any host that has none
yet, so relayd will still start on the first pass. The *real* certificate is only
issued on the gateway currently holding the DNS master IP — `acme.sh` skips
`acme-client` on the standby, which keeps the placeholder until a failover. That
is normal; the standby is not serving the name yet.

`gogios` is included because the TLS and HTTP checks for the new host are
rendered from `@acme_hosts`; without it `forge.f3s.buetow.org` gets no monitoring.

Deploying one gateway at a time avoids restarting both public frontends
simultaneously.

### 3. Deploy

```sh
kubectl apply -f ../argocd-apps/services/forgejo.yaml
```

This apply is required and cannot be skipped: there is no app-of-apps or
ApplicationSet watching `f3s/argocd-apps/`, so pushing the repo alone does
nothing. Once the Application exists, later edits to the chart do auto-sync.

### 4. Create the admin user

The web installer is locked (`INSTALL_LOCK=true`) and registration is disabled,
because this instance is reachable from the public internet. Create the first
account from the CLI:

```sh
just create-admin
```

## Repository URLs

```sh
# HTTPS
git clone https://forge.f3s.buetow.org/<user>/<repo>.git

# SSH, from anywhere -- relayd listens on 2022 and TCP-forwards to the NodePort
git clone ssh://git@forge.f3s.buetow.org:2022/<user>/<repo>.git

# SSH direct to a node, bypassing the gateways (LAN only)
git clone ssh://git@r0.lan.buetow.org:30222/<user>/<repo>.git
```

Port 2022 rather than 22 keeps the forge away from the mass scanning the default
port attracts; 2222 was unavailable, dserver (DTail) already uses it on the
gateways. To administer blowfish/fishfinger, SSH is on port 2 as usual.

To use the short `git@forge.f3s.buetow.org:user/repo.git` form, put the port in
`~/.ssh/config`:

```
Host forge.f3s.buetow.org
  Port 2022
  User git
```

## Operations

```sh
just status          # pods, services, ingress, PVCs, ArgoCD sync
just logs            # follow logs
just restart         # rollout restart
just shell           # shell inside the pod
just port-forward    # reach the UI on localhost:3000
```

## Backup

Covered by the ZFS snapshots and zrepl replication of `/data/nfs`. The SQLite
database and `app.ini` both live on that volume. To restore: roll back the ZFS
snapshot and restart the deployment.

Note that `/etc/gitea/app.ini` holds `SECRET_KEY` and `INTERNAL_TOKEN`, generated
on first start. Restoring the data volume without the matching config volume
invalidates sessions and stored credentials.
