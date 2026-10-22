# Foostats private visitor statistics (task an3)

Local preparation only. No production key, counter directory, export, daily
hook or public archive has been changed. Task an3 requires its own reviewed
rollout approval. It is composed on the prepared 9n3 branch, whose authenticated
album publishing and Joern ownership migration must be completed first.
Neither task authorizes PF, SSH, HTTPD, CARP, NFS or recovery-mode changes.

## Source and data flow

The deployed source is the separate `foostats` checkout, selected by
`GONF_FOOSTATS_ROOT` (default `~/git/foostats`). The legacy copy under
`frontends/scripts` is marked historical and is no longer a fallback.
Recording refuses a canonical source that lacks private-statistics support.
Review and deploy the paired source commits together; using an old script
with the new directories is not a supported partial rollout.

The Foostats source uses `Digest::HMAC` with SHA3-512 and the explicit
72-byte block length. Both nodes receive the same random hex key from
foostore entry `Infra/foostats-ip-hmac`, through
`paths.FrontendSecret("etc/foostats/ip-hmac.key")`. The key is root-owned
0600 under root-private `/etc/foostats`, outside all exported directories.
Missing, malformed, public or symlink keys fail closed, without printing
their values. No production entry has been created during preparation.

Counters move from the public `/var/www/htdocs/buetow.org/self/foostats` to
`/var/db/foostats`, mode 0750 root:www. Archives use mode 0640 and their
parent's group. Standalone creation uses 0700; world access, group write,
wrong directory ownership and a symlink directory are refused. The httpd
worker is chrooted under `/var/www` and cannot reach this private path.

The hidden read-only `foostats-private` rsync module exposes that directory
only to exact frontend WG addresses and loopback administration, not r-nodes
or arbitrary WG peers. Existing numeric access rules and album authentication
from 9n3 are preserved. This module uses the WireGuard peer boundary, with no
new password shared with Shuriken. It does not export the HMAC key.
Rsync is started by existing inetd per connection; no new listener/port,
daemon restart or HTTPD route is needed.

Each daily command pins the other frontend's WG IPv4 and filename hostname.
Replication has no public HTTPS fallback, uses argument-list execution and
bounded rsync timeouts, validates the scheme/key identity on the downloaded
temporary archive, and atomically replaces a good replica only after
validation. Both nodes' same-visitor sets merge to one unique visitor.
Different keys and legacy archives are refused rather than silently changing
the counts. Only merged aggregate HTML/Gemtext reports remain public at
`stats.foo.zone`; private identities and dataset metadata do not enter them.

## History and cleanup

Direct HMACs cannot be reconstructed from legacy unsalted hashes without
the original IP addresses. Start a fresh private dataset and rebuild from
retained raw logs on each host. Do not import old archives as keyed data.
The new rolling summary covers the rebuilt data, which may initially cover
less than thirty days. Existing aggregate pages for dates with no private
inputs are preserved. Review this coverage during approval and record it in
the live acceptance; do not claim unavailable history was reconstructed.

Read-only inspection of blowfish on 2026-10-22 found 2,728 raw gzip archive
names, 46 aggregate `.gmi` files and two filter-configuration text files in
the old public tree. Therefore cleanup does **not** remove the whole tree.
The by-name Operational task `frontends_foostats_purge_public` depends on
the new Foostats configuration and unlinks only regular files/symlinks named
`web_YYYYMMDD.<host>.json.gz` or `gemini_YYYYMMDD.<host>.json.gz`, including
their `.tmp` leftovers. It does not follow directory symlinks and refuses
a symlink root. Other files and aggregate reports survive. Refresh the
inventory on both hosts before approval; unexpected per-visitor files need
explicit inspection and a reviewed cleanup update.

## Approved rollout sequence (not executed)

1. Finish 9n3 review and its approved rollout. Review the paired an3 Foostats
   and conf commits and obtain explicit approval for key provisioning,
   private data rebuild, scoped frontend deployment and archive purge.
   Capture current public report health and archive-name counts on both
   hosts without printing visitor counters or keys.
2. Provision the shared high-entropy key in foostore if absent; validate an
   existing entry rather than silently overwrite it. Use the same reviewed
   source checkout on both deployments, with `GONF_FOOSTATS_ROOT` explicit
   until the canonical source branch is integrated. Keep secret values out
   of logs/arguments/environment.
3. Inspect a fresh `frontends_foostats` plan. Its prerequisites are 9n3 rsync
   resources/account and the required Perl packages; its own resources are
   the private config/key/counters, reporting directories, script, keyed
   daily hook and existing log rotation. No frontend aggregate or unrelated
   network/service task is authorized. Deploy one host at a time.
4. Run the new parser on retained logs in the new directory, verify both
   scheme/key identities agree without exposing values, and confirm actual
   peer replication/unique-count merging in both directions. Check private
   modes and public aggregate content. Preserve earlier aggregate report
   pages and record the rebuilt time window. Do not purge legacy archives
   until both nodes work and the new reports are usable.
5. After the explicit purge approval, run only
   `frontends_foostats_purge_public` on each host. It is not included in a
   normal aggregate. Verify old direct archive URLs fail and directory
   listings no longer contain visitor archives; keep existing aggregate
   pages/filter files. Check aliases and both address families. Do not
   delete unrelated files or backups.
6. Test that an ordinary unlisted source cannot list/read the private module
   and that the peer cannot write it. Confirm no key is in the export and
   no identity is in generated public artifacts. Verify normal subsequent
   daily reporting and monitoring before completing an3.

If a gate fails, stop the new reporting hook and retain private data for
diagnosis. Do not roll back by republishing unsalted visitor archives or
re-enabling public HTTP replication. Restoring old public raw files would
undo the privacy fix and needs a separate explicit decision.

## Local checks and limits

Read-only native preflight found `Digest::HMAC` missing on blowfish. The
OpenBSD 7.9 amd64 repository lists `p5-Digest-HMAC-1.05.tgz`; the scoped
recipe declares that dependency. No production package was installed during
preparation, and the new Perl source was not executed on a frontend.

```sh
# In the paired foostats checkout; set PERL5LIB if dependencies are local.
perl -c foostats.pl
prove -I. -lr t

# In conf, pointing at that exact Foostats source:
python3 frontends/scripts/tests/foostats-privacy.py /path/to/reviewed/foostats
cd gonf
go generate ./...
go build -p 1 ./...
go test -p 1 ./...
go vet ./...
errcheck ./...
```

The Perl tests use NIST HMAC vectors, real local counter production, native
loopback rsync, different-key/legacy rejection, two-node deduplication and
generated public artifacts. They also protect old aggregate pages when raw
history is absent. The conf driver records real recipes using dummy secrets,
checks private paths/owners/peer scope, rejects a stale source, and executes
the actual purge command only after replacing its fixed production root with
a temporary fixture. Symlink/outside-file/aggregate preservation is tested.
No `*_test.go` is added to conf. These checks do not prove native OpenBSD
permissions, real WG source identity, production deletion or live report
acceptance; those remain gates after approval.
