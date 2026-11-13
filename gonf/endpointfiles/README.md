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

## Retired in task a74

The task924 trial machinery is gone: arm/apply/confirm leases, rollback,
status, the every-minute `watchdog` cron, the boot restore hooks
(`gonf-endpoint-boot.service`, `/etc/rc.d/gonf_endpoint_migration`, the
OpenBSD `/etc/rc` block) and the per-service profiles (garage, ups, dtail,
dns, pf, fan, pins). `boot` and `watchdog` remain successful no-ops so a
hook not yet removed cannot fail a boot or mail cron errors.

`Retirement.Retire` (`*_endpoints_retire`) removes those host artifacts,
plus the WireGuard endpoint rollback watchdog and boot hooks that
wireguardmeshgenerator's endpoint tooling installed without a gonf owner
(`gonf-endpoint-rollback*` units on r0-r2; `gonf_endpoint_rollback` /
`gonf_endpoint_runtime` rc.d hooks and the `/etc/crontab` watchdog line on
f0-f3). It is
one-shot: delete `retire.go` and its registrations once every host has run
it (the normally stopped FreeBSD guest runs `freebsd_guest_endpoints_retire`
when it is next up). Left on hosts on purpose: the old journals under
`/var/{db,lib}/gonf-endpoint-migration` and the orphan
`/usr/local/sbin/gonf-endpoint-rollback` with `/var/{db,lib}/gonf-endpoint`.

`check-offline.py` runs the real helper and merger against temporary files,
and the OpenBSD `/etc/rc` edit from `retire.go` against fixtures.
