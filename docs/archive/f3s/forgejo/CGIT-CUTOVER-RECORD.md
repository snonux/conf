# Forgejo: cgit preseed and cutover record (2026-08)

Moved out of `f3s/forgejo/README.md`.

## Relationship to the retired cgit git-server

Forgejo is now the sole git forge in the cluster. It was preseeded from a
legacy cgit/git-server install (`cicd` namespace, `c-git.f3s.buetow.org`,
`/data/nfs/k3svolumes/git-server/repos`, SSH NodePort 30022) that served as a
14-day read-only rollback fallback after the 2026-08-05 cutover. That fallback
was retired 2026-08-30 (task 3x0) once the retention window passed with a
clean re-verification: its ArgoCD Application, helm chart and ArgoCD SSH
known_hosts/repo-creds were removed from this repo, and its hostnames were
dropped from `@f3s_hosts` in the then-current `frontends/Rexfile` (since
ported to `f3sHosts` in `gonf/frontends/data.go`). Its NFS data directory and
final ZFS snapshot are preserved pending a separate, explicitly-confirmed
deletion. See `f3s/argocd/README.md` for what now stands in for a rollback
path (ZFS/zrepl snapshots of `zdata/enc/nfsdata`, not a second git service).

All 80 cgit repositories were preseeded in Forgejo on 2026-08-04. ArgoCD reads
`conf` anonymously from `http://forgejo.services.svc.cluster.local/snonux/conf.git`.

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
- **Update 2026-08-30 (task 3x0):** the above old-service references were
  intentional only through the end of the 14-day retention window
  (2026-08-19T06:48:19Z). With that window long passed and a fresh
  verification pass clean (all repos present in Forgejo, anonymous clones and
  gitsyncer pushes/description updates working, no live automation left
  pointing at the old server), `f3s/git-server/` and
  `f3s/argocd/git-server-{repo-creds,known-hosts}.yaml` were removed from this
  repo, and `c-git.f3s.buetow.org`/`git.f3s.buetow.org` were dropped from
  the then-current `frontends/Rexfile`. The old NFS data directory and its final ZFS snapshot
  are preserved and untouched pending a separate, explicitly-confirmed
  deletion — see that commit's message for the exact runbook. There is no
  longer a git-server rollback path; if Forgejo itself needs recovery, restore
  from ZFS/zrepl snapshots of `zdata/enc/nfsdata` instead.

