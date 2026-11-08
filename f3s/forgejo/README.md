# Forgejo

The git forge, `https://code.f3s.buetow.org`, namespace `services`. It also
serves `snonux/conf` to ArgoCD
(`http://forgejo.services.svc.cluster.local/snonux/conf.git`, anonymous), so
every other Application depends on it.

```
Internet -> relayd (OpenBSD, TLS) -> WireGuard -> Traefik -> svc forgejo:80 -> pod:3000
git+ssh  -> relayd :2022 (TCP) ----> NodePort 30222 -------------------------> pod:2222
```

- Image `codeberg.org/forgejo/forgejo:17.0.1-rootless`: UID/GID 1000, all
  capabilities dropped.
- SQLite, one writer (`replicas: 1`, strategy `Recreate`). The DB sits on
  the NFS data volume, so WAL is off: `SQLITE_JOURNAL_MODE=DELETE`, 60 s busy
  timeout. If `locking protocol` errors come back, move the DB to local block
  storage or PostgreSQL.
- Volumes: `/var/lib/gitea` (repos, DB, active config and SSH keys) and
  retained `/etc/gitea` mount, both
  guarded by `.nfs-sentinel` (see
  [`../docs/nfs-sentinel-initcontainer.md`](../docs/nfs-sentinel-initcontainer.md)).
- Installer locked (`INSTALL_LOCK=true`), registration disabled.

17.0.0 was held because of upstream Git regressions. Version 17.0.1 includes
the follow-up Git fixes and is deployed (verified 2026-11-08). The old hold
must not be interpreted as a reason to deploy 17.0.0.

The active configuration is `/var/lib/gitea/custom/conf/app.ini`
(`data/custom/conf/app.ini` on NFS), not the retained `/etc/gitea` mount.
SSH host keys live under `data/ssh`. The old and new rootless images use the
same paths; no configuration was moved during the upgrade.

## First install

1. Storage, on the CARP storage master (`ifconfig | grep MASTER` on f0/f1).
   The PVs are `type: Directory`, so the pod won't schedule without them:

   ```sh
   doas mkdir -p /data/nfs/k3svolumes/forgejo/data /data/nfs/k3svolumes/forgejo/config
   doas touch    /data/nfs/k3svolumes/forgejo/data/.nfs-sentinel \
                 /data/nfs/k3svolumes/forgejo/config/.nfs-sentinel
   doas chown -R 1000:1000 /data/nfs/k3svolumes/forgejo
   doas chmod 0750 /data/nfs/k3svolumes/forgejo/data /data/nfs/k3svolumes/forgejo/config
   doas chmod 0644 /data/nfs/k3svolumes/forgejo/*/.nfs-sentinel
   ```

   Keep the sentinels 0644; don't sweep them into a recursive chmod.

2. Hostname: `code.f3s.buetow.org` must be in `f3sHosts` in
   `gonf/frontends/data.go` (drives DNS, relayd, ACME and gogios checks; its
   HTTPS relay port is in `siteHTTPSPorts`). relayd needs the keypair at
   startup, so deploy the cert first, one gateway at a time:

   ```sh
   ./gonf.sh push -- -p 2 rex@blowfish.buetow.org frontends_nsd frontends_httpd frontends_acme frontends_acme_invoke frontends_relayd frontends_gogios
   ./gonf.sh push -- -p 2 rex@fishfinger.buetow.org frontends_nsd frontends_httpd frontends_acme frontends_acme_invoke frontends_relayd frontends_gogios
   ```

   The standby gateway keeps the `foo.zone` placeholder cert until it takes
   over the DNS master IP. That's expected.

3. `kubectl apply -f ../argocd-apps/services/forgejo.yaml` (no app-of-apps;
   later chart edits auto-sync).
4. `just create-admin` (defaults `username=paul`; prints a random password
   once).

## Clone URLs

```sh
git clone https://code.f3s.buetow.org/<user>/<repo>.git
git clone ssh://git@code.f3s.buetow.org:2022/<user>/<repo>.git   # via relayd
git clone ssh://git@r0.lan.buetow.org:30222/<user>/<repo>.git    # LAN, direct NodePort
```

SSH is on 2022 because 22 draws scanners and 2222 is dserver on the gateways.
Gateway admin SSH stays on port 2. For the short form:

```
Host code.f3s.buetow.org
  Port 2022
  User git
```

## Operate

```sh
just status          # pods, svc forgejo/forgejo-ssh, ingresses, PVCs, ArgoCD
just logs [lines]
just restart
just shell
just port-forward    # localhost:3000
just sync            # ArgoCD refresh
```

## Backup and restore

Before the 17.0.1 migration, the writer was stopped and f0's snapshot
`zdata/enc/nfsdata@j34-before-forgejo17-20261108` was held with tag
`j34-forgejo-upgrade`, covering the complete Forgejo tree. A private archive
of the active configuration, database and SSH keys was downloaded and
SHA-256 checked under `~/.local/state/conf/j34-app-upgrades-20261107/`.

For rollback, stop writes and preserve any post-upgrade changes first.
Restore only the selected Forgejo tree from the snapshot with the previous
16.0.6 image; a binary-only downgrade cannot undo a database migration.
Never roll back the shared NFS dataset for one application. Keep data,
configuration and keys consistent when restoring.

After migration: all 96 repositories and their Git refs, selected database
row counts, configuration secrets and SSH host keys are unchanged. SQLite
quick_check is clean and journaling remains DELETE. The exact image config
ID, CLI/API health/version, anonymous Git HEAD through ArgoCD's repo-server,
and authenticated Git HEAD from the existing Rocky SSH identity all passed.
A fresh ArgoCD Git comparison also passed. No Git push was performed.
Automatic sync is held in the live and local Application until the matching
17.0.1 chart is published; do not reconcile from the old remote source.

The retired cgit git-server's data at `/data/nfs/k3svolumes/git-server` and
its final ZFS snapshot are still on disk; delete only with explicit
confirmation. There is no git-server fallback any more.
