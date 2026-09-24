# Frontends

Configuration of my internet-facing OpenBSD frontends (blowfish,
fishfinger). It is deployed with Gonf from `../gonf` (package
`gonf/frontends`, tasks `frontends_*`); this directory still holds the
assets those tasks install (`etc/`, `scripts/`, `var/`). The former Rex
`Rexfile` and its Perl `.tpl` templates were retired (conf task v42; see
git history): Gonf is the only configuration owner.

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

1. Controller inputs: the secrets under `gonf/secrets/frontends` (see
   `gonf/secrets/README.md`; the NSD TSIG key is required, the goprecords
   token optional) and the controller checkouts `~/git/shuriken.sh`
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

Uptimed stats are pushed once per day from `/etc/daily.local` by
`/usr/local/bin/goprecords-upload-client.sh` (task `frontends_goprecords`).
The per-host bearer token is a controller secret,
`gonf/secrets/frontends/etc/goprecords/<host>.token` (one line), installed
as `/etc/goprecords-upload.token` (0600). A host without its token gets no
uploader.

Issue or rotate keys on the goprecords daemon (Kubernetes example):

```bash
kubectl exec -n services deployment/goprecords -- \
  goprecords --create-client-key fishfinger -stats-dir=/data/stats
kubectl exec -n services deployment/goprecords -- \
  goprecords --create-client-key blowfish -stats-dir=/data/stats
```

Then update the matching token file and run `frontends_goprecords` again.
