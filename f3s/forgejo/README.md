# Forgejo

The git forge, `https://code.f3s.buetow.org`, namespace `services`. It also
serves `snonux/conf` to ArgoCD
(`http://forgejo.services.svc.cluster.local/snonux/conf.git`, anonymous), so
every other Application depends on it.

```
Internet -> relayd (OpenBSD, TLS) -> WireGuard -> Traefik -> svc forgejo:80 -> pod:3000
git+ssh  -> relayd :2022 (TCP) ----> NodePort 30222 -------------------------> pod:2222
```

- Image `codeberg.org/forgejo/forgejo:16.0.1-rootless`: UID/GID 1000, all
  capabilities dropped.
- SQLite, one writer (`replicas: 1`, strategy `Recreate`). The DB sits on
  the NFS data volume, so WAL is off: `SQLITE_JOURNAL_MODE=DELETE`, 60 s busy
  timeout. If `locking protocol` errors come back, move the DB to local block
  storage or PostgreSQL.
- Volumes: `/var/lib/gitea` (repos, DB) and `/etc/gitea` (`app.ini`), both
  guarded by `.nfs-sentinel` (see
  [`../docs/nfs-sentinel-initcontainer.md`](../docs/nfs-sentinel-initcontainer.md)).
- Installer locked (`INSTALL_LOCK=true`), registration disabled.

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

ZFS snapshots and zrepl replication of `/data/nfs` (`zdata/enc/nfsdata`)
cover repos, DB and `app.ini`. Restore: roll back the snapshot, restart the
deployment. `app.ini` holds `SECRET_KEY` and `INTERNAL_TOKEN`; restore data
and config together or sessions and stored credentials break.

The retired cgit git-server's data at `/data/nfs/k3svolumes/git-server` and
its final ZFS snapshot are still on disk; delete only with explicit
confirmation. There is no git-server fallback any more.
