# Garage private admin and authenticated metrics (cn3)

Prepared and reviewed on 2026-10-23; **deployed with explicit approval on
2026-10-24**. Future live changes require their own authorization. Credential
provisioning must precede publication and the Prometheus rollout.

## Production acceptance (2026-10-24)

The dedicated `Infra/garage-metrics` entry was created after actual vault
identity checks, then enrolled into the external `monitoring/garage-metrics`
Secret. Exact readback and subsequent read-only reuse checks passed without
displaying values or writing a metrics fallback file. Source `a4217ed` was
published to GitHub and Forgejo after enrollment. The selected Prometheus
Application diff added only the Secret mount; its documented bootstrap apply
was required because publication alone does not update the live valuesObject.
Argo completed successfully and Prometheus is Ready with the new mount/job
credential-file reference. Its Bound PVC UID is unchanged.

Garage was deployed **f2, then f1, then f0**, with exact root-private TOML
backups and independent 15-minute rollback guards. Each guard was accepted
before changing another node. All three passed:

- Native WireGuard-only listener, local non-WG/LAN refusal, TOML mode
  0640 root:garage, matching metrics credential, and identical parsed
  non-admin settings, including RPC identity and storage configuration.
- Unauthenticated health 200, missing/wrong metrics credentials 403, all three
  cluster nodes healthy, layout version 1 and unchanged Garage 2.4.1/RF3.
- Actual Gogios health plugin success and a real authenticated Prometheus
  scrape newer than that host's completed restart. Initial accepted scrapes
  were f2 07:42:20Z, f1 07:44:02Z and f0 07:45:32Z.
- Trusted external S3 authentication: anonymous listing denied 403,
  authenticated listing returned valid S3 XML, and an existing object read
  returned 200. No S3 object, bucket or credential was changed.

F1 briefly lacked a peer's descriptive metadata immediately after restart;
the next node was held until its full three-node view converged. No extra
restart or weakened health assertion was used. Actual reapplication on all
three nodes converged with zero changes and no additional restart. The active
frontend's normal 07:46 UTC Gogios report has all three Garage health checks OK.
Private backups and the syntax-checked rollback script remain in
`/root/cn3-garage-20261024` on each
node; accepted guards do not restore the old configuration. Controller-side
acceptance observations are private artifacts under the evidence directory
listed below. The unrelated ho3 cached certificate alert is tracked separately.

## Scope and baseline before rollout

Garage runs on f0/f1/f2. Their approved rm3 firewall rollout is complete; fresh
read-only checks confirm PF enabled, reviewed configuration and successful-load
checksums matching, active default deny and WireGuard access. F3 is not a Garage
member. Its pending PF work and ao3's NFS failover repair do not block this
local-ZFS Garage change; neither parent task is declared complete.

All three native Garage binaries are `cargo:2.4.1`. Selective configuration
reads show admin `0.0.0.0:3903`, no metrics credential and no required-token flag.
Their health endpoints return 200 through the frontend WireGuard path. Direct
workstation WG probes were unavailable and are not evidence of a Garage outage.
Prometheus runs 3.15.0; its current additional scrape targets already use the
three Garage WireGuard addresses. Frontend unauthenticated metrics requests
return 200 on all three nodes. The existing frontend WireGuard read proxy
confirms all three live Garage targets up, with successful scrapes and no errors
at 20:41 UTC. An earlier Kubernetes service-proxy query failed and was not used
as the target-health oracle. The installed CRD confirms Secret mounts at
`/etc/prometheus/secrets/<name>`.

The candidate:

- Binds each admin listener to its own `192.168.2.13N:3903`, from typed Garage
  host data in cluster inventory. Wildcard/public/malformed addresses fail
  recording before any SSH connection.
- Sets `metrics_require_token = true` and reads `garage/metrics_token` with
  MustSecret from `Infra/garage-metrics`, Password field. Empty or unsafe
  header values refuse recording. The TOML stays 0640 root:garage and explicitly
  secret-bearing. RPC identity, RPC/S3 addresses, replication, local data and
  layout configuration retain their existing values.
- Adds the external `monitoring/garage-metrics` Secret mount to both Prometheus
  values sets. Both public scrape-config variants reference its `token` file
  through `authorization.credentials_file`; no credential value enters Git.
- Provides `scripts/provision-metrics-secret.py`. Default mode checks only;
  `--apply` creates an absent Kubernetes Secret from an exact machine vault
  read. It refuses an existing different value or unexpected identity/type,
  never rotates a credential, and suppresses child/API credential-bearing
  output. Bytes pass in memory/stdin, not arguments or environment variables.
- Makes `just deploy HOST` act on one allowlisted node. Health/quorum acceptance
  is required before invoking it for the next host. The former all-node
  convenience target is not used for this cutover.

Before enrollment, the exact vault machine read returned exit 4
(reference/field not available) and the Kubernetes Secret was absent. The
RPC fallback initializer remains a legacy helper and must not run during this
rollout; it does not provision the metrics credential.

## Verification performed

Fourteen focused tests passed with zero skips:

- Six real recipe/daemon tests: sensitive plan and ownership, invalid/absent
  credentials, invalid listeners, real Garage metrics/auth/health/admin-scope
  behavior, required-token behavior with no configured token, and a real
  Prometheus scrape using a credential file.
- Six provisioning boundary tests use native kubectl with a disposable HTTP
  API and fake credential provider: read-only default, create/reuse, refusal
  to rotate or reuse the wrong Secret type, invalid header values, partial
  provider failure and API error redaction. No real vault/cluster was written.
- Two tests render pinned chart 92.10.1 with both values sets and run native
  Prometheus 3.15.0 promtool on both scrape configurations.

The real local Garage 2.4.1 returns **403** for missing/wrong metrics credentials,
200 with the correct token, and 200 for unauthenticated health. The metrics
credential receives 403 from a real admin endpoint; a separate disposable
admin credential receives 200 there, ruling out a nonexistent-route false pass.
The explicit listener rejects connections to another loopback address. These
are release-matched Linux fixtures, not completed FreeBSD rollout acceptance.
Only fixture storage/listeners and replication factor are adapted to a private
one-node test cluster; production retains replication factor 3.

Go generation/build/test/vet/errcheck and fixture errcheck passed. Justfile dry
runs and whitespace checks passed. Private artifacts and sanitized observations
are outside Git under
`/home/paul/.local/state/conf-worktrees/cn3-garage-metrics-20261023/evidence/`.
The native fixture binary was extracted from the official Garage 2.4.1 image;
a preliminary 2.3.0 image pull was not used after native inspection established
the installed release. No Garage package upgrade is included.

## Approved rollout procedure

1. Confirm the intended Kubernetes context and healthy current Prometheus,
   all three Garage nodes/layout, S3 read access, the three Garage Gogios
   checks, and their PF rules. Exclude concurrent package or storage maintenance.
   Keep current versions, RPC identity, buckets/keys, data/layout, SSH and
   workload placement unchanged. No failover, remount, reboot or power action.
2. Create/reuse the dedicated vault credential only after approval. Verify the
   canonical unique entry/group identity before writing; generate 32 random
   bytes encoded as 64 hex characters if absent. Preserve all other fields
   and existing credentials. Never use the RPC or S3 key as the metrics token.
3. Run the helper with `--apply`, then without it; require the existing
   Kubernetes Secret to match the vault before publication. Stop on mismatches,
   missing fields or any ambiguous/failed operation. Do not overwrite a
   different existing Secret or rotate credentials to get past an error.
4. Integrate and publish only reviewed task paths to GitHub/Forgejo, preserving
   other prepared/user changes. Argo will restart the singleton Prometheus pod
   for the Secret mount. Wait for Ready, the mounted credential and loaded
   file-based scrape configuration, and all three Garage targets up. At this
   point the old Garage nodes still accept metrics; adding the header first
   avoids a credential transition outage. Preserve existing receiver/admin/
   OTLP controls, PVC and historical data. Stop before Garage changes on failure.
5. Deploy f2, then f1, then f0, **one explicit host invocation at a time**. Before
   each, recheck all three nodes connected/healthy, correct WG address present,
   current PF and no competing maintenance. Compare the existing RPC secret
   with its authoritative vault value privately; emit a match boolean only.
   Save the exact TOML/metadata in a unique root-only local backup directory,
   with no NFS copy. Require a strict preview to show only intended TOML and
   conditional Garage restart. Check no unexpected identity/storage drift.
6. Apply only `garage_config` to that host (`just deploy fN` uses strict SSH,
   port 22 and doas). Require Garage running and local/WG health 200, all three
   cluster nodes fully healthy/connected, layout unchanged, normal S3 reads,
   Prometheus target up with a successful scrape later than this restart, and
   its Garage Gogios check OK before the next node.
   Require unauthenticated/wrong-token metrics 403 and LAN/non-WG admin
   connections refused; Prometheus's real successful scrape proves authorized
   access without printing/copying its token. Keep credentials out of process
   arguments and logs. Do not silence unrelated existing monitoring failures.
7. Reapply each selected task and require convergence without another restart.
   Recheck S3, all three metrics targets, health/Gogios checks and external
   admin denial. Record actual results and any boot/availability limits; task
   cn3 remains open until its required production acceptance passes.

An error/timeout is an uncertain outcome: inspect state and stop, rather than
blindly retrying or proceeding to another host. The review/approval request
must include credential provisioning, publication/Argo Prometheus restart,
serial Garage restarts and the exact tests. This scope was approved on
2026-10-24; it does not authorize unrelated future changes.

## Rollback

Restore only the affected host's saved TOML with its original ownership/mode
and restart that Garage service once; require full cluster/health/S3 recovery
before any other action. Never restore old data/layout or rotate the shared
RPC identity. Keep Prometheus authenticated while any updated Garage node
requires a token. Before removing Prometheus authentication, restore all changed
Garage TOMLs one node at a time with full health/quorum gates. Then revert this
task's values/scrape fields to their recorded prior published versions, keeping
the PVC and receiver controls; its pod may restart again. Keep the new external Secret and
vault entry unused for later review rather than deleting credentials. Verify
monitoring after rollback and stop before broadening the recovery.

References: [Garage 2.4.1 configuration](https://github.com/deuxfleurs-org/garage/blob/v2.4.1/doc/book/reference-manual/configuration.md),
[Prometheus authorization configuration](https://prometheus.io/docs/prometheus/latest/configuration/#http_config).
