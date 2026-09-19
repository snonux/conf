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
  cmd/gonf/          # main: RegisterMethods + Aggregates + CLI
  cluster/           # Host / Cluster inventory (SSH + per-host WithValue)
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
| `NameFrontends` | `frontends` / `frontends_*` |
| `NameNetBSDPis` | `pis_netbsd` / `pis_netbsd_*` |
| `NameRockyAll` | `rocky` / `rocky_*` |
| `NameFreeBSD` | `freebsd` / `freebsd_*` |

- Iterate with `ClusterHosts()` (the cluster from `WithCluster` on the current task).
- Store schedules on the host: `WithValue(cluster.ValueUnattendedCron, …)` /
  `ValueUnattendedOnCalendar` / `ValueUnattendedCronMinute` /
  `ValueUnattendedAllowReboot` in `cluster.Register()`.
- Read with `MustHostValue[T](host, key)` — missing key or wrong type fails
  fast (`logger.Fatal`, exit 1). Do **not** keep parallel hostname→value maps
  in the recipe packages.

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
