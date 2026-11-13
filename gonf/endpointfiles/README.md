# Endpoint-file apply helper

`assets/apply` is installed as `/usr/local/sbin/gonf-endpoint-migration`
(on earth into `/usr/local/bin`, behind Fedora's sbin alias) by the owners
that use it, through `InstallHelper`:

- `fleetfiles.MergeHostsIn` (every fleet host's hosts task) runs
  `apply hosts`, which merges `/usr/local/etc/gonf-fleet-hosts` into
  `/etc/hosts` with `gonf-fleet-hosts`.
- `netbsd.Controller.Config` (pi0/pi1) runs `adopt controller` before it
  installs a new `f3sctl.json.gonf-candidate`, then
  `apply controller`. Apply publishes the candidate only while the live
  `f3sctl.json` is an owned file: the legacy input, the current candidate,
  or the copy `adopt` recorded in the state directory
  (`/var/db/gonf-endpoint-migration/controller-owned`). A hand-edited file
  is never adopted and stays refused. The owner's two staging files,
  `f3sctl.json.gonf-candidate` and the legacy input
  `f3sctl.json.gonf-legacy`, were `f3sctl.json.triggerfish-{candidate,legacy}`
  until task f74.

The helper never prints configuration contents.

## Retired in task a74 (code removed in p74)

The task924 trial machinery is gone: arm/apply/confirm leases, rollback,
status, the every-minute `watchdog` cron, the boot restore hooks
(`gonf-endpoint-boot.service`, `/etc/rc.d/gonf_endpoint_migration`, the
OpenBSD `/etc/rc` block) and the per-service profiles (garage, ups, dtail,
dns, pf, fan, pins).

a74's one-shot `*_endpoints_retire` tasks removed those host artifacts,
plus the WireGuard endpoint rollback watchdog and boot hooks that
wireguardmeshgenerator's endpoint tooling installed without a gonf owner
(`gonf-endpoint-rollback*` units on r0-r2; `gonf_endpoint_rollback` /
`gonf_endpoint_runtime` rc.d hooks and the `/etc/crontab` watchdog line on
f0-f3). Every host ran them (the stopped FreeBSD guest in d74,
2026-11-12), and task p74 deleted the code (`retire.go` and its
registrations). Left on hosts on purpose: the old journals under
`/var/{db,lib}/gonf-endpoint-migration` and the orphan
`/usr/local/sbin/gonf-endpoint-rollback` with `/var/{db,lib}/gonf-endpoint`.

`boot` and `watchdog` are still successful no-ops in `assets/apply`. A
read-only check in p74 (2026-11-13) found no caller left on any reachable
fleet host (`/etc`, `/usr/local/etc`, root crontab). They stay only because
editing the helper reinstalls it on every fleet host: drop them, with the
header comment that explains them, the next time `assets/apply` changes for
a real reason.

`check-offline.py` runs the real helper and merger against temporary files.
