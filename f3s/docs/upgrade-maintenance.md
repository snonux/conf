# Upgrade maintenance and coverage (jn3)

Local preparation dated 2026-10-22. No upgrades, restarts, reboots, VM starts,
configuration deployments, Git publication or ArgoCD sync ran for this task.
Independent review and approved rollout remain pending.

## Current evidence

All three API nodes report Ready with kubelet **v1.36.5+k3s1**. Read-only
native `k3s --version`/channel probes on r0, r1 and r2 agree with the
`stable` and `v1.36` release channels at verification time. Thus audit S24's
claim that k3s had never been upgraded is stale; the existing skill already
records the September rolling upgrade. This does not replace a maintenance
cadence or prove future versions will be installed.

f3 (`192.168.1.133`) and standalone rocky (`192.168.1.123`) both returned
no route to host. They were not powered on or rebooted. f3's exporter target
is down. Its current kernel and rocky's current packages, sudo/bootstrap
state, clock and update services are therefore unverified.

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

Prepared task `rnodes_k3s_version_monitor` installs a Python-stdlib checker on
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
Daily package upgrades can restart affected services; deploying and enabling
the timer therefore needs explicit approval after a live preflight.

When ordinary access returns, verify the hostname, Rocky release, root SSH,
`sudo -n`, gonf bootstrap, clock, current timers and pre-existing update tools.
Preview and approve only the unattended tasks needed there; do not deploy the
whole evolving `rocky` aggregate. Verify the installed schedule, then observe
a real scheduled update, successful daily stamp and deferred reboot. That
runtime acceptance is still outstanding while the VM is unreachable.

## f3: approval-only kernel reboots with a visible pending state

Preserve f3's existing no-automatic-reboot policy, consistent with the user's
instruction against rebooting systems. Do not infer permission to stop its
Rocky guest, invoke recovery mode or reboot it from this task. Record pending
kernel fixes and arrange a separately approved guest-aware maintenance window
when the host is available. This is an explicit maintenance policy, not a
claim that an installed kernel fix has already taken effect.

Prepared task `freebsd_monitoring_kernel_reboot_metrics` installs a five-minute
read-only `freebsd-version -k` versus `-r` check on f0–f3. It publishes atomic
metrics in the existing `/var/tmp/node_exporter` collector. Empty, malformed
or failed version reads publish failed coverage, not a clean kernel result.
Alerts distinguish pending reboot from a missing/failed/stale check on a
reachable exporter. Target-down monitoring remains responsible when f3 is
offline. The existing updater's f0–f2 behavior is unchanged by this task.

The monitoring prerequisite also manages node_exporter's existing configuration
and may restart that exporter if it has drifted. Preview and include any such
restart explicitly in approval. The check itself never updates the OS, stops
guests or reboots a machine. f3's policy can only be accepted operationally
after access returns, its installed/running kernels are inspected and any
pending fix receives approved maintenance.

## Verification and rollout remaining

Local tests exercise real JSON/version comparisons, local HTTP and subprocess
I/O, failure publication, atomic replacement and kernel-read failures. Native
promtool evaluates all five manifest alerts across sixteen scenarios,
including absent/stale/failed/future data, wrong service, one missing peer,
target down and pending periods. The Rocky harness verifies a standalone
daily package run with a fresh core update still cannot reboot.

Unchanged producer sources were sent through SSH stdin with `--stdout`, which
writes no metrics or host file: r0–r2 each report a successful current-version
check; f0 reports successful kernel comparison with no reboot pending. These
are native read-only probes, not deployed cron/scrape/alert acceptance. Local
Gonf plans include the typed standalone timer and preserve the Dtail guard.
Read-only evaluation against the live Prometheus API confirms three expected
k3s inventory nodes and the current missing-check coverage on those three
nodes and f0–f2. The proposed producers are not yet scraped. Temporary local
port-forwards used for these reads were closed.

After fresh independent review, request a separate approval for the monitoring
scripts/crons, any exporter drift correction and isolated publication/sync of
`f3s/prometheus/manifests/upgrade-alerts.yaml`. Verify fresh files, successful
scheduled ticks, actual scraped series/label joins, loaded rules and expected
alert state. Alertmanager currently exposes these alerts in its UI; this
task adds no email/webhook route. Rocky/f3 rollout stays pending until ordinary
access and their host preflights succeed. No jn3 rollout has been approved.

Tests:

```sh
python3 -m unittest discover -s f3s/r-nodes/k3s/tests
PROMTOOL=/path/to/promtool python3 -m unittest discover \
  -s f3s/prometheus/tests -p test_upgrade_alerts.py
bash frontends/scripts/tests/unattended-upgrade-rocky.ksh
TEST_SHELL=/usr/bin/ksh bash frontends/scripts/tests/unattended-upgrade-rocky.ksh
```
