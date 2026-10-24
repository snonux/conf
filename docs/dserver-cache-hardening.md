# dserver cache hardening (task 3n3)

This change is prepared locally. **Every live write, package install, migration,
service stop/start/restart and deployment below requires explicit user approval.**
No live change, service start, host wake or recovery boot was performed during
preparation. Keep task 3n3 pending until the fleet's runtime acceptance passes.

## Behavior and compatibility

All four shipped cache scripts require root-owned, non-symlink runtime and cache
directories, mode 0750 with the daemon's primary group. Existing unsafe trees are
refused rather than recursively repaired. Keys are staged privately on the same
filesystem, set to root:daemon-group 0640, then atomically renamed over the old
entry. Destination symlinks, hardlinks and FIFOs never have their targets written,
chowned or chmodded; directories and symlinks to directories fail closed.

Source keys must be regular, non-symlink, single-link files owned by their login
user, with no symlink at the home or `.ssh` component. The source user opens the
file through `su -m`, so a path swap cannot expose a root-only file. Invalid or
removed sources revoke stale cache entries. Linux retains the existing root-key
support; BSD retains its `/home/*` source selection. A concurrent refresh fails
without touching another run's staging files. If SIGKILL leaves a lock behind,
verify no refresh is running before root removes `/var/run/dserver/key-cache.lock`.

`gonf/dtail` renders each host's configured address from typed inventory data.
Frontend binding is WireGuard-only; lab hosts bind their existing LAN address so
normal `*.lan.buetow.org` DTail clients continue to work. Default and user-specific
all-files permissions and example users are replaced with log paths only:
`/var/log/` and `/var/www/logs/`. Existing OS log permissions still apply. System
sshd authentication and password policies are unchanged. dserver's built-in
password-based health workflow is unchanged.

| Hosts | Bind address(es), TCP 2222 | Login for acceptance |
| --- | --- | --- |
| blowfish, fishfinger | 192.168.2.110, 192.168.2.111 | rex |
| f0–f3 | 192.168.1.130–133 | paul |
| r0–r2 | 192.168.1.120–122 | root |
| pi0, pi1 | 192.168.1.125, 192.168.1.126 | paul |
| pi2, pi3 | 192.168.1.127, 192.168.1.128 | paul |

BSD host keys remain at `/var/db/dserver/ssh_host_key`. Linux uses the persistent
`/var/lib/dserver/ssh_host_key`, separate from the root-owned authorization cache.
Both the Linux recipe and systemd startup refuse a missing/empty persistent key;
they do not silently rotate the server's identity. Linux's working directory is
its systemd-created state directory, so a missing volatile runtime directory does
not prevent the pre-start cache script from running. Runtime/log paths in JSON
are absolute. Existing `/var/log/dserver` logs are retained; any older runtime
logs must be retained in quarantine during migration, not deleted.

Package defaults bind loopback until gonf supplies inventory data. Package assets
and gonf overrides carry the same protected helper and service layout; do not
publish packages or change fleet package versions as part of unapproved staging.
Frontend Gogios DTail checks now target `HOST.wg0.wan.buetow.org:2222`; their stable
check names remain unchanged. Deploy that monitoring configuration with the
frontend bind changes using Operational `frontends_d_tail_checks`: it writes
only `/etc/gogios.json`, using the existing installation. Full `frontends_gogios`
also reconciles packages, accounts, plugins and cron and requires approval for
that wider scope. Coordinate public-port PF closure with task wm3.

For existing frontend dserver installations, Operational
`frontends_d_tail_hardening` applies only the helper/cache, configuration and
rc.d service changes. It reconciles no package or account. The normal
`frontends_d_tail` maintenance task also runs `pkg_add -u dtail`; use the scoped
hardening action for this rollout.

## Evidence collected read-only on 2026-10-22

- blowfish/fishfinger, pi0/pi1 and f0/f1/f2 use daemon-owned 0755 runtime/cache
  directories and wildcard binding. Their BSD host keys are persistent already.
- r0/r1/r2 and pi2 report RPM `dtail-4.3.2-1.ng`, actual unit
  `/usr/lib/systemd/system/dserver.service`, `WorkingDirectory=/var/run/dserver`,
  config `/etc/dserver/dtail.json`, and **inactive** (`MainPID=0`). Their JSON
  points to `cache/ssh_host_key`, but `/var/run/dserver/cache/ssh_host_key` is
  absent. This is the pre-change baseline, not a result of the prepared fix.
- **Linux rollout is blocked** until the previous identity is located and
  restored, or the user explicitly approves provisioning/replacing that host's
  key. No available key may be assumed from the configured filename alone.
- f3 returned no route to host; pi3 SSH timed out. Both need later read-only
  inspection and separate approval; do not wake, reboot or recover them here.
- Native BSD `stat` format and `su -m` were probed read-only. NetBSD/FreeBSD
  daemon accounts work with `su -m`. OpenBSD `_dserver` has an existing invalid
  `nologin` login class for su, so a source file for that service account is
  skipped; client users rex/paul do not use that class. This is not an sshd change.
- DTail's cached-key reader uses ordinary file reads, with no daemon-owner or
  0600-only restriction (also checked in the known frontend build source,
  upstream revision e76e0e6). Actual installed Linux support was verified locally:
  the r0 binary was copied read-only and successfully authenticated a generated
  fixture client as nobody using the helper's **root-owned 0640 group-readable**
  cache. The copied binary reports Go 1.26.8, nozstd, module devel with no VCS
  revision, so no unsupported revision claim is made for that RPM binary.

## Review and local verification

### Approved-base integration, 2026-10-24

The refreshed isolated candidate starts at approved `7e8e97f`, preserving the
deployed DN3, NodePort guard, namespace policies, TLS checks and Garage monitoring.
The reviewed cache helpers and attack/authentication tests are unchanged from
`21ec6f8`; the optional-reboot clarification from `2c6045c` is retained.

Two integration failures were corrected locally before any deployment: the old
patch introduced a duplicate DTailChecks declaration with 0744 permissions, and
its broad Rocky task registration included the newly added standalone VM without
dserver inventory. The candidate retains the current single `0640 root:_gogios`
writer and binds its Linux task to the existing `rocky-dtail` role. Standalone
`rocky` remains outside this server fleet.

Fresh Go generation/build/test/vet/errcheck and the actual fleet plan pass after
these corrections. The plan renders 13 dserver configurations, with no standalone
VM. The three-writer Gogios plan guard passes, retaining protected mute paths and
private permissions. Four root-run cache attack/authentication tests pass with
zero skips using the retained installed-r0 binary copy from 22 October.

Fresh read-only comparison of both complete live Gogios configurations permits
only the two intended frontend DTail target arguments to change to their WG
names. All 572 checks and other configuration fields remain otherwise identical.
These checks are preparation; no daemon, cache, identity or live configuration
was changed. The dated reachability/key findings above are historical and need
host-specific revalidation before any approved migration.

Fresh metadata-only SSH inspection reached both frontends, f0-f3 and Pi0/Pi1.
The original seven-host BSD proposal still has persistent keys at its configured
`/var/db/dserver/ssh_host_key` paths. f3 is reachable and serving, but its existing
key is in `/var/run/dserver/cache/ssh_host_key` and its persistent directory is
absent. Keep f3 outside that proposal until a key-preserving move to persistent
storage is separately prepared and approved. No host-key contents were read.
Linux's dated missing-key findings have not been refreshed or repaired here.

Strict previews of the scoped operations pass on the seven proposed BSD hosts
with zero actual changes. The initial normal frontend previews also proposed
`pkg_add -u dtail`; that unexpected broader action led to the scoped frontend
entry point above. Its actual-plan tests require only dserver helper/config/rc
files, cache refresh and dserver service operations. The real normal-maintenance
plan is a negative control and must fail that scope check. No package operation
is included in the refreshed frontend hardening preview.

```sh
# No live connection or write:
./gonf.sh -list
./gonf.sh plan -o /tmp/dserver-3n3-plan \
  frontends_d_tail_hardening pis_netbsd_dtail_config rocky_dtail_config freebsd_dtail_config
sudo python3 packages/tests/dserver-key-cache.py
# Optional installed Linux binary read-only copy, then local auth fixture:
scp -P 22 root@r0.lan.buetow.org:/usr/local/bin/dserver /tmp/dserver-r0
chmod 755 /tmp/dserver-r0
sudo env DSERVER_TEST_BINARY=/tmp/dserver-r0 python3 packages/tests/dserver-key-cache.py
```

The root-run fixtures rewrite only isolated paths/group constants. They use real
su, stat, cat, mv, Unix ownership and permissions. All four variants exercise
normal/group reads, denied daemon writes, source symlink/owner/hardlink rejection,
stale cleanup, destination symlink/hardlink/FIFO attacks against a root-only
sentinel, unsafe containers, directory refusal and a deterministic source-swap
race through the real su binary. The optional installed-binary fixture uses only
generated test SSH keys and loopback; it does not read or rotate a live host key.
Native BSD end-to-end authentication remains live acceptance, not a claimed local
test. Targeted Go build, vet and errcheck, go generate, and ShellCheck cover the
changed recipes/assets. No Go tests are added under `gonf/`.

## Approved rollout: one reachable canary per OS first

Before approval, review the generated plans and this procedure. Resolve Linux's
missing identity blocker separately. Check the current `ExecStart`, working
directory, configured HostKeyFile/LogDir and package version on each host. Record
only public fingerprints and file metadata; never print private keys. Back up
root-controlled config, helper, rc/unit files into a root-only backup directory.
Record the service's initial enabled/active state and scheduled refresh jobs.
Use a second administrative session. Stay outside BSD daily refresh windows and
confirm no cache updater is running; pause the Linux refresh timer during its
approved migration. Do not stop system cron or change system sshd/PF policies.

For a **BSD canary**, after the approved service stop, quarantine the old runtime
by renaming it into a fresh root-only directory on the same `/var/run` filesystem.
Do not recursively chown/chmod or read anything in the daemon-owned old tree as
root. Keep the recorded persistent host-key fingerprint unchanged. Service commands
are `rcctl stop dserver` on OpenBSD, `service dserver stop` on FreeBSD and
`/etc/rc.d/dserver stop` on NetBSD (as root, after approval). The following is a
root-shell fragment; choose `cache_group=_dserver` for
OpenBSD, `cache_group=dserver` for NetBSD/FreeBSD:

```sh
set -eu
umask 077
cache_group=dserver  # OpenBSD: _dserver
quarantine=$(mktemp -d /var/run/.dserver-3n3.XXXXXXXXXX)
# dserver and its updater must already be stopped/not running.
if [ -L /var/run/dserver ] || [ ! -d /var/run/dserver ]; then
  echo "Unsafe or absent runtime directory; inspect before migrating" >&2
  exit 1
fi
mv /var/run/dserver "$quarantine/runtime"
mkdir -m 700 /var/run/dserver
chown "root:$cache_group" /var/run/dserver
chmod 750 /var/run/dserver
mkdir -m 700 /var/run/dserver/cache
chown "root:$cache_group" /var/run/dserver/cache
chmod 750 /var/run/dserver/cache
# Keep the quarantine and its old logs for review/rollback. Do not delete it.
```

Then deploy only the canary's named task through its inventory host, for example
`./gonf.sh push -privilege=doas -- -p 22 -oBatchMode=yes paul@f0.lan.buetow.org freebsd_dtail_config`
or the corresponding pi0 destination and `pis_netbsd_dtail_config`. Frontends
use `./gonf.sh push -privilege=doas -- -p 2 -oBatchMode=yes rex@blowfish.buetow.org frontends_d_tail_hardening`,
followed by the same approved destination and `frontends_d_tail_checks` configuration. Tasks write the fixed helper, verify/rebuild the cache, write
host-specific JSON and rc/unit files, and restart dserver. Do not deploy an entire
aggregate to bypass a migration failure.

For **Linux**, do not run a generic migration against the current missing-key
baseline. First prepare a host-specific approved key recovery/provisioning step.
If an old key is found in a daemon-controlled tree, read it as dserver into a
fresh root-only staging directory; validate its key format/public fingerprint
against the accepted identity. Never root-copy through a daemon-controlled
source or write/chown through a daemon-controlled destination. Quarantine old
state/runtime trees by atomic same-filesystem rename under fresh root-only
parents. Populate a newly created root-owned private state directory before
handing its key and directory to dserver (0600 and 0700 respectively). Preserve
old logs and timers' prior state. Only then apply the reviewed
`rocky_dtail_config` to that host. Explicit pushes use the inventory's normal
`-privilege=sudo` for
`root@rN.lan.buetow.org` and `-privilege=sudo` for
`paul@piN.lan.buetow.org`, always `-- -p 22 -oBatchMode=yes`. systemd creates writable log/state directories;
the helper creates the root-owned runtime/cache directories at boot. Restore the
approved timer state after acceptance. Native SELinux and systemd startup must
be checked on that canary before proceeding to another Linux host.

After each canary passes, repeat individually for reachable peers. Cluster
commands are available after every member has completed migration:

```sh
./gonf.sh cluster frontends frontends_d_tail_hardening frontends_d_tail_checks
./gonf.sh cluster netbsd-pis pis_netbsd_dtail_config
./gonf.sh cluster rocky-dtail rocky_dtail_config
./gonf.sh cluster freebsd-hosts freebsd_dtail_config
```

Do not use cluster deployment while f3/pi3 are unresolved. A failed directory
precondition is deliberate: stop and investigate rather than chowning the old
cache into compliance.

## Runtime acceptance and rollback

For every host, verify all of these before marking task 3n3 complete:

1. Runtime/cache are root:daemon-group 0750 and cached keys are root:daemon-group
   0640. As the daemon, reading the expected user's cache succeeds and creating a
   cache symlink fails. Use a disposable root-only **sentinel**, never a real
   `/etc/shadow`/doas.conf, if repeating attack tests after approval.
2. The service is healthy, retains the previously accepted host-key fingerprint,
   and listens only on the inventory address at TCP 2222. Check `sockstat` on
   FreeBSD, `netstat` on OpenBSD/NetBSD and `ss` on Linux. No wildcard IPv4/IPv6
   listener may remain. Frontend public-port probes must fail after coordinated
   PF/bind rollout; WG health checks must succeed.
3. From the laptop, use trusted host keys and the existing login identities to
   read an actual OS-readable log under `/var/log/` (frontend HTTP logs under
   `/var/www/logs/` as needed). Use `HOST.wg0.wan.buetow.org` for frontends and
   `HOST.lan.buetow.org` for the lab. Confirm `/etc/fstab` and an unrelated `/tmp`
   file are denied by DTail. Do not use `--trustAllHosts` to hide identity changes.
4. Verify root on r-nodes and paul/rex on their normal hosts still authenticate;
   check health/timer/periodic daily flows and two successive cache refreshes.
   Native BSD 0640 group readability must be proven through dserver, not just ls.
During the approved canary service start, confirm the startup helper creates
missing runtime/cache directories with the required ownership and retains the
persistent host-key fingerprint.

As optional later maintenance verification, observe that a normally scheduled
reboot restores those directories and retains the persistent key. This is not a
task-completion gate. No reboot is authorized by this runbook alone.

If acceptance fails, stop dserver with approval and keep the hardened helper and
root-owned runtime/cache. Restore known-good log permissions or a validated
host-specific config while retaining restricted binding and log-only read rules.
Preserve the same host key. Do not silently restore vulnerable cache scripts,
daemon-writable cache directories, wildcard binding or broad file access. Retain
quarantine/backup material until acceptance and retention are explicitly agreed.
