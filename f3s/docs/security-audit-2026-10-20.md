# f3s security audit — 2026-10-20

Read-only audit of the repo (k8s manifests, gonf recipes, frontends tree, git
history) plus a live read-only exposure check (nmap, nc, curl, `sshd -T`,
`pfctl -sr`, `pkg audit`, read-only `kubectl get`). Nothing was changed.
No secret values are recorded here. Each finding has an `ask` task (tag
`+security`); the task description names the finding number (`S<n>`).

f3 and the `rocky` VM were down during the live check and were not scanned.

| Finding | Task(s) | Finding | Task(s) |
|---|---|---|---|
| S1 | qm3 (router), rm3 (f-host PF), sm3 (r-node/pi2-3 firewall) | S14 | 9n3 |
| S2 | tm3 | S15 | an3 |
| S3 | um3 (f-hosts), vm3 (Pis) | S16 | bn3 |
| S4 | wm3 | S17 | cn3 |
| S5 | xm3 → ym3 → zm3 → 0n3 | S18 | dn3 |
| S6 | 1n3 | S19 | en3 |
| S7 | 2n3 | S20 | fn3 |
| S8 | 3n3 | S21 | gn3 |
| S9 | 4n3 | S22 | hn3 |
| S10 | 5n3 | S23 | in3 |
| S11 | 6n3 | S24 | jn3 |
| S12 | 7n3 | S25 | kn3 |
| S13 | 8n3 | S26 | ln3 |

## Critical

**S1. LAN hosts are reachable from the internet over IPv6.** The LAN hosts
have global SLAAC addresses in `2a01:5a8:304:bc3::/64`. From blowfish (route
via `vio0`, the public interface, gateway `fe80::1%vio0`), `nc -6 -z`
succeeded on:

- f0: 22, 111, 2049, 2222 — f1: 22, 111, 2222 — f2: 22
- r0/r1: 22, 111, 6443 (k8s API), 10250 (kubelet), 9100 (node_exporter,
  `/metrics` returns 200)
- pi2: 22, 53, 80 (Pi-hole admin needs a login; no outside DNS answers)

Re-verified by hand 2026-10-20 for r0 (`…:5a9c:fcff:fe0d:133f`: 22, 6443,
10250, 9100) and f0 (`…:eaff:1eff:fed7:1cac`: 22, 111, 2049).

Causes: the OpenWrt router (192.168.1.1) forwards inbound IPv6 from wan6 to
the LAN; f0/f1 PF is `pass in quick all` (`f3s/freebsd-hosts/relayd/pf.conf`,
`gonf/freebsd/relayd.go`), f2/f3 have no PF; firewalld is off on r0–r2
(`gonf/rnodes/base.go`, decision from task tj2 assumed the LAN is not
reachable from outside). pi0/pi1 are fine (npf `block all` default).
Combined with S3 (keyboard-interactive SSH on f-hosts) this allows password
guessing from the internet.

## High

**S2. Public frontends accept password SSH.** `doas sshd -T` on blowfish and
fishfinger: `passwordauthentication yes`, `kbdinteractiveauthentication yes`
on port 2 (`permitrootlogin no`). A no-credential probe lists
`publickey,password,keyboard-interactive`. sshd_config and doas.conf on the
frontends are not managed by gonf. These hosts hold the WireGuard mesh keys
and TLS keys.

**S3. Password-capable SSH on LAN hosts.** f0–f2: `KbdInteractiveAuthentication
yes` + `UsePAM yes` (`f3s/freebsd-hosts/f3sctl/sshd_config:68`; the
`PasswordAuthentication no` only applies inside `Match User f3sctl`), `paul`
has a password and `permit nopass :wheel` (`base/doas.conf:10`). pi0/pi1:
`PasswordAuthentication yes` from the bake (`f3s/pi-netbsd/bake/setup.sh:56-63`),
not managed by gonf afterwards; the bake leaves the plaintext seed in
`~/Downloads/seed-piN/setup.sh` (`bake-golden.sh:46-55`). pi2:
`passwordauthentication yes` (pi3 not confirmed). r0–r2 are correct
(`f3s/r-nodes/base/sshd-00-auth.conf`).

**S4. Frontend PF passes everything inbound.** `frontends/etc/pf.conf.tpl:17-18`
has `block return` followed by a bare `pass`; the labelled rules (66-100) only
count. Publicly reachable as a result: httpd plain backend `*:8080`
(`gonf/frontends/assets/httpd.conf.tmpl:28`, directory index of `foostats/`,
`gogios/`; bypasses relayd Host blocks; `log style forwarded` trusts
spoofable XFF), rsyncd 873 as root from inetd (`frontends/etc/inetd.conf:2`,
module list readable), ZNC 4242 on fishfinger (self-signed `CN=host.unknown`),
dserver 2222 (`assets/dtail.json` `SSHBindAddress 0.0.0.0`). No SSH
brute-force rate limiting.

**S5. Unsigned in-house package repos, installed as root daily.**
`f3s/freebsd-hosts/base/pkg-custom.conf:4` `signature_type: "NONE"`;
`packages/scripts/pkg-freebsd.sh:49` and `pkg-dtail-freebsd.sh:79` run
`pkg repo` without a key; Rocky clients use `gpgcheck=0`; NetBSD `pkg_add`
installs unsigned packages (f3s skill `pkgrepo/repo-architecture.md:76-79`,
`client-setup.md:133`). Only OpenBSD packages are signed. The repo content
lives on NFS (`/data/nfs/k3svolumes/pkgrepo`, see S7), is served by an
HTTP-only ingress (`f3s/pkgrepo/helm-chart/templates/ingress.yaml:7`) behind
the internet-facing frontends. Root on a frontend, an r-node, or the NFS
data means root on every FreeBSD/Rocky/NetBSD host within a day. Related:
the OpenBSD signing key is used on fishfinger
(`packages/buildvm/provision.sh:53`) and the local
`packages/buildvm/custom-pkg.sec` is 0644 (gitignored, not tracked).

**S6. Unauthenticated plain-HTTP container registry.**
`f3s/registry/helm-chart/templates/service.yaml:13-14` (NodePort 30001 on
every r-node), no `REGISTRY_AUTH*` in `deployment.yaml`;
`curl http://r0.lan.buetow.org:30001/v2/_catalog` returns 200 anonymously.
12 live workloads pull from it by mutable tag (filebrowser, immich-postgres,
bgtutor, radicale, ychat, player, shuriken, anki, …). Push is very likely
anonymous too (not tested). Not reachable publicly.

**S7. NFS export gives uid 0 over all of `/data/nfs` to any local process.**
`f3s/freebsd-hosts/nfs/exports:1-2` `-sec=sys -alldirs -maproot=root
-network 127.0.0.1`; `gonf/freebsd/nfs.go:66,77` allow non-reserved source
ports; `set skip on lo0` in the f0/f1 pf.conf. Any unprivileged process on
the storage MASTER can connect to 127.0.0.1:2049 as root. On r-nodes the
stunnel entry `127.0.0.1:2323` (`gonf/rnodes/nfsclient.go:37`) is open to
every local process / hostNetwork pod. 2049/111 were also reachable over
public IPv6 on f0 (S1).

**S8. dserver key-cache script: `_dserver`/`dserver` → root via symlink.**
`frontends/scripts/dserver-update-key-cache{,-freebsd,-netbsd}.sh.tpl:20-22`
and `packages/files/dtail-rocky/dserver-update-key-cache.sh:17-19` run as
root and `cp`/`chown dserver`/`chmod` into the dserver-owned
`/var/run/dserver/cache`. A planted symlink makes root overwrite and chown
any file (e.g. `/etc/doas.conf`, `/etc/shadow`); user `authorized_keys`
symlinks are followed too. dserver listens on `0.0.0.0:2222` everywhere,
offers `password,publickey` (password path only serves built-in health
users), and `Permissions.Default` is `readfiles:^/.*$`.

## Medium

**S9. relayd routes any Host header to Traefik.** The first `forward to <f3s>
port 80` in `gonf/frontends/assets/relayd.conf.tmpl:73,85` catches unmatched
hosts (`gonf/frontends/web.go:451-453`); all `*-ingress-lan` ingresses bind
`web,websecure`. `curl -k -H 'Host: immich.f3s.lan.buetow.org'
https://blowfish.buetow.org/` returns 200. The `code.f3s.buetow.org` block
(`relayd.conf.tmpl:55`) is bypassed with `Host: code.f3s.lan.buetow.org` or
a `:443` suffix. Any future LAN-only ingress (e.g. the undeployed
`f3s/prometheus/grafana-ingress`) becomes public silently.

**S10. Forwarding headers from clients reach the apps.** relayd only sets
X-Forwarded-For/-Proto (`relayd.conf.tmpl:47-48,118-119`); Traefik trusts
forwarded headers from all RFC1918 ranges
(`f3s/traefik-config/helm-chart/templates/helmchartconfig.yaml:14-15`).
`X-Real-Ip`, `X-Forwarded-Host/Port/Prefix`, `Forwarded` pass through
unchanged (client-IP spoofing, host-header poisoning). No HSTS on any site;
keybr redirects https → `http://keybr.f3s.buetow.org/`.

**S11. Prometheus admin API and write receivers on NodePort 30090.**
`f3s/prometheus/persistence-values.yaml:49,51,57`
(`web.enable-admin-api`, remote-write and OTLP receivers),
`f3s/prometheus/manifests/prometheus-nodeport.yaml:9,13`. Anonymous from
LAN/WG/any pod: `delete_series`, `clean_tombstones`, fake series injection
— blinds alerting.

**S12. No NetworkPolicies outside `cicd`.** A compromised internet-facing
app reaches immich/miniflux postgres, protonbridge, pushgateway, Prometheus
admin (S11) and registry push (S6) directly.

**S13. ArgoCD is public with a local admin password only.**
`f3s/argocd/ingress.yaml:11,15`, `gonf/frontends/data.go:82`; live
`argocd-cm` `admin.enabled: true`, no dex/OIDC. One password guards
cluster-wide deploy rights from the internet.

**S14. Unauthenticated writable rsync modules on the frontends.**
`gonf/frontends/assets/rsyncd.conf:13-29`: `irregular-ninja` modules
`read only = no`, no `auth users`, `list = yes`, host-name based allow incl.
`localhost`. `frontends/scripts/rsync.sh.tpl:7` pulls third-party content as
root with `rsync -av --delete` (owners/modes kept, no `--safe-links`).

**S15. Public, reversible visitor IP hashes.** `frontends/scripts/foostats.pl:126-129`
unsalted `sha3_512_base64($ip)`, per-IP JSON in the public web root with
auto index (`httpd.conf.tmpl:27-33`), fetched by the partner at
`https://$partner/foostats/` (line 560). IPv4 is cheap to brute-force.

**S16. Vulnerable packages on the FreeBSD hosts.** `pkg audit`: f0 7 problems
in 6 packages (curl-8.22.0, python312-3.12.15, expat-2.8.4,
py312-setuptools-63.1.0_3, libXtst, libXrender); f1/f2 4 problems (curl,
python312, expat). Unattended upgrades did not clear them.

**S17. Garage admin/RPC/S3 on 0.0.0.0, no metrics token.**
`gonf/garage/garage.go:95-100` binds 3900/3901/3903 on all addresses;
`metrics_token` unset (anonymous S3 and admin v2 do return 403).

## Low

**S18. Frontend hardening batch.** `/tmp/f3s_taken_down` mute marker is in
world-writable `/tmp` (`gonf/frontends/monitoring.go:23`) — any local user
can silence alerts; `/etc/gogios.json` mode 0744 (`monitoring.go:116`);
goprecords token on the curl command line, visible in `ps`
(`frontends/scripts/goprecords-upload-client.sh:31-32`, used on all client
hosts); znc missing from `gonf/openbsd/assets/unattended-upgrade-services`;
nrpe installed but unused (`monitoring.go:110`); the remote-access doc uses
`ssh -A` to fishfinger (f3s skill `network/remote-access.md`) — forwarded
agent is usable by root on the gateway; `access.log` 0644
(`frontends/etc/newsyslog.conf:18`).

**S19. Mail/DNS authentication.** No DNSSEC, no CAA; SPF/DMARC only for
paul.buetow.org (DMARC `p=none`); other zones can be spoofed as mail senders.
(AXFR is correctly refused / TSIG-restricted; smtpd is not an open relay.)

**S20. CARP password world-readable.** `gonf/freebsd/carp.go:265-266` writes
the vhid password into `/etc/rc.conf` with `RootOwned` (0644).

**S21. stunnel and zrepl peer identity.** stunnel `verify = 2` without
`checkHost`, server and client certs from the same CA, no CRL
(`f3s/freebsd-hosts/nfs/stunnel.conf:11-13`, `gonf/rnodes/nfsclient.go:31-34`).
zrepl uses the `tcp` transport with source-IP identity
(`zrepl.yml.tmpl:19,48-52`) — acceptable over WireGuard /32 + PSK, sends are
raw-encrypted.

**S22. Protonbridge IMAP/SMTP on NodePorts.**
`f3s/protonbridge/helm-chart/templates/service.yaml:9,18,26`; 30143
advertises `AUTH=PLAIN` before STARTTLS to every LAN device.

**S23. Pod hardening.** ~21 non-system containers not forced non-root
(registry, pkgrepo, pushgateway, immich stack, both postgres, miniflux,
protonbridge, radicale, syncthing, wallabag); all automount SA tokens (no
bindings grant them anything today). No privileged/hostNetwork pods besides
node-exporter.

**S24. Unattended-upgrade gaps.** k3s is installed via `curl | sh`
(f3s skill `k3s/install.md:29`) and never upgraded automatically; the `rocky`
VM on f3 is not in the gonf inventory (`gonf/cluster/cluster.go`); f3 never
auto-reboots for kernel fixes.

**S25. Leaked credentials in public git history (neutralised).**
`eff2c9c` (removed in `5f06401`) committed an ed25519 private key
(`f3s/argocd/git-server-repo-creds.yaml`, fingerprint
`SHA256:O6Y3HIEDggtnaEcAJ3bxejyfAl6EM2mS/ZdPyaecbDY`); `8945a45` and `5e11e59`
committed ArgoCD admin password material. Verified rotated/dead, but the key
must be confirmed absent from every `authorized_keys` and Forgejo deploy key.
No pre-commit secret scanning exists.

**S26. Retired babylon5 scripts.** `babylon5/` (retired host) contains
Vaultwarden `:latest` on `0.0.0.0:90` over HTTP without `SIGNUPS_ALLOWED` /
`ADMIN_TOKEN`, Nextcloud AIO mounting `docker.sock`.

## Checked and OK

- No secret tracked in the current tree (`gonf/secrets`, `frontends/secrets`
  only README/.gitignore; `additional-scrape-configs-secret.yaml` holds only
  scrape targets).
- TLS on 443: TLS 1.2/1.3 only; 2443/1965 TLS 1.3, valid certs.
- AXFR refused on both nsd servers; no open recursion; Pi-hole not an open
  resolver from the internet; smtpd not an open relay.
- OpenBSD 7.9 frontends: no pending syspatch or package updates. r0–r2: no
  pending security updates, key-only SSH.
- All public app endpoints require auth (ArgoCD API, filebrowser, miniflux,
  immich, opodsync, webdav, radicale, player, syncthing, wallabag);
  monitoring hosts return 404 publicly.
- k8s: anonymous 6443/10250 → 401; cluster-admin only for system defaults;
  no privileged pods; Garage anonymous S3/admin → 403.
- bhyve VNC on loopback; node_exporter bound to wg0 on the frontends and
  f-hosts; WireGuard /32 AllowedIPs with PSKs; f3sctl agent locked down.
- pi2/pi3 kernel age is already covered by task 752 (user kept Rocky SIG
  AltArch, 2026-09-22) and is not re-filed.
