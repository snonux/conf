# Resolver migration — task 524

Pi3 now has source-owned dormant recovery files, a dormant firewall candidate
and the future VIP wildcard installed. None is activated: before/after profiles,
addresses/routes, loaded firewall, container identity/start/restarts, live DNS
files and boot are unchanged, and OLD UDP/TCP DNS passed. Evidence is private at
`524-pis-20261107/pi3-dormant-deploy/accepted.json`.

The user explicitly deferred Pi2 on 2026-11-07 and requested other work continue.
Pi2's unexplained SSH/HTTP/DTail/DNS outage is not a gate for unrelated hosts.
Pi3 stays on its working old address while it is the sole resolver. Live resolver
renumbering remains incomplete; do not mark task524 done or claim it is ready
for the final rack cutover. Pi0/pi1 work proceeds separately, using working
Pi3 DNS and retained OLD aliases. No Pi2 power action or renewed console request
is authorized by this deferral.

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
It owns only `inet f3s_guard`, retaining old-LAN client trust during staging.
NEW lab/ULA DNS is allowed. Both known wg2 identities receive TCP/UDP53 only
to the resolver pair; laptop SSH/web and phone HTTPS follow router permissions.
Recovery aliases accept only laptop TCP22, before general LAN trust. New Wi-Fi
and camera subnets receive no service trust. Existing diagnostics and old-mode
IPv6 recovery are preserved; the NM profile owns new-mode RA suppression.

Native activation must use the existing `f3s-firewall` check/apply/fresh-confirm
loader, two independent OLD SSH sessions and its own rollback timer. Preserve
old candidate/confirmed state and unrelated Docker/firewalld rules. Never load
the candidate directly with `nft -f` on a host. At accepted policy transition,
update canonical staging so it cannot overwrite the selected migration policy.

`pihole_migration_dns` stages the future VIP wildcard under
`/var/lib/f3s-pihole-migration/`, outside active bind mounts and dnsmasq includes.
The live wildcard remains 192.168.1.138. Do not place even a renamed candidate
in an active include directory. Activate 10.20.0.138 only with accepted storage
and ingress, including the authoritative public-zone update and cache handling.

For each Pi, record synchronized time, persistent NM profile/runtime, Docker
identity/image/state, live DNS configuration and private backup before mutation.
Pi3's read-only baseline confirms eth0, aarch64, healthy pinned Pi-hole, no wg0
and no network migration helper. Pi2's baseline still requires restored access.
Keep one resolver serving DNS while proving native timed restoration, fresh
OLD/NEW/recovery SSH and actual UDP/TCP DNS, then confirm separately. Check both
resolvers and existing Wi-Fi behavior between hosts. Do not change task914's
schedules or infer writer inactivity from an update stamp.

Verification completed locally: Go build/vet/errcheck, ARM64 cross-build, real
helper dispatch fixtures (including eth0/ULA expiry and boot restoration with no
k3s/Flannel operations), shell syntax/shellcheck and native nft packet tests in
disposable namespaces. These checks prove preparation, not native deployment,
DNS service acceptance or a Pi reboot. Read-only baseline and prepared plans
are private under `~/.local/state/conf/524-pis-20261107/`.
