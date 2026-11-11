# Resolver migration — task 524

Both resolvers completed shared-wire acceptance on 2026-11-07: NEW-primary
10.20.0.127/.128, default gateway 10.20.0.1, retained OLD addresses, native ULA
and the controller /32 return route through 192.168.1.2. Each separately
confirmed Stage and Promote after fresh OLD/NEW management, UDP/TCP DNS and
admin HTTP checks, then passed an actual confirmed NEW boot. Both former
transaction deadlines elapsed without changing the accepted state.

Pi2 proved the common recovery controller with actual 900-second expiry and
an unconfirmed normal boot. NetworkManager, compose and private TOML were
restored to their exact saved hashes; OLD management and both resolvers' DNS
passed afterwards. Pi3 used that reviewed common proof after matching native
interface, image, healthcheck and Compose preflight. One resolver stayed healthy
through each serial service change and boot. No power cycle or console was used.

Owned approval gates are removed and no network/firewall transaction remains
pending. Source reapply changed no managed file or service and preserved the
boot, container identity/start/restarts, NM fields and protected hashes. Its raw
summary reports one changed resource because the read-only pending-state test
is an executed Command; Docker, daemon reload and restorecon were skipped.
Private reproducible evidence is under
`524-pis-20261107/native-20261107T1050/`, particularly each
`piN-final-acceptance-accepted.json` and the Pi2 expiry/boot receipts.

History: the wildcard pointed at OLD VIP 192.168.1.138 until task i24
(2026-11-11) retired that VIP; it is now 10.20.0.138. The future VIP file was
dormant, outside active mounts/includes. Activation remains coordinated with
storage/ingress acceptance and the canonical mapping step, task 924.

`pihole_network_prepare` reuses the existing Linux snapshot/PID1 watchdog and
boot restoration, with role `resolver`, reviewed `eth0`, and native ULA. The
non-k3s runtime unit and legacy non-k3s snapshot marker avoid all k3s/Flannel
resources. All actions are Operational and excluded from routine aggregates.

| Host | Retained OLD | NEW | Dedicated recovery | Native ULA |
| --- | --- | --- | --- | --- |
| pi2 | 192.168.1.127 | 10.20.0.127 | 10.98.0.127 | fd02:ca0b:f1cb:20::127 |
| pi3 | 192.168.1.128 | 10.20.0.128 | 10.98.0.128 | fd02:ca0b:f1cb:20::128 |

Shared-wire transactions persist the controller 10.99.0.2/32 return route via
192.168.1.2. Dedicated aliases/10.98.0.1 return routing remain behind the later
physical-cable and igc1 gates. Retain OLD resolver addresses and current Wi-Fi
client DNS throughout. Never renumber the only working resolver.

`pihole_firewall_migration_stage` installs only the dormant migration policy.
It owns only `inet f3s_guard`. The old-LAN `192.168.1.0/24` client trust kept
during staging was removed in task 074 (2026-11-11), after i24 retired the
resolvers' old addresses. NEW lab/ULA DNS is allowed. Both known wg2 identities receive TCP/UDP53 only
to the resolver pair; laptop SSH/web and phone HTTPS follow router permissions.
Recovery aliases accept only laptop TCP22, before general LAN trust. New Wi-Fi
and camera subnets receive no service trust. Existing diagnostics and old-mode
IPv6 recovery are preserved; the NM profile owns new-mode RA suppression.

Native activation must use the existing `f3s-firewall` check/apply/fresh-confirm
loader, two independent OLD SSH sessions and its own rollback timer. Preserve
old candidate/confirmed state and unrelated Docker/firewalld rules. Never load
the candidate directly with `nft -f` on a host. At accepted policy transition,
update canonical staging so it cannot overwrite the selected migration policy.
Canonical `pihole_firewall_stage` now stages that same migration policy.

Pi-hole's original `LOCAL` mode rejects routed wg2 clients, even when the host
guard permits them. Canonical compose uses `SINGLE` on `eth0` (plus Pi-hole's
automatic loopback listener), leaving source authorization to the scoped guard.
`pihole_migration_service` stages this compose outside the live directory.
When that candidate exists, the existing resolver network transaction snapshots
both compose and private `pihole.toml` before mutation, then applies the candidate
and recreates only that resolver. Timed rollback restores compose and TOML;
runtime restoration stops FTL and restores TOML again after its shutdown write.
Boot persistent restoration precedes networking, and the resolver runtime hook
waits for Docker before restoring the service. There is no second controller.
Prove application restoration through native expiry and unconfirmed boot before
accepting the changed service. Do not deploy a candidate during a pending trial.

`pihole_migration_dns` stages the future VIP wildcard under
`/var/lib/f3s-pihole-migration/`, outside active bind mounts and dnsmasq includes.
Since i24 the live wildcard is 10.20.0.138 (OLD VIP .138 retired). Do not place even a renamed candidate
in an active include directory. Activate 10.20.0.138 only with accepted storage
and ingress, including the authoritative public-zone update and cache handling.

For each Pi, record synchronized time, persistent NM profile/runtime, Docker
identity/image/state, live DNS configuration and private backup before mutation.
Fresh native baselines confirm eth0, aarch64, healthy pinned Pi-hole and no wg0
on both resolvers. Each has private root-owned pre-mutation backups.
Keep one resolver serving DNS while proving native timed restoration, fresh
OLD/NEW/recovery SSH and actual UDP/TCP DNS, then confirm separately. Check both
resolvers and existing Wi-Fi behavior between hosts. Do not change task914's
schedules or infer writer inactivity from an update stamp.

Verification completed locally: Go build/vet/errcheck, ARM64 cross-build, real
helper dispatch fixtures (including eth0/ULA expiry and boot restoration with no
k3s/Flannel operations, private service-state recovery and unhealthy-restore
journal retention), shell syntax/shellcheck and native nft packet tests in
disposable namespaces. Actual laptop wg2 identity 10.99.0.2 passed management
and DNS on both NEW endpoints. The dormant phone identity's rules have namespace
coverage; no actual phone handshake is claimed. A single OLD Pi3 TCP DNS timeout
during post-router refresh did not reproduce in subsequent complete matrices or
the deadline monitor; the failed observation is retained without assigning a
cause. All 68 deadline-monitor endpoint observations passed UDP and TCP.
