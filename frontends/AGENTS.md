# Frontend infrastructure knowledge

OpenBSD frontends blowfish and fishfinger. Deployed with gonf from
`../gonf/frontends` (tasks `frontends_*`); this directory holds the assets.
Rex and its Perl `.tpl` templates are gone (git history only); the Go
renderers keep the same rules.

## Request flow

```
Internet -> relayd :443 -> routing decision -> httpd :8080 (<localhost>) or f3s k3s :80 (<f3s>, 192.168.2.120-122)
```

- relayd terminates TLS on 443 (IPv4 and IPv6) and routes on the Host header.
  No explicit match falls through to the default backends: f3s first, then
  localhost.
- httpd listens on 8080 (behind relayd) and on 80 for ACME challenges and
  HTTP->HTTPS redirects, with a dedicated block for each server's own FQDN.
- Routing is explicit: an httpd server block alone does not make relayd send
  traffic there.

## gonf model

- `data.go`: topology lists `acmeHosts`, `f3sHosts`, `garageBuckets`,
  `prefixes` (`''`, `www.`, `standby.`) and the per-site policy
  (`Site`/`SiteFor`: single-family ipv4./ipv6. names via `Site.Family`,
  frontend FQDNs via `Site.FrontendHost`, f3s sites, dedicated TLS ports,
  expected HTTPS statuses, relayd upstreams). `TemplateData` appends the f3s
  hosts and Garage bucket names to the ACME hosts.
- `web.go`, `maildns.go`, `monitoring.go`, `acme.go` render httpd, relayd,
  NSD, Gogios and ACME per frontend on the controller (`EachHost` over the
  inventory `Server` values, `WithData` in `gonf/cluster`).
- `acmeHosts` get ACME certs, port-80 challenge blocks and explicit relayd
  routes to `<localhost>`. A host not in it falls through to f3s.
- `f3sHosts` get httpd's cluster-down fallback page (rewrite to
  `/index.html`) and relayd routes to the f3s backends; they're excluded from
  the `<localhost>` routes.

## Server FQDNs vs shared hosts

Shared service domains (foo.zone, irregular.ninja, f3s.buetow.org): same on
both servers, both hold the certs, expanded with all prefixes.

Server FQDNs (blowfish.buetow.org, fishfinger.buetow.org): each server has
only its own cert and there are no www./standby. records. They stay in
`acmeHosts` for routing, but:

1. ACME/port-80 loop: skip the current server's FQDN; it has its dedicated
   block. Otherwise httpd fails with "server defined twice".
2. relayd `tls keypair` loop: skip both server FQDNs and load
   `tls keypair <hostname>.<domain>` once. Otherwise relayd fails on the
   missing partner cert.
3. Gogios checks: server FQDNs get bare-hostname checks only, never prefix
   variants (those would be 12 false CRITICALs from DNS failures).

## Unattended updates and package audit

Deployed by `frontends_script` and `frontends_cron`. The `audit` mode is a
native `pkg_add -Iun` audit using signed `quirks` metadata plus the official
and fleet repos, not a CVE scanner. Clean runs are only logged; findings and
failures mail root and exit non-zero. Never let the audit fall back to
official packages alone: that would silently omit the fleet packages.
Reference: `docs/unattended-upgrades.md`.

## Mail and authoritative DNS

`frontends_smtpd`, `frontends_nsd`, `frontends_dns_failover`. Both daemons
stage controller-rendered candidates and validate before touching live files:
`smtpd -n -f`; `nsd-checkzone` per zone then `nsd-checkconf`. Restart fan-in
is deliberate: aliases only run `newaliases`; smtpd restarts for config or
tables; nsd for key, config, zones or flags.

NSD TSIG key: controller secret
`gonf/secrets/frontends/var/nsd/etc/nsd_key.txt` (0600, vault-mapped, see
`../gonf/secrets/README.md`). Use the gonf secret helper; never put the value
in a resource name, description, command argument, log line or stdout plan.

Cron jobs adopt an identical unmarked crontab line (same schedule and exact
command, no `NAME=value` line after it): it is replaced by the gonf block.
Any other difference means it isn't adopted and runs next to the gonf job,
so check `crontab -l` before changing a command or schedule; for a
different old line add `WithLegacyCommand(old)`. No raw crontab surgery in
scripts.

rc flags: `Service(name, WithFlags(""))` (httpd, relayd, inetd) and
`WithFlags(...)` (node_exporter) set them through `rcctl set`. nsd keeps its
`nsd_flags=` line in `/etc/rc.conf.local` (`nsdFlags`): its rc.d script has
default flags, so empty flags would never converge. `pkg_scripts` is a
plain line too (`rcConfLocalLine`).

## Deploy and test

```sh
./gonf.sh cluster frontends frontends_httpd frontends_relayd
```

Plans are rendered per host, pushed to both in parallel, validate
`/etc/httpd.conf` and `/etc/relayd.conf` before a live write, and restart
only after a changed, validated config.

```sh
ssh -p 2 rex@<server> "doas httpd -n; doas relayd -n; doas smtpd -n"
ssh -p 2 rex@<server> "doas nsd-checkconf /var/nsd/etc/nsd.conf"
ssh -p 2 rex@<server> "doas nsd-checkzone <zone> /var/nsd/zones/master/<zone>.zone"
ssh -p 2 rex@<server> "doas rcctl check httpd relayd smtpd nsd"
```

## Gogios

- User `_gogios`, config `/etc/gogios.json` (`renderGogios` in
  `../gonf/frontends/monitoring.go`), output
  `/var/www/htdocs/buetow.org/self/gogios/index.html`, state
  `/var/run/gogios/state.json`.
- Cron every 5 minutes 08:00-22:00. `RunInterval` per check is independent:
  a 3600 s TLS check only reruns when its interval expired.

## Raspberry Pi roles (for monitoring)

- pi0/pi1: NetBSD 11.0, static HTTP on 80 with base-system bozohttpd. Check
  `http://pi0.lan.buetow.org` and `http://pi1.lan.buetow.org`. Validate NPF
  and its TCP 22/80/2222 rules.
- pi2/pi3: Pi-hole DNS on 53 and admin UI on 80. Check DNS against both and
  the admin UI. `firewall-cmd --state` guidance applies to these Rocky hosts
  only.
- relayd can't rewrite paths, so bozohttpd uses directory vhosts under
  `/var/www/html` (config in `/etc/rc.d/bozohttpd`, `/etc/rc.conf`):
  `f3s.buetow.org` (www., standby. are symlinks) and `snonux.foo`
  (www. symlink). relayd passes Host through unchanged.

## relayd file descriptors (manual, not gonf)

relayd loads 67+ keypairs. With the default daemon class limit of 1024 open
files SNI silently fails and relayd serves the first cert (`foo.zone`). The
daemon class in `/etc/login.conf` on both frontends is therefore:

```
daemon:\
        :ignorenologin:\
        :datasize=4096M:\
        :maxproc=infinity:\
        :openfiles-max=4096:\
        :openfiles-cur=4096:\
        :stacksize-cur=8M:\
        :tc=default:
```

This is a manual edit, not managed by gonf. Re-apply after an OpenBSD upgrade
replaces `/etc/login.conf`, then:

```sh
doas rm /etc/login.conf.db && doas cap_mkdb /etc/login.conf && doas rcctl restart relayd
doas relayd -dvv 2>&1 | grep socket_rlimit | head -1    # max open files 4096
```

Never ship `/etc/login.conf.d/daemon`: OpenBSD uses a fragment instead of
the whole class, so relayd would lose `ignorenologin`, `datasize`, `maxproc`
and `stacksize-cur`. `frontends_relayd` removes `/etc/login.conf.d/daemon` and
any `daemon.db` (`NoLoginClass("daemon")`) and restarts relayd once if it
removed something. The `inetd` fragment stays; its `tc=daemon` resolves
against `/etc/login.conf`.

## blowfish disk layout (manual, not gonf)

blowfish's `/var` is only 3.5G, and the irregular.ninja photo site outgrew it
(74% on 2026-09-25). The empty `/usr/obj` partition (`sd0j`, 5.8G; nothing
builds from source here) is now mounted at `/var/www/htdocs/irregular.ninja`:
fstab line `31bfd9d9a6788844.j /var/www/htdocs/irregular.ninja ffs
rw,nodev,nosuid,noexec 1 2`, placed after the `/var` line so `/var` mounts
first. Backup of the old fstab: `/etc/fstab.bak-20260925`. fishfinger has a
5G `/var` and keeps the stock layout. A reinstall of blowfish must recreate
this mount before restoring the site.

## OpenBSD release upgrades (sysupgrade)

`sysupgrade` fails if `/var/www` is a symlink. It used to point to
`/home/var/www` for disk-space reasons (see the foo.zone post "One reason why I
love OpenBSD", note to myself): undo such a symlink before the upgrade and
restore it afterwards. As of 2026-09-25 `/var/www` is a real directory on both
gateways; fishfinger still has the old link parked as `/var/www.DELETEME`, and
blowfish mounts `sd0j` inside it at `/var/www/htdocs/irregular.ninja` (see the
disk-layout section above) — unmount that for the upgrade if in doubt, and
check `/var` and `/usr` free space first. Then `sysupgrade`, `sysmerge`,
`pkg_add -u`, re-apply the relayd openfiles limit if `login.conf` was
replaced, and a gonf dry-run of the frontends aggregate.

7.8 -> 7.9 status: fishfinger upgraded 2026-09-26 (sysupgrade, 21 syspatches,
`pkg_add -u`, `sysmerge -b` touched only cert.pem and X11 files, `login.conf`
unchanged); blowfish still on 7.8 pending the user's go. `/var/gemini` and
`/var/www.DELETEME` (symlinks into /home) are not base-set paths and did not
disturb sysupgrade. Expect Gogios HTTPS criticals for a check run that lands
in the first minutes after boot: on 2026-09-26 the first cron run after
fishfinger's reboot (09:20) loaded the 1G host, relayd's `check http "/"` on
`<localhost>` failed briefly (09:20:50-59), and requests routed there ended as
"session failed" (znc.buetow.org HTTPS IPv4/IPv6 "No data received"). The
HTTPS checks rerun every 300 s and cleared on their own; the hourly TLS
checks keep a failure up to an hour. The custom pkgrepo only has an
`openbsd/7.8/` tree; fishfinger keeps using it until both frontends are on
7.9.
