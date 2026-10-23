# Upgrade maintenance and coverage (jn3)

Prepared 2026-10-22, reviewed and approved for scoped rollout 2026-10-23.
The approved source was published in integrated commit `91d7fb2`. Monitoring
is deployed on f0–f3 and r0–r2; standalone rocky's package updater completed
its first timer-triggered run. Read-only diagnosis established a two-minute
PAM session timeout on r0/r2. Reusing existing authenticated SSH connections
allowed their approved rollout without changing host authentication or
services. No host or guest was rebooted. See rollout acceptance for limits.

## Current evidence

All three API nodes report Ready with kubelet **v1.36.5+k3s1**. Read-only
native `k3s --version`/channel probes on r0, r1 and r2 agree with the
`stable` and `v1.36` release channels at verification time. Thus audit S24's
claim that k3s had never been upgraded is stale; the existing skill already
records the September rolling upgrade. This does not replace a maintenance
cadence or prove future versions will be installed.

The 2026-10-22 no-route observations for f3 (`192.168.1.133`) and standalone
rocky (`192.168.1.123`) are superseded by read-only preflights on 2026-10-23.
f3's native kernel producer probe succeeded with reboot pending **0**; all four
FreeBSD exporter targets were up. This checks installed versus running kernel
versions, not whether a newer kernel is available upstream.

Before rollout, standalone rocky accepted strict, verified root SSH on port 22
without sharing a multiplexed connection. It reported Rocky Linux **9.8**,
Europe/Sofia time,
working Python and `sudo -n true`, and root-owned mode-0755 gonf at
`/usr/local/bin/gonf` plus its `/usr/bin/gonf` symlink. `ksh`, `yum-utils` and
`needs-restarting` were absent, as were the unattended script, service, timer
and last-daily stamp. This confirmed the unattended coverage gap. Those
preflight checks installed no packages, refreshed no cache and started no
service.
Public-only evidence is retained outside Git beside this worktree in
`../evidence/f3-kernel-20261023.json` and `rocky-preflight-20261023.json`.

## k3s: attended upgrades with automatic detection

Use a documented manual cadence rather than a system-upgrade-controller
Plan. The three nodes are also etcd servers; upgrades need a reviewed target,
healthy storage and a serial recovery path. A controller adds privileged
upgrade Jobs and does not protect against unsupported minor jumps. The
[official manual](https://docs.k3s.io/upgrades/manual) requires server nodes
to upgrade one at a time, preserves installer arguments only when explicitly
supplied again, and documents the installer's service restart. The
[automated-upgrade guide](https://docs.k3s.io/upgrades/automated) also warns
that version-skew constraints still need operator review.

Review channels and release notes monthly, and triage security notices or
version alerts promptly. Schedule an approved patch window within seven days
of identifying an applicable patch; an urgent security advisory can require
an earlier window. Review a newer stable minor at the same cadence and plan
every intermediate minor. A notice is a request to review, not approval to
run the installer. The detailed existing procedure is in the f3s skill,
`references/k3s/install.md`, section "Upgrading k3s".

Task `rnodes_k3s_version_monitor` installs a Python-stdlib checker on
r0–r2 and an hourly root cron at `:17`. It also runs once when installed or
changed. The command has a 30-second timeout plus a five-second kill grace;
the checker uses a five-second version subprocess timeout, ten-second HTTPS
socket timeout and a one-MiB response limit. It reads the fixed official
channel endpoint with TLS verification, compares the installed minor's
latest release and the stable minor, and publishes atomic mode-0644 metrics
to `/var/lib/node_exporter/textfile_collector/k3s_version.prom`.

Minor channels can contain newer releases than stable; their alerts require
release-note review. No automatic upgrade, drain or restart is implemented.
Failed/malformed/missing/duplicate channel data publishes failure rather than
"up to date" and drops obsolete availability metrics. Alerts cover patches,
new stable minors, and checks that fail, disappear, become over two hours old
or have implausibly future timestamps. They use the live node-exporter
service labels and the existing kube-state-metrics node inventory; complete
loss of kube-state-metrics still needs its existing exporter/target alert.

Before an eventual upgrade, resolve **ao3**'s storage contract and the current
Player/Xplayer/goprecords recovery failures, check etcd quorum and API/NFS/app
baselines, prepare an etcd snapshot and reviewed rollback, then request
approval for the exact target and sequence. Upgrade r2, r1, r0 one at a time;
stop on any failed Ready/quorum/storage/application gate. Do not run this
procedure now or infer approval from deploying a monitoring check.

## Standalone rocky: package coverage without a new service role

The documented root SSH identity and explicit LAN port 22 are registered for
`rocky`, with typed `UnattendedCalendar` data at `:55`. `rocky-all` now includes
all six Rocky hosts. A separate `rocky-dtail` role retains only the existing
pi2/pi3/r0/r1/r2 members, so the general-purpose VM does not acquire dserver
through this inventory addition. No SSH authentication policy is changed.

The shared daily package updater explicitly permits standalone rocky to
update without a redundant partner and explicitly refuses its automatic
reboot path. Existing Pi/r-node partner and reboot gates stay in place.
Daily package upgrades can restart affected services. The user approved the
missing prerequisites, updater and timer, including an immediate package
update and affected service restarts, after the live preflight.

The rollout used only the required unattended prerequisites and explicit
`rocky_units` and `rocky_logrotate` tasks. It did not run the `rocky` aggregate.
The timer immediately triggered a real update; the native service result,
fresh successful stamp and explicit reboot deferral are recorded below.

## f3: approval-only kernel reboots with a visible pending state

The approved design preserves f3's existing no-automatic-reboot policy and
makes pending kernel fixes visible for a separately approved, guest-aware
maintenance window. No f3 reboot was approved or performed. The user's
restriction
covers recovery boots and SSH authentication changes; it is not a blanket ban
on normal reboots. This preparation authorizes no guest stop or reboot.

Task `freebsd_monitoring_kernel_reboot_metrics` installs a five-minute
read-only `freebsd-version -k` versus `-r` check on f0–f3. It publishes atomic
metrics in the existing `/var/tmp/node_exporter` collector. Empty, malformed
or failed version reads publish failed coverage, not a clean kernel result.
Alerts distinguish pending reboot from a missing/failed/stale check on a
reachable exporter. Target-down monitoring remains responsible when f3 is
offline. The existing updater's f0–f2 behavior is unchanged by this task.

The monitoring prerequisite also manages node_exporter's existing configuration
and may restart that exporter if it has drifted. Conditional exporter
restarts were included in approval; all four configurations matched, so no
exporter restart occurred. The check itself never updates the OS, stops
guests or reboots a machine. The successful f3 kernel comparison satisfied the
read-only preflight. Its deployed monitor passed real scheduled and scraped
acceptance below; future kernel maintenance still needs separate approval.

## Verification

Local tests exercise real JSON/version comparisons, local HTTP and subprocess
I/O, failure publication, atomic replacement and kernel-read failures. Native
promtool evaluates all five manifest alerts across sixteen scenarios,
including absent/stale/failed/future data, wrong service, one missing peer,
target down and pending periods. The Rocky harness verifies a standalone
daily package run with a fresh core update still cannot reboot.

An isolated synthetic fixture confirmed a root publication race in the new
shell producer's original public temporary-file write/chmod sequence: a
directory owner could substitute a symlink, overwrite a private sentinel and
change its mode from 0600 to 0644. The corrected producer writes and chmods
only inside a private directory beneath trusted sticky `/var/tmp`, pins the
physical collector directory, refuses cross-filesystem publication and renames
the complete file into that directory. Regression fixtures cover both race
boundaries, destination symlinks, directory replacement, failed rename,
cross-filesystem refusal and signal cleanup. The existing nobody-owned
exporter directory policy is preserved. The new Python producer's Gonf recipe
requires a root-owned mode-0755 collector, also observed by read-only stat on
r1. The Python producer writes/chmods through its retained file descriptor.
No equivalent publication defect was confirmed there.

Producer sources from the initial preparation were sent through SSH stdin
with `--stdout`, which writes no metrics or host file: r0–r2 each reported a
successful current-version check; f0 and the later f3 probe reported successful
kernel comparison with no reboot pending. These are native read-only probes,
not deployed cron/scrape/alert acceptance. The corrected shell producer also
passed FreeBSD's native `sh -n` through SSH stdin. Local
Gonf plans include the typed standalone timer and preserve the Dtail guard.
Pre-rollout read-only evaluation confirmed the expected k3s node inventory
and missing-check coverage. Temporary preflight port-forwards were closed.

## Rollout acceptance — 2026-10-23

Fresh independent review found no blocking issue. Deployment used compiled
source and assets pinned to `56f09d8`, fresh host previews, serial explicit
tasks and private logs/backups. The required normal Gonf bootstrap from
0.25.0 to 0.25.2 on r0/r1/r2/f3/rocky was approved, backed up privately and
performed through normal push; no version guard was bypassed.

- **f0–f3 kernel monitoring:** only the script and root cron changed. Existing
  exporter prerequisites matched and their process IDs stayed unchanged.
  Natural cron ticks at 15:05, 15:10 and 15:15 UTC published root:wheel 0644
  metrics. Actual Prometheus scrapes showed success 1, pending 0 and fresh
  timestamps on all four hosts, with textfile scrape errors 0. f0 remained
  CARP MASTER, f1 BACKUP with its replica dataset read-only, and guests kept
  their existing process IDs.
- **r1 version monitoring:** the initial run and natural 15:17 UTC cron run
  succeeded. The cron log recorded command start and completion; the fresh
  metric timestamp `1792768621` was scraped with the expected node-exporter
  service and node labels. Installed, same-minor and stable targets were
  `v1.36.5+k3s1`, with patch and minor availability both 0.
- **r0/r2 version monitoring:** read-only diagnosis showed that fresh SSH
  commands succeed after a roughly 120-second PAM timeout. Existing verified
  connections were reused through per-invocation `ControlPath` options.
  Normal Gonf version checks and privileged-task guards stayed in place;
  normal sudo apply also waited for PAM (121.07 seconds on r0 and 121.09 on
  r2). Deployment finished at 15:38:36 and 15:42:38 UTC respectively. Both
  native initial checks and actual scrapes showed success 1, matching source
  checksums, root:root 0644 metrics and the same current channel targets as
  r1. Each active cron daemon has exactly the hourly `:17` bounded command.
  The natural 16:17 UTC ticks passed after the known PAM delay: both cron
  logs recorded command start at 16:19:01 and completion at 16:19:02 UTC.
  Both root:root 0644 metric files advanced to timestamp `1792772341`, and
  actual Prometheus scrapes confirmed that same timestamp with success 1,
  patch availability 0 and minor availability 0. No collector was invoked
  manually for this acceptance check. All three Kubernetes nodes remained
  Ready at `v1.36.5+k3s1`; no k3s upgrade or restart, SSH/PAM/logind change
  or host-service restart was performed.
- **Standalone rocky:** missing ksh/yum-utils, updater, stamp directory,
  units and log rotation were installed. Enabling the approved timer
  triggered its first real daily package update at 15:03:10 UTC. It finished
  at 15:11:51 with native service result success and exit status 0, and wrote
  root:root 0600 `last-daily` containing `2026-10-23`. The timer remained
  enabled and active, next due at 18:55 Europe/Sofia. Kernel/package updates
  and affected service restarts occurred; the log explicitly deferred the
  standalone reboot. Its boot time remained 07:57:55 UTC
  (10:57:55 Europe/Sofia).
- **Rocky limits:** `auditd` refused the updater's manual systemctl restart
  (`RefuseManualStop=yes`); it remained active, and `needs-restarting -s`
  still listed it. A single longer strict fresh SSH probe, without a control
  connection, succeeded in 120.52 seconds, confirming ordinary access after
  the same session delay. The private pre-update connection remained usable
  and was closed only after alternate access was confirmed. No manual auditd
  workaround, authentication change or reboot was attempted.
- **Five alerts:** the single published `upgrade-alerts` PrometheusRule was
  byte-checked against `91d7fb2`, server-dry-run validated and applied alone.
  Native Prometheus loaded all five with healthy evaluations. Patch, minor,
  kernel-pending and kernel-coverage alerts were inactive. K3s coverage
  initially identified exactly the undeployed r0/r2 nodes, then cleared as
  their successful checks were scraped. At 15:43:49 UTC all five alerts were
  inactive with healthy evaluations. The same state was verified after the
  final natural r0/r2 cron ticks, with all three nodes reporting successful
  fresh checks. Alertmanager retains its existing UI-only routing; no
  email/webhook route was added.

Public-only acceptance evidence and private operation logs are outside Git
at `/home/paul/.local/state/conf-worktrees/jn3-rollout-20261023/live/`.
All approved producers are deployed and scraped, with natural scheduled
runs observed on every monitoring host. Final scheduled acceptance evidence
is retained as `live/acceptance-natural-cron-final.json` alongside the native
cron logs. The two-minute session delays and active but stale auditd process
remain explicit operational follow-up. No broad aggregate, NFS/CARP/failover,
remount, SSH authentication or reboot operation ran for this rollout.

Tests:

```sh
python3 -m unittest discover -s f3s/r-nodes/k3s/tests
PROMTOOL=/path/to/promtool python3 -m unittest discover \
  -s f3s/prometheus/tests -p test_upgrade_alerts.py
bash frontends/scripts/tests/unattended-upgrade-rocky.ksh
TEST_SHELL=/usr/bin/ksh bash frontends/scripts/tests/unattended-upgrade-rocky.ksh
```
