# Foostats private visitor statistics (task an3)

The reviewed privacy change is deployed on both OpenBSD frontends. See the
[live acceptance record](foostats-privacy-acceptance-20261023.md) for source
pins, retained-history coverage, actual private WireGuard replication and
scoped archive cleanup. The user approved keeping reports and applying this
fix after 9n3's authenticated publishing and nonroot mirror rollout completed.
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
the old public tree. A refreshed inspection of fishfinger on 2026-10-23
found 2,887 gzip archives: 2,732 matching the initial protocol/date/hostname
pattern, 93 with underscore hostname labels (18 Gemini and 75 web), and
62 with the older hostname prefix (31 each Gemini and web). All four missed
groups have the legacy `count`/`feed_ips`/`page_ips` source schema; dictionary
shape checks confirm visitor-hash sets without recording their values.
The historical source hashes IPs with unsalted SHA3-512 and places those
hashes in the feed/page dictionaries. Empty dictionaries in some archives
do not change their classification as legacy visitor data.
Therefore cleanup does **not** remove the whole tree.
The by-name Operational task `frontends_foostats_purge_public` depends on
the new Foostats configuration and unlinks only regular files/symlinks named
`<protocol>_YYYYMMDD.<host>.json.gz` or
`<host>.<protocol>_YYYYMMDD.json.gz`, including their `.tmp` leftovers.
Protocol is exactly `web` or `gemini`; hostname labels contain letters,
digits, underscores or hyphens, separated by single dots. No broad gzip
wildcard is used. It does not follow directory symlinks and refuses
a symlink root. Other files and aggregate reports survive. Refresh the
inventory on both hosts before approval; unexpected per-visitor files need
explicit inspection and a reviewed cleanup update.

## Approved rollout sequence

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

Read-only native preflight found `Digest::HMAC` missing on both frontends.
The new private directory and key file are absent on both; their existing
public archive counts on 2026-10-22 were 2,728 on blowfish and 2,883 on
fishfinger; the refreshed fishfinger count above supersedes its earlier
count and identifies historical filename forms that must also be purged. The
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
a temporary fixture. Both historical filename forms and `.tmp` leftovers
are covered. Near misses, unrelated gzip files, aggregate pages, matching
directories and symlink targets outside the public tree are preserved.
The old purge failed all twelve added regular-file fixtures and the added
hostname-prefix symlink; the corrected command passes the same checks.
No `*_test.go` is added to conf. These checks do not prove native OpenBSD
permissions, real WG source identity, production deletion or live report
acceptance; those remain gates after approval.
