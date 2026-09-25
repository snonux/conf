# conf

Personal homelab and server configuration: gonf recipes, k3s manifests and
Helm charts, host scripts.

## Layout

| Path | Content |
|---|---|
| `gonf/` | gonf recipes (Go module `github.com/snonux/conf/gonf`), run with `./gonf.sh` |
| `gonf.sh` | wrapper: `cd gonf && go run ./cmd/gonf "$@"` |
| `frontends/` | assets for the OpenBSD frontends blowfish and fishfinger, plus scripts for every OS ([README](frontends/README.md)) |
| `f3s/` | f3s homelab: k3s apps and Helm charts, ArgoCD apps, FreeBSD host scripts, Pi setup, docs |
| `packages/` | `make` targets that build and upload custom packages (gogios, f3sctl, dtail) to `pkgrepo.f3s.buetow.org` |
| `docs/archive/` | incident reports, plans, test results and other records ([README](docs/archive/README.md)) |
| `babylon5/` | old Docker start scripts of a retired server |
| `playground/` | scratch scripts |
| `rcm/` | Rakefile for the rcm task runner (`~/git/rcm`) |
| `AGENTS.md` | rules for coding agents working in this repo |

## gonf

The library is `github.com/snonux/gonf` (`~/git/gonf`, its
[reference](https://github.com/snonux/gonf/blob/main/docs/reference.md)).
This repo only holds recipes: `gonf/cmd/gonf/main.go` (secret provider),
`gonf/cluster/` (inventory), `gonf/tasks/tasks.go` (task registration and
aggregates), one package per host group.

```sh
./gonf.sh -list                                    # all tasks
./gonf.sh hosts | clusters                         # inventory
./gonf.sh cluster <cluster> <task>...              # push to every host of a cluster
./gonf.sh cluster -n <cluster> <task>...           # preview, applies nothing
./gonf.sh push -- -p 2 rex@blowfish.buetow.org <task>...   # one host, ssh args after --
./gonf.sh -privilege=doas plan -o /tmp/plan <task>...      # record only, read plan.jsonl
```

`./gonf.sh` works from any directory; relative path arguments are relative
to `gonf/`. It sets `GONF_CONF_ROOT` when run from a checkout other than
`~/git/conf` (e.g. a worktree). A preview still installs or updates the gonf
binary on a destination that lacks a current one. Pi, r-node and f-host
entries use SSH port 22 explicitly because `~/.ssh/config` maps
`*.buetow.org` to port 2.

Recipe style (conventions in [`AGENTS.md`](AGENTS.md)):

```go
// gonf/tasks/tasks.go: bind the group to a cluster, guard every task to it
RegisterMethods(freebsd.Unattended{}, WithPrefix("freebsd_"), OnCluster(cluster.NameFreeBSD))

// gonf/cluster/cluster.go: typed per-host data
Host("f0", fhost, WithData(freebsd.UnattendedSchedule{Minute: "5"})) // fhost: WithSSHDomain("lan.buetow.org"), WithPlatform("freebsd/amd64")

// a task: prerequisites, ownership, per-host data, cron
func (Unattended) OptsCron() TaskOptions { return TaskOptions{Needs("script", "services", "stamp_dir")} }
func (Unattended) Script() {
	EnsureDir("/usr/local/sbin", Perm(0o755, Root)) // Root: root:wheel on BSD, root:root on Rocky
	InstallFile("/usr/local/sbin/unattended-upgrade-freebsd", src, Perm(0o755, Root))
}
func (Unattended) Cron() {
	EachHost(func(s UnattendedSchedule) {
		Cron("unattended-upgrade-freebsd-daily", WithCommand(cmd), WithMinute(s.Minute))
	})
}
CronAt("frontend-rsync", "*/5 * * * *", "-ns /usr/local/bin/rsync.sh")
Service("httpd", WithFlags(""), WithRestart, OnChange(config))
```

### Hosts and clusters

Inventory: [`gonf/cluster/cluster.go`](gonf/cluster/cluster.go) (SSH target,
privilege, OS, per-host schedule values). Host details: the `f3s` skill
family.

| Cluster | Hosts | SSH / privilege | Parallel |
|---|---|---|---|
| `frontends` | blowfish, fishfinger | `rex@<host>.buetow.org:2`, doas | 5 |
| `netbsd-pis` | pi0, pi1 | `paul@piN.lan.buetow.org`, doas | 5 |
| `rocky-pis` | pi2, pi3 | `paul@piN.lan.buetow.org`, sudo | 5 |
| `pis` | pi0-pi3 (mixed OS, only for OS-gated shared tasks) | | 5 |
| `rocky-k3s` | r0, r1, r2 | `root@rN.lan.buetow.org`, sudo | 3 |
| `rocky-all` | pi2, pi3, r0, r1, r2 | | 5 |
| `freebsd-hosts` | f0, f1, f2, f3 | `paul@fN.lan.buetow.org`, doas | 5 |
| `garage` | f0, f1, f2 | | 1 |

### Tasks

61 tasks. Each group targets one cluster (`OnCluster`) and applies only on
its hosts. An aggregate runs its whole group, except the three `Operational()`
by-name tasks marked below. A task's "needs" are recorded before it
(`Needs`), also when it runs alone.

```sh
./gonf.sh cluster frontends frontends
./gonf.sh cluster netbsd-pis pis_netbsd
./gonf.sh cluster rocky-all rocky
./gonf.sh cluster rocky-k3s rnodes
./gonf.sh cluster freebsd-hosts freebsd
./gonf.sh cluster garage garage
```

Frontends (cluster `frontends`):

| Task | Does |
|---|---|
| `frontends` | aggregate of all below except the three marked "by name" |
| `frontends_acme` | ACME client config and daily renewal hook |
| `frontends_acme_invoke` | request/renew certificates; by name, not in the aggregate |
| `frontends_base` | base packages and local admin helpers |
| `frontends_cron` | root cron for unattended-upgrade base/pkgs/audit/reboot (needs `frontends_script`, `frontends_services`) |
| `frontends_d_tail` | DTail from the signed fleet repo, enables dserver |
| `frontends_dns_failover` | DNS failover script and cron |
| `frontends_foostats` | Foostats reporting, daily hook, log rotation |
| `frontends_gemtexter` | daily Gemtexter content updater |
| `frontends_gogios` | Gogios monitoring, checks, schedule, status page |
| `frontends_goprecords` | optional daily uptimed upload to goprecords |
| `frontends_httpd` | render, validate, converge httpd |
| `frontends_inetd` | inetd |
| `frontends_irc_bouncer` | ZNC IRC bouncer on fishfinger; by name, not in the aggregate |
| `frontends_myname` | hostname |
| `frontends_newsyslog` | unattended-upgrade log rotation |
| `frontends_nsd` | render, validate, converge authoritative NSD zones |
| `frontends_pf` | validate and reload PF, PF node_exporter metrics |
| `frontends_ping` | push-pipeline diagnostic; by name, not in the aggregate |
| `frontends_pkg_repo` | signed fleet package repo for `pkg_add` |
| `frontends_relayd` | render, validate, converge relayd (needs certs: `frontends_acme` + one `frontends_acme_invoke`) |
| `frontends_rsync` | rsyncd config and sync cron |
| `frontends_script` | `/usr/local/sbin/unattended-upgrade` |
| `frontends_service_accounts` | disabled Gorum service account |
| `frontends_services` | `/etc/unattended-upgrade-services` |
| `frontends_smtpd` | render, validate, converge OpenSMTPD (needs certs like relayd) |
| `frontends_uptimed` | uptimed |
| `frontends_wire_guard_hosts` | WireGuard mesh host entries in `/etc/hosts` |

NetBSD Pis (cluster `netbsd-pis`):

| Task | Does |
|---|---|
| `pis_netbsd` | aggregate `^pis_netbsd_` |
| `pis_netbsd_script` | `/usr/local/sbin/unattended-upgrade-netbsd` |
| `pis_netbsd_services` | `/etc/unattended-upgrade-services` |
| `pis_netbsd_cron` | root cron pkgs/reboot (needs script, services) |
| `pis_netbsd_newsyslog` | log rotation |
| `pis_netbsd_vuln_audit_packages` | curl for the audit |
| `pis_netbsd_vuln_audit_script` | `/usr/local/sbin/netbsd-vuln-audit` |
| `pis_netbsd_vuln_audit_state_dir` | `/var/db/netbsd-vuln-audit` |
| `pis_netbsd_vuln_audit_cron` | daily audit cron |

Rocky (cluster `rocky-all`, kernel audit `rocky-pis`):

| Task | Does |
|---|---|
| `rocky` | aggregate `^rocky_` (includes the kernel audit tasks) |
| `rocky_gonf_link` | `/usr/bin/gonf` -> `/usr/local/bin/gonf` (sudo `secure_path`) |
| `rocky_packages` | ksh, yum-utils |
| `rocky_script` | `/usr/local/sbin/unattended-upgrade-rocky` |
| `rocky_stamp_dir` | `/var/lib/unattended-upgrade` |
| `rocky_units` | unattended-upgrade-rocky timer (hourly, per-host minute) |
| `rocky_logrotate` | `/etc/logrotate.d/unattended-upgrade` |
| `rocky_kernel_audit` | aggregate `^rocky_kernel_audit_`, pi2/pi3 only |
| `rocky_kernel_audit_packages` | ksh, unzip, jq, curl |
| `rocky_kernel_audit_script` | `/usr/local/sbin/rocky-kernel-audit` |
| `rocky_kernel_audit_state_dir` | `/var/lib/rocky-kernel-audit` |
| `rocky_kernel_audit_units` | daily timer |

r-nodes (cluster `rocky-k3s`):

| Task | Does |
|---|---|
| `rnodes` | aggregate `^rnodes_` |
| `rnodes_nfs_mount_monitor` | NFS mount monitor (repairs mounts, reaps stuck pods) |
| `rnodes_persistent_journal` | bounded persistent journald |

FreeBSD (cluster `freebsd-hosts`) and Garage (cluster `garage`):

| Task | Does |
|---|---|
| `freebsd` | aggregate `^freebsd_` |
| `freebsd_packages` | ksh93 |
| `freebsd_script` | `/usr/local/sbin/unattended-upgrade-freebsd` |
| `freebsd_services` | `/etc/unattended-upgrade-services` |
| `freebsd_stamp_dir` | `/var/lib/unattended-upgrade` |
| `freebsd_cron` | hourly root cron (needs script, services, stamp dir) |
| `freebsd_newsyslog` | log rotation |
| `garage` | aggregate `^garage_` |
| `garage_config` | Garage TOML on f0-f2 (config only; Garage must already be installed) |

A new `frontends_*` task joins the `frontends` aggregate unless its `OptsX`
marks it `Operational()`.

Unattended upgrades across all OSes:
[`frontends/docs/unattended-upgrades.md`](frontends/docs/unattended-upgrades.md).

### Secrets

Recipes use `MustSecret`/`OptionalSecret` with logical paths from
`gonf/paths`. Mapped references come from the foostore/KeePass vault
(`Infra/nsd-tsig-key`, `Infra/garage-rpc`,
`Infra/goprecords-token-{blowfish,fishfinger}`, Password field); everything
else falls back to files under `gonf/secrets/`. A mapped secret the vault
can't read fails the plan, it never falls back. Prerequisites, bootstrap and
policy: [`gonf/secrets/README.md`](gonf/secrets/README.md). Never commit or
print secret values.

## f3s

| Doc | Topic |
|---|---|
| [`f3s/argocd/`](f3s/argocd/README.md), [`f3s/argocd-apps/`](f3s/argocd-apps/README.md) | GitOps; every app and its namespace |
| [`f3s/forgejo/`](f3s/forgejo/README.md) | git forge, source for ArgoCD |
| [`f3s/cert-manager/`](f3s/cert-manager/README.md), [`f3s/docs/lan-access-setup-guide.md`](f3s/docs/lan-access-setup-guide.md) | `*.f3s.lan.buetow.org` path and TLS |
| [`f3s/docs/nfs-sentinel-initcontainer.md`](f3s/docs/nfs-sentinel-initcontainer.md) | NFS sentinel pattern for hostPath PVs |
| [`f3s/docs/taskwarrior-s3-sync.md`](f3s/docs/taskwarrior-s3-sync.md) | Taskwarrior sync to Garage |
| [`f3s/prometheus/`](f3s/prometheus/README.md), [`f3s/pushgateway/`](f3s/pushgateway/README.md), [`f3s/loki/`](f3s/loki/README.md), [`f3s/tempo/`](f3s/tempo/README.md) | monitoring |
| [`f3s/registry/`](f3s/registry/README.md) | private registry, NodePort 30001 |
| [`f3s/pihole/`](f3s/pihole/README.md) | Pi-hole on pi2/pi3, LAN wildcard DNS |
| [`f3s/pi-netbsd/`](f3s/pi-netbsd/README.md) | NetBSD on pi0/pi1: rebuild procedure, upgrade lessons |
| [`f3s/freebsd-hosts/keys/`](f3s/freebsd-hosts/keys/README.md), [`zusb/`](f3s/freebsd-hosts/zusb/README.md), [`f3sctl/`](f3s/freebsd-hosts/f3sctl/README.md), [`shelly-fans/`](f3s/freebsd-hosts/shelly-fans/README.md) | FreeBSD host scripts |
| [`f3s/shuriken/`](f3s/shuriken/README.md), [`f3s/beets-art/`](f3s/beets-art/helm-chart/README.md), [`f3s/immich/`](f3s/immich/README.md), [`f3s/jellyfin/`](f3s/jellyfin/README.md), [`f3s/navidrome/`](f3s/navidrome/helm-chart/README.md) | media workloads |
| other `f3s/<app>/README.md` | per-app notes |

Most app directories have the same `Justfile`: `status`, `logs`,
`port-forward`, `sync` (ArgoCD refresh), `argocd-status`, `restart`.
