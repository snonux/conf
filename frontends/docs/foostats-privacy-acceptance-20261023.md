# Foostats privacy rollout — 2026-10-23

The user approved keeping reports and applying the reviewed privacy fix.
This rollout uses conf `563ff38` and Foostats `1d1bdf1`; the integrated
published conf source is byte-equivalent for these resources. Only the named
`frontends_foostats` and Operational `frontends_foostats_purge_public` tasks,
with their existing rsync prerequisites, were applied serially. No frontend
aggregate, Gogios, web, ACME, firewall, SSH, storage, power or reboot task was
deployed.

## Provisioning and deployment

- Verified the actual KeePass group structure and absent exact entry identity
  before adding `Infra/foostats-ip-hmac`. An encrypted vault backup is 0600;
  a new 32-byte random key encoded as 64 hex characters entered only via stdin.
  Exact readback and both installed keys agree, without printing values.
- Both frontend scripts match the reviewed app bytes. Native HMAC 1.05 loading
  and Perl syntax pass on both nodes.
- `/etc/foostats` is root:wheel 0700; its key is root:wheel 0600.
  `/var/db/foostats` is root:www 0750, outside the HTTPD chroot. All 44 private
  archives on each frontend are root:www 0640 with matching scheme/key metadata.
- Existing rsync module sections are byte-preserved after excluding the added
  private module. Publishing credentials, mirror script/ownership/crons, the
  570-check Gogios configuration, HTTPD, relayd, ACME, inetd and PF match their
  private pre-rollout baselines.
- Private rollback material and command logs remain only under
  `/root/an3-visitor-privacy-20261023` on each frontend, mode 0700. Controller
  evidence contains operational scripts and summarized counts, booleans, dates
  and modes; no visitor datasets were copied to controller files.

## Private transfer and reports

Both native private modules are hidden from module listings, reject peer uploads
as read-only, and do not export the key. Four real connections from the unlisted
r1 WireGuard source were refused at module selection across both frontends.
r1 has no rsync installation; stock Python sent the native greeting/module
exchange and checked its error, without installing a package or receiving data.

The installed daily Foostats command was invoked manually on each frontend,
including a successful subsequent run after public archive cleanup. These are
manual tests of the normal installed entrypoint. No natural scheduled daily tick
occurred during this rollout or is claimed as evidence. `/etc/daily` explicitly
sets umask 022 on both frontends.

Actual peer pulls use wg0. Native comparisons against each peer's exported
source validate all cached peer archives: 27 on blowfish and 15 on fishfinger.
Their decoded counter structures and private metadata agree; gzip/JSON
serialization differences are not treated as counter differences. Archive bytes
remain on frontend private paths, and no temporary replicas remain. The live merge oracle checked exact request sums and host
visitor unions on 15 days containing both sources, with 195 host-count checks
and 18 cross-source visitor groups deduplicated on each frontend.

The public artifact scans cover 2,138 HTML/Gemtext files on blowfish and 2,118
on fishfinger. They contain no installed key, private metadata or known private
visitor tags, and remain publicly readable. HTTP report health passes all three
aliases over IPv4 and IPv6 on both nodes. Native Gemini `stats.foo.zone` returns
20 with report content over both families on both nodes. The optional Gemini
`standby.stats.foo.zone` alias also works on fishfinger; it returns 51 on
blowfish, whose separate alias tree is absent. This task does not declare or
remove that alias and did not change Gemini routing.

Strict TLS/content/HSTS checks passed for 28 alias/address-family probes
per frontend, including the previously approved ro3 and qo3 certificates.

## Scoped cleanup

Immediately before deletion, the reviewed matcher found exactly 2,732 archives
on blowfish and 2,887 on fishfinger, including 93 underscore-host and 62
hostname-prefix historical variants. There were no unexpected files, other gzip
files, matching directories or matching symlinks. The Operational task removed
exactly these 5,619 archives, after the report/replication gates passed.

Both nodes retain 46 aggregate `.gmi` files and two filter text files, byte for
byte. Post-cleanup inventories contain zero matching archives, other gzip or
unexpected files. The subsequent normal reporting runs do not recreate public
visitor archives. Older daily report byte manifests (951 files on blowfish,
952 on fishfinger, earlier than 2026-09-27) are preserved.

Old direct URLs are denied on each node's own FQDN and five shared aliases over
both address families: 12 representative URL probes on blowfish and 36 on
fishfinger, covering the available historical filename forms. Another 12
listing probes per frontend contain no visitor archives. Strict TLS verification
is retained; the opposite frontend's distinct FQDN is not used with a mismatched
server certificate. No raw archive republication or broad gzip deletion occurred.

## History and operational limits

| Original retained-log rebuild | Web dates | Gemini dates |
|---|---|---|
| blowfish | 2026-10-08–2026-10-23, 16 days | 2026-10-23 only |
| fishfinger | 2026-09-27–2026-10-23, 27 days | 2026-10-22–2026-10-23 |

The new rolling 30-day summary has 26 completed web dates through October 22
and one completed Gemini date (October 22). October 23 is partial private input;
normal reporting/replication includes it only once it becomes a completed date.
Four dates in the current 30-day web window, September 23–26, have no rebuilt
web input. Earlier aggregate pages survive, but missing original-IP history was
not reconstructed from unsalted hashes. Historical unique-visitor sets therefore
cannot be carried into the new keyed identity space.

Fishfinger's first cold scan exceeded the controller's 900-second wait. The
original remote process continued and completed; its final current-day archive,
metadata, modes and subsequent successful normal entrypoint runs were verified.
The original timed-out controller invocation is not represented as a successful
zero-status command.

The first manual reporting invocation incorrectly inherited the operator's
umask 077 intended for its private log. The pre-purge HTTP gate detected the
resulting unreadable aggregate outputs. Exactly eight newly generated regular
report files per frontend were restored from 0600 to 0644 using owner, filename
and current-run mtime guards; no broad chmod or private key/counter mode change
occurred. The runner now protects the log outside an umask-022 reporting
subshell. Corrected reruns and the subsequent post-purge runs use the verified
normal `/etc/daily` semantics. No product source change was needed.

At the 16:15 UTC normal monitoring cycle, both frontends retained 570 checks;
all 29 Foostats/strict-legacy checks were OK, and 18 own Local checks on each
node had advanced through ordinary cycles. The cached operator-induced HTTPS
warnings cleared. The baseline standby code-certificate critical and
KubeJobFailed warning remain. A generic fishfinger load warning also remained
following the sustained cold rebuild and manual verification workload. At
16:17 UTC its native one/five/fifteen-minute loads were 0.44/1.27/1.42 against
unchanged warning thresholds 2/1/1; the longer averages were still settling.
This transient is recorded separately from the passing report, privacy,
replication and service checks.
