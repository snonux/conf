# Handoff — verifying & debugging unattended upgrades on the OpenBSD frontends

For an agent taking over checking/debugging of the unattended-upgrade
automation on **blowfish.buetow.org** and **fishfinger.buetow.org** (OpenBSD
7.8). Everything needed is in this file plus the companion docs below.

## 0. Ground rules

- **Everything is managed by gonf.** Do not install or modify anything on the
  hosts manually — redeploy via gonf (§5). Read-only ssh checks are fine.
- Annotate findings into the task system: `~/go/bin/ask annotate i22 "…"`
  (i22 is the active observation task). Never use raw `task`.
- The k3s cluster (r0/r1/r2) is **powered off for energy saving and comes
  online on demand**. The relayd front serves its "Server turned off" HTML
  page then; this is the **normal idle state**, not an outage.

## 1. Access & quick facts

| | blowfish | fishfinger |
|---|---|---|
| ssh | `ssh -p 2 rex@blowfish.buetow.org` (BatchMode works) | `ssh -p 2 rex@fishfinger.buetow.org` |
| privilege | `doas -n` (passwordless) | `doas -n` |
| gonf binary | `/usr/local/bin/gonf` → **0.7.8** | same |
| script | `/usr/local/sbin/unattended-upgrade` (0755 root:wheel) | same |
| script sha256 (first 16) | must equal the repo copy after approved deployment | same |
| log | `/var/log/unattended-upgrade.log` (0600 root — read via `doas`) | same |
| services list | `/etc/unattended-upgrade-services` (0644) | same |
| root crontab after approved deployment | 4 GONF marker blocks: `10 6`, `40 6`, `05 7`, `35 7` | `10 22`, `40 22`, `05 23`, `35 23` |
| repo script | `/home/paul/git/conf/frontends/scripts/unattended-upgrade.sh` (conf repo) | |

Compare deployed vs repo before debugging drift. The hash below is historical;
task 552 changes the repository script but does **not** deploy or verify it.
After explicit deployment approval, record and compare the new hash:
`ssh -p 2 -o BatchMode=yes rex@<host> 'doas -n sha256 -q /usr/local/sbin/unattended-upgrade'`
vs `sha256sum /home/paul/git/conf/frontends/scripts/unattended-upgrade.sh`.
At this handoff, **74e5cd2ccf0e5112** was the deployed historical hash; it
does not equal the changed repository source until an approved rollout.

Repo: `/home/paul/git/conf` (conf). Redeploys run from `/home/paul/git/conf`
via `./gonf.sh` (see §5).

## 2. The model (what the automation does)

Target root crontab per host after approved deployment: 4 GONF-managed jobs
(blowfish morning,
fishfinger evening, staggered):

| Job | Window | What it does |
|---|---|---|
| `base` | blowfish 06:10 / fishfinger 22:10 | `syspatch` — with an automatic **re-run after the tool self-update** (001_syspatch exits 2 with errata pending). Silence = clean no-op; patches → log + mail |
| `pkgs` | blowfish 06:40 / fishfinger 22:40 | `pkg_add -Iu`; official repos via `installpath`, the custom fleet repo (pkgrepo.f3s.buetow.org) **probe-gated**; restarts the daemons listed in `/etc/unattended-upgrade-services` when non-quirks packages updated |
| `audit` | blowfish 07:05 / fishfinger 23:05 | non-mutating `pkg_add -Iun` after `pkgs` has refreshed OpenBSD's signed `quirks` metadata and before reboot; it is scheduled five minutes after the latest possible `pkgs` start, and a lock conflict reports failure rather than claiming a result. A clean audit is retained quietly in the log, while pending packages/advisories, custom-repo unavailability, and command failures mail root and exit non-zero |
| `reboot` | blowfish 07:35 / fishfinger 23:35 | reboots **only when the on-disk kernel (`what /bsd`) differs from the booted one (`sysctl -n kern.version`)** — no runtime flags; picks up manually applied kernel patches too |

Cron runs have **0–20 min jitter** (non-tty runs). Manual runs with `-tt`
skip the jitter — for interactive debugging always use
`ssh -p 2 -tt rex@host 'doas -n /usr/local/sbin/unattended-upgrade <mode>'`.

Gates (each skips with a log line, never "fails" the host):

- **Partner gate**: the update runs only while the partner frontend answers
  its `https://<partner>.buetow.org/index.txt` "Welcome to <partner>" health
  check over IPv4+IPv6 (same check as dns-failover.ksh). Skip log:
  `skipped <mode>: partner <host> not operational`.
- **Lock**: `/var/run/unattended-upgrade.lock` (2 h stale-lock recovery).
- **Custom-repo probe**: pkgrepo.f3s.buetow.org is k3s-backed and the cluster
  sleeps for power saving. When it is down, relayd serves an HTTP-200
  "Server turned off" page — the probe detects it and `pkgs` **skips the
  custom-repo packages with a WARNING and exits 0** (official errata still
  apply). This is EXPECTED, not a failure. Custom packages (dtail/gogios)
  resume in a later window when the cluster is awake.
- **Audit custom-repo probe**: unlike `pkgs`, `audit` fails loudly while that
  repository is unavailable. A fallback would omit installed fleet packages
  and make the audit result incomplete.

There are **no runtime flags/needs-reboot markers** by design: the reboot
decision is derived from the kernel version compare.

## 3. Verification recipes

Run per host (all commands read-only):

```sh
ssh -p 2 -o BatchMode=yes rex@<host> '
  echo "== script sha vs repo:"; doas -n sha256 -q /usr/local/sbin/unattended-upgrade
  echo "== gonf:"; gonf -version                       # expect 0.7.8
  echo "== cron blocks (expect 4):"; doas -n crontab -l -u root | grep -c BEGIN
  echo "== schedules:"; doas -n crontab -l -u root | grep -A1 "BEGIN GONF"
  echo "== kernel pending? (expect: current):"
  booted=$(sed -n "1{s/[[:space:]]*\$//;p;}" /var/run/dmesg.boot)
  ondisk=$(doas -n what /bsd | grep -m1 OpenBSD | sed "s/^[[:space:]]*//")
  [ "$booted" != "$ondisk" ] && echo PENDING || echo "current"
  echo "== syspatch pending (expect 0):"; doas -n syspatch -c | wc -l
  echo "== lock/stamp (expect absent):"; ls -d /var/run/unattended-upgrade.lock /var/run/unattended-upgrade 2>&1
  echo "== log tail:"; doas -n tail -10 /var/log/unattended-upgrade.log
'
```

What each mode's log signature means:

| Log line | Meaning |
|---|---|
| *(nothing for a window)* | clean no-op (base with 0 pending; pkgs with nothing to update; reboot with current kernel) — **success** |
| `[ts] syspatch applied base patches:` + patch list | base applied errata (expect a `needs-reboot`-style reboot at the next reboot slot if the kernel was patched — the log then shows `rebooting to activate the patched kernel`) |
| `[ts] pkg_add -u updated:` + package list | pkgs applied updates; restarts follow unless the list is quirks-only |
| `[ts] package audit clean: …` | current OpenBSD package metadata found no pending package updates or quirks warnings; this is written quietly to the root-only log, not mailed |
| `[ts] package audit found …` / `package audit FAILED …` | **investigate**: pending package/advisory state, unavailable custom repository, or a package-manager failure; cron mail is deliberate |
| `[ts] WARNING: https://pkgrepo… not operational — skipping custom-repo packages this window` | the k3s cluster is asleep — EXPECTED; official updates continued |
| `[ts] WARNING: pkg_add -u rc=1 with the custom repo skipped — official updates applied, custom packages deferred` | companion line to the above; success by design |
| `[ts] WARNING: <partner> not operational` | partner gate skipped the window; retry at the next window |
| `[ts] pkg_add -u FAILED (rc≠0)` / `syspatch FAILED (rc≠0)` | **investigate** (see §4) |
| `[ts] rebooting to activate the patched kernel` | the host reboots seconds later — verify uptime + the new kernel afterwards |

## 4. Failure signatures & debugging

| Symptom | Root cause seen | Fix |
|---|---|---|
| `pkg_add -u FAILED (rc=1)` + `https://pkgrepo…: empty` + `Couldn't find updates for dtail… gogios…` | k3s cluster asleep → the relayd front serves its "Server turned off" page → custom index empty | **Fixed/expected with the current script** (WARNING + exit 0). If seen on an old script: redeploy `frontends_script` |
| `https://pkgrepo…/ not operational` WARNING only | cluster asleep, tolerated | none — by design |
| `skipped <mode>: partner … not operational` | partner frontend down/rebooting | none — retried next window |
| `reboot: not found` | PATH lost `/sbin` (fixed in the current script) | redeploy the script |
| `syspatch FAILED (rc>0)`, EOL text | release unsupported → planned attended `sysupgrade` (runbook §9 of the impl doc) | manual release upgrade |
| `pkg_add -u FAILED (rc=1)` with real package errors | official repo/mirror problem or dependency conflict | follow impl doc §9 triage; rerun manually |
| update took old code, daemon not restarted | restart-list gap | check `/etc/unattended-upgrade-services` vs `rcctl ls on`; the list is deployed by gonf |

Deep checks when a run misbehaves:

- Controller side (laptop, from `/home/paul/git/conf`): `./gonf.sh -version`
  (0.7.8), `./gonf.sh -list` (6 entries), record-only plan:
  `./gonf.sh -privilege=doas plan -o /tmp/plan frontends_script` —
  inspect `/tmp/plan/plan.jsonl` (elevate flags, schedules). **Never run a
  local dry-run of the privileged tasks** (a v0.7.8 library bug applies them
  for real; use `plan -o`).
- Remote apply staging: `/tmp/gonf-apply` (1777) — recreated per apply;
  leftovers are swept after 24 h.
- The gonf library tag deployed on the hosts is **0.7.8** (controller go.mod
  may be newer — registration-side changes only, plan schema v6 unchanged).

## 5. Redeployment (gonf-only)

From `/home/paul/git/conf` (the controller; script + tasks live in the repo):

```sh
# script (both hosts):
./gonf.sh cluster frontends frontends_script
# full stack:
./gonf.sh cluster frontends frontends_script \
  frontends_services frontends_cron \
  frontends_newsyslog
# or per host:
./gonf.sh -privilege=doas push rex@blowfish.buetow.org <task names…>
```

Idempotency check: a second identical run must print `0 changed` /
`0 would-change`. Plan inspection without applying:
`./gonf.sh -privilege=doas plan -o /tmp/plan <tasks>` (then read
`/tmp/plan/plan.jsonl`; delete the dir after).

Do **not** run local dry-runs of the privileged tasks (`./gonf.sh -n …`)
— a v0.7.8 library bug applies them for real on the laptop; use `plan -o`.

## 6. History snapshot (as of this handoff, 2026-09-16 15:5x EEST)

- Both hosts: gonf 0.7.8, script sha `74e5cd2ccf0e5112` (then == repo), 3
  GONF cron blocks, 0 syspatches pending, kernels current, no locks/stamps.
- fishfinger rebooted **2026-09-15 23:16** via the automation (patched
  kernel #20 loaded — the first fully-automated reboot of the fleet).
- blowfish this morning: 06:10 base silent ✓; 06:40 pkgs **FAILED** (the
  custom repo was down and the k3s-down tolerance was not yet deployed at
  that hour — failure mail went out; fixed and redeployed 12:54); 07:10
  reboot no-op ✓.
- blowfish 13:36: repo came back → pkgs re-run (official quirks) exit 0.
- gogios: no new CRITICALs caused by the rollout; pre-existing warnings on
  fishfinger only (taskwarrior-garage 403, shuriken age, hosta uptimed).
- Open items tracked in task **i22**: observe further cycles (blowfish
  mornings, fishfinger evenings — note fishfinger's first evening pkgs/base
  cycle with the fixed script runs tonight 22:10/22:40), and the
  notification decision for the Pis (no MTA there — OpenBSD hosts do mail).

## 7. Pointers

- Script source: `/home/paul/git/conf/frontends/scripts/unattended-upgrade.sh`
  (the deployed copies are byte-identical to it — the source of truth).
- Implementation doc: `docs/archive/frontends/docs/unattended-upgrades.implementation.md`
  (design + rollout + the hardening table). Plan/overview:
  `docs/archive/frontends/docs/unattended-upgrades.plan.md`. Pi/Rocky plan (for the
  on-demand Rocky pattern, later r0/r1/r2/pi2/pi3):
  `docs/archive/frontends/docs/unattended-upgrades-pi.plan.md`.
- gonf library: `/home/paul/git/gonf` (docs in `docs/`; the deployed remote
  binary is 0.7.8; the controller runs the conf go.mod version).
- The failure mail path: cron output → root → aliases → paul's Proton
  mailbox; verify arrivals via the local Proton Bridge IMAP
  (127.0.0.1:1143, credentials from `~/.config/aerc/protonbridge-password`,
  per the `protonbridge-aerc` skill) or simply the `/var/mail/` flow on the
  frontends.
- Task tracking: `~/go/bin/ask` — annotate findings with
  `~/go/bin/ask annotate i22 "…"`.
