# Agent guidelines — conf

This repository is the **homelab configuration** (recipes, scripts, cluster
inventory). The configuration-management **library** lives separately at
`~/git/gonf` (`github.com/snonux/gonf`).

## Where tests belong

- **Do not add `*_test.go` under `./gonf`.** That tree is thin recipe code:
  `RegisterMethods` task bodies, `cluster.Register()` inventory, and path
  helpers. It is not a place for unit tests.
- **Library behaviour** (resources, plan/apply, `WhenHostname`, `Host`/`Cluster`
  registry, options, privilege split, …) is tested in **`~/git/gonf`**. If a
  bug or invariant shows up while working here, reproduce it with a **generic**
  fixture in the gonf repo and bump the module version this tree depends on.
- **Host-specific inventory** (real hostnames, ports, doas/sudo) is verified by
  deploy (`./gonf.sh cluster …`), not by pinning every inventory row in a test.
  Prefer comments next to non-obvious `WithSSHPort` / `WithPrivilege` choices.

### Moving a test from conf → gonf

1. Strip conf names (`blowfish`, `pi2`, …). Use `a.example`-style hosts.
2. Assert the **API contract** (e.g. `Hosts()` round-trips `Port` and
   `Privilege`; omitting `WithSSHPort` leaves `Port == 0`).
3. Commit and tag in `~/git/gonf`, then `go get github.com/snonux/gonf@v…`
   in `./gonf`.

## Layout of `./gonf`

```
gonf/
  cmd/gonf/          # main: secret provider (vault + file fallback), cluster.Register, tasks.Register, CLI
  tasks/tasks.go     # composition root: RegisterMethods + aggregates
  cluster/           # Host / Cluster inventory (SSH + per-host WithValue)
  frontends/         # OpenBSD frontend recipes (Maintenance, Web, ...)
  openbsd/unattended.go
  netbsd/unattended.go
  rocky/unattended.go
  freebsd/unattended.go
  paths/             # repo-relative path constants
```

Unattended tasks follow one schema per OS package: type `Unattended`
with `RequiresRoot`, file name `unattended.go`, registered with a prefix
and cluster. Name methods for the action (`Script`, `Cron`, …), not
`UnattendedScript` — the type and `WithPrefix` already namespace:

```go
RegisterMethods(openbsd.Unattended{}, WithPrefix("frontends_"), WithCluster(cluster.NameFrontends))
```

### Hosts and per-host values

| Cluster constant | Tasks / Aggregate |
|-------------------|-------------------
| `NameFrontends` | `frontends` / explicit `frontends_*` list (see below) |
| `NameNetBSDPis` | `pis_netbsd` / `pis_netbsd_*` |
| `NameRockyAll` | `rocky` / `rocky_*` |
| `NameRockyPis` | `rocky_kernel_audit` / `rocky_kernel_audit_*` |
| `NameRockyK3s` | `rnodes` / `rnodes_*` |
| `NameFreeBSD` | `freebsd` / `freebsd_*` |
| `NameGarage` | `garage` / `garage_*` |

All aggregates are registered in `gonf/tasks/tasks.go`. Most are pattern
`Aggregate`s; `frontends` is an `AggregateTasks` with an explicit member list
(`frontendSetupTasks`). The operational `frontends_acme_invoke`,
`frontends_irc_bouncer` and `frontends_ping` (the unprivileged push-pipeline
diagnostic, excluded since 2026-09-22) are marked `Operational()` and listed
in `frontendExcludedTasks` instead. `checkFrontendMembership` reports a gonf
declaration error at registration (so every invocation, `-list` included,
is refused with exit 1) when a `frontends_*` task is in neither list, or a
listed name is not registered: a new frontend task must be added to one of
the two lists.

- Iterate hosts with a per-host value via `ForHosts(key, func(host string, v T) {…})`
  (the cluster from `WithCluster` on the current task). It type-checks every
  host's value before recording, wraps each body in a destination-side
  `hostname_contains` guard, and on runs with a known target (single-host
  push, local run) skips other hosts, so read host-specific inputs such as
  secrets inside the body. Cluster pushes and `gonf plan` still visit all hosts.
- `ClusterHosts()` plus `WhenHostname` remains for loops that need no typed
  value (it never narrows to the push target).
- Store schedules on the host: `WithValue(cluster.ValueUnattendedCron, …)` /
  `ValueUnattendedOnCalendar` / `ValueUnattendedCronMinute` /
  `ValueUnattendedAllowReboot` in `cluster.Register()`.
- Outside `ForHosts`, read with `MustHostValue[T](host, key)` — a missing key or
  wrong type is a gonf declaration error (the record fails, or the CLI
  refuses the run with exit 1). Do **not** keep parallel hostname→value maps
  in the recipe packages.

### Secrets

Recipes read secrets only with `MustSecret` / `OptionalSecret` and the
logical paths from `gonf/paths` (`FrontendSecret`, `GarageSecret`).
`cmd/gonf/main.go` resolves them through a foostore/KeePass vault for the
references its table maps, falling back to `gonf/secrets/` for every other
one (`secret.NewFallback`). Migrating a secret is one more table row; see
`gonf/secrets/README.md` for the mapped references, the controller
prerequisites and the policy. Never print, commit or log a secret value.

### DSL: use `List`, not `[]string{…}`

In recipe bodies prefer `List("a", "b")` (and `List(slice...)`) over a raw
`[]string{…}` literal — same convention as gonf docs/examples (`Command` args,
multi-path `File`/`Dir`, `WhenHostname`, `EachKV`).

## Running conf recipes

```bash
./gonf.sh -list
./gonf.sh cluster rocky-all rocky
./gonf.sh cluster netbsd-pis pis_netbsd
./gonf.sh cluster frontends frontends
./gonf.sh cluster freebsd-hosts freebsd
```

Pi, r-node, and f-host SSH must set `WithSSHPort(22)`: `~/.ssh/config` maps
`*.buetow.org` to port 2 (OpenBSD frontends).
