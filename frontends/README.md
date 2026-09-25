# Frontends

The internet-facing OpenBSD frontends blowfish and fishfinger (SSH
`rex@<host>.buetow.org` port 2, doas). Deployed with gonf from `../gonf`
(package `gonf/frontends`, tasks `frontends_*`); this directory holds the
assets those tasks install (`etc/`, `scripts/`, `var/`). Routing, TLS and
template rules: [`AGENTS.md`](AGENTS.md). Unattended upgrades for every OS:
[`docs/unattended-upgrades.md`](docs/unattended-upgrades.md).

```sh
./gonf.sh -list | grep frontends                 # from the repository root
./gonf.sh cluster frontends frontends            # setup aggregate
./gonf.sh push -n -- -p 2 rex@blowfish.buetow.org frontends_httpd   # preview one task
```

A preview (`-n`) applies nothing, but it still installs or updates the gonf
binary on the destination when that one is missing or too old.

The `frontends` aggregate installs every setup task. Three tasks are
deliberately outside it and only run by name: `frontends_acme_invoke`
(requests certificates), `frontends_irc_bouncer` (ZNC on fishfinger) and
`frontends_ping` (push-pipeline diagnostic). Gorum stays disabled (only its
service account exists, `frontends_service_accounts`).

## First install on a new frontend

The aggregate converges an existing frontend. A new host additionally needs,
in this order:

1. Controller inputs: the secrets, read from the foostore/KeePass vault
   with a file fallback under `gonf/secrets/frontends` (see
   `gonf/secrets/README.md`; the NSD TSIG key is required, the goprecords
   token optional and needs a vault entry or token file for the new host) and the controller checkouts `~/git/shuriken.sh`
   (`check_shuriken_age` for Gogios, required) and `~/git/foostats`
   (optional; `scripts/` holds a fallback copy). `GONF_SHURIKEN_ROOT` and
   `GONF_FOOSTATS_ROOT` point elsewhere.
2. `frontends_pkg_repo`, `frontends_base`: packages, the signed package
   repository and `pkg_scripts`. DTail and Gogios install from that
   repository (their tasks also pass `PKG_PATH` themselves).
3. Certificates: relayd (`frontends_relayd`) and smtpd (`frontends_smtpd`)
   validate against the keypairs in `/etc/ssl`, which only exist after
   `frontends_acme` and one explicit `frontends_acme_invoke` run (the renewal
   script copies `foo.zone` placeholders for names this host does not serve,
   so `/etc/ssl/foo.zone.*` must be present first). httpd must already serve
   the ACME challenge (`frontends_httpd`).
4. The rest of the aggregate. The unattended-upgrade cron job needs
   `frontends_script` and `frontends_services`; the task descriptions name
   such prerequisites.

## goprecords upload

Uptimed stats are pushed hourly (`15 * * * *`, root crontab entry
`goprecords-upload`) by `/usr/local/bin/goprecords-upload-client.sh` (task
`frontends_goprecords`); its output goes to syslog with tag
`goprecords-upload` (`/var/log/messages`). It used to run once a day from
`/etc/daily.local` at 01:30, which falls into the nightly f3s power-off:
relayd then falls back to the local httpd, the PUT gets 405, and no upload
landed from 2026-08-15 to 2026-09-25 (task xk2). A 405 in the log therefore
means "f3s was down at that hour", not a token problem (that is 401/403).
The per-host bearer token is a controller secret, logical reference
`frontends/etc/goprecords/<host>.token` (one line): for blowfish and
fishfinger it is read from the vault entry `Infra/goprecords-token-<host>`
(Password field), any other host falls back to the file of that name under
`gonf/secrets/` (see `gonf/secrets/README.md`). It is installed as
`/etc/goprecords-upload.token` (0600). For a host without a token gonf
declares nothing (no uploader; files already there are left as they are).

Issue or rotate keys on the goprecords daemon (Kubernetes example):

```bash
kubectl exec -n services deployment/goprecords -- \
  goprecords --create-client-key fishfinger -stats-dir=/data/stats
kubectl exec -n services deployment/goprecords -- \
  goprecords --create-client-key blowfish -stats-dir=/data/stats
```

Then update the host's vault entry (or, for a host without one, its token
file) and run `frontends_goprecords` again.
