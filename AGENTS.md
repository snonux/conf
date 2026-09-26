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
  cluster/           # Host / Cluster inventory (SSH, HostDefaults, per-host WithData)
  frontends/         # OpenBSD frontend recipes (Maintenance, Web, ...)
  openbsd/unattended.go
  netbsd/unattended.go
  rocky/unattended.go
  freebsd/unattended.go
  paths/             # repo-relative path constants
  mailbox/           # root-mailbox archive script shared by freebsd/netbsd
```

Unattended tasks follow one schema per OS package: type `Unattended`
with `RequiresRoot`, file name `unattended.go`, registered with a prefix
and `OnCluster`. Name methods for the action (`Script`, `Cron`, …), not
`UnattendedScript` — the type and `WithPrefix` already namespace:

```go
RegisterMethods(openbsd.Unattended{}, WithPrefix("frontends_"), OnCluster(cluster.NameFrontends))
// structs whose task prefix is gonf's default (package_type_):
RegisterOnCluster(cluster.NameFreeBSD, freebsd.Carp{}, freebsd.NFS{}, freebsd.Zrepl{})
```

Task descriptions come from the task method's doc comment: its first
sentence, without the method name ("// Script installs /usr/local/sbin/x."
lists as "Installs /usr/local/sbin/x"). Each recipe package's
`generate.go` runs `gonf-desc`; after adding or changing a task's doc
comment run `go generate ./...` in `gonf/` and commit the `desc_gen.go`
files. A hand-written `DescX` wins, for descriptions that do not read as a
sentence about the method or use constants.

### Hosts and per-host values

| Cluster constant | Tasks / Aggregate |
|-------------------|-------------------
| `NameFrontends` | `frontends` / `frontends_*` except `Operational()` |
| `NameNetBSDPis` | `pis_netbsd` / `pis_netbsd_*` |
| `NameRockyAll` | `rocky` / `rocky_*` |
| `NameRockyPis` | `rocky_kernel_audit` / `rocky_kernel_audit_*` |
| `NameRockyK3s` | `rnodes` / `rnodes_*` |
| `NameFreeBSD` | `freebsd` / `freebsd_*` |
| `NameGarage` | `garage` / `garage_*` |

All aggregates are pattern `Aggregate`s in `gonf/tasks/tasks.go`. A pattern
never picks up an `Operational()` task, so `frontends_acme_invoke`,
`frontends_irc_bouncer` and `frontends_ping` stay by-name actions. A new
`frontends_*` task joins the `frontends` aggregate unless its `OptsX`
marks it `Operational()`.

- `OnCluster(name)` on `RegisterMethods` binds the tasks to the cluster and
  guards each task to the cluster's hosts. Bodies need no
  `WhenHostname(ClusterHosts(), ...)` wrapper. A task for part of the
  cluster narrows in a `WhenX` companion (`return
  WhenHostnameIn(carpMembers...)`), not with a body wrapper; keep
  `WhenHostname(Master, ...)` for a fragment inside a larger body.
- Per-host data is a small struct in the recipe package
  (`openbsd.UnattendedSchedule`, `rocky.UnattendedCalendar`,
  `garage.Node`, `frontends.Server`, ...), attached with `WithData(...)` in
  `cluster.Register()` and read with `EachHost(func(v T) {…})`
  (`EachHostNamed` when the host name matters, `HostData[T](host)` outside a
  loop). Every cluster member must carry the type; a missing one fails the
  record before any SSH connection. For data only some hosts carry
  (`freebsd.UPSClient`, `freebsd.CarpNode`) use `EachHostWith`, which skips
  the others. `EachHost` wraps each body in a
  destination-side hostname guard and, on single-host pushes, skips other
  hosts, so read host-specific secrets inside the body.
- Shared host options go in a `HostDefaults(...)` bundle per group. LAN
  hosts (piN, rN, fN) keep `WithSSHPort(22)` (the `lan` bundle).
- Do **not** keep parallel hostname→value maps in the recipe packages.

### Recipe style

```go
func (Unattended) OptsCron() TaskOptions {
	return TaskOptions{Needs(Unattended.Script, Unattended.Services)}
}

func (Unattended) Cron() {
	EachHost(func(s UnattendedSchedule) {
		Cron("unattended-upgrade-netbsd-pkgs",
			WithCommand("/usr/local/sbin/unattended-upgrade-netbsd pkgs"),
			WithMinute("10"), WithHour(s.PkgsHour))
	})
}

EnsureDir("/usr/local/sbin", RootOwned)
InstallFile("/usr/local/sbin/x", src, RootExec) // ordered after its parent dir
File(rcConf, WithShellVar("vm_enable", "YES"), RootOwned, WithName("rc-conf-bhyve"))
File("/etc/httpd.conf", WithContentFrom(renderHTTPD(data)), RootOwned)
CronAt("frontend-rsync", "*/5 * * * *", "-ns /usr/local/bin/rsync.sh", DependsOn(script))
Sh("systemctl restart systemd-journald", OnChange(dropIn))
Service("httpd", WithFlags(""), WithRestart, OnChange(config))
```

- `RootOwned` (0644 file, 0755 dir), `RootExec` (0755) and `RootPrivate`
  (0600 file, 0700 dir) for root plus the OS root group (wheel / root);
  `Perm(mode, Root)` for other modes; spell other groups out (`"root:bin"`,
  `"_gogios:_gogios"`).
- `Needs(T.Method)` in `OptsX` for task prerequisites (method expressions,
  so the editor jumps to them), not prose in the description. `OptsX` adds to the `RequiresRoot` default, so do not
  repeat `Privileged()`; `Unprivileged()` opts one method out (see
  `frontends_ping`). Never need an `Operational()` task: the dependent would
  drop out of its aggregate.
- One import: `. "github.com/snonux/gonf/api"` (it re-exports the resource
  options, the inventory, `Refuse` and `Dependency`). Never also dot-import
  `api/options`.
- Inventory: `WithSSHDomain` in a `HostDefaults` bundle gives
  `<name>.<domain>`, and `WithPlatform("goos/goarch")`; no per-host
  `WithSSHHost` unless a host's name differs.
- `CronAt` for fixed schedules, options for per-host fields. An identical
  unmanaged line is adopted; `WithLegacyCommand` only for a different old
  command or schedule.
- `Sh("cmd args")` when no shell is needed; `Command("sh", List("-c", ...))`
  otherwise. `Noop(name)` for a do-nothing task. `Packages("a", "b")`.
- A path inside a `Dir`/`EnsureDir` of the same plan needs no `DependsOn`.

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
