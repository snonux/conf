# Frontend hardening preparation (dn3)

Prepared on 2026-10-23; **not deployed or published**. Task dn3's explicit
live-change approval gate still applies. Its old dependency on tm3 was removed:
the owner skipped SSH authentication changes, and none of these fixes needs
them. This work changes no SSH authentication configuration, keys or doas rules.

## Changes

- Move the cluster mute marker to `/var/db/f3s/monitoring/f3s_taken_down`.
  `/var/db/f3s` is root-owned 0755; `monitoring` is `f3sctl:_gogios` 0750.
  The existing restricted helper writes with umask 027; Gogios can inspect
  the marker but cannot create it. A boot-time rc.local line removes it,
  preserving the old gateway-reboot behavior. The bare SSH verbs remain the
  same. Shell globbing is disabled before parsing the untrusted verb.
- Make `/etc/gogios.json` mode 0640 `root:_gogios` in both the scoped task and
  normal Gogios recipe. Read-only native comparisons confirm the candidate
  content equals each live config apart from its 82 mute-path references.
- Pass the upload authorization header to curl through stdin (`-H @-`),
  keeping the token out of process arguments. Token files, URLs, data formats
  and schedules are preserved. The client uses umask 077 for temporary data.
- Add `znc` to the OpenBSD updater's restart list. Its existing running-service
  check means a stopped or absent ZNC is not started by that list.
- Stop installing NRPE with Gogios; provide an explicit operational
  `frontends_retire_nrpe` task for its final removal after acceptance.
- Update the f3s skill's roaming instructions to ProxyJump with agent forwarding
  disabled, keeping authentication on the originating client.
- Make the active access log and future rotations mode 0640 `root:_dserver`.
  That group preserves DTail's read access without granting write access.
  Existing rotated archives are outside this permission change.

`frontends_low_risk` applies only the marker directory/helper, rc.local boot
cleanup, Gogios configuration and active log/rotation permissions. It does not
install/upgrade Gogios, alter cron, change users or restart daemons.
`frontends_services` publishes only the unattended restart list.
`frontends_retire_nrpe` is kept out of aggregates so removal is a deliberate
last step. Later normal Gogios applies preserve the protected marker and mode.

The paired f3sctl source changes only its `GogiosMuteFile` inventory metadata;
no executable code uses that field to perform muting. Its existing clients
already send the same bare verbs, so no client binary rollout is needed for
this marker cutover. The f3s skill changes live in the separate dotfiles repo.
The historical `setup-f3sctl-agent-openbsd.sh` only receives comment updates;
**do not run that SSH bootstrap** as part of this task.

## Read-only preflight and tests

Both native frontends have the existing f3sctl, _gogios and _dserver accounts,
no old mute marker and no new directory. Their config is 0744 root:wheel and
active log 0644 root:daemon. Actual credential-dropped processes confirm
_dserver can currently read the log. NRPE is installed on both, with no daemon,
listener, reverse package dependencies, or references in the inspected
inetd/rc/Gogios/root-cron/Gogios-cron configurations. These are bounded checks,
not a claim about arbitrary custom jobs. Both have curl 8.22.0.

Checks actually run:

- Five real Unix permission/helper tests, including denied writes/reads,
  invalid commands and wildcard expansion, plus one real uploader/curl process
  privacy test: PASS, zero skips. A control using the old actual uploader
  correctly fails the argv-privacy assertion without printing credentials.
- All five existing pn3 authentication cases using the exact revised client
  against pn3's retained native image binary: PASS, zero skips. Cases include
  wrong key/host, empty key database, missing token and valid real payloads.
  The test driver is `23eaacd:f3s/goprecords/tests/test_client_e2e.py`; it was
  imported with its CLIENT path set to this checkout, without changing bytes.
- Conf Go generate/build/test/vet/errcheck, shellcheck at warning level: PASS.
  f3sctl Go build/test/vet and its repository-configured errcheck gate: PASS.
  Standalone unconfigured errcheck reports 158 lines byte-identical to its
  unchanged baseline; it is not claimed green or refactored in this task.
- Native OpenBSD `sh -n` for both executable scripts and read-only
  `newsyslog -n` parsing of the candidate access-log configuration: PASS.

Follow-up strict uploader previews succeeded on both frontends, f0-f3 and
pi0-pi2, each with exactly one would-change: the shared script. Pi2 also prints
an unconditional ensure-directory notice for its already-existing 0755 root:root
systemd directory. The initial isolated Pi previews declared no client because
the checkout lacks the ignored file-backed tokens. Re-running the compiled
candidate from the canonical gonf working directory, with GONF_CONF_ROOT set
to this reviewed checkout, uses the existing fallback store in place and the
candidate assets. No tokens were copied, symlinked or changed. Use that same
working-directory arrangement for isolated deployment; symlinking secrets is
neither needed nor supported by the file provider. Pi3's separate known session
and old-runtime limits remain pending; it is still included in final acceptance.

Private controller evidence is under
`/home/paul/.local/state/conf-worktrees/dn3-hardening-20261023/evidence/`.
Initial Python-based preflight was unavailable on fishfinger; subsequent base
Perl probes succeeded on both. An initial `su` read test on blowfish failed
because of its pre-existing missing nologin class, not a log-access denial;
actual UID/GID-drop testing corrected that result. No login class was changed.
An early strict preview caught a content-less File operation for metadata;
using EnsureFile corrected it before any apply. Final strict-preview results
and source/review identifiers are recorded in the task annotations.

## Current approved-base integration (2026-10-24)

The new isolated candidate is under
`/home/paul/.local/state/conf-worktrees/dn3-rollout-20261024/`. Read-only
GitHub and Forgejo head checks agreed on conf `e5ec8e7`, dotfiles `3d06d93`,
and f3sctl `10d0c8e`. Those are its bases. Only DN3's reviewed commits were
cherry-picked; the canonical conf preparations and dotfiles user changes
were left untouched. The dotfiles base retains both the deployed Garage
documentation and the upstream AI-usage change.

Two blocking findings were fixed during the focused integration checks:

- The newer scoped DTail publisher still used 0744 `root:wheel`; it now
  preserves 0640 `root:_gogios`, like the other Gogios writers.
- Rendering the normal Gogios task exposed a latent original-DN3 duplicate
  `/etc/rc.local` resource registration. Earlier scoped tests had not rendered
  this task. Its runtime startup line edit now has a distinct name and depends
  on the mute helper, reusing that helper's existing file declaration.

`frontends/tests/test_gogios_plan_permissions.py` renders all three actual
configuration publishers locally, requires both hosts' configurations to be
identical across publishers with private permissions, and verifies both the
mute boot-clear and the existing Gogios runtime startup lines. It initially
exposed the duplicate registration and passes after the fixes, with zero
skips. The candidate build, targeted `go vet ./frontends`, diff checks and
staged secret scan passed. Prior unchanged implementation checks remain the
2026-10-23 results above; no redundant full suite was rerun.

Fresh exact native comparisons of each full live JSON object against the
matching rendered host pass after substituting only the 82 old mute paths.
Each retains all 572 checks, including both newly deployed frontend-specific
Forgejo TLS checks with SNI, the three Garage health checks, and all other
fields. Strict read-only previews of `frontends_low_risk`,
`frontends_services` and `frontends_retire_nrpe` pass on both frontends:
27 operations, zero changed, nine would-change. The local plan has an explicit
path/mode/owner allowlist and only the separate NRPE package removal.

Fresh uploader previews pass on eight hosts (both frontends, f0-f3 and
pi0/pi1), each showing only the shared script would change. Pi2's fresh
strict preview exceeded its 55-second bound without a result; its earlier
2026-10-23 pass remains historical evidence, not a current pass. Pi3's known
slow session/old-runtime check was not repeated. Both Pi checks remain
explicitly pending, without retries, runtime bootstrap or service repair.

Private fresh evidence, the compiled candidate and bounded preflight drivers
are in the sibling `evidence/` directory. No production writes, bootstrap,
publication, uploads, restarts, power/mute actions or credential changes were
performed. Required live backup, cutover, natural-upload and rollback checks
remain pending explicit DN3 approval. The original review covers unchanged
code; the two fixes and new guard require their focused review before rollout.

## Refreshed source and server compatibility (2026-10-24)

The conf candidate now includes final deployed policy documentation `b042676`
through merge `e0e900d`; no Gonf or frontend recipe bytes changed in that merge.
The paired skill candidate retains published policy reference `ab3a381`.

Goprecords 0.6.2 has since been released independently. The exact revised DN3
client was tested against the local binary independently verified to match the
running container: all five existing real-client authentication cases passed,
zero skips. These cover valid payloads, wrong key preserving existing files,
wrong hostname, empty-key default refusal and a missing token file. The tests
used loopback, disposable databases and freshly generated temporary keys; they
made no production upload or credential change. Private results are in
`evidence/server062-compatibility.json` beside the current candidate checkout.
The server image and release are outside this rollout.

## Proposed rollout, after review and explicit approval

1. Integrate/publish only this task's reviewed conf, f3sctl metadata and two
   skill files to the active GitHub/Forgejo repositories. Preserve all other
   pending changes and the immutable pn3 release artifact. The shared revised
   fleet client has already passed its five local auth cases against that
   artifact; this task does not change the server image or release tag.
2. Work on one frontend at a time, fishfinger then blowfish. Confirm no power or
   mute job is active and coordinate a cutover window without such jobs. Require
   old and new mute markers absent and known baseline monitoring unchanged;
   if a marker exists, stop rather than adopt an untrusted file from /tmp.
   Recheck NRPE's unused status and regular-file/ownership baselines. Require
   actual config contents to differ only by the intended marker path. A prior
   concurrent rollout, including ho3, requires rebase onto its published source.
3. On the current host, create a unique root-only backup directory and preserve
   exact modes/owners/contents of `/etc/gogios.json`, `/etc/rc.local`,
   `/etc/newsyslog.conf`, `/etc/unattended-upgrade-services`, and the installed
   mute helper/uploader. Record the active access log's metadata without copying
   or restoring its changing contents. Preserve the exact installed NRPE
   package for rollback before deletion: from the private backup directory use
   `pkg_create -f /var/db/pkg/<exact-installed-nrpe>/+CONTENTS`, then verify the
   resulting archive/listing and checksum. Stop if a usable backup is missing.
4. Repeat strict previews with the exact reviewed checkout and strict host-key
   checking. Use `push -preview -privilege=doas -- ... rex@HOST
   frontends_low_risk frontends_services`; `-preview` never bootstraps or writes.
   Reject unexpected changes. Apply the same two tasks only after those gates.
   Keep the host's retained admin path working throughout the short producer/
   reader cutover; no SSH reload or auth change is part of this work.
5. Verify owner/mode/path agreement, helper syntax and config JSON. Using the
   existing Pi0/Pi1 restricted connection, run status/mute/status/unmute/status
   and require that only the protected marker controls Gogios eligibility.
   Keep the mute interval bounded and restore unmuted state even on failure;
   do not run power commands. Verify an unrelated local identity cannot create
   the marker, _gogios can read its config but cannot write it/mute, and _dserver
   can read but cannot write the log. Check native dry-run rotation metadata,
   unchanged httpd PID and successful real web requests/log growth. Require
   monitoring to match the pre-change baseline; record any pre-existing
   failure separately without silencing it. Do not reboot to test boot cleanup.
6. Deploy the shared uploader serially to both frontends, f0-f3 and pi0-pi3 via
   their existing gonf upload tasks (frontends_goprecords,
   freebsd_goprecords_upload, netbsd_goprecords_upload,
   rocky_goprecords_upload). Previews must show only the script needs changing;
   stop on token/cron/service drift. Retain each script's private backup and
   verify exact bytes. An old gonf runtime requires an explicitly approved
   runtime bootstrap; do not use ordinary `-n` as a no-write preview. Pi3's
   known two-minute SSH/sudo delay needs adequate bounds or deferral until 1g3;
   do not infer approval of its service repair from this task. Offline personal
   clients use the same source on their next normal managed update and must be
   recorded as unverified until observed.
7. Require each changed host's next ordinary upload to succeed with 204 and
   records to advance. Inspect process arguments for header option shape only;
   never print arguments from the old uploader or secret values. Negative auth
   tests stay on the local disposable daemon: do not send negative PUTs to the
   production server. Do not change schedules or rotate tokens.
8. Only after frontend acceptance, recheck NRPE users/dependencies/listeners and
   apply `frontends_retire_nrpe` separately per host. Require the package absent,
   all local plugins still present and existing monitoring unchanged. Reapply
   the selected tasks and require convergence. Task dn3 remains open until all
   required live acceptance is actually observed; source preparation is not
   deployment success.

## Rollback

Stop on any new failure. With power/mute operations still excluded, restore
only the saved configuration/helper/uploader/list files using their recorded
owners and modes. Restore active access-log metadata without replacing log
contents. Keep monitoring unmuted throughout rollback; restore the matched old
config/helper pair together. The new directory can remain harmlessly unused;
do not recursively delete it or restore a mute from /tmp. If package removal
must be undone, install only the captured checksummed local NRPE archive (a
per-invocation `pkg_add -D unsigned` may be needed for this locally recreated
backup); preserve the previously disabled service state. This changes no
system-wide signature policy. No service restart/reboot is a rollback step.
Recheck monitoring, log readers, web requests, uploader behavior and convergence.

References: [curl header input](https://curl.se/docs/manpage.html#-H),
[OpenBSD newsyslog](https://man.openbsd.org/newsyslog.8), and
[recreating an installed OpenBSD package](https://man.openbsd.org/pkg_create.1).
